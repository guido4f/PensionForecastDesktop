# Pension Strategy Recommendations

This document reviews the existing pension strategies in the forecast application and suggests additional strategies that may work better or complement the current implementation.

---

## Recently Implemented Strategies

### High Priority (Batch 1)

1. **UFPLS Strategy** (`UFPLSStrategy`) - Flexible lump sum withdrawals where each withdrawal is 25% tax-free and 75% taxable, without formally crystallising the pot. Preserves 25% entitlement on remaining funds.

2. **State Pension Deferral** - Model delayed state pension with configurable enhancement rate (default 5.8%/year). Configure via:
   - `state_pension_defer_years` per person
   - `state_pension_deferral_rate` in financial config

3. **Enhanced Spouse Optimisation** - Improved Tax Optimized strategy that:
   - Fills ALL spouses' personal allowances before any basic rate tax
   - Fills basic rate band proportionally across all spouses
   - Prevents pushing one spouse into higher rate while another has basic rate space

4. **Emergency Fund Preservation** - Never reduce ISA below a minimum threshold. Configure via:
   - `emergency_fund_months` - months of expenses to preserve
   - `emergency_fund_inflation_adjust` - whether to grow threshold with inflation

### Medium Priority (Batch 2)

5. **Guardrails Strategy (Guyton-Klinger)** - Dynamic withdrawal adjustments based on portfolio performance. Configure via:
   - `guardrails_enabled` - enable the strategy
   - `guardrails_upper_limit` - upper guardrail (e.g., 1.20 = 120% of initial rate triggers reduction)
   - `guardrails_lower_limit` - lower guardrail (e.g., 0.80 = 80% triggers increase)
   - `guardrails_adjustment` - adjustment percentage (e.g., 0.10 = 10%)

6. **DB Pension Options** - Full modelling of defined benefit pension decisions:
   - `db_pension_normal_age` - normal retirement age for DB pension
   - `db_pension_early_factor` - reduction per year for early retirement (e.g., 0.04 = 4%/year)
   - `db_pension_late_factor` - enhancement per year for late retirement
   - `db_pension_commutation` - percentage to take as tax-free lump sum (0-0.25)
   - `db_pension_commute_factor` - factor for lump sum calculation (e.g., 12 = £12 per £1 pension)

7. **Phased Retirement** - Part-time work income bridge. Configure per person:
   - `part_time_income` - annual income from part-time work
   - `part_time_start_age` - age when part-time work begins
   - `part_time_end_age` - age when part-time work ends

8. **VPW Strategy (Variable Percentage Withdrawal)** - Age-based withdrawal rates. Configure via:
   - `vpw_enabled` - enable VPW-based income calculation
   - `vpw_floor` - minimum annual withdrawal (optional floor)
   - `vpw_ceiling` - maximum as multiple of floor (e.g., 1.5 = 150%)

---

## Current Strategies Summary

The application now implements **6 drawdown strategies** (5 original + UFPLS):

| Strategy | Description | Best For |
|----------|-------------|----------|
| **Savings First** | ISA → Pension | Maximise pension tax-free growth |
| **Pension First** | Pension → ISA | Preserve liquid savings |
| **Tax Optimized** | Enhanced spouse optimisation with proportional withdrawals | Minimise combined household tax |
| **Pension to ISA** | Overdraw pension to fill tax bands, excess to ISA | Convert taxable wealth to tax-free |
| **Pension Only** | Never touch ISAs | Conservative, preserve emergency buffer |
| **UFPLS** | Each withdrawal 25% tax-free, 75% taxable | Flexibility, preserve 25% entitlement |

### Crystallisation
- **Gradual Crystallisation** with 25% PCLS (tax-free) and 75% taxable
- One-time PCLS lump sum option

### Mortgage Options
- Early payoff, normal term, extended +10 years, PCLS payoff

---

## Recommended Additional Strategies

### 1. UFPLS (Uncrystallised Funds Pension Lump Sum)

**What it is:** Take ad-hoc lump sums directly from uncrystallised pension where each withdrawal is 25% tax-free and 75% taxable, without formally crystallising the pot.

**Advantages over current PCLS approach:**
- More flexible - take only what you need when you need it
- Preserves remaining 25% tax-free entitlement for future
- No need to decide upfront how much to crystallise
- Simpler for irregular income needs

**Implementation suggestion:**
```go
type CrystallisationStrategy int
const (
    GradualCrystallisation Strategy = iota
    UFPLSStrategy  // NEW: Each withdrawal is 25% TFC, 75% taxable
)
```

**Best for:** Those who want flexibility and don't need regular drawdown income.

---

### 2. Variable Percentage Withdrawal (VPW)

**What it is:** Dynamic withdrawal rate that adjusts annually based on:
- Remaining portfolio value
- Life expectancy (using actuarial tables)
- Expected returns

**Formula:**
```
Annual Withdrawal = Portfolio Value × VPW Rate
VPW Rate = 1 / Remaining Life Expectancy (adjusted for returns)
```

**Advantages:**
- Self-adjusting - reduces sequence of returns risk
- Higher withdrawals when portfolio grows
- Naturally reduces spending as portfolio depletes
- Based on evidence from academic research (Bogleheads)

**Implementation suggestion:**
```go
func CalculateVPWWithdrawal(portfolioValue float64, age int, expectedReturn float64) float64 {
    remainingYears := LifeExpectancy(age) - age
    vpwRate := CalculateVPWRate(remainingYears, expectedReturn)
    return portfolioValue * vpwRate
}
```

**Best for:** Those comfortable with variable income who want to maximise sustainable withdrawals.

---

### 3. Guardrails Strategy (Guyton-Klinger)

**What it is:** Set upper and lower "guardrails" around a base withdrawal rate. Adjust withdrawals when portfolio performance pushes you outside the guardrails.

**Rules:**
- **Base withdrawal:** 4-5% of initial portfolio, inflation-adjusted
- **Upper guardrail:** If current rate falls below 80% of initial rate → increase withdrawal 10%
- **Lower guardrail:** If current rate exceeds 120% of initial rate → decrease withdrawal 10%

**Advantages:**
- Balances stability with responsiveness
- Prevents catastrophic depletion
- Captures upside when markets perform well
- Well-researched (high historical success rates)

**Implementation suggestion:**
```go
type GuardrailsConfig struct {
    InitialWithdrawalRate float64
    UpperGuardrail        float64  // e.g., 0.80
    LowerGuardrail        float64  // e.g., 1.20
    AdjustmentPercent     float64  // e.g., 0.10
}
```

**Best for:** Those wanting more stability than VPW but more responsiveness than fixed withdrawals.

---

### 4. Bucket Strategy

**What it is:** Divide assets into three "buckets" based on time horizon:

| Bucket | Horizon | Assets | Purpose |
|--------|---------|--------|---------|
| **Short-term** | 0-2 years | Cash, money market | Immediate income needs |
| **Medium-term** | 3-7 years | Bonds, gilts | Refill short-term bucket |
| **Long-term** | 8+ years | Equities | Growth for future |

**Mechanics:**
1. Spend from short-term bucket
2. Periodically refill short-term from medium-term
3. Rebalance long-term to medium-term when equities perform well

**Advantages:**
- Psychological comfort during market downturns
- Reduces need to sell equities in bear markets
- Natural sequence of returns protection
- Separates spending from investment decisions

**Implementation suggestion:**
```go
type BucketConfig struct {
    ShortTermYears   int     // Cash buffer years
    MediumTermYears  int     // Bond allocation years
    RebalanceThreshold float64 // When to move from long to medium
}
```

**Best for:** Risk-averse investors who worry about market volatility.

---

### 5. State Pension Deferral Strategy

**What it is:** Delay claiming state pension for increased payments.

**UK Rules (2024/25):**
- Deferral increases pension by ~5.8% per year deferred
- Can take as increased weekly amount or lump sum
- Break-even typically around 17-20 years

**Integration with drawdown:**
- Model scenarios with state pension starting at 67, 68, 69, 70
- Calculate break-even age for each scenario
- Factor in health/life expectancy

**Implementation suggestion:**
```go
type StatePensionDeferral struct {
    DeferralYears     int
    EnhancementRate   float64  // Currently 5.8% per year
    BreakEvenAge      int      // Calculated
}
```

**Best for:** Those in good health with sufficient other income to bridge the gap.

---

### 6. Annuity Floor Strategy

**What it is:** Purchase an annuity to cover essential expenses, use drawdown for discretionary spending.

**Structure:**
- **Annuity:** Covers fixed costs (utilities, food, council tax, insurance)
- **Drawdown:** Covers variable/luxury spending (holidays, hobbies)

**Advantages:**
- Guaranteed income for essentials regardless of markets
- Freedom to take investment risk with discretionary pot
- Reduces stress about running out of money
- Can use enhanced annuity rates if health conditions apply

**Implementation suggestion:**
```go
type AnnuityFloorConfig struct {
    AnnuityPurchaseAge     int
    AnnuityPurchaseAmount  float64
    AnnuityType            string  // "Single", "Joint", "Escalating"
    EssentialExpenses      float64 // Monthly amount to cover
}

func CalculateAnnuityIncome(purchaseAmount float64, age int, type string) float64 {
    // Use current annuity rates (API or lookup table)
}
```

**Best for:** Those who prioritise security over maximising income.

---

### 7. Small Pot Rules Exploitation

**What it is:** UK rules allow taking small pension pots (under £10,000) as a lump sum with 25% tax-free.

**Strategy:**
- Identify small pots across multiple providers
- Take as lump sums before crystallising main pot
- First 25% is tax-free, remainder taxed as income
- Up to 3 small pots can be taken this way

**Implementation suggestion:**
- Add field for multiple pension pots per person
- Flag pots eligible for small pot rules
- Model taking small pots first before main drawdown

**Best for:** Those with multiple small workplace pensions.

---

### 8. Spouse Income Splitting Optimisation

**What it is:** Better optimisation of income between spouses to minimise combined tax.

**Current gap:** The Tax Optimized strategy uses proportional allocation by pot size, but doesn't fully optimise for:
- Filling both personal allowances before any basic rate tax
- Balancing state pension start dates
- One spouse with DB pension, one without

**Improved algorithm:**
```go
func OptimalSpouseAllocation(need float64, people []Person) map[string]float64 {
    // 1. Calculate remaining personal allowance for each
    // 2. Fill lower earner's allowance first
    // 3. Balance basic rate usage between spouses
    // 4. Consider future state pension to avoid bunching
}
```

**Best for:** Couples with unequal pension pots or different retirement ages.

---

### 9. Inheritance Tax (IHT) Optimised Strategy

**What it is:** Prioritise spending assets that fall within IHT scope, preserve pension (which sits outside estate).

**UK rules:**
- Pensions typically outside estate for IHT
- ISAs and other assets within estate
- Estate over £325,000 (or £500,000 with residence) faces 40% IHT

**Strategy:**
- Spend ISAs and taxable accounts first
- Preserve pension for beneficiaries
- Consider passing on wealth before death via gifts

**Implementation suggestion:**
```go
type IHTOptimisedConfig struct {
    EstateThreshold    float64
    PreservePensionForIHT bool
    GiftingStrategy    string  // "None", "Annual", "SevenYear"
}
```

**Best for:** Those with significant assets who want to pass wealth to beneficiaries tax-efficiently.

---

### 10. Sequence of Returns Risk Mitigation

**What it is:** Strategies to protect against poor market returns in early retirement years.

**Options to model:**
1. **Cash buffer:** Hold 1-3 years expenses in cash
2. **Bond tent:** Increase bond allocation at retirement, gradually shift back to equities
3. **Flexible spending:** Reduce withdrawals by 10-20% during market downturns
4. **Part-time work:** Bridge income with earnings in early years

**Implementation suggestion:**
```go
type SequenceRiskConfig struct {
    CashBufferYears    int
    BondTentEnabled    bool
    FlexibleSpending   bool
    FlexReductionRate  float64  // How much to cut in downturns
}
```

**Best for:** Those retiring into uncertain markets or with concentrated equity exposure.

---

### 11. Phased Retirement Strategy

**What it is:** Model gradual transition from full-time work to full retirement with part-time work phase.

**Structure:**
- **Phase 1 (Age 55-60):** Full-time work, continue pension contributions
- **Phase 2 (Age 60-65):** Part-time work, partial pension access
- **Phase 3 (Age 65+):** Full retirement, full drawdown

**Advantages:**
- Delays full drawdown, extending pot life
- Allows continued pension contributions
- Smoother lifestyle transition
- May preserve higher rate tax relief on contributions

**Implementation suggestion:**
```go
type PhasedRetirementConfig struct {
    PartTimeStartAge    int
    PartTimeEndAge      int
    PartTimeIncome      float64  // Annual from work
    ContinueContributions bool
    ContributionAmount  float64
}
```

**Best for:** Those who can and want to work part-time in early "retirement".

---

### 12. DB Pension Options Modelling

**What it is:** Model the trade-offs for defined benefit pension decisions.

**Options to add:**
1. **Commutation:** Take 25% as lump sum for reduced annual pension
2. **Early retirement factors:** Model taking DB early with reduction
3. **Late retirement factors:** Model deferring DB for enhancement
4. **Transfer value:** Compare keeping DB vs. transferring to DC

**Implementation suggestion:**
```go
type DBPensionConfig struct {
    FullPension          float64
    NormalRetirementAge  int
    EarlyRetirementFactor float64  // e.g., 3-6% per year early
    LateRetirementFactor  float64  // e.g., 5% per year late
    CommutationFactor     float64  // e.g., 12:1 (£12 lump sum per £1 pension)
}
```

**Best for:** Those with DB pensions facing decisions about when/how to take them.

---

### 13. Emergency Fund Preservation

**What it is:** Always maintain a minimum emergency fund regardless of strategy.

**Rules:**
- Never draw ISA below emergency threshold (e.g., 6 months expenses)
- Emergency fund grows with inflation
- Only breach in genuine emergency scenarios

**Implementation suggestion:**
```go
type EmergencyFundConfig struct {
    MinimumMonths     int      // e.g., 6 months
    PreserveInISA     bool     // Keep emergency in ISA
    InflationAdjust   bool     // Grow threshold with inflation
}
```

**Best for:** Those wanting security against unexpected expenses.

---

## Strategy Comparison Matrix

| Strategy | Tax Efficiency | Flexibility | Security | Complexity |
|----------|----------------|-------------|----------|------------|
| Current: Tax Optimized | High | Medium | Low | Medium |
| Current: Pension to ISA | High | Medium | Medium | Medium |
| **New: UFPLS** | Medium | High | Low | Low |
| **New: VPW** | Medium | High | Medium | Medium |
| **New: Guardrails** | Medium | Medium | High | Medium |
| **New: Bucket** | Medium | Medium | High | High |
| **New: Annuity Floor** | Low | Low | Very High | Medium |
| **New: IHT Optimised** | Low | Medium | Medium | Medium |

---

## Implementation Priority Recommendation

### High Priority - IMPLEMENTED

1. **UFPLS Strategy** - DONE: Added flexible lump sum withdrawals
2. **State Pension Deferral** - DONE: Configurable deferral with enhancement
3. **Enhanced Spouse Optimisation** - DONE: Improved personal allowance filling
4. **Emergency Fund Preservation** - DONE: Configurable ISA minimum threshold

### Medium Priority - IMPLEMENTED

5. **Guardrails Strategy** - DONE: Guyton-Klinger dynamic withdrawal adjustments based on portfolio performance
6. **DB Pension Options** - DONE: Commutation (lump sum), early/late retirement factors
7. **Phased Retirement** - DONE: Part-time work income bridge with configurable start/end ages
8. **VPW Strategy** - DONE: Variable Percentage Withdrawal based on age and life expectancy

### Lower Priority (Specialised Use Cases - Not Yet Implemented)

9. **Bucket Strategy** - More psychological than mathematical benefit
10. **Annuity Floor** - Requires annuity rate data integration
11. **IHT Optimisation** - Specialised for wealthier clients
12. **Sequence Risk Mitigation** - Can be approximated with sensitivity analysis

---

## Additional Enhancements

### Sensitivity Analysis Improvements

Current sensitivity analysis varies growth rates. Consider adding:
- Inflation rate sensitivity
- Longevity sensitivity (different life expectancy assumptions)
- Tax band freeze scenarios (as currently happening in UK)
- State pension age increase scenarios (67 → 68)

### Monte Carlo Simulation

Replace or supplement deterministic projections with:
- Random return sequences
- Probability of success metrics
- Confidence intervals on outcomes
- Stress testing (e.g., 2008-style crash in year 1)

### Scenario Comparison

Side-by-side comparison tool:
- "What if I retire at 55 vs 60?"
- "What if I take PCLS vs keep it invested?"
- "What if I buy an annuity at 75?"

---

## Summary

The implementation now includes comprehensive retirement planning strategies:

### Implemented Features (8 strategies)
1. **UFPLS Strategy** - Flexible lump sum withdrawals
2. **State Pension Deferral** - Delayed claiming with enhancement
3. **Enhanced Spouse Optimisation** - Optimal tax band usage across couples
4. **Emergency Fund Preservation** - Minimum ISA threshold protection
5. **Guardrails Strategy** - Guyton-Klinger dynamic withdrawal adjustments
6. **DB Pension Options** - Commutation, early/late retirement factors
7. **Phased Retirement** - Part-time work income bridge
8. **VPW Strategy** - Variable percentage withdrawal by age

### Remaining Lower Priority Items
- Bucket Strategy (psychological comfort)
- Annuity Floor (requires rate data)
- IHT Optimisation (specialised)
- Sequence Risk Mitigation (approximated via sensitivity analysis)

The tool now covers the most common retirement planning scenarios with robust tax optimisation and flexible income strategies.