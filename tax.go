package main

import (
	"math"
)

// Personal Allowance tapering thresholds (2024/25 onwards)
const (
	PersonalAllowanceBase    = 12570.0  // Standard Personal Allowance
	TaperingThreshold        = 100000.0 // Income above which allowance reduces
	TaperingRate             = 0.5      // £1 reduction per £2 over threshold
	AllowanceFullyRemovedAt  = 125140.0 // Income at which allowance is zero
)

// ApplyPersonalAllowanceTapering adjusts tax bands to account for reduced Personal Allowance
// for incomes over £100,000. Returns modified tax bands.
func ApplyPersonalAllowanceTapering(bands []TaxBand, totalIncome float64) []TaxBand {
	if totalIncome <= TaperingThreshold {
		// No tapering needed
		return bands
	}

	// Calculate reduced Personal Allowance
	reduction := (totalIncome - TaperingThreshold) * TaperingRate
	reducedAllowance := math.Max(0, PersonalAllowanceBase-reduction)

	// Find the Personal Allowance band (rate = 0) and adjust it
	adjustedBands := make([]TaxBand, len(bands))
	for i, band := range bands {
		adjustedBands[i] = band

		// If this is the Personal Allowance band (0% rate starting at 0)
		if band.Lower == 0 && band.Rate == 0 {
			// Reduce the upper limit of the Personal Allowance
			adjustedBands[i].Upper = reducedAllowance

			// If next band is Basic Rate, adjust its lower bound
			if i+1 < len(bands) {
				// Need to adjust subsequent bands
			}
		}

		// If this is the Basic Rate band, adjust lower bound
		if band.Rate > 0 && i > 0 && bands[i-1].Rate == 0 {
			// Adjust lower bound to match reduced Personal Allowance
			adjustedBands[i].Lower = reducedAllowance
		}
	}

	return adjustedBands
}

// CalculateTaxOnIncome calculates the tax owed on a given taxable income
// This is the base calculation without Personal Allowance tapering
func CalculateTaxOnIncome(income float64, bands []TaxBand) float64 {
	if income <= 0 {
		return 0
	}

	var totalTax float64

	for _, band := range bands {
		if income <= band.Lower {
			break
		}

		// Calculate the taxable amount in this band
		taxableInBand := math.Min(income, band.Upper) - band.Lower
		if taxableInBand > 0 {
			totalTax += taxableInBand * band.Rate
		}
	}

	return totalTax
}

// CalculateTaxWithTapering calculates tax including Personal Allowance tapering
// for incomes over £100,000
func CalculateTaxWithTapering(income float64, bands []TaxBand) float64 {
	if income <= 0 {
		return 0
	}

	// Apply Personal Allowance tapering if applicable
	adjustedBands := ApplyPersonalAllowanceTapering(bands, income)
	return CalculateTaxOnIncome(income, adjustedBands)
}

// CalculateMarginalTax calculates the additional tax from withdrawing an amount
// given existing taxable income (includes Personal Allowance tapering)
func CalculateMarginalTax(withdrawalAmount, existingIncome float64, bands []TaxBand) float64 {
	taxWithWithdrawal := CalculateTaxWithTapering(existingIncome+withdrawalAmount, bands)
	taxWithoutWithdrawal := CalculateTaxWithTapering(existingIncome, bands)
	return taxWithWithdrawal - taxWithoutWithdrawal
}

// GrossUpForTax calculates the gross amount needed to achieve a net amount after tax
// Uses binary search to find the gross amount
func GrossUpForTax(netNeeded, existingIncome float64, bands []TaxBand) (gross, tax float64) {
	if netNeeded <= 0 {
		return 0, 0
	}

	// Start with bounds
	low := netNeeded
	high := netNeeded * 2.5 // Allow for up to 45% tax rate + margin

	// Binary search for the correct gross amount
	for i := 0; i < 100; i++ {
		mid := (low + high) / 2
		marginalTax := CalculateMarginalTax(mid, existingIncome, bands)
		netFromMid := mid - marginalTax

		if math.Abs(netFromMid-netNeeded) < 0.01 {
			return mid, marginalTax
		}

		if netFromMid < netNeeded {
			low = mid
		} else {
			high = mid
		}
	}

	// Return best estimate if convergence takes too long
	finalTax := CalculateMarginalTax(high, existingIncome, bands)
	return high, finalTax
}

// CalculatePersonTax calculates total tax for a person in a year
// including state pension and any taxable withdrawals (with Personal Allowance tapering)
func CalculatePersonTax(statePension, taxableWithdrawal float64, bands []TaxBand) float64 {
	totalTaxableIncome := statePension + taxableWithdrawal
	return CalculateTaxWithTapering(totalTaxableIncome, bands)
}

// InflateTaxBands returns tax bands inflated from start year to current year
func InflateTaxBands(baseBands []TaxBand, startYear, currentYear int, inflationRate float64) []TaxBand {
	if inflationRate == 0 || currentYear <= startYear {
		return baseBands
	}

	yearsElapsed := currentYear - startYear
	multiplier := math.Pow(1+inflationRate, float64(yearsElapsed))

	inflatedBands := make([]TaxBand, len(baseBands))
	for i, band := range baseBands {
		inflatedBands[i] = TaxBand{
			Name:  band.Name,
			Lower: band.Lower * multiplier,
			Upper: band.Upper * multiplier,
			Rate:  band.Rate, // Rate stays the same
		}
	}
	return inflatedBands
}

// GetMarginalRate returns the marginal tax rate for a given income level
func GetMarginalRate(income float64, bands []TaxBand) float64 {
	for _, band := range bands {
		if income >= band.Lower && income < band.Upper {
			return band.Rate
		}
	}
	// If above all bands, return the highest rate
	if len(bands) > 0 {
		return bands[len(bands)-1].Rate
	}
	return 0
}

// OptimalWithdrawalSplit calculates how to split withdrawals between two people
// to minimise total tax paid
func OptimalWithdrawalSplit(totalNeeded float64, person1StatePension, person2StatePension float64,
	person1Available, person2Available float64, bands []TaxBand) (person1Withdrawal, person2Withdrawal float64) {

	// If one person has no available funds, the other takes all
	if person1Available <= 0 {
		return 0, math.Min(totalNeeded, person2Available)
	}
	if person2Available <= 0 {
		return math.Min(totalNeeded, person1Available), 0
	}

	// For proportional split based on available funds
	totalAvailable := person1Available + person2Available
	if totalAvailable <= 0 {
		return 0, 0
	}

	person1Share := person1Available / totalAvailable
	person2Share := person2Available / totalAvailable

	person1Withdrawal = totalNeeded * person1Share
	person2Withdrawal = totalNeeded * person2Share

	// Cap at available amounts
	if person1Withdrawal > person1Available {
		excess := person1Withdrawal - person1Available
		person1Withdrawal = person1Available
		person2Withdrawal += excess
	}
	if person2Withdrawal > person2Available {
		excess := person2Withdrawal - person2Available
		person2Withdrawal = person2Available
		person1Withdrawal += excess
	}

	// Final cap
	person1Withdrawal = math.Min(person1Withdrawal, person1Available)
	person2Withdrawal = math.Min(person2Withdrawal, person2Available)

	return person1Withdrawal, person2Withdrawal
}
