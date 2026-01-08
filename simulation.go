package main

import (
	"math"
)

// InitializePeople creates Person structs from config
func InitializePeople(config *Config) []*Person {
	people := make([]*Person, len(config.People))
	for i, pc := range config.People {
		// Use per-person ISA limit if set, otherwise use default of £20,000
		isaLimit := pc.ISAAnnualLimit
		if isaLimit <= 0 {
			isaLimit = 20000
		}
		// Use configured deferral rate or default to 5.8%
		deferralRate := config.Financial.StatePensionDeferralRate
		if deferralRate <= 0 {
			deferralRate = 0.058 // UK default is 5.8% per year
		}

		// Calculate retirement tax year and age from retirement date or age
		retirementTaxYear, retirementAge := pc.GetRetirementInfo()

		// Use PensionAccessAge if set, otherwise default to RetirementAge
		pensionAccessAge := pc.PensionAccessAge
		if pensionAccessAge <= 0 {
			pensionAccessAge = retirementAge
		}

		// ISA to SIPP defaults
		pensionAnnualAllowance := pc.PensionAnnualAllowance
		if pensionAnnualAllowance <= 0 {
			pensionAnnualAllowance = 60000 // UK default
		}
		isaToSIPPMaxPercent := pc.ISAToSIPPMaxPercent
		if isaToSIPPMaxPercent <= 0 {
			isaToSIPPMaxPercent = 1.0 // 100% by default
		}
		isaToSIPPPreserveMonths := pc.ISAToSIPPPreserveMonths
		if isaToSIPPPreserveMonths <= 0 {
			isaToSIPPPreserveMonths = 12 // 12 months by default
		}

		people[i] = &Person{
			Name:              pc.Name,
			BirthYear:         GetBirthYear(pc.BirthDate),
			BirthDate:         pc.BirthDate, // Store full date for tax year age calculations
			RetirementDate:    pc.RetirementDate,
			RetirementAge:     retirementAge,
			RetirementTaxYear: retirementTaxYear,
			PensionAccessAge:  pensionAccessAge,
			StatePensionAge:   pc.StatePensionAge,
			TaxFreeSavings:    pc.TaxFreeSavings,
			UncrystallisedPot: pc.Pension,
			CrystallisedPot:   0,
			ISAAnnualLimit:    isaLimit,
			// DB Pension
			DBPensionAmount:        pc.DBPensionAmount,
			DBPensionStartAge:      pc.DBPensionStartAge,
			DBPensionName:          pc.DBPensionName,
			DBPensionNormalAge:     pc.DBPensionNormalAge,
			DBPensionEarlyFactor:   pc.DBPensionEarlyFactor,
			DBPensionLateFactor:    pc.DBPensionLateFactor,
			DBPensionCommutation:   pc.DBPensionCommutation,
			DBPensionCommuteFactor: pc.DBPensionCommuteFactor,
			// State Pension Deferral
			StatePensionDeferYears:   pc.StatePensionDeferYears,
			StatePensionDeferralRate: deferralRate,
			// Phased Retirement
			PartTimeIncome:   pc.PartTimeIncome,
			PartTimeStartAge: pc.PartTimeStartAge,
			PartTimeEndAge:   pc.PartTimeEndAge,
			// Pre-retirement work income
			WorkIncome: pc.WorkIncome,
			// ISA to SIPP Transfer
			ISAToSIPPEnabled:        pc.ISAToSIPPEnabled,
			PensionAnnualAllowance:  pensionAnnualAllowance,
			EmployerContribution:    pc.EmployerContribution,
			ISAToSIPPMaxPercent:     isaToSIPPMaxPercent,
			ISAToSIPPPreserveMonths: isaToSIPPPreserveMonths,
		}
	}
	return people
}

// ClonePeople creates deep copies of all people
func ClonePeople(people []*Person) []*Person {
	clones := make([]*Person, len(people))
	for i, p := range people {
		clones[i] = p.Clone()
	}
	return clones
}

// GetReferencePerson finds the reference person by name
func GetReferencePerson(people []*Person, name string) *Person {
	for _, p := range people {
		if p.Name == name {
			return p
		}
	}
	return people[0]
}

// ApplyParamsToConfig creates a modified copy of the config with strategy params applied
// This allows factors like GuardrailsEnabled and StatePensionDeferYears
// to override the config settings when they are set in the SimulationParams
func ApplyParamsToConfig(params SimulationParams, config *Config) *Config {
	// Create a shallow copy of the config
	newConfig := *config

	// Deep copy the people slice since we might modify it
	newConfig.People = make([]PersonConfig, len(config.People))
	copy(newConfig.People, config.People)

	// Deep copy nested structs that might be modified
	newConfig.IncomeRequirements = config.IncomeRequirements
	newConfig.Financial = config.Financial

	// Apply GuardrailsEnabled from params (overrides config if set)
	if params.GuardrailsEnabled {
		newConfig.IncomeRequirements.GuardrailsEnabled = true
	}

	// Apply state pension deferral to all people if set in params
	if params.StatePensionDeferYears > 0 {
		for i := range newConfig.People {
			newConfig.People[i].StatePensionDeferYears = params.StatePensionDeferYears
		}
	}

	return &newConfig
}

// RunSimulationV2 runs the simulation with params applied to config
// This is the V2 version that supports the new factor system
func RunSimulationV2(params SimulationParams, config *Config) SimulationResult {
	effectiveConfig := ApplyParamsToConfig(params, config)
	return RunSimulation(params, effectiveConfig)
}

// RunSimulation runs the complete retirement simulation for given parameters
func RunSimulation(params SimulationParams, config *Config) SimulationResult {
	// Initialize people
	people := InitializePeople(config)

	// Get reference person for income requirements and simulation end
	refPersonName := config.IncomeRequirements.ReferencePerson
	simRefPersonName := config.Simulation.ReferencePerson

	// Calculate end year
	simRefPerson := GetReferencePerson(people, simRefPersonName)
	endYear := simRefPerson.BirthYear + config.Simulation.EndAge

	result := SimulationResult{
		Params:        params,
		Years:         make([]YearState, 0),
		FinalBalances: make(map[string]PersonBalances),
	}

	// Calculate initial portfolio value (used for percentage-based income tiers)
	initialPortfolio := 0.0
	for _, p := range people {
		initialPortfolio += p.TotalWealth()
	}

	// Legacy: Base income requirements (annual) - only used if no tiers configured
	baseIncomeBeforeAge := config.IncomeRequirements.MonthlyBeforeAge * 12
	baseIncomeAfterAge := config.IncomeRequirements.MonthlyAfterAge * 12
	ageThreshold := config.IncomeRequirements.AgeThreshold

	// Initialize guardrails state if enabled
	var guardrails *GuardrailsState
	if config.IncomeRequirements.GuardrailsEnabled {
		guardrails = NewGuardrailsState(config)
	}

	// Get growth decline reference person (if enabled)
	var growthDeclineRefPerson *PersonConfig
	var growthDeclineStartAge int
	if config.Financial.GrowthDeclineEnabled {
		growthDeclineRefPerson = config.GetGrowthDeclineReferencePerson()
		if growthDeclineRefPerson != nil {
			growthDeclineStartAge = config.Simulation.StartYear - GetBirthYear(growthDeclineRefPerson.BirthDate)
		}
	}

	// Run simulation year by year
	for year := config.Simulation.StartYear; year <= endYear; year++ {
		state := NewYearState(year)
		yearsFromStart := year - config.Simulation.StartYear

		// Calculate growth rates for this year (may be declining based on age)
		pensionRate := config.Financial.PensionGrowthRate
		savingsRate := config.Financial.SavingsGrowthRate

		if config.Financial.GrowthDeclineEnabled && growthDeclineRefPerson != nil {
			currentAge := year - GetBirthYear(growthDeclineRefPerson.BirthDate)
			targetAge := config.Financial.GrowthDeclineTargetAge

			pensionRate = GetGrowthRateForYear(
				config.Financial.PensionGrowthRate,
				config.Financial.PensionGrowthEndRate,
				growthDeclineStartAge, currentAge, targetAge)
			savingsRate = GetGrowthRateForYear(
				config.Financial.SavingsGrowthRate,
				config.Financial.SavingsGrowthEndRate,
				growthDeclineStartAge, currentAge, targetAge)
		}

		// Store growth rates used this year
		state.PensionGrowthRateUsed = pensionRate
		state.SavingsGrowthRateUsed = savingsRate

		// Apply growth at start of year (except first year)
		if year > config.Simulation.StartYear {
			for _, p := range people {
				ApplyGrowth(p, savingsRate, pensionRate)
			}
		}

		// Calculate current portfolio value (for guardrails)
		currentPortfolio := 0.0
		for _, p := range people {
			currentPortfolio += p.TotalWealth()
		}
		state.StartBalance = currentPortfolio

		// Calculate ages (using tax year age calculation)
		for _, p := range people {
			if p.BirthDate != "" {
				state.Ages[p.Name] = GetAgeInTaxYear(p.BirthDate, year)
			} else {
				state.Ages[p.Name] = year - p.BirthYear
			}
		}

		// Calculate required income (with inflation)
		// Income is only required once the reference person has retired
		refPerson := GetReferencePerson(people, refPersonName)
		var refAge int
		if refPerson.BirthDate != "" {
			refAge = GetAgeInTaxYear(refPerson.BirthDate, year)
		} else {
			refAge = year - refPerson.BirthYear
		}

		// Calculate years since retirement for inflation (not years since simulation start)
		// Use RetirementTaxYear which is calculated from RetirementDate or RetirementAge
		retirementYear := refPerson.RetirementTaxYear
		yearsFromRetirement := year - retirementYear
		if yearsFromRetirement < 0 {
			yearsFromRetirement = 0
		}
		inflationMultiplier := math.Pow(1+config.Financial.IncomeInflationRate, float64(yearsFromRetirement))

		var baseIncome float64
		// Only require income once retired
		if refAge >= refPerson.RetirementAge {
			if config.IncomeRequirements.HasTiers() {
				// Use tiered income system
				annualIncome := config.IncomeRequirements.GetAnnualIncomeForAge(refAge, initialPortfolio, 1.0)

				// Check if this is an investment gains tier (returns -12 from GetAnnualIncomeForAge)
				if annualIncome < 0 {
					// Investment gains income: real returns (growth minus inflation)
					// Calculate total pension and ISA values
					totalPension := 0.0
					totalISA := 0.0
					for _, p := range people {
						totalPension += p.CrystallisedPot + p.UncrystallisedPot
						totalISA += p.TaxFreeSavings
					}

					// Calculate nominal gains
					pensionGains := totalPension * pensionRate
					isaGains := totalISA * savingsRate
					totalGains := pensionGains + isaGains

					// Subtract inflation (real returns)
					inflationLoss := currentPortfolio * config.Financial.IncomeInflationRate
					baseIncome = totalGains - inflationLoss

					// Ensure non-negative (can't have negative income requirement)
					if baseIncome < 0 {
						baseIncome = 0
					}
				} else {
					// Both fixed and percentage tiers: apply inflation to the amount
					// For percentage: 3.5% of initial = £35k, then £35k inflates each year
					baseIncome = annualIncome * inflationMultiplier
				}
			} else {
				// Legacy before/after threshold system
				if refAge < ageThreshold {
					baseIncome = baseIncomeBeforeAge * inflationMultiplier
				} else {
					baseIncome = baseIncomeAfterAge * inflationMultiplier
				}
			}
		}
		state.RequiredIncome = baseIncome

		// Apply guardrails adjustment if enabled (only once retired)
		if guardrails != nil && refAge >= refPerson.RetirementAge {
			if yearsFromRetirement == 0 {
				// Initialize guardrails in first year of retirement
				guardrails.Initialize(currentPortfolio, baseIncome)
			} else {
				// Check if guardrails are triggered
				state.GuardrailsTriggered = guardrails.IsTriggered(currentPortfolio)
				// Apply inflation to current withdrawal, then apply guardrails
				guardrails.CurrentWithdrawal *= (1 + config.Financial.IncomeInflationRate)
				adjustedIncome := guardrails.CalculateAdjustedWithdrawal(currentPortfolio, baseIncome)
				state.GuardrailsAdjusted = adjustedIncome
				state.RequiredIncome = adjustedIncome
			}
		}

		// Update emergency fund minimums for each person
		// Based on configured months of expenses
		if config.Financial.EmergencyFundMonths > 0 {
			monthlyExpenses := state.RequiredIncome / 12
			baseEmergencyFund := monthlyExpenses * float64(config.Financial.EmergencyFundMonths)

			// If inflation-adjusted, use current year's expenses; otherwise use base year
			var emergencyFundPerPerson float64
			if config.Financial.EmergencyFundInflationAdjust {
				emergencyFundPerPerson = baseEmergencyFund / float64(len(people))
			} else {
				// Use base income without inflation adjustment
				var baseMonthly float64
				if config.IncomeRequirements.HasTiers() {
					baseMonthly = config.IncomeRequirements.GetMonthlyIncomeForAge(refAge, initialPortfolio, 1.0)
				} else {
					baseMonthly = config.IncomeRequirements.MonthlyBeforeAge
					if refAge >= ageThreshold {
						baseMonthly = config.IncomeRequirements.MonthlyAfterAge
					}
				}
				emergencyFundPerPerson = (baseMonthly * float64(config.Financial.EmergencyFundMonths)) / float64(len(people))
			}

			for _, p := range people {
				p.EmergencyFundMinimum = emergencyFundPerPerson
			}
		}

		// Handle mortgage payments based on mortgage option
		// Annual payment is calculated from all mortgage parts
		annualPayment := config.GetTotalAnnualPayment()

		// Determine payoff year based on mortgage option
		var payoffYear int
		switch params.MortgageOpt {
		case MortgageEarly:
			payoffYear = config.Mortgage.EarlyPayoffYear
		case MortgageExtended:
			payoffYear = config.GetExtendedEndYear()
		case PCLSMortgagePayoff:
			payoffYear = config.Mortgage.EarlyPayoffYear // PCLS uses early payoff year
		default: // MortgageNormal
			payoffYear = config.Mortgage.EndYear
		}

		// Pay annual payments until payoff year, then pay off remaining balance
		if year < payoffYear {
			state.MortgageCost = annualPayment
		}
		// Track PCLS tax-free available for this year (used for mortgage payoff)
		var pclsTaxFreeTotal float64

		if year == payoffYear {
			state.MortgageCost = config.GetTotalPayoffAmount(year)

			// For PCLS mortgage payoff, take 25% lump sum from each person's pension
			// The tax-free portion is used directly to pay the mortgage (not stored in ISA)
			if params.MortgageOpt == PCLSMortgagePayoff {
				for _, p := range people {
					if p.CanAccessPension(year) && p.UncrystallisedPot > 0 && !p.PCLSTaken {
						result := TakePCLSLumpSum(p)
						if result.AmountCrystallised > 0 {
							// Record the tax-free portion as a withdrawal (it pays the mortgage)
							state.Withdrawals.TaxFreeFromPension[p.Name] += result.TaxFreePortion
							state.Withdrawals.TotalTaxFree += result.TaxFreePortion
							pclsTaxFreeTotal += result.TaxFreePortion

							// Remove from ISA since we're using it directly for mortgage, not storing it
							// (TakePCLSLumpSum added it to TaxFreeSavings, but for PCLS mortgage we use it immediately)
							p.TaxFreeSavings -= result.TaxFreePortion
						}
					}
				}
			}
		}
		// No payments after payoff year

		state.TotalRequired = state.RequiredIncome + state.MortgageCost

		// Calculate state pension income (accounting for deferral enhancement)
		for _, p := range people {
			if p.ReceivesStatePension(year) {
				// Calculate years since this person started receiving state pension
				// Use effective start age which includes deferral
				var effectiveStartYear int
				if p.BirthDate != "" {
					effectiveStartYear = GetTaxYearForAge(p.BirthDate, p.EffectiveStatePensionAge())
				} else {
					effectiveStartYear = p.BirthYear + p.EffectiveStatePensionAge()
				}
				yearsSinceStart := year - effectiveStartYear
				if yearsSinceStart < 0 {
					yearsSinceStart = 0
				}
				// Get the base amount enhanced by any deferral
				baseAmount := p.GetDeferredStatePensionAmount(config.Financial.StatePensionAmount)
				// Apply inflation from when they started receiving it
				pensionInflation := math.Pow(1+config.Financial.StatePensionInflation, float64(yearsSinceStart))
				state.StatePensionByPerson[p.Name] = baseAmount * pensionInflation
				state.TotalStatePension += state.StatePensionByPerson[p.Name]
			}
		}

		// Calculate DB pension income (e.g., Teachers Pension)
		// Uses effective pension after early/late adjustments and commutation
		for _, p := range people {
			if p.ReceivesDBPension(year) {
				// Handle DB pension lump sum (commutation) on first year
				var startYear int
				if p.BirthDate != "" {
					startYear = GetTaxYearForAge(p.BirthDate, p.DBPensionStartAge)
				} else {
					startYear = p.BirthYear + p.DBPensionStartAge
				}
				if year == startYear && !p.DBPensionLumpSumTaken && p.DBPensionCommutation > 0 {
					lumpSum := p.GetDBPensionLumpSum()
					if lumpSum > 0 {
						p.TaxFreeSavings += lumpSum // DB pension lump sum is tax-free
						p.DBPensionLumpSum = lumpSum
						p.DBPensionLumpSumTaken = true
					}
				}

				// Calculate years since this person started receiving DB pension
				yearsSinceStart := year - startYear
				if yearsSinceStart < 0 {
					yearsSinceStart = 0
				}

				// Get effective pension (after early/late and commutation adjustments)
				effectiveDBPension := p.GetEffectiveDBPension()

				// Apply same inflation rate as state pension
				pensionInflation := math.Pow(1+config.Financial.StatePensionInflation, float64(yearsSinceStart))
				state.DBPensionByPerson[p.Name] = effectiveDBPension * pensionInflation
				state.TotalDBPension += state.DBPensionByPerson[p.Name]
			}
		}

		// Calculate part-time income (phased retirement)
		for _, p := range people {
			if p.IsReceivingPartTimeIncome(year) {
				// Apply inflation to part-time income
				partTimeInflation := math.Pow(1+config.Financial.IncomeInflationRate, float64(yearsFromStart))
				state.PartTimeIncome += p.PartTimeIncome * partTimeInflation
			}
		}

		// Calculate work income (pre-retirement employment)
		for _, p := range people {
			if p.IsWorking(year) {
				// Apply inflation to work income (salary increases)
				workInflation := math.Pow(1+config.Financial.IncomeInflationRate, float64(yearsFromStart))
				inflatedWorkIncome := p.WorkIncome * workInflation
				state.WorkIncomeByPerson[p.Name] = inflatedWorkIncome
				state.TotalWorkIncome += inflatedWorkIncome
			}
		}

		// Net amount needed from withdrawals (after state pension, DB pension, part-time income, work income, and PCLS tax-free)
		state.NetRequired = state.TotalRequired - state.TotalStatePension - state.TotalDBPension - state.PartTimeIncome - state.TotalWorkIncome - pclsTaxFreeTotal
		if state.NetRequired < 0 {
			state.NetRequired = 0
		}

		// Split NetRequired into income and mortgage components
		// Other income sources first cover income needs, then mortgage if excess
		totalOtherIncome := state.TotalStatePension + state.TotalDBPension + state.PartTimeIncome + state.TotalWorkIncome + pclsTaxFreeTotal
		if totalOtherIncome >= state.RequiredIncome {
			// Other income fully covers income needs, excess goes to mortgage
			state.NetIncomeRequired = 0
			excessForMortgage := totalOtherIncome - state.RequiredIncome
			state.NetMortgageRequired = state.MortgageCost - excessForMortgage
			if state.NetMortgageRequired < 0 {
				state.NetMortgageRequired = 0
			}
		} else {
			// Other income doesn't fully cover income needs
			state.NetIncomeRequired = state.RequiredIncome - totalOtherIncome
			state.NetMortgageRequired = state.MortgageCost // Full mortgage still needed
		}

		// Inflate tax bands for current year
		taxBands := InflateTaxBands(config.TaxBands, config.Simulation.StartYear, year, config.Financial.TaxBandInflation)

		// Store inflated tax band values for display
		if len(taxBands) > 0 && taxBands[0].Rate == 0 {
			state.PersonalAllowance = taxBands[0].Upper
		}
		if len(taxBands) > 1 && taxBands[1].Rate == 0.20 {
			state.BasicRateLimit = taxBands[1].Upper
		}

		// Execute drawdown (amounts are grossed up to provide net income after tax)
		// Combine state pension, DB pension, and part-time income for tax calculations
		// Preserve PCLS withdrawals that were recorded earlier
		pclsWithdrawals := state.Withdrawals // Save PCLS withdrawals before ExecuteDrawdown overwrites

		if state.NetRequired > 0 {
			taxableIncomeByPerson := make(map[string]float64)
			for _, p := range people {
				taxableIncome := state.StatePensionByPerson[p.Name] + state.DBPensionByPerson[p.Name]
				// Add part-time income if receiving
				if p.IsReceivingPartTimeIncome(year) {
					partTimeInflation := math.Pow(1+config.Financial.IncomeInflationRate, float64(yearsFromStart))
					taxableIncome += p.PartTimeIncome * partTimeInflation
				}
				// Add work income if still employed
				if p.IsWorking(year) {
					taxableIncome += state.WorkIncomeByPerson[p.Name]
				}
				taxableIncomeByPerson[p.Name] = taxableIncome
			}
			state.Withdrawals = ExecuteDrawdown(people, state.NetRequired, params, year, taxableIncomeByPerson, taxBands)
		}

		// Merge PCLS withdrawals back (they were made before ExecuteDrawdown)
		if pclsTaxFreeTotal > 0 {
			for name, amount := range pclsWithdrawals.TaxFreeFromPension {
				state.Withdrawals.TaxFreeFromPension[name] += amount
			}
			state.Withdrawals.TotalTaxFree += pclsWithdrawals.TotalTaxFree
		}

		// Calculate tax for each person (state pension + DB pension + part-time income + work income + taxable withdrawals)
		for _, p := range people {
			statePension := state.StatePensionByPerson[p.Name]
			dbPension := state.DBPensionByPerson[p.Name]
			partTimeIncome := 0.0
			if p.IsReceivingPartTimeIncome(year) {
				partTimeInflation := math.Pow(1+config.Financial.IncomeInflationRate, float64(yearsFromStart))
				partTimeIncome = p.PartTimeIncome * partTimeInflation
			}
			workIncome := state.WorkIncomeByPerson[p.Name] // Will be 0 if not working
			taxableWithdrawal := state.Withdrawals.TaxableFromPension[p.Name]
			// State pension, DB pension, part-time income, and work income are all taxable
			tax := CalculatePersonTax(statePension+dbPension+partTimeIncome+workIncome, taxableWithdrawal, taxBands)
			state.TaxByPerson[p.Name] = tax
			state.TotalTaxPaid += tax
		}

		// Calculate net income received (spendable after tax and mortgage)
		// = State Pension + DB Pension + Part-time income + Work income + Tax-free withdrawals + Taxable withdrawals - Tax paid - Mortgage
		totalWithdrawals := state.Withdrawals.TotalTaxFree + state.Withdrawals.TotalTaxable
		state.NetIncomeReceived = state.TotalStatePension + state.TotalDBPension + state.PartTimeIncome + state.TotalWorkIncome + totalWithdrawals - state.TotalTaxPaid - state.MortgageCost

		// Handle surplus work income - deposit to ISA if work income exceeds expenses
		// This only applies when NetRequired is 0 or negative (all expenses covered by work income)
		if state.TotalWorkIncome > 0 && state.NetRequired == 0 {
			// Calculate how much of work income was needed for expenses
			// Work income is used after state pension, DB pension, part-time income, and PCLS
			otherIncomeExcludingWork := state.TotalStatePension + state.TotalDBPension + state.PartTimeIncome + pclsTaxFreeTotal
			expensesCoveredByOther := math.Min(otherIncomeExcludingWork, state.TotalRequired)
			remainingExpenses := state.TotalRequired - expensesCoveredByOther
			workIncomeUsedForExpenses := math.Min(state.TotalWorkIncome, remainingExpenses)
			surplusWorkIncome := state.TotalWorkIncome - workIncomeUsedForExpenses

			if surplusWorkIncome > 0 {
				// The surplus is already taxed as part of the person's total income
				// Calculate net surplus after tax and deposit to ISA
				for _, p := range people {
					if p.IsWorking(year) && state.WorkIncomeByPerson[p.Name] > 0 {
						// Calculate this person's share of surplus (proportional to their work income)
						personShare := (state.WorkIncomeByPerson[p.Name] / state.TotalWorkIncome) * surplusWorkIncome

						// Estimate effective tax rate on this person's surplus
						// Use the marginal rate based on their total taxable income
						totalTaxable := state.StatePensionByPerson[p.Name] + state.DBPensionByPerson[p.Name] + state.WorkIncomeByPerson[p.Name]
						if p.IsReceivingPartTimeIncome(year) {
							partTimeInflation := math.Pow(1+config.Financial.IncomeInflationRate, float64(yearsFromStart))
							totalTaxable += p.PartTimeIncome * partTimeInflation
						}
						marginalRate := GetMarginalTaxRate(totalTaxable, taxBands)
						netSurplus := personShare * (1 - marginalRate)

						// Deposit to ISA up to annual limit
						isaDeposit := math.Min(netSurplus, p.ISAAnnualLimit)
						p.TaxFreeSavings += isaDeposit
						state.ISAContributions[p.Name] = isaDeposit
						state.TotalISAContributions += isaDeposit
					}
				}
			}
		}

		// ISA to SIPP Transfer Strategy (pre-retirement optimization)
		// While working, transfer ISA funds to pension to get tax relief at marginal rate
		// The gross contribution (net + tax relief) goes into the pension
		// Key insight: £80 from ISA becomes £100 in pension for 20% taxpayer, £133 for 40% taxpayer
		// Both params.ISAToSIPPEnabled (strategy factor) AND p.ISAToSIPPEnabled (per-person config) must be true
		// The per-person setting allows filtering (e.g., DB pension holders shouldn't use this)
		if params.ISAToSIPPEnabled && state.TotalWorkIncome > 0 {
			for _, p := range people {
				// Only transfer if person is working (has earnings) and ISA to SIPP is enabled for them
				// Note: DB pensions (like Teachers Pension) don't accept additional contributions
				// This only applies to DC pensions/SIPPs
				if !p.IsWorking(year) || !p.ISAToSIPPEnabled {
					continue
				}

				// Calculate available pension contribution room
				// Annual allowance is the lower of: annual allowance limit or 100% of earnings
				earnings := state.WorkIncomeByPerson[p.Name]
				annualAllowanceLimit := p.PensionAnnualAllowance - p.EmployerContribution
				availableAllowance := math.Min(annualAllowanceLimit, earnings) * p.ISAToSIPPMaxPercent

				if availableAllowance <= 0 {
					continue
				}

				// Calculate how much ISA is available (preserve minimum months of expenses)
				incomeInflation := math.Pow(1+config.Financial.IncomeInflationRate, float64(yearsFromStart))
				monthlyExpenses := config.IncomeRequirements.MonthlyBeforeAge
				if config.IncomeRequirements.HasTiers() && len(config.IncomeRequirements.Tiers) > 0 {
					monthlyExpenses = config.IncomeRequirements.Tiers[0].MonthlyAmount
				}
				preserveAmount := monthlyExpenses * incomeInflation * float64(p.ISAToSIPPPreserveMonths)
				availableISA := math.Max(0, p.TaxFreeSavings-preserveAmount)

				if availableISA <= 0 {
					continue
				}

				// Calculate marginal tax rate for tax relief
				totalTaxable := state.StatePensionByPerson[p.Name] + state.DBPensionByPerson[p.Name] + earnings
				if p.IsReceivingPartTimeIncome(year) {
					partTimeInflation := math.Pow(1+config.Financial.IncomeInflationRate, float64(yearsFromStart))
					totalTaxable += p.PartTimeIncome * partTimeInflation
				}
				marginalRate := GetMarginalTaxRate(totalTaxable, taxBands)

				// The net amount from ISA (already tax-paid money)
				// When contributed to pension, it gets grossed up by tax relief
				// Net contribution / (1 - marginalRate) = Gross contribution
				// So for 40% taxpayer: £60 net becomes £100 gross (£40 tax relief)
				// For 20% taxpayer: £80 net becomes £100 gross (£20 tax relief)
				netContribution := math.Min(availableISA, availableAllowance)

				// Cap at the amount that can get relief (can't exceed earnings)
				if marginalRate > 0 {
					grossContribution := netContribution / (1 - marginalRate)
					taxRelief := grossContribution - netContribution

					// Transfer: reduce ISA by net amount, increase pension by gross amount
					p.TaxFreeSavings -= netContribution
					p.UncrystallisedPot += grossContribution

					// Track the transfer
					state.ISAToSIPPByPerson[p.Name] = netContribution
					state.ISAToSIPPTaxRelief[p.Name] = taxRelief
					state.TotalISAToSIPP += netContribution
					state.TotalISAToSIPPRelief += taxRelief
				}
			}
		}

		// Record end of year balances
		for _, p := range people {
			state.EndBalances[p.Name] = PersonBalances{
				TaxFreeSavings:    p.TaxFreeSavings,
				UncrystallisedPot: p.UncrystallisedPot,
				CrystallisedPot:   p.CrystallisedPot,
			}
			state.TotalBalance += p.TotalWealth()
		}

		// Check if ran out of money
		totalWithdrawn := state.Withdrawals.TotalTaxFree + state.Withdrawals.TotalTaxable
		if state.NetRequired > 0 && totalWithdrawn < state.NetRequired-1 {
			if !result.RanOutOfMoney {
				result.RanOutOfMoney = true
				result.RanOutYear = year
			}
		}

		result.Years = append(result.Years, state)
		result.TotalTaxPaid += state.TotalTaxPaid
		result.TotalWithdrawn += totalWithdrawn
	}

	// Record final balances
	for _, p := range people {
		result.FinalBalances[p.Name] = PersonBalances{
			TaxFreeSavings:    p.TaxFreeSavings,
			UncrystallisedPot: p.UncrystallisedPot,
			CrystallisedPot:   p.CrystallisedPot,
		}
	}

	return result
}
