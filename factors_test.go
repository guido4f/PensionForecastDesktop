package main

import (
	"testing"
)

// TestFactorRegistryCreation verifies the factor registry is correctly initialized
func TestFactorRegistryCreation(t *testing.T) {
	registry := NewFactorRegistry()

	// Verify all 7 factors are registered
	expectedFactors := []FactorID{
		FactorCrystallisation,
		FactorDrawdown,
		FactorMortgage,
		FactorMaximizeCoupleISA,
		FactorISAToSIPP,
		FactorGuardrails,
		FactorStatePensionDefer,
	}

	for _, factorID := range expectedFactors {
		factor := registry.Get(factorID)
		if factor == nil {
			t.Errorf("Factor %s not found in registry", factorID)
		}
	}

	// Verify total count
	allFactors := registry.GetAll()
	if len(allFactors) != 7 {
		t.Errorf("Expected 7 factors, got %d", len(allFactors))
	}
}

// TestConstraintValidation verifies that constraints correctly filter invalid combinations
func TestConstraintValidation(t *testing.T) {
	constraints := DefaultConstraints()

	tests := []struct {
		name    string
		combo   StrategyCombo
		valid   bool
	}{
		{
			name: "MaximizeCoupleISA with PensionToISA - valid",
			combo: StrategyCombo{Values: map[FactorID]FactorValue{
				FactorMaximizeCoupleISA: {ID: "on", Value: true},
				FactorDrawdown:          {ID: "pension_to_isa", Value: PensionToISA},
			}},
			valid: true,
		},
		{
			name: "MaximizeCoupleISA with SavingsFirst - invalid",
			combo: StrategyCombo{Values: map[FactorID]FactorValue{
				FactorMaximizeCoupleISA: {ID: "on", Value: true},
				FactorDrawdown:          {ID: "savings_first", Value: SavingsFirst},
			}},
			valid: false,
		},
		{
			name: "ISAToSIPP with PensionToISA - invalid",
			combo: StrategyCombo{Values: map[FactorID]FactorValue{
				FactorISAToSIPP: {ID: "on", Value: true},
				FactorDrawdown:  {ID: "pension_to_isa", Value: PensionToISA},
			}},
			valid: false,
		},
		{
			name: "ISAToSIPP with TaxOptimized - valid",
			combo: StrategyCombo{Values: map[FactorID]FactorValue{
				FactorISAToSIPP: {ID: "on", Value: true},
				FactorDrawdown:  {ID: "tax_optimized", Value: TaxOptimized},
			}},
			valid: true,
		},
		{
			name: "UFPLS with PCLS payoff - invalid",
			combo: StrategyCombo{Values: map[FactorID]FactorValue{
				FactorCrystallisation: {ID: "ufpls", Value: UFPLSStrategy},
				FactorMortgage:        {ID: "pcls", Value: PCLSMortgagePayoff},
			}},
			valid: false,
		},
		{
			name: "Gradual with PCLS payoff - valid",
			combo: StrategyCombo{Values: map[FactorID]FactorValue{
				FactorCrystallisation: {ID: "gradual", Value: GradualCrystallisation},
				FactorMortgage:        {ID: "pcls", Value: PCLSMortgagePayoff},
			}},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := true
			for _, constraint := range constraints {
				if !constraint.Validate(tt.combo) {
					valid = false
					break
				}
			}
			if valid != tt.valid {
				t.Errorf("Expected valid=%v, got %v", tt.valid, valid)
			}
		})
	}
}

// TestCombinationGeneration verifies that the combination generator produces the expected number of strategies
func TestCombinationGeneration(t *testing.T) {
	// Test with a config that has mortgage and work income (to enable all factors)
	config := &Config{
		People: []PersonConfig{
			{
				Name:       "Person1",
				BirthDate:  "1970-01-01",
				WorkIncome: 50000,
			},
			{
				Name:       "Person2",
				BirthDate:  "1972-01-01",
				WorkIncome: 40000,
			},
		},
		Mortgage: MortgageConfig{
			Parts: []MortgagePartConfig{
				{Principal: 100000, InterestRate: 0.04, IsRepayment: true, TermYears: 25, StartYear: 2020},
			},
			EndYear:         2030,
			AllowExtension:  true, // Enable extended mortgage strategies
			ExtendedEndYear: 2040,
		},
		Simulation: SimulationConfig{
			StartYear: 2025,
			EndAge:    90,
		},
	}

	counts := GetCombinationCount(config)

	t.Logf("Combination counts with mortgage and work income:")
	t.Logf("  Quick: %d", counts[ModeQuick])
	t.Logf("  Standard: %d", counts[ModeStandard])
	t.Logf("  Thorough: %d", counts[ModeThorough])
	t.Logf("  Comprehensive: %d", counts[ModeComprehensive])

	// Quick mode should be smallest
	if counts[ModeQuick] >= counts[ModeStandard] {
		t.Errorf("Quick mode (%d) should have fewer combinations than Standard (%d)",
			counts[ModeQuick], counts[ModeStandard])
	}

	// Standard mode should be smaller than Thorough
	if counts[ModeStandard] >= counts[ModeThorough] {
		t.Errorf("Standard mode (%d) should have fewer combinations than Thorough (%d)",
			counts[ModeStandard], counts[ModeThorough])
	}

	// Thorough mode should be smaller than Comprehensive
	if counts[ModeThorough] >= counts[ModeComprehensive] {
		t.Errorf("Thorough mode (%d) should have fewer combinations than Comprehensive (%d)",
			counts[ModeThorough], counts[ModeComprehensive])
	}

	// Verify reasonable ranges
	// Quick: Core strategies only (~4-16)
	if counts[ModeQuick] < 4 || counts[ModeQuick] > 20 {
		t.Errorf("Quick mode should have 4-20 combinations, got %d", counts[ModeQuick])
	}
	// Standard: Common variations (~30-200)
	if counts[ModeStandard] < 30 || counts[ModeStandard] > 200 {
		t.Errorf("Standard mode should have 30-200 combinations, got %d", counts[ModeStandard])
	}
	// Thorough: More analysis (~150-500)
	if counts[ModeThorough] < 100 || counts[ModeThorough] > 600 {
		t.Errorf("Thorough mode should have 100-600 combinations, got %d", counts[ModeThorough])
	}
	// Comprehensive: All valid combinations (can be large for exhaustive analysis)
	if counts[ModeComprehensive] < 500 {
		t.Errorf("Comprehensive mode should have at least 500 combinations, got %d", counts[ModeComprehensive])
	}
}

// TestCombinationGenerationNoMortgage verifies counts without mortgage
func TestCombinationGenerationNoMortgage(t *testing.T) {
	config := &Config{
		People: []PersonConfig{
			{
				Name:       "Person1",
				BirthDate:  "1970-01-01",
				WorkIncome: 50000,
			},
		},
		Simulation: SimulationConfig{
			StartYear: 2025,
			EndAge:    90,
		},
	}

	counts := GetCombinationCount(config)

	t.Logf("Combination counts without mortgage:")
	t.Logf("  Quick: %d", counts[ModeQuick])
	t.Logf("  Standard: %d", counts[ModeStandard])
	t.Logf("  Comprehensive: %d", counts[ModeComprehensive])

	// Without mortgage, should have fewer combinations than with mortgage
	// Mortgage factor (4 options) not included, so ~1/4 the combinations
	// But still can be large for exhaustive analysis
	if counts[ModeComprehensive] < 50 {
		t.Errorf("Without mortgage, Comprehensive should have at least 50 combinations, got %d",
			counts[ModeComprehensive])
	}
}

// TestToSimulationParams verifies the conversion from StrategyCombo to SimulationParams
func TestToSimulationParams(t *testing.T) {
	combo := StrategyCombo{Values: map[FactorID]FactorValue{
		FactorCrystallisation:   {ID: "ufpls", Value: UFPLSStrategy},
		FactorDrawdown:          {ID: "tax_optimized", Value: TaxOptimized},
		FactorMortgage:          {ID: "early", Value: MortgageEarly},
		FactorGuardrails:        {ID: "on", Value: true},
		FactorStatePensionDefer: {ID: "2y", Value: 2},
		FactorISAToSIPP:         {ID: "on", Value: true},
	}}

	params := combo.ToSimulationParams()

	if params.CrystallisationStrategy != UFPLSStrategy {
		t.Errorf("Expected UFPLSStrategy, got %v", params.CrystallisationStrategy)
	}
	if params.DrawdownOrder != TaxOptimized {
		t.Errorf("Expected TaxOptimized, got %v", params.DrawdownOrder)
	}
	if params.MortgageOpt != MortgageEarly {
		t.Errorf("Expected MortgageEarly, got %v", params.MortgageOpt)
	}
	if !params.GuardrailsEnabled {
		t.Error("Expected GuardrailsEnabled to be true")
	}
	if params.StatePensionDeferYears != 2 {
		t.Errorf("Expected StatePensionDeferYears=2, got %d", params.StatePensionDeferYears)
	}
	if !params.ISAToSIPPEnabled {
		t.Error("Expected ISAToSIPPEnabled to be true")
	}
}

// TestApplyParamsToConfig verifies that config is correctly modified by params
func TestApplyParamsToConfig(t *testing.T) {
	config := &Config{
		People: []PersonConfig{
			{Name: "Person1", StatePensionDeferYears: 0},
			{Name: "Person2", StatePensionDeferYears: 0},
		},
		IncomeRequirements: IncomeConfig{
			GuardrailsEnabled: false,
		},
		Financial: FinancialConfig{
			PensionGrowthRate: 0.05,
			SavingsGrowthRate: 0.04,
		},
	}

	params := SimulationParams{
		GuardrailsEnabled:      true,
		StatePensionDeferYears: 3,
	}

	newConfig := ApplyParamsToConfig(params, config)

	// Verify original config unchanged
	if config.IncomeRequirements.GuardrailsEnabled {
		t.Error("Original config GuardrailsEnabled should still be false")
	}
	if config.People[0].StatePensionDeferYears != 0 {
		t.Error("Original config StatePensionDeferYears should still be 0")
	}

	// Verify new config has updated values
	if !newConfig.IncomeRequirements.GuardrailsEnabled {
		t.Error("New config GuardrailsEnabled should be true")
	}
	if newConfig.People[0].StatePensionDeferYears != 3 {
		t.Errorf("New config StatePensionDeferYears should be 3, got %d",
			newConfig.People[0].StatePensionDeferYears)
	}
	if newConfig.People[1].StatePensionDeferYears != 3 {
		t.Errorf("New config Person2 StatePensionDeferYears should be 3, got %d",
			newConfig.People[1].StatePensionDeferYears)
	}

	// Verify growth rates are preserved (not modified by params)
	if newConfig.Financial.PensionGrowthRate != 0.05 {
		t.Errorf("Expected PensionGrowthRate=0.05 (unchanged), got %f",
			newConfig.Financial.PensionGrowthRate)
	}
	if newConfig.Financial.SavingsGrowthRate != 0.04 {
		t.Errorf("Expected SavingsGrowthRate=0.04 (unchanged), got %f",
			newConfig.Financial.SavingsGrowthRate)
	}
}

// TestGetStrategiesForConfigV2 verifies the V2 strategy generation works
func TestGetStrategiesForConfigV2(t *testing.T) {
	config := &Config{
		People: []PersonConfig{
			{
				Name:       "Person1",
				BirthDate:  "1970-01-01",
				WorkIncome: 50000,
			},
		},
		Simulation: SimulationConfig{
			StartYear: 2025,
			EndAge:    90,
		},
	}

	// Test Quick mode
	quickStrategies := GetStrategiesForConfigV2(config, ModeQuick)
	if len(quickStrategies) == 0 {
		t.Error("Quick mode should generate at least some strategies")
	}

	// Verify each strategy has valid values
	for i, s := range quickStrategies {
		// Crystallisation should be valid
		if s.CrystallisationStrategy != GradualCrystallisation && s.CrystallisationStrategy != UFPLSStrategy {
			t.Errorf("Strategy %d has invalid crystallisation: %v", i, s.CrystallisationStrategy)
		}
	}

	t.Logf("Generated %d strategies in Quick mode", len(quickStrategies))
}
