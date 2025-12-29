package main

import (
	"math"
	"testing"
)

// Mortgage Calculation Validation Tests
//
// These tests validate mortgage amortization calculations against standard formulas.
// Reference: https://www.moneysavingexpert.com/mortgages/mortgage-rate-calculator/
//
// Standard mortgage formulas:
//
// Monthly Payment (Repayment):
//   M = P × [r(1+r)^n] / [(1+r)^n - 1]
//   Where:
//     M = Monthly payment
//     P = Principal (loan amount)
//     r = Monthly interest rate (annual rate / 12)
//     n = Total number of payments (years × 12)
//
// Monthly Payment (Interest-Only):
//   M = P × (annual_rate / 12)
//
// Remaining Balance:
//   B = P × [(1+r)^n - (1+r)^p] / [(1+r)^n - 1]
//   Where:
//     p = Number of payments already made

const mortgageTolerance = 0.50 // £0.50 tolerance for rounding

func assertMortgageEquals(t *testing.T, expected, actual float64, description string) {
	t.Helper()
	if math.Abs(expected-actual) > mortgageTolerance {
		t.Errorf("%s: expected £%.2f, got £%.2f (diff: £%.2f)",
			description, expected, actual, actual-expected)
	}
}

// =============================================================================
// Repayment Mortgage Monthly Payment Tests
// =============================================================================

func TestMortgage_RepaymentMonthlyPayment(t *testing.T) {
	// Reference: MSE Mortgage Calculator
	// https://www.moneysavingexpert.com/mortgages/mortgage-rate-calculator/
	tests := []struct {
		principal       float64
		interestRate    float64
		termYears       int
		expectedMonthly float64
		description     string
	}{
		{
			principal:       200000,
			interestRate:    0.04,
			termYears:       25,
			expectedMonthly: 1055.67,
			description:     "£200k @ 4% for 25 years",
			// Formula: M = 200000 × [0.00333(1.00333)^300] / [(1.00333)^300 - 1]
			// M = 200000 × [0.00333 × 2.7138] / [2.7138 - 1]
			// M = 200000 × 0.00904 / 1.7138 = 1055.67
		},
		{
			principal:       300000,
			interestRate:    0.05,
			termYears:       30,
			expectedMonthly: 1610.46,
			description:     "£300k @ 5% for 30 years",
		},
		{
			principal:       150000,
			interestRate:    0.035,
			termYears:       20,
			expectedMonthly: 869.94,
			description:     "£150k @ 3.5% for 20 years",
		},
		{
			principal:       500000,
			interestRate:    0.06,
			termYears:       25,
			expectedMonthly: 3221.51,
			description:     "£500k @ 6% for 25 years",
		},
		{
			principal:       100000,
			interestRate:    0.00,
			termYears:       10,
			expectedMonthly: 833.33,
			description:     "£100k @ 0% for 10 years (interest-free)",
			// Simple: 100000 / 120 = 833.33
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			mortgage := MortgagePartConfig{
				Principal:    tc.principal,
				InterestRate: tc.interestRate,
				TermYears:    tc.termYears,
				IsRepayment:  true,
				StartYear:    2024,
			}

			monthly := mortgage.CalculateMonthlyPayment()
			assertMortgageEquals(t, tc.expectedMonthly, monthly, tc.description)
		})
	}
}

func TestMortgage_RepaymentAnnualPayment(t *testing.T) {
	mortgage := MortgagePartConfig{
		Principal:    200000,
		InterestRate: 0.04,
		TermYears:    25,
		IsRepayment:  true,
		StartYear:    2024,
	}

	monthly := mortgage.CalculateMonthlyPayment()
	annual := mortgage.CalculateAnnualPayment()

	expectedAnnual := monthly * 12

	assertMortgageEquals(t, expectedAnnual, annual, "Annual = Monthly × 12")
}

// =============================================================================
// Interest-Only Mortgage Tests
// =============================================================================

func TestMortgage_InterestOnlyMonthlyPayment(t *testing.T) {
	// Interest-only formula: M = P × (r / 12)
	tests := []struct {
		principal       float64
		interestRate    float64
		expectedMonthly float64
		description     string
	}{
		{
			principal:       200000,
			interestRate:    0.04,
			expectedMonthly: 666.67, // 200000 × 0.04 / 12 = 666.67
			description:     "£200k @ 4% interest-only",
		},
		{
			principal:       300000,
			interestRate:    0.05,
			expectedMonthly: 1250.00, // 300000 × 0.05 / 12 = 1250
			description:     "£300k @ 5% interest-only",
		},
		{
			principal:       500000,
			interestRate:    0.06,
			expectedMonthly: 2500.00, // 500000 × 0.06 / 12 = 2500
			description:     "£500k @ 6% interest-only",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			mortgage := MortgagePartConfig{
				Principal:    tc.principal,
				InterestRate: tc.interestRate,
				IsRepayment:  false,
				StartYear:    2024,
			}

			monthly := mortgage.CalculateMonthlyPayment()
			assertMortgageEquals(t, tc.expectedMonthly, monthly, tc.description)
		})
	}
}

func TestMortgage_InterestOnlyBalanceNeverDecreases(t *testing.T) {
	mortgage := MortgagePartConfig{
		Principal:    300000,
		InterestRate: 0.05,
		IsRepayment:  false,
		StartYear:    2024,
	}

	// For interest-only, balance should always equal principal
	years := []int{2025, 2030, 2040, 2050}

	for _, year := range years {
		balance := mortgage.CalculateRemainingBalance(year)
		if balance != mortgage.Principal {
			t.Errorf("Year %d: Interest-only balance should be £%.0f, got £%.2f",
				year, mortgage.Principal, balance)
		}
	}
}

// =============================================================================
// Remaining Balance Tests (Amortization Schedule)
// =============================================================================

func TestMortgage_RemainingBalance(t *testing.T) {
	// Reference: Standard amortization formula
	// B = P × [(1+r)^n - (1+r)^p] / [(1+r)^n - 1]

	mortgage := MortgagePartConfig{
		Principal:    200000,
		InterestRate: 0.04,
		TermYears:    25,
		IsRepayment:  true,
		StartYear:    2024,
	}

	tests := []struct {
		year            int
		expectedBalance float64
		description     string
	}{
		{2024, 200000.00, "At start (year 0)"},
		{2025, 195245.38, "After 1 year"},
		{2029, 174209.23, "After 5 years"},
		{2034, 142718.79, "After 10 years"},
		{2039, 104269.07, "After 15 years"},
		{2044, 57322.10, "After 20 years"},
		{2049, 0.00, "After 25 years (paid off)"},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			balance := mortgage.CalculateRemainingBalance(tc.year)

			// Use larger tolerance for balance (£1)
			if math.Abs(balance-tc.expectedBalance) > 1.0 {
				t.Errorf("%s: expected £%.2f, got £%.2f",
					tc.description, tc.expectedBalance, balance)
			}
		})
	}
}

func TestMortgage_BalanceDecreases(t *testing.T) {
	// Property: For repayment mortgage, balance should decrease each year
	mortgage := MortgagePartConfig{
		Principal:    200000,
		InterestRate: 0.04,
		TermYears:    25,
		IsRepayment:  true,
		StartYear:    2024,
	}

	previousBalance := mortgage.Principal

	for year := 2025; year <= 2048; year++ {
		currentBalance := mortgage.CalculateRemainingBalance(year)

		if currentBalance >= previousBalance {
			t.Errorf("Year %d: Balance £%.2f should be less than previous £%.2f",
				year, currentBalance, previousBalance)
		}

		previousBalance = currentBalance
	}
}

func TestMortgage_TotalInterestPaid(t *testing.T) {
	// Total paid = Monthly payment × number of payments
	// Total interest = Total paid - Principal

	mortgage := MortgagePartConfig{
		Principal:    200000,
		InterestRate: 0.04,
		TermYears:    25,
		IsRepayment:  true,
		StartYear:    2024,
	}

	monthlyPayment := mortgage.CalculateMonthlyPayment()
	totalPayments := float64(mortgage.TermYears * 12)
	totalPaid := monthlyPayment * totalPayments
	totalInterest := totalPaid - mortgage.Principal

	// Expected total interest for £200k @ 4% over 25 years: ~£116,700
	expectedInterest := 116702.0

	if math.Abs(totalInterest-expectedInterest) > 50.0 {
		t.Errorf("Total interest expected ~£%.0f, got £%.2f", expectedInterest, totalInterest)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestMortgage_ZeroPrincipal(t *testing.T) {
	mortgage := MortgagePartConfig{
		Principal:    0,
		InterestRate: 0.04,
		TermYears:    25,
		IsRepayment:  true,
		StartYear:    2024,
	}

	monthly := mortgage.CalculateMonthlyPayment()
	if monthly != 0 {
		t.Errorf("Zero principal should have zero payment, got £%.2f", monthly)
	}

	balance := mortgage.CalculateRemainingBalance(2030)
	if balance != 0 {
		t.Errorf("Zero principal should have zero balance, got £%.2f", balance)
	}
}

func TestMortgage_BeforeStartYear(t *testing.T) {
	mortgage := MortgagePartConfig{
		Principal:    200000,
		InterestRate: 0.04,
		TermYears:    25,
		IsRepayment:  true,
		StartYear:    2024,
	}

	// Before start year, balance should be full principal
	balance := mortgage.CalculateRemainingBalance(2020)
	if balance != mortgage.Principal {
		t.Errorf("Before start year, balance should be £%.0f, got £%.2f",
			mortgage.Principal, balance)
	}
}

func TestMortgage_AfterEndYear(t *testing.T) {
	mortgage := MortgagePartConfig{
		Principal:    200000,
		InterestRate: 0.04,
		TermYears:    25,
		IsRepayment:  true,
		StartYear:    2024,
	}

	// After end year, balance should be zero
	balance := mortgage.CalculateRemainingBalance(2060)
	if balance != 0 {
		t.Errorf("After end year, balance should be £0, got £%.2f", balance)
	}
}

// =============================================================================
// Multiple Mortgage Parts Tests
// =============================================================================

func TestMortgage_CombinedPayments(t *testing.T) {
	config := &Config{
		Mortgage: MortgageConfig{
			Parts: []MortgagePartConfig{
				{
					Name:         "Part 1",
					Principal:    200000,
					InterestRate: 0.04,
					TermYears:    25,
					IsRepayment:  true,
					StartYear:    2024,
				},
				{
					Name:         "Part 2",
					Principal:    100000,
					InterestRate: 0.05,
					TermYears:    10,
					IsRepayment:  false, // Interest-only
					StartYear:    2024,
				},
			},
		},
	}

	totalAnnual := config.GetTotalAnnualPayment()

	// Part 1: £1055.67/month × 12 = £12,668.04/year
	// Part 2: £100000 × 0.05 / 12 × 12 = £5,000/year
	// Total: ~£17,668/year

	expected := (1055.67 * 12) + 5000.0 // ~17668

	if math.Abs(totalAnnual-expected) > 10.0 {
		t.Errorf("Combined annual payment expected ~£%.0f, got £%.2f",
			expected, totalAnnual)
	}
}

func TestMortgage_CombinedPayoffAmount(t *testing.T) {
	config := &Config{
		Mortgage: MortgageConfig{
			Parts: []MortgagePartConfig{
				{
					Name:         "Part 1",
					Principal:    200000,
					InterestRate: 0.04,
					TermYears:    25,
					IsRepayment:  true,
					StartYear:    2024,
				},
				{
					Name:         "Part 2",
					Principal:    100000,
					InterestRate: 0.05,
					IsRepayment:  false, // Interest-only
					StartYear:    2024,
				},
			},
		},
	}

	// After 5 years
	payoff := config.GetTotalPayoffAmount(2029)

	// Part 1 after 5 years: ~£174,209
	// Part 2 (interest-only): £100,000
	// Total: ~£274,209

	expected := 174209.23 + 100000.0

	if math.Abs(payoff-expected) > 10.0 {
		t.Errorf("Payoff at 2029 expected ~£%.0f, got £%.2f", expected, payoff)
	}
}

// =============================================================================
// Real-World Scenario Tests
// =============================================================================

func TestMortgage_RealWorldScenario_FirstTimeBuyer(t *testing.T) {
	// Typical first-time buyer scenario:
	// £250,000 property, 10% deposit = £225,000 mortgage
	// 4.5% interest rate, 30-year term

	mortgage := MortgagePartConfig{
		Principal:    225000,
		InterestRate: 0.045,
		TermYears:    30,
		IsRepayment:  true,
		StartYear:    2024,
	}

	monthly := mortgage.CalculateMonthlyPayment()

	// Verify monthly payment is reasonable (should be around £1,140)
	if monthly < 1100 || monthly > 1200 {
		t.Errorf("First-time buyer monthly payment seems wrong: £%.2f", monthly)
	}

	// Verify total paid is significantly more than principal (interest)
	totalPaid := monthly * 360 // 30 years × 12 months
	if totalPaid <= mortgage.Principal*1.5 {
		t.Errorf("Expected significant interest over 30 years. Total paid: £%.0f, Principal: £%.0f",
			totalPaid, mortgage.Principal)
	}

	t.Logf("First-time buyer: Monthly=£%.2f, Total paid=£%.0f, Interest=£%.0f",
		monthly, totalPaid, totalPaid-mortgage.Principal)
}

func TestMortgage_RealWorldScenario_Remortgage(t *testing.T) {
	// Remortgage scenario after 5 years:
	// Original: £300k @ 5%, 25 years
	// After 5 years, what's the remaining balance?

	original := MortgagePartConfig{
		Principal:    300000,
		InterestRate: 0.05,
		TermYears:    25,
		IsRepayment:  true,
		StartYear:    2019,
	}

	balanceAfter5Years := original.CalculateRemainingBalance(2024)

	// Should be approximately £262k-£268k
	if balanceAfter5Years < 260000 || balanceAfter5Years > 270000 {
		t.Errorf("Remaining balance after 5 years seems wrong: £%.2f", balanceAfter5Years)
	}

	t.Logf("Remortgage balance after 5 years: £%.2f (paid off £%.2f)",
		balanceAfter5Years, original.Principal-balanceAfter5Years)
}

// =============================================================================
// Property Tests - Mathematical Invariants
// =============================================================================

func TestMortgageInvariant_PaymentCoversInterestInitially(t *testing.T) {
	// Property: Monthly payment must be greater than first month's interest
	// Otherwise principal would never decrease

	mortgage := MortgagePartConfig{
		Principal:    200000,
		InterestRate: 0.04,
		TermYears:    25,
		IsRepayment:  true,
		StartYear:    2024,
	}

	monthlyPayment := mortgage.CalculateMonthlyPayment()
	firstMonthInterest := mortgage.Principal * mortgage.InterestRate / 12

	if monthlyPayment <= firstMonthInterest {
		t.Errorf("Payment £%.2f must exceed first month interest £%.2f",
			monthlyPayment, firstMonthInterest)
	}
}

func TestMortgageInvariant_FullyPaidAtTerm(t *testing.T) {
	// Property: At end of term, balance should be exactly zero

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
			t.Errorf("£%.0f @ %.1f%% for %d years: Balance at end should be £0, got £%.2f",
				tc.principal, tc.rate*100, tc.term, balance)
		}
	}
}

func TestMortgageInvariant_ShorterTermHigherPayment(t *testing.T) {
	// Property: Shorter term = higher monthly payment (same principal and rate)

	principal := 200000.0
	rate := 0.04

	mortgage25 := MortgagePartConfig{
		Principal: principal, InterestRate: rate, TermYears: 25, IsRepayment: true,
	}
	mortgage20 := MortgagePartConfig{
		Principal: principal, InterestRate: rate, TermYears: 20, IsRepayment: true,
	}
	mortgage15 := MortgagePartConfig{
		Principal: principal, InterestRate: rate, TermYears: 15, IsRepayment: true,
	}

	payment25 := mortgage25.CalculateMonthlyPayment()
	payment20 := mortgage20.CalculateMonthlyPayment()
	payment15 := mortgage15.CalculateMonthlyPayment()

	if payment15 <= payment20 || payment20 <= payment25 {
		t.Errorf("Shorter term should have higher payment: 15yr=£%.2f, 20yr=£%.2f, 25yr=£%.2f",
			payment15, payment20, payment25)
	}

	t.Logf("Term comparison: 25yr=£%.2f, 20yr=£%.2f, 15yr=£%.2f",
		payment25, payment20, payment15)
}

// =============================================================================
// Early vs Normal Payoff Analysis Test
// =============================================================================

// TestMortgage_EarlyVsNormalPayoff_FinancialAnalysis verifies the financial
// logic of early vs normal mortgage payoff when investment returns differ
func TestMortgage_EarlyVsNormalPayoff_FinancialAnalysis(t *testing.T) {
	// Scenario: £200k mortgage at 4% over 25 years
	// ISA grows at 7%
	// Early payoff in 2026, Normal payoff in 2049

	mortgageRate := 0.04
	principal := 200000.0
	isaRate := 0.07
	startYear := 2024
	earlyPayoffYear := 2026
	normalPayoffYear := 2049

	mortgage := MortgagePartConfig{
		Principal:    principal,
		InterestRate: mortgageRate,
		TermYears:    25,
		IsRepayment:  true,
		StartYear:    startYear,
	}

	annualPayment := mortgage.CalculateAnnualPayment()
	earlyPayoffAmount := mortgage.CalculateRemainingBalance(earlyPayoffYear)
	normalPayoffAmount := mortgage.CalculateRemainingBalance(normalPayoffYear)

	t.Logf("=== Mortgage Details ===")
	t.Logf("Principal: £%.0f at %.2f%%", principal, mortgageRate*100)
	t.Logf("Annual Payment: £%.2f", annualPayment)
	t.Logf("Early Payoff (%d): £%.2f remaining", earlyPayoffYear, earlyPayoffAmount)
	t.Logf("Normal Payoff (%d): £%.2f remaining", normalPayoffYear, normalPayoffAmount)

	// Calculate total mortgage cost for each scenario
	yearsToEarly := earlyPayoffYear - startYear
	yearsToNormal := normalPayoffYear - startYear

	totalMortgageEarly := float64(yearsToEarly)*annualPayment + earlyPayoffAmount
	totalMortgageNormal := float64(yearsToNormal)*annualPayment + normalPayoffAmount

	t.Logf("\n=== Total Mortgage Payments ===")
	t.Logf("Early: %d years × £%.0f + £%.0f payoff = £%.0f",
		yearsToEarly, annualPayment, earlyPayoffAmount, totalMortgageEarly)
	t.Logf("Normal: %d years × £%.0f + £%.0f payoff = £%.0f",
		yearsToNormal, annualPayment, normalPayoffAmount, totalMortgageNormal)
	t.Logf("Difference: £%.0f more with Normal", totalMortgageNormal-totalMortgageEarly)

	// Now calculate the opportunity cost of early payoff
	// If you pay off early, you lose the investment growth on that lump sum
	yearsAfterEarlyPayoff := normalPayoffYear - earlyPayoffYear
	lumpSumGrowth := earlyPayoffAmount * (math.Pow(1+isaRate, float64(yearsAfterEarlyPayoff)) - 1)

	t.Logf("\n=== Investment Opportunity Cost ===")
	t.Logf("Early payoff amount: £%.0f", earlyPayoffAmount)
	t.Logf("If invested at %.0f%% for %d years: £%.0f growth",
		isaRate*100, yearsAfterEarlyPayoff, lumpSumGrowth)

	// But with normal payoff, you're also making extra annual payments
	// which could have been invested instead
	// Calculate the compound value of NOT making those extra payments
	extraPaymentValue := 0.0
	for y := 0; y < yearsAfterEarlyPayoff; y++ {
		// Each saved payment compounds for remaining years
		yearsToCompound := yearsAfterEarlyPayoff - y - 1
		extraPaymentValue += annualPayment * math.Pow(1+isaRate, float64(yearsToCompound))
	}

	t.Logf("\n=== Saved Payments Value (Early Payoff) ===")
	t.Logf("Saved %d payments of £%.0f, compounded at %.0f%%: £%.0f",
		yearsAfterEarlyPayoff, annualPayment, isaRate*100, extraPaymentValue)

	// Net comparison
	// Early payoff: -lumpSum + savedPaymentsCompounded
	// Normal payoff: 0 (baseline)
	earlyNetBenefit := extraPaymentValue - lumpSumGrowth

	t.Logf("\n=== Financial Comparison ===")
	t.Logf("Investment growth lost (early payoff): £%.0f", lumpSumGrowth)
	t.Logf("Compounded saved payments (early payoff): £%.0f", extraPaymentValue)
	t.Logf("Net financial benefit of EARLY payoff: £%.0f", earlyNetBenefit)

	if earlyNetBenefit > 0 {
		t.Logf("CONCLUSION: Early payoff is financially better by £%.0f", earlyNetBenefit)
	} else {
		t.Logf("CONCLUSION: Normal payoff is financially better by £%.0f", -earlyNetBenefit)
	}

	// Additional insight: the crossover rate
	// At what ISA rate does early payoff become beneficial?
	t.Logf("\n=== Crossover Analysis ===")
	for testRate := 0.03; testRate <= 0.10; testRate += 0.01 {
		growth := earlyPayoffAmount * (math.Pow(1+testRate, float64(yearsAfterEarlyPayoff)) - 1)
		savedValue := 0.0
		for y := 0; y < yearsAfterEarlyPayoff; y++ {
			savedValue += annualPayment * math.Pow(1+testRate, float64(yearsAfterEarlyPayoff-y-1))
		}
		benefit := savedValue - growth
		status := "Early better"
		if benefit < 0 {
			status = "Normal better"
		}
		t.Logf("At %.0f%%: %s by £%.0f", testRate*100, status, math.Abs(benefit))
	}

	// Verify the annual payment calculation is correct
	// Monthly payment formula: M = P * [r(1+r)^n] / [(1+r)^n - 1]
	monthlyRate := mortgageRate / 12
	numPayments := float64(25 * 12)
	expectedMonthly := principal * (monthlyRate * math.Pow(1+monthlyRate, numPayments)) / (math.Pow(1+monthlyRate, numPayments) - 1)
	actualMonthly := mortgage.CalculateMonthlyPayment()

	if math.Abs(expectedMonthly-actualMonthly) > 1.0 {
		t.Errorf("Monthly payment calculation error: expected £%.2f, got £%.2f",
			expectedMonthly, actualMonthly)
	}
}
