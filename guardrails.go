package main

// GuardrailsState tracks the state needed for Guyton-Klinger guardrails strategy
type GuardrailsState struct {
	InitialWithdrawalRate float64 // The withdrawal rate in year 1 (withdrawal / portfolio)
	InitialPortfolioValue float64 // Portfolio value at start of retirement
	CurrentWithdrawal     float64 // Current year's withdrawal amount (adjusted)
	UpperLimit            float64 // Upper guardrail (e.g., 1.20 = 120%)
	LowerLimit            float64 // Lower guardrail (e.g., 0.80 = 80%)
	AdjustmentRate        float64 // How much to adjust (e.g., 0.10 = 10%)
}

// NewGuardrailsState creates a new guardrails state with default values
func NewGuardrailsState(config *Config) *GuardrailsState {
	upperLimit := config.IncomeRequirements.GuardrailsUpperLimit
	if upperLimit <= 0 {
		upperLimit = 1.20 // Default: 120% of initial rate
	}
	lowerLimit := config.IncomeRequirements.GuardrailsLowerLimit
	if lowerLimit <= 0 {
		lowerLimit = 0.80 // Default: 80% of initial rate
	}
	adjustmentRate := config.IncomeRequirements.GuardrailsAdjustment
	if adjustmentRate <= 0 {
		adjustmentRate = 0.10 // Default: 10% adjustment
	}

	return &GuardrailsState{
		UpperLimit:     upperLimit,
		LowerLimit:     lowerLimit,
		AdjustmentRate: adjustmentRate,
	}
}

// Initialize sets up the initial state based on first year values
func (g *GuardrailsState) Initialize(portfolioValue, withdrawal float64) {
	g.InitialPortfolioValue = portfolioValue
	g.CurrentWithdrawal = withdrawal
	if portfolioValue > 0 {
		g.InitialWithdrawalRate = withdrawal / portfolioValue
	}
}

// CalculateAdjustedWithdrawal applies guardrails logic to determine the withdrawal for the current year
// portfolioValue: current total portfolio value
// baseWithdrawal: what the withdrawal would be without guardrails (inflation-adjusted from initial)
// Returns: adjusted withdrawal amount
func (g *GuardrailsState) CalculateAdjustedWithdrawal(portfolioValue, baseWithdrawal float64) float64 {
	// If not initialized, just return base withdrawal
	if g.InitialWithdrawalRate <= 0 || portfolioValue <= 0 {
		return baseWithdrawal
	}

	// Start with the current adjusted withdrawal (not base)
	// Apply inflation to current withdrawal
	currentWithdrawal := g.CurrentWithdrawal
	if currentWithdrawal <= 0 {
		currentWithdrawal = baseWithdrawal
	}

	// Calculate current withdrawal rate
	currentRate := currentWithdrawal / portfolioValue

	// Calculate ratio of current rate to initial rate
	rateRatio := currentRate / g.InitialWithdrawalRate

	// Apply guardrails
	if rateRatio > g.UpperLimit {
		// Portfolio has fallen significantly - reduce withdrawal by adjustment rate
		// Only reduce if we're not already at a very low level
		currentWithdrawal = currentWithdrawal * (1 - g.AdjustmentRate)
	} else if rateRatio < g.LowerLimit {
		// Portfolio has grown significantly - can increase withdrawal
		currentWithdrawal = currentWithdrawal * (1 + g.AdjustmentRate)
	}

	// Update state
	g.CurrentWithdrawal = currentWithdrawal

	return currentWithdrawal
}

// GetCurrentRate returns the current withdrawal rate
func (g *GuardrailsState) GetCurrentRate(portfolioValue float64) float64 {
	if portfolioValue <= 0 {
		return 0
	}
	return g.CurrentWithdrawal / portfolioValue
}

// IsTriggered returns whether guardrails were triggered and in which direction
// Returns: -1 if reduced, 0 if no change, 1 if increased
func (g *GuardrailsState) IsTriggered(portfolioValue float64) int {
	if g.InitialWithdrawalRate <= 0 || portfolioValue <= 0 {
		return 0
	}

	currentRate := g.CurrentWithdrawal / portfolioValue
	rateRatio := currentRate / g.InitialWithdrawalRate

	if rateRatio > g.UpperLimit {
		return -1 // Will reduce
	} else if rateRatio < g.LowerLimit {
		return 1 // Will increase
	}
	return 0
}
