package main

import (
	"math"
)

// DepletionResult holds the result of depletion mode calculation for one strategy
type DepletionResult struct {
	Params                SimulationParams
	SustainableMultiplier float64          // The calculated multiplier that depletes at target age
	MonthlyBeforeAge      float64          // Calculated monthly income before age threshold
	MonthlyAfterAge       float64          // Calculated monthly income after age threshold
	SimulationResult      SimulationResult // Full simulation result with calculated income
	ConvergenceError      float64          // Remaining balance at target age (ideally close to 0)
}

// CalculateDepletionIncome uses binary search to find the sustainable income
// that depletes funds at exactly the target age
func CalculateDepletionIncome(params SimulationParams, config *Config) DepletionResult {
	ic := config.IncomeRequirements

	// Get reference person to calculate target year
	refPerson := config.GetSimulationReferencePerson()
	targetYear := GetBirthYear(refPerson.BirthDate) + ic.TargetDepletionAge

	// Binary search bounds for multiplier
	// The multiplier is applied to the ratio (e.g., multiplier 1000 with ratio 5:3 = 5000:3000/month)
	lowMultiplier := 100.0    // ~£500/month minimum
	highMultiplier := 50000.0 // ~£250k/month maximum

	var bestResult DepletionResult
	tolerance := 1000.0 // Consider converged if balance within £1000
	maxIterations := 100

	// Track best result with lowest absolute balance
	bestBalance := math.MaxFloat64

	for i := 0; i < maxIterations; i++ {
		midMultiplier := (lowMultiplier + highMultiplier) / 2

		// Create modified config with this multiplier
		testConfig := cloneConfigWithMultiplier(config, midMultiplier)

		// Run simulation
		result := RunSimulation(params, testConfig)

		// Find the balance at end of target age year
		balanceAtTarget := getBalanceAtYear(result, targetYear)

		// Store best result so far (one with lowest absolute balance at target)
		beforeAge := ic.IncomeRatioPhase1 * midMultiplier
		afterAge := ic.IncomeRatioPhase2 * midMultiplier

		currentResult := DepletionResult{
			Params:                params,
			SustainableMultiplier: midMultiplier,
			MonthlyBeforeAge:      beforeAge,
			MonthlyAfterAge:       afterAge,
			SimulationResult:      result,
			ConvergenceError:      balanceAtTarget,
		}

		// Keep track of result closest to zero
		if math.Abs(balanceAtTarget) < bestBalance {
			bestBalance = math.Abs(balanceAtTarget)
			bestResult = currentResult
		}

		// Check convergence
		if math.Abs(balanceAtTarget) < tolerance {
			return currentResult
		}

		if balanceAtTarget > 0 {
			// Still have money at target age - can afford more income
			lowMultiplier = midMultiplier
		} else {
			// Ran out of money before target age - need less income
			highMultiplier = midMultiplier
		}
	}

	return bestResult
}

// cloneConfigWithMultiplier creates a copy of config with income set by multiplier
func cloneConfigWithMultiplier(config *Config, multiplier float64) *Config {
	// Create a deep copy of the config
	newConfig := *config

	// Copy slices to avoid shared references
	newConfig.People = make([]PersonConfig, len(config.People))
	copy(newConfig.People, config.People)

	newConfig.Mortgage.Parts = make([]MortgagePartConfig, len(config.Mortgage.Parts))
	copy(newConfig.Mortgage.Parts, config.Mortgage.Parts)

	newConfig.TaxBands = make([]TaxBand, len(config.TaxBands))
	copy(newConfig.TaxBands, config.TaxBands)

	// Calculate income based on ratios and multiplier
	newConfig.IncomeRequirements.MonthlyBeforeAge =
		config.IncomeRequirements.IncomeRatioPhase1 * multiplier
	newConfig.IncomeRequirements.MonthlyAfterAge =
		config.IncomeRequirements.IncomeRatioPhase2 * multiplier

	// Disable depletion mode in the cloned config to use fixed mode in simulation
	newConfig.IncomeRequirements.TargetDepletionAge = 0

	// Income inflation is now enabled in depletion mode for consistency with fixed income mode.
	// Both modes apply the same income inflation rate to required income over time.

	return &newConfig
}

// getBalanceAtYear returns a value indicating how close we are to depleting at target year
// Returns negative if ISA+pension pots depleted BEFORE target year (income too high)
// Returns positive if ISA+pension pots still have money at target year (income too low)
// Returns ~0 if pots deplete at or very close to target year (perfect)
func getBalanceAtYear(result SimulationResult, targetYear int) float64 {
	// Find when ISA+pension pots actually depleted (hit near zero)
	depletionYear := 0
	prevBalance := -1.0

	for _, yearState := range result.Years {
		// Calculate total ISA + pension balance (excluding state pension which is income, not a pot)
		totalPots := yearState.TotalBalance

		// Detect when pots deplete (balance drops below £1000)
		if prevBalance > 1000 && totalPots < 1000 && depletionYear == 0 {
			depletionYear = yearState.Year
		}
		prevBalance = totalPots
	}

	// If pots never depleted during simulation, return the final balance (positive = income too low)
	if depletionYear == 0 {
		if len(result.Years) > 0 {
			return result.Years[len(result.Years)-1].TotalBalance
		}
		return 1000000 // Large positive = need more income
	}

	// If pots depleted before target year, return negative proportional to how early
	if depletionYear < targetYear {
		yearsEarly := targetYear - depletionYear
		return float64(-yearsEarly * 10000)
	}

	// If pots depleted at target year, perfect!
	if depletionYear == targetYear {
		return 0
	}

	// If pots depleted after target year (shouldn't happen with proper search), return positive
	yearsLate := depletionYear - targetYear
	return float64(yearsLate * 10000)
}

// RunAllDepletionCalculations runs depletion calculation for all strategies
func RunAllDepletionCalculations(config *Config) []DepletionResult {
	// Get strategies based on whether there's a mortgage
	strategies := GetDepletionStrategiesForConfig(config)

	// Apply config settings to strategies
	maximizeCoupleISA := config.Strategy.ShouldMaximizeCoupleISA()
	for i := range strategies {
		strategies[i].MaximizeCoupleISA = maximizeCoupleISA
	}

	results := make([]DepletionResult, len(strategies))
	for i, params := range strategies {
		results[i] = CalculateDepletionIncome(params, config)
	}

	return results
}

// FindBestDepletionStrategy returns the index of the best strategy
// Best = highest sustainable income among strategies that actually converge
// Convergence = balance at target is between -£5k and £50k (small overshoot or undershoot)
func FindBestDepletionStrategy(results []DepletionResult) int {
	if len(results) == 0 {
		return -1
	}

	// First pass: find strategies that properly converge (balance close to zero at target)
	convergedIndices := []int{}
	for i, r := range results {
		// Converged if balance is between -£5k (slight overshoot) and £50k (small undershoot)
		if r.ConvergenceError >= -5000 && r.ConvergenceError <= 50000 {
			convergedIndices = append(convergedIndices, i)
		}
	}

	// If no strategies converged, fall back to selecting by best convergence
	if len(convergedIndices) == 0 {
		bestIdx := 0
		bestConvergence := math.Abs(results[0].ConvergenceError)
		for i := 1; i < len(results); i++ {
			convergence := math.Abs(results[i].ConvergenceError)
			if convergence < bestConvergence {
				bestConvergence = convergence
				bestIdx = i
			}
		}
		return bestIdx
	}

	// Among converged strategies, select highest income (then lowest tax as tiebreaker)
	bestIdx := convergedIndices[0]
	bestIncome := results[bestIdx].MonthlyBeforeAge
	bestTax := results[bestIdx].SimulationResult.TotalTaxPaid

	for _, i := range convergedIndices[1:] {
		income := results[i].MonthlyBeforeAge
		tax := results[i].SimulationResult.TotalTaxPaid

		// Higher income is better; if same income, lower tax is better
		if income > bestIncome || (income == bestIncome && tax < bestTax) {
			bestIncome = income
			bestTax = tax
			bestIdx = i
		}
	}

	return bestIdx
}

// DepletionSensitivityResult holds results for one growth rate combination
type DepletionSensitivityResult struct {
	PensionGrowth    float64
	SavingsGrowth    float64
	Results          []DepletionResult
	BestStrategyIdx  int
	BestIncome       float64 // Monthly income (phase 1)
	BestStrategyName string
}

// DepletionSensitivityAnalysis holds the complete sensitivity analysis
type DepletionSensitivityAnalysis struct {
	Config  *Config
	Results []DepletionSensitivityResult
}

// RunDepletionSensitivityAnalysis runs depletion calculations across growth rate combinations
func RunDepletionSensitivityAnalysis(config *Config) DepletionSensitivityAnalysis {
	analysis := DepletionSensitivityAnalysis{
		Config:  config,
		Results: make([]DepletionSensitivityResult, 0),
	}

	sens := config.Sensitivity

	// Iterate through all growth rate combinations
	for pensionGrowth := sens.PensionGrowthMin; pensionGrowth <= sens.PensionGrowthMax+0.001; pensionGrowth += sens.StepSize {
		for savingsGrowth := sens.SavingsGrowthMin; savingsGrowth <= sens.SavingsGrowthMax+0.001; savingsGrowth += sens.StepSize {
			// Clone config with these growth rates
			testConfig := cloneConfigForSensitivity(config, pensionGrowth, savingsGrowth)

			// Run all depletion calculations
			results := RunAllDepletionCalculations(testConfig)

			// Find best strategy
			bestIdx := FindBestDepletionStrategy(results)
			bestIncome := 0.0
			bestName := ""
			if bestIdx >= 0 {
				bestIncome = results[bestIdx].MonthlyBeforeAge
				bestName = results[bestIdx].Params.ShortName()
			}

			analysis.Results = append(analysis.Results, DepletionSensitivityResult{
				PensionGrowth:    pensionGrowth,
				SavingsGrowth:    savingsGrowth,
				Results:          results,
				BestStrategyIdx:  bestIdx,
				BestIncome:       bestIncome,
				BestStrategyName: bestName,
			})
		}
	}

	return analysis
}

// cloneConfigForSensitivity creates a config copy with specific growth rates
func cloneConfigForSensitivity(config *Config, pensionGrowth, savingsGrowth float64) *Config {
	newConfig := *config

	// Copy slices
	newConfig.People = make([]PersonConfig, len(config.People))
	copy(newConfig.People, config.People)

	newConfig.Mortgage.Parts = make([]MortgagePartConfig, len(config.Mortgage.Parts))
	copy(newConfig.Mortgage.Parts, config.Mortgage.Parts)

	newConfig.TaxBands = make([]TaxBand, len(config.TaxBands))
	copy(newConfig.TaxBands, config.TaxBands)

	// Set growth rates
	newConfig.Financial.PensionGrowthRate = pensionGrowth
	newConfig.Financial.SavingsGrowthRate = savingsGrowth

	return &newConfig
}

// CalculatePensionOnlyDepletionIncome uses binary search to find the sustainable income
// that depletes ONLY pensions at the target age (ISAs are preserved)
func CalculatePensionOnlyDepletionIncome(params SimulationParams, config *Config) DepletionResult {
	ic := config.IncomeRequirements

	// Get reference person to calculate target year
	refPerson := config.GetSimulationReferencePerson()
	targetYear := GetBirthYear(refPerson.BirthDate) + ic.TargetDepletionAge

	// Binary search bounds for multiplier
	lowMultiplier := 100.0    // ~£500/month minimum
	highMultiplier := 50000.0 // ~£250k/month maximum

	var bestResult DepletionResult
	tolerance := 1000.0 // Consider converged if pension balance within £1000
	maxIterations := 100

	// Track best result with lowest absolute pension balance
	bestBalance := math.MaxFloat64

	for i := 0; i < maxIterations; i++ {
		midMultiplier := (lowMultiplier + highMultiplier) / 2

		// Create modified config with this multiplier
		testConfig := cloneConfigWithMultiplier(config, midMultiplier)

		// Run simulation with PensionOnly strategy
		pensionOnlyParams := SimulationParams{
			CrystallisationStrategy: params.CrystallisationStrategy,
			DrawdownOrder:           PensionOnly,
			MortgageOpt:             params.MortgageOpt,
		}
		result := RunSimulation(pensionOnlyParams, testConfig)

		// Find the PENSION balance only at end of target age year
		pensionBalanceAtTarget := getPensionBalanceAtYear(result, targetYear)

		// Store best result so far
		beforeAge := ic.IncomeRatioPhase1 * midMultiplier
		afterAge := ic.IncomeRatioPhase2 * midMultiplier

		currentResult := DepletionResult{
			Params:                params,
			SustainableMultiplier: midMultiplier,
			MonthlyBeforeAge:      beforeAge,
			MonthlyAfterAge:       afterAge,
			SimulationResult:      result,
			ConvergenceError:      pensionBalanceAtTarget,
		}

		// Keep track of result closest to zero pension balance
		if math.Abs(pensionBalanceAtTarget) < bestBalance {
			bestBalance = math.Abs(pensionBalanceAtTarget)
			bestResult = currentResult
		}

		// Check convergence
		if math.Abs(pensionBalanceAtTarget) < tolerance {
			return currentResult
		}

		if pensionBalanceAtTarget > 0 {
			// Still have pension money at target age - can afford more income
			lowMultiplier = midMultiplier
		} else {
			// Ran out of pension before target age - need less income
			highMultiplier = midMultiplier
		}
	}

	return bestResult
}

// getPensionBalanceAtYear returns only the pension balance (not ISA) at the end of a specific year
// Returns negative if pension ran out BEFORE the target year
func getPensionBalanceAtYear(result SimulationResult, targetYear int) float64 {
	// Find the year state at target year
	for _, yearState := range result.Years {
		if yearState.Year == targetYear {
			pensionTotal := 0.0
			for _, balances := range yearState.EndBalances {
				pensionTotal += balances.UncrystallisedPot + balances.CrystallisedPot
			}
			return pensionTotal
		}
	}

	// Check last year's pension balance
	if len(result.Years) > 0 {
		lastYear := result.Years[len(result.Years)-1]
		// If we didn't reach target year and pension is depleted, return negative
		if lastYear.Year < targetYear {
			pensionTotal := 0.0
			for _, balances := range lastYear.EndBalances {
				pensionTotal += balances.UncrystallisedPot + balances.CrystallisedPot
			}
			if pensionTotal < 100 { // Effectively depleted
				yearsEarly := targetYear - lastYear.Year
				return float64(-yearsEarly * 10000)
			}
			return pensionTotal
		}
		// Return last year's pension balance
		pensionTotal := 0.0
		for _, balances := range lastYear.EndBalances {
			pensionTotal += balances.UncrystallisedPot + balances.CrystallisedPot
		}
		return pensionTotal
	}
	return 0
}

// RunPensionOnlyDepletionCalculations runs pension-only depletion calculation for strategies
// Uses PensionOnly drawdown with mortgage options based on config
func RunPensionOnlyDepletionCalculations(config *Config) []DepletionResult {
	// Get strategies based on whether there's a mortgage
	strategies := GetPensionOnlyStrategiesForConfig(config)

	results := make([]DepletionResult, len(strategies))
	for i, params := range strategies {
		results[i] = CalculatePensionOnlyDepletionIncome(params, config)
	}

	return results
}

// RunPensionToISADepletionCalculations runs depletion using PensionToISA strategy
// This efficiently moves excess pension money to ISAs while depleting
func RunPensionToISADepletionCalculations(config *Config) []DepletionResult {
	// Get strategies based on whether there's a mortgage
	strategies := GetPensionToISAStrategiesForConfig(config)

	// Apply config settings to strategies
	maximizeCoupleISA := config.Strategy.ShouldMaximizeCoupleISA()
	for i := range strategies {
		strategies[i].MaximizeCoupleISA = maximizeCoupleISA
	}

	results := make([]DepletionResult, len(strategies))
	for i, params := range strategies {
		results[i] = CalculateDepletionIncome(params, config)
	}

	return results
}

// PensionToISASensitivityResult holds results for one growth rate combination with PensionToISA strategy
type PensionToISASensitivityResult struct {
	PensionGrowth    float64
	SavingsGrowth    float64
	Results          []DepletionResult
	BestStrategyIdx  int
	BestIncome       float64
	BestStrategyName string
	FinalISABalance  float64 // How much ISA remains after pension depletion
}

// PensionToISASensitivityAnalysis holds the complete PensionToISA sensitivity analysis
type PensionToISASensitivityAnalysis struct {
	Config  *Config
	Results []PensionToISASensitivityResult
}

// RunPensionToISASensitivityAnalysis runs PensionToISA depletion across growth rate combinations
func RunPensionToISASensitivityAnalysis(config *Config) PensionToISASensitivityAnalysis {
	analysis := PensionToISASensitivityAnalysis{
		Config:  config,
		Results: make([]PensionToISASensitivityResult, 0),
	}

	sens := config.Sensitivity

	// Iterate through all growth rate combinations
	for pensionGrowth := sens.PensionGrowthMin; pensionGrowth <= sens.PensionGrowthMax+0.001; pensionGrowth += sens.StepSize {
		for savingsGrowth := sens.SavingsGrowthMin; savingsGrowth <= sens.SavingsGrowthMax+0.001; savingsGrowth += sens.StepSize {
			// Clone config with these growth rates
			testConfig := cloneConfigForSensitivity(config, pensionGrowth, savingsGrowth)

			// Run PensionToISA depletion calculations
			results := RunPensionToISADepletionCalculations(testConfig)

			// Find best strategy
			bestIdx := FindBestDepletionStrategy(results)
			bestIncome := 0.0
			bestName := ""
			finalISA := 0.0
			if bestIdx >= 0 {
				bestIncome = results[bestIdx].MonthlyBeforeAge
				bestName = results[bestIdx].Params.ShortName()
				// Get final ISA balance
				if len(results[bestIdx].SimulationResult.Years) > 0 {
					lastYear := results[bestIdx].SimulationResult.Years[len(results[bestIdx].SimulationResult.Years)-1]
					for _, balances := range lastYear.EndBalances {
						finalISA += balances.TaxFreeSavings
					}
				}
			}

			analysis.Results = append(analysis.Results, PensionToISASensitivityResult{
				PensionGrowth:    pensionGrowth,
				SavingsGrowth:    savingsGrowth,
				Results:          results,
				BestStrategyIdx:  bestIdx,
				BestIncome:       bestIncome,
				BestStrategyName: bestName,
				FinalISABalance:  finalISA,
			})
		}
	}

	return analysis
}
