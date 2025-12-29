package main

// VPW (Variable Percentage Withdrawal) strategy
// Based on the principle that withdrawal rates should increase with age
// as remaining life expectancy decreases.

// VPWTable contains withdrawal percentages by age
// Based on IRS Required Minimum Distribution tables adapted for UK use
// These percentages represent what portion of the portfolio to withdraw each year
var VPWTable = map[int]float64{
	55: 0.030, // 3.0% at age 55
	56: 0.031,
	57: 0.032,
	58: 0.033,
	59: 0.034,
	60: 0.035, // 3.5% at age 60
	61: 0.036,
	62: 0.037,
	63: 0.039,
	64: 0.040,
	65: 0.042, // 4.2% at age 65
	66: 0.043,
	67: 0.045,
	68: 0.047,
	69: 0.048,
	70: 0.050, // 5.0% at age 70
	71: 0.052,
	72: 0.054,
	73: 0.056,
	74: 0.058,
	75: 0.061, // 6.1% at age 75
	76: 0.064,
	77: 0.067,
	78: 0.070,
	79: 0.073,
	80: 0.077, // 7.7% at age 80
	81: 0.081,
	82: 0.085,
	83: 0.089,
	84: 0.094,
	85: 0.100, // 10.0% at age 85
	86: 0.106,
	87: 0.113,
	88: 0.120,
	89: 0.128,
	90: 0.137, // 13.7% at age 90
	91: 0.147,
	92: 0.159,
	93: 0.172,
	94: 0.187,
	95: 0.204, // 20.4% at age 95
	96: 0.224,
	97: 0.247,
	98: 0.274,
	99: 0.307,
	100: 0.350, // 35% at age 100+
}

// GetVPWRate returns the VPW withdrawal rate for a given age
// For ages below 55, returns 0 (not yet retired typically)
// For ages above 100, returns the rate for age 100
func GetVPWRate(age int) float64 {
	if age < 55 {
		return 0.030 // Minimum rate for very early retirees
	}
	if age > 100 {
		return 0.350 // Maximum rate
	}
	if rate, ok := VPWTable[age]; ok {
		return rate
	}
	// Interpolate if age not in table (shouldn't happen with full table)
	return 0.050 // Default to 5%
}

// VPWState tracks state for VPW calculations
type VPWState struct {
	Enabled            bool
	FloorEnabled       bool    // If true, never withdraw less than floor
	FloorAmount        float64 // Minimum annual withdrawal (inflation adjusted)
	CeilingEnabled     bool    // If true, never withdraw more than ceiling
	CeilingMultiplier  float64 // Maximum as multiple of floor (e.g., 1.5 = 150%)
	InitialFloor       float64 // Floor amount in year 1 (for inflation adjustment)
}

// NewVPWState creates a VPW state from config
func NewVPWState(config *Config) *VPWState {
	if !config.IncomeRequirements.VPWEnabled {
		return nil
	}

	floor := config.IncomeRequirements.VPWFloor
	ceiling := config.IncomeRequirements.VPWCeiling

	return &VPWState{
		Enabled:           true,
		FloorEnabled:      floor > 0,
		FloorAmount:       floor,
		InitialFloor:      floor,
		CeilingEnabled:    ceiling > 0,
		CeilingMultiplier: ceiling,
	}
}

// CalculateVPWWithdrawal calculates the VPW-based withdrawal amount
// portfolioValue: current total portfolio value
// age: age of the reference person
// inflationMultiplier: cumulative inflation since retirement start
// Returns: suggested annual withdrawal amount
func (v *VPWState) CalculateVPWWithdrawal(portfolioValue float64, age int, inflationMultiplier float64) float64 {
	if v == nil || !v.Enabled {
		return 0
	}

	// Get base VPW rate for this age
	rate := GetVPWRate(age)

	// Calculate raw VPW amount
	vpwAmount := portfolioValue * rate

	// Apply floor if enabled
	if v.FloorEnabled && v.InitialFloor > 0 {
		inflatedFloor := v.InitialFloor * inflationMultiplier
		if vpwAmount < inflatedFloor {
			vpwAmount = inflatedFloor
		}
		v.FloorAmount = inflatedFloor // Track current floor

		// Apply ceiling if enabled (as multiple of floor)
		if v.CeilingEnabled && v.CeilingMultiplier > 0 {
			ceiling := inflatedFloor * v.CeilingMultiplier
			if vpwAmount > ceiling {
				vpwAmount = ceiling
			}
		}
	}

	return vpwAmount
}

// GetCurrentRate returns the VPW percentage for given age
func (v *VPWState) GetCurrentRate(age int) float64 {
	return GetVPWRate(age)
}

// LifeExpectancyYears returns estimated remaining years based on age
// Uses UK ONS life expectancy data approximation
func LifeExpectancyYears(age int) float64 {
	// Simplified life expectancy table (UK averages, blended male/female)
	switch {
	case age <= 55:
		return 30.0
	case age <= 60:
		return 25.0
	case age <= 65:
		return 21.0
	case age <= 70:
		return 17.0
	case age <= 75:
		return 13.5
	case age <= 80:
		return 10.0
	case age <= 85:
		return 7.5
	case age <= 90:
		return 5.5
	case age <= 95:
		return 4.0
	default:
		return 3.0
	}
}
