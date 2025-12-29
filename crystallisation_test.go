package main

import (
	"math"
	"testing"
)

// Pension Crystallisation Validation Tests
//
// These tests validate the 25% tax-free / 75% taxable crystallisation rules
// per HMRC guidelines.
//
// Reference: https://www.gov.uk/hmrc-internal-manuals/pensions-tax-manual/ptm063240
//
// Key rules:
// - Pension Commencement Lump Sum (PCLS) is normally 25% of the crystallised value
// - The remaining 75% becomes taxable income when withdrawn
// - Lump Sum Allowance (LSA) for 2024/25 is £268,275 (not tested here - focus on 25/75 split)

const crystalTolerance = 0.01

func assertCrystalEquals(t *testing.T, expected, actual float64, description string) {
	t.Helper()
	if math.Abs(expected-actual) > crystalTolerance {
		t.Errorf("%s: expected £%.2f, got £%.2f (diff: £%.2f)",
			description, expected, actual, actual-expected)
	}
}

// =============================================================================
// Upfront Crystallisation Tests
// =============================================================================

func TestTakePCLSLumpSum_BasicSplit(t *testing.T) {
	// Reference: HMRC PTM063240 - PCLS is 25% of crystallised benefits
	tests := []struct {
		uncrystallised   float64
		expectedTaxFree  float64
		expectedTaxable  float64
		description      string
	}{
		{
			uncrystallised:  100000,
			expectedTaxFree: 25000,  // 100000 × 0.25
			expectedTaxable: 75000,  // 100000 × 0.75
			description:     "£100k pension - standard split",
		},
		{
			uncrystallised:  500000,
			expectedTaxFree: 125000, // 500000 × 0.25
			expectedTaxable: 375000, // 500000 × 0.75
			description:     "£500k pension - larger pot",
		},
		{
			uncrystallised:  1000000,
			expectedTaxFree: 250000, // 1000000 × 0.25
			expectedTaxable: 750000, // 1000000 × 0.75
			description:     "£1M pension",
		},
		{
			uncrystallised:  12570,
			expectedTaxFree: 3142.50, // 12570 × 0.25
			expectedTaxable: 9427.50, // 12570 × 0.75
			description:     "Small pot equal to Personal Allowance",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			person := &Person{
				Name:              "Test",
				UncrystallisedPot: tc.uncrystallised,
				CrystallisedPot:   0,
				TaxFreeSavings:    0,
			}

			result := TakePCLSLumpSum(person)

			// Check result structure
			assertCrystalEquals(t, tc.uncrystallised, result.AmountCrystallised, "Amount crystallised")
			assertCrystalEquals(t, tc.expectedTaxFree, result.TaxFreePortion, "Tax-free portion")
			assertCrystalEquals(t, tc.expectedTaxable, result.TaxablePortion, "Taxable portion")

			// Check person's state updated correctly
			assertCrystalEquals(t, 0, person.UncrystallisedPot, "Uncrystallised pot should be empty")
			assertCrystalEquals(t, tc.expectedTaxFree, person.TaxFreeSavings, "ISA should have tax-free amount")
			assertCrystalEquals(t, tc.expectedTaxable, person.CrystallisedPot, "Crystallised pot should have taxable amount")

			// Verify 25/75 split mathematically
			if result.TaxFreePortion+result.TaxablePortion != result.AmountCrystallised {
				t.Errorf("Tax-free + Taxable should equal Amount crystallised")
			}
		})
	}
}

func TestTakePCLSLumpSum_AddToExistingISA(t *testing.T) {
	// When crystallising, tax-free portion adds to existing ISA
	person := &Person{
		Name:              "Test",
		UncrystallisedPot: 100000,
		TaxFreeSavings:    50000, // Existing ISA
	}

	TakePCLSLumpSum(person)

	// ISA should now have original 50k + 25k (25% of 100k)
	expectedISA := 75000.0
	assertCrystalEquals(t, expectedISA, person.TaxFreeSavings, "ISA should include existing balance + tax-free")
}

func TestTakePCLSLumpSum_ZeroPot(t *testing.T) {
	person := &Person{
		Name:              "Test",
		UncrystallisedPot: 0,
	}

	result := TakePCLSLumpSum(person)

	if result.AmountCrystallised != 0 {
		t.Errorf("Zero pot should crystallise zero, got £%.2f", result.AmountCrystallised)
	}
}

func TestTakePCLSLumpSum_PreservesTotal(t *testing.T) {
	// Property: Total assets should be preserved during crystallisation
	// (just moved between buckets)
	initialPot := 100000.0

	person := &Person{
		Name:              "Test",
		UncrystallisedPot: initialPot,
		TaxFreeSavings:    0,
		CrystallisedPot:   0,
	}

	TakePCLSLumpSum(person)

	totalAfter := person.UncrystallisedPot + person.TaxFreeSavings + person.CrystallisedPot

	if math.Abs(totalAfter-initialPot) > 0.01 {
		t.Errorf("Total assets should be preserved. Before: £%.2f, After: £%.2f",
			initialPot, totalAfter)
	}
}

// =============================================================================
// Gradual Crystallisation Tests
// =============================================================================

func TestGradualCrystallise_PartialAmount(t *testing.T) {
	person := &Person{
		Name:              "Test",
		UncrystallisedPot: 100000,
	}

	// Crystallise only £20k (not the full pot)
	result := GradualCrystallise(person, 20000)

	assertCrystalEquals(t, 20000, result.AmountCrystallised, "Amount crystallised")
	assertCrystalEquals(t, 5000, result.TaxFreePortion, "Tax-free (25% of 20k)")
	assertCrystalEquals(t, 15000, result.TaxablePortion, "Taxable (75% of 20k)")

	// Check remaining uncrystallised
	assertCrystalEquals(t, 80000, person.UncrystallisedPot, "Should have £80k remaining uncrystallised")
}

func TestGradualCrystallise_ExceedsAvailable(t *testing.T) {
	person := &Person{
		Name:              "Test",
		UncrystallisedPot: 50000,
	}

	// Try to crystallise more than available
	result := GradualCrystallise(person, 100000)

	// Should only crystallise what's available
	assertCrystalEquals(t, 50000, result.AmountCrystallised, "Should crystallise max available")
	assertCrystalEquals(t, 12500, result.TaxFreePortion, "Tax-free (25% of 50k)")
	assertCrystalEquals(t, 37500, result.TaxablePortion, "Taxable (75% of 50k)")
	assertCrystalEquals(t, 0, person.UncrystallisedPot, "Pot should be empty")
}

func TestGradualCrystallise_MultipleWithdrawals(t *testing.T) {
	person := &Person{
		Name:              "Test",
		UncrystallisedPot: 100000,
	}

	// First withdrawal: £30k
	result1 := GradualCrystallise(person, 30000)
	assertCrystalEquals(t, 30000, result1.AmountCrystallised, "First: amount crystallised")
	assertCrystalEquals(t, 70000, person.UncrystallisedPot, "After first: remaining")

	// Second withdrawal: £40k
	result2 := GradualCrystallise(person, 40000)
	assertCrystalEquals(t, 40000, result2.AmountCrystallised, "Second: amount crystallised")
	assertCrystalEquals(t, 30000, person.UncrystallisedPot, "After second: remaining")

	// Third withdrawal: £50k (only 30k available)
	result3 := GradualCrystallise(person, 50000)
	assertCrystalEquals(t, 30000, result3.AmountCrystallised, "Third: should cap at available")
	assertCrystalEquals(t, 0, person.UncrystallisedPot, "After third: empty")
}

func TestGradualCrystallise_ZeroAmount(t *testing.T) {
	person := &Person{
		Name:              "Test",
		UncrystallisedPot: 100000,
	}

	result := GradualCrystallise(person, 0)

	if result.AmountCrystallised != 0 {
		t.Errorf("Zero request should crystallise zero, got £%.2f", result.AmountCrystallised)
	}
	assertCrystalEquals(t, 100000, person.UncrystallisedPot, "Pot should be unchanged")
}

// =============================================================================
// ISA Withdrawal Tests
// =============================================================================

func TestWithdrawFromISA(t *testing.T) {
	person := &Person{
		Name:           "Test",
		TaxFreeSavings: 50000,
	}

	withdrawn := WithdrawFromISA(person, 20000)

	assertCrystalEquals(t, 20000, withdrawn, "Should withdraw requested amount")
	assertCrystalEquals(t, 30000, person.TaxFreeSavings, "ISA should have remainder")
}

func TestWithdrawFromISA_ExceedsBalance(t *testing.T) {
	person := &Person{
		Name:           "Test",
		TaxFreeSavings: 20000,
	}

	withdrawn := WithdrawFromISA(person, 50000)

	assertCrystalEquals(t, 20000, withdrawn, "Should only withdraw available")
	assertCrystalEquals(t, 0, person.TaxFreeSavings, "ISA should be empty")
}

func TestWithdrawFromISA_ZeroBalance(t *testing.T) {
	person := &Person{
		Name:           "Test",
		TaxFreeSavings: 0,
	}

	withdrawn := WithdrawFromISA(person, 10000)

	if withdrawn != 0 {
		t.Errorf("Zero balance should withdraw zero, got £%.2f", withdrawn)
	}
}

// =============================================================================
// Crystallised Pot Withdrawal Tests
// =============================================================================

func TestWithdrawFromCrystallised(t *testing.T) {
	person := &Person{
		Name:            "Test",
		CrystallisedPot: 75000,
	}

	withdrawn := WithdrawFromCrystallised(person, 25000)

	assertCrystalEquals(t, 25000, withdrawn, "Should withdraw requested amount")
	assertCrystalEquals(t, 50000, person.CrystallisedPot, "Pot should have remainder")
}

func TestWithdrawFromCrystallised_ExceedsBalance(t *testing.T) {
	person := &Person{
		Name:            "Test",
		CrystallisedPot: 30000,
	}

	withdrawn := WithdrawFromCrystallised(person, 50000)

	assertCrystalEquals(t, 30000, withdrawn, "Should only withdraw available")
	assertCrystalEquals(t, 0, person.CrystallisedPot, "Pot should be empty")
}

// =============================================================================
// Real-World Scenario Tests
// =============================================================================

func TestScenario_RetirementCrystallisation(t *testing.T) {
	// Scenario: Person retires at 55 with £400k pension
	// Crystallises entire pot upfront

	person := &Person{
		Name:              "Retiree",
		BirthYear:         1970,
		RetirementAge:     55,
		UncrystallisedPot: 400000,
		TaxFreeSavings:    50000, // Existing ISA
	}

	result := TakePCLSLumpSum(person)

	// Verify 25% tax-free = £100k
	assertCrystalEquals(t, 100000, result.TaxFreePortion, "Tax-free portion")

	// Verify 75% taxable = £300k
	assertCrystalEquals(t, 300000, result.TaxablePortion, "Taxable portion")

	// Total ISA should now be £150k (original £50k + £100k tax-free)
	assertCrystalEquals(t, 150000, person.TaxFreeSavings, "Total ISA balance")

	// Crystallised pot should be £300k
	assertCrystalEquals(t, 300000, person.CrystallisedPot, "Crystallised pot")

	t.Logf("Retirement: £400k pension → £150k ISA + £300k crystallised pot")
}

func TestScenario_GradualDrawdown(t *testing.T) {
	// Scenario: Person takes £30k per year from pension using gradual crystallisation
	// over 5 years

	person := &Person{
		Name:              "Drawdown",
		UncrystallisedPot: 500000,
	}

	totalTaxFree := 0.0
	totalTaxable := 0.0

	for year := 1; year <= 5; year++ {
		result := GradualCrystallise(person, 30000)
		totalTaxFree += result.TaxFreePortion
		totalTaxable += result.TaxablePortion
	}

	// 5 years × £30k = £150k total crystallised
	totalCrystallised := 150000.0

	// Tax-free should be 25% of total = £37,500
	assertCrystalEquals(t, 37500, totalTaxFree, "5 years total tax-free")

	// Taxable should be 75% of total = £112,500
	assertCrystalEquals(t, 112500, totalTaxable, "5 years total taxable")

	// Remaining should be £350k
	assertCrystalEquals(t, 350000, person.UncrystallisedPot, "Remaining after 5 years")

	t.Logf("Gradual drawdown: £150k crystallised (£37.5k tax-free, £112.5k taxable), £350k remaining")
	_ = totalCrystallised
}

// =============================================================================
// Property/Invariant Tests
// =============================================================================

func TestCrystallisationInvariant_25_75_Split(t *testing.T) {
	// Property: Tax-free is always exactly 25%, taxable is always exactly 75%

	testAmounts := []float64{1000, 10000, 50000, 100000, 500000, 1000000}

	for _, amount := range testAmounts {
		person := &Person{UncrystallisedPot: amount}
		result := TakePCLSLumpSum(person)

		taxFreeRatio := result.TaxFreePortion / result.AmountCrystallised
		taxableRatio := result.TaxablePortion / result.AmountCrystallised

		if math.Abs(taxFreeRatio-0.25) > 0.0001 {
			t.Errorf("£%.0f: Tax-free ratio should be 0.25, got %.4f", amount, taxFreeRatio)
		}
		if math.Abs(taxableRatio-0.75) > 0.0001 {
			t.Errorf("£%.0f: Taxable ratio should be 0.75, got %.4f", amount, taxableRatio)
		}
	}
}

func TestCrystallisationInvariant_TotalPreserved(t *testing.T) {
	// Property: Total assets must be preserved during crystallisation

	testCases := []struct {
		uncrystallised float64
		existingISA    float64
		existingCryst  float64
	}{
		{100000, 0, 0},
		{200000, 50000, 0},
		{300000, 100000, 50000},
		{500000, 200000, 100000},
	}

	for _, tc := range testCases {
		person := &Person{
			UncrystallisedPot: tc.uncrystallised,
			TaxFreeSavings:    tc.existingISA,
			CrystallisedPot:   tc.existingCryst,
		}

		totalBefore := person.UncrystallisedPot + person.TaxFreeSavings + person.CrystallisedPot

		TakePCLSLumpSum(person)

		totalAfter := person.UncrystallisedPot + person.TaxFreeSavings + person.CrystallisedPot

		if math.Abs(totalAfter-totalBefore) > 0.01 {
			t.Errorf("Assets not preserved. Before: £%.2f, After: £%.2f", totalBefore, totalAfter)
		}
	}
}

func TestCrystallisationInvariant_NonNegativeBalances(t *testing.T) {
	// Property: Balances should never go negative

	person := &Person{
		UncrystallisedPot: 100000,
	}

	// Crystallise multiple times
	for i := 0; i < 10; i++ {
		GradualCrystallise(person, 20000)

		if person.UncrystallisedPot < 0 {
			t.Errorf("Uncrystallised pot went negative: £%.2f", person.UncrystallisedPot)
		}
	}
}
