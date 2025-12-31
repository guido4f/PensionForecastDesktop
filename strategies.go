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
		// Over-draw pension to fill tax bands, excess goes to ISA
		return ExecutePensionToISADrawdown(people, netNeeded, params.CrystallisationStrategy, year, statePensionByPerson, taxBands, params.MaximizeCoupleISA)
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

		// Tax-free from pension is from gradual crystallisation (25%)
		// When crystallising: 25% is tax-free, 75% is taxable
		if taxFreeFromPension > 0 && strategy == GradualCrystallisation {
			// The amount crystallised = taxFreeFromPension / 0.25 = taxFreeFromPension * 4
			amountCrystallised := taxFreeFromPension * 4
			taxablePortion := amountCrystallised * 0.75

			// Actually crystallise from uncrystallised pot
			if p.UncrystallisedPot >= amountCrystallised {
				p.UncrystallisedPot -= amountCrystallised
				// 25% goes directly to breakdown as tax-free withdrawal
				breakdown.TaxFreeFromPension[p.Name] = taxFreeFromPension
				breakdown.TotalTaxFree += taxFreeFromPension

				// 75% becomes crystallised pot, then withdrawn as taxable
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
			result := GradualCrystallise(p, amountToCrystallise)
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
				result := GradualCrystallise(p, amountToCrystallise)
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
	if config.HasMortgage() {
		// Full set of strategies with all mortgage options
		return []SimulationParams{
			// === GRADUAL CRYSTALLISATION STRATEGIES ===
			// Early payoff strategies
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: SavingsFirst, MortgageOpt: MortgageEarly},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionFirst, MortgageOpt: MortgageEarly},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: TaxOptimized, MortgageOpt: MortgageEarly},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionToISA, MortgageOpt: MortgageEarly},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: FillBasicRate, MortgageOpt: MortgageEarly},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: StatePensionBridge, MortgageOpt: MortgageEarly},
			// Normal payoff strategies
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: SavingsFirst, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionFirst, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: TaxOptimized, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionToISA, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: FillBasicRate, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: StatePensionBridge, MortgageOpt: MortgageNormal},
			// Extended (+10 years) payoff strategies
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: SavingsFirst, MortgageOpt: MortgageExtended},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionFirst, MortgageOpt: MortgageExtended},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: TaxOptimized, MortgageOpt: MortgageExtended},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionToISA, MortgageOpt: MortgageExtended},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: FillBasicRate, MortgageOpt: MortgageExtended},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: StatePensionBridge, MortgageOpt: MortgageExtended},
			// PCLS mortgage payoff (use 25% lump sum, no further tax-free)
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: SavingsFirst, MortgageOpt: PCLSMortgagePayoff},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionFirst, MortgageOpt: PCLSMortgagePayoff},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: TaxOptimized, MortgageOpt: PCLSMortgagePayoff},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionToISA, MortgageOpt: PCLSMortgagePayoff},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: FillBasicRate, MortgageOpt: PCLSMortgagePayoff},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: StatePensionBridge, MortgageOpt: PCLSMortgagePayoff},

			// === UFPLS STRATEGIES (25% tax-free on each withdrawal) ===
			// Normal payoff only for UFPLS to keep combinations manageable
			{CrystallisationStrategy: UFPLSStrategy, DrawdownOrder: SavingsFirst, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: UFPLSStrategy, DrawdownOrder: PensionFirst, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: UFPLSStrategy, DrawdownOrder: TaxOptimized, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: UFPLSStrategy, DrawdownOrder: FillBasicRate, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: UFPLSStrategy, DrawdownOrder: StatePensionBridge, MortgageOpt: MortgageNormal},
		}
	}

	// No mortgage - only test drawdown order strategies (mortgage options are irrelevant)
	return []SimulationParams{
		// Gradual Crystallisation
		{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: SavingsFirst, MortgageOpt: MortgageNormal},
		{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionFirst, MortgageOpt: MortgageNormal},
		{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: TaxOptimized, MortgageOpt: MortgageNormal},
		{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionToISA, MortgageOpt: MortgageNormal},
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

// GetDepletionStrategiesForConfig returns strategies for depletion mode
// Similar to GetStrategiesForConfig but used in depletion calculations
func GetDepletionStrategiesForConfig(config *Config) []SimulationParams {
	return GetStrategiesForConfig(config)
}

// GetPensionOnlyStrategiesForConfig returns strategies for pension-only mode
func GetPensionOnlyStrategiesForConfig(config *Config) []SimulationParams {
	if config.HasMortgage() {
		return []SimulationParams{
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionOnly, MortgageOpt: MortgageEarly},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionOnly, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionOnly, MortgageOpt: MortgageExtended},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionOnly, MortgageOpt: PCLSMortgagePayoff},
		}
	}
	return []SimulationParams{
		{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionOnly, MortgageOpt: MortgageNormal},
	}
}

// GetPensionToISAStrategiesForConfig returns strategies for pension-to-ISA mode
func GetPensionToISAStrategiesForConfig(config *Config) []SimulationParams {
	if config.HasMortgage() {
		return []SimulationParams{
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionToISA, MortgageOpt: MortgageEarly},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionToISA, MortgageOpt: MortgageNormal},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionToISA, MortgageOpt: MortgageExtended},
			{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionToISA, MortgageOpt: PCLSMortgagePayoff},
		}
	}
	return []SimulationParams{
		{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionToISA, MortgageOpt: MortgageNormal},
	}
}
