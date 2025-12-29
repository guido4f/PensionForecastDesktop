package main

import (
	"fmt"
	"strings"
)

// getTotalFinalBalance calculates the total final balance across all people
func getTotalFinalBalance(r SimulationResult) float64 {
	total := 0.0
	for _, bal := range r.FinalBalances {
		total += bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot
	}
	return total
}

// FormatMoney formats a float as a currency string
func FormatMoney(amount float64) string {
	if amount >= 1000000 {
		return fmt.Sprintf("£%.2fM", amount/1000000)
	}
	if amount >= 1000 {
		return fmt.Sprintf("£%.0fk", amount/1000)
	}
	return fmt.Sprintf("£%.0f", amount)
}

// FormatMoneyFull formats a float as full currency (no abbreviation)
func FormatMoneyFull(amount float64) string {
	return fmt.Sprintf("£%.0f", amount)
}

// PrintHeader prints the simulation header
func PrintHeader(config *Config) {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              PENSION DRAWDOWN TAX OPTIMISATION SIMULATION                    ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("──────────────")

	for _, p := range config.People {
		birthYear := GetBirthYear(p.BirthDate)
		fmt.Printf("  %s: Born %d, Retire at %d, State Pension at %d\n",
			p.Name, birthYear, p.RetirementAge, p.StatePensionAge)
		fmt.Printf("          ISA: %s, Pension: %s\n",
			FormatMoney(p.TaxFreeSavings), FormatMoney(p.Pension))
		if p.DBPensionAmount > 0 {
			fmt.Printf("          %s: %s/year from age %d\n",
				p.DBPensionName, FormatMoney(p.DBPensionAmount), p.DBPensionStartAge)
		}
	}

	fmt.Println()
	fmt.Printf("  Pension Growth: %.0f%% | Savings Growth: %.0f%% | Inflation: %.0f%% | Tax Band Inflation: %.0f%%\n",
		config.Financial.PensionGrowthRate*100,
		config.Financial.SavingsGrowthRate*100,
		config.Financial.IncomeInflationRate*100,
		config.Financial.TaxBandInflation*100)
	fmt.Printf("  Income Need: £%.0f/month (before %d), £%.0f/month (after %d) [after tax]\n",
		config.IncomeRequirements.MonthlyBeforeAge,
		config.IncomeRequirements.AgeThreshold,
		config.IncomeRequirements.MonthlyAfterAge,
		config.IncomeRequirements.AgeThreshold)
	// Show mortgage details
	totalPayoff := config.GetTotalPayoffAmount(config.Mortgage.EndYear)
	annualPayment := config.GetTotalAnnualPayment()
	if len(config.Mortgage.Parts) > 0 {
		fmt.Printf("  Mortgage: £%.0fk balance, £%.0f/year until %d\n",
			totalPayoff/1000, annualPayment, config.Mortgage.EndYear)
		for _, part := range config.Mortgage.Parts {
			typeStr := "Interest-only"
			if part.IsRepayment {
				typeStr = "Repayment"
			}
			fmt.Printf("          %s: £%.0fk @ %.2f%% (%s)\n",
				part.Name, part.Principal/1000, part.InterestRate*100, typeStr)
		}
	}
	fmt.Printf("  Simulation: %d to %s at age %d\n",
		config.Simulation.StartYear,
		config.Simulation.ReferencePerson,
		config.Simulation.EndAge)
	fmt.Println()
}

// PrintResultSummary prints a single strategy's results
func PrintResultSummary(result SimulationResult, config *Config) {
	fmt.Println()
	fmt.Printf("╔══════════════════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║ Strategy: %-66s ║\n", result.Params.String())
	fmt.Printf("╚══════════════════════════════════════════════════════════════════════════════╝\n")
	fmt.Println()

	// Get person names for column headers
	var names []string
	for _, p := range config.People {
		names = append(names, p.Name)
	}

	// Print year-by-year table
	fmt.Printf("%-6s", "Year")
	for _, name := range names {
		fmt.Printf(" %s Age", name[:1])
	}
	fmt.Printf(" │ %10s %10s │ %10s %10s │ %10s │ %10s │ %12s\n",
		"Required", "StatePen", "TaxFree", "Taxable", "Tax Paid", "Net Income", "Balance")

	fmt.Println(strings.Repeat("─", 115))

	// Print every 5th year and key milestone years
	for i, year := range result.Years {
		// Print every 5 years, first year, last year, or milestone years
		isKeyYear := i == 0 || i == len(result.Years)-1 || year.Year%5 == 0 ||
			year.Year == config.Mortgage.EndYear

		if isKeyYear {
			fmt.Printf("%-6d", year.Year)
			for _, name := range names {
				fmt.Printf(" %7d", year.Ages[name])
			}

			totalTaxFree := year.Withdrawals.TotalTaxFree
			totalTaxable := year.Withdrawals.TotalTaxable

			fmt.Printf(" │ %10s %10s │ %10s %10s │ %10s │ %10s │ %12s\n",
				FormatMoney(year.TotalRequired),
				FormatMoney(year.TotalStatePension),
				FormatMoney(totalTaxFree),
				FormatMoney(totalTaxable),
				FormatMoney(year.TotalTaxPaid),
				FormatMoney(year.NetIncomeReceived),
				FormatMoney(year.TotalBalance))
		}
	}

	fmt.Println(strings.Repeat("─", 115))

	// Summary
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  Total Tax Paid:    %s\n", FormatMoney(result.TotalTaxPaid))
	fmt.Printf("  Total Withdrawn:   %s\n", FormatMoney(result.TotalWithdrawn))

	if result.RanOutOfMoney {
		fmt.Printf("  ⚠️  WARNING: Ran out of money in year %d\n", result.RanOutYear)
	}

	// Final balances
	fmt.Println()
	fmt.Println("Final Balances:")
	var totalRemaining float64
	for name, balances := range result.FinalBalances {
		total := balances.TaxFreeSavings + balances.UncrystallisedPot + balances.CrystallisedPot
		if total > 0 {
			fmt.Printf("  %s: ISA %s, Pension %s (total %s)\n",
				name,
				FormatMoney(balances.TaxFreeSavings),
				FormatMoney(balances.CrystallisedPot+balances.UncrystallisedPot),
				FormatMoney(total))
		}
		totalRemaining += total
	}
	fmt.Printf("  Total Remaining:   %s\n", FormatMoney(totalRemaining))
}

// PrintAllComparison prints a comparison of all 4 strategy combinations
func PrintAllComparison(results []SimulationResult) {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                    STRATEGY COMPARISON                                             ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Header
	fmt.Printf("%-25s", "Metric")
	for _, r := range results {
		fmt.Printf(" │ %-18s", r.Params.ShortName())
	}
	fmt.Println()
	fmt.Println(strings.Repeat("─", 25+len(results)*22))

	// Total Tax Paid
	fmt.Printf("%-25s", "Total Tax Paid")
	for _, r := range results {
		fmt.Printf(" │ %-18s", FormatMoney(r.TotalTaxPaid))
	}
	fmt.Println()

	// Total Withdrawn
	fmt.Printf("%-25s", "Total Withdrawn")
	for _, r := range results {
		fmt.Printf(" │ %-18s", FormatMoney(r.TotalWithdrawn))
	}
	fmt.Println()

	// Final Balance
	fmt.Printf("%-25s", "Final Balance")
	for _, r := range results {
		var total float64
		for _, b := range r.FinalBalances {
			total += b.TaxFreeSavings + b.UncrystallisedPot + b.CrystallisedPot
		}
		fmt.Printf(" │ %-18s", FormatMoney(total))
	}
	fmt.Println()

	// Ran Out of Money
	fmt.Printf("%-25s", "Ran Out of Money")
	for _, r := range results {
		status := "No"
		if r.RanOutOfMoney {
			status = fmt.Sprintf("Yes (%d)", r.RanOutYear)
		}
		fmt.Printf(" │ %-18s", status)
	}
	fmt.Println()

	// Savings Remaining (total at end)
	fmt.Printf("%-25s", "Savings Remaining")
	for _, r := range results {
		var total float64
		for _, b := range r.FinalBalances {
			total += b.TaxFreeSavings + b.UncrystallisedPot + b.CrystallisedPot
		}
		fmt.Printf(" │ %-18s", FormatMoney(total))
	}
	fmt.Println()

	fmt.Println(strings.Repeat("─", 25+len(results)*22))

	// Find best strategy (highest final balance among those that don't run out)
	// This properly accounts for the trade-off between paying off mortgages early vs investing
	var bestIdx int = -1
	var bestFinalBalance float64 = -1
	for i, r := range results {
		if !r.RanOutOfMoney {
			finalBalance := getTotalFinalBalance(r)
			if bestFinalBalance < 0 || finalBalance > bestFinalBalance {
				bestFinalBalance = finalBalance
				bestIdx = i
			}
		}
	}

	// If all run out, find the one that lasts longest
	var longestIdx int = 0
	var longestYear int = 0
	allRunOut := bestIdx < 0
	if allRunOut {
		for i, r := range results {
			if r.RanOutYear > longestYear {
				longestYear = r.RanOutYear
				longestIdx = i
			}
		}
	}

	// Recommendation
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                     RECOMMENDATION                                                  ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	if allRunOut {
		fmt.Println("  ⚠️  WARNING: All strategies run out of money before age 90!")
		fmt.Println()
		best := results[longestIdx]
		fmt.Printf("  BEST OPTION: %s\n", best.Params.String())
		fmt.Printf("  Lasts until: %d\n", best.RanOutYear)
		fmt.Printf("  Total Tax: %s\n", FormatMoney(best.TotalTaxPaid))
		fmt.Println()
		fmt.Println("  Comparison (by year money runs out):")
		for _, r := range results {
			marker := "  "
			if r.RanOutYear == longestYear {
				marker = "→ "
			}
			fmt.Printf("    %s%s: %d\n", marker, r.Params.ShortName(), r.RanOutYear)
		}
		fmt.Println()
		fmt.Println("  Consider: reducing income, working longer, or reviewing mortgage payment")
	} else {
		best := results[bestIdx]
		bestBalance := getTotalFinalBalance(best)
		fmt.Printf("  RECOMMENDED: %s\n", best.Params.String())
		fmt.Printf("  Final Balance: %s | Total Tax: %s\n", FormatMoney(bestBalance), FormatMoney(best.TotalTaxPaid))
		fmt.Println()

		// Show comparison vs other strategies that don't run out
		fmt.Println("  Comparison vs Other Viable Strategies:")
		for i, r := range results {
			if i != bestIdx && !r.RanOutOfMoney {
				otherBalance := getTotalFinalBalance(r)
				balanceDiff := bestBalance - otherBalance
				taxDiff := r.TotalTaxPaid - best.TotalTaxPaid
				fmt.Printf("    vs %s: %s more wealth", r.Params.ShortName(), FormatMoney(balanceDiff))
				if taxDiff != 0 {
					if taxDiff > 0 {
						fmt.Printf(", %s less tax", FormatMoney(taxDiff))
					} else {
						fmt.Printf(", %s more tax", FormatMoney(-taxDiff))
					}
				}
				fmt.Println()
			}
		}

		fmt.Println()
		fmt.Println("  Why this strategy wins:")
		if best.Params.EarlyMortgagePayoff {
			fmt.Println("  - Early mortgage payoff frees cash flow sooner")
		} else {
			fmt.Println("  - Keeping investments growing beats mortgage interest cost")
			fmt.Println("  - 5%+ growth > ~4% mortgage rate = more wealth over time")
		}

		if best.Params.DrawdownOrder == PensionToISA {
			fmt.Println("  - Pension-to-ISA fills tax bands efficiently each year")
			fmt.Println("  - Converts taxable pension to tax-free ISA for later years")
		}
	}

	fmt.Println()
}

// PrintDetailedYear prints detailed information for a specific year
func PrintDetailedYear(year YearState, config *Config) {
	fmt.Printf("\n=== Year %d Detail ===\n", year.Year)
	fmt.Printf("Ages: ")
	for name, age := range year.Ages {
		fmt.Printf("%s=%d ", name, age)
	}
	fmt.Println()

	fmt.Printf("Required Income: %s\n", FormatMoney(year.RequiredIncome))
	fmt.Printf("Mortgage Cost: %s\n", FormatMoney(year.MortgageCost))
	fmt.Printf("Total Required: %s\n", FormatMoney(year.TotalRequired))
	fmt.Printf("State Pension: %s\n", FormatMoney(year.TotalStatePension))
	fmt.Printf("Net Required: %s\n", FormatMoney(year.NetRequired))

	fmt.Println("\nWithdrawals:")
	for name, amount := range year.Withdrawals.TaxFreeFromISA {
		if amount > 0 {
			fmt.Printf("  %s ISA: %s\n", name, FormatMoney(amount))
		}
	}
	for name, amount := range year.Withdrawals.TaxFreeFromPension {
		if amount > 0 {
			fmt.Printf("  %s Tax-Free Pension: %s\n", name, FormatMoney(amount))
		}
	}
	for name, amount := range year.Withdrawals.TaxableFromPension {
		if amount > 0 {
			fmt.Printf("  %s Taxable Pension: %s\n", name, FormatMoney(amount))
		}
	}

	fmt.Println("\nTax:")
	for name, tax := range year.TaxByPerson {
		fmt.Printf("  %s: %s\n", name, FormatMoney(tax))
	}
	fmt.Printf("  Total: %s\n", FormatMoney(year.TotalTaxPaid))

	fmt.Println("\nEnd Balances:")
	for name, bal := range year.EndBalances {
		total := bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot
		fmt.Printf("  %s: %s (ISA: %s, Pension: %s)\n",
			name, FormatMoney(total),
			FormatMoney(bal.TaxFreeSavings),
			FormatMoney(bal.CrystallisedPot+bal.UncrystallisedPot))
	}
}

// PrintDrawdownDetails prints detailed drawdown breakdown for a simulation
func PrintDrawdownDetails(result SimulationResult, config *Config) {
	fmt.Println()
	fmt.Printf("╔══════════════════════════════════════════════════════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║ DRAWDOWN DETAILS: %-87s ║\n", result.Params.String())
	fmt.Printf("╚══════════════════════════════════════════════════════════════════════════════════════════════════════════════════╝\n")

	// Get person names
	var names []string
	for _, p := range config.People {
		names = append(names, p.Name)
	}

	// Calculate totals per person
	personTotals := make(map[string]struct {
		statePen   float64
		dbPen      float64
		isa        float64
		penTaxFree float64
		penTaxable float64
		tax        float64
	})

	for _, name := range names {
		personTotals[name] = struct {
			statePen   float64
			dbPen      float64
			isa        float64
			penTaxFree float64
			penTaxable float64
			tax        float64
		}{}
	}

	for _, year := range result.Years {
		for _, name := range names {
			t := personTotals[name]
			t.statePen += year.StatePensionByPerson[name]
			t.dbPen += year.DBPensionByPerson[name]
			t.isa += year.Withdrawals.TaxFreeFromISA[name]
			t.penTaxFree += year.Withdrawals.TaxFreeFromPension[name]
			t.penTaxable += year.Withdrawals.TaxableFromPension[name]
			t.tax += year.TaxByPerson[name]
			personTotals[name] = t
		}
	}

	// Summary section first
	fmt.Println()
	fmt.Println("LIFETIME WITHDRAWAL SUMMARY BY PERSON:")
	fmt.Println(strings.Repeat("═", 80))

	var grandTotalISA, grandTotalPenTaxFree, grandTotalPenTaxable, grandTotalStatePen, grandTotalDBPen, grandTotalTax float64

	for _, name := range names {
		t := personTotals[name]
		totalFromPerson := t.isa + t.penTaxFree + t.penTaxable
		fmt.Printf("\n  %s:\n", name)
		fmt.Printf("    ┌─────────────────────────────────────────────────────────────┐\n")
		fmt.Printf("    │ INCOME SOURCES:                                             │\n")
		fmt.Printf("    │   State Pension:           %12s  (taxable income)    │\n", FormatMoney(t.statePen))
		if t.dbPen > 0 {
			fmt.Printf("    │   DB Pension:              %12s  (taxable income)    │\n", FormatMoney(t.dbPen))
		}
		fmt.Printf("    ├─────────────────────────────────────────────────────────────┤\n")
		fmt.Printf("    │ WITHDRAWALS:                                                │\n")
		fmt.Printf("    │   From ISA:                %12s  ← tax-free           │\n", FormatMoney(t.isa))
		fmt.Printf("    │   From Pension (25%% TF):   %12s  ← tax-free (cryst)  │\n", FormatMoney(t.penTaxFree))
		fmt.Printf("    │   From Pension (taxable):  %12s  ← income tax        │\n", FormatMoney(t.penTaxable))
		fmt.Printf("    ├─────────────────────────────────────────────────────────────┤\n")
		fmt.Printf("    │ TOTALS:                                                     │\n")
		fmt.Printf("    │   Total Withdrawn:         %12s                      │\n", FormatMoney(totalFromPerson))
		fmt.Printf("    │   Tax Paid:                %12s                      │\n", FormatMoney(t.tax))
		taxableIncome := t.statePen + t.dbPen + t.penTaxable
		effectiveRate := 0.0
		if taxableIncome > 0 {
			effectiveRate = (t.tax / taxableIncome) * 100
		}
		fmt.Printf("    │   Effective Tax Rate:      %11.1f%%                      │\n", effectiveRate)
		fmt.Printf("    └─────────────────────────────────────────────────────────────┘\n")

		grandTotalISA += t.isa
		grandTotalPenTaxFree += t.penTaxFree
		grandTotalPenTaxable += t.penTaxable
		grandTotalStatePen += t.statePen
		grandTotalDBPen += t.dbPen
		grandTotalTax += t.tax
	}

	fmt.Println()
	fmt.Println(strings.Repeat("═", 80))
	fmt.Println("  COMBINED TOTALS:")
	fmt.Printf("    Total State Pension:     %12s\n", FormatMoney(grandTotalStatePen))
	if grandTotalDBPen > 0 {
		fmt.Printf("    Total DB Pension:        %12s\n", FormatMoney(grandTotalDBPen))
	}
	fmt.Printf("    Total ISA Withdrawals:   %12s  (tax-free)\n", FormatMoney(grandTotalISA))
	fmt.Printf("    Total Pension (TaxFree): %12s  (25%% crystallisation)\n", FormatMoney(grandTotalPenTaxFree))
	fmt.Printf("    Total Pension (Taxable): %12s  (income tax paid)\n", FormatMoney(grandTotalPenTaxable))
	fmt.Printf("    ════════════════════════════════════\n")
	fmt.Printf("    TOTAL WITHDRAWN:         %12s\n", FormatMoney(grandTotalISA+grandTotalPenTaxFree+grandTotalPenTaxable))
	fmt.Printf("    TOTAL TAX PAID:          %12s\n", FormatMoney(grandTotalTax))
	taxableTotal := grandTotalStatePen + grandTotalDBPen + grandTotalPenTaxable
	if taxableTotal > 0 {
		fmt.Printf("    Overall Effective Rate:  %11.1f%%\n", (grandTotalTax/taxableTotal)*100)
	}

	// Detailed year-by-year breakdown
	fmt.Println()
	fmt.Println()
	fmt.Println("YEAR-BY-YEAR EXTRACTION DETAILS:")
	fmt.Println(strings.Repeat("═", 120))

	// Print each year with full details
	for i, year := range result.Years {
		isKeyYear := i == 0 || i == len(result.Years)-1 || year.Year%5 == 0 ||
			year.Year == config.Mortgage.EndYear ||
			year.Year == config.Mortgage.EndYear-1

		if isKeyYear {
			fmt.Printf("\n┌─ YEAR %d ", year.Year)
			fmt.Print(strings.Repeat("─", 108))
			fmt.Println("┐")

			// Ages
			fmt.Printf("│ Ages: ")
			for _, name := range names {
				fmt.Printf("%s=%d  ", name, year.Ages[name])
			}
			fmt.Println()

			// Income requirement
			fmt.Printf("│ Required: %s", FormatMoney(year.TotalRequired))
			if year.MortgageCost > 0 {
				fmt.Printf(" (includes %s mortgage)", FormatMoney(year.MortgageCost))
			}
			fmt.Println()
			if year.TotalDBPension > 0 {
				fmt.Printf("│ State Pension: %s | DB Pension: %s  →  Net needed from savings: %s\n",
					FormatMoney(year.TotalStatePension), FormatMoney(year.TotalDBPension), FormatMoney(year.NetRequired))
			} else {
				fmt.Printf("│ State Pension: %s  →  Net needed from savings: %s\n",
					FormatMoney(year.TotalStatePension), FormatMoney(year.NetRequired))
			}

			fmt.Println("│")
			fmt.Println("│ EXTRACTIONS BY PERSON:")

			for _, name := range names {
				statePen := year.StatePensionByPerson[name]
				dbPen := year.DBPensionByPerson[name]
				isaWithdraw := year.Withdrawals.TaxFreeFromISA[name]
				penTaxFree := year.Withdrawals.TaxFreeFromPension[name]
				penTaxable := year.Withdrawals.TaxableFromPension[name]
				tax := year.TaxByPerson[name]
				bal := year.EndBalances[name]

				hasActivity := statePen > 0 || dbPen > 0 || isaWithdraw > 0 || penTaxFree > 0 || penTaxable > 0

				if hasActivity {
					fmt.Printf("│   %s:\n", name)

					if statePen > 0 {
						fmt.Printf("│     State Pension:    %10s  (taxable)\n", FormatMoney(statePen))
					}

					if dbPen > 0 {
						fmt.Printf("│     DB Pension:       %10s  (taxable)\n", FormatMoney(dbPen))
					}

					if isaWithdraw > 0 {
						fmt.Printf("│     Extract from ISA: %10s  → remaining ISA: %s\n",
							FormatMoney(isaWithdraw), FormatMoney(bal.TaxFreeSavings))
					}

					if penTaxFree > 0 {
						// This means crystallisation happened
						crystallised := penTaxFree * 4 // 25% tax-free means we crystallised 4x this amount
						fmt.Printf("│     Crystallise:      %10s  (25%% = %s tax-free, 75%% = %s taxable)\n",
							FormatMoney(crystallised), FormatMoney(penTaxFree), FormatMoney(crystallised*0.75))
						fmt.Printf("│       → remaining uncrystallised: %s\n", FormatMoney(bal.UncrystallisedPot))
					}

					if penTaxable > 0 {
						fmt.Printf("│     Extract taxable:  %10s  → remaining crystallised: %s\n",
							FormatMoney(penTaxable), FormatMoney(bal.CrystallisedPot))
					}

					if tax > 0 {
						fmt.Printf("│     Tax on £%s income: %s\n",
							FormatMoney(statePen+dbPen+penTaxable), FormatMoney(tax))
					}
				}
			}

			fmt.Println("│")
			fmt.Printf("│ NET INCOME RECEIVED: %s", FormatMoney(year.NetIncomeReceived))
			if year.NetIncomeReceived < year.TotalRequired-1 {
				shortfall := year.TotalRequired - year.NetIncomeReceived
				fmt.Printf("  ⚠️  SHORTFALL: %s", FormatMoney(shortfall))
			}
			fmt.Println()

			// End balances
			fmt.Println("│")
			fmt.Println("│ END OF YEAR BALANCES:")
			for _, name := range names {
				bal := year.EndBalances[name]
				total := bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot
				fmt.Printf("│   %s: ISA %s | Pension (cryst) %s | Pension (uncryst) %s | TOTAL: %s\n",
					name,
					FormatMoney(bal.TaxFreeSavings),
					FormatMoney(bal.CrystallisedPot),
					FormatMoney(bal.UncrystallisedPot),
					FormatMoney(total))
			}

			fmt.Printf("└")
			fmt.Print(strings.Repeat("─", 118))
			fmt.Println("┘")
		}
	}
}

// PrintDepletionHeader prints the header for depletion mode
func PrintDepletionHeader(config *Config) {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           PENSION DRAWDOWN - DEPLETION MODE CALCULATION                      ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	ic := config.IncomeRequirements
	refPerson := config.GetSimulationReferencePerson()

	fmt.Printf("Target: Deplete funds by age %d (%s)\n", ic.TargetDepletionAge, refPerson.Name)
	fmt.Printf("Income Ratio: %.0f:%.0f (before/after age %d)\n",
		ic.IncomeRatioPhase1, ic.IncomeRatioPhase2, ic.AgeThreshold)
	fmt.Println()

	fmt.Println("Configuration:")
	fmt.Println("──────────────")
	for _, p := range config.People {
		birthYear := GetBirthYear(p.BirthDate)
		fmt.Printf("  %s: Born %d, State Pension at %d\n",
			p.Name, birthYear, p.StatePensionAge)
		fmt.Printf("          ISA: %s, Pension: %s\n",
			FormatMoney(p.TaxFreeSavings), FormatMoney(p.Pension))
		if p.DBPensionAmount > 0 {
			fmt.Printf("          %s: %s/year from age %d\n",
				p.DBPensionName, FormatMoney(p.DBPensionAmount), p.DBPensionStartAge)
		}
	}
	fmt.Println()
	fmt.Printf("  Pension Growth: %.0f%% | Savings Growth: %.0f%% | Inflation: %.0f%%\n",
		config.Financial.PensionGrowthRate*100,
		config.Financial.SavingsGrowthRate*100,
		config.Financial.IncomeInflationRate*100)
	fmt.Println()
}

// PrintDepletionComparison prints comparison of depletion mode results
func PrintDepletionComparison(results []DepletionResult, config *Config) {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                               SUSTAINABLE INCOME COMPARISON                                        ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Header
	fmt.Printf("%-25s │ %15s │ %15s │ %12s │ %12s\n",
		"Strategy", "Monthly (Before)", "Monthly (After)", "Annual", "Total Tax")
	fmt.Println(strings.Repeat("─", 95))

	// Find best for highlighting
	bestIdx := FindBestDepletionStrategy(results)

	for i, r := range results {
		marker := "  "
		if i == bestIdx {
			marker = "* "
		}
		annualBefore := r.MonthlyBeforeAge * 12
		fmt.Printf("%s%-23s │ %15s │ %15s │ %12s │ %12s\n",
			marker,
			r.Params.ShortName(),
			FormatMoney(r.MonthlyBeforeAge),
			FormatMoney(r.MonthlyAfterAge),
			FormatMoney(annualBefore),
			FormatMoney(r.SimulationResult.TotalTaxPaid))
	}

	fmt.Println()
	fmt.Println("* = Best strategy (highest sustainable income)")
	fmt.Println()

	// Print recommendation
	if bestIdx >= 0 {
		best := results[bestIdx]
		fmt.Println("╔════════════════════════════════════════════════════════════════════════════════════════════════════╗")
		fmt.Println("║                                     RECOMMENDATION                                                  ║")
		fmt.Println("╚════════════════════════════════════════════════════════════════════════════════════════════════════╝")
		fmt.Println()
		fmt.Printf("  RECOMMENDED: %s\n", best.Params.String())
		fmt.Printf("  Sustainable Income: %s/month (before age %d) / %s/month (after)\n",
			FormatMoney(best.MonthlyBeforeAge),
			config.IncomeRequirements.AgeThreshold,
			FormatMoney(best.MonthlyAfterAge))
		fmt.Printf("  Annual Income: %s (phase 1) / %s (phase 2)\n",
			FormatMoney(best.MonthlyBeforeAge*12),
			FormatMoney(best.MonthlyAfterAge*12))
		fmt.Printf("  Total Lifetime Tax: %s\n", FormatMoney(best.SimulationResult.TotalTaxPaid))
		fmt.Printf("  Balance at target age: %s\n", FormatMoney(best.ConvergenceError))
		fmt.Println()
	}
}

// PrintPensionOnlyDepletionComparison prints comparison of pension-only depletion results
func PrintPensionOnlyDepletionComparison(results []DepletionResult, config *Config) {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                          PENSION-ONLY SUSTAINABLE INCOME COMPARISON                                ║")
	fmt.Println("║                              (ISAs are preserved, not touched)                                     ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Header
	fmt.Printf("%-25s │ %15s │ %15s │ %12s │ %12s │ %12s\n",
		"Strategy", "Monthly (Before)", "Monthly (After)", "Annual", "Total Tax", "Final ISA")
	fmt.Println(strings.Repeat("─", 110))

	// Find best for highlighting
	bestIdx := FindBestDepletionStrategy(results)

	for i, r := range results {
		marker := "  "
		if i == bestIdx {
			marker = "* "
		}
		annualBefore := r.MonthlyBeforeAge * 12

		// Calculate final ISA balance
		finalISA := 0.0
		if len(r.SimulationResult.Years) > 0 {
			lastYear := r.SimulationResult.Years[len(r.SimulationResult.Years)-1]
			for _, bal := range lastYear.EndBalances {
				finalISA += bal.TaxFreeSavings
			}
		}

		fmt.Printf("%s%-23s │ %15s │ %15s │ %12s │ %12s │ %12s\n",
			marker,
			r.Params.ShortName(),
			FormatMoney(r.MonthlyBeforeAge),
			FormatMoney(r.MonthlyAfterAge),
			FormatMoney(annualBefore),
			FormatMoney(r.SimulationResult.TotalTaxPaid),
			FormatMoney(finalISA))
	}

	fmt.Println()
	fmt.Println("* = Best strategy (highest sustainable income from pensions only)")
	fmt.Println()

	// Print recommendation
	if bestIdx >= 0 {
		best := results[bestIdx]

		// Calculate final ISA balance
		finalISA := 0.0
		if len(best.SimulationResult.Years) > 0 {
			lastYear := best.SimulationResult.Years[len(best.SimulationResult.Years)-1]
			for _, bal := range lastYear.EndBalances {
				finalISA += bal.TaxFreeSavings
			}
		}

		fmt.Println("╔════════════════════════════════════════════════════════════════════════════════════════════════════╗")
		fmt.Println("║                                     RECOMMENDATION                                                  ║")
		fmt.Println("╚════════════════════════════════════════════════════════════════════════════════════════════════════╝")
		fmt.Println()
		fmt.Printf("  RECOMMENDED: %s\n", best.Params.String())
		fmt.Printf("  Sustainable Income (from pensions): %s/month (before age %d) / %s/month (after)\n",
			FormatMoney(best.MonthlyBeforeAge),
			config.IncomeRequirements.AgeThreshold,
			FormatMoney(best.MonthlyAfterAge))
		fmt.Printf("  Annual Income: %s (phase 1) / %s (phase 2)\n",
			FormatMoney(best.MonthlyBeforeAge*12),
			FormatMoney(best.MonthlyAfterAge*12))
		fmt.Printf("  Total Lifetime Tax: %s\n", FormatMoney(best.SimulationResult.TotalTaxPaid))
		fmt.Printf("  Pension balance at target age: %s\n", FormatMoney(best.ConvergenceError))
		fmt.Printf("  ISA balance preserved: %s (continues to grow)\n", FormatMoney(finalISA))
		fmt.Println()
		fmt.Println("  Note: ISAs remain untouched and available for later years or inheritance.")
		fmt.Println()
	}
}

// PrintPensionToISADepletionComparison prints comparison of PensionToISA depletion results
func PrintPensionToISADepletionComparison(results []DepletionResult, config *Config) {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                          PENSION-TO-ISA SUSTAINABLE INCOME COMPARISON                              ║")
	fmt.Println("║                          (Efficiently moves excess pension to ISAs)                                ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Header
	fmt.Printf("%-25s │ %15s │ %15s │ %12s │ %12s │ %12s\n",
		"Strategy", "Monthly (Before)", "Monthly (After)", "Annual", "Total Tax", "Final ISA")
	fmt.Println(strings.Repeat("─", 110))

	// Find best for highlighting
	bestIdx := FindBestDepletionStrategy(results)

	for i, r := range results {
		marker := "  "
		if i == bestIdx {
			marker = "* "
		}
		annualBefore := r.MonthlyBeforeAge * 12

		// Calculate final ISA balance
		finalISA := 0.0
		if len(r.SimulationResult.Years) > 0 {
			lastYear := r.SimulationResult.Years[len(r.SimulationResult.Years)-1]
			for _, bal := range lastYear.EndBalances {
				finalISA += bal.TaxFreeSavings
			}
		}

		fmt.Printf("%s%-23s │ %15s │ %15s │ %12s │ %12s │ %12s\n",
			marker,
			r.Params.ShortName(),
			FormatMoney(r.MonthlyBeforeAge),
			FormatMoney(r.MonthlyAfterAge),
			FormatMoney(annualBefore),
			FormatMoney(r.SimulationResult.TotalTaxPaid),
			FormatMoney(finalISA))
	}

	fmt.Println()
	fmt.Println("* = Best strategy (highest sustainable income with tax-efficient ISA transfers)")
	fmt.Println()

	// Print recommendation
	if bestIdx >= 0 {
		best := results[bestIdx]

		// Calculate final ISA balance
		finalISA := 0.0
		if len(best.SimulationResult.Years) > 0 {
			lastYear := best.SimulationResult.Years[len(best.SimulationResult.Years)-1]
			for _, bal := range lastYear.EndBalances {
				finalISA += bal.TaxFreeSavings
			}
		}

		fmt.Println("╔════════════════════════════════════════════════════════════════════════════════════════════════════╗")
		fmt.Println("║                                     RECOMMENDATION                                                  ║")
		fmt.Println("╚════════════════════════════════════════════════════════════════════════════════════════════════════╝")
		fmt.Println()
		fmt.Printf("  RECOMMENDED: %s\n", best.Params.String())
		fmt.Printf("  Sustainable Income: %s/month (before age %d) / %s/month (after)\n",
			FormatMoney(best.MonthlyBeforeAge),
			config.IncomeRequirements.AgeThreshold,
			FormatMoney(best.MonthlyAfterAge))
		fmt.Printf("  Annual Income: %s (phase 1) / %s (phase 2)\n",
			FormatMoney(best.MonthlyBeforeAge*12),
			FormatMoney(best.MonthlyAfterAge*12))
		fmt.Printf("  Total Lifetime Tax: %s\n", FormatMoney(best.SimulationResult.TotalTaxPaid))
		fmt.Printf("  Final Balance at target age: %s\n", FormatMoney(best.ConvergenceError))
		fmt.Printf("  Final ISA Balance: %s (includes efficient pension transfers)\n", FormatMoney(finalISA))
		fmt.Println()
		fmt.Println("  Strategy: Overdraws pension to fill tax bands (personal allowance + basic rate)")
		fmt.Println("  Any excess after spending needs is deposited into ISA for tax-free growth.")
		fmt.Println()
	}
}
