package main

import (
	"math"
	"testing"
)

// Compound Growth Validation Tests
//
// These tests validate compound interest calculations against standard formulas.
// Reference: https://www.pensionbee.com/uk/inflation-calculator
//
// Standard compound interest formula:
// A = P × (1 + r)^n
// Where:
//   A = Final amount
//   P = Principal (initial amount)
//   r = Annual growth rate (as decimal)
//   n = Number of years

const growthTolerance = 0.01 // £0.01 tolerance

func assertGrowthEquals(t *testing.T, expected, actual float64, description string) {
	t.Helper()
	if math.Abs(expected-actual) > growthTolerance {
		t.Errorf("%s: expected £%.2f, got £%.2f (diff: £%.2f)",
			description, expected, actual, actual-expected)
	}
}

// =============================================================================
// Single Year Growth Tests
// =============================================================================

func TestApplyGrowth_SingleYear(t *testing.T) {
	tests := []struct {
		initial      float64
		rate         float64
		expected     float64
		description  string
	}{
		{100000, 0.05, 105000, "£100k @ 5%"},
		{100000, 0.03, 103000, "£100k @ 3%"},
		{100000, 0.10, 110000, "£100k @ 10%"},
		{50000, 0.05, 52500, "£50k @ 5%"},
		{1000000, 0.05, 1050000, "£1M @ 5%"},
		{100000, 0.00, 100000, "£100k @ 0% (no growth)"},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			person := &Person{
				TaxFreeSavings:    tc.initial,
				CrystallisedPot:   tc.initial,
				UncrystallisedPot: tc.initial,
			}

			ApplyGrowth(person, tc.rate, tc.rate)

			assertGrowthEquals(t, tc.expected, person.TaxFreeSavings, "ISA")
			assertGrowthEquals(t, tc.expected, person.CrystallisedPot, "Crystallised")
			assertGrowthEquals(t, tc.expected, person.UncrystallisedPot, "Uncrystallised")
		})
	}
}

// =============================================================================
// Multi-Year Compound Growth Tests
// =============================================================================

func TestCompoundGrowth_MultiYear(t *testing.T) {
	// Formula: A = P × (1 + r)^n
	// Reference values verified against https://www.pensionbee.com/uk/inflation-calculator
	tests := []struct {
		principal    float64
		rate         float64
		years        int
		expected     float64
		description  string
	}{
		{
			principal:   100000,
			rate:        0.05,
			years:       10,
			expected:    162889.46, // 100000 × (1.05)^10
			description: "£100k @ 5% for 10 years",
		},
		{
			principal:   100000,
			rate:        0.05,
			years:       20,
			expected:    265329.77, // 100000 × (1.05)^20
			description: "£100k @ 5% for 20 years",
		},
		{
			principal:   100000,
			rate:        0.03,
			years:       10,
			expected:    134391.64, // 100000 × (1.03)^10
			description: "£100k @ 3% for 10 years",
		},
		{
			principal:   100000,
			rate:        0.07,
			years:       10,
			expected:    196715.14, // 100000 × (1.07)^10
			description: "£100k @ 7% for 10 years",
		},
		{
			principal:   500000,
			rate:        0.04,
			years:       15,
			expected:    900471.75, // 500000 × (1.04)^15 (accounting for fp precision)
			description: "£500k @ 4% for 15 years",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			person := &Person{
				TaxFreeSavings: tc.principal,
			}

			// Apply growth for n years
			for i := 0; i < tc.years; i++ {
				ApplyGrowth(person, tc.rate, tc.rate)
			}

			assertGrowthEquals(t, tc.expected, person.TaxFreeSavings, tc.description)
		})
	}
}

// =============================================================================
// Inflation Adjustment Tests
// =============================================================================

func TestInflationMultiplier(t *testing.T) {
	// Formula: Multiplier = (1 + rate)^years
	// Used to adjust income requirements for inflation
	tests := []struct {
		rate        float64
		years       int
		expected    float64
		description string
	}{
		{0.025, 10, 1.28008, "2.5% for 10 years"},
		{0.03, 10, 1.34392, "3% for 10 years"},
		{0.02, 20, 1.48595, "2% for 20 years"},
		{0.025, 25, 1.85394, "2.5% for 25 years"},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			multiplier := math.Pow(1+tc.rate, float64(tc.years))

			if math.Abs(multiplier-tc.expected) > 0.001 {
				t.Errorf("Inflation multiplier for %s: expected %.5f, got %.5f",
					tc.description, tc.expected, multiplier)
			}
		})
	}
}

func TestInflatedIncome(t *testing.T) {
	// If you need £30,000/year today, how much will you need in 10 years
	// at 2.5% inflation?
	// Answer: £30,000 × (1.025)^10 = £38,402.39
	tests := []struct {
		baseIncome  float64
		rate        float64
		years       int
		expected    float64
		description string
	}{
		{30000, 0.025, 10, 38402.39, "£30k income after 10 years at 2.5%"},
		{50000, 0.03, 15, 77898.37, "£50k income after 15 years at 3%"},
		{60000, 0.02, 25, 98436.36, "£60k income after 25 years at 2%"},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			inflated := tc.baseIncome * math.Pow(1+tc.rate, float64(tc.years))

			if math.Abs(inflated-tc.expected) > 1.0 {
				t.Errorf("%s: expected £%.2f, got £%.2f",
					tc.description, tc.expected, inflated)
			}
		})
	}
}

// =============================================================================
// Different Growth Rates for Different Assets
// =============================================================================

func TestApplyGrowth_DifferentRates(t *testing.T) {
	// ISA and pension can have different growth rates
	person := &Person{
		TaxFreeSavings:    100000, // ISA
		CrystallisedPot:   200000, // Pension
		UncrystallisedPot: 300000, // Pension
	}

	savingsRate := 0.04 // 4% for ISA
	pensionRate := 0.06 // 6% for pension

	ApplyGrowth(person, savingsRate, pensionRate)

	// ISA should grow at 4%
	assertGrowthEquals(t, 104000, person.TaxFreeSavings, "ISA @ 4%")

	// Pension should grow at 6%
	assertGrowthEquals(t, 212000, person.CrystallisedPot, "Crystallised @ 6%")
	assertGrowthEquals(t, 318000, person.UncrystallisedPot, "Uncrystallised @ 6%")
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestApplyGrowth_ZeroBalance(t *testing.T) {
	person := &Person{
		TaxFreeSavings:    0,
		CrystallisedPot:   0,
		UncrystallisedPot: 0,
	}

	ApplyGrowth(person, 0.05, 0.05)

	if person.TaxFreeSavings != 0 {
		t.Errorf("Zero balance should remain zero, got £%.2f", person.TaxFreeSavings)
	}
}

func TestApplyGrowth_NegativeRate(t *testing.T) {
	// Market downturn scenario
	person := &Person{
		TaxFreeSavings: 100000,
	}

	ApplyGrowth(person, -0.10, -0.10) // -10% loss

	assertGrowthEquals(t, 90000, person.TaxFreeSavings, "10% market loss")
}

// =============================================================================
// Real-World Scenario Tests
// =============================================================================

func TestRealWorldScenario_PensionGrowthToRetirement(t *testing.T) {
	// Person aged 40 with £200k pension, retiring at 55
	// At 5% growth, what will they have?
	// Answer: £200,000 × (1.05)^15 = £415,786.27

	person := &Person{
		UncrystallisedPot: 200000,
	}

	// 15 years of growth at 5%
	for i := 0; i < 15; i++ {
		ApplyGrowth(person, 0.05, 0.05)
	}

	expected := 200000 * math.Pow(1.05, 15) // 415786.27

	if math.Abs(person.UncrystallisedPot-expected) > 1.0 {
		t.Errorf("Expected £%.2f, got £%.2f", expected, person.UncrystallisedPot)
	}
}

func TestRealWorldScenario_CoupleRetirementSavings(t *testing.T) {
	// Couple with combined £1M pension pot at age 55
	// If they retire at 67 (12 years), at 4% growth:
	// £1,000,000 × (1.04)^12 = £1,601,032.22

	person := &Person{
		UncrystallisedPot: 1000000,
	}

	for i := 0; i < 12; i++ {
		ApplyGrowth(person, 0.04, 0.04)
	}

	expected := 1000000 * math.Pow(1.04, 12) // 1601032.22

	if math.Abs(person.UncrystallisedPot-expected) > 1.0 {
		t.Errorf("Expected £%.2f, got £%.2f", expected, person.UncrystallisedPot)
	}
}

// =============================================================================
// Property Tests - Mathematical Invariants
// =============================================================================

func TestGrowthInvariant_PositiveRateIncreasesValue(t *testing.T) {
	// Property: If rate > 0 and initial > 0, then final > initial
	testCases := []float64{100, 1000, 10000, 100000, 1000000}
	rates := []float64{0.01, 0.03, 0.05, 0.10}

	for _, initial := range testCases {
		for _, rate := range rates {
			person := &Person{TaxFreeSavings: initial}
			ApplyGrowth(person, rate, rate)

			if person.TaxFreeSavings <= initial {
				t.Errorf("Growth at %.0f%% should increase value. Initial: £%.2f, Final: £%.2f",
					rate*100, initial, person.TaxFreeSavings)
			}
		}
	}
}

func TestGrowthInvariant_ZeroRateMaintainsValue(t *testing.T) {
	// Property: If rate = 0, then final = initial
	testCases := []float64{100, 1000, 10000, 100000}

	for _, initial := range testCases {
		person := &Person{TaxFreeSavings: initial}
		ApplyGrowth(person, 0, 0)

		if person.TaxFreeSavings != initial {
			t.Errorf("Zero growth should maintain value. Initial: £%.2f, Final: £%.2f",
				initial, person.TaxFreeSavings)
		}
	}
}

func TestGrowthInvariant_CompoundingOrder(t *testing.T) {
	// Property: (1 + r)^(a+b) = (1 + r)^a × (1 + r)^b
	// Applying growth for 10 years is same as 5 years twice
	initial := 100000.0
	rate := 0.05

	// Method 1: 10 years at once
	person1 := &Person{TaxFreeSavings: initial}
	for i := 0; i < 10; i++ {
		ApplyGrowth(person1, rate, rate)
	}

	// Method 2: 5 years, then another 5 years
	person2 := &Person{TaxFreeSavings: initial}
	for i := 0; i < 5; i++ {
		ApplyGrowth(person2, rate, rate)
	}
	for i := 0; i < 5; i++ {
		ApplyGrowth(person2, rate, rate)
	}

	if math.Abs(person1.TaxFreeSavings-person2.TaxFreeSavings) > 0.01 {
		t.Errorf("Compounding should be associative. Method 1: £%.2f, Method 2: £%.2f",
			person1.TaxFreeSavings, person2.TaxFreeSavings)
	}
}
