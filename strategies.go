package main

import (
	"math"
)

// CrystallisationResult holds the result of a crystallisation operation
type CrystallisationResult struct {
	AmountCrystallised float64
	TaxFreePortion     float64
	TaxablePortion     float64
}

// TakePCLSLumpSum takes the 25% PCLS lump sum from a person's entire pension pot
// 25% becomes tax-free (added to ISA), 75% becomes crystallised (taxable)
// This sets PCLSTaken = true, so no further 25% tax-free on future withdrawals
func TakePCLSLumpSum(person *Person) CrystallisationResult {
	if person.UncrystallisedPot <= 0 || person.PCLSTaken {
		return CrystallisationResult{}
	}

	amount := person.UncrystallisedPot
	taxFree := amount * 0.25
	taxable := amount * 0.75

	// Move to ISA and crystallised pot
	person.TaxFreeSavings += taxFree
	person.CrystallisedPot += taxable
	person.UncrystallisedPot = 0
	person.PCLSTaken = true

	return CrystallisationResult{
		AmountCrystallised: amount,
		TaxFreePortion:     taxFree,
		TaxablePortion:     taxable,
	}
}

// GradualCrystallise crystallises just enough pension to meet the needed amount
// Returns the tax-free and taxable portions withdrawn
// If PCLSTaken is true, all crystallised amount is taxable (no 25% tax-free)
func GradualCrystallise(person *Person, amountNeeded float64) CrystallisationResult {
	if amountNeeded <= 0 || person.UncrystallisedPot <= 0 {
		return CrystallisationResult{}
	}

	crystalliseAmount := math.Min(person.UncrystallisedPot, amountNeeded)

	var taxFree, taxable float64
	if person.PCLSTaken {
		// PCLS already taken - all crystallisation is taxable
		taxFree = 0
		taxable = crystalliseAmount
	} else {
		// Normal gradual crystallisation - 25% tax-free, 75% taxable
		taxFree = crystalliseAmount * 0.25
		taxable = crystalliseAmount * 0.75
	}

	person.UncrystallisedPot -= crystalliseAmount

	return CrystallisationResult{
		AmountCrystallised: crystalliseAmount,
		TaxFreePortion:     taxFree,
		TaxablePortion:     taxable,
	}
}

// UFPLSWithdraw withdraws directly from uncrystallised pot using UFPLS rules
// Each withdrawal is 25% tax-free and 75% taxable, without formally crystallising
// The pot remains uncrystallised and retains full 25% tax-free entitlement for future withdrawals
// This is more flexible than PCLS as it preserves the 25% entitlement on remaining funds
func UFPLSWithdraw(person *Person, amountNeeded float64) CrystallisationResult {
	if amountNeeded <= 0 || person.UncrystallisedPot <= 0 {
		return CrystallisationResult{}
	}

	// Withdraw directly from uncrystallised pot
	withdrawAmount := math.Min(person.UncrystallisedPot, amountNeeded)

	// UFPLS: always 25% tax-free, 75% taxable on each withdrawal
	// Unlike PCLS, this doesn't affect future withdrawals - each withdrawal gets 25% tax-free
	taxFree := withdrawAmount * 0.25
	taxable := withdrawAmount * 0.75

	person.UncrystallisedPot -= withdrawAmount

	return CrystallisationResult{
		AmountCrystallised: withdrawAmount,
		TaxFreePortion:     taxFree,
		TaxablePortion:     taxable,
	}
}

// WithdrawFromISA withdraws from a person's ISA (tax-free savings)
// Respects emergency fund minimum - will not reduce ISA below the minimum threshold
func WithdrawFromISA(person *Person, amount float64) float64 {
	if amount <= 0 {
		return 0
	}

	// Calculate available ISA after preserving emergency fund
	available := person.AvailableISA()
	if available <= 0 {
		return 0
	}

	withdrawal := math.Min(amount, available)
	person.TaxFreeSavings -= withdrawal
	return withdrawal
}

// WithdrawFromCrystallised withdraws from a person's crystallised pot (taxable)
func WithdrawFromCrystallised(person *Person, amount float64) float64 {
	if amount <= 0 || person.CrystallisedPot <= 0 {
		return 0
	}

	withdrawal := math.Min(amount, person.CrystallisedPot)
	person.CrystallisedPot -= withdrawal
	return withdrawal
}

// ApplyGrowth applies growth rates to a person's assets
func ApplyGrowth(person *Person, savingsRate, pensionRate float64) {
	person.TaxFreeSavings *= (1 + savingsRate)
	person.CrystallisedPot *= (1 + pensionRate)
	person.UncrystallisedPot *= (1 + pensionRate)
}

// GetGrowthRateForYear calculates linearly declining growth rate based on age
// Returns endRate if currentAge >= targetAge, startRate if currentAge <= startAge
// Otherwise linearly interpolates between startRate and endRate
func GetGrowthRateForYear(startRate, endRate float64, startAge, currentAge, targetAge int) float64 {
	if currentAge >= targetAge {
		return endRate
	}
	if currentAge <= startAge {
		return startRate
	}
	// Linear interpolation
	progress := float64(currentAge-startAge) / float64(targetAge-startAge)
	return startRate + (endRate-startRate)*progress
}

// ProportionalSplit calculates proportional withdrawal amounts from two people
func ProportionalSplit(totalNeeded float64, person1Available, person2Available float64) (p1Amount, p2Amount float64) {
	totalAvailable := person1Available + person2Available
	if totalAvailable <= 0 {
		return 0, 0
	}

	if totalNeeded >= totalAvailable {
		return person1Available, person2Available
	}

	p1Ratio := person1Available / totalAvailable
	p2Ratio := person2Available / totalAvailable

	p1Amount = totalNeeded * p1Ratio
	p2Amount = totalNeeded * p2Ratio

	return p1Amount, p2Amount
}

// ExecuteDrawdown executes a full drawdown for a given year
// For SavingsFirst: ISAs -> Crystallised Pension -> Crystallise more
// For PensionFirst: Crystallised Pension -> Crystallise more -> ISAs
// For TaxOptimized: Uses optimal mix to minimize tax burden
// For PensionToISA: Over-draw pension to fill tax bands, excess to ISA
// For PensionOnly: Only use pension, never touch ISAs (for pension-only depletion)
// For FillBasicRate: Withdraw pension up to basic rate limit, excess to ISA
// For StatePensionBridge: Draw heavily before state pension, reduce after
// netNeeded is the after-tax amount required - taxable withdrawals are grossed up
func ExecuteDrawdown(people []*Person, netNeeded float64, params SimulationParams, year int, statePensionByPerson map[string]float64, taxBands []TaxBand) WithdrawalBreakdown {
	breakdown := NewWithdrawalBreakdown()
	remaining := netNeeded

	if params.DrawdownOrder == TaxOptimized {
		// Use optimized withdrawal strategy
		return ExecuteOptimizedDrawdown(people, netNeeded, params.CrystallisationStrategy, year, statePensionByPerson, taxBands)
	} else if params.DrawdownOrder == PensionToISA {
		// Over-draw pension to fill tax bands, excess goes to ISA (only when income needed)
		return ExecutePensionToISADrawdown(people, netNeeded, params.CrystallisationStrategy, year, statePensionByPerson, taxBands, params.MaximizeCoupleISA)
	} else if params.DrawdownOrder == PensionToISAProactive {
		// Extract pension to fill tax bands even when work income covers expenses
		return ExecutePensionToISAProactiveDrawdown(people, netNeeded, params.CrystallisationStrategy, year, statePensionByPerson, taxBands, params.MaximizeCoupleISA)
	} else if params.DrawdownOrder == PensionOnly {
		// Only use pension, never touch ISAs
		withdrawFromPensionGrossedUp(people, remaining, params.CrystallisationStrategy, year, &breakdown, statePensionByPerson, taxBands)
		return breakdown
	} else if params.DrawdownOrder == FillBasicRate {
		// Fill basic rate band from pension, excess to ISA (controlled version of PensionToISA)
		return ExecuteFillBasicRateDrawdown(people, netNeeded, params.CrystallisationStrategy, year, statePensionByPerson, taxBands)
	} else if params.DrawdownOrder == StatePensionBridge {
		// Draw heavily before state pension, reduce after
		return ExecuteStatePensionBridgeDrawdown(people, netNeeded, params.CrystallisationStrategy, year, statePensionByPerson, taxBands)
	} else if params.DrawdownOrder == SavingsFirst {
		// Order: ISAs first, then pension
		remaining = withdrawFromISAs(people, remaining, &breakdown)
		remaining = withdrawFromPensionGrossedUp(people, remaining, params.CrystallisationStrategy, year, &breakdown, statePensionByPerson, taxBands)
	} else {
		// Order: Pension first, save ISAs for last
		remaining = withdrawFromPensionGrossedUp(people, remaining, params.CrystallisationStrategy, year, &breakdown, statePensionByPerson, taxBands)
		remaining = withdrawFromISAs(people, remaining, &breakdown)
	}

	return breakdown
}

// ExecuteOptimizedDrawdown uses the optimizer to minimize tax burden
func ExecuteOptimizedDrawdown(people []*Person, netNeeded float64, strategy Strategy, year int, statePensionByPerson map[string]float64, taxBands []TaxBand) WithdrawalBreakdown {
	breakdown := NewWithdrawalBreakdown()

	if netNeeded <= 0 {
		return breakdown
	}

	// Get the optimized plan
	plan := CalculateOptimizedWithdrawals(people, netNeeded, year, statePensionByPerson, taxBands, strategy)

	// Execute the plan by updating actual person balances
	for _, p := range people {
		// Withdraw from ISA
		if isaAmount, ok := plan.TaxFreeFromISA[p.Name]; ok && isaAmount > 0 {
			actual := WithdrawFromISA(p, isaAmount)
			breakdown.TaxFreeFromISA[p.Name] = actual
			breakdown.TotalTaxFree += actual
		}

		// Handle pension withdrawals
		taxableAmount := plan.TaxableFromPension[p.Name]
		taxFreeFromPension := plan.TaxFreeFromPension[p.Name]

		// Tax-free from pension is from gradual crystallisation or UFPLS (25%)
		// When crystallising/UFPLS: 25% is tax-free, 75% is taxable
		if taxFreeFromPension > 0 && (strategy == GradualCrystallisation || strategy == UFPLSStrategy) {
			// The amount crystallised = taxFreeFromPension / 0.25 = taxFreeFromPension * 4
			amountCrystallised := taxFreeFromPension * 4
			taxablePortion := amountCrystallised * 0.75

			// Actually withdraw from uncrystallised pot
			if p.UncrystallisedPot >= amountCrystallised {
				p.UncrystallisedPot -= amountCrystallised
				// 25% goes directly to breakdown as tax-free withdrawal
				breakdown.TaxFreeFromPension[p.Name] = taxFreeFromPension
				breakdown.TotalTaxFree += taxFreeFromPension

				// 75% is taxable
				// The taxableAmount from plan already includes this, so just track it
				breakdown.TaxableFromPension[p.Name] += taxablePortion
				breakdown.TotalTaxable += taxablePortion

				// Reduce the remaining taxable amount to withdraw from crystallised pot
				taxableAmount -= taxablePortion
			}
		}

		// Taxable from crystallised pot (already crystallised funds)
		if taxableAmount > 0.01 {
			actual := WithdrawFromCrystallised(p, taxableAmount)
			breakdown.TaxableFromPension[p.Name] += actual
			breakdown.TotalTaxable += actual
		}
	}

	return breakdown
}

// withdrawFromISAs withdraws from ISAs proportionally
// Respects emergency fund minimum for each person
func withdrawFromISAs(people []*Person, remaining float64, breakdown *WithdrawalBreakdown) float64 {
	if remaining <= 0 {
		return 0
	}

	// Calculate total available ISA (after emergency fund preservation)
	totalAvailableISA := 0.0
	for _, p := range people {
		totalAvailableISA += p.AvailableISA()
	}

	if totalAvailableISA > 0 {
		isaNeeded := math.Min(remaining, totalAvailableISA)
		for _, p := range people {
			available := p.AvailableISA()
			if available > 0 {
				share := available / totalAvailableISA
				withdrawal := math.Min(isaNeeded*share, available)
				actual := WithdrawFromISA(p, withdrawal)
				breakdown.TaxFreeFromISA[p.Name] += actual
				breakdown.TotalTaxFree += actual
				remaining -= actual
			}
		}
	}

	return remaining
}

// withdrawFromPensionGrossedUp withdraws from pension with grossing up for tax
// This ensures the net amount received after tax equals the remaining needed
func withdrawFromPensionGrossedUp(people []*Person, remaining float64, strategy Strategy, year int, breakdown *WithdrawalBreakdown, statePensionByPerson map[string]float64, taxBands []TaxBand) float64 {
	if remaining <= 0 {
		return 0
	}

	// For UFPLS strategy, skip crystallised pot and go straight to uncrystallised
	if strategy == UFPLSStrategy {
		return withdrawFromPensionUFPLS(people, remaining, year, breakdown, statePensionByPerson, taxBands)
	}

	// First: withdraw from already crystallised pots (taxable - need to gross up)
	// Keep looping until remaining is covered or all crystallised funds exhausted
	for remaining > 1 {
		madeProgress := false
		for _, p := range people {
			if p.CanAccessPension(year) && p.CrystallisedPot > 0 && remaining > 1 {
				// Get existing taxable income (state pension + any previous taxable withdrawals)
				existingTaxable := statePensionByPerson[p.Name] + breakdown.TaxableFromPension[p.Name]

				// Calculate gross amount needed to get net amount after tax
				grossNeeded, _ := GrossUpForTax(remaining, existingTaxable, taxBands)

				// Cap at available funds
				withdrawal := math.Min(grossNeeded, p.CrystallisedPot)
				if withdrawal < 1 {
					continue
				}
				actual := WithdrawFromCrystallised(p, withdrawal)

				// Calculate net received after tax on this withdrawal
				taxOnWithdrawal := CalculateMarginalTax(actual, existingTaxable, taxBands)
				netReceived := actual - taxOnWithdrawal

				breakdown.TaxableFromPension[p.Name] += actual
				breakdown.TotalTaxable += actual
				remaining -= netReceived
				madeProgress = true
			}
		}
		if !madeProgress {
			break
		}
	}

	// Second: crystallise more pension (gradual strategy only)
	// Keep looping until remaining is covered or all uncrystallised funds exhausted
	if remaining > 1 && strategy == GradualCrystallisation {
		for remaining > 1 {
			madeProgress := false
			for _, p := range people {
				if p.CanAccessPension(year) && p.UncrystallisedPot > 0 && remaining > 1 {
					// Get existing taxable income
					existingTaxable := statePensionByPerson[p.Name] + breakdown.TaxableFromPension[p.Name]

					// Iteratively find how much to crystallise to get 'remaining' net
					// 25% is tax-free, 75% is taxable
					toGet := remaining * 1.3 // Initial estimate
					for i := 0; i < 20; i++ {
						if toGet > p.UncrystallisedPot {
							toGet = p.UncrystallisedPot
						}
						taxFree := toGet * 0.25
						taxableGross := toGet * 0.75
						taxOnTaxable := CalculateMarginalTax(taxableGross, existingTaxable, taxBands)
						taxableNet := taxableGross - taxOnTaxable
						totalNet := taxFree + taxableNet

						if math.Abs(totalNet-remaining) < 1 || toGet >= p.UncrystallisedPot {
							break
						}
						ratio := remaining / totalNet
						toGet = toGet * ratio
					}

					if toGet > p.UncrystallisedPot {
						toGet = p.UncrystallisedPot
					}
					if toGet < 1 {
						continue
					}

					result := GradualCrystallise(p, toGet)

					// Tax-free portion counts fully toward remaining
					breakdown.TaxFreeFromPension[p.Name] += result.TaxFreePortion
					breakdown.TotalTaxFree += result.TaxFreePortion
					remaining -= result.TaxFreePortion

					// Taxable portion - calculate net after tax
					taxOnTaxable := CalculateMarginalTax(result.TaxablePortion, existingTaxable, taxBands)
					netFromTaxable := result.TaxablePortion - taxOnTaxable

					breakdown.TaxableFromPension[p.Name] += result.TaxablePortion
					breakdown.TotalTaxable += result.TaxablePortion
					remaining -= netFromTaxable
					madeProgress = true
				}
			}
			if !madeProgress {
				break
			}
		}
	}

	return remaining
}

// withdrawFromPensionUFPLS withdraws using UFPLS rules (each withdrawal 25% tax-free, 75% taxable)
// Unlike crystallisation, this preserves the 25% tax-free entitlement on remaining funds
func withdrawFromPensionUFPLS(people []*Person, remaining float64, year int, breakdown *WithdrawalBreakdown, statePensionByPerson map[string]float64, taxBands []TaxBand) float64 {
	if remaining <= 0 {
		return 0
	}

	// With UFPLS, we withdraw directly from uncrystallised pot
	// Each withdrawal is 25% tax-free, 75% taxable
	for remaining > 1 {
		madeProgress := false
		for _, p := range people {
			if p.CanAccessPension(year) && p.UncrystallisedPot > 0 && remaining > 1 {
				// Get existing taxable income
				existingTaxable := statePensionByPerson[p.Name] + breakdown.TaxableFromPension[p.Name]

				// Iteratively find how much to withdraw to get 'remaining' net
				// 25% is tax-free, 75% is taxable
				toGet := remaining * 1.3 // Initial estimate
				for i := 0; i < 20; i++ {
					if toGet > p.UncrystallisedPot {
						toGet = p.UncrystallisedPot
					}
					taxFree := toGet * 0.25
					taxableGross := toGet * 0.75
					taxOnTaxable := CalculateMarginalTax(taxableGross, existingTaxable, taxBands)
					taxableNet := taxableGross - taxOnTaxable
					totalNet := taxFree + taxableNet

					if math.Abs(totalNet-remaining) < 1 || toGet >= p.UncrystallisedPot {
						break
					}
					ratio := remaining / totalNet
					toGet = toGet * ratio
				}

				if toGet > p.UncrystallisedPot {
					toGet = p.UncrystallisedPot
				}
				if toGet < 1 {
					continue
				}

				result := UFPLSWithdraw(p, toGet)

				// Tax-free portion counts fully toward remaining
				breakdown.TaxFreeFromPension[p.Name] += result.TaxFreePortion
				breakdown.TotalTaxFree += result.TaxFreePortion
				remaining -= result.TaxFreePortion

				// Taxable portion - calculate net after tax
				taxOnTaxable := CalculateMarginalTax(result.TaxablePortion, existingTaxable, taxBands)
				netFromTaxable := result.TaxablePortion - taxOnTaxable

				breakdown.TaxableFromPension[p.Name] += result.TaxablePortion
				breakdown.TotalTaxable += result.TaxablePortion
				remaining -= netFromTaxable
				madeProgress = true
			}
		}
		if !madeProgress {
			break
		}
	}

	// Also try crystallised pot if available (from previous PCLS or other crystallisation)
	for remaining > 1 {
		madeProgress := false
		for _, p := range people {
			if p.CanAccessPension(year) && p.CrystallisedPot > 0 && remaining > 1 {
				existingTaxable := statePensionByPerson[p.Name] + breakdown.TaxableFromPension[p.Name]
				grossNeeded, _ := GrossUpForTax(remaining, existingTaxable, taxBands)
				withdrawal := math.Min(grossNeeded, p.CrystallisedPot)
				if withdrawal < 1 {
					continue
				}
				actual := WithdrawFromCrystallised(p, withdrawal)
				taxOnWithdrawal := CalculateMarginalTax(actual, existingTaxable, taxBands)
				netReceived := actual - taxOnWithdrawal

				breakdown.TaxableFromPension[p.Name] += actual
				breakdown.TotalTaxable += actual
				remaining -= netReceived
				madeProgress = true
			}
		}
		if !madeProgress {
			break
		}
	}

	return remaining
}

// ExecutePensionToISADrawdown over-draws from pension to fill tax bands
// Any excess beyond what's needed for spending is deposited into ISA (up to per-person annual limit)
// Strategy: Fill personal allowance + basic rate band from pension, excess to ISA
// If maximizeCoupleISA is true, will withdraw extra from one person's pension to fill both ISA allowances
func ExecutePensionToISADrawdown(people []*Person, netNeeded float64, strategy Strategy, year int, statePensionByPerson map[string]float64, taxBands []TaxBand, maximizeCoupleISA bool) WithdrawalBreakdown {
	breakdown := NewWithdrawalBreakdown()

	if netNeeded <= 0 {
		return breakdown
	}

	// Get personal allowance and basic rate limit from (inflated) tax bands
	personalAllowance := 12570.0 // Default fallback
	basicRateLimit := 50270.0    // Default fallback

	// Extract actual values from tax bands (which are inflated for the current year)
	if len(taxBands) > 0 && taxBands[0].Rate == 0 {
		personalAllowance = taxBands[0].Upper
	}
	if len(taxBands) > 1 && taxBands[1].Rate == 0.20 {
		basicRateLimit = taxBands[1].Upper
	}

	// Calculate total ISA allowance across all people
	totalISAAllowance := 0.0
	for _, p := range people {
		totalISAAllowance += p.ISAAnnualLimit
	}

	// For each person, calculate how much pension to withdraw to fill tax bands
	for _, p := range people {
		if !p.CanAccessPension(year) {
			continue
		}

		statePension := statePensionByPerson[p.Name]

		// Calculate space available in each band
		personalAllowanceSpace := math.Max(0, personalAllowance-statePension)
		basicRateSpace := math.Max(0, basicRateLimit-math.Max(statePension, personalAllowance))

		// Target: fill personal allowance at minimum, optionally fill basic rate too
		// We'll fill up to basic rate limit to maximize pension drawdown into ISA
		targetTaxableWithdrawal := personalAllowanceSpace + basicRateSpace

		if targetTaxableWithdrawal <= 0 {
			continue
		}

		// Calculate how much we need to crystallise/withdraw to get this taxable amount
		// When crystallising: 25% tax-free, 75% taxable (unless PCLSTaken)
		// To get X taxable from crystallisation, we need to crystallise X/0.75 (or just X if PCLSTaken)
		var amountToCrystallise float64
		if p.PCLSTaken {
			// PCLS already taken - all crystallisation is taxable
			amountToCrystallise = targetTaxableWithdrawal
		} else {
			// Normal - 25% tax-free, 75% taxable
			amountToCrystallise = targetTaxableWithdrawal / 0.75
		}

		if amountToCrystallise > p.UncrystallisedPot {
			amountToCrystallise = p.UncrystallisedPot
		}

		if amountToCrystallise > 0 {
			result := GradualCrystallise(p, amountToCrystallise)
			breakdown.TaxFreeFromPension[p.Name] += result.TaxFreePortion
			breakdown.TotalTaxFree += result.TaxFreePortion
			breakdown.TaxableFromPension[p.Name] += result.TaxablePortion
			breakdown.TotalTaxable += result.TaxablePortion
		}

		// Also withdraw from crystallised pot if available
		if p.CrystallisedPot > 0 && breakdown.TaxableFromPension[p.Name] < targetTaxableWithdrawal {
			needed := targetTaxableWithdrawal - breakdown.TaxableFromPension[p.Name]
			withdrawal := math.Min(needed, p.CrystallisedPot)
			if withdrawal > 0 {
				actual := WithdrawFromCrystallised(p, withdrawal)
				breakdown.TaxableFromPension[p.Name] += actual
				breakdown.TotalTaxable += actual
			}
		}
	}

	// Calculate current gross withdrawn and tax paid
	totalGrossWithdrawn := breakdown.TotalTaxFree + breakdown.TotalTaxable
	totalTaxPaid := 0.0
	for _, p := range people {
		statePension := statePensionByPerson[p.Name]
		taxableWithdrawal := breakdown.TaxableFromPension[p.Name]
		totalIncome := statePension + taxableWithdrawal
		totalTaxPaid += CalculateTaxOnIncome(totalIncome, taxBands)
	}

	// Net income available = gross withdrawn - tax paid
	netFromPension := totalGrossWithdrawn - totalTaxPaid

	// Calculate current excess (what would go to ISA)
	currentExcess := netFromPension - netNeeded

	// If maximizeCoupleISA is enabled and we haven't filled all ISA allowances,
	// withdraw additional from whoever can access their pension
	if maximizeCoupleISA && currentExcess < totalISAAllowance && currentExcess >= 0 {
		// How much more net income do we need to fill all ISA allowances?
		additionalNetNeeded := totalISAAllowance - currentExcess

		// We need to gross this up for higher rate tax (40%)
		// Net = Gross - Tax, where Tax = 0.40 * (Gross - tax already used bands)
		// For simplicity, assume additional withdrawals are in higher rate (40%)
		// Net = Gross * 0.60, so Gross = Net / 0.60
		// But we also get 25% PCLS if not taken, so effective is better
		// For a rough estimate: if PCLS available, gross needed = net / 0.85 (25% tax-free + 75% taxed at ~40%)
		// Actually: Gross = PCLS (25%) + Taxable (75%)
		// Net from Gross = 0.25*Gross + 0.75*Gross*(1-0.40) = 0.25*Gross + 0.45*Gross = 0.70*Gross
		// So Gross = Net / 0.70 when in higher rate with PCLS available

		for _, p := range people {
			if !p.CanAccessPension(year) || additionalNetNeeded <= 0 {
				continue
			}

			// Check if this person has pension to withdraw
			availablePension := p.UncrystallisedPot + p.CrystallisedPot
			if availablePension <= 0 {
				continue
			}

			// Calculate gross needed to generate the additional net
			var grossMultiplier float64
			if p.PCLSTaken {
				// No PCLS, all taxable at 40%: Net = Gross * 0.60
				grossMultiplier = 1.0 / 0.60
			} else {
				// With PCLS: Net = 0.25*Gross + 0.75*Gross*0.60 = 0.70*Gross
				grossMultiplier = 1.0 / 0.70
			}

			grossNeeded := additionalNetNeeded * grossMultiplier

			// First try uncrystallised pot
			if p.UncrystallisedPot > 0 {
				toCrystallise := math.Min(grossNeeded, p.UncrystallisedPot)
				if toCrystallise > 0 {
					result := GradualCrystallise(p, toCrystallise)
					breakdown.TaxFreeFromPension[p.Name] += result.TaxFreePortion
					breakdown.TotalTaxFree += result.TaxFreePortion
					breakdown.TaxableFromPension[p.Name] += result.TaxablePortion
					breakdown.TotalTaxable += result.TaxablePortion
					grossNeeded -= toCrystallise
				}
			}

			// Then try crystallised pot
			if grossNeeded > 0 && p.CrystallisedPot > 0 {
				toWithdraw := math.Min(grossNeeded, p.CrystallisedPot)
				if toWithdraw > 0 {
					actual := WithdrawFromCrystallised(p, toWithdraw)
					breakdown.TaxableFromPension[p.Name] += actual
					breakdown.TotalTaxable += actual
				}
			}

			// Recalculate how much additional net we still need
			// (Simplified - assume we got what we needed from this person)
			additionalNetNeeded = 0
		}

		// Recalculate totals after additional withdrawals
		totalGrossWithdrawn = breakdown.TotalTaxFree + breakdown.TotalTaxable
		totalTaxPaid = 0.0
		for _, p := range people {
			statePension := statePensionByPerson[p.Name]
			taxableWithdrawal := breakdown.TaxableFromPension[p.Name]
			totalIncome := statePension + taxableWithdrawal
			totalTaxPaid += CalculateTaxOnIncome(totalIncome, taxBands)
		}
		netFromPension = totalGrossWithdrawn - totalTaxPaid
	}

	// If we have more net than needed, excess goes to ISA (up to annual limit per person)
	excess := netFromPension - netNeeded
	if excess > 0 {
		// Distribute excess equally to ALL people's ISAs (not just those who withdrew)
		// Each person can receive up to their annual ISA limit
		remainingExcess := excess
		for _, p := range people {
			if remainingExcess <= 0 {
				break
			}
			isaDeposit := remainingExcess / float64(len(people))
			// Cap at per-person annual ISA limit
			if isaDeposit > p.ISAAnnualLimit {
				isaDeposit = p.ISAAnnualLimit
			}
			p.TaxFreeSavings += isaDeposit
			breakdown.ISADeposits[p.Name] = isaDeposit
			breakdown.TotalISADeposits += isaDeposit
			remainingExcess -= isaDeposit
		}
		// If there's still excess after first pass (someone hit their limit), give remainder to others
		for _, p := range people {
			if remainingExcess <= 0 {
				break
			}
			spaceLeft := p.ISAAnnualLimit - breakdown.ISADeposits[p.Name]
			if spaceLeft > 0 {
				additional := math.Min(remainingExcess, spaceLeft)
				p.TaxFreeSavings += additional
				breakdown.ISADeposits[p.Name] += additional
				breakdown.TotalISADeposits += additional
				remainingExcess -= additional
			}
		}
	} else if netFromPension < netNeeded {
		// Need more - try ISA first, then more pension if needed
		shortfall := netNeeded - netFromPension

		// First: try to cover from ISA
		totalISA := 0.0
		for _, p := range people {
			totalISA += p.TaxFreeSavings
		}

		if totalISA > 0 {
			isaNeeded := math.Min(shortfall, totalISA)
			for _, p := range people {
				if p.TaxFreeSavings > 0 {
					share := p.TaxFreeSavings / totalISA
					withdrawal := math.Min(isaNeeded*share, p.TaxFreeSavings)
					actual := WithdrawFromISA(p, withdrawal)
					breakdown.TaxFreeFromISA[p.Name] += actual
					breakdown.TotalTaxFree += actual
					shortfall -= actual
				}
			}
		}

		// Second: if still short, draw more from pension (even at higher tax rates)
		if shortfall > 1 {
			_ = withdrawFromPensionGrossedUp(people, shortfall, strategy, year,
				&breakdown, statePensionByPerson, taxBands)
		}
	}

	return breakdown
}

// ExecutePensionToISAProactiveDrawdown extracts pension to fill tax bands even when work income covers expenses
// This is useful for maximizing tax-efficient pension-to-ISA transfers while still employed
// Key difference from ExecutePensionToISADrawdown: works even when netNeeded <= 0
func ExecutePensionToISAProactiveDrawdown(people []*Person, netNeeded float64, strategy Strategy, year int, taxableIncomeByPerson map[string]float64, taxBands []TaxBand, maximizeCoupleISA bool) WithdrawalBreakdown {
	breakdown := NewWithdrawalBreakdown()

	// Get personal allowance and basic rate limit from (inflated) tax bands
	personalAllowance := 12570.0 // Default fallback
	basicRateLimit := 50270.0    // Default fallback

	if len(taxBands) > 0 && taxBands[0].Rate == 0 {
		personalAllowance = taxBands[0].Upper
	}
	if len(taxBands) > 1 && taxBands[1].Rate == 0.20 {
		basicRateLimit = taxBands[1].Upper
	}

	// For each person, calculate how much pension to withdraw to fill remaining tax band space
	for _, p := range people {
		if !p.CanAccessPension(year) {
			continue
		}

		// Get existing taxable income (includes state pension, DB pension, work income, part-time)
		existingTaxableIncome := taxableIncomeByPerson[p.Name]

		// Calculate space available in each band after existing income
		personalAllowanceSpace := math.Max(0, personalAllowance-existingTaxableIncome)
		basicRateSpace := math.Max(0, basicRateLimit-math.Max(existingTaxableIncome, personalAllowance))

		// Target: fill personal allowance and basic rate band
		targetTaxableWithdrawal := personalAllowanceSpace + basicRateSpace

		// Also check if we have ISA allowance space - no point extracting if we can't deposit
		if p.ISAAnnualLimit <= 0 {
			continue
		}

		if targetTaxableWithdrawal <= 0 {
			continue
		}

		// Calculate how much we need to crystallise/withdraw to get this taxable amount
		var amountToCrystallise float64
		if p.PCLSTaken {
			amountToCrystallise = targetTaxableWithdrawal
		} else {
			// 25% tax-free, 75% taxable
			amountToCrystallise = targetTaxableWithdrawal / 0.75
		}

		if amountToCrystallise > p.UncrystallisedPot {
			amountToCrystallise = p.UncrystallisedPot
		}

		if amountToCrystallise > 0 {
			result := GradualCrystallise(p, amountToCrystallise)
			breakdown.TaxFreeFromPension[p.Name] += result.TaxFreePortion
			breakdown.TotalTaxFree += result.TaxFreePortion
			breakdown.TaxableFromPension[p.Name] += result.TaxablePortion
			breakdown.TotalTaxable += result.TaxablePortion
		}

		// Also withdraw from crystallised pot if available
		if p.CrystallisedPot > 0 && breakdown.TaxableFromPension[p.Name] < targetTaxableWithdrawal {
			needed := targetTaxableWithdrawal - breakdown.TaxableFromPension[p.Name]
			withdrawal := math.Min(needed, p.CrystallisedPot)
			if withdrawal > 0 {
				actual := WithdrawFromCrystallised(p, withdrawal)
				breakdown.TaxableFromPension[p.Name] += actual
				breakdown.TotalTaxable += actual
			}
		}
	}

	// Calculate gross withdrawn and tax paid
	totalGrossWithdrawn := breakdown.TotalTaxFree + breakdown.TotalTaxable
	totalTaxPaid := 0.0
	for _, p := range people {
		existingTaxable := taxableIncomeByPerson[p.Name]
		taxableWithdrawal := breakdown.TaxableFromPension[p.Name]
		totalIncome := existingTaxable + taxableWithdrawal
		// Tax on combined income minus tax already paid on existing income
		totalTaxPaid += CalculateTaxOnIncome(totalIncome, taxBands) - CalculateTaxOnIncome(existingTaxable, taxBands)
	}

	// Net from pension = gross - additional tax
	netFromPension := totalGrossWithdrawn - totalTaxPaid

	// Calculate excess to deposit to ISA
	// If netNeeded > 0, excess = net - needed
	// If netNeeded <= 0, all of net goes to ISA (we didn't need income)
	var excess float64
	if netNeeded > 0 {
		excess = netFromPension - netNeeded
	} else {
		excess = netFromPension
	}

	if excess > 0 {
		// Distribute excess to ISAs (up to annual limit per person)
		remainingExcess := excess
		for _, p := range people {
			if remainingExcess <= 0 {
				break
			}
			isaDeposit := remainingExcess / float64(len(people))
			if isaDeposit > p.ISAAnnualLimit {
				isaDeposit = p.ISAAnnualLimit
			}
			p.TaxFreeSavings += isaDeposit
			breakdown.ISADeposits[p.Name] = isaDeposit
			breakdown.TotalISADeposits += isaDeposit
			remainingExcess -= isaDeposit
		}
		// Second pass for remaining excess
		for _, p := range people {
			if remainingExcess <= 0 {
				break
			}
			spaceLeft := p.ISAAnnualLimit - breakdown.ISADeposits[p.Name]
			if spaceLeft > 0 {
				additional := math.Min(remainingExcess, spaceLeft)
				p.TaxFreeSavings += additional
				breakdown.ISADeposits[p.Name] += additional
				breakdown.TotalISADeposits += additional
				remainingExcess -= additional
			}
		}
	}

	// If we still need income (netNeeded > 0 and excess < 0), cover shortfall from ISA
	if netNeeded > 0 && netFromPension < netNeeded {
		shortfall := netNeeded - netFromPension

		totalISA := 0.0
		for _, p := range people {
			totalISA += p.TaxFreeSavings
		}

		if totalISA > 0 {
			isaNeeded := math.Min(shortfall, totalISA)
			for _, p := range people {
				if p.TaxFreeSavings > 0 {
					share := p.TaxFreeSavings / totalISA
					withdrawal := math.Min(isaNeeded*share, p.TaxFreeSavings)
					actual := WithdrawFromISA(p, withdrawal)
					breakdown.TaxFreeFromISA[p.Name] += actual
					breakdown.TotalTaxFree += actual
					shortfall -= actual
				}
			}
		}

		// If still short, draw more from pension
		if shortfall > 1 {
			_ = withdrawFromPensionGrossedUp(people, shortfall, strategy, year,
				&breakdown, taxableIncomeByPerson, taxBands)
		}
	}

	return breakdown
}

// ExecuteFillBasicRateDrawdown withdraws from pension to fill the basic rate band exactly
// This crystallizes tax at 20% now rather than potentially 40% later
// Any excess goes to ISA, shortfall is covered from ISA
func ExecuteFillBasicRateDrawdown(people []*Person, netNeeded float64, strategy Strategy, year int, statePensionByPerson map[string]float64, taxBands []TaxBand) WithdrawalBreakdown {
	breakdown := NewWithdrawalBreakdown()

	// Get personal allowance and basic rate limit from (inflated) tax bands
	personalAllowance := 12570.0 // Default fallback
	basicRateLimit := 50270.0    // Default fallback

	if len(taxBands) > 0 && taxBands[0].Rate == 0 {
		personalAllowance = taxBands[0].Upper
	}
	if len(taxBands) > 1 && taxBands[1].Rate == 0.20 {
		basicRateLimit = taxBands[1].Upper
	}

	// For each person, withdraw pension to fill exactly to basic rate limit (not beyond)
	for _, p := range people {
		if !p.CanAccessPension(year) {
			continue
		}

		statePension := statePensionByPerson[p.Name]

		// Calculate space available up to basic rate limit (not beyond)
		personalAllowanceSpace := math.Max(0, personalAllowance-statePension)
		basicRateSpace := math.Max(0, basicRateLimit-math.Max(statePension, personalAllowance))

		// Target: fill exactly to basic rate limit
		targetTaxableWithdrawal := personalAllowanceSpace + basicRateSpace

		if targetTaxableWithdrawal <= 0 {
			continue
		}

		// Calculate how much we need to crystallise to get this taxable amount
		var amountToCrystallise float64
		if p.PCLSTaken {
			amountToCrystallise = targetTaxableWithdrawal
		} else {
			// 25% tax-free, 75% taxable: to get X taxable, crystallise X/0.75
			amountToCrystallise = targetTaxableWithdrawal / 0.75
		}

		if amountToCrystallise > p.UncrystallisedPot {
			amountToCrystallise = p.UncrystallisedPot
		}

		if amountToCrystallise > 0 {
			var result CrystallisationResult
			if strategy == UFPLSStrategy {
				result = UFPLSWithdraw(p, amountToCrystallise)
			} else {
				result = GradualCrystallise(p, amountToCrystallise)
			}
			breakdown.TaxFreeFromPension[p.Name] += result.TaxFreePortion
			breakdown.TotalTaxFree += result.TaxFreePortion
			breakdown.TaxableFromPension[p.Name] += result.TaxablePortion
			breakdown.TotalTaxable += result.TaxablePortion
		}

		// Also withdraw from crystallised pot if available and haven't reached target
		if p.CrystallisedPot > 0 && breakdown.TaxableFromPension[p.Name] < targetTaxableWithdrawal {
			needed := targetTaxableWithdrawal - breakdown.TaxableFromPension[p.Name]
			withdrawal := math.Min(needed, p.CrystallisedPot)
			if withdrawal > 0 {
				actual := WithdrawFromCrystallised(p, withdrawal)
				breakdown.TaxableFromPension[p.Name] += actual
				breakdown.TotalTaxable += actual
			}
		}
	}

	// Calculate net income from pension withdrawals
	totalGrossWithdrawn := breakdown.TotalTaxFree + breakdown.TotalTaxable
	totalTaxPaid := 0.0
	for _, p := range people {
		statePension := statePensionByPerson[p.Name]
		taxableWithdrawal := breakdown.TaxableFromPension[p.Name]
		totalIncome := statePension + taxableWithdrawal
		totalTaxPaid += CalculateTaxOnIncome(totalIncome, taxBands)
	}
	netFromPension := totalGrossWithdrawn - totalTaxPaid

	// If excess, deposit to ISA
	excess := netFromPension - netNeeded
	if excess > 0 {
		for _, p := range people {
			if excess <= 0 {
				break
			}
			isaDeposit := math.Min(excess, p.ISAAnnualLimit)
			p.TaxFreeSavings += isaDeposit
			breakdown.ISADeposits[p.Name] = isaDeposit
			breakdown.TotalISADeposits += isaDeposit
			excess -= isaDeposit
		}
	} else if netFromPension < netNeeded {
		// Shortfall - cover from ISA first
		shortfall := netNeeded - netFromPension
		for _, p := range people {
			if shortfall <= 0 || p.TaxFreeSavings <= 0 {
				continue
			}
			withdrawal := math.Min(shortfall, p.TaxFreeSavings)
			actual := WithdrawFromISA(p, withdrawal)
			breakdown.TaxFreeFromISA[p.Name] += actual
			breakdown.TotalTaxFree += actual
			shortfall -= actual
		}

		// If still short, draw more from pension (at higher rates)
		if shortfall > 1 {
			_ = withdrawFromPensionGrossedUp(people, shortfall, strategy, year,
				&breakdown, statePensionByPerson, taxBands)
		}
	}

	return breakdown
}

// ExecuteStatePensionBridgeDrawdown draws heavily from private pension before state pension starts
// to maximize use of personal allowance, then reduces private pension withdrawals after
func ExecuteStatePensionBridgeDrawdown(people []*Person, netNeeded float64, strategy Strategy, year int, statePensionByPerson map[string]float64, taxBands []TaxBand) WithdrawalBreakdown {
	breakdown := NewWithdrawalBreakdown()

	if netNeeded <= 0 {
		return breakdown
	}

	// Get personal allowance from (inflated) tax bands
	personalAllowance := 12570.0 // Default fallback
	basicRateLimit := 50270.0    // Default fallback

	if len(taxBands) > 0 && taxBands[0].Rate == 0 {
		personalAllowance = taxBands[0].Upper
	}
	if len(taxBands) > 1 && taxBands[1].Rate == 0.20 {
		basicRateLimit = taxBands[1].Upper
	}

	// Check if any person is receiving state pension this year
	anyReceivingStatePension := false
	for _, p := range people {
		if statePensionByPerson[p.Name] > 0 {
			anyReceivingStatePension = true
			break
		}
	}

	if !anyReceivingStatePension {
		// BEFORE state pension: Draw heavily from private pension to use full personal allowance
		// Fill up to basic rate limit from pension (similar to FillBasicRate)
		for _, p := range people {
			if !p.CanAccessPension(year) {
				continue
			}

			// Target: fill personal allowance fully, plus basic rate band
			targetTaxableWithdrawal := personalAllowance + (basicRateLimit - personalAllowance)

			var amountToCrystallise float64
			if p.PCLSTaken {
				amountToCrystallise = targetTaxableWithdrawal
			} else {
				amountToCrystallise = targetTaxableWithdrawal / 0.75
			}

			if amountToCrystallise > p.UncrystallisedPot {
				amountToCrystallise = p.UncrystallisedPot
			}

			if amountToCrystallise > 0 {
				var result CrystallisationResult
				if strategy == UFPLSStrategy {
					result = UFPLSWithdraw(p, amountToCrystallise)
				} else {
					result = GradualCrystallise(p, amountToCrystallise)
				}
				breakdown.TaxFreeFromPension[p.Name] += result.TaxFreePortion
				breakdown.TotalTaxFree += result.TaxFreePortion
				breakdown.TaxableFromPension[p.Name] += result.TaxablePortion
				breakdown.TotalTaxable += result.TaxablePortion
			}

			// Draw from crystallised pot too
			if p.CrystallisedPot > 0 && breakdown.TaxableFromPension[p.Name] < targetTaxableWithdrawal {
				needed := targetTaxableWithdrawal - breakdown.TaxableFromPension[p.Name]
				withdrawal := math.Min(needed, p.CrystallisedPot)
				if withdrawal > 0 {
					actual := WithdrawFromCrystallised(p, withdrawal)
					breakdown.TaxableFromPension[p.Name] += actual
					breakdown.TotalTaxable += actual
				}
			}
		}
	} else {
		// AFTER state pension starts: Use pension first strategy (state pension uses some allowance)
		// Only withdraw what's needed, pension first then ISA
		remaining := netNeeded
		remaining = withdrawFromPensionGrossedUp(people, remaining, strategy, year, &breakdown, statePensionByPerson, taxBands)
		if remaining > 0 {
			remaining = withdrawFromISAs(people, remaining, &breakdown)
		}
		return breakdown
	}

	// Calculate net income from pension withdrawals (for pre-state-pension case)
	totalGrossWithdrawn := breakdown.TotalTaxFree + breakdown.TotalTaxable
	totalTaxPaid := 0.0
	for _, p := range people {
		statePension := statePensionByPerson[p.Name]
		taxableWithdrawal := breakdown.TaxableFromPension[p.Name]
		totalIncome := statePension + taxableWithdrawal
		totalTaxPaid += CalculateTaxOnIncome(totalIncome, taxBands)
	}
	netFromPension := totalGrossWithdrawn - totalTaxPaid

	// If excess, deposit to ISA (preserves for later when state pension takes allowance)
	excess := netFromPension - netNeeded
	if excess > 0 {
		for _, p := range people {
			if excess <= 0 {
				break
			}
			isaDeposit := math.Min(excess, p.ISAAnnualLimit)
			p.TaxFreeSavings += isaDeposit
			breakdown.ISADeposits[p.Name] = isaDeposit
			breakdown.TotalISADeposits += isaDeposit
			excess -= isaDeposit
		}
	} else if netFromPension < netNeeded {
		// Shortfall - cover from ISA
		shortfall := netNeeded - netFromPension
		for _, p := range people {
			if shortfall <= 0 || p.TaxFreeSavings <= 0 {
				continue
			}
			withdrawal := math.Min(shortfall, p.TaxFreeSavings)
			actual := WithdrawFromISA(p, withdrawal)
			breakdown.TaxFreeFromISA[p.Name] += actual
			breakdown.TotalTaxFree += actual
			shortfall -= actual
		}

		// If still short, draw more from pension
		if shortfall > 1 {
			_ = withdrawFromPensionGrossedUp(people, shortfall, strategy, year,
				&breakdown, statePensionByPerson, taxBands)
		}
	}

	return breakdown
}

// GetStrategiesForConfig returns the appropriate simulation strategies based on config
// If there's no mortgage, only returns strategies with MortgageNormal (no point testing different payoff options)
func GetStrategiesForConfig(config *Config) []SimulationParams {
	if !config.HasMortgage() {
		// No mortgage - only test drawdown order strategies (mortgage options are irrelevant)
		return []SimulationParams{
			// Gradual Crystallisation
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: SavingsFirst, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionFirst, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: TaxOptimized, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionToISA, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionToISAProactive, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: FillBasicRate, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: StatePensionBridge, MortgageOpt: MortgageNormal},
			// UFPLS
			{CrystallisationStrategy: UFPLSStrategy, DrawdownOrder: SavingsFirst, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: UFPLSStrategy, DrawdownOrder: PensionFirst, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: UFPLSStrategy, DrawdownOrder: TaxOptimized, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: UFPLSStrategy, DrawdownOrder: FillBasicRate, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: UFPLSStrategy, DrawdownOrder: StatePensionBridge, MortgageOpt: MortgageNormal},
		}
	}

	// Has mortgage - build strategies based on allowed mortgage options
	drawdownOrders := []DrawdownOrder{
		SavingsFirst, PensionFirst, TaxOptimized, PensionToISA,
		PensionToISAProactive, FillBasicRate, StatePensionBridge,
	}

	// Determine which mortgage options to include
	mortgageOpts := []MortgageOption{MortgageEarly, MortgageNormal, PCLSMortgagePayoff}
	if config.ShouldIncludeExtendedMortgage() {
		mortgageOpts = []MortgageOption{MortgageEarly, MortgageNormal, MortgageExtended, PCLSMortgagePayoff}
	}

	var strategies []SimulationParams

	// Gradual Crystallisation with all mortgage options
	for _, mortOpt := range mortgageOpts {
		for _, drawdown := range drawdownOrders {
			strategies = append(strategies, SimulationParams{
				CrystallisationStrategy: GradualCrystallisation,
				DrawdownOrder:           drawdown,
				MortgageOpt:             mortOpt,
			})
		}
	}

	// UFPLS with Normal mortgage only (to keep combinations manageable)
	ufplsDrawdowns := []DrawdownOrder{SavingsFirst, PensionFirst, TaxOptimized, FillBasicRate, StatePensionBridge}
	for _, drawdown := range ufplsDrawdowns {
		strategies = append(strategies, SimulationParams{
			CrystallisationStrategy: UFPLSStrategy,
			DrawdownOrder:           drawdown,
			MortgageOpt:             MortgageNormal,
		})
	}

	return strategies
}

// GetDepletionStrategiesForConfig returns strategies for depletion mode
// Similar to GetStrategiesForConfig but used in depletion calculations
func GetDepletionStrategiesForConfig(config *Config) []SimulationParams {
	return GetStrategiesForConfig(config)
}

// GetPensionOnlyStrategiesForConfig returns strategies for pension-only mode
func GetPensionOnlyStrategiesForConfig(config *Config) []SimulationParams {
	if !config.HasMortgage() {
		return []SimulationParams{
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionOnly, MortgageOpt: MortgageNormal},
		}
	}

	strategies := []SimulationParams{
		{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionOnly, MortgageOpt: MortgageEarly},
		{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionOnly, MortgageOpt: MortgageNormal},
		{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionOnly, MortgageOpt: PCLSMortgagePayoff},
	}
	if config.ShouldIncludeExtendedMortgage() {
		strategies = append(strategies,
			SimulationParams{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionOnly, MortgageOpt: MortgageExtended},
		)
	}
	return strategies
}

// GetPensionToISAStrategiesForConfig returns strategies for pension-to-ISA mode
func GetPensionToISAStrategiesForConfig(config *Config) []SimulationParams {
	if !config.HasMortgage() {
		return []SimulationParams{
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionToISA, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionToISAProactive, MortgageOpt: MortgageNormal},
		}
	}

	// Base mortgage options
	mortgageOpts := []MortgageOption{MortgageEarly, MortgageNormal, PCLSMortgagePayoff}
	if config.ShouldIncludeExtendedMortgage() {
		mortgageOpts = []MortgageOption{MortgageEarly, MortgageNormal, MortgageExtended, PCLSMortgagePayoff}
	}

	var strategies []SimulationParams
	for _, mortOpt := range mortgageOpts {
		// Standard PensionToISA (only extracts when income needed)
		strategies = append(strategies, SimulationParams{
			CrystallisationStrategy: GradualCrystallisation,
			DrawdownOrder:           PensionToISA,
			MortgageOpt:             mortOpt,
		})
		// Proactive PensionToISA (extracts even when work income covers expenses)
		strategies = append(strategies, SimulationParams{
			CrystallisationStrategy: GradualCrystallisation,
			DrawdownOrder:           PensionToISAProactive,
			MortgageOpt:             mortOpt,
		})
	}
	return strategies
}

// GetISAToSIPPStrategiesForConfig returns strategies with ISA to SIPP pre-retirement transfers enabled
// This is the reverse of PensionToISA: while working, move ISA money into pension to get tax relief
// The idea is to get tax relief at your marginal rate now, then withdraw later when potentially in lower bracket
// Note: Only works for DC pensions/SIPPs, not DB pensions like Teachers Pension
func GetISAToSIPPStrategiesForConfig(config *Config) []SimulationParams {
	drawdowns := []DrawdownOrder{TaxOptimized, PensionFirst, SavingsFirst, FillBasicRate, StatePensionBridge}

	if !config.HasMortgage() {
		var strategies []SimulationParams
		for _, drawdown := range drawdowns {
			strategies = append(strategies, SimulationParams{
				CrystallisationStrategy: GradualCrystallisation,
				DrawdownOrder:           drawdown,
				MortgageOpt:             MortgageNormal,
				ISAToSIPPEnabled:        true,
			})
		}
		return strategies
	}

	// With mortgage - include relevant mortgage options
	mortgageOpts := []MortgageOption{MortgageNormal, MortgageEarly}
	if config.ShouldIncludeExtendedMortgage() {
		mortgageOpts = append(mortgageOpts, MortgageExtended)
	}

	var strategies []SimulationParams
	for _, mortOpt := range mortgageOpts {
		for _, drawdown := range drawdowns {
			strategies = append(strategies, SimulationParams{
				CrystallisationStrategy: GradualCrystallisation,
				DrawdownOrder:           drawdown,
				MortgageOpt:             mortOpt,
				ISAToSIPPEnabled:        true,
			})
		}
	}
	return strategies
}

// HasISAToSIPPEnabled checks if any person in the config has ISA to SIPP transfers enabled
func HasISAToSIPPEnabled(config *Config) bool {
	for _, p := range config.People {
		if p.ISAToSIPPEnabled {
			return true
		}
	}
	return false
}

// GetAllStrategiesIncludingISAToSIPP returns all strategies including ISA to SIPP variants
// This is useful when you want to compare with and without ISA to SIPP
func GetAllStrategiesIncludingISAToSIPP(config *Config) []SimulationParams {
	strategies := GetStrategiesForConfig(config)
	isaToSIPPStrategies := GetISAToSIPPStrategiesForConfig(config)
	return append(strategies, isaToSIPPStrategies...)
}

// =============================================================================
// Strategy Permutation Engine V2
// =============================================================================

// Constraint represents a rule that invalidates certain combinations
type Constraint struct {
	ID          string
	Description string
	Validate    func(combo StrategyCombo) bool // Returns false if invalid
}

// DefaultConstraints returns the standard constraints for strategy combinations
func DefaultConstraints() []Constraint {
	return []Constraint{
		{
			ID:          "maximize_isa_only_pension_to_isa",
			Description: "MaximizeCoupleISA only applies to PensionToISA strategies",
			Validate: func(combo StrategyCombo) bool {
				maxVal, maxOK := combo.Values[FactorMaximizeCoupleISA]
				if !maxOK {
					return true
				}
				maximize, _ := maxVal.Value.(bool)
				if !maximize {
					return true
				}
				drawdownVal, drawOK := combo.Values[FactorDrawdown]
				if !drawOK {
					return true
				}
				drawdown, _ := drawdownVal.Value.(DrawdownOrder)
				return drawdown == PensionToISA || drawdown == PensionToISAProactive
			},
		},
		{
			ID:          "pension_only_no_maximize_isa",
			Description: "PensionOnly strategy cannot use MaximizeCoupleISA",
			Validate: func(combo StrategyCombo) bool {
				drawdownVal, drawOK := combo.Values[FactorDrawdown]
				if !drawOK {
					return true
				}
				drawdown, _ := drawdownVal.Value.(DrawdownOrder)
				if drawdown != PensionOnly {
					return true
				}
				maxVal, maxOK := combo.Values[FactorMaximizeCoupleISA]
				if !maxOK {
					return true
				}
				maximize, _ := maxVal.Value.(bool)
				return !maximize
			},
		},
		{
			ID:          "isa_to_sipp_not_with_pension_to_isa",
			Description: "ISA to SIPP and PensionToISA are contradictory strategies",
			Validate: func(combo StrategyCombo) bool {
				isaToSippVal, isaOK := combo.Values[FactorISAToSIPP]
				if !isaOK {
					return true
				}
				isaToSipp, _ := isaToSippVal.Value.(bool)
				if !isaToSipp {
					return true
				}
				drawdownVal, drawOK := combo.Values[FactorDrawdown]
				if !drawOK {
					return true
				}
				drawdown, _ := drawdownVal.Value.(DrawdownOrder)
				return drawdown != PensionToISA && drawdown != PensionToISAProactive
			},
		},
		{
			ID:          "ufpls_not_with_pcls_payoff",
			Description: "UFPLS strategy cannot use PCLS mortgage payoff",
			Validate: func(combo StrategyCombo) bool {
				crystVal, crystOK := combo.Values[FactorCrystallisation]
				if !crystOK {
					return true
				}
				cryst, _ := crystVal.Value.(Strategy)
				if cryst != UFPLSStrategy {
					return true
				}
				mortgageVal, mortOK := combo.Values[FactorMortgage]
				if !mortOK {
					return true
				}
				mortgage, _ := mortgageVal.Value.(MortgageOption)
				return mortgage != PCLSMortgagePayoff
			},
		},
	}
}

// CombinationGenerator generates valid strategy combinations
type CombinationGenerator struct {
	registry    *FactorRegistry
	constraints []Constraint
	config      *Config
}

// NewCombinationGenerator creates a new generator for the given config
func NewCombinationGenerator(config *Config) *CombinationGenerator {
	return &CombinationGenerator{
		registry:    NewFactorRegistry(),
		constraints: DefaultConstraints(),
		config:      config,
	}
}

// GenerateCombinations generates all valid combinations for a mode
func (g *CombinationGenerator) GenerateCombinations(mode PermutationMode) []StrategyCombo {
	// Get factors filtered by mode
	factors := g.registry.GetFactorsByMode(g.config, mode)

	// Generate all combinations (cartesian product)
	allCombos := g.cartesianProduct(factors)

	// Filter by constraints
	validCombos := make([]StrategyCombo, 0)
	for _, combo := range allCombos {
		if g.isValid(combo) {
			validCombos = append(validCombos, combo)
		}
	}

	return validCombos
}

// cartesianProduct generates all combinations of factor values
func (g *CombinationGenerator) cartesianProduct(factors []*Factor) []StrategyCombo {
	if len(factors) == 0 {
		return []StrategyCombo{{Values: make(map[FactorID]FactorValue)}}
	}

	result := make([]StrategyCombo, 0)

	// Start with first factor
	for _, val := range factors[0].Values {
		combo := StrategyCombo{Values: make(map[FactorID]FactorValue)}
		combo.Values[factors[0].ID] = val
		result = append(result, combo)
	}

	// Extend with remaining factors
	for i := 1; i < len(factors); i++ {
		newResult := make([]StrategyCombo, 0)
		for _, existingCombo := range result {
			for _, val := range factors[i].Values {
				newCombo := existingCombo.Clone()
				newCombo.Values[factors[i].ID] = val
				newResult = append(newResult, newCombo)
			}
		}
		result = newResult
	}

	return result
}

// isValid checks if a combination passes all constraints
func (g *CombinationGenerator) isValid(combo StrategyCombo) bool {
	for _, constraint := range g.constraints {
		if !constraint.Validate(combo) {
			return false
		}
	}
	return true
}

// ToSimulationParams converts a StrategyCombo to SimulationParams
func (combo StrategyCombo) ToSimulationParams() SimulationParams {
	params := SimulationParams{}

	if v, ok := combo.Values[FactorCrystallisation]; ok {
		params.CrystallisationStrategy, _ = v.Value.(Strategy)
	}
	if v, ok := combo.Values[FactorDrawdown]; ok {
		params.DrawdownOrder, _ = v.Value.(DrawdownOrder)
	}
	if v, ok := combo.Values[FactorMortgage]; ok {
		params.MortgageOpt, _ = v.Value.(MortgageOption)
	} else {
		params.MortgageOpt = MortgageNormal // Default when no mortgage factor
	}
	if v, ok := combo.Values[FactorMaximizeCoupleISA]; ok {
		params.MaximizeCoupleISA, _ = v.Value.(bool)
	}
	if v, ok := combo.Values[FactorISAToSIPP]; ok {
		params.ISAToSIPPEnabled, _ = v.Value.(bool)
	}
	if v, ok := combo.Values[FactorGuardrails]; ok {
		params.GuardrailsEnabled, _ = v.Value.(bool)
	}
	if v, ok := combo.Values[FactorStatePensionDefer]; ok {
		params.StatePensionDeferYears, _ = v.Value.(int)
	}

	params.SourceCombo = &combo
	return params
}

// GetStrategiesForConfigV2 generates strategy combinations using the new permutation engine
func GetStrategiesForConfigV2(config *Config, mode PermutationMode) []SimulationParams {
	generator := NewCombinationGenerator(config)
	combos := generator.GenerateCombinations(mode)

	params := make([]SimulationParams, len(combos))
	for i, combo := range combos {
		params[i] = combo.ToSimulationParams()
	}

	return params
}

// GetCombinationCount returns the expected number of combinations for each mode
func GetCombinationCount(config *Config) map[PermutationMode]int {
	counts := make(map[PermutationMode]int)
	generator := NewCombinationGenerator(config)

	for _, mode := range []PermutationMode{ModeQuick, ModeStandard, ModeThorough, ModeComprehensive} {
		combos := generator.GenerateCombinations(mode)
		counts[mode] = len(combos)
	}

	return counts
}
