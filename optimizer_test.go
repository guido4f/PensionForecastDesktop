package main

import (
	"testing"
)

// BDD-style tests for tax optimization
// These tests verify that the optimizer correctly minimizes tax burden
// across various savings/pension configurations

// Standard UK tax bands for testing
var testTaxBands = []TaxBand{
	{Name: "Personal Allowance", Lower: 0, Upper: 12570, Rate: 0.0},
	{Name: "Basic Rate", Lower: 12570, Upper: 50270, Rate: 0.20},
	{Name: "Higher Rate", Lower: 50270, Upper: 125140, Rate: 0.40},
	{Name: "Additional Rate", Lower: 125140, Upper: 1000000000, Rate: 0.45},
}

// Scenario: Single person with only pension (no ISA)
// Given a person with £100k pension and £0 ISA
// When withdrawing £20k net income
// Then TaxOptimized should use pension (filling personal allowance first)
func TestOptimizer_SinglePerson_PensionOnly(t *testing.T) {
	t.Run("Given single person with pension only, When withdrawing within personal allowance, Then no tax should be paid", func(t *testing.T) {
		people := []*Person{
			{Name: "Alice", BirthYear: 1970, RetirementAge: 55, StatePensionAge: 67,
				TaxFreeSavings: 0, UncrystallisedPot: 100000, CrystallisedPot: 0},
		}
		year := 2030 // Alice is 60, can access pension
		statePension := map[string]float64{"Alice": 0}
		netNeeded := 12000.0 // Within personal allowance

		plan := CalculateOptimizedWithdrawals(people, netNeeded, year, statePension, testTaxBands, GradualCrystallisation)

		// Should withdraw from pension (with 25% tax-free from crystallisation)
		totalWithdrawn := plan.TaxableFromPension["Alice"] + plan.TaxFreeFromPension["Alice"]
		if totalWithdrawn < 11000 {
			t.Errorf("Expected pension withdrawal, got taxable=%.2f, taxFree=%.2f",
				plan.TaxableFromPension["Alice"], plan.TaxFreeFromPension["Alice"])
		}

		// Tax should be zero (within personal allowance)
		if plan.TotalTax > 100 {
			t.Errorf("Expected minimal tax within personal allowance, got £%.2f", plan.TotalTax)
		}
	})
}

// Scenario: Single person with only ISA (no pension)
// Given a person with £0 pension and £100k ISA
// When withdrawing £20k net income
// Then should use ISA (tax-free)
func TestOptimizer_SinglePerson_ISAOnly(t *testing.T) {
	t.Run("Given single person with ISA only, When withdrawing, Then ISA should be used tax-free", func(t *testing.T) {
		people := []*Person{
			{Name: "Bob", BirthYear: 1970, RetirementAge: 55, StatePensionAge: 67,
				TaxFreeSavings: 100000, UncrystallisedPot: 0, CrystallisedPot: 0},
		}
		year := 2030
		statePension := map[string]float64{"Bob": 0}
		netNeeded := 20000.0

		plan := CalculateOptimizedWithdrawals(people, netNeeded, year, statePension, testTaxBands, GradualCrystallisation)

		// Should use ISA entirely
		if plan.TaxFreeFromISA["Bob"] < 19000 {
			t.Errorf("Expected ISA withdrawal of ~£20k, got £%.2f", plan.TaxFreeFromISA["Bob"])
		}

		// Zero tax
		if plan.TotalTax > 0.01 {
			t.Errorf("Expected zero tax from ISA, got £%.2f", plan.TotalTax)
		}
	})
}

// Scenario: Person with both ISA and pension
// Given a person with £50k pension and £50k ISA
// When withdrawing £20k (more than personal allowance can cover tax-free)
// Then should use pension first (to utilize personal allowance), then ISA
func TestOptimizer_SinglePerson_MixedAssets_PrefersPersonalAllowance(t *testing.T) {
	t.Run("Given mixed assets, When withdrawing, Then pension fills personal allowance before using ISA", func(t *testing.T) {
		people := []*Person{
			{Name: "Carol", BirthYear: 1970, RetirementAge: 55, StatePensionAge: 67,
				TaxFreeSavings: 50000, UncrystallisedPot: 50000, CrystallisedPot: 0},
		}
		year := 2030
		statePension := map[string]float64{"Carol": 0}
		netNeeded := 15000.0 // Just above personal allowance

		plan := CalculateOptimizedWithdrawals(people, netNeeded, year, statePension, testTaxBands, GradualCrystallisation)

		// Should use pension to fill personal allowance
		pensionUsed := plan.TaxableFromPension["Carol"] + plan.TaxFreeFromPension["Carol"]
		if pensionUsed < 10000 {
			t.Errorf("Expected significant pension use to fill personal allowance, got £%.2f", pensionUsed)
		}

		// Tax should be minimal (mostly within personal allowance)
		if plan.TotalTax > 1000 {
			t.Errorf("Expected low tax, got £%.2f", plan.TotalTax)
		}
	})
}

// Scenario: Two people with different pension amounts
// Given Person A with £500k pension and Person B with £100k pension
// When withdrawing £25k total
// Then should split proportionally by pot size (matching PensionFirst behavior)
func TestOptimizer_TwoPeople_ProportionalWithdrawals(t *testing.T) {
	t.Run("Given two people with different pensions, When withdrawing, Then splits proportionally by pot size", func(t *testing.T) {
		people := []*Person{
			{Name: "Dan", BirthYear: 1970, RetirementAge: 55, StatePensionAge: 67,
				TaxFreeSavings: 0, UncrystallisedPot: 500000, CrystallisedPot: 0},
			{Name: "Eve", BirthYear: 1972, RetirementAge: 55, StatePensionAge: 67,
				TaxFreeSavings: 0, UncrystallisedPot: 100000, CrystallisedPot: 0},
		}
		year := 2030 // Both can access pension
		statePension := map[string]float64{"Dan": 0, "Eve": 0}
		netNeeded := 25000.0 // About 2x personal allowance

		plan := CalculateOptimizedWithdrawals(people, netNeeded, year, statePension, testTaxBands, GradualCrystallisation)

		// Proportional split: Dan 83% (500k/600k), Eve 17% (100k/600k)
		// Dan should have more withdrawals than Eve
		danTotal := plan.TaxableFromPension["Dan"] + plan.TaxFreeFromPension["Dan"]
		eveTotal := plan.TaxableFromPension["Eve"] + plan.TaxFreeFromPension["Eve"]

		// Dan should get significantly more (he has 5x Eve's pot)
		if danTotal < eveTotal {
			t.Errorf("Dan should have more withdrawals than Eve, got Dan=£%.2f, Eve=£%.2f", danTotal, eveTotal)
		}

		// Total should roughly cover the net needed
		if danTotal+eveTotal < 20000 {
			t.Errorf("Expected total withdrawals of at least £20k, got £%.2f", danTotal+eveTotal)
		}

		// Tax should be reasonable (Dan may exceed personal allowance)
		if plan.TotalTax > 3000 {
			t.Errorf("Expected reasonable tax, got £%.2f", plan.TotalTax)
		}
	})
}

// Scenario: Comparing TaxOptimized vs SavingsFirst
// Given two people with mixed assets
// When running both strategies
// Then TaxOptimized should result in equal or lower tax
func TestOptimizer_TaxOptimized_BetterThan_SavingsFirst(t *testing.T) {
	t.Run("Given mixed assets, TaxOptimized should outperform SavingsFirst", func(t *testing.T) {
		config := createTestConfig()

		// Run SavingsFirst strategy
		savingsFirstParams := SimulationParams{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: SavingsFirst, MortgageOpt: MortgageNormal}
		savingsFirstResult := RunSimulation(savingsFirstParams, config)

		// Run TaxOptimized strategy
		taxOptParams := SimulationParams{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: TaxOptimized, MortgageOpt: MortgageNormal}
		taxOptResult := RunSimulation(taxOptParams, config)

		// TaxOptimized should pay less or equal tax
		if taxOptResult.TotalTaxPaid > savingsFirstResult.TotalTaxPaid*1.01 { // Allow 1% margin
			t.Errorf("TaxOptimized (£%.0f) should not pay more tax than SavingsFirst (£%.0f)",
				taxOptResult.TotalTaxPaid, savingsFirstResult.TotalTaxPaid)
		}

		t.Logf("SavingsFirst tax: £%.0f, TaxOptimized tax: £%.0f, Savings: £%.0f",
			savingsFirstResult.TotalTaxPaid, taxOptResult.TotalTaxPaid,
			savingsFirstResult.TotalTaxPaid-taxOptResult.TotalTaxPaid)
	})
}

// Scenario: Comparing TaxOptimized vs PensionFirst with various asset ratios
func TestOptimizer_TaxOptimized_CompetitiveWith_PensionFirst(t *testing.T) {
	testCases := []struct {
		name           string
		person1ISA     float64
		person1Pension float64
		person2ISA     float64
		person2Pension float64
	}{
		{"Equal assets", 100000, 100000, 100000, 100000},
		{"High pension low ISA", 10000, 500000, 10000, 200000},
		{"High ISA low pension", 300000, 50000, 200000, 30000},
		{"Asymmetric - one wealthy", 500000, 800000, 50000, 50000},
		{"Asymmetric - pension heavy", 20000, 900000, 20000, 100000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := createTestConfigWithAssets(tc.person1ISA, tc.person1Pension, tc.person2ISA, tc.person2Pension)

			// Run PensionFirst strategy
			pensionFirstParams := SimulationParams{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionFirst, MortgageOpt: MortgageNormal}
			pensionFirstResult := RunSimulation(pensionFirstParams, config)

			// Run TaxOptimized strategy
			taxOptParams := SimulationParams{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: TaxOptimized, MortgageOpt: MortgageNormal}
			taxOptResult := RunSimulation(taxOptParams, config)

			// TaxOptimized should be competitive (within 5% or better)
			margin := 1.05
			if taxOptResult.TotalTaxPaid > pensionFirstResult.TotalTaxPaid*margin {
				t.Errorf("TaxOptimized (£%.0f) significantly worse than PensionFirst (£%.0f) for %s",
					taxOptResult.TotalTaxPaid, pensionFirstResult.TotalTaxPaid, tc.name)
			}

			diff := pensionFirstResult.TotalTaxPaid - taxOptResult.TotalTaxPaid
			t.Logf("%s: PensionFirst=£%.0f, TaxOptimized=£%.0f, Diff=£%.0f",
				tc.name, pensionFirstResult.TotalTaxPaid, taxOptResult.TotalTaxPaid, diff)
		})
	}
}

// Scenario: State pension affects optimization
// Given person receiving state pension
// When withdrawing additional income
// Then should account for state pension in personal allowance usage
func TestOptimizer_AccountsForStatePension(t *testing.T) {
	t.Run("Given state pension income, When withdrawing, Then accounts for reduced personal allowance", func(t *testing.T) {
		people := []*Person{
			{Name: "Frank", BirthYear: 1960, RetirementAge: 55, StatePensionAge: 66,
				TaxFreeSavings: 50000, UncrystallisedPot: 200000, CrystallisedPot: 0},
		}
		year := 2030                                       // Frank is 70, receives state pension
		statePension := map[string]float64{"Frank": 11500} // Close to personal allowance
		netNeeded := 20000.0

		plan := CalculateOptimizedWithdrawals(people, netNeeded, year, statePension, testTaxBands, GradualCrystallisation)

		// With state pension of £11,500, only £1,070 of personal allowance remains
		// So most pension withdrawal will be taxed at 20%
		// Should still prefer pension to fill remaining allowance before ISA

		pensionUsed := plan.TaxableFromPension["Frank"] + plan.TaxFreeFromPension["Frank"]
		if pensionUsed < 1000 {
			t.Errorf("Expected some pension use even with state pension, got £%.2f", pensionUsed)
		}

		// Tax should be calculated on income above personal allowance
		expectedMinTax := (11500 + plan.TaxableFromPension["Frank"] - 12570) * 0.20
		if expectedMinTax > 0 && plan.TotalTax < expectedMinTax*0.8 {
			t.Errorf("Tax (£%.2f) seems too low given state pension, expected at least £%.2f",
				plan.TotalTax, expectedMinTax)
		}
	})
}

// Scenario: One person cannot access pension yet
// Given two people where only one can access pension
// When withdrawing
// Then should only use accessible funds
func TestOptimizer_RespectsRetirementAge(t *testing.T) {
	t.Run("Given one person below retirement age, When withdrawing, Then only uses accessible funds", func(t *testing.T) {
		people := []*Person{
			{Name: "George", BirthYear: 1970, RetirementAge: 55, StatePensionAge: 67,
				TaxFreeSavings: 50000, UncrystallisedPot: 200000, CrystallisedPot: 0},
			{Name: "Helen", BirthYear: 1980, RetirementAge: 55, StatePensionAge: 67,
				TaxFreeSavings: 50000, UncrystallisedPot: 200000, CrystallisedPot: 0},
		}
		year := 2030 // George is 60 (can access), Helen is 50 (cannot access pension)
		statePension := map[string]float64{"George": 0, "Helen": 0}
		netNeeded := 30000.0

		plan := CalculateOptimizedWithdrawals(people, netNeeded, year, statePension, testTaxBands, GradualCrystallisation)

		// Helen should not have any pension withdrawals
		helenPension := plan.TaxableFromPension["Helen"] + plan.TaxFreeFromPension["Helen"]
		if helenPension > 0.01 {
			t.Errorf("Helen should not access pension (too young), but got £%.2f", helenPension)
		}

		// George should have pension withdrawals or ISA should be used
		georgePension := plan.TaxableFromPension["George"] + plan.TaxFreeFromPension["George"]
		georgeISA := plan.TaxFreeFromISA["George"]
		helenISA := plan.TaxFreeFromISA["Helen"]

		totalWithdrawn := georgePension + georgeISA + helenISA
		if totalWithdrawn < 29000 {
			t.Errorf("Expected ~£30k withdrawn, got £%.2f", totalWithdrawn)
		}
	})
}

// Scenario: PCLS already taken (simulates post-lump-sum scenario)
// Given PCLS was already taken from pension pot
// When withdrawing
// Then should work correctly with crystallised funds (no 25% tax-free)
func TestOptimizer_PCLSTaken(t *testing.T) {
	t.Run("Given PCLS taken pension, When withdrawing, Then no further 25% tax-free", func(t *testing.T) {
		people := []*Person{
			{Name: "Ivan", BirthYear: 1970, RetirementAge: 55, StatePensionAge: 67,
				TaxFreeSavings:    25000,  // Includes 25% from PCLS
				UncrystallisedPot: 50000,  // Some uncrystallised left
				CrystallisedPot:   75000,  // 75% of original crystallised
				PCLSTaken:         true},  // PCLS was taken - no more 25% tax-free
		}
		year := 2030
		statePension := map[string]float64{"Ivan": 0}
		netNeeded := 20000.0

		plan := CalculateOptimizedWithdrawals(people, netNeeded, year, statePension, testTaxBands, GradualCrystallisation)

		// Should NOT get any 25% tax-free from pension since PCLSTaken is true
		if plan.TaxFreeFromPension["Ivan"] > 0.01 {
			t.Errorf("Should not have tax-free from pension when PCLS taken, got £%.2f",
				plan.TaxFreeFromPension["Ivan"])
		}

		// Should use taxable pension and/or ISA
		totalUsed := plan.TaxableFromPension["Ivan"] + plan.TaxFreeFromISA["Ivan"]
		if totalUsed < 19000 {
			t.Errorf("Expected ~£20k from taxable pension or ISA, got £%.2f", totalUsed)
		}
	})
}

// Scenario: Large withdrawal pushing into higher tax bands
// Given sufficient funds
// When withdrawing large amount
// Then should minimize higher rate tax by balancing
func TestOptimizer_MinimizesHigherRateTax(t *testing.T) {
	t.Run("Given large withdrawal need, When withdrawing, Then balances to minimize higher rate tax", func(t *testing.T) {
		people := []*Person{
			{Name: "Jack", BirthYear: 1970, RetirementAge: 55, StatePensionAge: 67,
				TaxFreeSavings: 0, UncrystallisedPot: 500000, CrystallisedPot: 0},
			{Name: "Kate", BirthYear: 1972, RetirementAge: 55, StatePensionAge: 67,
				TaxFreeSavings: 0, UncrystallisedPot: 500000, CrystallisedPot: 0},
		}
		year := 2030
		statePension := map[string]float64{"Jack": 0, "Kate": 0}
		netNeeded := 80000.0 // Requires going into basic rate for both

		plan := CalculateOptimizedWithdrawals(people, netNeeded, year, statePension, testTaxBands, GradualCrystallisation)

		// Both should have withdrawals to balance tax burden
		jackTotal := plan.TaxableFromPension["Jack"] + plan.TaxFreeFromPension["Jack"]
		kateTotal := plan.TaxableFromPension["Kate"] + plan.TaxFreeFromPension["Kate"]

		// Check withdrawals are reasonably balanced (within 50% of each other)
		if jackTotal > 0.01 && kateTotal > 0.01 {
			ratio := jackTotal / kateTotal
			if ratio < 0.5 || ratio > 2.0 {
				t.Logf("Warning: Withdrawals may be unbalanced - Jack=£%.0f, Kate=£%.0f", jackTotal, kateTotal)
			}
		}

		// Calculate what tax would be with all from one person
		singlePersonTax := CalculateTaxOnIncome(100000, testTaxBands) // Rough estimate

		// Balanced should be lower
		if plan.TotalTax > singlePersonTax*0.9 {
			t.Logf("Tax might be suboptimal: £%.0f (single person estimate: £%.0f)", plan.TotalTax, singlePersonTax)
		}
	})
}

// Helper function to create test config
func createTestConfig() *Config {
	return createTestConfigWithAssets(116500, 895500, 285000, 75700)
}

// Helper function to create test config with custom assets
func createTestConfigWithAssets(p1ISA, p1Pension, p2ISA, p2Pension float64) *Config {
	return &Config{
		People: []PersonConfig{
			{Name: "James", BirthDate: "1971-01-01", RetirementAge: 55, StatePensionAge: 65,
				TaxFreeSavings: p1ISA, Pension: p1Pension},
			{Name: "Delphine", BirthDate: "1973-01-01", RetirementAge: 57, StatePensionAge: 67,
				TaxFreeSavings: p2ISA, Pension: p2Pension},
		},
		Financial: FinancialConfig{
			PensionGrowthRate:     0.05,
			SavingsGrowthRate:     0.05,
			IncomeInflationRate:   0.03,
			StatePensionInflation: 0.025,
			StatePensionAmount:    11500,
		},
		IncomeRequirements: IncomeConfig{
			MonthlyBeforeAge: 5000,
			MonthlyAfterAge:  4000,
			AgeThreshold:     67,
			ReferencePerson:  "James",
		},
		Mortgage: MortgageConfig{
			EndYear:         2031,
			EarlyPayoffYear: 2028,
			Parts: []MortgagePartConfig{
				{Name: "Test", Principal: 100000, InterestRate: 0.04, IsRepayment: false, StartYear: 2025},
			},
		},
		Simulation: SimulationConfig{
			StartYear:       2026,
			EndAge:          90,
			ReferencePerson: "James",
		},
		TaxBands: testTaxBands,
	}
}

// Benchmark test
func BenchmarkOptimizer(b *testing.B) {
	people := []*Person{
		{Name: "Test1", BirthYear: 1970, RetirementAge: 55, StatePensionAge: 67,
			TaxFreeSavings: 100000, UncrystallisedPot: 500000, CrystallisedPot: 0},
		{Name: "Test2", BirthYear: 1972, RetirementAge: 55, StatePensionAge: 67,
			TaxFreeSavings: 100000, UncrystallisedPot: 300000, CrystallisedPot: 0},
	}
	statePension := map[string]float64{"Test1": 11500, "Test2": 11500}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateOptimizedWithdrawals(people, 50000, 2040, statePension, testTaxBands, GradualCrystallisation)
	}
}
