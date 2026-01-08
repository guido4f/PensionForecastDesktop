package main

import (
	"fmt"
	"time"
)

// TaxYearLabel formats a year as UK tax year format (e.g., 2024 -> "2024/25")
// The year parameter represents the calendar year in which the tax year starts (April 6)
func TaxYearLabel(year int) string {
	nextYearShort := (year + 1) % 100
	return fmt.Sprintf("%d/%02d", year, nextYearShort)
}

// TaxYearLabelShort formats a year as short UK tax year format (e.g., 2024 -> "24/25")
func TaxYearLabelShort(year int) string {
	yearShort := year % 100
	nextYearShort := (year + 1) % 100
	return fmt.Sprintf("%02d/%02d", yearShort, nextYearShort)
}

// GetAgeInTaxYear calculates the age a person reaches during a given tax year
// The tax year runs from April 6 to April 5 of the following year
// birthDate should be in YYYY-MM-DD format
// taxYearStart is the calendar year when the tax year begins (e.g., 2026 for tax year 2026/27)
//
// Example:
// - Person born July 15, 1971, taxYearStart 2026: Returns 55 (turns 55 on July 15, 2026 during tax year 2026/27)
// - Person born Feb 10, 1971, taxYearStart 2026: Returns 56 (turns 56 on Feb 10, 2027 during tax year 2026/27)
func GetAgeInTaxYear(birthDate string, taxYearStart int) int {
	t, err := time.Parse("2006-01-02", birthDate)
	if err != nil {
		// Fallback to simple year subtraction if date parsing fails
		return taxYearStart - GetBirthYear(birthDate)
	}

	birthYear := t.Year()
	birthMonth := int(t.Month())
	birthDay := t.Day()

	// Tax year runs April 6 to April 5
	// We want to return the maximum age reached during the tax year
	//
	// If birthday is April 6 or later (April 6 - Dec 31):
	//   They turn (taxYearStart - birthYear) sometime during April-December of taxYearStart
	//
	// If birthday is before April 6 (Jan 1 - April 5):
	//   They turn (taxYearStart + 1 - birthYear) sometime during Jan-April 5 of taxYearStart+1

	// Check if birthday is in the first part of tax year (Apr 6 - Dec 31)
	if birthMonth > 4 || (birthMonth == 4 && birthDay >= 6) {
		// Birthday is April 6 or later - they turn this age during the first calendar year of the tax year
		return taxYearStart - birthYear
	}

	// Birthday is Jan 1 - April 5 - they turn this age during the second calendar year of the tax year
	return (taxYearStart + 1) - birthYear
}

// GetAgeAtTaxYearStart calculates the age at the start of a tax year (April 6)
// This is useful for determining if someone can already access their pension at the start of the year
func GetAgeAtTaxYearStart(birthDate string, taxYearStart int) int {
	t, err := time.Parse("2006-01-02", birthDate)
	if err != nil {
		return taxYearStart - GetBirthYear(birthDate)
	}

	birthYear := t.Year()
	birthMonth := int(t.Month())
	birthDay := t.Day()

	// On April 6 of taxYearStart:
	// - If birthday is April 6 or earlier, they've already had their birthday this calendar year
	// - If birthday is after April 6, they haven't had their birthday yet

	ageAtTaxYearStart := taxYearStart - birthYear
	if birthMonth > 4 || (birthMonth == 4 && birthDay > 6) {
		// Birthday is after April 6, so they haven't had their birthday yet this calendar year
		ageAtTaxYearStart--
	}

	return ageAtTaxYearStart
}

// GetTaxYearForAge returns the tax year (start year) when a person reaches a given age
// Returns the tax year during which they first reach that age
func GetTaxYearForAge(birthDate string, targetAge int) int {
	t, err := time.Parse("2006-01-02", birthDate)
	if err != nil {
		return GetBirthYear(birthDate) + targetAge
	}

	birthYear := t.Year()
	birthMonth := int(t.Month())
	birthDay := t.Day()

	// Calendar year when they turn targetAge
	calendarYearOfBirthday := birthYear + targetAge

	// If birthday is before April 6, the tax year started in the previous calendar year
	if birthMonth < 4 || (birthMonth == 4 && birthDay <= 5) {
		return calendarYearOfBirthday - 1
	}

	// Birthday is April 6 or later, tax year starts in the same calendar year as their birthday
	return calendarYearOfBirthday
}

// OptimizationGoal represents what to optimize for when selecting the best strategy
type OptimizationGoal int

const (
	OptimizeTax     OptimizationGoal = iota // Minimize total tax paid
	OptimizeIncome                          // Maximize total net income over period
	OptimizeBalance                         // Maximize final balance
)

func (o OptimizationGoal) String() string {
	switch o {
	case OptimizeTax:
		return "Tax Efficiency"
	case OptimizeIncome:
		return "Total Income"
	case OptimizeBalance:
		return "Final Balance"
	default:
		return "Unknown"
	}
}

// Strategy represents a crystallisation strategy
type Strategy int

const (
	GradualCrystallisation Strategy = iota
	UFPLSStrategy                   // Uncrystallised Funds Pension Lump Sum - each withdrawal is 25% tax-free
)

func (s Strategy) String() string {
	switch s {
	case GradualCrystallisation:
		return "Gradual Crystallisation"
	case UFPLSStrategy:
		return "UFPLS (Flexible Lump Sums)"
	default:
		return "Unknown"
	}
}

// DrawdownOrder represents which assets to use first
type DrawdownOrder int

const (
	SavingsFirst          DrawdownOrder = iota // Use ISAs first, then pension
	PensionFirst                               // Use pension first, save ISAs for last
	TaxOptimized                               // Optimize withdrawal mix to minimize tax
	PensionToISA                               // Over-draw pension to fill tax bands, excess to ISA (only when income needed)
	PensionToISAProactive                      // Extract pension to ISA even when work income covers expenses
	PensionOnly                                // Only use pension, never touch ISAs (for pension-only depletion)
	FillBasicRate                              // Withdraw from pension up to basic rate limit, excess to ISA
	StatePensionBridge                         // Draw heavily before state pension, reduce after
)

func (d DrawdownOrder) String() string {
	switch d {
	case SavingsFirst:
		return "Savings First"
	case PensionFirst:
		return "Pension First"
	case TaxOptimized:
		return "Tax Optimized"
	case PensionToISA:
		return "Pension to ISA"
	case PensionToISAProactive:
		return "Pension to ISA (Proactive)"
	case PensionOnly:
		return "Pension Only"
	case FillBasicRate:
		return "Fill Basic Rate"
	case StatePensionBridge:
		return "State Pension Bridge"
	default:
		return "Unknown"
	}
}

// MortgageOption represents how the mortgage is paid off
type MortgageOption int

const (
	MortgageEarly      MortgageOption = iota // Pay off at early payoff year
	MortgageNormal                           // Pay off at normal end year
	MortgageExtended                         // Extend by 10 years beyond normal
	PCLSMortgagePayoff                       // Use 25% PCLS lump sum to pay mortgage (no further 25% tax-free)
)

// PermutationMode controls how many strategy combinations to generate
type PermutationMode int

const (
	ModeQuick         PermutationMode = iota // ~8 combinations - core strategies only
	ModeStandard                              // ~100 combinations - common variations
	ModeThorough                              // ~250 combinations - thorough analysis
	ModeComprehensive                         // ~4000 combinations - all valid combinations
)

func (m PermutationMode) String() string {
	switch m {
	case ModeQuick:
		return "Quick"
	case ModeStandard:
		return "Standard"
	case ModeThorough:
		return "Thorough"
	case ModeComprehensive:
		return "Comprehensive"
	default:
		return "Unknown"
	}
}

// FactorID uniquely identifies a strategy factor type
type FactorID string

const (
	FactorCrystallisation   FactorID = "crystallisation"
	FactorDrawdown          FactorID = "drawdown"
	FactorMortgage          FactorID = "mortgage"
	FactorMaximizeCoupleISA FactorID = "maximize_couple_isa"
	FactorISAToSIPP         FactorID = "isa_to_sipp"
	FactorGuardrails        FactorID = "guardrails"
	FactorStatePensionDefer FactorID = "state_pension_defer"
)

// FactorValue represents one possible value for a factor
type FactorValue struct {
	ID        string      // Unique identifier (e.g., "gradual", "ufpls")
	Name      string      // Human-readable name
	ShortName string      // For compact display
	Value     interface{} // The actual value (type depends on factor)
}

// Factor represents a dimension in the strategy space
type Factor struct {
	ID             FactorID       // Unique identifier
	Name           string         // Human-readable name
	Description    string         // Explanation of the factor
	Values         []FactorValue  // Available values for this factor
	DefaultValueID string         // ID of the default value
	DependsOn      []FactorID     // Other factors this depends on
	// ApplicableFunc is set by the registry at runtime
}

// StrategyCombo represents a complete combination of factor values
type StrategyCombo struct {
	Values map[FactorID]FactorValue
}

// Clone creates a copy of the StrategyCombo
func (sc StrategyCombo) Clone() StrategyCombo {
	clone := StrategyCombo{Values: make(map[FactorID]FactorValue)}
	for k, v := range sc.Values {
		clone.Values[k] = v
	}
	return clone
}

// SimulationParams combines crystallisation strategy and drawdown order
type SimulationParams struct {
	// Existing core factors
	CrystallisationStrategy Strategy
	DrawdownOrder           DrawdownOrder
	EarlyMortgagePayoff     bool           // Deprecated: use MortgageOpt instead
	MortgageOpt             MortgageOption // How mortgage is handled
	MaximizeCoupleISA       bool           // For PensionToISA: fill both people's ISA allowances from one pension
	ISAToSIPPEnabled        bool           // Enable ISA to SIPP transfers while working (pre-retirement optimization)

	// NEW: Dynamic adjustment strategies
	GuardrailsEnabled bool // Enable Guyton-Klinger guardrails

	// NEW: State pension deferral (applies to all people)
	StatePensionDeferYears int // Years to defer state pension (0, 2, or 5)

	// Metadata for tracking and filtering
	SourceCombo *StrategyCombo // Original combo this was generated from
}

func (sp SimulationParams) String() string {
	base := sp.DrawdownOrder.String()
	if sp.ISAToSIPPEnabled {
		base = "ISA→SIPP " + base
	}
	if sp.GuardrailsEnabled {
		base = base + " +Guardrails"
	}
	if sp.StatePensionDeferYears > 0 {
		base = base + fmt.Sprintf(" +Defer%dy", sp.StatePensionDeferYears)
	}
	switch sp.MortgageOpt {
	case MortgageEarly:
		return base + " (Early Payoff)"
	case MortgageExtended:
		return base + " (Extended +10y)"
	case PCLSMortgagePayoff:
		return base + " (PCLS Payoff)"
	default:
		return base + " (Normal Payoff)"
	}
}

func (sp SimulationParams) ShortName() string {
	var orderShort string
	switch sp.DrawdownOrder {
	case SavingsFirst:
		orderShort = "ISAFirst"
	case PensionFirst:
		orderShort = "PenFirst"
	case TaxOptimized:
		orderShort = "TaxOpt"
	case PensionToISA:
		orderShort = "Combined"
	case PensionToISAProactive:
		orderShort = "Combined+"
	case PensionOnly:
		orderShort = "PenOnly"
	case FillBasicRate:
		orderShort = "FillBasic"
	case StatePensionBridge:
		orderShort = "SPBridge"
	default:
		orderShort = "Unknown"
	}

	// Prefix with U/ for UFPLS
	if sp.CrystallisationStrategy == UFPLSStrategy {
		orderShort = "U/" + orderShort
	}

	// Prefix with I2S/ for ISA to SIPP
	if sp.ISAToSIPPEnabled {
		orderShort = "I2S/" + orderShort
	}

	// Add new factor suffixes
	if sp.GuardrailsEnabled {
		orderShort = orderShort + "/GR"
	}
	if sp.StatePensionDeferYears > 0 {
		orderShort = orderShort + fmt.Sprintf("/D%d", sp.StatePensionDeferYears)
	}

	switch sp.MortgageOpt {
	case MortgageEarly:
		return orderShort + "/Early"
	case MortgageExtended:
		return orderShort + "/Ext+10"
	case PCLSMortgagePayoff:
		return orderShort + "/PCLS"
	default:
		return orderShort + "/Normal"
	}
}

// DescriptiveName returns a human-readable description of the strategy
func (sp SimulationParams) DescriptiveName(mortgagePayoffYear int) string {
	var drawdownDesc string
	switch sp.DrawdownOrder {
	case SavingsFirst:
		drawdownDesc = "ISA First, Then Pension"
	case PensionFirst:
		drawdownDesc = "Pension First, Then ISA"
	case TaxOptimized:
		drawdownDesc = "Tax Optimized Withdrawals"
	case PensionToISA:
		drawdownDesc = "Combined ISA And Pension"
	case PensionToISAProactive:
		drawdownDesc = "Combined ISA And Pension (Proactive)"
	case PensionOnly:
		drawdownDesc = "Pension Only"
	case FillBasicRate:
		drawdownDesc = "Fill Basic Rate Band"
	case StatePensionBridge:
		drawdownDesc = "State Pension Bridge"
	default:
		drawdownDesc = "Unknown Strategy"
	}

	// Add crystallisation type if UFPLS
	if sp.CrystallisationStrategy == UFPLSStrategy {
		drawdownDesc = "UFPLS " + drawdownDesc
	}

	// Add ISA to SIPP pre-retirement strategy
	if sp.ISAToSIPPEnabled {
		drawdownDesc = "ISA→SIPP " + drawdownDesc
	}

	// Add new factor descriptions
	var extras []string
	if sp.GuardrailsEnabled {
		extras = append(extras, "Guardrails")
	}
	if sp.StatePensionDeferYears > 0 {
		extras = append(extras, fmt.Sprintf("SP Defer %dy", sp.StatePensionDeferYears))
	}
	if len(extras) > 0 {
		drawdownDesc = drawdownDesc + " (" + joinStrings(extras, ", ") + ")"
	}

	// Only add mortgage description if there is a mortgage (year > 0)
	if mortgagePayoffYear <= 0 {
		return drawdownDesc
	}

	var mortgageDesc string
	switch sp.MortgageOpt {
	case MortgageEarly:
		mortgageDesc = fmt.Sprintf("Mortgage repaid %d", mortgagePayoffYear)
	case MortgageNormal:
		mortgageDesc = fmt.Sprintf("Mortgage repaid %d", mortgagePayoffYear)
	case MortgageExtended:
		mortgageDesc = fmt.Sprintf("Mortgage extended to %d", mortgagePayoffYear)
	case PCLSMortgagePayoff:
		mortgageDesc = fmt.Sprintf("PCLS lump sum for mortgage %d", mortgagePayoffYear)
	default:
		return drawdownDesc
	}

	return drawdownDesc + ", " + mortgageDesc
}

// joinStrings joins strings with a separator (helper for DescriptiveName)
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// Person represents a person's financial state during simulation
type Person struct {
	Name               string
	BirthYear          int    // Deprecated: Use BirthDate for tax year calculations
	BirthDate          string // Full birth date in YYYY-MM-DD format for tax year age calculations
	RetirementDate     string // Full retirement date in YYYY-MM-DD format (if specified)
	RetirementAge      int    // Age when income requirements start (stop working)
	RetirementTaxYear  int    // Tax year when retirement begins (calculated from RetirementDate or RetirementAge)
	PensionAccessAge   int    // Age when DC pension can be accessed (may be later than RetirementAge)
	StatePensionAge    int
	TaxFreeSavings    float64 // ISA (includes crystallised tax-free lump sums)
	UncrystallisedPot float64 // Pension not yet accessed
	CrystallisedPot   float64 // Taxable pension pot
	PCLSTaken         bool    // True if 25% PCLS lump sum was taken (no further 25% tax-free)
	ISAAnnualLimit    float64 // Per-person ISA annual contribution limit

	// DB Pension Configuration
	DBPensionAmount        float64 // Annual DB pension at normal retirement age
	DBPensionStartAge      int     // Age when DB pension starts
	DBPensionName          string  // Name of DB pension scheme
	DBPensionNormalAge     int     // Normal retirement age for the scheme
	DBPensionEarlyFactor   float64 // Reduction per year early (e.g., 0.04 = 4%)
	DBPensionLateFactor    float64 // Increase per year late (e.g., 0.05 = 5%)
	DBPensionCommutation   float64 // Fraction to commute (0-0.25)
	DBPensionCommuteFactor float64 // Commutation factor (e.g., 12 = £12 per £1 pension)
	DBPensionLumpSum       float64 // Lump sum received from commutation (added to ISA on first DB pension year)
	DBPensionLumpSumTaken  bool    // Whether lump sum has been taken

	// State Pension Deferral
	StatePensionDeferYears   int     // Years to defer state pension (0 = no deferral)
	StatePensionDeferralRate float64 // Enhancement per year deferred (e.g., 0.058 = 5.8%)

	// Emergency Fund
	EmergencyFundMinimum float64 // Minimum ISA balance to preserve (calculated from months × expenses)

	// Phased Retirement
	PartTimeIncome    float64 // Annual income from part-time work
	PartTimeStartAge  int     // Age when part-time work starts
	PartTimeEndAge    int     // Age when part-time work ends

	// Pre-retirement work income
	WorkIncome float64 // Annual salary while still employed (before RetirementDate)

	// ISA to SIPP Transfer Configuration
	ISAToSIPPEnabled        bool    // Enable ISA to SIPP transfers while working
	PensionAnnualAllowance  float64 // Annual pension contribution limit (default £60,000)
	EmployerContribution    float64 // Annual employer pension contribution (reduces available allowance)
	ISAToSIPPMaxPercent     float64 // Max % of remaining allowance to use (default 100%)
	ISAToSIPPPreserveMonths int     // Months of expenses to preserve in ISA
}

// Clone creates a deep copy of a Person
func (p *Person) Clone() *Person {
	return &Person{
		Name:              p.Name,
		BirthYear:         p.BirthYear,
		BirthDate:         p.BirthDate,
		RetirementDate:    p.RetirementDate,
		RetirementAge:     p.RetirementAge,
		RetirementTaxYear: p.RetirementTaxYear,
		PensionAccessAge:  p.PensionAccessAge,
		StatePensionAge:   p.StatePensionAge,
		TaxFreeSavings:    p.TaxFreeSavings,
		UncrystallisedPot: p.UncrystallisedPot,
		CrystallisedPot:   p.CrystallisedPot,
		PCLSTaken:         p.PCLSTaken,
		ISAAnnualLimit:    p.ISAAnnualLimit,
		// DB Pension
		DBPensionAmount:        p.DBPensionAmount,
		DBPensionStartAge:      p.DBPensionStartAge,
		DBPensionName:          p.DBPensionName,
		DBPensionNormalAge:     p.DBPensionNormalAge,
		DBPensionEarlyFactor:   p.DBPensionEarlyFactor,
		DBPensionLateFactor:    p.DBPensionLateFactor,
		DBPensionCommutation:   p.DBPensionCommutation,
		DBPensionCommuteFactor: p.DBPensionCommuteFactor,
		DBPensionLumpSum:       p.DBPensionLumpSum,
		DBPensionLumpSumTaken:  p.DBPensionLumpSumTaken,
		// State Pension Deferral
		StatePensionDeferYears:   p.StatePensionDeferYears,
		StatePensionDeferralRate: p.StatePensionDeferralRate,
		// Emergency Fund
		EmergencyFundMinimum: p.EmergencyFundMinimum,
		// Phased Retirement
		PartTimeIncome:   p.PartTimeIncome,
		PartTimeStartAge: p.PartTimeStartAge,
		PartTimeEndAge:   p.PartTimeEndAge,
		// Pre-retirement work income
		WorkIncome: p.WorkIncome,
		// ISA to SIPP Transfer
		ISAToSIPPEnabled:        p.ISAToSIPPEnabled,
		PensionAnnualAllowance:  p.PensionAnnualAllowance,
		EmployerContribution:    p.EmployerContribution,
		ISAToSIPPMaxPercent:     p.ISAToSIPPMaxPercent,
		ISAToSIPPPreserveMonths: p.ISAToSIPPPreserveMonths,
	}
}

// AvailableISA returns the ISA balance available for withdrawal after preserving emergency fund
func (p *Person) AvailableISA() float64 {
	available := p.TaxFreeSavings - p.EmergencyFundMinimum
	if available < 0 {
		return 0
	}
	return available
}

// TotalPension returns the total pension value (crystallised + uncrystallised)
func (p *Person) TotalPension() float64 {
	return p.CrystallisedPot + p.UncrystallisedPot
}

// TotalWealth returns total assets
func (p *Person) TotalWealth() float64 {
	return p.TaxFreeSavings + p.TotalPension()
}

// CanAccessPension returns true if the person can access their DC pension during this tax year
// Uses PensionAccessAge (minimum pension age) not RetirementAge (when income needs start)
// year is the tax year start (e.g., 2026 for tax year 2026/27)
func (p *Person) CanAccessPension(year int) bool {
	// Guard against invalid birth year (would cause incorrect age calculation)
	if p.BirthYear < 1900 || p.BirthYear > year {
		return false
	}
	// Use tax year age calculation if BirthDate is available
	var age int
	if p.BirthDate != "" {
		age = GetAgeInTaxYear(p.BirthDate, year)
	} else {
		age = year - p.BirthYear
	}
	// Use PensionAccessAge if set, otherwise fall back to RetirementAge for backwards compatibility
	accessAge := p.PensionAccessAge
	if accessAge <= 0 {
		accessAge = p.RetirementAge
	}
	return age >= accessAge
}

// EffectiveStatePensionAge returns the age at which state pension starts (after any deferral)
func (p *Person) EffectiveStatePensionAge() int {
	return p.StatePensionAge + p.StatePensionDeferYears
}

// ReceivesStatePension returns true if the person receives state pension during this tax year
// Accounts for any deferral period
// year is the tax year start (e.g., 2026 for tax year 2026/27)
func (p *Person) ReceivesStatePension(year int) bool {
	// Guard against invalid birth year
	if p.BirthYear < 1900 || p.BirthYear > year {
		return false
	}
	// Use tax year age calculation if BirthDate is available
	var age int
	if p.BirthDate != "" {
		age = GetAgeInTaxYear(p.BirthDate, year)
	} else {
		age = year - p.BirthYear
	}
	return age >= p.EffectiveStatePensionAge()
}

// GetDeferredStatePensionAmount calculates the enhanced state pension after deferral
// baseAmount is the standard state pension amount
// The enhancement is compounded: baseAmount * (1 + rate)^deferralYears
func (p *Person) GetDeferredStatePensionAmount(baseAmount float64) float64 {
	if p.StatePensionDeferYears <= 0 || p.StatePensionDeferralRate <= 0 {
		return baseAmount
	}
	// Each year of deferral increases the pension by the deferral rate
	enhancement := 1.0
	for i := 0; i < p.StatePensionDeferYears; i++ {
		enhancement *= (1 + p.StatePensionDeferralRate)
	}
	return baseAmount * enhancement
}

// ReceivesDBPension returns true if the person receives their DB pension during this tax year
// year is the tax year start (e.g., 2026 for tax year 2026/27)
func (p *Person) ReceivesDBPension(year int) bool {
	if p.DBPensionAmount <= 0 || p.DBPensionStartAge <= 0 {
		return false
	}
	// Guard against invalid birth year
	if p.BirthYear < 1900 || p.BirthYear > year {
		return false
	}
	// Use tax year age calculation if BirthDate is available
	var age int
	if p.BirthDate != "" {
		age = GetAgeInTaxYear(p.BirthDate, year)
	} else {
		age = year - p.BirthYear
	}
	return age >= p.DBPensionStartAge
}

// GetEffectiveDBPension calculates the DB pension amount after applying:
// 1. Early/late retirement adjustments
// 2. Commutation (taking some as lump sum)
// Returns the annual pension amount
func (p *Person) GetEffectiveDBPension() float64 {
	if p.DBPensionAmount <= 0 {
		return 0
	}

	effectiveAmount := p.DBPensionAmount

	// Apply early/late retirement adjustment
	if p.DBPensionNormalAge > 0 && p.DBPensionStartAge > 0 {
		yearsDifference := p.DBPensionStartAge - p.DBPensionNormalAge
		if yearsDifference < 0 && p.DBPensionEarlyFactor > 0 {
			// Early retirement - reduce pension
			reduction := float64(-yearsDifference) * p.DBPensionEarlyFactor
			effectiveAmount = effectiveAmount * (1 - reduction)
		} else if yearsDifference > 0 && p.DBPensionLateFactor > 0 {
			// Late retirement - increase pension
			increase := float64(yearsDifference) * p.DBPensionLateFactor
			effectiveAmount = effectiveAmount * (1 + increase)
		}
	}

	// Apply commutation - reduce pension by commuted portion
	if p.DBPensionCommutation > 0 {
		effectiveAmount = effectiveAmount * (1 - p.DBPensionCommutation)
	}

	return effectiveAmount
}

// GetDBPensionLumpSum calculates the lump sum from commutation
// This is typically taken at the start of DB pension
func (p *Person) GetDBPensionLumpSum() float64 {
	if p.DBPensionAmount <= 0 || p.DBPensionCommutation <= 0 {
		return 0
	}

	// Calculate the annual pension being given up
	effectiveAmount := p.DBPensionAmount

	// Apply early/late adjustment first
	if p.DBPensionNormalAge > 0 && p.DBPensionStartAge > 0 {
		yearsDifference := p.DBPensionStartAge - p.DBPensionNormalAge
		if yearsDifference < 0 && p.DBPensionEarlyFactor > 0 {
			reduction := float64(-yearsDifference) * p.DBPensionEarlyFactor
			effectiveAmount = effectiveAmount * (1 - reduction)
		} else if yearsDifference > 0 && p.DBPensionLateFactor > 0 {
			increase := float64(yearsDifference) * p.DBPensionLateFactor
			effectiveAmount = effectiveAmount * (1 + increase)
		}
	}

	// Pension given up = base * commutation fraction
	pensionGivenUp := effectiveAmount * p.DBPensionCommutation

	// Commutation factor (default 12 if not set)
	factor := p.DBPensionCommuteFactor
	if factor <= 0 {
		factor = 12.0 // Default: £12 lump sum per £1 annual pension
	}

	return pensionGivenUp * factor
}

// IsReceivingPartTimeIncome returns true if person is earning part-time income during this tax year
// year is the tax year start (e.g., 2026 for tax year 2026/27)
func (p *Person) IsReceivingPartTimeIncome(year int) bool {
	if p.PartTimeIncome <= 0 {
		return false
	}
	// Guard against invalid birth year
	if p.BirthYear < 1900 || p.BirthYear > year {
		return false
	}
	// Use tax year age calculation if BirthDate is available
	var age int
	if p.BirthDate != "" {
		age = GetAgeInTaxYear(p.BirthDate, year)
	} else {
		age = year - p.BirthYear
	}
	return age >= p.PartTimeStartAge && age < p.PartTimeEndAge
}

// IsWorking returns true if the person is still employed (before retirement date/age)
// year is the tax year start (e.g., 2026 for tax year 2026/27)
func (p *Person) IsWorking(year int) bool {
	if p.WorkIncome <= 0 {
		return false
	}
	// Use RetirementTaxYear if set (calculated from RetirementDate or RetirementAge)
	if p.RetirementTaxYear > 0 {
		return year < p.RetirementTaxYear
	}
	// Fallback to age-based calculation
	var age int
	if p.BirthDate != "" {
		age = GetAgeInTaxYear(p.BirthDate, year)
	} else {
		age = year - p.BirthYear
	}
	return age < p.RetirementAge
}

// PersonBalances holds end-of-year balances for a person
type PersonBalances struct {
	TaxFreeSavings    float64
	UncrystallisedPot float64
	CrystallisedPot   float64
}

// WithdrawalBreakdown shows where money came from and where it went
type WithdrawalBreakdown struct {
	TaxFreeFromISA     map[string]float64 // Per person
	TaxFreeFromPension map[string]float64 // 25% crystallisation per person
	TaxableFromPension map[string]float64 // Per person
	TotalTaxFree       float64
	TotalTaxable       float64
	ISADeposits        map[string]float64 // Per person - excess deposited to ISA
	TotalISADeposits   float64
}

// YearState holds the complete state for a simulation tax year
type YearState struct {
	Year                 int    // Tax year start (e.g., 2026 for tax year 2026/27)
	TaxYearLabel         string // Formatted tax year (e.g., "2026/27")
	Ages                 map[string]int
	StartBalance         float64 // Total balance at start of year (before withdrawals/growth)
	RequiredIncome       float64
	MortgageCost         float64
	TotalRequired        float64
	StatePensionByPerson map[string]float64
	TotalStatePension    float64
	DBPensionByPerson    map[string]float64 // DB pension per person (e.g., Teachers Pension)
	TotalDBPension       float64
	NetRequired          float64 // After state pension and DB pension - this is the after-tax income needed
	NetIncomeRequired    float64 // Income portion of NetRequired (living expenses not covered by pensions)
	NetMortgageRequired  float64 // Mortgage portion of NetRequired (mortgage payments not covered by excess pension income)
	Withdrawals          WithdrawalBreakdown
	TaxByPerson          map[string]float64
	TotalTaxPaid         float64
	NetIncomeReceived    float64 // Actual spendable income (withdrawals - tax + state pension + DB pension)
	EndBalances          map[string]PersonBalances
	TotalBalance         float64
	// Guardrails tracking
	GuardrailsTriggered   int     // -1 = reduced, 0 = no change, 1 = increased
	GuardrailsAdjusted    float64 // The adjusted income amount (if guardrails enabled)
	PartTimeIncome        float64 // Income from part-time work (phased retirement)
	// Pre-retirement work income
	WorkIncomeByPerson    map[string]float64 // Work income per person (before retirement)
	TotalWorkIncome       float64            // Combined work income from both people
	ISAContributions      map[string]float64 // Surplus work income added to ISA per person
	TotalISAContributions float64            // Total surplus added to ISA
	// Tax band tracking
	PersonalAllowance    float64 // Inflated personal allowance for this year
	BasicRateLimit       float64 // Inflated basic rate limit for this year
	// Growth rate tracking (for gradual decline feature)
	PensionGrowthRateUsed float64 // Actual pension growth rate used this year
	SavingsGrowthRateUsed float64 // Actual ISA growth rate used this year
	// ISA to SIPP transfers (pre-retirement optimization)
	ISAToSIPPByPerson     map[string]float64 // Net amount transferred from ISA per person
	ISAToSIPPTaxRelief    map[string]float64 // Tax relief received per person
	TotalISAToSIPP        float64            // Total net transferred from ISA
	TotalISAToSIPPRelief  float64            // Total tax relief received
}

// SimulationResult holds the complete results of a simulation run
type SimulationResult struct {
	Params         SimulationParams
	Years          []YearState
	TotalTaxPaid   float64
	TotalWithdrawn float64
	RanOutOfMoney  bool
	RanOutYear     int
	FinalBalances  map[string]PersonBalances
}

// NewWithdrawalBreakdown creates a new initialized WithdrawalBreakdown
func NewWithdrawalBreakdown() WithdrawalBreakdown {
	return WithdrawalBreakdown{
		TaxFreeFromISA:     make(map[string]float64),
		TaxFreeFromPension: make(map[string]float64),
		TaxableFromPension: make(map[string]float64),
		ISADeposits:        make(map[string]float64),
	}
}

// NewYearState creates a new initialized YearState for a tax year
// year is the tax year start (e.g., 2026 for tax year 2026/27)
func NewYearState(year int) YearState {
	return YearState{
		Year:                 year,
		TaxYearLabel:         TaxYearLabel(year),
		Ages:                 make(map[string]int),
		StatePensionByPerson: make(map[string]float64),
		DBPensionByPerson:    make(map[string]float64),
		Withdrawals:          NewWithdrawalBreakdown(),
		TaxByPerson:          make(map[string]float64),
		EndBalances:          make(map[string]PersonBalances),
		WorkIncomeByPerson:   make(map[string]float64),
		ISAContributions:     make(map[string]float64),
		ISAToSIPPByPerson:    make(map[string]float64),
		ISAToSIPPTaxRelief:   make(map[string]float64),
	}
}
