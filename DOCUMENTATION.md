# goPensionForecast - Complete Documentation

A sophisticated UK pension drawdown tax optimization simulator that helps retirees determine the most tax-efficient retirement income strategy.

---

## Table of Contents

1. [Overview](#overview)
2. [Operating Modes](#operating-modes)
3. [Command Line Interface](#command-line-interface)
4. [Strategies](#strategies)
5. [Configuration Reference](#configuration-reference)
6. [Tax Calculations](#tax-calculations)
7. [Advanced Features](#advanced-features)
8. [Output Formats](#output-formats)
9. [Web Server & API](#web-server--api)
10. [Examples](#examples)

---

## Overview

goPensionForecast is a Go application that simulates UK pension drawdown scenarios to identify the most tax-efficient retirement strategy. It compares multiple approaches to withdrawing money from pensions and ISAs, accounting for UK tax rules, state pension, defined benefit pensions, and mortgage payments.

### Core Question Answered

> "Which combination of withdrawal strategies, crystallisation methods, and mortgage options will minimize tax while achieving my retirement income goals?"

### Key Capabilities

- Compare 8+ strategy combinations automatically
- Calculate sustainable income for a target depletion age
- Model UK tax rules including Personal Allowance tapering
- Support couples with separate pensions and ISAs
- Generate detailed HTML/PDF reports with year-by-year breakdowns
- Run sensitivity analysis across growth rate scenarios

---

## Operating Modes

### 1. Fixed Income Mode (Default)

**Purpose:** Determine how long funds will last with a specified income.

**Use Case:** "Can I afford £4,000/month in retirement? When will I run out of money?"

**How It Works:**
- You specify your required monthly income (can vary by age)
- Simulator calculates how long funds last under each strategy
- Compares total tax paid across strategies
- Shows which strategy is most tax-efficient

**Command:**
```bash
./goPensionForecast                    # Console output
./goPensionForecast -html              # Generate HTML report
./goPensionForecast -details           # Year-by-year breakdown
```

---

### 2. Depletion Mode

**Purpose:** Find the maximum sustainable income for a target end age.

**Use Case:** "How much can I spend if I want funds to last until age 90?"

**How It Works:**
- You specify a target depletion age (when funds should reach zero)
- Simulator uses binary search to find maximum sustainable income
- Supports income ratios for different life phases (e.g., spend more early)
- Reports sustainable income for each strategy

**Command:**
```bash
./goPensionForecast -depletion
./goPensionForecast -depletion -html
```

**Income Ratios Example:**
```yaml
income_requirements:
  tiers:
    - end_age: 70
      ratio: 5.0      # Spend 5× the base multiplier before 70
    - start_age: 70
      ratio: 3.0      # Spend 3× the base multiplier after 70
  target_depletion_age: 90
```

---

### 3. Pension-Only Depletion Mode

**Purpose:** Deplete pensions while preserving ISAs/savings.

**Use Case:** "I want to use up my pension first and keep my ISA for emergencies or inheritance."

**How It Works:**
- Focuses exclusively on pension drawdown
- ISA balances are preserved (only used if pension depleted)
- Finds maximum sustainable income from pension alone
- Useful for inheritance tax planning (pensions can pass tax-free)

**Command:**
```bash
./goPensionForecast -pension-only
./goPensionForecast -pension-only -html
```

---

### 4. Pension-to-ISA Mode

**Purpose:** Efficiently move excess pension to ISA to minimize lifetime tax.

**Use Case:** "I have a large pension and want to extract it tax-efficiently into my ISA over time."

**How It Works:**
- Over-draws pension to fill Personal Allowance + Basic Rate band
- Excess (after tax) deposited into ISA
- ISA shields future growth from tax
- Particularly beneficial if you expect to pay higher tax rates later

**Command:**
```bash
./goPensionForecast -pension-to-isa
./goPensionForecast -pension-to-isa -html
```

---

### 5. Sensitivity Analysis

**Purpose:** See how results vary across different market conditions.

**Use Case:** "What if growth rates are lower than expected? Which strategy is most robust?"

**How It Works:**
- Tests combinations of pension and ISA growth rates
- Creates a matrix showing best strategy at each growth rate combination
- Identifies which strategies are sensitive to assumptions
- Available in all modes (fixed, depletion, pension-to-isa)

**Command:**
```bash
./goPensionForecast -sensitivity
./goPensionForecast -depletion -sensitivity
./goPensionForecast -pension-to-isa -sensitivity
```

**Configuration:**
```yaml
sensitivity:
  pension_growth_min: 0.02
  pension_growth_max: 0.08
  savings_growth_min: 0.02
  savings_growth_max: 0.08
  step_size: 0.01
```

---

## Command Line Interface

### Mode Selection Flags

| Flag | Description |
|------|-------------|
| `-web` | Start web server with external browser |
| `-ui` | Start embedded browser window (requires CGO) |
| `-console` | Force console output (no GUI) |

### Simulation Mode Flags

| Flag | Description |
|------|-------------|
| (none) | Fixed income mode |
| `-depletion` | Depletion mode (find sustainable income) |
| `-pension-only` | Pension-only depletion |
| `-pension-to-isa` | Pension-to-ISA depletion |
| `-sensitivity` | Run sensitivity analysis |

### Output Flags

| Flag | Description |
|------|-------------|
| `-html` | Generate interactive HTML reports |
| `-details` | Show year-by-year breakdown for all strategies |
| `-drawdown` | Show detailed withdrawal sources for best strategy |
| `-year 2030` | Show detailed breakdown for specific year |

### Configuration Flags

| Flag | Description |
|------|-------------|
| `-config file.yaml` | Use custom configuration file |
| `-addr :8080` | Web server port (default: auto-assign) |

### Examples

```bash
# Quick console comparison
./goPensionForecast

# Generate HTML report
./goPensionForecast -html

# Find sustainable income for age 90, with report
./goPensionForecast -depletion -html

# Run web interface
./goPensionForecast -web

# Sensitivity analysis with depletion
./goPensionForecast -depletion -sensitivity -html

# Use custom config
./goPensionForecast -config my-scenario.yaml -html
```

---

## Strategies

The simulator automatically compares combinations of crystallisation methods, drawdown orders, and mortgage options.

### Crystallisation Methods

#### Gradual Crystallisation
- Each withdrawal crystallises a portion of the uncrystallised pot
- 25% of crystallised amount is tax-free
- Remaining 75% is taxable income
- Preserves flexibility for future 25% tax-free access

#### UFPLS (Uncrystallised Funds Pension Lump Sum)
- Each withdrawal: 25% tax-free, 75% taxable
- No separate crystallisation step
- Simpler but less flexible
- Entire pot remains uncrystallised until withdrawn

### Drawdown Orders

| Order | Description |
|-------|-------------|
| **Savings First** | Deplete ISA before touching pension. Good if pension has valuable tax-free component. |
| **Pension First** | Deplete pension before ISA. Good for inheritance (pension passes tax-free). |
| **Tax Optimized** | Dynamically choose source each year to minimize tax. Most sophisticated approach. |
| **Pension to ISA** | Over-draw pension to fill tax bands, deposit excess to ISA. |
| **Pension to ISA Proactive** | More aggressive pension-to-ISA transfers. |
| **Pension Only** | Only draw from pension, preserve ISA entirely. |
| **Fill Basic Rate** | Draw pension up to basic rate threshold only. |
| **State Pension Bridge** | Bridge income gap until state pension starts. |

### Mortgage Options

| Option | Description |
|--------|-------------|
| **Early Payoff** | Pay off mortgage at specified early payoff year |
| **Normal Payoff** | Let mortgage run to natural end date |
| **Extended** | Extend mortgage term (e.g., +10 years) |
| **PCLS Payoff** | Use 25% Pension Commencement Lump Sum to pay mortgage |

### Strategy Permutation Modes

The simulator can test different numbers of strategy combinations:

| Mode | Combinations | Use Case |
|------|--------------|----------|
| Quick | ~8 | Fast comparison of core strategies |
| Standard | ~100 | Common variations |
| Thorough | ~250 | Detailed analysis |
| Comprehensive | ~4,000 | All valid combinations |

---

## Configuration Reference

### config.yaml Structure

```yaml
# Person Configuration
people:
  - name: "Person1"
    birth_date: "1970-12-15"        # Date of birth (YYYY-MM-DD)
    retirement_date: "2026-07-15"   # When work income ends
    retirement_age: 55               # Alternative to retirement_date
    pension_access_age: 55           # When DC pension accessible (min 55)
    state_pension_age: 67            # State pension start age
    tax_free_savings: 100000         # ISA balance
    pension: 500000                  # Total DC pension pot
    isa_annual_limit: 20000          # Annual ISA contribution limit
    work_income_net: 3500            # Monthly take-home pay (preferred)
    work_income: 50000               # Annual gross salary (legacy)

    # Defined Benefit Pension
    db_pension_amount: 15000         # Annual DB pension amount
    db_pension_start_age: 60         # When DB pension starts
    db_pension_name: "Civil Service"
    db_pension_normal_age: 65        # Normal retirement age for scheme
    db_pension_early_factor: 0.04    # 4% reduction per year early
    db_pension_late_factor: 0.05     # 5% increase per year late
    db_pension_commutation: 0.25     # Fraction to commute (max 0.25)
    db_pension_commute_factor: 12    # Lump sum per £1 pension given up

    # Part-Time / Phased Retirement
    part_time_income: 20000          # Annual part-time income
    part_time_start_age: 60          # When part-time work starts
    part_time_end_age: 65            # When part-time work ends

    # State Pension Deferral
    state_pension_defer_years: 0     # Years to defer (0, 2, or 5)

  - name: "Person2"
    # ... second person configuration

# Financial Parameters
financial:
  pension_growth_rate: 0.05          # Annual pension growth
  savings_growth_rate: 0.05          # Annual ISA growth
  income_inflation_rate: 0.03        # Annual increase in income needs
  state_pension_amount: 12547.60     # Current full state pension
  state_pension_inflation: 0.03      # Annual state pension increase
  tax_band_inflation: 0.02           # Tax band adjustment rate

  # Growth Rate Decline (Age in Bonds)
  growth_decline_enabled: false
  pension_growth_end_rate: 0.04
  savings_growth_end_rate: 0.04
  growth_decline_target_age: 80
  growth_decline_reference_person: "Person1"

  # Depletion Mode Growth Decline
  depletion_growth_decline_enabled: false
  depletion_growth_decline_percent: 0.03

  # Emergency Fund
  emergency_fund_months: 6           # Minimum ISA balance
  emergency_fund_inflation_adjust: false

# Income Requirements
income_requirements:
  # Tiered Income (Recommended)
  tiers:
    - end_age: 65
      monthly_amount: 5000
      is_percentage: false
    - start_age: 65
      end_age: 75
      monthly_amount: 5              # 5% annual withdrawal rate
      is_percentage: true
    - start_age: 75
      monthly_amount: 3000
      is_percentage: false
  reference_person: "Person1"

  # Depletion Mode Ratios
  target_depletion_age: 90

  # Guardrails (Guyton-Klinger)
  guardrails_enabled: false
  guardrails_upper_limit: 1.20
  guardrails_lower_limit: 0.80
  guardrails_adjustment: 0.10

# Legacy Income Format (still supported)
income_requirements:
  monthly_before_age: 4000
  monthly_after_age: 2500
  age_threshold: 67
  income_ratio_phase1: 5.0
  income_ratio_phase2: 3.0

# Mortgage Configuration
mortgage:
  parts:
    - name: "Main Residence"
      principal: 200000
      interest_rate: 0.045
      is_repayment: true
      term_years: 20
      start_year: 2020
  end_year: 2040
  early_payoff_year: 2035
  allow_extension: true
  extended_end_year: 2050

# ISA to SIPP Transfers
isa_to_sipp:
  enabled: false
  pension_annual_allowance: 60000
  employer_contribution: 10000
  max_percent: 100
  preserve_months: 12

# Simulation Parameters
simulation:
  start_year: 2026
  end_age: 95
  reference_person: "Person1"

# Tax Configuration
tax_bands:
  - name: "Personal Allowance"
    lower: 0
    upper: 12570
    rate: 0.00
  - name: "Basic Rate"
    lower: 12570
    upper: 50270
    rate: 0.20
  - name: "Higher Rate"
    lower: 50270
    upper: 125140
    rate: 0.40
  - name: "Additional Rate"
    lower: 125140
    upper: 999999999
    rate: 0.45

tax:
  personal_allowance: 12570
  tapering_threshold: 100000
  tapering_rate: 0.5

# Sensitivity Analysis
sensitivity:
  pension_growth_min: 0.02
  pension_growth_max: 0.08
  savings_growth_min: 0.02
  savings_growth_max: 0.08
  step_size: 0.01
```

---

## Tax Calculations

### UK Tax Features Implemented

#### Personal Allowance Tapering
- Standard Personal Allowance: £12,570
- Taper threshold: £100,000
- For every £2 of income over £100,000, £1 of PA is lost
- PA completely eliminated at £125,140
- Creates effective 60% marginal rate in £100k-£125k band

#### Tax Bands (2024/25)

| Band | Income Range | Rate |
|------|--------------|------|
| Personal Allowance | £0 - £12,570 | 0% |
| Basic Rate | £12,570 - £50,270 | 20% |
| Higher Rate | £50,270 - £125,140 | 40% |
| Additional Rate | Over £125,140 | 45% |

#### Pension Crystallisation

**25% Tax-Free (PCLS):**
- Take 25% of uncrystallised pension as tax-free lump sum
- Remaining 75% becomes crystallised and fully taxable on withdrawal

**Gradual Crystallisation:**
- Crystallise portions over time
- Each crystallisation: 25% tax-free, 75% to crystallised pot
- Future withdrawals from crystallised pot are 100% taxable

**UFPLS:**
- No formal crystallisation step
- Each withdrawal: 25% tax-free, 75% taxable
- Simpler but less control over timing

#### State Pension

- Fixed annual amount (currently ~£12,547.60)
- Fully taxable (no 25% tax-free element)
- Inflates at specified rate annually
- **Deferral:** 5.8% enhancement per year deferred
  - Can defer 0, 2, or 5 years
  - Enhancement compounds

#### Defined Benefit Pensions

- Annual amount paid for life
- **Early retirement factor:** Reduction per year taken early
- **Late retirement factor:** Increase per year taken late
- **Commutation:** Convert up to 25% to tax-free lump sum
  - Lump sum = pension given up × commutation factor

---

## Advanced Features

### Guardrails (Guyton-Klinger Strategy)

Dynamic withdrawal adjustments based on portfolio performance:

- **Upper Guardrail (120%):** If withdrawal rate rises above 120% of initial rate, reduce spending by 10%
- **Lower Guardrail (80%):** If withdrawal rate falls below 80% of initial rate, increase spending by 10%

```yaml
income_requirements:
  guardrails_enabled: true
  guardrails_upper_limit: 1.20
  guardrails_lower_limit: 0.80
  guardrails_adjustment: 0.10
```

### Growth Rate Decline (Age in Bonds)

Gradually shift from equities to bonds over time:

```yaml
financial:
  growth_decline_enabled: true
  pension_growth_rate: 0.07          # Starting rate
  pension_growth_end_rate: 0.04      # Ending rate
  growth_decline_target_age: 80      # When to reach end rate
```

Formula: Linear interpolation from start rate to end rate.

### ISA to SIPP Transfers

Convert ISA to pension while working for tax relief:

```yaml
isa_to_sipp:
  enabled: true
  pension_annual_allowance: 60000
  employer_contribution: 10000
  max_percent: 100
  preserve_months: 12
```

**Benefit:** Tax relief on pension contributions effectively doubles the transfer.

### Maximize Couple ISA

For couples, fill both ISA allowances from one person's pension:

- Extract £40k from high-earner's pension
- Deposit £20k to each person's ISA
- Maximizes tax-sheltered growth

### Stock Market Historical Data

Built-in returns for 15+ major indices:

- UK: FTSE 100, FTSE 250, FTSE AIM 100, FTSE All-Share
- US: S&P 500, NASDAQ, Dow Jones
- Europe: DAX
- Asia: Nikkei, Hang Seng
- Global: MSCI World, FTSE All-World

Access via: `GET /api/stock-indices`

---

## Output Formats

### Console Output

```
Strategy                     | Total Tax | Final Balance | Ran Out
-----------------------------------------------------------------
Pension First (Gradual)      | £45,234   | £123,456      | No
Savings First (UFPLS)        | £52,891   | £98,234       | No
Tax Optimized (Gradual)      | £41,567   | £145,678      | No  ★ BEST
```

### HTML Reports

- Interactive tables with year-by-year data
- Withdrawal breakdown charts
- Color-coded events (pension drawn, ISA depleted, ran out)
- Strategy comparison side-by-side
- Opens automatically in browser

### PDF Reports

Professional action plan documents containing:

- Title page with strategy summary
- Strategy overview and explanation
- Yearly summary table
- Year-by-year detailed pages with:
  - Monthly breakdown/schedule
  - Withdrawal sources
  - Tax calculations
  - Mortgage payoff details
- Summary page with key metrics

### CSV Export

Year-by-year data for import into spreadsheets.

---

## Web Server & API

### Starting the Server

```bash
./goPensionForecast -web              # Auto port, opens browser
./goPensionForecast -web -addr :8080  # Custom port
./goPensionForecast -ui               # Embedded browser (requires CGO)
```

### REST API Endpoints

#### Configuration

```
GET /api/config
Returns: Current configuration object
```

#### Simulations

```
POST /api/simulate
POST /api/simulate/fixed
POST /api/simulate/depletion
POST /api/simulate/pension-only
POST /api/simulate/pension-to-isa
POST /api/simulate/sensitivity

Body: APISimulationRequest
Returns: APISimulationResponse
```

#### Exports

```
POST /api/export-csv
POST /api/export-pdf
GET /api/download-pdf?file=path
POST /api/open-folder
POST /api/open-file
```

#### Stock Data

```
GET /api/stock-indices
Returns: Historical returns for major indices
```

### API Response Structure

```json
{
  "success": true,
  "results": [
    {
      "strategy_idx": 0,
      "rank": 1,
      "strategy": "Tax Optimized (Gradual)",
      "short_name": "TaxOpt/Gradual",
      "total_tax_paid": 45000,
      "total_withdrawn": 1200000,
      "total_income": 1155000,
      "ran_out_of_money": false,
      "final_balance": 250000,
      "years": [
        {
          "year": 2026,
          "tax_year_label": "2026/27",
          "ages": {"Person1": 55, "Person2": 53},
          "required_income": 60000,
          "net_income_required": 45000,
          "state_pension": 0,
          "db_pension": 15000,
          "tax_paid": 5000,
          "total_balance": 1200000,
          "isa_withdrawal": 20000,
          "pension_withdrawal": 25000,
          "tax_free_withdrawal": 6250
        }
      ]
    }
  ],
  "best": { ... }
}
```

---

## Examples

### Example 1: Simple Retirement Check

**Scenario:** Can I afford £3,500/month from age 60?

```yaml
people:
  - name: "John"
    birth_date: "1965-03-15"
    retirement_age: 60
    state_pension_age: 67
    tax_free_savings: 150000
    pension: 400000

income_requirements:
  tiers:
    - monthly_amount: 3500
```

```bash
./goPensionForecast -html
```

### Example 2: Phased Income Requirements

**Scenario:** Spend more while active (60-70), less later.

```yaml
income_requirements:
  tiers:
    - end_age: 70
      monthly_amount: 5000
    - start_age: 70
      end_age: 80
      monthly_amount: 3500
    - start_age: 80
      monthly_amount: 2500
```

### Example 3: Find Sustainable Income

**Scenario:** What's the maximum I can spend if funds must last to 95?

```yaml
income_requirements:
  target_depletion_age: 95
  tiers:
    - end_age: 70
      ratio: 5.0    # Spend more in early retirement
    - start_age: 70
      ratio: 3.0    # Reduce spending later
```

```bash
./goPensionForecast -depletion -html
```

### Example 4: Couple with Different Ages

```yaml
people:
  - name: "Alice"
    birth_date: "1968-06-01"
    retirement_age: 57
    state_pension_age: 67
    tax_free_savings: 200000
    pension: 600000

  - name: "Bob"
    birth_date: "1965-11-20"
    retirement_age: 60
    state_pension_age: 66
    tax_free_savings: 100000
    pension: 300000
    db_pension_amount: 12000
    db_pension_start_age: 60
```

### Example 5: Sensitivity Analysis

**Scenario:** How robust is my plan across different growth rates?

```yaml
sensitivity:
  pension_growth_min: 0.02
  pension_growth_max: 0.08
  savings_growth_min: 0.02
  savings_growth_max: 0.08
  step_size: 0.01
```

```bash
./goPensionForecast -depletion -sensitivity -html
```

---

## Limitations & Assumptions

1. **Discrete Years:** Simulations run year-by-year, not monthly
2. **Deterministic:** Uses average growth rates (no Monte Carlo)
3. **UK Only:** Tax calculations specific to UK
4. **Linear Inflation:** Inflation applied at year boundaries
5. **No Sequence Risk:** Average growth applied uniformly
6. **No Fees Modeling:** Growth rates assumed net of fees
7. **Simplified Mortgage:** No mid-term rate changes

---

## Technical Details

### Build Requirements

- Go 1.21 or later
- CGO required for embedded UI (`-ui` flag)
- No external dependencies for core simulation

### File Structure

```
goPensionForecast/
├── main.go              # CLI entry point
├── types.go             # Core data structures
├── config.go            # Configuration loading
├── simulation.go        # Main simulation loop
├── strategies.go        # Strategy generation
├── depletion.go         # Binary search for sustainable income
├── tax.go               # UK tax calculations
├── guardrails.go        # Guyton-Klinger logic
├── optimizer.go         # Tax optimization
├── webserver.go         # REST API
├── html_report.go       # HTML generation
├── pdf_report.go        # PDF generation
├── config.yaml          # User configuration
└── default-config.yaml  # Template
```

### Running Tests

```bash
make test                              # All tests
go test -v -run TestTax ./...          # Tax tests
go test -v -run TestScenario ./...     # Scenario tests
```

---

## Quick Start

1. **Copy the default config:**
   ```bash
   cp default-config.yaml config.yaml
   ```

2. **Edit config.yaml with your details:**
   - Birth dates and retirement ages
   - Pension and ISA balances
   - Income requirements
   - Mortgage details (if applicable)

3. **Run the simulator:**
   ```bash
   ./goPensionForecast -html
   ```

4. **Review the HTML report** that opens in your browser

5. **Try different modes:**
   ```bash
   ./goPensionForecast -depletion -html       # Find sustainable income
   ./goPensionForecast -sensitivity -html     # Test across growth rates
   ```

---

*Generated for goPensionForecast v0.0.35+*
