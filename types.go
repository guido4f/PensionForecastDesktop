package main

import "fmt"

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
	SavingsFirst  DrawdownOrder = iota // Use ISAs first, then pension
	PensionFirst                       // Use pension first, save ISAs for last
	TaxOptimized                       // Optimize withdrawal mix to minimize tax
	PensionToISA                       // Over-draw pension to fill tax bands, excess to ISA
	PensionOnly                        // Only use pension, never touch ISAs (for pension-only depletion)
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
	case PensionOnly:
		return "Pension Only"
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

// SimulationParams combines crystallisation strategy and drawdown order
type SimulationParams struct {
	CrystallisationStrategy Strategy
	DrawdownOrder           DrawdownOrder
	EarlyMortgagePayoff     bool           // Deprecated: use MortgageOpt instead
	MortgageOpt             MortgageOption // How mortgage is handled
	MaximizeCoupleISA       bool           // For PensionToISA: fill both people's ISA allowances from one pension
}

func (sp SimulationParams) String() string {
	base := sp.DrawdownOrder.String()
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
	orderShort := "ISAFirst"
	if sp.DrawdownOrder == PensionFirst {
		orderShort = "PenFirst"
	} else if sp.DrawdownOrder == TaxOptimized {
		orderShort = "TaxOpt"
	} else if sp.DrawdownOrder == PensionToISA {
		orderShort = "Combined"
	} else if sp.DrawdownOrder == PensionOnly {
		orderShort = "PenOnly"
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
	case PensionOnly:
		drawdownDesc = "Pension Only"
	default:
		drawdownDesc = "Unknown Strategy"
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

// Person represents a person's financial state during simulation
type Person struct {
	Name              string
	BirthYear         int
	RetirementAge     int
	StatePensionAge   int
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
}

// Clone creates a deep copy of a Person
func (p *Person) Clone() *Person {
	return &Person{
		Name:                     p.Name,
		BirthYear:                p.BirthYear,
		RetirementAge:            p.RetirementAge,
		StatePensionAge:          p.StatePensionAge,
		TaxFreeSavings:           p.TaxFreeSavings,
		UncrystallisedPot:        p.UncrystallisedPot,
		CrystallisedPot:          p.CrystallisedPot,
		PCLSTaken:                p.PCLSTaken,
		ISAAnnualLimit:           p.ISAAnnualLimit,
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

// CanAccessPension returns true if the person can access their pension
func (p *Person) CanAccessPension(year int) bool {
	// Guard against invalid birth year (would cause incorrect age calculation)
	if p.BirthYear < 1900 || p.BirthYear > year {
		return false
	}
	age := year - p.BirthYear
	return age >= p.RetirementAge
}

// EffectiveStatePensionAge returns the age at which state pension starts (after any deferral)
func (p *Person) EffectiveStatePensionAge() int {
	return p.StatePensionAge + p.StatePensionDeferYears
}

// ReceivesStatePension returns true if the person receives state pension
// Accounts for any deferral period
func (p *Person) ReceivesStatePension(year int) bool {
	// Guard against invalid birth year
	if p.BirthYear < 1900 || p.BirthYear > year {
		return false
	}
	age := year - p.BirthYear
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

// ReceivesDBPension returns true if the person receives their DB pension
func (p *Person) ReceivesDBPension(year int) bool {
	if p.DBPensionAmount <= 0 || p.DBPensionStartAge <= 0 {
		return false
	}
	// Guard against invalid birth year
	if p.BirthYear < 1900 || p.BirthYear > year {
		return false
	}
	age := year - p.BirthYear
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

// IsReceivingPartTimeIncome returns true if person is earning part-time income
func (p *Person) IsReceivingPartTimeIncome(year int) bool {
	if p.PartTimeIncome <= 0 {
		return false
	}
	// Guard against invalid birth year
	if p.BirthYear < 1900 || p.BirthYear > year {
		return false
	}
	age := year - p.BirthYear
	return age >= p.PartTimeStartAge && age < p.PartTimeEndAge
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

// YearState holds the complete state for a simulation year
type YearState struct {
	Year                 int
	Ages                 map[string]int
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
	// VPW tracking
	VPWRate              float64 // VPW percentage for reference person's age
	VPWSuggestedIncome   float64 // VPW-calculated income (portfolio * rate)
	// Tax band tracking
	PersonalAllowance    float64 // Inflated personal allowance for this year
	BasicRateLimit       float64 // Inflated basic rate limit for this year
	// Growth rate tracking (for gradual decline feature)
	PensionGrowthRateUsed float64 // Actual pension growth rate used this year
	SavingsGrowthRateUsed float64 // Actual ISA growth rate used this year
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

// NewYearState creates a new initialized YearState
func NewYearState(year int) YearState {
	return YearState{
		Year:                 year,
		Ages:                 make(map[string]int),
		StatePensionByPerson: make(map[string]float64),
		DBPensionByPerson:    make(map[string]float64),
		Withdrawals:          NewWithdrawalBreakdown(),
		TaxByPerson:          make(map[string]float64),
		EndBalances:          make(map[string]PersonBalances),
	}
}
