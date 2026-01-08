package main

import "fmt"

// FactorRegistry holds all available strategy factors and generates combinations
type FactorRegistry struct {
	factors map[FactorID]*Factor
	order   []FactorID // Determines generation order
}

// NewFactorRegistry creates a registry with all standard factors
func NewFactorRegistry() *FactorRegistry {
	r := &FactorRegistry{
		factors: make(map[FactorID]*Factor),
		order:   make([]FactorID, 0),
	}

	// Register crystallisation factor
	r.Register(&Factor{
		ID:          FactorCrystallisation,
		Name:        "Crystallisation Strategy",
		Description: "How pension pots are crystallised for tax purposes",
		Values: []FactorValue{
			{ID: "gradual", Name: "Gradual Crystallisation", ShortName: "Grad", Value: GradualCrystallisation},
			{ID: "ufpls", Name: "UFPLS", ShortName: "UFPLS", Value: UFPLSStrategy},
		},
		DefaultValueID: "gradual",
	})

	// Register drawdown order factor
	r.Register(&Factor{
		ID:          FactorDrawdown,
		Name:        "Drawdown Order",
		Description: "Order in which to withdraw from different account types",
		Values: []FactorValue{
			{ID: "savings_first", Name: "Savings First", ShortName: "ISAFirst", Value: SavingsFirst},
			{ID: "pension_first", Name: "Pension First", ShortName: "PenFirst", Value: PensionFirst},
			{ID: "tax_optimized", Name: "Tax Optimized", ShortName: "TaxOpt", Value: TaxOptimized},
			{ID: "pension_to_isa", Name: "Pension to ISA", ShortName: "Pen2ISA", Value: PensionToISA},
			{ID: "pension_to_isa_proactive", Name: "Pension to ISA (Proactive)", ShortName: "Pen2ISA+", Value: PensionToISAProactive},
			{ID: "pension_only", Name: "Pension Only", ShortName: "PenOnly", Value: PensionOnly},
			{ID: "fill_basic_rate", Name: "Fill Basic Rate", ShortName: "FillBasic", Value: FillBasicRate},
			{ID: "state_pension_bridge", Name: "State Pension Bridge", ShortName: "SPBridge", Value: StatePensionBridge},
		},
		DefaultValueID: "tax_optimized",
	})

	// Register mortgage factor
	r.Register(&Factor{
		ID:          FactorMortgage,
		Name:        "Mortgage Option",
		Description: "How the mortgage is paid off",
		Values: []FactorValue{
			{ID: "early", Name: "Early Payoff", ShortName: "Early", Value: MortgageEarly},
			{ID: "normal", Name: "Normal Payoff", ShortName: "Normal", Value: MortgageNormal},
			{ID: "extended", Name: "Extended +10y", ShortName: "Ext+10", Value: MortgageExtended},
			{ID: "pcls", Name: "PCLS Payoff", ShortName: "PCLS", Value: PCLSMortgagePayoff},
		},
		DefaultValueID: "normal",
	})

	// Register maximize couple ISA factor
	r.Register(&Factor{
		ID:          FactorMaximizeCoupleISA,
		Name:        "Maximize Couple ISA",
		Description: "Fill both people's ISA allowances from one pension (for PensionToISA strategies)",
		Values: []FactorValue{
			{ID: "off", Name: "Disabled", ShortName: "Off", Value: false},
			{ID: "on", Name: "Enabled", ShortName: "On", Value: true},
		},
		DefaultValueID: "on",
		DependsOn:      []FactorID{FactorDrawdown},
	})

	// Register ISA to SIPP factor
	r.Register(&Factor{
		ID:          FactorISAToSIPP,
		Name:        "ISA to SIPP Transfers",
		Description: "Transfer ISA to pension while working for tax relief",
		Values: []FactorValue{
			{ID: "off", Name: "Disabled", ShortName: "Off", Value: false},
			{ID: "on", Name: "Enabled", ShortName: "On", Value: true},
		},
		DefaultValueID: "off",
	})

	// Register guardrails factor
	r.Register(&Factor{
		ID:          FactorGuardrails,
		Name:        "Guardrails",
		Description: "Guyton-Klinger dynamic withdrawal adjustments",
		Values: []FactorValue{
			{ID: "off", Name: "Disabled", ShortName: "Off", Value: false},
			{ID: "on", Name: "Enabled", ShortName: "On", Value: true},
		},
		DefaultValueID: "off",
	})

	// Register state pension deferral factor
	r.Register(&Factor{
		ID:          FactorStatePensionDefer,
		Name:        "State Pension Deferral",
		Description: "Years to defer state pension (5.8%/year enhancement)",
		Values: []FactorValue{
			{ID: "0y", Name: "No Deferral", ShortName: "0y", Value: 0},
			{ID: "2y", Name: "2 Years", ShortName: "2y", Value: 2},
			{ID: "5y", Name: "5 Years", ShortName: "5y", Value: 5},
		},
		DefaultValueID: "0y",
	})

	return r
}

// Register adds a factor to the registry
func (r *FactorRegistry) Register(f *Factor) {
	r.factors[f.ID] = f
	r.order = append(r.order, f.ID)
}

// Get returns a factor by ID
func (r *FactorRegistry) Get(id FactorID) *Factor {
	return r.factors[id]
}

// GetAll returns all factors in registration order
func (r *FactorRegistry) GetAll() []*Factor {
	result := make([]*Factor, len(r.order))
	for i, id := range r.order {
		result[i] = r.factors[id]
	}
	return result
}

// GetApplicableFactors returns factors that apply to the given config
func (r *FactorRegistry) GetApplicableFactors(config *Config) []*Factor {
	result := make([]*Factor, 0, len(r.order))
	for _, id := range r.order {
		f := r.factors[id]
		if r.isFactorApplicable(f, config) {
			// Filter mortgage factor values based on config
			if f.ID == FactorMortgage {
				f = r.filterMortgageFactorByConfig(f, config)
			}
			result = append(result, f)
		}
	}
	return result
}

// filterMortgageFactorByConfig returns a mortgage factor with values filtered by config
func (r *FactorRegistry) filterMortgageFactorByConfig(f *Factor, config *Config) *Factor {
	// If extension is allowed, return all values (with dynamic name)
	if config.ShouldIncludeExtendedMortgage() {
		// Update the extended option name to show the actual year
		extendedYear := config.GetExtendedEndYear()
		values := make([]FactorValue, len(f.Values))
		copy(values, f.Values)
		for i, v := range values {
			if v.ID == "extended" {
				values[i].Name = fmt.Sprintf("Extended to %d", extendedYear)
				values[i].ShortName = fmt.Sprintf("Ext%d", extendedYear)
			}
		}
		return &Factor{
			ID:             f.ID,
			Name:           f.Name,
			Description:    f.Description,
			DefaultValueID: f.DefaultValueID,
			Values:         values,
		}
	}

	// Extension not allowed - filter out the extended option
	filteredValues := make([]FactorValue, 0, len(f.Values))
	for _, v := range f.Values {
		if v.ID != "extended" {
			filteredValues = append(filteredValues, v)
		}
	}
	return &Factor{
		ID:             f.ID,
		Name:           f.Name,
		Description:    f.Description,
		DefaultValueID: f.DefaultValueID,
		Values:         filteredValues,
	}
}

// isFactorApplicable checks if a factor applies to the given config
func (r *FactorRegistry) isFactorApplicable(f *Factor, config *Config) bool {
	switch f.ID {
	case FactorMortgage:
		// Only applicable if there is a mortgage
		return config.HasMortgage()
	case FactorISAToSIPP:
		// Only applicable if someone has work income
		for _, p := range config.People {
			if p.WorkIncome > 0 {
				return true
			}
		}
		return false
	case FactorMaximizeCoupleISA:
		// Only applicable for couples
		return len(config.People) >= 2
	default:
		return true
	}
}

// GetFactorsByMode returns factors filtered by permutation mode
func (r *FactorRegistry) GetFactorsByMode(config *Config, mode PermutationMode) []*Factor {
	applicable := r.GetApplicableFactors(config)
	result := make([]*Factor, 0)

	for _, f := range applicable {
		filteredFactor := r.filterFactorByMode(f, mode)
		if filteredFactor != nil && len(filteredFactor.Values) > 0 {
			result = append(result, filteredFactor)
		}
	}

	return result
}

// filterFactorByMode limits which values are used based on mode
func (r *FactorRegistry) filterFactorByMode(f *Factor, mode PermutationMode) *Factor {
	switch mode {
	case ModeQuick:
		// Only essential factors with default values
		return r.limitToEssentialFactors(f)
	case ModeStandard:
		// Common factors with common values
		return r.limitToCommonValues(f)
	case ModeThorough:
		// More factors but not exhaustive - targets ~200-300 combinations
		return r.limitToThoroughValues(f)
	case ModeComprehensive:
		// All factors, all values
		return f
	default:
		return f
	}
}

// limitToEssentialFactors returns only core factors for Quick mode
// Quick mode aims for ~15-25 combinations
func (r *FactorRegistry) limitToEssentialFactors(f *Factor) *Factor {
	// Quick mode: only Crystallisation, Drawdown, and Mortgage (if applicable)
	switch f.ID {
	case FactorCrystallisation:
		// Only Gradual in Quick mode for simplicity
		return &Factor{
			ID:             f.ID,
			Name:           f.Name,
			Description:    f.Description,
			DefaultValueID: f.DefaultValueID,
			Values:         filterValues(f.Values, []string{"gradual"}),
		}
	case FactorDrawdown:
		// Limit to 4 core drawdown orders
		return &Factor{
			ID:             f.ID,
			Name:           f.Name,
			Description:    f.Description,
			DefaultValueID: f.DefaultValueID,
			Values: filterValues(f.Values, []string{
				"savings_first", "pension_first", "tax_optimized", "pension_to_isa",
			}),
		}
	case FactorMortgage:
		// Limit to normal and early
		return &Factor{
			ID:             f.ID,
			Name:           f.Name,
			Description:    f.Description,
			DefaultValueID: f.DefaultValueID,
			Values:         filterValues(f.Values, []string{"normal", "early"}),
		}
	default:
		// Other factors not included in Quick mode
		return nil
	}
}

// limitToCommonValues returns factors with commonly used values for Standard mode
// Standard mode aims for ~50-100 combinations
func (r *FactorRegistry) limitToCommonValues(f *Factor) *Factor {
	switch f.ID {
	case FactorCrystallisation:
		// Both crystallisation options in Standard
		return f
	case FactorDrawdown:
		// Include most drawdown orders except proactive variants
		return &Factor{
			ID:             f.ID,
			Name:           f.Name,
			Description:    f.Description,
			DefaultValueID: f.DefaultValueID,
			Values: filterValues(f.Values, []string{
				"savings_first", "pension_first", "tax_optimized",
				"pension_to_isa", "fill_basic_rate",
			}),
		}
	case FactorMortgage:
		// All mortgage options in Standard
		return f
	case FactorGuardrails:
		// Include guardrails in Standard
		return f
	case FactorISAToSIPP:
		// Only include ISAToSIPP off in Standard (enabled only in Comprehensive)
		return &Factor{
			ID:             f.ID,
			Name:           f.Name,
			Description:    f.Description,
			DefaultValueID: f.DefaultValueID,
			Values:         filterValues(f.Values, []string{"off"}),
		}
	case FactorMaximizeCoupleISA:
		// Skip MaximizeCoupleISA in Standard (it's dependent on PensionToISA)
		return nil
	case FactorStatePensionDefer:
		// Only no deferral in Standard
		return &Factor{
			ID:             f.ID,
			Name:           f.Name,
			Description:    f.Description,
			DefaultValueID: f.DefaultValueID,
			Values:         filterValues(f.Values, []string{"0y"}),
		}
	default:
		return f
	}
}

// limitToThoroughValues returns factors for Thorough mode (~200-300 combinations)
// Includes more factors than Standard but not all options from Comprehensive
func (r *FactorRegistry) limitToThoroughValues(f *Factor) *Factor {
	switch f.ID {
	case FactorCrystallisation:
		// Both crystallisation options
		return f
	case FactorDrawdown:
		// Include all main drawdown orders (excluding proactive)
		return &Factor{
			ID:             f.ID,
			Name:           f.Name,
			Description:    f.Description,
			DefaultValueID: f.DefaultValueID,
			Values: filterValues(f.Values, []string{
				"savings_first", "pension_first", "tax_optimized",
				"pension_to_isa", "pension_only", "fill_basic_rate", "state_pension_bridge",
			}),
		}
	case FactorMortgage:
		// All mortgage options
		return f
	case FactorGuardrails:
		// Include guardrails
		return f
	case FactorISAToSIPP:
		// Include ISA to SIPP in Thorough
		return f
	case FactorMaximizeCoupleISA:
		// Include for couples
		return f
	case FactorStatePensionDefer:
		// Include 0 and 2 year deferral options
		return &Factor{
			ID:             f.ID,
			Name:           f.Name,
			Description:    f.Description,
			DefaultValueID: f.DefaultValueID,
			Values:         filterValues(f.Values, []string{"0y", "2y"}),
		}
	default:
		return f
	}
}

// filterValues returns only values with IDs in the allowedIDs list
func filterValues(values []FactorValue, allowedIDs []string) []FactorValue {
	result := make([]FactorValue, 0)
	allowedSet := make(map[string]bool)
	for _, id := range allowedIDs {
		allowedSet[id] = true
	}
	for _, v := range values {
		if allowedSet[v.ID] {
			result = append(result, v)
		}
	}
	return result
}

// GetDefaultCombo returns a StrategyCombo with all default values
func (r *FactorRegistry) GetDefaultCombo() StrategyCombo {
	combo := StrategyCombo{Values: make(map[FactorID]FactorValue)}
	for _, f := range r.GetAll() {
		for _, v := range f.Values {
			if v.ID == f.DefaultValueID {
				combo.Values[f.ID] = v
				break
			}
		}
	}
	return combo
}
