package main

import (
	"math"
)

// PersonTaxState tracks taxable income and available funds for a person
type PersonTaxState struct {
	Name                  string
	StatePension          float64
	CurrentTaxableIncome  float64 // Includes state pension + withdrawals so far
	AvailableCrystallised float64
	AvailableUncryst      float64
	AvailableISA          float64
	CanAccessPension      bool
	PCLSTaken             bool // True if 25% PCLS lump sum was taken (no further 25% tax-free)
}

// OptimizedWithdrawalPlan contains the optimal withdrawal amounts per person
type OptimizedWithdrawalPlan struct {
	TaxableFromPension map[string]float64 // Gross taxable withdrawals
	TaxFreeFromPension map[string]float64 // 25% from crystallisation
	TaxFreeFromISA     map[string]float64
	TotalTax           float64
}

// CalculateOptimizedWithdrawals determines the optimal mix of withdrawals
// to minimize total tax while achieving the required net income.
//
// Enhanced Spouse Optimisation Strategy:
// 1. Fill ALL spouses' personal allowances first (tax-free withdrawals)
// 2. Fill basic rate band proportionally across all spouses
// 3. Only then move to higher rate band
// 4. Use ISA (tax-free) only when pension sources are exhausted
func CalculateOptimizedWithdrawals(
	people []*Person,
	netNeeded float64,
	year int,
	statePensionByPerson map[string]float64,
	taxBands []TaxBand,
	strategy Strategy,
) OptimizedWithdrawalPlan {
	plan := OptimizedWithdrawalPlan{
		TaxableFromPension: make(map[string]float64),
		TaxFreeFromPension: make(map[string]float64),
		TaxFreeFromISA:     make(map[string]float64),
	}

	if netNeeded <= 0 {
		return plan
	}

	// Initialize tax state for each person
	states := make([]*PersonTaxState, len(people))
	for i, p := range people {
		states[i] = &PersonTaxState{
			Name:                  p.Name,
			StatePension:          statePensionByPerson[p.Name],
			CurrentTaxableIncome:  statePensionByPerson[p.Name],
			AvailableCrystallised: p.CrystallisedPot,
			AvailableUncryst:      p.UncrystallisedPot,
			AvailableISA:          p.AvailableISA(), // Use available ISA (respects emergency fund)
			CanAccessPension:      p.CanAccessPension(year),
			PCLSTaken:             p.PCLSTaken,
		}
	}

	remaining := netNeeded

	// Phase 1: Fill ALL personal allowances first (0% tax band)
	// This ensures we maximise tax-free income before paying any tax
	remaining = fillAllPersonalAllowances(states, remaining, taxBands, plan.TaxableFromPension, plan.TaxFreeFromPension, strategy)

	// Phase 2: Fill basic rate band proportionally across all spouses
	// This ensures we don't push one spouse into higher rate while another has basic rate space
	if remaining > 0.01 {
		remaining = fillBasicRateBandProportionally(states, remaining, taxBands, plan.TaxableFromPension, plan.TaxFreeFromPension, strategy)
	}

	// Phase 3: Continue with proportional withdrawals at higher rates if needed
	if remaining > 0.01 {
		remaining = proportionalPensionWithdrawals(states, remaining, taxBands, plan.TaxableFromPension, plan.TaxFreeFromPension, strategy)
	}

	// Phase 4: Use ISA money for any remaining need
	remaining = withdrawFromISAsOptimized(states, remaining, plan.TaxFreeFromISA)

	// Calculate total tax
	for _, state := range states {
		taxableWithdrawal := plan.TaxableFromPension[state.Name]
		tax := CalculatePersonTax(state.StatePension, taxableWithdrawal, taxBands)
		plan.TotalTax += tax
	}

	return plan
}

// fillAllPersonalAllowances fills ALL spouses' personal allowances before moving to taxable bands
// This is the key enhancement for spouse optimisation - ensures both partners use their 0% band
func fillAllPersonalAllowances(
	states []*PersonTaxState,
	remaining float64,
	taxBands []TaxBand,
	taxableFromPension map[string]float64,
	taxFreeFromPension map[string]float64,
	strategy Strategy,
) float64 {
	if remaining <= 0 {
		return 0
	}

	// Find personal allowance threshold (0% band upper limit)
	personalAllowance := 12570.0
	if len(taxBands) > 0 && taxBands[0].Rate == 0 {
		personalAllowance = taxBands[0].Upper
	}

	// Calculate total available personal allowance space across all people
	type allowanceInfo struct {
		state              *PersonTaxState
		spaceInAllowance   float64
		availablePension   float64
	}

	var infos []allowanceInfo
	totalSpace := 0.0

	for _, state := range states {
		if !state.CanAccessPension {
			continue
		}

		spaceInAllowance := math.Max(0, personalAllowance-state.CurrentTaxableIncome)
		if spaceInAllowance <= 0 {
			continue
		}

		available := state.AvailableCrystallised
		if strategy == GradualCrystallisation || strategy == UFPLSStrategy {
			available += state.AvailableUncryst
		}
		if available <= 0 {
			continue
		}

		// Effective space is minimum of allowance space and available pension
		effectiveSpace := math.Min(spaceInAllowance, available)
		infos = append(infos, allowanceInfo{state, effectiveSpace, available})
		totalSpace += effectiveSpace
	}

	if totalSpace <= 0 {
		return remaining
	}

	// Calculate how much to withdraw to fill allowances
	// Since withdrawals in personal allowance are tax-free, we just need the net amount
	toWithdraw := math.Min(remaining, totalSpace)

	// Distribute proportionally based on available allowance space
	for _, info := range infos {
		if remaining <= 0.01 {
			break
		}

		share := info.spaceInAllowance / totalSpace
		personAmount := toWithdraw * share
		personAmount = math.Min(personAmount, info.spaceInAllowance)
		personAmount = math.Min(personAmount, remaining)

		if personAmount > 0.01 {
			// Withdraw tax-free from personal allowance
			netReceived := simpleWithdrawPension(info.state, personAmount, taxBands, taxableFromPension, taxFreeFromPension, strategy)
			remaining -= netReceived
		}
	}

	return remaining
}

// fillBasicRateBandProportionally fills the basic rate band proportionally across all spouses
// This prevents pushing one spouse into higher rate while another has basic rate space
func fillBasicRateBandProportionally(
	states []*PersonTaxState,
	remaining float64,
	taxBands []TaxBand,
	taxableFromPension map[string]float64,
	taxFreeFromPension map[string]float64,
	strategy Strategy,
) float64 {
	if remaining <= 0 {
		return 0
	}

	// Find basic rate band limits
	basicRateLower := 12570.0
	basicRateUpper := 50270.0
	basicRate := 0.20

	for _, band := range taxBands {
		if band.Rate > 0 && band.Rate <= 0.25 { // Basic rate is typically 20%
			basicRateLower = band.Lower
			basicRateUpper = band.Upper
			basicRate = band.Rate
			break
		}
	}

	// Calculate available space in basic rate band for each person
	type bandInfo struct {
		state          *PersonTaxState
		spaceInBand    float64
		available      float64
	}

	var infos []bandInfo
	totalSpace := 0.0

	for _, state := range states {
		if !state.CanAccessPension {
			continue
		}

		// If already above basic rate band, skip
		if state.CurrentTaxableIncome >= basicRateUpper {
			continue
		}

		// Calculate space in basic rate band
		startInBand := math.Max(state.CurrentTaxableIncome, basicRateLower)
		spaceInBand := basicRateUpper - startInBand
		if spaceInBand <= 0 {
			continue
		}

		available := state.AvailableCrystallised
		if strategy == GradualCrystallisation || strategy == UFPLSStrategy {
			available += state.AvailableUncryst
		}
		if available <= 0 {
			continue
		}

		effectiveSpace := math.Min(spaceInBand, available)
		infos = append(infos, bandInfo{state, effectiveSpace, available})
		totalSpace += effectiveSpace
	}

	if totalSpace <= 0 {
		return remaining
	}

	// Calculate net amount we can get from basic rate band
	// For each £1 gross, we get £(1-rate) net
	// So to get X net, we need X/(1-rate) gross
	grossNeededForRemaining := remaining / (1 - basicRate)

	// But with crystallisation, we also get 25% tax-free
	// Adjust for that benefit
	if strategy == GradualCrystallisation || strategy == UFPLSStrategy {
		// When crystallising X: 0.25X is tax-free, 0.75X is taxable at basic rate
		// Net = 0.25X + 0.75X*(1-rate) = 0.25X + 0.75X*0.8 = 0.25X + 0.6X = 0.85X
		// So to get Y net, we need Y/0.85 to crystallise
		grossNeededForRemaining = remaining / 0.85
	}

	toWithdraw := math.Min(grossNeededForRemaining, totalSpace)

	// Distribute proportionally based on available band space
	for _, info := range infos {
		if remaining <= 0.01 {
			break
		}

		share := info.spaceInBand / totalSpace
		personGross := toWithdraw * share
		personGross = math.Min(personGross, info.spaceInBand)

		// Convert to net target for this person
		personNet := personGross * (1 - basicRate)
		if strategy == GradualCrystallisation || strategy == UFPLSStrategy {
			personNet = personGross * 0.85 // 25% tax-free + 75%*80%
		}
		personNet = math.Min(personNet, remaining)

		if personNet > 0.01 {
			netReceived := simpleWithdrawPension(info.state, personNet, taxBands, taxableFromPension, taxFreeFromPension, strategy)
			remaining -= netReceived
		}
	}

	return remaining
}

// fillPersonalAllowances fills each person's personal allowance with taxable pension
func fillPersonalAllowances(
	states []*PersonTaxState,
	remaining float64,
	taxBands []TaxBand,
	taxableFromPension map[string]float64,
	taxFreeFromPension map[string]float64,
	strategy Strategy,
) float64 {
	if remaining <= 0 {
		return 0
	}

	// Find personal allowance threshold (0% band upper limit)
	personalAllowance := 12570.0 // Default UK personal allowance
	if len(taxBands) > 0 {
		personalAllowance = taxBands[0].Upper
	}

	// For each person, calculate how much personal allowance remains after state pension
	for _, state := range states {
		if remaining <= 0 {
			break
		}
		if !state.CanAccessPension {
			continue
		}

		allowanceRemaining := personalAllowance - state.CurrentTaxableIncome
		if allowanceRemaining <= 0 {
			continue
		}

		// Calculate how much we can withdraw tax-free using remaining personal allowance
		netToWithdraw := math.Min(remaining, allowanceRemaining)

		// Try to get this from pension (crystallised first, then uncrystallised if gradual)
		netReceived := withdrawPensionForPerson(state, netToWithdraw, taxBands, taxableFromPension, taxFreeFromPension, strategy)
		remaining -= netReceived
	}

	return remaining
}

// balanceTaxableWithdrawals withdraws from the person with lowest marginal rate first
func balanceTaxableWithdrawals(
	states []*PersonTaxState,
	remaining float64,
	taxBands []TaxBand,
	taxableFromPension map[string]float64,
	taxFreeFromPension map[string]float64,
	strategy Strategy,
) float64 {
	if remaining <= 0 {
		return 0
	}

	maxIterations := 1000 // Safety limit

	// Iteratively withdraw from the person with the lowest marginal tax rate
	for iteration := 0; remaining > 0.01 && iteration < maxIterations; iteration++ {
		// Find person with lowest marginal rate who can still withdraw
		var bestState *PersonTaxState
		bestRate := 1.0 // Start with impossibly high rate

		for _, state := range states {
			if !state.CanAccessPension {
				continue
			}
			available := state.AvailableCrystallised
			if strategy == GradualCrystallisation || strategy == UFPLSStrategy {
				available += state.AvailableUncryst
			}
			if available <= 0.01 {
				continue
			}

			marginalRate := GetMarginalRate(state.CurrentTaxableIncome, taxBands)
			if marginalRate < bestRate {
				bestRate = marginalRate
				bestState = state
			}
		}

		if bestState == nil {
			// No more pension available
			break
		}

		// Calculate how much to withdraw from this person
		// Withdraw up to the next tax band boundary or remaining amount
		nextBandThreshold := getNextBandThreshold(bestState.CurrentTaxableIncome, taxBands)
		roomInBand := nextBandThreshold - bestState.CurrentTaxableIncome

		if roomInBand <= 0.01 {
			// Already at band boundary, force a small withdrawal to move to next band
			roomInBand = remaining
		}

		// Calculate net amount we can get with this room (accounting for tax)
		maxGross := roomInBand
		taxOnMax := CalculateMarginalTax(maxGross, bestState.CurrentTaxableIncome, taxBands)
		maxNet := maxGross - taxOnMax
		netToWithdraw := math.Min(remaining, maxNet)

		if netToWithdraw <= 0.01 {
			break
		}

		// Cap at available
		available := bestState.AvailableCrystallised
		if strategy == GradualCrystallisation || strategy == UFPLSStrategy {
			available += bestState.AvailableUncryst
		}

		prevRemaining := remaining
		netReceived := withdrawPensionForPerson(bestState, netToWithdraw, taxBands, taxableFromPension, taxFreeFromPension, strategy)
		remaining -= netReceived

		// Safety check: if we didn't make progress, break to avoid infinite loop
		if math.Abs(remaining-prevRemaining) < 0.01 {
			break
		}
	}

	return remaining
}

// withdrawPensionForPerson withdraws from a person's pension to achieve netNeeded after tax
func withdrawPensionForPerson(
	state *PersonTaxState,
	netNeeded float64,
	taxBands []TaxBand,
	taxableFromPension map[string]float64,
	taxFreeFromPension map[string]float64,
	strategy Strategy,
) float64 {
	if netNeeded <= 0 {
		return 0
	}

	netReceived := 0.0

	// First: draw from crystallised pot
	if state.AvailableCrystallised > 0 {
		grossNeeded, _ := GrossUpForTax(netNeeded-netReceived, state.CurrentTaxableIncome, taxBands)
		grossNeeded = math.Min(grossNeeded, state.AvailableCrystallised)

		if grossNeeded > 0 {
			tax := CalculateMarginalTax(grossNeeded, state.CurrentTaxableIncome, taxBands)
			net := grossNeeded - tax

			state.AvailableCrystallised -= grossNeeded
			state.CurrentTaxableIncome += grossNeeded
			taxableFromPension[state.Name] += grossNeeded
			netReceived += net
		}
	}

	// Second: withdraw from uncrystallised pot if gradual or UFPLS strategy and still need more
	if (strategy == GradualCrystallisation || strategy == UFPLSStrategy) && state.AvailableUncryst > 0 && netReceived < netNeeded {
		stillNeeded := netNeeded - netReceived

		// If PCLS already taken and using GradualCrystallisation, all is taxable (no 25% tax-free)
		// For UFPLS, always get 25% tax-free regardless of PCLSTaken
		if state.PCLSTaken && strategy == GradualCrystallisation {
			grossNeeded, _ := GrossUpForTax(stillNeeded, state.CurrentTaxableIncome, taxBands)
			grossNeeded = math.Min(grossNeeded, state.AvailableUncryst)

			if grossNeeded > 0.01 {
				tax := CalculateMarginalTax(grossNeeded, state.CurrentTaxableIncome, taxBands)
				net := grossNeeded - tax

				state.AvailableUncryst -= grossNeeded
				state.CurrentTaxableIncome += grossNeeded
				taxableFromPension[state.Name] += grossNeeded
				netReceived += net
			}
		} else {
			// When crystallising/UFPLS: 25% is tax-free, 75% is taxable
			// Iteratively find the right amount to withdraw
			toCrystallise := stillNeeded * 1.3 // Initial estimate

			for i := 0; i < 20; i++ {
				if toCrystallise > state.AvailableUncryst {
					toCrystallise = state.AvailableUncryst
				}

				taxFree := toCrystallise * 0.25
				taxableGross := toCrystallise * 0.75
				taxOnTaxable := CalculateMarginalTax(taxableGross, state.CurrentTaxableIncome, taxBands)
				taxableNet := taxableGross - taxOnTaxable
				totalNet := taxFree + taxableNet

				if math.Abs(totalNet-stillNeeded) < 1 {
					break
				}
				ratio := stillNeeded / totalNet
				toCrystallise = toCrystallise * ratio
			}

			if toCrystallise > state.AvailableUncryst {
				toCrystallise = state.AvailableUncryst
			}

			if toCrystallise > 0.01 {
				taxFree := toCrystallise * 0.25
				taxableGross := toCrystallise * 0.75
				taxOnTaxable := CalculateMarginalTax(taxableGross, state.CurrentTaxableIncome, taxBands)
				taxableNet := taxableGross - taxOnTaxable

				state.AvailableUncryst -= toCrystallise
				state.CurrentTaxableIncome += taxableGross
				taxableFromPension[state.Name] += taxableGross
				taxFreeFromPension[state.Name] += taxFree
				netReceived += taxFree + taxableNet
			}
		}
	}

	return netReceived
}

// withdrawFromISAsOptimized withdraws from ISAs proportionally
func withdrawFromISAsOptimized(
	states []*PersonTaxState,
	remaining float64,
	taxFreeFromISA map[string]float64,
) float64 {
	if remaining <= 0 {
		return 0
	}

	totalISA := 0.0
	for _, state := range states {
		totalISA += state.AvailableISA
	}

	if totalISA > 0 {
		isaNeeded := math.Min(remaining, totalISA)
		for _, state := range states {
			if state.AvailableISA > 0 {
				share := state.AvailableISA / totalISA
				withdrawal := math.Min(isaNeeded*share, state.AvailableISA)
				state.AvailableISA -= withdrawal
				taxFreeFromISA[state.Name] += withdrawal
				remaining -= withdrawal
			}
		}
	}

	return remaining
}

// getNextBandThreshold returns the upper limit of the current tax band
func getNextBandThreshold(income float64, bands []TaxBand) float64 {
	for _, band := range bands {
		if income >= band.Lower && income < band.Upper {
			return band.Upper
		}
	}
	// Above all bands
	return income + 100000 // Large number for highest band
}

// fillPersonalAllowancesSimple fills each person's personal allowance with taxable pension
// using proportional allocation to balance between people
func fillPersonalAllowancesSimple(
	states []*PersonTaxState,
	remaining float64,
	taxBands []TaxBand,
	taxableFromPension map[string]float64,
	taxFreeFromPension map[string]float64,
	strategy Strategy,
) float64 {
	if remaining <= 0 {
		return 0
	}

	// Find personal allowance threshold (0% band upper limit)
	personalAllowance := 12570.0
	if len(taxBands) > 0 {
		personalAllowance = taxBands[0].Upper
	}

	// Calculate total available personal allowance across all people
	type allowanceInfo struct {
		state              *PersonTaxState
		allowanceRemaining float64
		availablePension   float64
	}

	var infos []allowanceInfo
	totalAllowance := 0.0

	for _, state := range states {
		if !state.CanAccessPension {
			continue
		}

		allowanceRemaining := personalAllowance - state.CurrentTaxableIncome
		if allowanceRemaining <= 0 {
			continue
		}

		available := state.AvailableCrystallised
		if strategy == GradualCrystallisation || strategy == UFPLSStrategy {
			available += state.AvailableUncryst
		}
		if available <= 0 {
			continue
		}

		// Cap allowance at what we can actually withdraw
		effectiveAllowance := math.Min(allowanceRemaining, available)
		infos = append(infos, allowanceInfo{state, effectiveAllowance, available})
		totalAllowance += effectiveAllowance
	}

	if totalAllowance <= 0 {
		return remaining
	}

	// Distribute withdrawal proportionally among people based on their available allowance
	toWithdraw := math.Min(remaining, totalAllowance)

	for _, info := range infos {
		if remaining <= 0.01 {
			break
		}

		share := info.allowanceRemaining / totalAllowance
		personAmount := toWithdraw * share
		personAmount = math.Min(personAmount, info.allowanceRemaining)
		personAmount = math.Min(personAmount, remaining)

		if personAmount > 0.01 {
			netReceived := simpleWithdrawPension(info.state, personAmount, taxBands, taxableFromPension, taxFreeFromPension, strategy)
			remaining -= netReceived
		}
	}

	return remaining
}

// getEffectiveTaxRate calculates the effective tax rate for a person considering
// whether they have uncrystallised funds that provide 25% tax-free benefit.
// When crystallising: 25% is tax-free, 75% is taxable at marginal rate.
// Effective rate = 0.75 × marginal_rate
// Note: If PCLSTaken is true, no 25% tax-free benefit applies (for GradualCrystallisation).
// For UFPLS, each withdrawal is always 25% tax-free regardless of PCLSTaken.
func getEffectiveTaxRate(state *PersonTaxState, marginalRate float64, strategy Strategy) float64 {
	if strategy == UFPLSStrategy && state.AvailableUncryst > 0.01 {
		// UFPLS: each withdrawal is always 25% tax-free, regardless of PCLSTaken
		// So effective tax rate is only 75% of the marginal rate
		return marginalRate * 0.75
	}
	if strategy == GradualCrystallisation && state.AvailableUncryst > 0.01 && !state.PCLSTaken {
		// When we have uncrystallised funds and haven't taken PCLS, we get 25% tax-free
		// So effective tax rate is only 75% of the marginal rate
		return marginalRate * 0.75
	}
	// Crystallised funds are 100% taxable at marginal rate
	// Also applies if PCLS was already taken (for GradualCrystallisation)
	return marginalRate
}

// proportionalPensionWithdrawals withdraws from pension proportionally by pot size
// This mimics PensionFirst behavior: apply share to current remaining, not original amount
// This naturally keeps smaller pot holders within their personal allowance
func proportionalPensionWithdrawals(
	states []*PersonTaxState,
	remaining float64,
	taxBands []TaxBand,
	taxableFromPension map[string]float64,
	taxFreeFromPension map[string]float64,
	strategy Strategy,
) float64 {
	if remaining <= 0 {
		return 0
	}

	// Calculate total available pension (snapshot at start, like PensionFirst)
	totalAvailable := 0.0
	for _, state := range states {
		if !state.CanAccessPension {
			continue
		}
		available := state.AvailableCrystallised
		if strategy == GradualCrystallisation || strategy == UFPLSStrategy {
			available += state.AvailableUncryst
		}
		totalAvailable += available
	}

	if totalAvailable <= 0.01 {
		return remaining
	}

	// Process each person sequentially, applying their share to current remaining
	// This matches PensionFirst behavior where remaining decreases between people
	for _, state := range states {
		if remaining <= 0.01 {
			break
		}
		if !state.CanAccessPension {
			continue
		}

		available := state.AvailableCrystallised
		if strategy == GradualCrystallisation || strategy == UFPLSStrategy {
			available += state.AvailableUncryst
		}
		if available <= 0.01 {
			continue
		}

		// Calculate share based on original total, applied to current remaining
		share := available / totalAvailable
		personTargetNet := remaining * share

		if personTargetNet > 0.01 {
			netReceived := simpleWithdrawPension(state, personTargetNet, taxBands, taxableFromPension, taxFreeFromPension, strategy)
			remaining -= netReceived
		}
	}

	return remaining
}

// balancedWithdrawals withdraws proportionally from people in the same tax band
// to achieve optimal balancing
func balancedWithdrawals(
	states []*PersonTaxState,
	remaining float64,
	taxBands []TaxBand,
	taxableFromPension map[string]float64,
	taxFreeFromPension map[string]float64,
	strategy Strategy,
) float64 {
	if remaining <= 0 {
		return 0
	}

	maxIterations := 500

	for iteration := 0; iteration < maxIterations && remaining > 0.01; iteration++ {
		// Group people by effective rate and calculate available funds
		type personInfo struct {
			state         *PersonTaxState
			effectiveRate float64
			available     float64
			roomInBand    float64
		}

		var infos []personInfo
		lowestRate := 2.0

		for _, state := range states {
			if !state.CanAccessPension {
				continue
			}
			available := state.AvailableCrystallised
			if strategy == GradualCrystallisation || strategy == UFPLSStrategy {
				available += state.AvailableUncryst
			}
			if available <= 0.01 {
				continue
			}

			marginalRate := GetMarginalRate(state.CurrentTaxableIncome, taxBands)
			// Use effective rate that accounts for 25% tax-free crystallisation benefit
			effectiveRate := getEffectiveTaxRate(state, marginalRate, strategy)
			nextThreshold := getNextBandThreshold(state.CurrentTaxableIncome, taxBands)
			roomInBand := nextThreshold - state.CurrentTaxableIncome

			// When crystallising/UFPLS, only 75% counts toward taxable income
			// So we can withdraw more before hitting the next band
			// If we have X room in band, we can withdraw X/0.75 (with 25% tax-free)
			// For GradualCrystallisation: only applies if PCLS not taken
			// For UFPLS: always applies (each withdrawal is 25% tax-free)
			if state.AvailableUncryst > 0.01 &&
				(strategy == UFPLSStrategy || (strategy == GradualCrystallisation && !state.PCLSTaken)) {
				roomInBand = roomInBand / 0.75
			}

			infos = append(infos, personInfo{state, effectiveRate, available, roomInBand})
			if effectiveRate < lowestRate {
				lowestRate = effectiveRate
			}
		}

		if len(infos) == 0 {
			break
		}

		// Filter to only people at the lowest effective rate
		var lowestRateInfos []personInfo
		totalAvailable := 0.0
		totalRoom := 0.0

		for _, info := range infos {
			if math.Abs(info.effectiveRate-lowestRate) < 0.001 {
				lowestRateInfos = append(lowestRateInfos, info)
				totalAvailable += info.available
				totalRoom += info.roomInBand
			}
		}

		if len(lowestRateInfos) == 0 || totalAvailable <= 0.01 {
			break
		}

		// Calculate how much we can withdraw at this rate before anyone moves to next band
		// Use proportional splitting within the band
		maxNetAtThisRate := 0.0
		for _, info := range lowestRateInfos {
			// Net from this person's room in band
			grossRoom := math.Min(info.roomInBand, info.available)
			tax := grossRoom * info.effectiveRate
			netRoom := grossRoom - tax
			maxNetAtThisRate += netRoom
		}

		netToWithdraw := math.Min(remaining, maxNetAtThisRate)
		if netToWithdraw <= 0.01 {
			// Force a small withdrawal to make progress
			netToWithdraw = math.Min(remaining, 100)
		}

		// Split proportionally among people at this rate based on available funds
		for _, info := range lowestRateInfos {
			if remaining <= 0.01 {
				break
			}

			share := info.available / totalAvailable
			personNet := netToWithdraw * share
			personNet = math.Min(personNet, remaining)

			if personNet > 0.01 {
				netReceived := simpleWithdrawPension(info.state, personNet, taxBands, taxableFromPension, taxFreeFromPension, strategy)
				remaining -= netReceived
			}
		}
	}

	return remaining
}

// simpleWithdrawPension withdraws from pension using a straightforward approach
func simpleWithdrawPension(
	state *PersonTaxState,
	netNeeded float64,
	taxBands []TaxBand,
	taxableFromPension map[string]float64,
	taxFreeFromPension map[string]float64,
	strategy Strategy,
) float64 {
	if netNeeded <= 0 {
		return 0
	}

	netReceived := 0.0

	// For UFPLS, prefer uncrystallised pot first (each withdrawal gets 25% tax-free)
	if strategy == UFPLSStrategy && state.AvailableUncryst > 0.01 && netReceived < netNeeded {
		stillNeeded := netNeeded - netReceived

		// Binary search to find the right withdrawal amount (25% tax-free, 75% taxable)
		low := 0.0
		high := state.AvailableUncryst
		var toWithdraw float64

		for i := 0; i < 50; i++ {
			mid := (low + high) / 2
			taxFree := mid * 0.25
			taxableGross := mid * 0.75
			taxOnTaxable := CalculateMarginalTax(taxableGross, state.CurrentTaxableIncome, taxBands)
			totalNet := taxFree + (taxableGross - taxOnTaxable)

			if math.Abs(totalNet-stillNeeded) < 0.01 {
				toWithdraw = mid
				break
			}

			if totalNet < stillNeeded {
				low = mid
			} else {
				high = mid
			}
			toWithdraw = mid
		}

		toWithdraw = math.Min(toWithdraw, state.AvailableUncryst)

		if toWithdraw > 0.01 {
			taxFree := toWithdraw * 0.25
			taxableGross := toWithdraw * 0.75
			taxOnTaxable := CalculateMarginalTax(taxableGross, state.CurrentTaxableIncome, taxBands)
			taxableNet := taxableGross - taxOnTaxable

			state.AvailableUncryst -= toWithdraw
			state.CurrentTaxableIncome += taxableGross
			taxableFromPension[state.Name] += taxableGross
			taxFreeFromPension[state.Name] += taxFree
			netReceived += taxFree + taxableNet
		}
	}

	// Draw from crystallised pot (for all strategies, or as fallback for UFPLS)
	if state.AvailableCrystallised > 0.01 && netReceived < netNeeded {
		stillNeeded := netNeeded - netReceived
		grossNeeded, _ := GrossUpForTax(stillNeeded, state.CurrentTaxableIncome, taxBands)
		grossNeeded = math.Min(grossNeeded, state.AvailableCrystallised)

		if grossNeeded > 0.01 {
			tax := CalculateMarginalTax(grossNeeded, state.CurrentTaxableIncome, taxBands)
			net := grossNeeded - tax

			state.AvailableCrystallised -= grossNeeded
			state.CurrentTaxableIncome += grossNeeded
			taxableFromPension[state.Name] += grossNeeded
			netReceived += net
		}
	}

	// Crystallise more if gradual strategy (not UFPLS - that's handled above)
	if strategy == GradualCrystallisation && state.AvailableUncryst > 0.01 && netReceived < netNeeded {
		stillNeeded := netNeeded - netReceived

		// If PCLS already taken, all crystallisation is taxable (no 25% tax-free)
		if state.PCLSTaken {
			// Gross up for tax - all is taxable
			grossNeeded, _ := GrossUpForTax(stillNeeded, state.CurrentTaxableIncome, taxBands)
			grossNeeded = math.Min(grossNeeded, state.AvailableUncryst)

			if grossNeeded > 0.01 {
				tax := CalculateMarginalTax(grossNeeded, state.CurrentTaxableIncome, taxBands)
				net := grossNeeded - tax

				state.AvailableUncryst -= grossNeeded
				state.CurrentTaxableIncome += grossNeeded
				taxableFromPension[state.Name] += grossNeeded
				netReceived += net
			}
		} else {
			// Binary search to find the right crystallisation amount (with 25% tax-free)
			low := 0.0
			high := state.AvailableUncryst
			var toCrystallise float64

			for i := 0; i < 50; i++ {
				mid := (low + high) / 2
				taxFree := mid * 0.25
				taxableGross := mid * 0.75
				taxOnTaxable := CalculateMarginalTax(taxableGross, state.CurrentTaxableIncome, taxBands)
				totalNet := taxFree + (taxableGross - taxOnTaxable)

				if math.Abs(totalNet-stillNeeded) < 0.01 {
					toCrystallise = mid
					break
				}

				if totalNet < stillNeeded {
					low = mid
				} else {
					high = mid
				}
				toCrystallise = mid
			}

			toCrystallise = math.Min(toCrystallise, state.AvailableUncryst)

			if toCrystallise > 0.01 {
				taxFree := toCrystallise * 0.25
				taxableGross := toCrystallise * 0.75
				taxOnTaxable := CalculateMarginalTax(taxableGross, state.CurrentTaxableIncome, taxBands)
				taxableNet := taxableGross - taxOnTaxable

				state.AvailableUncryst -= toCrystallise
				state.CurrentTaxableIncome += taxableGross
				taxableFromPension[state.Name] += taxableGross
				taxFreeFromPension[state.Name] += taxFree
				netReceived += taxFree + taxableNet
			}
		}
	}

	return netReceived
}
