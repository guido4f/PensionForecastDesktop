package main

import (
	"math"
	"testing"
)

// Mathematical Invariants Test Suite
//
// This file contains property-based tests that verify mathematical
// invariants that must always hold regardless of input values.
//
// These tests validate the logical consistency of the financial
// calculations rather than specific numeric values.

// =============================================================================
// Tax Calculation Invariants
// =============================================================================

func TestInvariant_TaxMonotonicallyIncreases(t *testing.T) {
	// Property: For any income increase, tax should increase or stay same
	// (never decrease)

	incomes := []float64{0, 10000, 12570, 20000, 50270, 60000, 100000, 125140, 150000, 200000}

	var previousTax float64 = 0

	for _, income := range incomes {
		tax := CalculateTaxWithTapering(income, ukTaxBands2024)

		if tax < previousTax {
			t.Errorf("Tax decreased from £%.2f to £%.2f when income increased to £%.0f",
				previousTax, tax, income)
		}

		previousTax = tax
	}
}

func TestInvariant_TaxNeverExceedsIncome(t *testing.T) {
	// Property: Tax can never exceed gross income

	incomes := []float64{1000, 10000, 50000, 100000, 200000, 500000}

	for _, income := range incomes {
		tax := CalculateTaxWithTapering(income, ukTaxBands2024)

		if tax > income {
			t.Errorf("Tax £%.2f exceeds income £%.0f", tax, income)
		}
	}
}

func TestInvariant_ZeroIncomeZeroTax(t *testing.T) {
	// Property: Zero or negative income should result in zero tax

	tax := CalculateTaxWithTapering(0, ukTaxBands2024)
	if tax != 0 {
		t.Errorf("Zero income should have zero tax, got £%.2f", tax)
	}

	taxNegative := CalculateTaxWithTapering(-1000, ukTaxBands2024)
	if taxNegative != 0 {
		t.Errorf("Negative income should have zero tax, got £%.2f", taxNegative)
	}
}

func TestInvariant_MarginalTaxNonNegative(t *testing.T) {
	// Property: Marginal tax on additional income is never negative

	incomes := []float64{0, 12570, 50270, 100000, 125140}
	withdrawals := []float64{1000, 5000, 10000, 20000}

	for _, income := range incomes {
		for _, withdrawal := range withdrawals {
			marginalTax := CalculateMarginalTax(withdrawal, income, ukTaxBands2024)

			if marginalTax < 0 {
				t.Errorf("Marginal tax on £%.0f withdrawal from £%.0f income is negative: £%.2f",
					withdrawal, income, marginalTax)
			}
		}
	}
}

func TestInvariant_GrossUpNetEqualsOriginal(t *testing.T) {
	// Property: GrossUpForTax should find gross such that gross - tax = net needed

	testCases := []struct {
		netNeeded      float64
		existingIncome float64
	}{
		{10000, 0},
		{20000, 12570},
		{30000, 50000},
		{15000, 100000},
	}

	for _, tc := range testCases {
		gross, tax := GrossUpForTax(tc.netNeeded, tc.existingIncome, ukTaxBands2024)
		netReceived := gross - tax

		if math.Abs(netReceived-tc.netNeeded) > 1.0 {
			t.Errorf("GrossUp failed: needed £%.0f net, got gross £%.2f - tax £%.2f = net £%.2f",
				tc.netNeeded, gross, tax, netReceived)
		}
	}
}

// =============================================================================
// Growth Calculation Invariants
// =============================================================================

func TestInvariant_PositiveGrowthIncreasesValue(t *testing.T) {
	// Property: Positive growth rate always increases value

	rates := []float64{0.01, 0.03, 0.05, 0.10}
	initials := []float64{1000, 50000, 100000, 1000000}

	for _, rate := range rates {
		for _, initial := range initials {
			person := &Person{TaxFreeSavings: initial}
			ApplyGrowth(person, rate, rate)

			if person.TaxFreeSavings <= initial {
				t.Errorf("Growth at %.0f%% should increase £%.0f, got £%.2f",
					rate*100, initial, person.TaxFreeSavings)
			}
		}
	}
}

func TestInvariant_ZeroGrowthPreservesValue(t *testing.T) {
	// Property: Zero growth rate preserves value exactly

	initials := []float64{1000, 50000, 100000}

	for _, initial := range initials {
		person := &Person{TaxFreeSavings: initial}
		ApplyGrowth(person, 0, 0)

		if person.TaxFreeSavings != initial {
			t.Errorf("Zero growth should preserve £%.0f, got £%.2f",
				initial, person.TaxFreeSavings)
		}
	}
}

func TestInvariant_NegativeGrowthDecreasesValue(t *testing.T) {
	// Property: Negative growth rate decreases value

	rates := []float64{-0.05, -0.10, -0.20}
	initial := 100000.0

	for _, rate := range rates {
		person := &Person{TaxFreeSavings: initial}
		ApplyGrowth(person, rate, rate)

		if person.TaxFreeSavings >= initial {
			t.Errorf("Negative growth at %.0f%% should decrease £%.0f, got £%.2f",
				rate*100, initial, person.TaxFreeSavings)
		}
	}
}

func TestInvariant_GrowthIsMultiplicative(t *testing.T) {
	// Property: Two consecutive 10% growths should equal one (1.1)^2 = 1.21 = 21% growth

	initial := 100000.0
	rate := 0.10

	// Method 1: Apply growth twice
	person1 := &Person{TaxFreeSavings: initial}
	ApplyGrowth(person1, rate, rate)
	ApplyGrowth(person1, rate, rate)

	// Method 2: Calculate directly
	expected := initial * math.Pow(1+rate, 2)

	if math.Abs(person1.TaxFreeSavings-expected) > 0.01 {
		t.Errorf("Growth should be multiplicative: expected £%.2f, got £%.2f",
			expected, person1.TaxFreeSavings)
	}
}

// =============================================================================
// Withdrawal Invariants
// =============================================================================

func TestInvariant_WithdrawalNeverExceedsBalance(t *testing.T) {
	// Property: Withdrawals cannot exceed available balance

	person := &Person{
		TaxFreeSavings: 50000,
		CrystallisedPot: 75000,
	}

	// Try to withdraw more than available from ISA
	isaWithdrawal := WithdrawFromISA(person, 100000)
	if isaWithdrawal > 50000 {
		t.Errorf("ISA withdrawal £%.2f exceeds balance £50000", isaWithdrawal)
	}

	// Try to withdraw more than available from crystallised
	crystWithdrawal := WithdrawFromCrystallised(person, 200000)
	if crystWithdrawal > 75000 {
		t.Errorf("Crystallised withdrawal £%.2f exceeds balance £75000", crystWithdrawal)
	}
}

func TestInvariant_BalancesNeverNegative(t *testing.T) {
	// Property: Account balances should never go negative

	person := &Person{
		TaxFreeSavings:    50000,
		CrystallisedPot:   75000,
		UncrystallisedPot: 100000,
	}

	// Withdraw everything
	WithdrawFromISA(person, 1000000)
	WithdrawFromCrystallised(person, 1000000)
	GradualCrystallise(person, 1000000)

	if person.TaxFreeSavings < 0 {
		t.Errorf("ISA balance went negative: £%.2f", person.TaxFreeSavings)
	}
	if person.CrystallisedPot < 0 {
		t.Errorf("Crystallised pot went negative: £%.2f", person.CrystallisedPot)
	}
	if person.UncrystallisedPot < 0 {
		t.Errorf("Uncrystallised pot went negative: £%.2f", person.UncrystallisedPot)
	}
}

// =============================================================================
// Crystallisation Invariants
// =============================================================================

func TestInvariant_Crystallisation25_75Split(t *testing.T) {
	// Property: Crystallisation always splits 25% tax-free, 75% taxable

	amounts := []float64{10000, 50000, 100000, 500000, 1000000}

	for _, amount := range amounts {
		person := &Person{UncrystallisedPot: amount}
		result := TakePCLSLumpSum(person)

		// Check exact ratios
		if math.Abs(result.TaxFreePortion/amount-0.25) > 0.0001 {
			t.Errorf("Tax-free portion should be 25%%, got %.4f%%",
				result.TaxFreePortion/amount*100)
		}

		if math.Abs(result.TaxablePortion/amount-0.75) > 0.0001 {
			t.Errorf("Taxable portion should be 75%%, got %.4f%%",
				result.TaxablePortion/amount*100)
		}
	}
}

func TestInvariant_CrystallisationPreservesTotal(t *testing.T) {
	// Property: Total assets are preserved during crystallisation

	testCases := []struct {
		initial float64
	}{
		{100000},
		{500000},
		{1000000},
	}

	for _, tc := range testCases {
		person := &Person{
			UncrystallisedPot: tc.initial,
			TaxFreeSavings:    0,
			CrystallisedPot:   0,
		}

		TakePCLSLumpSum(person)

		total := person.UncrystallisedPot + person.TaxFreeSavings + person.CrystallisedPot

		if math.Abs(total-tc.initial) > 0.01 {
			t.Errorf("Total should be preserved at £%.0f, got £%.2f", tc.initial, total)
		}
	}
}

// =============================================================================
// Mortgage Invariants
// =============================================================================

func TestInvariant_RepaymentBalanceDecreases(t *testing.T) {
	// Property: For repayment mortgages, balance strictly decreases each year

	mortgage := MortgagePartConfig{
		Principal:    200000,
		InterestRate: 0.04,
		TermYears:    25,
		IsRepayment:  true,
		StartYear:    2024,
	}

	previousBalance := mortgage.Principal

	for year := 2025; year <= 2048; year++ {
		balance := mortgage.CalculateRemainingBalance(year)

		if balance >= previousBalance {
			t.Errorf("Year %d: Balance £%.2f should be less than previous £%.2f",
				year, balance, previousBalance)
		}

		previousBalance = balance
	}
}

func TestInvariant_InterestOnlyBalanceConstant(t *testing.T) {
	// Property: Interest-only mortgage balance stays constant

	mortgage := MortgagePartConfig{
		Principal:    200000,
		InterestRate: 0.05,
		IsRepayment:  false,
		StartYear:    2024,
	}

	for year := 2025; year <= 2050; year++ {
		balance := mortgage.CalculateRemainingBalance(year)

		if balance != mortgage.Principal {
			t.Errorf("Year %d: Interest-only balance should be £%.0f, got £%.2f",
				year, mortgage.Principal, balance)
		}
	}
}

func TestInvariant_MortgagePaymentCoversInterest(t *testing.T) {
	// Property: Monthly payment must exceed first month's interest for principal to decrease

	testCases := []struct {
		principal float64
		rate      float64
		term      int
	}{
		{200000, 0.04, 25},
		{300000, 0.05, 30},
		{500000, 0.06, 25},
	}

	for _, tc := range testCases {
		mortgage := MortgagePartConfig{
			Principal:    tc.principal,
			InterestRate: tc.rate,
			TermYears:    tc.term,
			IsRepayment:  true,
		}

		payment := mortgage.CalculateMonthlyPayment()
		firstMonthInterest := tc.principal * tc.rate / 12

		if payment <= firstMonthInterest {
			t.Errorf("Payment £%.2f must exceed first month interest £%.2f for £%.0f @ %.1f%%",
				payment, firstMonthInterest, tc.principal, tc.rate*100)
		}
	}
}

func TestInvariant_MortgageFullyPaidAtTerm(t *testing.T) {
	// Property: At end of term, repayment mortgage balance is zero

	testCases := []struct {
		principal float64
		rate      float64
		term      int
	}{
		{200000, 0.04, 25},
		{300000, 0.05, 30},
		{100000, 0.03, 15},
	}

	for _, tc := range testCases {
		mortgage := MortgagePartConfig{
			Principal:    tc.principal,
			InterestRate: tc.rate,
			TermYears:    tc.term,
			IsRepayment:  true,
			StartYear:    2024,
		}

		endYear := 2024 + tc.term
		balance := mortgage.CalculateRemainingBalance(endYear)

		if balance > 0.01 {
			t.Errorf("£%.0f @ %.1f%% for %d years: Balance at term end should be £0, got £%.2f",
				tc.principal, tc.rate*100, tc.term, balance)
		}
	}
}

// =============================================================================
// Inflation Invariants
// =============================================================================

func TestInvariant_InflatedBandsIncrease(t *testing.T) {
	// Property: Positive inflation increases tax band thresholds

	inflated := InflateTaxBands(ukTaxBands2024, 2024, 2034, 0.025)

	for i, band := range inflated {
		if band.Upper < ukTaxBands2024[i].Upper {
			t.Errorf("Band %d upper limit should increase with inflation", i)
		}
		if band.Lower < ukTaxBands2024[i].Lower && i > 0 {
			t.Errorf("Band %d lower limit should increase with inflation", i)
		}
	}
}

func TestInvariant_InflationRatesPreserved(t *testing.T) {
	// Property: Tax rates remain unchanged after inflation adjustment

	inflated := InflateTaxBands(ukTaxBands2024, 2024, 2050, 0.03)

	for i, band := range inflated {
		if band.Rate != ukTaxBands2024[i].Rate {
			t.Errorf("Band %d rate changed from %.2f to %.2f",
				i, ukTaxBands2024[i].Rate, band.Rate)
		}
	}
}

// =============================================================================
// Net vs Gross Invariants
// =============================================================================

func TestInvariant_NetAlwaysLessThanOrEqualGross(t *testing.T) {
	// Property: Net received (after tax) is always <= gross withdrawal

	withdrawals := []float64{10000, 30000, 50000, 100000}
	existingIncomes := []float64{0, 12570, 50270, 100000}

	for _, withdrawal := range withdrawals {
		for _, existing := range existingIncomes {
			tax := CalculateMarginalTax(withdrawal, existing, ukTaxBands2024)
			net := withdrawal - tax

			if net > withdrawal {
				t.Errorf("Net £%.2f > Gross £%.0f with existing income £%.0f",
					net, withdrawal, existing)
			}
		}
	}
}
