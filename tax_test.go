package main

import (
	"math"
	"testing"
)

// Tax Calculation Validation Tests
//
// These tests validate tax calculations against official UK Government figures.
// Reference: https://www.gov.uk/income-tax-rates (2024/25 tax year)
//
// Tax bands for 2024/25:
// - Personal Allowance: £0 - £12,570 (0%)
// - Basic Rate: £12,571 - £50,270 (20%)
// - Higher Rate: £50,271 - £125,140 (40%)
// - Additional Rate: £125,140+ (45%)
//
// Personal Allowance Tapering:
// - Starts at £100,000 income
// - Reduces by £1 for every £2 above £100,000
// - Fully removed at £125,140
// Reference: https://www.gov.uk/income-tax-rates/income-over-100000

// Standard UK tax bands for 2024/25
var ukTaxBands2024 = []TaxBand{
	{Name: "Personal Allowance", Lower: 0, Upper: 12570, Rate: 0.0},
	{Name: "Basic Rate", Lower: 12570, Upper: 50270, Rate: 0.20},
	{Name: "Higher Rate", Lower: 50270, Upper: 125140, Rate: 0.40},
	{Name: "Additional Rate", Lower: 125140, Upper: 1000000000, Rate: 0.45},
}

// tolerance for floating point comparisons (£0.01)
const taxTolerance = 0.01

func assertTaxEquals(t *testing.T, expected, actual float64, description string) {
	t.Helper()
	if math.Abs(expected-actual) > taxTolerance {
		t.Errorf("%s: expected £%.2f, got £%.2f (diff: £%.2f)",
			description, expected, actual, actual-expected)
	}
}

// =============================================================================
// Basic Tax Calculation Tests (Without Tapering)
// =============================================================================

func TestTaxCalculation_WithinPersonalAllowance(t *testing.T) {
	// Reference: GOV.UK - Income up to Personal Allowance is tax-free
	tests := []struct {
		income      float64
		expectedTax float64
	}{
		{0, 0},
		{5000, 0},
		{10000, 0},
		{12570, 0}, // Exactly at Personal Allowance
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			tax := CalculateTaxOnIncome(tc.income, ukTaxBands2024)
			assertTaxEquals(t, tc.expectedTax, tax,
				"Income £%.0f within Personal Allowance should be tax-free")
		})
	}
}

func TestTaxCalculation_BasicRateBand(t *testing.T) {
	// Reference: GOV.UK - Basic rate is 20% on income £12,571 to £50,270
	tests := []struct {
		income      float64
		expectedTax float64
		calculation string
	}{
		{
			income:      20000,
			expectedTax: 1486.00, // (20000 - 12570) * 0.20 = 7430 * 0.20 = 1486
			calculation: "(20000 - 12570) × 0.20 = 1486",
		},
		{
			income:      30000,
			expectedTax: 3486.00, // (30000 - 12570) * 0.20 = 17430 * 0.20 = 3486
			calculation: "(30000 - 12570) × 0.20 = 3486",
		},
		{
			income:      50270,
			expectedTax: 7540.00, // (50270 - 12570) * 0.20 = 37700 * 0.20 = 7540
			calculation: "(50270 - 12570) × 0.20 = 7540",
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			tax := CalculateTaxOnIncome(tc.income, ukTaxBands2024)
			assertTaxEquals(t, tc.expectedTax, tax,
				"Income £%.0f: %s")
		})
	}
}

func TestTaxCalculation_HigherRateBand(t *testing.T) {
	// Reference: GOV.UK - Higher rate is 40% on income £50,271 to £125,140
	// Tax = Basic rate portion + Higher rate portion
	tests := []struct {
		income      float64
		expectedTax float64
		calculation string
	}{
		{
			income:      60000,
			expectedTax: 11432.00,
			// Basic: (50270 - 12570) * 0.20 = 7540
			// Higher: (60000 - 50270) * 0.40 = 3892
			// Total: 7540 + 3892 = 11432
			calculation: "Basic: 7540 + Higher: 3892 = 11432",
		},
		{
			income:      80000,
			expectedTax: 19432.00,
			// Basic: 7540
			// Higher: (80000 - 50270) * 0.40 = 11892
			// Total: 7540 + 11892 = 19432
			calculation: "Basic: 7540 + Higher: 11892 = 19432",
		},
		{
			income:      100000,
			expectedTax: 27432.00,
			// Basic: 7540
			// Higher: (100000 - 50270) * 0.40 = 19892
			// Total: 7540 + 19892 = 27432
			calculation: "Basic: 7540 + Higher: 19892 = 27432",
		},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			tax := CalculateTaxOnIncome(tc.income, ukTaxBands2024)
			assertTaxEquals(t, tc.expectedTax, tax,
				"Income £%.0f: %s")
		})
	}
}

// =============================================================================
// Personal Allowance Tapering Tests
// =============================================================================
// Reference: https://www.gov.uk/income-tax-rates/income-over-100000
// "Your Personal Allowance goes down by £1 for every £2 that your adjusted
// net income is above £100,000. This means your allowance is zero if your
// income is £125,140 or above."

func TestPersonalAllowanceTapering_Constants(t *testing.T) {
	// Verify constants match GOV.UK 2024/25 figures
	if PersonalAllowanceBase != 12570.0 {
		t.Errorf("PersonalAllowanceBase should be £12,570, got £%.0f", PersonalAllowanceBase)
	}
	if TaperingThreshold != 100000.0 {
		t.Errorf("TaperingThreshold should be £100,000, got £%.0f", TaperingThreshold)
	}
	if TaperingRate != 0.5 {
		t.Errorf("TaperingRate should be 0.5 (£1 per £2), got %.2f", TaperingRate)
	}
	if AllowanceFullyRemovedAt != 125140.0 {
		t.Errorf("AllowanceFullyRemovedAt should be £125,140, got £%.0f", AllowanceFullyRemovedAt)
	}
}

func TestTaxWithTapering_60PercentTrap(t *testing.T) {
	// Reference: https://www.rathbones.com/100k-tax-trap-to-hit-2m-taxpayers
	// The "60% tax trap" occurs between £100,000 and £125,140
	// For every £2 earned, you lose £1 of Personal Allowance
	// This creates an effective marginal rate of ~60%:
	// - 40% higher rate tax
	// - Plus 20% effective rate from losing Personal Allowance (40% × 50%)

	tests := []struct {
		income             float64
		expectedTax        float64
		reducedAllowance   float64
		effectiveMarginal  float64
		description        string
	}{
		{
			income:            105000,
			reducedAllowance:  10070, // 12570 - (105000-100000)*0.5 = 12570 - 2500 = 10070
			expectedTax:       29932.00,
			effectiveMarginal: 0.60,
			description:       "£5k into tapering zone",
			// Tax calculation with reduced allowance:
			// Personal: 0-10070 @ 0% = 0
			// Basic: 10070-50270 @ 20% = 40200 * 0.20 = 8040
			// Higher: 50270-105000 @ 40% = 54730 * 0.40 = 21892
			// Total: 0 + 8040 + 21892 = 29932
		},
		{
			income:            110000,
			reducedAllowance:  7570, // 12570 - (110000-100000)*0.5 = 12570 - 5000 = 7570
			expectedTax:       32432.00,
			effectiveMarginal: 0.60,
			description:       "£10k into tapering zone",
			// Tax calculation:
			// Personal: 0-7570 @ 0% = 0
			// Basic: 7570-50270 @ 20% = 42700 * 0.20 = 8540
			// Higher: 50270-110000 @ 40% = 59730 * 0.40 = 23892
			// Total: 0 + 8540 + 23892 = 32432
		},
		{
			income:            125140,
			reducedAllowance:  0, // Allowance fully removed
			expectedTax:       40002.00,
			effectiveMarginal: 0.40, // Back to normal higher rate
			description:       "Allowance fully removed",
			// Tax calculation:
			// Basic: 0-50270 @ 20% = 50270 * 0.20 = 10054
			// Higher: 50270-125140 @ 40% = 74870 * 0.40 = 29948
			// Total: 10054 + 29948 = 40002
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			tax := CalculateTaxWithTapering(tc.income, ukTaxBands2024)

			// Verify the reduced allowance calculation
			reduction := (tc.income - TaperingThreshold) * TaperingRate
			actualReducedAllowance := math.Max(0, PersonalAllowanceBase-reduction)
			if math.Abs(actualReducedAllowance-tc.reducedAllowance) > 0.01 {
				t.Errorf("Reduced allowance should be £%.0f, got £%.0f",
					tc.reducedAllowance, actualReducedAllowance)
			}

			assertTaxEquals(t, tc.expectedTax, tax, tc.description)
		})
	}
}

func TestTaxWithTapering_AdditionalRate(t *testing.T) {
	// Reference: GOV.UK - Additional rate is 45% above £125,140
	// At this point Personal Allowance is zero
	tests := []struct {
		income      float64
		expectedTax float64
		description string
	}{
		{
			income:      150000,
			expectedTax: 51189.00,
			// No Personal Allowance
			// Basic: 50270 * 0.20 = 10054
			// Higher: (125140 - 50270) * 0.40 = 74870 * 0.40 = 29948
			// Additional: (150000 - 125140) * 0.45 = 24860 * 0.45 = 11187
			// Total: 10054 + 29948 + 11187 = 51189
			description: "£150k with additional rate",
		},
		{
			income:      200000,
			expectedTax: 73689.00,
			// Basic: 50270 * 0.20 = 10054
			// Higher: 74870 * 0.40 = 29948
			// Additional: (200000 - 125140) * 0.45 = 74860 * 0.45 = 33687
			// Total: 10054 + 29948 + 33687 = 73689
			description: "£200k with additional rate",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			tax := CalculateTaxWithTapering(tc.income, ukTaxBands2024)
			assertTaxEquals(t, tc.expectedTax, tax, tc.description)
		})
	}
}

// =============================================================================
// Marginal Tax Rate Tests
// =============================================================================

func TestMarginalTax_FromZeroIncome(t *testing.T) {
	// Marginal tax on withdrawals when starting from zero income
	tests := []struct {
		withdrawal  float64
		expectedTax float64
		description string
	}{
		{10000, 0, "Within Personal Allowance"},
		{12570, 0, "Exactly at Personal Allowance"},
		{20000, 1486, "Into Basic Rate"},
		{50270, 7540, "Exactly at top of Basic Rate"},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			tax := CalculateMarginalTax(tc.withdrawal, 0, ukTaxBands2024)
			assertTaxEquals(t, tc.expectedTax, tax, tc.description)
		})
	}
}

func TestMarginalTax_WithExistingIncome(t *testing.T) {
	// Reference: Marginal tax = Tax(income + withdrawal) - Tax(income)
	tests := []struct {
		withdrawal     float64
		existingIncome float64
		expectedTax    float64
		description    string
	}{
		{
			withdrawal:     10000,
			existingIncome: 12570, // Already used Personal Allowance
			expectedTax:    2000,  // All at 20% basic rate
			description:    "Basic rate marginal",
		},
		{
			withdrawal:     10000,
			existingIncome: 50270, // At top of basic rate
			expectedTax:    4000,  // All at 40% higher rate
			description:    "Higher rate marginal",
		},
		{
			withdrawal:     10000,
			existingIncome: 125140, // At additional rate threshold
			expectedTax:    4500,   // All at 45% additional rate
			description:    "Additional rate marginal",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			tax := CalculateMarginalTax(tc.withdrawal, tc.existingIncome, ukTaxBands2024)
			assertTaxEquals(t, tc.expectedTax, tax, tc.description)
		})
	}
}

// =============================================================================
// GrossUpForTax Tests
// =============================================================================

func TestGrossUpForTax_BasicRate(t *testing.T) {
	// If you need £8,000 net and you're in basic rate (20%)
	// Gross needed = Net / (1 - rate) = 8000 / 0.80 = 10000
	// Tax = 10000 * 0.20 = 2000

	netNeeded := 8000.0
	existingIncome := 12570.0 // Used all Personal Allowance

	gross, tax := GrossUpForTax(netNeeded, existingIncome, ukTaxBands2024)

	expectedGross := 10000.0
	expectedTax := 2000.0

	if math.Abs(gross-expectedGross) > 1.0 {
		t.Errorf("Gross should be ~£%.0f, got £%.2f", expectedGross, gross)
	}
	if math.Abs(tax-expectedTax) > 1.0 {
		t.Errorf("Tax should be ~£%.0f, got £%.2f", expectedTax, tax)
	}

	// Verify: gross - tax = net needed
	netReceived := gross - tax
	if math.Abs(netReceived-netNeeded) > 1.0 {
		t.Errorf("Net received (£%.2f) should equal net needed (£%.0f)", netReceived, netNeeded)
	}
}

func TestGrossUpForTax_HigherRate(t *testing.T) {
	// If you need £6,000 net and you're in higher rate (40%)
	// Gross needed = Net / (1 - rate) = 6000 / 0.60 = 10000
	// Tax = 10000 * 0.40 = 4000

	netNeeded := 6000.0
	existingIncome := 60000.0 // In higher rate band

	gross, tax := GrossUpForTax(netNeeded, existingIncome, ukTaxBands2024)

	expectedGross := 10000.0
	expectedTax := 4000.0

	if math.Abs(gross-expectedGross) > 1.0 {
		t.Errorf("Gross should be ~£%.0f, got £%.2f", expectedGross, gross)
	}
	if math.Abs(tax-expectedTax) > 1.0 {
		t.Errorf("Tax should be ~£%.0f, got £%.2f", expectedTax, tax)
	}
}

// =============================================================================
// Tax Band Inflation Tests
// =============================================================================

func TestInflateTaxBands(t *testing.T) {
	// Reference: Tax bands can be frozen or inflated
	// Formula: Inflated = Base × (1 + rate)^years

	startYear := 2024
	currentYear := 2034 // 10 years later
	inflationRate := 0.025 // 2.5% per year

	inflatedBands := InflateTaxBands(ukTaxBands2024, startYear, currentYear, inflationRate)

	// Expected multiplier: (1.025)^10 = 1.28008...
	expectedMultiplier := math.Pow(1.025, 10)

	// Check Personal Allowance inflated correctly
	expectedPA := 12570 * expectedMultiplier // ~16,092
	if math.Abs(inflatedBands[0].Upper-expectedPA) > 1.0 {
		t.Errorf("Inflated Personal Allowance should be ~£%.0f, got £%.2f",
			expectedPA, inflatedBands[0].Upper)
	}

	// Check rates are unchanged
	for i, band := range inflatedBands {
		if band.Rate != ukTaxBands2024[i].Rate {
			t.Errorf("Band %d rate should remain %.2f, got %.2f",
				i, ukTaxBands2024[i].Rate, band.Rate)
		}
	}
}

// =============================================================================
// Effective Tax Rate Tests
// =============================================================================

func TestEffectiveTaxRate(t *testing.T) {
	// Verify effective tax rates match calculated values
	// Effective rate = Total tax / Gross income
	tests := []struct {
		income             float64
		expectedEffective  float64
		tolerance          float64
		description        string
	}{
		{20000, 0.0743, 0.01, "£20k ~7.4%"},         // 1486/20000 = 0.0743
		{50270, 0.150, 0.01, "£50k ~15%"},           // 7540/50270 = 0.150
		{100000, 0.2743, 0.01, "£100k ~27.4%"},      // 27432/100000 = 0.2743
		{150000, 0.341, 0.01, "£150k ~34.1%"},       // 51189/150000 = 0.341
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			tax := CalculateTaxWithTapering(tc.income, ukTaxBands2024)
			effectiveRate := tax / tc.income

			if math.Abs(effectiveRate-tc.expectedEffective) > tc.tolerance {
				t.Errorf("Effective rate should be ~%.1f%%, got %.1f%%",
					tc.expectedEffective*100, effectiveRate*100)
			}
		})
	}
}

// =============================================================================
// Boundary Condition Tests
// =============================================================================

func TestTaxBoundaries(t *testing.T) {
	// Test exact boundary values
	boundaries := []struct {
		income      float64
		description string
	}{
		{12570, "Personal Allowance threshold"},
		{12570.01, "Just above Personal Allowance"},
		{50270, "Basic/Higher rate boundary"},
		{50270.01, "Just into Higher rate"},
		{100000, "Tapering threshold"},
		{100000.01, "Just into tapering"},
		{125140, "Allowance fully removed"},
		{125140.01, "Just into Additional rate zone"},
	}

	for _, b := range boundaries {
		t.Run(b.description, func(t *testing.T) {
			tax := CalculateTaxWithTapering(b.income, ukTaxBands2024)
			if tax < 0 {
				t.Errorf("Tax at boundary £%.2f should not be negative", b.income)
			}
			// Just verify no crashes at boundaries
			t.Logf("%s (£%.2f): Tax = £%.2f", b.description, b.income, tax)
		})
	}
}
