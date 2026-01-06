package main

import (
	_ "embed"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed default-config.yaml
var defaultConfigYAML string

// PersonConfig represents a person's configuration from YAML
type PersonConfig struct {
	Name            string  `yaml:"name" json:"name"`
	BirthDate       string  `yaml:"birth_date" json:"birth_date"`
	RetirementAge   int     `yaml:"retirement_age" json:"retirement_age"`
	StatePensionAge int     `yaml:"state_pension_age" json:"state_pension_age"`
	TaxFreeSavings  float64 `yaml:"tax_free_savings" json:"tax_free_savings"`
	Pension         float64 `yaml:"pension" json:"pension"`
	ISAAnnualLimit  float64 `yaml:"isa_annual_limit" json:"isa_annual_limit"` // Per-person ISA annual limit (default 20000)

	// DB Pension Configuration
	DBPensionAmount        float64 `yaml:"db_pension_amount" json:"db_pension_amount"`                 // Annual DB pension at normal retirement age
	DBPensionStartAge      int     `yaml:"db_pension_start_age" json:"db_pension_start_age"`           // Age when DB pension starts (can differ from normal retirement age)
	DBPensionName          string  `yaml:"db_pension_name" json:"db_pension_name"`                     // Name of DB pension scheme
	DBPensionNormalAge     int     `yaml:"db_pension_normal_age" json:"db_pension_normal_age"`         // Normal retirement age for the scheme
	DBPensionEarlyFactor   float64 `yaml:"db_pension_early_factor" json:"db_pension_early_factor"`     // Reduction per year early (e.g., 0.04 = 4% per year)
	DBPensionLateFactor    float64 `yaml:"db_pension_late_factor" json:"db_pension_late_factor"`       // Increase per year late (e.g., 0.05 = 5% per year)
	DBPensionCommutation   float64 `yaml:"db_pension_commutation" json:"db_pension_commutation"`       // Fraction to commute (0-0.25, e.g., 0.25 = take 25% as lump sum)
	DBPensionCommuteFactor float64 `yaml:"db_pension_commute_factor" json:"db_pension_commute_factor"` // Commutation factor (e.g., 12 = £12 lump sum per £1 pension given up)

	// State Pension Deferral
	StatePensionDeferYears int `yaml:"state_pension_defer_years" json:"state_pension_defer_years"` // Years to defer state pension (0 = no deferral)

	// Phased Retirement (Part-time work)
	PartTimeIncome   float64 `yaml:"part_time_income" json:"part_time_income"`       // Annual income from part-time work
	PartTimeStartAge int     `yaml:"part_time_start_age" json:"part_time_start_age"` // Age when part-time work starts
	PartTimeEndAge   int     `yaml:"part_time_end_age" json:"part_time_end_age"`     // Age when part-time work ends
}

// FinancialConfig holds growth and inflation rates
type FinancialConfig struct {
	// Growth Rate Source: "custom" for manual entry, or stock index ID (e.g., "sp500", "ftse100")
	GrowthRateSource      string `yaml:"growth_rate_source,omitempty" json:"growth_rate_source,omitempty"`
	GrowthRatePeriodYears int    `yaml:"growth_rate_period_years,omitempty" json:"growth_rate_period_years,omitempty"` // Selected time period (3, 5, 10, 25, etc.)

	PensionGrowthRate float64 `yaml:"pension_growth_rate" json:"pension_growth_rate"`
	SavingsGrowthRate float64 `yaml:"savings_growth_rate" json:"savings_growth_rate"`
	IncomeInflationRate      float64 `yaml:"income_inflation_rate" json:"income_inflation_rate"`
	StatePensionAmount       float64 `yaml:"state_pension_amount" json:"state_pension_amount"`
	StatePensionInflation    float64 `yaml:"state_pension_inflation" json:"state_pension_inflation"`
	TaxBandInflation         float64 `yaml:"tax_band_inflation" json:"tax_band_inflation"`                   // Annual inflation rate for tax bands
	StatePensionDeferralRate float64 `yaml:"state_pension_deferral_rate" json:"state_pension_deferral_rate"` // Enhancement per year deferred (default 5.8% = 0.058)
	// Emergency Fund Preservation
	EmergencyFundMonths          int  `yaml:"emergency_fund_months" json:"emergency_fund_months"`                     // Minimum months of expenses to keep in ISA (0 = no minimum)
	EmergencyFundInflationAdjust bool `yaml:"emergency_fund_inflation_adjust" json:"emergency_fund_inflation_adjust"` // Grow threshold with inflation
	// Gradual Growth Rate Decline (models "age in bonds" strategy - shifting from equities to bonds)
	GrowthDeclineEnabled         bool    `yaml:"growth_decline_enabled" json:"growth_decline_enabled"`                   // Enable gradual decline of growth rates
	PensionGrowthEndRate         float64 `yaml:"pension_growth_end_rate" json:"pension_growth_end_rate"`                 // Pension growth rate at target age
	SavingsGrowthEndRate         float64 `yaml:"savings_growth_end_rate" json:"savings_growth_end_rate"`                 // ISA growth rate at target age
	GrowthDeclineTargetAge       int     `yaml:"growth_decline_target_age" json:"growth_decline_target_age"`             // Age when growth rates reach end rate
	GrowthDeclineReferencePerson string  `yaml:"growth_decline_reference_person" json:"growth_decline_reference_person"` // Whose age determines decline (default: simulation reference person)
	// Depletion Mode Growth Decline (simpler: decline by X% over the depletion period)
	DepletionGrowthDeclineEnabled bool    `yaml:"depletion_growth_decline_enabled" json:"depletion_growth_decline_enabled"` // Enable growth decline in depletion mode
	DepletionGrowthDeclinePercent float64 `yaml:"depletion_growth_decline_percent" json:"depletion_growth_decline_percent"` // Percentage to decline (e.g., 0.03 = 3%, so 7% -> 4%)
}

// IncomeTier represents a single income tier with an age range
// Tiers are specified with start and end ages. Use nil (omit in YAML) for open-ended ranges:
//   - end_age: 64 → from start until age 64
//   - start_age: 64, end_age: 70 → from age 64 to 70
//   - start_age: 70 → from age 70 until end
//
// Income types (mutually exclusive):
//   - Fixed: MonthlyAmount is £/month
//   - Percentage: IsPercentage=true, MonthlyAmount is annual % of initial portfolio
//   - Investment Gains: IsInvestmentGains=true, income = real returns (growth - inflation)
type IncomeTier struct {
	StartAge           *int    `yaml:"start_age,omitempty" json:"start_age,omitempty"`               // Age when tier starts (nil = from retirement)
	EndAge             *int    `yaml:"end_age,omitempty" json:"end_age,omitempty"`                   // Age when tier ends (nil = until simulation end)
	MonthlyAmount      float64 `yaml:"monthly_amount" json:"monthly_amount"`                         // Monthly income (£) or percentage if IsPercentage
	Ratio              float64 `yaml:"ratio" json:"ratio"`                                           // For depletion mode: ratio relative to other tiers
	IsPercentage       bool    `yaml:"is_percentage,omitempty" json:"is_percentage"`                 // If true, MonthlyAmount is annual % of initial portfolio
	IsInvestmentGains  bool    `yaml:"is_investment_gains,omitempty" json:"is_investment_gains"`     // If true, income = investment gains after inflation
}

// IncomeConfig holds income requirement settings
type IncomeConfig struct {
	// Tiered income (new system) - takes precedence over legacy fields if non-empty
	Tiers []IncomeTier `yaml:"tiers,omitempty" json:"tiers,omitempty"`

	// Legacy fixed income mode (used when Tiers is empty and TargetDepletionAge is 0)
	MonthlyBeforeAge float64 `yaml:"monthly_before_age,omitempty" json:"monthly_before_age,omitempty"`
	MonthlyAfterAge  float64 `yaml:"monthly_after_age,omitempty" json:"monthly_after_age,omitempty"`

	// Depletion mode (used when TargetDepletionAge > 0)
	TargetDepletionAge int     `yaml:"target_depletion_age" json:"target_depletion_age"` // If > 0, calculate income to deplete at this age
	IncomeRatioPhase1  float64 `yaml:"income_ratio_phase1" json:"income_ratio_phase1"`   // Legacy: e.g., 5.0 for 5:3 ratio (used if Tiers empty)
	IncomeRatioPhase2  float64 `yaml:"income_ratio_phase2" json:"income_ratio_phase2"`   // Legacy: e.g., 3.0 for 5:3 ratio (used if Tiers empty)

	// Guardrails Strategy (Guyton-Klinger) - dynamic withdrawal adjustments
	GuardrailsEnabled    bool    `yaml:"guardrails_enabled" json:"guardrails_enabled"`         // Enable guardrails adjustments
	GuardrailsUpperLimit float64 `yaml:"guardrails_upper_limit" json:"guardrails_upper_limit"` // Upper guardrail (e.g., 1.20 = 120% of initial rate)
	GuardrailsLowerLimit float64 `yaml:"guardrails_lower_limit" json:"guardrails_lower_limit"` // Lower guardrail (e.g., 0.80 = 80% of initial rate)
	GuardrailsAdjustment float64 `yaml:"guardrails_adjustment" json:"guardrails_adjustment"`   // Adjustment percentage (e.g., 0.10 = 10%)

	// VPW (Variable Percentage Withdrawal) Strategy
	VPWEnabled bool    `yaml:"vpw_enabled" json:"vpw_enabled"` // Enable VPW-based income calculation
	VPWFloor   float64 `yaml:"vpw_floor" json:"vpw_floor"`     // Minimum annual withdrawal (optional floor)
	VPWCeiling float64 `yaml:"vpw_ceiling" json:"vpw_ceiling"` // Maximum as multiple of floor (e.g., 1.5 = 150%)

	// Legacy common field (used when Tiers is empty)
	AgeThreshold    int    `yaml:"age_threshold,omitempty" json:"age_threshold,omitempty"`
	ReferencePerson string `yaml:"reference_person" json:"reference_person"`
}

// IsDepletionMode returns true if depletion mode is configured
func (ic *IncomeConfig) IsDepletionMode() bool {
	return ic.TargetDepletionAge > 0
}

// HasTiers returns true if tiered income is configured
func (ic *IncomeConfig) HasTiers() bool {
	return len(ic.Tiers) > 0
}

// HasPercentageTiers returns true if any tier uses percentage-based income
func (ic *IncomeConfig) HasPercentageTiers() bool {
	for _, tier := range ic.Tiers {
		if tier.IsPercentage {
			return true
		}
	}
	return false
}

// HasInvestmentGainsTiers returns true if any tier uses investment gains income
func (ic *IncomeConfig) HasInvestmentGainsTiers() bool {
	for _, tier := range ic.Tiers {
		if tier.IsInvestmentGains {
			return true
		}
	}
	return false
}

// GetTierForAge returns the income tier applicable for a given age
// Returns nil if no tier matches (shouldn't happen with properly configured tiers)
func (ic *IncomeConfig) GetTierForAge(age int) *IncomeTier {
	for i := range ic.Tiers {
		tier := &ic.Tiers[i]
		// Check if age falls within this tier's range
		startOK := tier.StartAge == nil || age >= *tier.StartAge
		endOK := tier.EndAge == nil || age < *tier.EndAge
		if startOK && endOK {
			return tier
		}
	}
	// If no tier matches, return the last tier (open-ended)
	if len(ic.Tiers) > 0 {
		return &ic.Tiers[len(ic.Tiers)-1]
	}
	return nil
}

// GetMonthlyIncomeForAge returns the monthly income for a given age
// initialPortfolio is used when tiers specify percentage-based income
// multiplier is used in depletion mode to scale ratio-based tiers
// Note: For IsInvestmentGains tiers, returns -1 (calculation happens in simulation.go)
func (ic *IncomeConfig) GetMonthlyIncomeForAge(age int, initialPortfolio, multiplier float64) float64 {
	if !ic.HasTiers() {
		// Legacy mode: use before/after age threshold
		if ic.IsDepletionMode() {
			if age < ic.AgeThreshold {
				return ic.IncomeRatioPhase1 * multiplier
			}
			return ic.IncomeRatioPhase2 * multiplier
		}
		if age < ic.AgeThreshold {
			return ic.MonthlyBeforeAge
		}
		return ic.MonthlyAfterAge
	}

	tier := ic.GetTierForAge(age)
	if tier == nil {
		return 0
	}

	// Investment gains tiers: return -1 to signal simulation.go should calculate
	// Income = (pension × pension_growth_rate + savings × savings_growth_rate) - inflation
	if tier.IsInvestmentGains {
		return -1
	}

	// In depletion mode, use ratios scaled by multiplier
	if ic.IsDepletionMode() && tier.Ratio > 0 {
		return tier.Ratio * multiplier
	}

	// For percentage-based tiers: MonthlyAmount is annual % of initial portfolio
	// e.g., 6% of £1M = £60k/year = £5k/month
	if tier.IsPercentage {
		annualAmount := initialPortfolio * (tier.MonthlyAmount / 100.0)
		return annualAmount / 12.0
	}

	// Absolute monthly amount
	return tier.MonthlyAmount
}

// GetAnnualIncomeForAge returns the annual income for a given age
func (ic *IncomeConfig) GetAnnualIncomeForAge(age int, initialPortfolio, multiplier float64) float64 {
	return ic.GetMonthlyIncomeForAge(age, initialPortfolio, multiplier) * 12
}

// GetRatioForAge returns the ratio for a tier at a given age (for depletion mode)
func (ic *IncomeConfig) GetRatioForAge(age int) float64 {
	if !ic.HasTiers() {
		// Legacy mode
		if age < ic.AgeThreshold {
			return ic.IncomeRatioPhase1
		}
		return ic.IncomeRatioPhase2
	}

	tier := ic.GetTierForAge(age)
	if tier == nil {
		return 1.0
	}
	if tier.Ratio > 0 {
		return tier.Ratio
	}
	return 1.0
}

// GetIncomeAmounts returns monthly income amounts for a given multiplier (legacy compatibility)
// In fixed mode, multiplier is ignored and fixed amounts are returned
// In depletion mode, returns amounts based on ratios and multiplier
func (ic *IncomeConfig) GetIncomeAmounts(multiplier float64) (beforeAge, afterAge float64) {
	if !ic.IsDepletionMode() {
		return ic.MonthlyBeforeAge, ic.MonthlyAfterAge
	}
	return ic.IncomeRatioPhase1 * multiplier, ic.IncomeRatioPhase2 * multiplier
}

// ConvertLegacyToTiers converts legacy before/after age fields to tiered format
// This is useful for migration and consistent internal handling
func (ic *IncomeConfig) ConvertLegacyToTiers() {
	if ic.HasTiers() {
		return // Already has tiers
	}

	threshold := ic.AgeThreshold
	if threshold == 0 {
		threshold = 67 // Default
	}

	if ic.IsDepletionMode() {
		ic.Tiers = []IncomeTier{
			{EndAge: &threshold, Ratio: ic.IncomeRatioPhase1},
			{StartAge: &threshold, Ratio: ic.IncomeRatioPhase2},
		}
	} else {
		ic.Tiers = []IncomeTier{
			{EndAge: &threshold, MonthlyAmount: ic.MonthlyBeforeAge},
			{StartAge: &threshold, MonthlyAmount: ic.MonthlyAfterAge},
		}
	}
}

// DescribeTiers returns a human-readable description of the income tiers
func (ic *IncomeConfig) DescribeTiers(initialPortfolio float64) string {
	if !ic.HasTiers() {
		return ""
	}

	var parts []string
	for _, tier := range ic.Tiers {
		var ageRange string
		if tier.StartAge == nil && tier.EndAge != nil {
			ageRange = "until " + strconv.Itoa(*tier.EndAge)
		} else if tier.StartAge != nil && tier.EndAge == nil {
			ageRange = strconv.Itoa(*tier.StartAge) + "+"
		} else if tier.StartAge != nil && tier.EndAge != nil {
			ageRange = strconv.Itoa(*tier.StartAge) + "-" + strconv.Itoa(*tier.EndAge)
		} else {
			ageRange = "all ages"
		}

		var amount string
		if ic.IsDepletionMode() && tier.Ratio > 0 {
			amount = strconv.FormatFloat(tier.Ratio, 'f', 1, 64) + "x"
		} else if tier.IsPercentage {
			amount = strconv.FormatFloat(tier.MonthlyAmount, 'f', 1, 64) + "%"
			if initialPortfolio > 0 {
				monthly := initialPortfolio * (tier.MonthlyAmount / 100.0) / 12.0
				amount += " (£" + formatDefaultMoney(monthly) + "/mo)"
			}
		} else {
			amount = "£" + formatDefaultMoney(tier.MonthlyAmount) + "/mo"
		}

		parts = append(parts, ageRange+": "+amount)
	}

	return strings.Join(parts, ", ")
}

// MortgagePartConfig holds details for one mortgage part
type MortgagePartConfig struct {
	Name         string  `yaml:"name" json:"name"`                   // e.g., "Repayment" or "Interest Only"
	Principal    float64 `yaml:"principal" json:"principal"`         // Original loan amount
	InterestRate float64 `yaml:"interest_rate" json:"interest_rate"` // Annual interest rate (e.g., 0.0389 = 3.89%)
	IsRepayment  bool    `yaml:"is_repayment" json:"is_repayment"`   // true = repayment, false = interest-only
	TermYears    int     `yaml:"term_years" json:"term_years"`       // Term in years (for repayment mortgages)
	StartYear    int     `yaml:"start_year" json:"start_year"`       // Year mortgage started (to calculate remaining balance)
}

// MortgageConfig holds mortgage details
type MortgageConfig struct {
	Parts           []MortgagePartConfig `yaml:"parts" json:"parts"`                         // Individual mortgage parts
	EndYear         int                  `yaml:"end_year" json:"end_year"`                   // Year all mortgages end (for normal payoff scenario)
	EarlyPayoffYear int                  `yaml:"early_payoff_year" json:"early_payoff_year"` // Year when early payoff can happen (if tied in)
}

// SimulationConfig holds simulation parameters
type SimulationConfig struct {
	StartYear       int    `yaml:"start_year" json:"start_year"`
	EndAge          int    `yaml:"end_age" json:"end_age"`
	ReferencePerson string `yaml:"reference_person" json:"reference_person"`
}

// SensitivityConfig holds sensitivity analysis parameters
type SensitivityConfig struct {
	PensionGrowthMin float64 `yaml:"pension_growth_min" json:"pension_growth_min"` // Min pension growth rate (e.g., 0.04 = 4%)
	PensionGrowthMax float64 `yaml:"pension_growth_max" json:"pension_growth_max"` // Max pension growth rate (e.g., 0.12 = 12%)
	SavingsGrowthMin float64 `yaml:"savings_growth_min" json:"savings_growth_min"` // Min savings growth rate
	SavingsGrowthMax float64 `yaml:"savings_growth_max" json:"savings_growth_max"` // Max savings growth rate
	StepSize         float64 `yaml:"step_size" json:"step_size"`                   // Step size (e.g., 0.01 = 1%)
}

// StrategyConfig holds strategy-specific options
type StrategyConfig struct {
	// MaximizeCoupleISA allows one person's pension to over-withdraw to fill both
	// people's ISA allowances, even if the second person can't access their pension yet.
	// This is beneficial for couples where one retires earlier. Default: true
	MaximizeCoupleISA *bool `yaml:"maximize_couple_isa" json:"maximize_couple_isa"`
}

// ShouldMaximizeCoupleISA returns whether to maximize ISA transfers for couples (default: true)
func (s *StrategyConfig) ShouldMaximizeCoupleISA() bool {
	if s.MaximizeCoupleISA == nil {
		return true // default to true
	}
	return *s.MaximizeCoupleISA
}

// TaxBand represents a tax band from configuration
type TaxBand struct {
	Name  string  `yaml:"name" json:"name"`
	Lower float64 `yaml:"lower" json:"lower"`
	Upper float64 `yaml:"upper" json:"upper"`
	Rate  float64 `yaml:"rate" json:"rate"`
}

// TaxConfig holds UK tax configuration including personal allowance tapering
// These values are set by HMRC and may change with each tax year
type TaxConfig struct {
	// Personal Allowance is the amount you can earn tax-free (2024/25: £12,570)
	PersonalAllowance float64 `yaml:"personal_allowance" json:"personal_allowance"`
	// TaperingThreshold is the income level above which personal allowance starts to reduce (2024/25: £100,000)
	TaperingThreshold float64 `yaml:"tapering_threshold" json:"tapering_threshold"`
	// TaperingRate is how much allowance is lost per £1 over threshold (2024/25: £0.50, so £1 lost per £2 earned)
	TaperingRate float64 `yaml:"tapering_rate" json:"tapering_rate"`
}

// GetPersonalAllowance returns the personal allowance, using default if not set
func (tc *TaxConfig) GetPersonalAllowance() float64 {
	if tc.PersonalAllowance <= 0 {
		return 12570.0 // 2024/25 default
	}
	return tc.PersonalAllowance
}

// GetTaperingThreshold returns the tapering threshold, using default if not set
func (tc *TaxConfig) GetTaperingThreshold() float64 {
	if tc.TaperingThreshold <= 0 {
		return 100000.0 // 2024/25 default
	}
	return tc.TaperingThreshold
}

// GetTaperingRate returns the tapering rate, using default if not set
func (tc *TaxConfig) GetTaperingRate() float64 {
	if tc.TaperingRate <= 0 {
		return 0.5 // 2024/25 default: £1 lost per £2 over threshold
	}
	return tc.TaperingRate
}

// GetAllowanceRemovedThreshold returns the income at which personal allowance is fully removed
// This is calculated from personal allowance, tapering threshold and rate
func (tc *TaxConfig) GetAllowanceRemovedThreshold() float64 {
	pa := tc.GetPersonalAllowance()
	threshold := tc.GetTaperingThreshold()
	rate := tc.GetTaperingRate()
	// PA is reduced by rate for each £1 over threshold
	// PA = 0 when: threshold + (PA / rate)
	return threshold + (pa / rate)
}

// DefaultTaxConfig returns the default UK tax configuration for 2024/25
func DefaultTaxConfig() TaxConfig {
	return TaxConfig{
		PersonalAllowance: 12570.0,
		TaperingThreshold: 100000.0,
		TaperingRate:      0.5,
	}
}

// Config holds the complete configuration
type Config struct {
	People             []PersonConfig    `yaml:"people" json:"people"`
	Financial          FinancialConfig   `yaml:"financial" json:"financial"`
	IncomeRequirements IncomeConfig      `yaml:"income_requirements" json:"income_requirements"`
	Mortgage           MortgageConfig    `yaml:"mortgage" json:"mortgage"`
	Simulation         SimulationConfig  `yaml:"simulation" json:"simulation"`
	Sensitivity        SensitivityConfig `yaml:"sensitivity" json:"sensitivity"`
	Strategy           StrategyConfig    `yaml:"strategy" json:"strategy"`
	TaxBands           []TaxBand         `yaml:"tax_bands" json:"tax_bands"`
	Tax                TaxConfig         `yaml:"tax" json:"tax"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig saves configuration to a YAML file
func SaveConfig(config *Config, filename string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	// Add a header comment with instructions
	header := []byte(`# Pension Forecast Configuration
# Generated interactively - feel free to edit manually
#
# ═══════════════════════════════════════════════════════════════════════════════
# TWO OPERATING MODES
# ═══════════════════════════════════════════════════════════════════════════════
#
# FIXED INCOME MODE (default):
#   You specify monthly income needs, simulator shows how long funds last.
#   Key settings:
#     income_requirements.monthly_before_age: £/month before age threshold
#     income_requirements.monthly_after_age:  £/month after age threshold
#   Question answered: "Can I afford £X/month? How long will funds last?"
#
# DEPLETION MODE (-depletion flag):
#   You specify target age, simulator calculates maximum sustainable income.
#   Key settings:
#     income_requirements.target_depletion_age: Age to deplete funds by
#     income_requirements.income_ratio_phase1:  Ratio before threshold (e.g., 5)
#     income_requirements.income_ratio_phase2:  Ratio after threshold (e.g., 3)
#   Question answered: "How much can I spend to last until age X?"
#   Note: Ratio 5:3 means income before threshold is 5/3 times income after.
#
# ═══════════════════════════════════════════════════════════════════════════════
# VALUE FORMATS
# ═══════════════════════════════════════════════════════════════════════════════
#   Percentages: 0.05 = 5% (enter as decimal)
#   Money: values are in GBP (e.g., 500000 = £500k)
#   Dates: YYYY-MM-DD format (e.g., 1975-06-15)
#
# ═══════════════════════════════════════════════════════════════════════════════
# RUN COMMANDS
# ═══════════════════════════════════════════════════════════════════════════════
#   ./goPensionForecast                       Interactive mode selector
#   ./goPensionForecast -html                 Fixed income mode with HTML reports
#   ./goPensionForecast -depletion            Depletion mode (console)
#   ./goPensionForecast -depletion -html      Depletion mode with HTML reports
#   ./goPensionForecast -sensitivity          Fixed income sensitivity analysis
#   ./goPensionForecast -depletion -sensitivity  Depletion sensitivity analysis
#   ./goPensionForecast -help                 Show all options
#
# See default-config.yaml for all available options with detailed comments.

`)
	content := append(header, data...)
	return os.WriteFile(filename, content, 0644)
}

// LoadDefaultConfig loads the default configuration from embedded default-config.yaml
// It handles percentage format (e.g., "5%" -> 0.05)
func LoadDefaultConfig() (*Config, error) {
	// Use embedded default config (compiled into binary)
	content := preprocessPercentages(defaultConfigYAML)

	var config Config
	err := yaml.Unmarshal([]byte(content), &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// preprocessPercentages converts percentage values like "5%" to decimal "0.05"
func preprocessPercentages(content string) string {
	// Match patterns like: key: 5% or key: 3.89%
	// But not inside strings (already quoted)
	re := regexp.MustCompile(`(:\s*)(\d+\.?\d*)%`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the number before %
		parts := re.FindStringSubmatch(match)
		if len(parts) >= 3 {
			numStr := parts[2]
			num, err := strconv.ParseFloat(numStr, 64)
			if err == nil {
				return parts[1] + strconv.FormatFloat(num/100.0, 'f', -1, 64)
			}
		}
		return match
	})
}

// GetDefaultValue returns a default value from the default config for display purposes
func GetDefaultValue(fieldPath string, defaultConfig *Config) string {
	if defaultConfig == nil {
		return ""
	}

	switch fieldPath {
	// Person fields
	case "person.name":
		if len(defaultConfig.People) > 0 {
			return defaultConfig.People[0].Name
		}
	case "person.birth_date":
		if len(defaultConfig.People) > 0 {
			return defaultConfig.People[0].BirthDate
		}
	case "person.retirement_age":
		if len(defaultConfig.People) > 0 {
			return strconv.Itoa(defaultConfig.People[0].RetirementAge)
		}
	case "person.state_pension_age":
		if len(defaultConfig.People) > 0 {
			return strconv.Itoa(defaultConfig.People[0].StatePensionAge)
		}
	case "person.pension":
		if len(defaultConfig.People) > 0 {
			return formatDefaultMoney(defaultConfig.People[0].Pension)
		}
	case "person.tax_free_savings":
		if len(defaultConfig.People) > 0 {
			return formatDefaultMoney(defaultConfig.People[0].TaxFreeSavings)
		}

	// Person2 fields
	case "person2.name":
		if len(defaultConfig.People) > 1 {
			return defaultConfig.People[1].Name
		}
	case "person2.birth_date":
		if len(defaultConfig.People) > 1 {
			return defaultConfig.People[1].BirthDate
		}
	case "person2.retirement_age":
		if len(defaultConfig.People) > 1 {
			return strconv.Itoa(defaultConfig.People[1].RetirementAge)
		}
	case "person2.state_pension_age":
		if len(defaultConfig.People) > 1 {
			return strconv.Itoa(defaultConfig.People[1].StatePensionAge)
		}
	case "person2.pension":
		if len(defaultConfig.People) > 1 {
			return formatDefaultMoney(defaultConfig.People[1].Pension)
		}
	case "person2.tax_free_savings":
		if len(defaultConfig.People) > 1 {
			return formatDefaultMoney(defaultConfig.People[1].TaxFreeSavings)
		}
	case "person2.db_pension_name":
		if len(defaultConfig.People) > 1 {
			return defaultConfig.People[1].DBPensionName
		}
	case "person2.db_pension_amount":
		if len(defaultConfig.People) > 1 {
			return formatDefaultMoney(defaultConfig.People[1].DBPensionAmount)
		}
	case "person2.db_pension_start_age":
		if len(defaultConfig.People) > 1 {
			return strconv.Itoa(defaultConfig.People[1].DBPensionStartAge)
		}

	// Financial fields
	case "financial.pension_growth_rate":
		return formatDefaultPercent(defaultConfig.Financial.PensionGrowthRate)
	case "financial.savings_growth_rate":
		return formatDefaultPercent(defaultConfig.Financial.SavingsGrowthRate)
	case "financial.income_inflation_rate":
		return formatDefaultPercent(defaultConfig.Financial.IncomeInflationRate)
	case "financial.state_pension_amount":
		return formatDefaultMoney(defaultConfig.Financial.StatePensionAmount)
	case "financial.state_pension_inflation":
		return formatDefaultPercent(defaultConfig.Financial.StatePensionInflation)

	// Income requirements
	case "income.monthly_before_age":
		return formatDefaultMoney(defaultConfig.IncomeRequirements.MonthlyBeforeAge)
	case "income.monthly_after_age":
		return formatDefaultMoney(defaultConfig.IncomeRequirements.MonthlyAfterAge)
	case "income.target_depletion_age":
		return strconv.Itoa(defaultConfig.IncomeRequirements.TargetDepletionAge)
	case "income.income_ratio_phase1":
		return strconv.FormatFloat(defaultConfig.IncomeRequirements.IncomeRatioPhase1, 'f', 0, 64)
	case "income.income_ratio_phase2":
		return strconv.FormatFloat(defaultConfig.IncomeRequirements.IncomeRatioPhase2, 'f', 0, 64)
	case "income.age_threshold":
		return strconv.Itoa(defaultConfig.IncomeRequirements.AgeThreshold)

	// Mortgage fields
	case "mortgage.end_year":
		return strconv.Itoa(defaultConfig.Mortgage.EndYear)
	case "mortgage.early_payoff_year":
		return strconv.Itoa(defaultConfig.Mortgage.EarlyPayoffYear)
	case "mortgage.principal":
		if len(defaultConfig.Mortgage.Parts) > 0 {
			return formatDefaultMoney(defaultConfig.Mortgage.Parts[0].Principal)
		}
	case "mortgage.interest_rate":
		if len(defaultConfig.Mortgage.Parts) > 0 {
			return formatDefaultPercent(defaultConfig.Mortgage.Parts[0].InterestRate)
		}
	case "mortgage.term_years":
		if len(defaultConfig.Mortgage.Parts) > 0 {
			return strconv.Itoa(defaultConfig.Mortgage.Parts[0].TermYears)
		}
	case "mortgage.start_year":
		if len(defaultConfig.Mortgage.Parts) > 0 {
			return strconv.Itoa(defaultConfig.Mortgage.Parts[0].StartYear)
		}

	// Simulation fields
	case "simulation.start_year":
		return strconv.Itoa(defaultConfig.Simulation.StartYear)
	case "simulation.end_age":
		return strconv.Itoa(defaultConfig.Simulation.EndAge)

	// Sensitivity fields
	case "sensitivity.pension_growth_min":
		return formatDefaultPercent(defaultConfig.Sensitivity.PensionGrowthMin)
	case "sensitivity.pension_growth_max":
		return formatDefaultPercent(defaultConfig.Sensitivity.PensionGrowthMax)
	case "sensitivity.savings_growth_min":
		return formatDefaultPercent(defaultConfig.Sensitivity.SavingsGrowthMin)
	case "sensitivity.savings_growth_max":
		return formatDefaultPercent(defaultConfig.Sensitivity.SavingsGrowthMax)
	case "sensitivity.step_size":
		return formatDefaultPercent(defaultConfig.Sensitivity.StepSize)
	}

	return ""
}

func formatDefaultMoney(amount float64) string {
	if amount >= 1000000 {
		return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(amount/1000000, 'f', 1, 64), "0"), ".") + "m"
	} else if amount >= 1000 {
		return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(amount/1000, 'f', 1, 64), "0"), ".") + "k"
	}
	return strconv.FormatFloat(amount, 'f', 0, 64)
}

func formatDefaultPercent(rate float64) string {
	return strconv.FormatFloat(rate*100, 'f', 2, 64) + "%"
}

// GetBirthYear extracts the birth year from a date string (YYYY-MM-DD)
func GetBirthYear(birthDate string) int {
	t, err := time.Parse("2006-01-02", birthDate)
	if err != nil {
		return 0
	}
	return t.Year()
}

// FindPerson finds a person by name in the config
func (c *Config) FindPerson(name string) *PersonConfig {
	for i := range c.People {
		if c.People[i].Name == name {
			return &c.People[i]
		}
	}
	return nil
}

// GetReferencePerson returns the reference person for income requirements
func (c *Config) GetReferencePerson() *PersonConfig {
	return c.FindPerson(c.IncomeRequirements.ReferencePerson)
}

// GetSimulationReferencePerson returns the reference person for simulation end
func (c *Config) GetSimulationReferencePerson() *PersonConfig {
	return c.FindPerson(c.Simulation.ReferencePerson)
}

// GetGrowthDeclineReferencePerson returns the reference person for growth decline
// Defaults to simulation reference person if not specified
func (c *Config) GetGrowthDeclineReferencePerson() *PersonConfig {
	if c.Financial.GrowthDeclineReferencePerson != "" {
		return c.FindPerson(c.Financial.GrowthDeclineReferencePerson)
	}
	return c.GetSimulationReferencePerson()
}

// CalculateMonthlyPayment calculates the monthly payment for a repayment mortgage
// Using formula: M = P * [r(1+r)^n] / [(1+r)^n - 1]
func (m *MortgagePartConfig) CalculateMonthlyPayment() float64 {
	if !m.IsRepayment || m.TermYears == 0 {
		// Interest-only: just pay interest
		return m.Principal * m.InterestRate / 12
	}

	monthlyRate := m.InterestRate / 12
	numPayments := float64(m.TermYears * 12)

	if monthlyRate == 0 {
		return m.Principal / numPayments
	}

	factor := math.Pow(1+monthlyRate, numPayments)
	return m.Principal * (monthlyRate * factor) / (factor - 1)
}

// CalculateAnnualPayment returns the total annual payment for this mortgage part
func (m *MortgagePartConfig) CalculateAnnualPayment() float64 {
	return m.CalculateMonthlyPayment() * 12
}

// CalculateRemainingBalance calculates the remaining principal balance at a given year
// For repayment mortgages: uses amortization formula
// For interest-only: always returns full principal
func (m *MortgagePartConfig) CalculateRemainingBalance(atYear int) float64 {
	if !m.IsRepayment {
		// Interest-only: full principal always remains
		return m.Principal
	}

	yearsElapsed := atYear - m.StartYear
	if yearsElapsed <= 0 {
		return m.Principal
	}

	endYear := m.StartYear + m.TermYears
	if atYear >= endYear {
		return 0 // Fully paid off
	}

	monthlyRate := m.InterestRate / 12
	totalPayments := float64(m.TermYears * 12)
	paymentsMade := float64(yearsElapsed * 12)

	if monthlyRate == 0 {
		// No interest: simple linear payoff
		return m.Principal * (1 - paymentsMade/totalPayments)
	}

	// Remaining balance formula: B = P * [(1+r)^n - (1+r)^p] / [(1+r)^n - 1]
	factorN := math.Pow(1+monthlyRate, totalPayments)
	factorP := math.Pow(1+monthlyRate, paymentsMade)

	return m.Principal * (factorN - factorP) / (factorN - 1)
}

// GetTotalAnnualPayment returns the combined annual payment for all mortgage parts
func (c *Config) GetTotalAnnualPayment() float64 {
	total := 0.0
	for _, part := range c.Mortgage.Parts {
		total += part.CalculateAnnualPayment()
	}
	return total
}

// GetTotalPayoffAmount returns the total amount needed to pay off all mortgages at a given year
func (c *Config) GetTotalPayoffAmount(atYear int) float64 {
	total := 0.0
	for _, part := range c.Mortgage.Parts {
		total += part.CalculateRemainingBalance(atYear)
	}
	return total
}

// HasMortgage returns true if there is an active mortgage with principal > 0
func (c *Config) HasMortgage() bool {
	if len(c.Mortgage.Parts) == 0 {
		return false
	}
	for _, part := range c.Mortgage.Parts {
		if part.Principal > 0 {
			return true
		}
	}
	return false
}
