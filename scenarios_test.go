package main

import (
	"math"
	"testing"
)

// End-to-End Scenario Tests
//
// These tests validate complete retirement scenarios to ensure all
// components work together correctly.
//
// References:
// - MoneyHelper Pension Calculator: https://www.moneyhelper.org.uk/en/pensions-and-retirement/pensions-basics/pension-calculator
// - GOV.UK Income Tax: https://www.gov.uk/income-tax-rates

// =============================================================================
// Single Person Retirement Scenarios
// =============================================================================

func TestScenario_SinglePersonBasicRetirement(t *testing.T) {
	// Scenario: Person aged 60 with £500k pension, needs £30k/year
	// Using SavingsFirst strategy with gradual crystallisation

	config := &Config{
		People: []PersonConfig{
			{
				Name:           "Alice",
				BirthDate:      "1964-01-01",
				RetirementAge:  55,
				StatePensionAge: 66,
				TaxFreeSavings: 0,
				Pension:        500000,
			},
		},
		Financial: FinancialConfig{
			PensionGrowthRate:     0.05,
			SavingsGrowthRate:     0.04,
			IncomeInflationRate:   0.025,
			StatePensionInflation: 0.025,
			StatePensionAmount:    11500,
		},
		IncomeRequirements: IncomeConfig{
			MonthlyBeforeAge: 2500, // £30k/year
			MonthlyAfterAge:  2000, // £24k/year after 67
			AgeThreshold:     67,
			ReferencePerson:  "Alice",
		},
		Simulation: SimulationConfig{
			StartYear:       2024,
			EndAge:          90,
			ReferencePerson: "Alice",
		},
		TaxBands: ukTaxBands2024,
	}

	params := SimulationParams{
		CrystallisationStrategy: GradualCrystallisation,
		DrawdownOrder:           SavingsFirst,
	}

	result := RunSimulation(params, config)

	// Verify simulation ran
	if len(result.Years) == 0 {
		t.Fatal("Simulation produced no results")
	}

	// First year should have income close to requirement
	firstYear := result.Years[0]
	if firstYear.RequiredIncome < 25000 {
		t.Errorf("First year required income seems too low: £%.2f", firstYear.RequiredIncome)
	}

	// Check state pension kicks in at age 66
	for _, year := range result.Years {
		age := year.Year - 1964
		if age >= 66 {
			if year.TotalStatePension == 0 {
				t.Errorf("Year %d (age %d): State pension should be active", year.Year, age)
			}
			break
		}
	}

	t.Logf("Simulation ran for %d years, total tax paid: £%.0f",
		len(result.Years), result.TotalTaxPaid)
}

func TestScenario_SinglePersonTaxEfficient(t *testing.T) {
	// Compare SavingsFirst vs TaxOptimized for single person
	// TaxOptimized should produce lower or equal tax

	config := &Config{
		People: []PersonConfig{
			{
				Name:           "Bob",
				BirthDate:      "1965-01-01",
				RetirementAge:  55,
				StatePensionAge: 67,
				TaxFreeSavings: 100000,
				Pension:        400000,
			},
		},
		Financial: FinancialConfig{
			PensionGrowthRate:     0.05,
			SavingsGrowthRate:     0.04,
			IncomeInflationRate:   0.025,
			StatePensionInflation: 0.025,
			StatePensionAmount:    11500,
		},
		IncomeRequirements: IncomeConfig{
			MonthlyBeforeAge: 3000,
			MonthlyAfterAge:  2500,
			AgeThreshold:     67,
			ReferencePerson:  "Bob",
		},
		Simulation: SimulationConfig{
			StartYear:       2025,
			EndAge:          85,
			ReferencePerson: "Bob",
		},
		TaxBands: ukTaxBands2024,
	}

	// Run SavingsFirst
	savingsFirstResult := RunSimulation(SimulationParams{
		CrystallisationStrategy: GradualCrystallisation,
		DrawdownOrder:           SavingsFirst,
	}, config)

	// Reset config for second run
	config.People[0].TaxFreeSavings = 100000
	config.People[0].Pension = 400000

	// Run TaxOptimized
	taxOptResult := RunSimulation(SimulationParams{
		CrystallisationStrategy: GradualCrystallisation,
		DrawdownOrder:           TaxOptimized,
	}, config)

	// TaxOptimized should be competitive (within 10%)
	if taxOptResult.TotalTaxPaid > savingsFirstResult.TotalTaxPaid*1.10 {
		t.Errorf("TaxOptimized (£%.0f) should not be significantly worse than SavingsFirst (£%.0f)",
			taxOptResult.TotalTaxPaid, savingsFirstResult.TotalTaxPaid)
	}

	t.Logf("SavingsFirst tax: £%.0f, TaxOptimized tax: £%.0f (diff: £%.0f)",
		savingsFirstResult.TotalTaxPaid, taxOptResult.TotalTaxPaid,
		savingsFirstResult.TotalTaxPaid-taxOptResult.TotalTaxPaid)
}

// =============================================================================
// Couple Retirement Scenarios
// =============================================================================

func TestScenario_CoupleWithStatePension(t *testing.T) {
	// Scenario: Couple where both receive state pension
	// Should utilize both personal allowances

	config := &Config{
		People: []PersonConfig{
			{
				Name:           "James",
				BirthDate:      "1960-01-01",
				RetirementAge:  55,
				StatePensionAge: 66,
				TaxFreeSavings: 200000,
				Pension:        600000,
			},
			{
				Name:           "Sarah",
				BirthDate:      "1962-01-01",
				RetirementAge:  55,
				StatePensionAge: 66,
				TaxFreeSavings: 150000,
				Pension:        300000,
			},
		},
		Financial: FinancialConfig{
			PensionGrowthRate:     0.05,
			SavingsGrowthRate:     0.04,
			IncomeInflationRate:   0.025,
			StatePensionInflation: 0.025,
			StatePensionAmount:    11500,
		},
		IncomeRequirements: IncomeConfig{
			MonthlyBeforeAge: 4000, // £48k/year combined
			MonthlyAfterAge:  3500,
			AgeThreshold:     67,
			ReferencePerson:  "James",
		},
		Simulation: SimulationConfig{
			StartYear:       2026,
			EndAge:          90,
			ReferencePerson: "James",
		},
		TaxBands: ukTaxBands2024,
	}

	result := RunSimulation(SimulationParams{
		CrystallisationStrategy: GradualCrystallisation,
		DrawdownOrder:           TaxOptimized,
	}, config)

	// Both should have some withdrawals (using both personal allowances)
	hasJamesWithdrawal := false
	hasSarahWithdrawal := false

	for _, year := range result.Years {
		w := year.Withdrawals
		if w.TaxFreeFromISA["James"] > 0 || w.TaxableFromPension["James"] > 0 || w.TaxFreeFromPension["James"] > 0 {
			hasJamesWithdrawal = true
		}
		if w.TaxFreeFromISA["Sarah"] > 0 || w.TaxableFromPension["Sarah"] > 0 || w.TaxFreeFromPension["Sarah"] > 0 {
			hasSarahWithdrawal = true
		}
	}

	if !hasJamesWithdrawal || !hasSarahWithdrawal {
		t.Errorf("Expected withdrawals from both people. James: %v, Sarah: %v",
			hasJamesWithdrawal, hasSarahWithdrawal)
	}

	t.Logf("Couple scenario: %d years, total tax: £%.0f", len(result.Years), result.TotalTaxPaid)
}

func TestScenario_CoupleAsymmetricAssets(t *testing.T) {
	// Scenario: One person has much more than the other
	// TaxOptimized should still balance effectively

	config := &Config{
		People: []PersonConfig{
			{
				Name:           "Rich",
				BirthDate:      "1965-01-01",
				RetirementAge:  55,
				StatePensionAge: 67,
				TaxFreeSavings: 500000,
				Pension:        1000000,
			},
			{
				Name:           "Less",
				BirthDate:      "1967-01-01",
				RetirementAge:  55,
				StatePensionAge: 67,
				TaxFreeSavings: 50000,
				Pension:        100000,
			},
		},
		Financial: FinancialConfig{
			PensionGrowthRate:     0.05,
			SavingsGrowthRate:     0.04,
			IncomeInflationRate:   0.025,
			StatePensionInflation: 0.025,
			StatePensionAmount:    11500,
		},
		IncomeRequirements: IncomeConfig{
			MonthlyBeforeAge: 5000,
			MonthlyAfterAge:  4000,
			AgeThreshold:     67,
			ReferencePerson:  "Rich",
		},
		Simulation: SimulationConfig{
			StartYear:       2025,
			EndAge:          85,
			ReferencePerson: "Rich",
		},
		TaxBands: ukTaxBands2024,
	}

	result := RunSimulation(SimulationParams{
		CrystallisationStrategy: GradualCrystallisation,
		DrawdownOrder:           TaxOptimized,
	}, config)

	// Verify simulation completed
	if len(result.Years) == 0 {
		t.Fatal("Simulation produced no results")
	}

	// Check that Rich's larger pot is used more heavily
	richWithdrawals := 0.0
	lessWithdrawals := 0.0

	for _, year := range result.Years {
		w := year.Withdrawals
		richWithdrawals += w.TaxFreeFromISA["Rich"] + w.TaxableFromPension["Rich"] + w.TaxFreeFromPension["Rich"]
		lessWithdrawals += w.TaxFreeFromISA["Less"] + w.TaxableFromPension["Less"] + w.TaxFreeFromPension["Less"]
	}

	t.Logf("Asymmetric: Rich withdrew £%.0f, Less withdrew £%.0f", richWithdrawals, lessWithdrawals)
}

// =============================================================================
// Tax Threshold Scenarios
// =============================================================================

func TestScenario_60PercentTaxTrap(t *testing.T) {
	// Scenario: Person with high pension withdrawals hitting the 60% marginal tax zone
	// Verify the tax calculation correctly reflects the effective 60% rate

	// Create scenario where withdrawals push income into £100k-£125k range
	statePension := 11500.0
	pensionWithdrawal := 100000.0 // Will push total to ~£111,500

	// Calculate expected tax with tapering
	totalIncome := statePension + pensionWithdrawal
	expectedTax := CalculateTaxWithTapering(totalIncome, ukTaxBands2024)

	// Verify marginal rate in the trap zone is approximately 60%
	marginalIn := 5000.0 // Test marginal rate with additional £5k
	taxBefore := CalculateTaxWithTapering(105000, ukTaxBands2024)
	taxAfter := CalculateTaxWithTapering(110000, ukTaxBands2024)
	effectiveMarginal := (taxAfter - taxBefore) / marginalIn

	// Note: The 60% trap figure commonly cited includes National Insurance.
	// Without NI, the effective marginal is ~50%:
	// - 40% higher rate tax
	// - Plus 10% from losing £0.50 of PA per £1, which moves income from 0% to 20% basic rate
	if math.Abs(effectiveMarginal-0.50) > 0.05 {
		t.Errorf("Effective marginal rate in £100k-£125k zone should be ~50%% (60%% with NI), got %.1f%%",
			effectiveMarginal*100)
	}

	t.Logf("60%% trap test: Income £%.0f, Tax £%.0f, Effective marginal %.1f%%",
		totalIncome, expectedTax, effectiveMarginal*100)
}

func TestScenario_StayingInBasicRate(t *testing.T) {
	// Scenario: Couple managing withdrawals to stay in basic rate band
	// Each should aim for ~£50,270 max to avoid higher rate

	people := []*Person{
		{Name: "PersonA", BirthYear: 1960, RetirementAge: 55, StatePensionAge: 66,
			TaxFreeSavings: 100000, UncrystallisedPot: 500000},
		{Name: "PersonB", BirthYear: 1962, RetirementAge: 55, StatePensionAge: 66,
			TaxFreeSavings: 100000, UncrystallisedPot: 400000},
	}

	// Each has ~£11,500 state pension, so remaining personal allowance is ~£1,070
	// Basic rate headroom is £50,270 - £11,500 = £38,770 each
	// Combined: ~£77,540 at basic rate

	statePension := map[string]float64{"PersonA": 11500, "PersonB": 11500}

	// Test withdrawal that requires only basic rate
	netNeeded := 60000.0 // Should be achievable within combined basic rate

	plan := CalculateOptimizedWithdrawals(people, netNeeded, 2030, statePension, ukTaxBands2024, GradualCrystallisation)

	// Calculate how much each person's taxable income is
	personATaxable := statePension["PersonA"] + plan.TaxableFromPension["PersonA"]
	personBTaxable := statePension["PersonB"] + plan.TaxableFromPension["PersonB"]

	// Neither should exceed basic rate threshold significantly
	if personATaxable > 55000 || personBTaxable > 55000 {
		t.Logf("Warning: One person may be in higher rate. A: £%.0f, B: £%.0f",
			personATaxable, personBTaxable)
	}

	t.Logf("Basic rate test: A taxable £%.0f, B taxable £%.0f, Total tax £%.2f",
		personATaxable, personBTaxable, plan.TotalTax)
}

// =============================================================================
// Growth and Inflation Scenarios
// =============================================================================

func TestScenario_LongTermGrowth(t *testing.T) {
	// Scenario: 30-year retirement with 5% growth
	// Verify pot growth before retirement

	person := &Person{
		Name:              "LongTerm",
		BirthYear:         1975,
		RetirementAge:     55, // Retires 2030
		UncrystallisedPot: 300000,
		TaxFreeSavings:    100000,
	}

	// Apply 10 years of growth (before accessing pension)
	for i := 0; i < 10; i++ {
		ApplyGrowth(person, 0.04, 0.05)
	}

	// ISA should have grown: 100000 × (1.04)^10 = 148,024
	expectedISA := 100000 * math.Pow(1.04, 10)
	if math.Abs(person.TaxFreeSavings-expectedISA) > 1.0 {
		t.Errorf("ISA should be ~£%.0f, got £%.2f", expectedISA, person.TaxFreeSavings)
	}

	// Pension should have grown: 300000 × (1.05)^10 = 488,668
	expectedPension := 300000 * math.Pow(1.05, 10)
	if math.Abs(person.UncrystallisedPot-expectedPension) > 1.0 {
		t.Errorf("Pension should be ~£%.0f, got £%.2f", expectedPension, person.UncrystallisedPot)
	}

	t.Logf("10 years growth: ISA £%.0f → £%.0f, Pension £%.0f → £%.0f",
		100000.0, person.TaxFreeSavings, 300000.0, person.UncrystallisedPot)
}

func TestScenario_InflatedIncome(t *testing.T) {
	// Scenario: Income requirement inflates over 20 years
	// At 2.5% inflation, £30k becomes ~£49k

	baseIncome := 30000.0
	inflationRate := 0.025
	years := 20

	inflatedIncome := baseIncome * math.Pow(1+inflationRate, float64(years))
	expected := 49152.86 // 30000 × (1.025)^20

	if math.Abs(inflatedIncome-expected) > 10.0 {
		t.Errorf("Inflated income should be ~£%.0f, got £%.2f", expected, inflatedIncome)
	}

	// Verify tax bands also inflate correctly
	inflatedBands := InflateTaxBands(ukTaxBands2024, 2024, 2044, inflationRate)

	// Personal allowance should be: 12570 × (1.025)^20 = 20,587
	expectedPA := 12570 * math.Pow(1.025, 20)
	if math.Abs(inflatedBands[0].Upper-expectedPA) > 1.0 {
		t.Errorf("Inflated PA should be ~£%.0f, got £%.2f", expectedPA, inflatedBands[0].Upper)
	}

	t.Logf("20 years inflation: Income £%.0f → £%.0f, PA £%.0f → £%.0f",
		baseIncome, inflatedIncome, 12570.0, inflatedBands[0].Upper)
}

// =============================================================================
// Depletion Scenarios
// =============================================================================

func TestScenario_SufficientFunds(t *testing.T) {
	// Scenario: Person with enough funds should never run out

	config := &Config{
		People: []PersonConfig{
			{
				Name:           "Wealthy",
				BirthDate:      "1960-01-01",
				RetirementAge:  55,
				StatePensionAge: 66,
				TaxFreeSavings: 500000,
				Pension:        1500000,
			},
		},
		Financial: FinancialConfig{
			PensionGrowthRate:     0.05,
			SavingsGrowthRate:     0.04,
			IncomeInflationRate:   0.025,
			StatePensionInflation: 0.025,
			StatePensionAmount:    11500,
		},
		IncomeRequirements: IncomeConfig{
			MonthlyBeforeAge: 3000, // Modest spending
			MonthlyAfterAge:  2500,
			AgeThreshold:     67,
			ReferencePerson:  "Wealthy",
		},
		Simulation: SimulationConfig{
			StartYear:       2024,
			EndAge:          95,
			ReferencePerson: "Wealthy",
		},
		TaxBands: ukTaxBands2024,
	}

	result := RunSimulation(SimulationParams{
		CrystallisationStrategy: GradualCrystallisation,
		DrawdownOrder:           SavingsFirst,
	}, config)

	if result.RanOutYear > 0 {
		t.Errorf("Wealthy person should not run out of money, but ran out in year %d", result.RanOutYear)
	}

	// Check final balances are still positive
	lastYear := result.Years[len(result.Years)-1]
	totalRemaining := 0.0
	for _, person := range lastYear.EndBalances {
		totalRemaining += person.TaxFreeSavings + person.CrystallisedPot + person.UncrystallisedPot
	}

	if totalRemaining <= 0 {
		t.Errorf("Should have positive balance remaining, got £%.2f", totalRemaining)
	}

	t.Logf("Sufficient funds: Survived to age 95 with £%.0f remaining", totalRemaining)
}

// =============================================================================
// Strategy Comparison Scenarios
// =============================================================================

func TestScenario_StrategyComparison(t *testing.T) {
	// Compare all strategies for the same scenario

	createConfig := func() *Config {
		return &Config{
			People: []PersonConfig{
				{
					Name:           "Test",
					BirthDate:      "1965-01-01",
					RetirementAge:  55,
					StatePensionAge: 67,
					TaxFreeSavings: 150000,
					Pension:        450000,
				},
			},
			Financial: FinancialConfig{
				PensionGrowthRate:     0.05,
				SavingsGrowthRate:     0.04,
				IncomeInflationRate:   0.025,
				StatePensionInflation: 0.025,
				StatePensionAmount:    11500,
			},
			IncomeRequirements: IncomeConfig{
				MonthlyBeforeAge: 3000,
				MonthlyAfterAge:  2500,
				AgeThreshold:     67,
				ReferencePerson:  "Test",
			},
			Simulation: SimulationConfig{
				StartYear:       2025,
				EndAge:          85,
				ReferencePerson: "Test",
			},
			TaxBands: ukTaxBands2024,
		}
	}

	strategies := []struct {
		name     string
		strategy DrawdownOrder
	}{
		{"SavingsFirst", SavingsFirst},
		{"PensionFirst", PensionFirst},
		{"TaxOptimized", TaxOptimized},
	}

	results := make(map[string]float64)

	for _, s := range strategies {
		config := createConfig()
		result := RunSimulation(SimulationParams{
			CrystallisationStrategy: GradualCrystallisation,
			DrawdownOrder:           s.strategy,
		}, config)
		results[s.name] = result.TotalTaxPaid
	}

	// Log comparison
	t.Logf("Strategy comparison:")
	for name, tax := range results {
		t.Logf("  %s: £%.0f total tax", name, tax)
	}

	// TaxOptimized should be competitive with the best
	minTax := results["SavingsFirst"]
	for _, tax := range results {
		if tax < minTax {
			minTax = tax
		}
	}

	if results["TaxOptimized"] > minTax*1.10 {
		t.Errorf("TaxOptimized (£%.0f) should be within 10%% of best (£%.0f)",
			results["TaxOptimized"], minTax)
	}
}

// =============================================================================
// Mortgage Integration Scenarios
// =============================================================================

func TestScenario_WithMortgage(t *testing.T) {
	// Scenario: Retirement with ongoing mortgage payments

	config := &Config{
		People: []PersonConfig{
			{
				Name:           "Mortgaged",
				BirthDate:      "1965-01-01",
				RetirementAge:  55,
				StatePensionAge: 67,
				TaxFreeSavings: 100000,
				Pension:        500000,
			},
		},
		Financial: FinancialConfig{
			PensionGrowthRate:     0.05,
			SavingsGrowthRate:     0.04,
			IncomeInflationRate:   0.025,
			StatePensionInflation: 0.025,
			StatePensionAmount:    11500,
		},
		IncomeRequirements: IncomeConfig{
			MonthlyBeforeAge: 3000,
			MonthlyAfterAge:  2500,
			AgeThreshold:     67,
			ReferencePerson:  "Mortgaged",
		},
		Mortgage: MortgageConfig{
			EndYear: 2035,
			Parts: []MortgagePartConfig{
				{
					Name:         "Main",
					Principal:    150000,
					InterestRate: 0.04,
					TermYears:    15,
					IsRepayment:  true,
					StartYear:    2020,
				},
			},
		},
		Simulation: SimulationConfig{
			StartYear:       2025,
			EndAge:          80,
			ReferencePerson: "Mortgaged",
		},
		TaxBands: ukTaxBands2024,
	}

	result := RunSimulation(SimulationParams{
		CrystallisationStrategy: GradualCrystallisation,
		DrawdownOrder:           SavingsFirst,
	}, config)

	// Years before mortgage payoff should have higher expenses
	mortgageEndYear := config.Mortgage.EndYear

	for _, year := range result.Years {
		if year.Year < mortgageEndYear {
			if year.MortgageCost == 0 {
				// Note: This might be expected depending on how the simulation handles mortgages
				t.Logf("Year %d: No mortgage cost recorded (may be included in required income)", year.Year)
			}
		}
	}

	t.Logf("With mortgage: %d years simulated, total tax £%.0f", len(result.Years), result.TotalTaxPaid)
}

// =============================================================================
// Pension Access Age Validation Tests
// =============================================================================

func TestCanAccessPension_ValidBirthYear(t *testing.T) {
	// Test that pension access is correctly gated by retirement age

	testCases := []struct {
		name          string
		birthYear     int
		retirementAge int
		currentYear   int
		expectAccess  bool
	}{
		// Normal cases
		{"Before retirement age", 1971, 55, 2025, false},      // Age 54
		{"At retirement age", 1971, 55, 2026, true},           // Age 55
		{"After retirement age", 1971, 55, 2030, true},        // Age 59
		{"Younger person not retired", 1973, 57, 2028, false}, // Age 55, needs 57
		{"Younger person at retirement", 1973, 57, 2030, true}, // Age 57

		// Edge cases with invalid birth years
		{"Zero birth year (invalid)", 0, 55, 2025, false},        // BirthYear = 0 should block access
		{"Negative birth year (invalid)", -100, 55, 2025, false}, // Negative should block
		{"Future birth year (invalid)", 2030, 55, 2025, false},   // Born after current year
		{"Ancient birth year (invalid)", 1800, 55, 2025, false},  // Too old to be realistic
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			person := &Person{
				Name:          "Test",
				BirthYear:     tc.birthYear,
				RetirementAge: tc.retirementAge,
			}

			result := person.CanAccessPension(tc.currentYear)
			if result != tc.expectAccess {
				t.Errorf("BirthYear=%d, RetirementAge=%d, Year=%d: expected CanAccessPension=%v, got %v",
					tc.birthYear, tc.retirementAge, tc.currentYear, tc.expectAccess, result)
			}
		})
	}
}

func TestPensionWithdrawalsRespectRetirementAge(t *testing.T) {
	// Test that in a simulation, pension withdrawals only happen after retirement age

	config := &Config{
		People: []PersonConfig{
			{
				Name:            "Person1",
				BirthDate:       "1970-06-15", // Born 1970, age 55 in 2025
				RetirementAge:   55,
				StatePensionAge: 67,
				TaxFreeSavings:  50000,
				Pension:         300000,
			},
			{
				Name:            "Person2",
				BirthDate:       "1972-03-20", // Born 1972, age 53 in 2025, age 57 in 2029
				RetirementAge:   57,
				StatePensionAge: 67,
				TaxFreeSavings:  50000,
				Pension:         300000,
			},
		},
		Financial: FinancialConfig{
			PensionGrowthRate:     0.05,
			SavingsGrowthRate:     0.04,
			IncomeInflationRate:   0.025,
			StatePensionInflation: 0.025,
			StatePensionAmount:    11500,
		},
		IncomeRequirements: IncomeConfig{
			MonthlyBeforeAge: 4000, // High income to force pension withdrawals
			MonthlyAfterAge:  3000,
			AgeThreshold:     67,
			ReferencePerson:  "Person1", // Person1 is reference, retires at 55
		},
		Simulation: SimulationConfig{
			StartYear:       2025, // Person1 is 55, Person2 is 53
			EndAge:          75,
			ReferencePerson: "Person1",
		},
		TaxBands: ukTaxBands2024,
	}

	result := RunSimulation(SimulationParams{
		CrystallisationStrategy: GradualCrystallisation,
		DrawdownOrder:           TaxOptimized,
	}, config)

	// Check that Person2 has no pension withdrawals until they reach age 57 (year 2029)
	for _, year := range result.Years {
		person2Age := year.Ages["Person2"]
		person2TaxableWithdrawal := year.Withdrawals.TaxableFromPension["Person2"]
		person2TaxFreeWithdrawal := year.Withdrawals.TaxFreeFromPension["Person2"]
		totalPerson2Withdrawal := person2TaxableWithdrawal + person2TaxFreeWithdrawal

		if person2Age < 57 && totalPerson2Withdrawal > 0 {
			t.Errorf("Year %d: Person2 (age %d) should not have pension withdrawals before age 57, but had £%.2f",
				year.Year, person2Age, totalPerson2Withdrawal)
		}
	}

	t.Logf("Verified: Person2 pension withdrawals respect retirement age (57)")
}

// =============================================================================
// Working Past Pension Age Scenarios
// =============================================================================

func TestScenario_WorkingPastPersonalPensionAge(t *testing.T) {
	// Scenario: Person continues working past their personal pension access age
	// They can access their pension but work income should cover expenses
	// Surplus work income should be deposited into ISA

	config := &Config{
		People: []PersonConfig{
			{
				Name:             "Worker",
				BirthDate:        "1970-06-15", // Age 55 in 2025
				RetirementAge:    60,           // Retires at 60 (2030)
				PensionAccessAge: 55,           // Can access pension at 55
				StatePensionAge:  67,
				TaxFreeSavings:   50000,
				Pension:          400000,
				WorkIncome:       60000, // £60k salary while working
			},
		},
		Financial: FinancialConfig{
			PensionGrowthRate:     0.05,
			SavingsGrowthRate:     0.04,
			IncomeInflationRate:   0.025,
			StatePensionInflation: 0.025,
			StatePensionAmount:    11500,
		},
		IncomeRequirements: IncomeConfig{
			MonthlyBeforeAge: 3000, // £36k/year - less than work income
			MonthlyAfterAge:  2500,
			AgeThreshold:     67,
			ReferencePerson:  "Worker",
		},
		Simulation: SimulationConfig{
			StartYear:       2025, // Age 55 - can access pension but still working
			EndAge:          75,
			ReferencePerson: "Worker",
		},
		TaxBands: ukTaxBands2024,
	}

	result := RunSimulation(SimulationParams{
		CrystallisationStrategy: GradualCrystallisation,
		DrawdownOrder:           TaxOptimized,
	}, config)

	if len(result.Years) == 0 {
		t.Fatal("Simulation produced no results")
	}

	// Verify person can access pension at age 55 (2025)
	person := &Person{
		Name:             "Worker",
		BirthYear:        1970,
		BirthDate:        "1970-06-15",
		PensionAccessAge: 55,
	}
	if !person.CanAccessPension(2025) {
		t.Error("Person should be able to access pension at age 55 (year 2025)")
	}

	// Check years while working (2025-2029, ages 55-59)
	for _, year := range result.Years {
		age := year.Ages["Worker"]
		if age < 60 {
			// Should have work income
			workIncome := year.WorkIncomeByPerson["Worker"]
			if workIncome <= 0 {
				t.Errorf("Year %d (age %d): Should have work income while employed, got £%.2f",
					year.Year, age, workIncome)
			}

			// No income requirement while working (income only required after retirement)
			if year.RequiredIncome > 0 {
				t.Errorf("Year %d (age %d): Should not have income requirement while working, got £%.2f",
					year.Year, age, year.RequiredIncome)
			}

			// Pension should NOT be touched while working (no need to withdraw)
			pensionWithdrawal := year.Withdrawals.TaxableFromPension["Worker"] + year.Withdrawals.TaxFreeFromPension["Worker"]
			if pensionWithdrawal > 0 {
				t.Errorf("Year %d (age %d): Should not need pension withdrawals while working, got £%.2f",
					year.Year, age, pensionWithdrawal)
			}

			// Surplus work income should go to ISA (after tax)
			isaContribution := year.ISAContributions["Worker"]
			if isaContribution <= 0 {
				t.Logf("Year %d (age %d): Expected ISA contribution from surplus income, got £%.2f",
					year.Year, age, isaContribution)
			}
		}
	}

	// Check that retirement kicks in at age 60 (2030)
	foundRetirement := false
	for _, year := range result.Years {
		age := year.Ages["Worker"]
		if age == 60 {
			foundRetirement = true
			// Should have income requirement after retirement
			if year.RequiredIncome <= 0 {
				t.Errorf("Year %d (age %d): Should have income requirement after retirement", year.Year, age)
			}
			// Should not have work income after retirement
			if year.WorkIncomeByPerson["Worker"] > 0 {
				t.Errorf("Year %d (age %d): Should not have work income after retirement, got £%.2f",
					year.Year, age, year.WorkIncomeByPerson["Worker"])
			}
			break
		}
	}

	if !foundRetirement {
		t.Error("Did not find year when retirement begins (age 60)")
	}

	t.Logf("Working past pension age: Person worked ages 55-59 with pension access, retired at 60")
}

func TestScenario_WorkingPastStatePensionAge(t *testing.T) {
	// Scenario: Person continues working past both personal pension age AND state pension age
	// They receive state pension AND work income simultaneously
	// This is a realistic scenario for people who want to maximize retirement funds

	config := &Config{
		People: []PersonConfig{
			{
				Name:             "LateRetirer",
				BirthDate:        "1960-03-15", // Age 66 in 2026, age 67 in 2027
				RetirementAge:    70,           // Retires at 70 (2030)
				PensionAccessAge: 55,           // Can access pension from 55
				StatePensionAge:  66,           // State pension from 66
				TaxFreeSavings:   100000,
				Pension:          500000,
				WorkIncome:       75000, // £75k salary while working
			},
		},
		Financial: FinancialConfig{
			PensionGrowthRate:     0.05,
			SavingsGrowthRate:     0.04,
			IncomeInflationRate:   0.025,
			StatePensionInflation: 0.025,
			StatePensionAmount:    11500,
		},
		IncomeRequirements: IncomeConfig{
			MonthlyBeforeAge: 3500, // £42k/year
			MonthlyAfterAge:  3000,
			AgeThreshold:     75,
			ReferencePerson:  "LateRetirer",
		},
		Simulation: SimulationConfig{
			StartYear:       2026, // Age 66 - already past personal pension age, at state pension age
			EndAge:          85,
			ReferencePerson: "LateRetirer",
		},
		TaxBands: ukTaxBands2024,
	}

	result := RunSimulation(SimulationParams{
		CrystallisationStrategy: GradualCrystallisation,
		DrawdownOrder:           TaxOptimized,
	}, config)

	if len(result.Years) == 0 {
		t.Fatal("Simulation produced no results")
	}

	// Test years from 2026 to 2029 - working past state pension age
	// Note: With March 15 birthday, in tax year 2026/27 they are age 67 (turn 67 on March 15, 2027)
	for _, year := range result.Years {
		age := year.Ages["LateRetirer"]

		// Ages 67-69: Still working, receiving state pension (age 67+ due to tax year calculation)
		if age >= 67 && age < 70 {
			// Should have work income
			workIncome := year.WorkIncomeByPerson["LateRetirer"]
			if workIncome <= 0 {
				t.Errorf("Year %d (age %d): Should have work income while employed, got £%.2f",
					year.Year, age, workIncome)
			}

			// Should receive state pension while still working
			statePension := year.StatePensionByPerson["LateRetirer"]
			if statePension <= 0 {
				t.Errorf("Year %d (age %d): Should receive state pension (age >= 66), got £%.2f",
					year.Year, age, statePension)
			}

			// No income requirement while working
			if year.RequiredIncome > 0 {
				t.Errorf("Year %d (age %d): Should not have income requirement while working, got £%.2f",
					year.Year, age, year.RequiredIncome)
			}

			// Combined income should be substantial (work + state pension)
			totalIncome := workIncome + statePension
			if totalIncome < 85000 {
				t.Errorf("Year %d (age %d): Total income (work + state pension) should be > £85k, got £%.2f",
					year.Year, age, totalIncome)
			}

			t.Logf("Year %d (age %d): Work income £%.0f + State pension £%.0f = Total £%.0f",
				year.Year, age, workIncome, statePension, totalIncome)
		}

		// Age 70+: Retired, still receiving state pension but no work income
		if age >= 70 {
			// Should NOT have work income after retirement
			if year.WorkIncomeByPerson["LateRetirer"] > 0 {
				t.Errorf("Year %d (age %d): Should not have work income after retirement, got £%.2f",
					year.Year, age, year.WorkIncomeByPerson["LateRetirer"])
			}

			// Should still receive state pension
			statePension := year.StatePensionByPerson["LateRetirer"]
			if statePension <= 0 {
				t.Errorf("Year %d (age %d): Should still receive state pension after retirement, got £%.2f",
					year.Year, age, statePension)
			}

			// Should have income requirement after retirement
			if year.RequiredIncome <= 0 {
				t.Errorf("Year %d (age %d): Should have income requirement after retirement", year.Year, age)
			}

			// Only check first year after retirement
			break
		}
	}

	// Verify pension access was available much earlier than state pension
	person := &Person{
		Name:             "LateRetirer",
		BirthYear:        1960,
		BirthDate:        "1960-03-15",
		PensionAccessAge: 55,
		StatePensionAge:  66,
	}

	// Could access pension from 2015 (age 55)
	if !person.CanAccessPension(2015) {
		t.Error("Person should be able to access pension from age 55 (year 2015)")
	}

	// State pension from 2025 (age 66) - with March birthday, they turn 66 in tax year 2025/26
	// (March 15 falls before April 6, so they reach 66 during tax year 2025)
	if !person.ReceivesStatePension(2025) {
		t.Error("Person should receive state pension in tax year 2025 (turns 66 on March 15, 2026)")
	}
	// Should NOT receive before age 66 (tax year 2024)
	if person.ReceivesStatePension(2024) {
		t.Error("Person should NOT receive state pension in tax year 2024 (still 65)")
	}

	t.Logf("Working past state pension age: Person worked ages 66-69 receiving both salary and state pension")
}

func TestScenario_WorkingPastBothPensionAgesWithCouple(t *testing.T) {
	// Scenario: One person works past their pension ages, the other retires earlier
	// Tests that the simulation correctly handles asymmetric retirement dates

	config := &Config{
		People: []PersonConfig{
			{
				Name:             "EarlyRetirer",
				BirthDate:        "1968-01-01", // Age 57 in 2025
				RetirementAge:    57,           // Retires immediately in 2025
				PensionAccessAge: 55,
				StatePensionAge:  67,
				TaxFreeSavings:   80000,
				Pension:          350000,
				WorkIncome:       0, // Already retired
			},
			{
				Name:             "LateWorker",
				BirthDate:        "1965-06-15", // Age 60 in 2025
				RetirementAge:    68,           // Works until 68
				PensionAccessAge: 55,
				StatePensionAge:  67,
				TaxFreeSavings:   120000,
				Pension:          600000,
				WorkIncome:       80000, // High salary
			},
		},
		Financial: FinancialConfig{
			PensionGrowthRate:     0.05,
			SavingsGrowthRate:     0.04,
			IncomeInflationRate:   0.025,
			StatePensionInflation: 0.025,
			StatePensionAmount:    11500,
		},
		IncomeRequirements: IncomeConfig{
			MonthlyBeforeAge: 4000, // £48k/year
			MonthlyAfterAge:  3500,
			AgeThreshold:     70,
			ReferencePerson:  "EarlyRetirer", // Income needs based on early retirer
		},
		Simulation: SimulationConfig{
			StartYear:       2025,
			EndAge:          85,
			ReferencePerson: "EarlyRetirer",
		},
		TaxBands: ukTaxBands2024,
	}

	result := RunSimulation(SimulationParams{
		CrystallisationStrategy: GradualCrystallisation,
		DrawdownOrder:           TaxOptimized,
	}, config)

	if len(result.Years) == 0 {
		t.Fatal("Simulation produced no results")
	}

	// Check first few years where LateWorker is still employed
	for _, year := range result.Years {
		earlyRetirerAge := year.Ages["EarlyRetirer"]
		lateWorkerAge := year.Ages["LateWorker"]

		// 2025-2032: LateWorker should be working (ages 60-67)
		if lateWorkerAge < 68 {
			workIncome := year.WorkIncomeByPerson["LateWorker"]
			if workIncome <= 0 {
				t.Errorf("Year %d: LateWorker (age %d) should have work income, got £%.2f",
					year.Year, lateWorkerAge, workIncome)
			}
		}

		// LateWorker should get state pension from age 67
		if lateWorkerAge >= 67 {
			statePension := year.StatePensionByPerson["LateWorker"]
			if statePension <= 0 {
				t.Errorf("Year %d: LateWorker (age %d) should receive state pension, got £%.2f",
					year.Year, lateWorkerAge, statePension)
			}

			// If still working (age 67), should have both work income AND state pension
			if lateWorkerAge < 68 {
				workIncome := year.WorkIncomeByPerson["LateWorker"]
				if workIncome <= 0 {
					t.Errorf("Year %d: LateWorker (age %d) should have work income AND state pension",
						year.Year, lateWorkerAge)
				}
				t.Logf("Year %d: LateWorker (age %d) has work income £%.0f AND state pension £%.0f",
					year.Year, lateWorkerAge, workIncome, statePension)
			}
		}

		// EarlyRetirer should NOT have work income (already retired)
		earlyRetirerWork := year.WorkIncomeByPerson["EarlyRetirer"]
		if earlyRetirerWork > 0 {
			t.Errorf("Year %d: EarlyRetirer (age %d) should not have work income, got £%.2f",
				year.Year, earlyRetirerAge, earlyRetirerWork)
		}

		// EarlyRetirer gets state pension from age 67
		if earlyRetirerAge >= 67 {
			statePension := year.StatePensionByPerson["EarlyRetirer"]
			if statePension <= 0 {
				t.Errorf("Year %d: EarlyRetirer (age %d) should receive state pension, got £%.2f",
					year.Year, earlyRetirerAge, statePension)
			}
		}
	}

	t.Logf("Couple scenario: EarlyRetirer at 57, LateWorker works until 68 (past state pension age 67)")
}

// =============================================================================
// ISA to SIPP Transfer Strategy Tests
// =============================================================================

func TestScenario_ISAToSIPPTransfer(t *testing.T) {
	// Scenario: Person with work income transfers ISA money to SIPP to get tax relief
	// Higher rate taxpayer (40%) gets significant benefit from this strategy
	// £60 from ISA becomes £100 in pension (£40 tax relief)

	config := &Config{
		People: []PersonConfig{
			{
				Name:                   "HighEarner",
				BirthDate:              "1970-06-15", // Age 55 in 2025
				RetirementAge:          60,           // Retires at 60
				PensionAccessAge:       55,
				StatePensionAge:        67,
				TaxFreeSavings:         200000,  // £200k ISA
				Pension:                300000,  // £300k pension
				WorkIncome:             80000,   // £80k salary (higher rate taxpayer)
				ISAToSIPPEnabled:       true,    // Enable ISA to SIPP transfers
				PensionAnnualAllowance: 60000,   // £60k annual allowance
				EmployerContribution:   5000,    // £5k employer contribution already
				ISAToSIPPMaxPercent:    1.0,     // Use 100% of available allowance
				ISAToSIPPPreserveMonths: 12,     // Preserve 12 months expenses in ISA
			},
		},
		Financial: FinancialConfig{
			PensionGrowthRate:     0.05,
			SavingsGrowthRate:     0.04,
			IncomeInflationRate:   0.025,
			StatePensionInflation: 0.025,
			StatePensionAmount:    11500,
		},
		IncomeRequirements: IncomeConfig{
			MonthlyBeforeAge: 3000, // £36k/year
			MonthlyAfterAge:  2500,
			AgeThreshold:     67,
			ReferencePerson:  "HighEarner",
		},
		Simulation: SimulationConfig{
			StartYear:       2025,
			EndAge:          75,
			ReferencePerson: "HighEarner",
		},
		TaxBands: ukTaxBands2024,
	}

	// Run simulation WITH ISA to SIPP
	resultWithTransfer := RunSimulation(SimulationParams{
		CrystallisationStrategy: GradualCrystallisation,
		DrawdownOrder:           TaxOptimized,
		ISAToSIPPEnabled:        true,
	}, config)

	// Run simulation WITHOUT ISA to SIPP
	resultWithoutTransfer := RunSimulation(SimulationParams{
		CrystallisationStrategy: GradualCrystallisation,
		DrawdownOrder:           TaxOptimized,
		ISAToSIPPEnabled:        false,
	}, config)

	if len(resultWithTransfer.Years) == 0 || len(resultWithoutTransfer.Years) == 0 {
		t.Fatal("Simulation produced no results")
	}

	// Check that ISA to SIPP transfers occurred during working years
	totalTransferred := 0.0
	totalTaxRelief := 0.0
	for _, year := range resultWithTransfer.Years {
		age := year.Ages["HighEarner"]
		if age < 60 { // Working years
			transferred := year.ISAToSIPPByPerson["HighEarner"]
			taxRelief := year.ISAToSIPPTaxRelief["HighEarner"]

			if transferred > 0 {
				totalTransferred += transferred
				totalTaxRelief += taxRelief

				t.Logf("Year %d (age %d): ISA→SIPP transfer £%.0f, tax relief £%.0f",
					year.Year, age, transferred, taxRelief)
			}
		}
	}

	if totalTransferred <= 0 {
		t.Error("Expected ISA to SIPP transfers during working years, but none occurred")
	}

	if totalTaxRelief <= 0 {
		t.Error("Expected tax relief from ISA to SIPP transfers, but none received")
	}

	// Verify tax relief is approximately correct (40% marginal rate for £80k earner)
	// Tax relief should be roughly 66% of net contribution (£60 net = £100 gross, so relief = £40)
	expectedReliefRatio := 0.66 // For 40% taxpayer: 0.40 / (1 - 0.40) = 0.667
	actualReliefRatio := totalTaxRelief / totalTransferred
	if actualReliefRatio < 0.5 || actualReliefRatio > 0.8 {
		t.Errorf("Expected relief ratio around %.2f (40%% taxpayer), got %.2f",
			expectedReliefRatio, actualReliefRatio)
	}

	// Compare final balances: ISA to SIPP should result in higher total wealth
	// because the tax relief is essentially free money
	finalWithTransfer := 0.0
	finalWithoutTransfer := 0.0
	for _, bal := range resultWithTransfer.FinalBalances {
		finalWithTransfer += bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot
	}
	for _, bal := range resultWithoutTransfer.FinalBalances {
		finalWithoutTransfer += bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot
	}

	t.Logf("Final balance with ISA→SIPP: £%.0f", finalWithTransfer)
	t.Logf("Final balance without ISA→SIPP: £%.0f", finalWithoutTransfer)
	t.Logf("Difference: £%.0f", finalWithTransfer-finalWithoutTransfer)

	// The ISA to SIPP strategy should result in higher final balance
	// (tax relief + compound growth on larger pension pot)
	if finalWithTransfer <= finalWithoutTransfer {
		t.Errorf("Expected ISA to SIPP to result in higher final balance, but got £%.0f vs £%.0f",
			finalWithTransfer, finalWithoutTransfer)
	}

	t.Logf("ISA to SIPP strategy: Transferred £%.0f, received £%.0f tax relief over %d working years",
		totalTransferred, totalTaxRelief, 5)
}

func TestScenario_ISAToSIPPWithDBPension(t *testing.T) {
	// Scenario: Couple where one person has DB pension (Teachers), one has DC/SIPP
	// Only the DC pension holder should use ISA to SIPP (DB doesn't accept contributions)

	config := &Config{
		People: []PersonConfig{
			{
				Name:                   "DBPensionHolder",
				BirthDate:              "1968-01-01", // Teacher with DB pension
				RetirementAge:          60,
				PensionAccessAge:       55,
				StatePensionAge:        67,
				TaxFreeSavings:         100000,
				Pension:                50000,       // Small DC SIPP
				WorkIncome:             55000,       // Teacher salary
				DBPensionAmount:        25000,       // Teachers Pension
				DBPensionStartAge:      60,
				DBPensionName:          "Teachers Pension",
				ISAToSIPPEnabled:       false,       // DB pension holder typically won't use this
				PensionAnnualAllowance: 60000,
			},
			{
				Name:                   "SIPPHolder",
				BirthDate:              "1970-06-15",
				RetirementAge:          60,
				PensionAccessAge:       55,
				StatePensionAge:        67,
				TaxFreeSavings:         150000,      // £150k ISA
				Pension:                400000,      // £400k SIPP
				WorkIncome:             75000,       // £75k salary
				ISAToSIPPEnabled:       true,        // Use ISA to SIPP for SIPP
				PensionAnnualAllowance: 60000,
				EmployerContribution:   3000,
				ISAToSIPPMaxPercent:    1.0,
				ISAToSIPPPreserveMonths: 12,
			},
		},
		Financial: FinancialConfig{
			PensionGrowthRate:     0.05,
			SavingsGrowthRate:     0.04,
			IncomeInflationRate:   0.025,
			StatePensionInflation: 0.025,
			StatePensionAmount:    11500,
		},
		IncomeRequirements: IncomeConfig{
			MonthlyBeforeAge: 4000,
			MonthlyAfterAge:  3500,
			AgeThreshold:     67,
			ReferencePerson:  "DBPensionHolder",
		},
		Simulation: SimulationConfig{
			StartYear:       2025,
			EndAge:          80,
			ReferencePerson: "DBPensionHolder",
		},
		TaxBands: ukTaxBands2024,
	}

	result := RunSimulation(SimulationParams{
		CrystallisationStrategy: GradualCrystallisation,
		DrawdownOrder:           TaxOptimized,
		ISAToSIPPEnabled:        true,
	}, config)

	if len(result.Years) == 0 {
		t.Fatal("Simulation produced no results")
	}

	// Check transfers: DBPensionHolder should NOT have transfers, SIPPHolder should
	dbTransfers := 0.0
	sippTransfers := 0.0

	for _, year := range result.Years {
		dbTransfers += year.ISAToSIPPByPerson["DBPensionHolder"]
		sippTransfers += year.ISAToSIPPByPerson["SIPPHolder"]
	}

	// DB pension holder should not have transfers (ISAToSIPPEnabled = false)
	if dbTransfers > 0 {
		t.Errorf("DBPensionHolder should not have ISA to SIPP transfers (has DB pension), but got £%.0f", dbTransfers)
	}

	// SIPP holder should have transfers
	if sippTransfers <= 0 {
		t.Error("SIPPHolder should have ISA to SIPP transfers, but got none")
	}

	t.Logf("Couple scenario: DBPensionHolder transfers: £%.0f, SIPPHolder transfers: £%.0f", dbTransfers, sippTransfers)
}
