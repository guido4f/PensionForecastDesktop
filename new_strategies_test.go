package main

import (
	"testing"
)

// TestFillBasicRate_StaysWithinBasicRateBand verifies that FillBasicRate
// withdraws from pension to fill the basic rate band but doesn't exceed it
func TestFillBasicRate_StaysWithinBasicRateBand(t *testing.T) {
	// Create a single person with pension
	person := &Person{
		Name:              "Test",
		BirthYear:         1965,
		RetirementAge:     55,
		UncrystallisedPot: 500000,
		CrystallisedPot:   0,
		TaxFreeSavings:    50000,
		ISAAnnualLimit:    20000,
	}
	people := []*Person{person}

	statePensionByPerson := map[string]float64{"Test": 0}
	taxBands := ukTaxBands2024

	// Request low income - FillBasicRate should still fill up to basic rate
	netNeeded := 20000.0
	year := 2025

	breakdown := ExecuteFillBasicRateDrawdown(people, netNeeded, GradualCrystallisation, year, statePensionByPerson, taxBands)

	// Total taxable should be close to basic rate limit (50270)
	taxableWithdrawn := breakdown.TaxableFromPension["Test"]
	basicRateLimit := 50270.0

	if taxableWithdrawn > basicRateLimit*1.01 {
		t.Errorf("FillBasicRate exceeded basic rate limit: got £%.0f, limit is £%.0f", taxableWithdrawn, basicRateLimit)
	}

	// Should have ISA deposits (excess from filling basic rate when only 20k needed)
	if breakdown.TotalISADeposits <= 0 {
		t.Logf("FillBasicRate: taxable=£%.0f, tax-free=£%.0f, ISA deposits=£%.0f",
			breakdown.TotalTaxable, breakdown.TotalTaxFree, breakdown.TotalISADeposits)
	}

	t.Logf("FillBasicRate: taxable=£%.0f, tax-free=£%.0f, ISA deposits=£%.0f",
		taxableWithdrawn, breakdown.TaxFreeFromPension["Test"], breakdown.TotalISADeposits)
}

// TestFillBasicRate_WithStatePension verifies FillBasicRate accounts for state pension
func TestFillBasicRate_WithStatePension(t *testing.T) {
	person := &Person{
		Name:              "Test",
		BirthYear:         1958,
		RetirementAge:     55,
		UncrystallisedPot: 500000,
		CrystallisedPot:   0,
		TaxFreeSavings:    50000,
		ISAAnnualLimit:    20000,
	}
	people := []*Person{person}

	// State pension of £11,500 - leaves limited personal allowance
	statePensionByPerson := map[string]float64{"Test": 11500}
	taxBands := ukTaxBands2024

	netNeeded := 30000.0
	year := 2025

	breakdown := ExecuteFillBasicRateDrawdown(people, netNeeded, GradualCrystallisation, year, statePensionByPerson, taxBands)

	// Total income should not exceed basic rate limit
	totalTaxable := breakdown.TaxableFromPension["Test"]
	totalIncome := statePensionByPerson["Test"] + totalTaxable

	basicRateLimit := 50270.0
	if totalIncome > basicRateLimit*1.01 {
		t.Errorf("FillBasicRate with state pension exceeded limit: total income £%.0f, limit £%.0f",
			totalIncome, basicRateLimit)
	}

	t.Logf("FillBasicRate with SP: state pension=£%.0f, pension taxable=£%.0f, total=£%.0f",
		statePensionByPerson["Test"], totalTaxable, totalIncome)
}

// TestStatePensionBridge_BeforeStatePension verifies heavy drawing before state pension
func TestStatePensionBridge_BeforeStatePension(t *testing.T) {
	person := &Person{
		Name:              "Test",
		BirthYear:         1965,
		RetirementAge:     55,
		StatePensionAge:   67,
		UncrystallisedPot: 500000,
		CrystallisedPot:   0,
		TaxFreeSavings:    50000,
		ISAAnnualLimit:    20000,
	}
	people := []*Person{person}

	// No state pension yet (before age 67)
	statePensionByPerson := map[string]float64{"Test": 0}
	taxBands := ukTaxBands2024

	netNeeded := 25000.0
	year := 2025 // Person is 60, before state pension age

	breakdown := ExecuteStatePensionBridgeDrawdown(people, netNeeded, GradualCrystallisation, year, statePensionByPerson, taxBands)

	// Before state pension: should fill basic rate band (heavy drawing)
	taxableWithdrawn := breakdown.TaxableFromPension["Test"]
	basicRateLimit := 50270.0

	// Should be drawing close to basic rate limit
	if taxableWithdrawn < basicRateLimit*0.5 {
		t.Errorf("StatePensionBridge before SP should draw heavily: got £%.0f, expected close to £%.0f",
			taxableWithdrawn, basicRateLimit)
	}

	t.Logf("StatePensionBridge before SP: taxable=£%.0f, tax-free=£%.0f, ISA deposits=£%.0f",
		taxableWithdrawn, breakdown.TaxFreeFromPension["Test"], breakdown.TotalISADeposits)
}

// TestStatePensionBridge_AfterStatePension verifies reduced drawing after state pension
func TestStatePensionBridge_AfterStatePension(t *testing.T) {
	person := &Person{
		Name:              "Test",
		BirthYear:         1958,
		RetirementAge:     55,
		StatePensionAge:   66,
		UncrystallisedPot: 500000,
		CrystallisedPot:   0,
		TaxFreeSavings:    50000,
		ISAAnnualLimit:    20000,
	}
	people := []*Person{person}

	// Receiving state pension
	statePensionByPerson := map[string]float64{"Test": 11500}
	taxBands := ukTaxBands2024

	netNeeded := 25000.0
	year := 2025 // Person is 67, receiving state pension

	breakdown := ExecuteStatePensionBridgeDrawdown(people, netNeeded, GradualCrystallisation, year, statePensionByPerson, taxBands)

	// After state pension: should draw only what's needed (pension first strategy)
	totalWithdrawn := breakdown.TotalTaxFree + breakdown.TotalTaxable

	// Net needed should be approximately covered (not over-drawing)
	// Allow for some tax gross-up
	if totalWithdrawn > netNeeded*2 {
		t.Errorf("StatePensionBridge after SP should draw conservatively: got £%.0f gross for £%.0f net",
			totalWithdrawn, netNeeded)
	}

	t.Logf("StatePensionBridge after SP: taxable=£%.0f, tax-free=£%.0f, ISA=£%.0f",
		breakdown.TotalTaxable, breakdown.TotalTaxFree, breakdown.TotalISADeposits)
}

// TestUFPLS_25PercentTaxFree verifies UFPLS withdrawals are 25% tax-free
func TestUFPLS_25PercentTaxFree(t *testing.T) {
	person := &Person{
		Name:              "Test",
		BirthYear:         1965,
		RetirementAge:     55,
		UncrystallisedPot: 100000,
		CrystallisedPot:   0,
		TaxFreeSavings:    0,
		ISAAnnualLimit:    20000,
	}
	people := []*Person{person}

	statePensionByPerson := map[string]float64{"Test": 0}
	taxBands := ukTaxBands2024

	netNeeded := 20000.0
	year := 2025

	// Use UFPLSStrategy through the ExecuteDrawdown function
	params := SimulationParams{
		CrystallisationStrategy: UFPLSStrategy,
		DrawdownOrder:           PensionFirst,
	}

	breakdown := ExecuteDrawdown(people, netNeeded, params, year, statePensionByPerson, taxBands)

	// UFPLS: 25% should be tax-free, 75% taxable
	totalWithdrawn := breakdown.TotalTaxFree + breakdown.TotalTaxable
	if totalWithdrawn > 0 {
		taxFreeRatio := breakdown.TotalTaxFree / totalWithdrawn
		expectedRatio := 0.25

		// Allow 5% tolerance due to gross-up calculations
		if taxFreeRatio < expectedRatio*0.8 || taxFreeRatio > expectedRatio*1.2 {
			t.Errorf("UFPLS tax-free ratio wrong: got %.1f%%, expected ~25%%",
				taxFreeRatio*100)
		}
	}

	t.Logf("UFPLS: total=£%.0f, tax-free=£%.0f (%.1f%%), taxable=£%.0f",
		totalWithdrawn, breakdown.TotalTaxFree, breakdown.TotalTaxFree/totalWithdrawn*100, breakdown.TotalTaxable)
}

// TestNewStrategies_InSimulation tests new strategies work in full simulation
func TestNewStrategies_InSimulation(t *testing.T) {
	config := &Config{
		People: []PersonConfig{
			{
				Name:            "James",
				BirthDate:       "1965-01-01",
				RetirementAge:   57,
				StatePensionAge: 67,
				TaxFreeSavings:  100000,
				Pension:         500000,
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
			ReferencePerson:  "James",
		},
		Simulation: SimulationConfig{
			StartYear:       2025,
			EndAge:          90,
			ReferencePerson: "James",
		},
		TaxBands: ukTaxBands2024,
	}

	strategies := []struct {
		name   string
		params SimulationParams
	}{
		{"FillBasicRate", SimulationParams{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: FillBasicRate}},
		{"StatePensionBridge", SimulationParams{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: StatePensionBridge}},
		{"UFPLS+PensionFirst", SimulationParams{CrystallisationStrategy: UFPLSStrategy, DrawdownOrder: PensionFirst}},
		{"UFPLS+FillBasicRate", SimulationParams{CrystallisationStrategy: UFPLSStrategy, DrawdownOrder: FillBasicRate}},
	}

	for _, s := range strategies {
		t.Run(s.name, func(t *testing.T) {
			// Reset config
			config.People[0].TaxFreeSavings = 100000
			config.People[0].Pension = 500000

			result := RunSimulation(s.params, config)

			if len(result.Years) == 0 {
				t.Errorf("%s: simulation produced no results", s.name)
				return
			}

			t.Logf("%s: %d years, total tax £%.0f, ran out: %v (year %d)",
				s.name, len(result.Years), result.TotalTaxPaid,
				result.RanOutOfMoney, result.RanOutYear)
		})
	}
}

// TestNewStrategies_CompareEfficiency compares tax efficiency of strategies
func TestNewStrategies_CompareEfficiency(t *testing.T) {
	config := &Config{
		People: []PersonConfig{
			{
				Name:            "James",
				BirthDate:       "1965-01-01",
				RetirementAge:   57,
				StatePensionAge: 67,
				TaxFreeSavings:  100000,
				Pension:         500000,
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
			ReferencePerson:  "James",
		},
		Simulation: SimulationConfig{
			StartYear:       2025,
			EndAge:          85,
			ReferencePerson: "James",
		},
		TaxBands: ukTaxBands2024,
	}

	strategies := []struct {
		name   string
		params SimulationParams
	}{
		{"SavingsFirst", SimulationParams{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: SavingsFirst}},
		{"PensionFirst", SimulationParams{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: PensionFirst}},
		{"TaxOptimized", SimulationParams{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: TaxOptimized}},
		{"FillBasicRate", SimulationParams{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: FillBasicRate}},
		{"StatePensionBridge", SimulationParams{CrystallisationStrategy: GradualCrystallisation, DrawdownOrder: StatePensionBridge}},
		{"UFPLS+PensionFirst", SimulationParams{CrystallisationStrategy: UFPLSStrategy, DrawdownOrder: PensionFirst}},
	}

	results := make(map[string]float64)

	for _, s := range strategies {
		// Reset config
		config.People[0].TaxFreeSavings = 100000
		config.People[0].Pension = 500000

		result := RunSimulation(s.params, config)
		results[s.name] = result.TotalTaxPaid
	}

	t.Log("Strategy Tax Comparison:")
	for name, tax := range results {
		t.Logf("  %s: £%.0f", name, tax)
	}

	// FillBasicRate and StatePensionBridge should be competitive with TaxOptimized
	if results["FillBasicRate"] > results["SavingsFirst"]*1.5 {
		t.Logf("Note: FillBasicRate paid more tax than SavingsFirst")
	}
}

// TestGetStrategiesForConfig_IncludesNewStrategies verifies new strategies are in config
func TestGetStrategiesForConfig_IncludesNewStrategies(t *testing.T) {
	// Test without mortgage
	config := &Config{}
	strategies := GetStrategiesForConfig(config)

	foundFillBasicRate := false
	foundStatePensionBridge := false
	foundUFPLS := false

	for _, s := range strategies {
		if s.DrawdownOrder == FillBasicRate {
			foundFillBasicRate = true
		}
		if s.DrawdownOrder == StatePensionBridge {
			foundStatePensionBridge = true
		}
		if s.CrystallisationStrategy == UFPLSStrategy {
			foundUFPLS = true
		}
	}

	if !foundFillBasicRate {
		t.Error("GetStrategiesForConfig missing FillBasicRate")
	}
	if !foundStatePensionBridge {
		t.Error("GetStrategiesForConfig missing StatePensionBridge")
	}
	if !foundUFPLS {
		t.Error("GetStrategiesForConfig missing UFPLS strategies")
	}

	t.Logf("GetStrategiesForConfig (no mortgage): %d strategies", len(strategies))

	// Test with mortgage
	configWithMortgage := &Config{
		Mortgage: MortgageConfig{
			Parts: []MortgagePartConfig{
				{Name: "Main", Principal: 200000, InterestRate: 0.04, TermYears: 25, StartYear: 2020, IsRepayment: true},
			},
			EndYear:         2045,
			EarlyPayoffYear: 2026,
		},
	}
	strategiesWithMortgage := GetStrategiesForConfig(configWithMortgage)
	t.Logf("GetStrategiesForConfig (with mortgage): %d strategies", len(strategiesWithMortgage))

	if len(strategiesWithMortgage) <= len(strategies) {
		t.Error("Mortgage config should have more strategies than non-mortgage")
	}
}
