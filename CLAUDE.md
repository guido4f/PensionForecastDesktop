# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
make build          # Build for current platform
make test           # Run all tests
go test -v -run TestName ./...  # Run single test

# Web server mode (no CGO, cross-platform)
make web            # Build and run (auto port, opens browser)
make web-port PORT=8080  # Custom port
make web-all        # Build for all platforms

# Embedded UI mode (requires CGO + GTK3 on Linux)
make run-ui         # Build and run embedded browser window
make ui-linux       # Linux build (requires native GTK3)

# Console mode (no CGO, smallest binary)
make console-all    # Build for all platforms
```

## Architecture

Single-package Go application simulating UK pension drawdown with tax optimization.

### Core Flow

`main.go` → parses flags, selects mode (web/ui/console) → loads `config.yaml` → runs simulation via `simulation.go` → outputs results via `output.go` or `html_report.go`

### Key Files

- **simulation.go** - Main simulation loop, year-by-year calculations
- **strategies.go** - Crystallisation (Gradual/UFPLS) and drawdown order logic (SavingsFirst/PensionFirst/TaxOptimized/PensionToISA/PensionOnly)
- **depletion.go** - Binary search to find max sustainable income for target depletion age
- **tax.go** - UK tax band calculations with personal allowance taper
- **webserver.go** - REST API (`POST /api/simulate`) and web interface
- **html_report.go** - Interactive HTML report generation
- **config.go** - YAML config loading/saving, `default-config.yaml` embedded via `//go:embed`

### Operating Modes

1. **Fixed Income** - Specify required monthly income, see how long funds last
2. **Depletion Mode** (`-depletion`) - Specify target age, find max sustainable income
3. **Pension-Only** (`-pension-only`) - Deplete pensions, preserve ISAs
4. **Pension-to-ISA** (`-pension-to-isa`) - Over-draw pension to fill tax bands, transfer to ISA
5. **Sensitivity** (`-sensitivity`) - Grid analysis across growth rate ranges

### Strategies (8 combinations tested)

Each simulation tests combinations of:
- **Crystallisation**: Gradual (25% tax-free per withdrawal) vs UFPLS (flexible, preserves 25% entitlement)
- **Drawdown Order**: SavingsFirst, PensionFirst, TaxOptimized, PensionToISA, PensionOnly
- **Mortgage**: Early payoff, Normal, Extended, PCLS lump sum payoff

### Advanced Features

- **Guardrails** (Guyton-Klinger) - Dynamic withdrawal adjustments based on portfolio performance (`guardrails.go`)
- **VPW** - Age-based variable percentage withdrawal rates (`vpw.go`)
- **State Pension Deferral** - 5.8%/year enhancement for delayed claiming
- **DB Pension Options** - Commutation, early/late retirement factors
- **Phased Retirement** - Part-time work income bridge
- **Emergency Fund** - Minimum ISA threshold preservation

### Build Tags

- No tag: Full build with webview (requires CGO)
- `console`: Console-only, no webview dependency

### Types (types.go)

- `Person` - Individual financial state (pensions, ISAs, ages, DB pension config)
- `SimulationParams` - Strategy combination being tested
- `YearState` - Complete year breakdown (income, withdrawals, tax, balances)
- `SimulationResult` - Final results with all yearly states

## Testing

```bash
make test                           # All tests
go test -v -run TestTax ./...       # Tax calculation tests
go test -v -run TestScenario ./...  # Complex scenario tests
```

Test files cover: tax calculations (`tax_test.go`), crystallisation logic, mortgage scenarios, financial invariants, optimizer validation.

## Configuration

`config.yaml` - User config (auto-saved in interactive mode)
`default-config.yaml` - Embedded template

Key sections: `people` (2), `financial` (growth/inflation rates), `income` (requirements), `mortgage`, `simulation` (start year, end age).
