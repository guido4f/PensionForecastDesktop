package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GenerateHTMLReport generates an HTML detailed report for a simulation result
func GenerateHTMLReport(result SimulationResult, config *Config, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// Get person names
	var names []string
	for _, p := range config.People {
		names = append(names, p.Name)
	}

	// Write HTML header
	fmt.Fprintf(f, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Pension Forecast: %s</title>
    <style>
        :root {
            --primary: #2563eb;
            --success: #16a34a;
            --warning: #ea580c;
            --danger: #dc2626;
            --bg: #f8fafc;
            --card-bg: #ffffff;
            --text: #1e293b;
            --text-muted: #64748b;
            --border: #e2e8f0;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.6;
            padding: 2rem;
        }
        .container { max-width: 1400px; margin: 0 auto; }
        h1 {
            font-size: 1.75rem;
            margin-bottom: 0.5rem;
            color: var(--primary);
        }
        h2 {
            font-size: 1.25rem;
            margin: 1.5rem 0 1rem;
            padding-bottom: 0.5rem;
            border-bottom: 2px solid var(--primary);
        }
        h3 { font-size: 1rem; margin-bottom: 0.5rem; }
        .subtitle {
            color: var(--text-muted);
            margin-bottom: 1.5rem;
        }
        .card {
            background: var(--card-bg);
            border-radius: 8px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
            padding: 1.5rem;
            margin-bottom: 1.5rem;
        }
        .grid { display: grid; gap: 1rem; }
        .grid-2 { grid-template-columns: repeat(2, 1fr); }
        .grid-4 { grid-template-columns: repeat(4, 1fr); }
        @media (max-width: 768px) {
            .grid-2, .grid-4 { grid-template-columns: 1fr; }
        }
        .metric {
            text-align: center;
            padding: 1rem;
            border-radius: 8px;
            background: var(--bg);
        }
        .metric-value {
            font-size: 1.5rem;
            font-weight: 700;
            color: var(--primary);
        }
        .metric-label {
            font-size: 0.875rem;
            color: var(--text-muted);
        }
        .metric.success .metric-value { color: var(--success); }
        .metric.warning .metric-value { color: var(--warning); }
        .metric.danger .metric-value { color: var(--danger); }
        table {
            width: 100%%;
            border-collapse: collapse;
            font-size: 0.875rem;
        }
        th, td {
            padding: 0.75rem 0.5rem;
            text-align: right;
            border-bottom: 1px solid var(--border);
        }
        th {
            background: var(--bg);
            font-weight: 600;
            text-align: right;
            position: sticky;
            top: 0;
        }
        th:first-child, td:first-child { text-align: left; }
        tr:hover { background: #f1f5f9; }
        .highlight { background: #fef3c7 !important; }
        .negative { color: var(--danger); }
        .positive { color: var(--success); }
        .balance-row {
            background: var(--bg);
            font-weight: 600;
        }
        .footer {
            text-align: center;
            color: var(--text-muted);
            font-size: 0.75rem;
            margin-top: 2rem;
            padding-top: 1rem;
            border-top: 1px solid var(--border);
        }
        .badge {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            border-radius: 9999px;
            font-size: 0.75rem;
            font-weight: 600;
        }
        .badge-success { background: #dcfce7; color: var(--success); }
        .badge-danger { background: #fee2e2; color: var(--danger); }
        /* Accordion styles for expandable rows */
        .expandable-row { cursor: pointer; }
        .expandable-row:hover { background: #e0e7ff !important; }
        .expandable-row.highlight { background: #fef3c7; }
        .expandable-row.highlight:hover { background: #fde68a !important; }
        .expandable-row td:first-child::before {
            content: '▶';
            display: inline-block;
            margin-right: 0.5rem;
            font-size: 0.75rem;
            transition: transform 0.2s;
        }
        .expandable-row.expanded td:first-child::before {
            transform: rotate(90deg);
        }
        .year-details {
            display: none;
            background: #f8fafc;
        }
        .year-details.show { display: table-row; }
        .year-details td {
            padding: 1rem;
            border-bottom: 2px solid var(--primary);
        }
        .detail-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 1rem;
        }
        .detail-box {
            background: white;
            padding: 0.75rem;
            border-radius: 4px;
            border: 1px solid var(--border);
        }
        .detail-box-header {
            font-size: 0.75rem;
            color: var(--text-muted);
            margin-bottom: 0.25rem;
        }
        .detail-box-value {
            font-weight: 600;
            font-size: 1rem;
        }
        .detail-table {
            width: 100%%;
            margin-top: 0.5rem;
            font-size: 0.8rem;
        }
        .detail-table th {
            background: #e0e7ff;
            padding: 0.5rem;
        }
        .detail-table td {
            padding: 0.5rem;
            border-bottom: 1px solid var(--border);
        }
    </style>
    <script>
        function toggleYear(year) {
            const row = document.getElementById('row-' + year);
            const details = document.getElementById('details-' + year);
            if (row && details) {
                row.classList.toggle('expanded');
                details.classList.toggle('show');
            }
        }
    </script>
</head>
<body>
    <div class="container">
        <h1>Pension Drawdown Forecast</h1>
        <p class="subtitle">Strategy: %s</p>
`, result.Params.String(), result.Params.String())

	// Summary metrics
	var totalRemaining float64
	for _, b := range result.FinalBalances {
		totalRemaining += b.TaxFreeSavings + b.UncrystallisedPot + b.CrystallisedPot
	}

	ranOutClass := "success"
	ranOutText := "No"
	if result.RanOutOfMoney {
		ranOutClass = "danger"
		ranOutText = fmt.Sprintf("Yes (%d)", result.RanOutYear)
	}

	fmt.Fprintf(f, `
        <div class="card">
            <h2>Summary</h2>
            <div class="grid grid-4">
                <div class="metric">
                    <div class="metric-value">%s</div>
                    <div class="metric-label">Total Tax Paid</div>
                </div>
                <div class="metric">
                    <div class="metric-value">%s</div>
                    <div class="metric-label">Total Withdrawn</div>
                </div>
                <div class="metric success">
                    <div class="metric-value">%s</div>
                    <div class="metric-label">Savings Remaining</div>
                </div>
                <div class="metric %s">
                    <div class="metric-value">%s</div>
                    <div class="metric-label">Ran Out of Money</div>
                </div>
            </div>
        </div>
`, FormatMoney(result.TotalTaxPaid), FormatMoney(result.TotalWithdrawn),
		FormatMoney(totalRemaining), ranOutClass, ranOutText)

	// Strategy description
	writeStrategyDescription(f, result.Params)

	// Configuration summary
	fmt.Fprintf(f, `
        <div class="card">
            <h2>Configuration</h2>
            <div class="grid grid-2">
                <div>
                    <h3>People</h3>
                    <table>
                        <tr><th>Name</th><th>Birth Year</th><th>Retire Age</th><th>State Pension Age</th><th>ISA</th><th>Pension</th></tr>
`)
	for _, p := range config.People {
		birthYear := GetBirthYear(p.BirthDate)
		fmt.Fprintf(f, "                        <tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%s</td><td>%s</td></tr>\n",
			p.Name, birthYear, p.RetirementAge, p.StatePensionAge,
			FormatMoney(p.TaxFreeSavings), FormatMoney(p.Pension))
	}
	fmt.Fprintf(f, `                    </table>
                </div>
                <div>
                    <h3>Parameters</h3>
                    <table>
                        <tr><td>Pension Growth Rate</td><td>%.0f%%</td></tr>
                        <tr><td>Savings Growth Rate</td><td>%.0f%%</td></tr>
                        <tr><td>Income Inflation</td><td>%.0f%%</td></tr>
                        <tr><td>Monthly Income (before %d)</td><td>%s [after tax]</td></tr>
                        <tr><td>Monthly Income (after %d)</td><td>%s [after tax]</td></tr>
                        <tr><td>Mortgage (until %d)</td><td>%s/year</td></tr>
                        <tr><td>Simulation Period</td><td>%d to age %d</td></tr>
                    </table>
                </div>
            </div>
        </div>
`, config.Financial.PensionGrowthRate*100, config.Financial.SavingsGrowthRate*100, config.Financial.IncomeInflationRate*100,
		config.IncomeRequirements.AgeThreshold,
		FormatMoney(config.IncomeRequirements.MonthlyBeforeAge),
		config.IncomeRequirements.AgeThreshold,
		FormatMoney(config.IncomeRequirements.MonthlyAfterAge),
		config.Mortgage.EndYear, FormatMoney(config.GetTotalAnnualPayment()),
		config.Simulation.StartYear, config.Simulation.EndAge)

	// Mortgage details (if any mortgage parts exist)
	if len(config.Mortgage.Parts) > 0 {
		fmt.Fprintf(f, `
        <div class="card">
            <h2>Mortgage Details</h2>
            <table>
                <tr><th>Name</th><th>Balance</th><th>Interest Rate</th><th>Type</th><th>Annual Payment</th><th>Term Ends</th></tr>
`)
		for _, part := range config.Mortgage.Parts {
			mortgageType := "Repayment"
			if !part.IsRepayment {
				mortgageType = "Interest Only"
			}
			endYear := part.StartYear + part.TermYears
			annualPayment := part.CalculateAnnualPayment()
			fmt.Fprintf(f, "                <tr><td>%s</td><td>%s</td><td>%.1f%%</td><td>%s</td><td>%s</td><td>%d</td></tr>\n",
				part.Name, FormatMoney(part.Principal), part.InterestRate*100, mortgageType, FormatMoney(annualPayment), endYear)
		}
		fmt.Fprintf(f, `            </table>
            <p style="margin-top: 0.5rem; color: var(--text-muted);">
                <strong>Total Annual Payment:</strong> %s |
                <strong>Normal End Year:</strong> %d |
                <strong>Early Payoff Year:</strong> %d
            </p>
        </div>
`, FormatMoney(config.GetTotalAnnualPayment()), config.Mortgage.EndYear, config.Mortgage.EarlyPayoffYear)
	}

	// Final balances breakdown
	fmt.Fprintf(f, `
        <div class="card">
            <h2>Final Balances (Age %d)</h2>
            <table>
                <tr><th>Person</th><th>Tax-Free (ISA+PCLS)</th><th>Crystallised Pension</th><th>Uncrystallised Pension</th><th>Total</th></tr>
`, config.Simulation.EndAge)
	for name, bal := range result.FinalBalances {
		total := bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot
		fmt.Fprintf(f, "                <tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td><strong>%s</strong></td></tr>\n",
			name, FormatMoney(bal.TaxFreeSavings), FormatMoney(bal.CrystallisedPot),
			FormatMoney(bal.UncrystallisedPot), FormatMoney(total))
	}
	fmt.Fprintf(f, "                <tr class=\"balance-row\"><td><strong>Total</strong></td><td></td><td></td><td></td><td><strong>%s</strong></td></tr>\n",
		FormatMoney(totalRemaining))
	fmt.Fprintf(f, `            </table>
        </div>
`)

	// Year-by-year table
	fmt.Fprintf(f, `
        <div class="card">
            <h2>Year-by-Year Breakdown</h2>
            <div style="overflow-x: auto;">
                <table>
                    <tr>
                        <th>Year</th>
`)
	for _, name := range names {
		fmt.Fprintf(f, "                        <th>%s Age</th>\n", name)
	}
	fmt.Fprintf(f, `                        <th>Required</th>
                        <th>State Pen</th>
                        <th>DB Pen</th>
                        <th>Tax-Free</th>
                        <th>Taxable</th>
                        <th>Tax Paid</th>
                        <th>Net Required</th>
                        <th>Net Received</th>
`)
	for _, name := range names {
		fmt.Fprintf(f, "                        <th>%s ISA</th>\n", name)
		fmt.Fprintf(f, "                        <th>%s Pen</th>\n", name)
	}
	fmt.Fprintf(f, `                    </tr>
`)

	// Determine which year to highlight (mortgage payoff year)
	mortgagePayoffYear := config.Mortgage.EndYear
	if result.Params.EarlyMortgagePayoff {
		mortgagePayoffYear = config.Mortgage.EarlyPayoffYear
	}

	// Calculate colspan for details row (Year + ages + 8 columns + 2 per person for balances)
	colspan := 1 + len(names) + 8 + len(names)*2

	for _, year := range result.Years {
		highlightClass := ""
		if year.Year == mortgagePayoffYear {
			highlightClass = " highlight"
		}

		// Main row (clickable)
		fmt.Fprintf(f, `                    <tr id="row-%d" class="expandable-row%s" onclick="toggleYear(%d)">
`, year.Year, highlightClass, year.Year)
		fmt.Fprintf(f, "                        <td>%d</td>\n", year.Year)
		for _, name := range names {
			fmt.Fprintf(f, "                        <td>%d</td>\n", year.Ages[name])
		}
		fmt.Fprintf(f, "                        <td>%s</td>\n", FormatMoney(year.TotalRequired))
		fmt.Fprintf(f, "                        <td>%s</td>\n", FormatMoney(year.TotalStatePension))
		fmt.Fprintf(f, "                        <td>%s</td>\n", formatOrDash(year.TotalDBPension))
		fmt.Fprintf(f, "                        <td>%s</td>\n", FormatMoney(year.Withdrawals.TotalTaxFree))
		fmt.Fprintf(f, "                        <td>%s</td>\n", FormatMoney(year.Withdrawals.TotalTaxable))
		fmt.Fprintf(f, "                        <td class=\"negative\">%s</td>\n", FormatMoney(year.TotalTaxPaid))
		fmt.Fprintf(f, "                        <td>%s</td>\n", FormatMoney(year.NetRequired))
		fmt.Fprintf(f, "                        <td class=\"positive\">%s</td>\n", FormatMoney(year.NetIncomeReceived))
		for _, name := range names {
			bal := year.EndBalances[name]
			fmt.Fprintf(f, "                        <td>%s</td>\n", FormatMoney(bal.TaxFreeSavings))
			fmt.Fprintf(f, "                        <td>%s</td>\n", FormatMoney(bal.CrystallisedPot+bal.UncrystallisedPot))
		}
		fmt.Fprintf(f, "                    </tr>\n")

		// Details row (hidden by default)
		fmt.Fprintf(f, `                    <tr id="details-%d" class="year-details">
                        <td colspan="%d">
`, year.Year, colspan)

		// Write the expandable details content
		writeYearDetailsContent(f, year, names)

		fmt.Fprintf(f, `                        </td>
                    </tr>
`)
	}

	fmt.Fprintf(f, `                </table>
            </div>
        </div>
`)

	// Lifetime withdrawal summary
	writeDrawdownSummaryHTML(f, result, names)

	// Detailed year-by-year extraction
	writeDrawdownDetailsHTML(f, result, config, names)

	// Footer
	fmt.Fprintf(f, `
        <div class="footer">
            Generated on %s | Pension Drawdown Tax Optimisation Simulation
        </div>
    </div>
</body>
</html>
`, time.Now().Format("2006-01-02 15:04:05"))

	return nil
}

// GenerateAllHTMLReports generates HTML reports for all strategies
// Returns the combined report filename and any error
// GenerateAllHTMLReports generates all HTML reports in the current directory
func GenerateAllHTMLReports(results []SimulationResult, config *Config) (string, error) {
	timestamp := time.Now().Format("2006-01-02_1504")
	return GenerateAllHTMLReportsInDir(results, config, ".", timestamp)
}

// GenerateAllHTMLReportsInDir generates all HTML reports in a specified directory
func GenerateAllHTMLReportsInDir(results []SimulationResult, config *Config, outputDir string, timestamp string) (string, error) {
	// Create output directory if it doesn't exist
	if outputDir != "." {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Generate individual reports
	for _, result := range results {
		filename := fmt.Sprintf("report_%s.html", result.Params.ShortName())
		filename = sanitizeFilename(filename)
		fullPath := filepath.Join(outputDir, filename)

		err := GenerateHTMLReport(result, config, fullPath)
		if err != nil {
			return "", fmt.Errorf("failed to generate report for %s: %w", result.Params.String(), err)
		}
	}

	// Generate combined tabbed report
	combinedFilename := "summary.html"
	combinedPath := filepath.Join(outputDir, combinedFilename)
	err := GenerateCombinedHTMLReport(results, config, combinedPath)
	if err != nil {
		return "", fmt.Errorf("failed to generate combined report: %w", err)
	}

	return combinedPath, nil
}

// GenerateCombinedHTMLReport generates a single HTML file with tabs for each strategy
func GenerateCombinedHTMLReport(results []SimulationResult, config *Config, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// Get person names
	var names []string
	for _, p := range config.People {
		names = append(names, p.Name)
	}

	// Find best strategy (highest final balance among those that don't run out)
	// This properly accounts for investment returns vs mortgage costs
	var bestIdx int = -1
	var bestBalance float64 = -1
	for i, r := range results {
		if !r.RanOutOfMoney {
			finalBalance := getTotalFinalBalance(r)
			if bestBalance < 0 || finalBalance > bestBalance {
				bestBalance = finalBalance
				bestIdx = i
			}
		}
	}

	// If all run out, find the one that lasts longest
	if bestIdx < 0 {
		longestYear := 0
		for i, r := range results {
			if r.RanOutYear > longestYear {
				longestYear = r.RanOutYear
				bestIdx = i
			}
		}
	}

	// Write HTML header with tabs
	fmt.Fprintf(f, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Pension Forecast Report - %s</title>
    <style>
        :root {
            --primary: #2563eb;
            --success: #16a34a;
            --warning: #ea580c;
            --danger: #dc2626;
            --bg: #f8fafc;
            --card-bg: #ffffff;
            --text: #1e293b;
            --text-muted: #64748b;
            --border: #e2e8f0;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.6;
        }
        .header {
            background: linear-gradient(135deg, #1e40af 0%%, #3b82f6 100%%);
            color: white;
            padding: 1.5rem 2rem;
            margin-bottom: 0;
        }
        .header h1 { font-size: 1.5rem; margin-bottom: 0.25rem; }
        .header .subtitle { color: rgba(255,255,255,0.8); font-size: 0.875rem; }
        .tabs {
            display: flex;
            background: #1e3a8a;
            padding: 0 1rem;
            overflow-x: auto;
        }
        .tab {
            padding: 1rem 1.5rem;
            cursor: pointer;
            color: rgba(255,255,255,0.7);
            border: none;
            background: none;
            font-size: 0.875rem;
            font-weight: 500;
            white-space: nowrap;
            transition: all 0.2s;
            border-bottom: 3px solid transparent;
        }
        .tab:hover { color: white; background: rgba(255,255,255,0.1); }
        .tab.active {
            color: white;
            background: rgba(255,255,255,0.1);
            border-bottom-color: #fbbf24;
        }
        .tab.recommended::after {
            content: '★';
            margin-left: 0.5rem;
            color: #fbbf24;
        }
        .container { max-width: 1400px; margin: 0 auto; padding: 2rem; }
        .tab-content { display: none; }
        .tab-content.active { display: block; }
        h2 {
            font-size: 1.25rem;
            margin: 1.5rem 0 1rem;
            padding-bottom: 0.5rem;
            border-bottom: 2px solid var(--primary);
        }
        h3 { font-size: 1rem; margin-bottom: 0.5rem; }
        .card {
            background: var(--card-bg);
            border-radius: 8px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
            padding: 1.5rem;
            margin-bottom: 1.5rem;
        }
        .grid { display: grid; gap: 1rem; }
        .grid-2 { grid-template-columns: repeat(2, 1fr); }
        .grid-4 { grid-template-columns: repeat(4, 1fr); }
        .grid-5 { grid-template-columns: repeat(5, 1fr); }
        @media (max-width: 1024px) { .grid-5 { grid-template-columns: repeat(3, 1fr); } }
        @media (max-width: 768px) { .grid-2, .grid-4, .grid-5 { grid-template-columns: 1fr; } }
        .metric {
            text-align: center;
            padding: 1rem;
            border-radius: 8px;
            background: var(--bg);
        }
        .metric-value {
            font-size: 1.5rem;
            font-weight: 700;
            color: var(--primary);
        }
        .metric-label {
            font-size: 0.875rem;
            color: var(--text-muted);
        }
        .metric.success .metric-value { color: var(--success); }
        .metric.warning .metric-value { color: var(--warning); }
        .metric.danger .metric-value { color: var(--danger); }
        table {
            width: 100%%;
            border-collapse: collapse;
            font-size: 0.875rem;
        }
        th, td {
            padding: 0.75rem 0.5rem;
            text-align: right;
            border-bottom: 1px solid var(--border);
        }
        th {
            background: var(--bg);
            font-weight: 600;
            position: sticky;
            top: 0;
        }
        th:first-child, td:first-child { text-align: left; }
        tr:hover { background: #f1f5f9; }
        .highlight { background: #fef3c7 !important; }
        .negative { color: var(--danger); }
        .positive { color: var(--success); }
        .balance-row { background: var(--bg); font-weight: 600; }
        .comparison-table td { text-align: center; }
        .comparison-table th { text-align: center; }
        .comparison-table td:first-child, .comparison-table th:first-child { text-align: left; }
        .comparison-table th:not(:first-child), .comparison-table td:not(:first-child) {
            cursor: pointer;
            transition: background 0.2s;
        }
        .comparison-table th:not(:first-child):hover { background: var(--primary-dark); }
        .comparison-table td:not(:first-child):hover { background: #e2e8f0 !important; }
        .best { background: #dcfce7 !important; }
        .footer {
            text-align: center;
            color: var(--text-muted);
            font-size: 0.75rem;
            margin-top: 2rem;
            padding-top: 1rem;
            border-top: 1px solid var(--border);
        }
        /* Accordion styles for expandable rows */
        .expandable-row { cursor: pointer; }
        .expandable-row:hover { background: #e0e7ff !important; }
        .expandable-row.highlight { background: #fef3c7; }
        .expandable-row.highlight:hover { background: #fde68a !important; }
        .expandable-row td:first-child::before {
            content: '▶';
            display: inline-block;
            margin-right: 0.5rem;
            font-size: 0.75rem;
            transition: transform 0.2s;
        }
        .expandable-row.expanded td:first-child::before {
            transform: rotate(90deg);
        }
        .year-details {
            display: none;
            background: #f8fafc;
        }
        .year-details.show { display: table-row; }
        .year-details td {
            padding: 1rem;
            border-bottom: 2px solid var(--primary);
        }
        .detail-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 1rem;
        }
        .detail-box {
            background: white;
            border-radius: 8px;
            padding: 0.75rem;
            box-shadow: 0 1px 2px rgba(0,0,0,0.05);
        }
        .detail-box-header {
            font-size: 0.75rem;
            color: var(--text-muted);
            margin-bottom: 0.25rem;
        }
        .detail-box-value {
            font-size: 1rem;
            font-weight: 600;
            color: var(--text);
        }
        .detail-section {
            margin-top: 1rem;
        }
        .detail-section h4 {
            font-size: 0.875rem;
            color: var(--primary);
            margin-bottom: 0.5rem;
        }
        .detail-table {
            width: 100%%;
            font-size: 0.8rem;
        }
        .detail-table th, .detail-table td {
            padding: 0.5rem;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>Pension Drawdown Tax Optimisation Report</h1>
        <p class="subtitle">Generated %s</p>
    </div>
    <div class="tabs">
        <button class="tab active" onclick="showTab('comparison')">Comparison</button>
`, time.Now().Format("2006-01-02"), time.Now().Format("2 January 2006 at 15:04"))

	// Add tabs for each strategy
	for i, result := range results {
		recommended := ""
		if i == bestIdx {
			recommended = " recommended"
		}
		fmt.Fprintf(f, `        <button class="tab%s" onclick="showTab('strategy%d')">%s</button>
`, recommended, i, result.Params.ShortName())
	}

	fmt.Fprintf(f, `    </div>

    <!-- Comparison Tab -->
    <div id="comparison" class="tab-content active">
        <div class="container">
            <div class="card">
                <h2>Strategy Comparison</h2>
                <p style="margin-bottom: 1rem; color: var(--text-secondary); font-size: 0.9rem;">
                    Compares different withdrawal strategies. <strong>PCLS</strong> (Pension Commencement Lump Sum) is the 25%% tax-free amount
                    you can take when crystallising your pension. Click any strategy column to view details.
                </p>
                <table class="comparison-table">
                    <tr>
                        <th>Metric</th>
`)
	for i, r := range results {
		fmt.Fprintf(f, "                        <th onclick=\"showTab('strategy%d')\" title=\"Click to view %s details\">%s</th>\n", i, r.Params.ShortName(), r.Params.ShortName())
	}
	fmt.Fprintf(f, `                    </tr>
                    <tr>
                        <td>Total Tax Paid</td>
`)
	for i, r := range results {
		class := ""
		if i == bestIdx {
			class = " best"
		}
		fmt.Fprintf(f, "                        <td class=\"%s\" onclick=\"showTab('strategy%d')\">%s</td>\n", class, i, FormatMoney(r.TotalTaxPaid))
	}
	fmt.Fprintf(f, `                    </tr>
                    <tr>
                        <td>Total Withdrawn</td>
`)
	for i, r := range results {
		fmt.Fprintf(f, "                        <td onclick=\"showTab('strategy%d')\">%s</td>\n", i, FormatMoney(r.TotalWithdrawn))
	}
	fmt.Fprintf(f, `                    </tr>
                    <tr>
                        <td>Savings Remaining</td>
`)
	for i, r := range results {
		var total float64
		for _, b := range r.FinalBalances {
			total += b.TaxFreeSavings + b.UncrystallisedPot + b.CrystallisedPot
		}
		fmt.Fprintf(f, "                        <td onclick=\"showTab('strategy%d')\">%s</td>\n", i, FormatMoney(total))
	}
	fmt.Fprintf(f, `                    </tr>
                    <tr>
                        <td>Ran Out of Money</td>
`)
	for i, r := range results {
		status := "No"
		class := "positive"
		if r.RanOutOfMoney {
			status = fmt.Sprintf("Yes (%d)", r.RanOutYear)
			class = "negative"
		}
		fmt.Fprintf(f, "                        <td class=\"%s\" onclick=\"showTab('strategy%d')\">%s</td>\n", class, i, status)
	}
	fmt.Fprintf(f, `                    </tr>
                </table>
            </div>

            <div class="card">
                <h2>Configuration</h2>
                <div class="grid grid-2">
                    <div>
                        <h3>People</h3>
                        <table>
                            <tr><th>Name</th><th>Birth</th><th>Retire</th><th>State Pen</th><th>ISA</th><th>Pension</th></tr>
`)
	for _, p := range config.People {
		birthYear := GetBirthYear(p.BirthDate)
		fmt.Fprintf(f, "                            <tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td><td>%s</td><td>%s</td></tr>\n",
			p.Name, birthYear, p.RetirementAge, p.StatePensionAge,
			FormatMoney(p.TaxFreeSavings), FormatMoney(p.Pension))
	}
	fmt.Fprintf(f, `                        </table>
                    </div>
                    <div>
                        <h3>Parameters</h3>
                        <table>
                            <tr><td>Pension Growth Rate</td><td>%.0f%%</td></tr>
                            <tr><td>Savings Growth Rate</td><td>%.0f%%</td></tr>
                            <tr><td>Income Inflation</td><td>%.0f%%</td></tr>
                            <tr><td>Income (before %d)</td><td>%s/month [after tax]</td></tr>
                            <tr><td>Income (after %d)</td><td>%s/month [after tax]</td></tr>
                            <tr><td>Mortgage</td><td>%s/year until %d</td></tr>
                        </table>
                    </div>
                </div>
            </div>
`, config.Financial.PensionGrowthRate*100, config.Financial.SavingsGrowthRate*100, config.Financial.IncomeInflationRate*100,
		config.IncomeRequirements.AgeThreshold,
		FormatMoney(config.IncomeRequirements.MonthlyBeforeAge),
		config.IncomeRequirements.AgeThreshold,
		FormatMoney(config.IncomeRequirements.MonthlyAfterAge),
		FormatMoney(config.GetTotalAnnualPayment()), config.Mortgage.EndYear)

	// Recommendation section
	if bestIdx >= 0 {
		best := results[bestIdx]
		bestFinalBalance := getTotalFinalBalance(best)
		fmt.Fprintf(f, `
            <div class="card">
                <h2>Recommendation</h2>
                <p><strong>Best Strategy:</strong> %s</p>
                <p>This strategy results in the highest final balance (%s) with total tax of %s`,
			best.Params.String(), FormatMoney(bestFinalBalance), FormatMoney(best.TotalTaxPaid))
		if best.RanOutOfMoney {
			fmt.Fprintf(f, ", running out of money in %d.</p>\n", best.RanOutYear)
		} else {
			fmt.Fprintf(f, ".</p>\n")
		}
		if !best.Params.EarlyMortgagePayoff {
			fmt.Fprintf(f, `                <p><em>Normal mortgage payoff is better because investment growth (5%%+) exceeds mortgage interest (~4%%).</em></p>
`)
		}
		fmt.Fprintf(f, `            </div>
`)
	}

	fmt.Fprintf(f, `        </div>
    </div>
`)

	// Generate each strategy tab
	for i, result := range results {
		var totalRemaining float64
		for _, b := range result.FinalBalances {
			totalRemaining += b.TaxFreeSavings + b.UncrystallisedPot + b.CrystallisedPot
		}

		ranOutClass := "success"
		ranOutText := "No"
		if result.RanOutOfMoney {
			ranOutClass = "danger"
			ranOutText = fmt.Sprintf("Yes (%d)", result.RanOutYear)
		}

		fmt.Fprintf(f, `
    <!-- Strategy %d: %s -->
    <div id="strategy%d" class="tab-content">
        <div class="container">
            <div class="card">
                <h2>Summary: %s</h2>
                <div class="grid grid-5">
                    <div class="metric">
                        <div class="metric-value">%s</div>
                        <div class="metric-label">Total Tax Paid</div>
                    </div>
                    <div class="metric">
                        <div class="metric-value">%s</div>
                        <div class="metric-label">Total Withdrawn</div>
                    </div>
                    <div class="metric success">
                        <div class="metric-value">%s</div>
                        <div class="metric-label">Savings Remaining</div>
                    </div>
                    <div class="metric %s">
                        <div class="metric-value">%s</div>
                        <div class="metric-label">Ran Out of Money</div>
                    </div>
                    <div class="metric">
                        <div class="metric-value">%d</div>
                        <div class="metric-label">Years Simulated</div>
                    </div>
                </div>
            </div>
`, i, result.Params.String(), i, result.Params.String(),
			FormatMoney(result.TotalTaxPaid), FormatMoney(result.TotalWithdrawn),
			FormatMoney(totalRemaining), ranOutClass, ranOutText, len(result.Years))

		// Strategy description
		writeStrategyDescription(f, result.Params)

		// Final balances
		fmt.Fprintf(f, `
            <div class="card">
                <h2>Final Balances (Age %d)</h2>
                <table>
                    <tr><th>Person</th><th>Tax-Free (ISA+PCLS)</th><th>Crystallised</th><th>Uncrystallised</th><th>Total</th></tr>
`, config.Simulation.EndAge)
		for name, bal := range result.FinalBalances {
			total := bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot
			fmt.Fprintf(f, "                    <tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td><strong>%s</strong></td></tr>\n",
				name, FormatMoney(bal.TaxFreeSavings), FormatMoney(bal.CrystallisedPot),
				FormatMoney(bal.UncrystallisedPot), FormatMoney(total))
		}
		fmt.Fprintf(f, "                    <tr class=\"balance-row\"><td><strong>Total</strong></td><td></td><td></td><td></td><td><strong>%s</strong></td></tr>\n",
			FormatMoney(totalRemaining))
		fmt.Fprintf(f, `                </table>
            </div>
`)

		// Year-by-year table
		fmt.Fprintf(f, `
            <div class="card">
                <h2>Year-by-Year Breakdown</h2>
                <div style="overflow-x: auto;">
                    <table>
                        <tr>
                            <th>Year</th>
`)
		for _, name := range names {
			fmt.Fprintf(f, "                            <th>%s</th>\n", name)
		}
		fmt.Fprintf(f, `                            <th>Required</th>
                            <th>State Pen</th>
                            <th>DB Pen</th>
                            <th>Tax-Free</th>
                            <th>Taxable</th>
                            <th>Tax</th>
                            <th>Net Required</th>
                            <th>Net Received</th>
`)
		for _, name := range names {
			fmt.Fprintf(f, "                            <th>%s ISA</th>\n", name)
			fmt.Fprintf(f, "                            <th>%s Pen</th>\n", name)
		}
		fmt.Fprintf(f, `                        </tr>
`)

		// Determine which year to highlight (mortgage payoff year)
		mortgagePayoffYear := config.Mortgage.EndYear
		if result.Params.EarlyMortgagePayoff {
			mortgagePayoffYear = config.Mortgage.EarlyPayoffYear
		}

		// Calculate colspan for details row
		colspan := 1 + len(names) + 8 + len(names)*2

		for _, year := range result.Years {
			highlightClass := ""
			if year.Year == mortgagePayoffYear {
				highlightClass = " highlight"
			}

			// Use unique IDs for combined report (include strategy index)
			rowID := fmt.Sprintf("s%d-%d", i, year.Year)

			// Main row (clickable)
			fmt.Fprintf(f, `                        <tr id="row-%s" class="expandable-row%s" onclick="toggleYear('%s')">
`, rowID, highlightClass, rowID)
			fmt.Fprintf(f, "                            <td>%d</td>\n", year.Year)
			for _, name := range names {
				fmt.Fprintf(f, "                            <td>%d</td>\n", year.Ages[name])
			}
			fmt.Fprintf(f, "                            <td>%s</td>\n", FormatMoney(year.TotalRequired))
			fmt.Fprintf(f, "                            <td>%s</td>\n", FormatMoney(year.TotalStatePension))
			fmt.Fprintf(f, "                            <td>%s</td>\n", formatOrDash(year.TotalDBPension))
			fmt.Fprintf(f, "                            <td>%s</td>\n", FormatMoney(year.Withdrawals.TotalTaxFree))
			fmt.Fprintf(f, "                            <td>%s</td>\n", FormatMoney(year.Withdrawals.TotalTaxable))
			fmt.Fprintf(f, "                            <td class=\"negative\">%s</td>\n", FormatMoney(year.TotalTaxPaid))
			fmt.Fprintf(f, "                            <td>%s</td>\n", FormatMoney(year.NetRequired))
			fmt.Fprintf(f, "                            <td class=\"positive\">%s</td>\n", FormatMoney(year.NetIncomeReceived))
			for _, name := range names {
				bal := year.EndBalances[name]
				fmt.Fprintf(f, "                            <td>%s</td>\n", FormatMoney(bal.TaxFreeSavings))
				fmt.Fprintf(f, "                            <td>%s</td>\n", FormatMoney(bal.CrystallisedPot+bal.UncrystallisedPot))
			}
			fmt.Fprintf(f, "                        </tr>\n")

			// Details row (hidden by default)
			fmt.Fprintf(f, `                        <tr id="details-%s" class="year-details">
                            <td colspan="%d">
`, rowID, colspan)
			writeYearDetailsContent(f, year, names)
			fmt.Fprintf(f, `                            </td>
                        </tr>
`)
		}

		fmt.Fprintf(f, `                    </table>
                </div>
            </div>
`)

		// Add lifetime withdrawal summary and detailed extraction for each strategy
		writeDrawdownSummaryHTML(f, result, names)
		writeDrawdownDetailsHTML(f, result, config, names)

		fmt.Fprintf(f, `        </div>
    </div>
`)
	}

	// JavaScript for tabs and footer
	fmt.Fprintf(f, `
    <div class="container">
        <div class="footer">
            Generated on %s | Pension Drawdown Tax Optimisation Simulation
        </div>
    </div>

    <script>
        function showTab(tabId) {
            // Hide all tab contents
            document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
            // Remove active from all tabs
            document.querySelectorAll('.tab').forEach(el => el.classList.remove('active'));
            // Show selected tab content
            document.getElementById(tabId).classList.add('active');
            // Mark clicked tab as active
            event.target.classList.add('active');
        }
        function toggleYear(year) {
            const row = document.getElementById('row-' + year);
            const details = document.getElementById('details-' + year);
            if (row && details) {
                row.classList.toggle('expanded');
                details.classList.toggle('show');
            }
        }
    </script>
</body>
</html>
`, time.Now().Format("2006-01-02 15:04:05"))

	return nil
}

// getStrategyDescription returns a detailed description of how the strategy works
func getStrategyDescription(params SimulationParams) string {
	var desc string

	// Crystallisation description - always gradual unless PCLS mortgage payoff is used
	if params.MortgageOpt == PCLSMortgagePayoff {
		desc = `<p><strong>PCLS Mortgage Payoff:</strong> At the mortgage payoff year, 25% of each person's
		pension pot is taken as a tax-free <em>Pension Commencement Lump Sum (PCLS)</em> to pay off the mortgage.
		The remaining 75% becomes taxable crystallised pension. After taking PCLS, future pension withdrawals
		are 100% taxable (no further 25% tax-free allowance).</p>`
	} else {
		desc = `<p><strong>Gradual Crystallisation:</strong> Pension is crystallised only as needed each year.
		When crystallising, 25% becomes tax-free cash (PCLS) and 75% becomes taxable. This approach keeps
		more money in the pension wrapper where it grows tax-free.</p>`
	}

	// Drawdown order description
	switch params.DrawdownOrder {
	case SavingsFirst:
		desc += `<p><strong>Savings First (ISA → Pension):</strong> Withdrawals come from ISA first (tax-free),
		then from crystallised pension (taxable), then crystallise more pension if needed. This preserves
		pension funds but uses up ISA early.</p>`
	case PensionFirst:
		desc += `<p><strong>Pension First (Pension → ISA):</strong> Withdrawals come from crystallised pension first
		(taxable), crystallising more as needed, using ISA only when pension is exhausted. This preserves
		ISA funds for later years.</p>`
	case TaxOptimized:
		desc += `<p><strong>Tax Optimized:</strong> Each year, calculates the optimal mix of ISA and pension
		withdrawals to minimise tax. Attempts to fill personal allowances with taxable income before using
		tax-free ISA funds, and balances withdrawals between people to equalise marginal tax rates.</p>`
	case PensionToISA:
		desc += `<p><strong>Pension to ISA:</strong> Over-draws from pension each year to fill tax bands
		(personal allowance + basic rate band). Any excess beyond spending needs is deposited into ISA
		(subject to £20,000/person annual ISA limit). This converts taxable pension into tax-free ISA
		while paying tax at lower rates.</p>
		<p><em>Note: The 25% PCLS from crystallisation is tax-free cash, not an ISA contribution.
		Only the deliberate excess transfers are limited to £20k/year per person.</em></p>`
	}

	return desc
}

// writeStrategyDescription writes the strategy explanation card
func writeStrategyDescription(f *os.File, params SimulationParams) {
	desc := getStrategyDescription(params)
	fmt.Fprintf(f, `
        <div class="card">
            <h2>How This Strategy Works</h2>
            <div style="line-height: 1.8; color: var(--text);">
                %s
            </div>
        </div>
`, desc)
}

// sanitizeFilename replaces characters that are not safe in filenames
func sanitizeFilename(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c == '/' || c == '\\' || c == ':' || c == '*' || c == '?' || c == '"' || c == '<' || c == '>' || c == '|' {
			result = append(result, '_')
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}

// writeDrawdownSummaryHTML writes the lifetime withdrawal summary section
func writeDrawdownSummaryHTML(f *os.File, result SimulationResult, names []string) {
	// Calculate totals per person
	personTotals := make(map[string]struct {
		statePen   float64
		dbPen      float64
		isa        float64
		penTaxFree float64
		penTaxable float64
		isaDeposit float64
		tax        float64
	})

	for _, name := range names {
		personTotals[name] = struct {
			statePen   float64
			dbPen      float64
			isa        float64
			penTaxFree float64
			penTaxable float64
			isaDeposit float64
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
			t.isaDeposit += year.Withdrawals.ISADeposits[name]
			t.tax += year.TaxByPerson[name]
			personTotals[name] = t
		}
	}

	fmt.Fprintf(f, `
        <div class="card">
            <h2>Lifetime Withdrawal Summary by Person</h2>
            <div class="grid grid-2">
`)

	var grandTotalISA, grandTotalPenTaxFree, grandTotalPenTaxable, grandTotalStatePen, grandTotalDBPen, grandTotalTax float64

	for _, name := range names {
		t := personTotals[name]
		totalFromPerson := t.isa + t.penTaxFree + t.penTaxable
		taxableIncome := t.statePen + t.dbPen + t.penTaxable
		effectiveRate := 0.0
		if taxableIncome > 0 {
			effectiveRate = (t.tax / taxableIncome) * 100
		}

		isaDepositRow := ""
		if t.isaDeposit > 0 {
			isaDepositRow = fmt.Sprintf(`<tr><td>Deposited to ISA</td><td>%s <span style="color: var(--primary);">→ ISA</span></td></tr>`, FormatMoney(t.isaDeposit))
		}

		dbPenRow := ""
		if t.dbPen > 0 {
			dbPenRow = fmt.Sprintf(`<tr><td>DB Pension</td><td>%s <span style="color: var(--text-muted);">(taxable)</span></td></tr>`, FormatMoney(t.dbPen))
		}

		fmt.Fprintf(f, `
                <div style="background: var(--bg); padding: 1rem; border-radius: 8px;">
                    <h3 style="color: var(--primary); margin-bottom: 1rem;">%s</h3>
                    <table>
                        <tr><td colspan="2" style="background: #e0e7ff; font-weight: 600;">Income Sources</td></tr>
                        <tr><td>State Pension</td><td>%s <span style="color: var(--text-muted);">(taxable)</span></td></tr>
                        %s
                        <tr><td colspan="2" style="background: #e0e7ff; font-weight: 600;">Withdrawals</td></tr>
                        <tr><td>From ISA</td><td>%s <span style="color: var(--success);">← tax-free</span></td></tr>
                        <tr><td>From Pension (25%% Tax-Free)</td><td>%s <span style="color: var(--success);">← tax-free</span></td></tr>
                        <tr><td>From Pension (Taxable)</td><td>%s <span style="color: var(--danger);">← income tax</span></td></tr>
                        %s
                        <tr><td colspan="2" style="background: #e0e7ff; font-weight: 600;">Totals</td></tr>
                        <tr><td><strong>Total Withdrawn</strong></td><td><strong>%s</strong></td></tr>
                        <tr><td><strong>Tax Paid</strong></td><td class="negative"><strong>%s</strong></td></tr>
                        <tr><td><strong>Effective Tax Rate</strong></td><td><strong>%.1f%%</strong></td></tr>
                    </table>
                </div>
`, name, FormatMoney(t.statePen), dbPenRow, FormatMoney(t.isa), FormatMoney(t.penTaxFree),
			FormatMoney(t.penTaxable), isaDepositRow, FormatMoney(totalFromPerson), FormatMoney(t.tax), effectiveRate)

		grandTotalISA += t.isa
		grandTotalPenTaxFree += t.penTaxFree
		grandTotalPenTaxable += t.penTaxable
		grandTotalStatePen += t.statePen
		grandTotalDBPen += t.dbPen
		grandTotalTax += t.tax
	}

	taxableTotal := grandTotalStatePen + grandTotalDBPen + grandTotalPenTaxable
	overallRate := 0.0
	if taxableTotal > 0 {
		overallRate = (grandTotalTax / taxableTotal) * 100
	}

	// Determine grid columns based on whether there's DB pension
	gridCols := "grid-4"
	if grandTotalDBPen > 0 {
		gridCols = "grid-5"
	}

	fmt.Fprintf(f, `
            </div>
            <div style="margin-top: 1.5rem; padding: 1rem; background: var(--bg); border-radius: 8px;">
                <h3 style="margin-bottom: 1rem;">Combined Totals</h3>
                <div class="grid %s" style="text-align: center;">
                    <div>
                        <div style="font-size: 1.25rem; font-weight: 700;">%s</div>
                        <div style="font-size: 0.75rem; color: var(--text-muted);">State Pension</div>
                    </div>`, gridCols, FormatMoney(grandTotalStatePen))

	if grandTotalDBPen > 0 {
		fmt.Fprintf(f, `
                    <div>
                        <div style="font-size: 1.25rem; font-weight: 700;">%s</div>
                        <div style="font-size: 0.75rem; color: var(--text-muted);">DB Pension</div>
                    </div>`, FormatMoney(grandTotalDBPen))
	}

	fmt.Fprintf(f, `
                    <div>
                        <div style="font-size: 1.25rem; font-weight: 700; color: var(--success);">%s</div>
                        <div style="font-size: 0.75rem; color: var(--text-muted);">Tax-Free Withdrawals</div>
                    </div>
                    <div>
                        <div style="font-size: 1.25rem; font-weight: 700;">%s</div>
                        <div style="font-size: 0.75rem; color: var(--text-muted);">Taxable Withdrawals</div>
                    </div>
                    <div>
                        <div style="font-size: 1.25rem; font-weight: 700; color: var(--danger);">%s</div>
                        <div style="font-size: 0.75rem; color: var(--text-muted);">Total Tax (%.1f%% eff.)</div>
                    </div>
                </div>
            </div>
        </div>
`, FormatMoney(grandTotalISA+grandTotalPenTaxFree),
		FormatMoney(grandTotalPenTaxable), FormatMoney(grandTotalTax), overallRate)
}

// writeYearDetailsContent writes the expandable details content for a single year
func writeYearDetailsContent(f *os.File, year YearState, names []string) {
	// Summary boxes
	fmt.Fprintf(f, `                            <div class="detail-grid">
                                <div class="detail-box">
                                    <div class="detail-box-header">Total Required</div>
                                    <div class="detail-box-value">%s</div>
                                </div>
                                <div class="detail-box">
                                    <div class="detail-box-header">State Pension</div>
                                    <div class="detail-box-value">%s</div>
                                </div>
`, FormatMoney(year.TotalRequired), FormatMoney(year.TotalStatePension))

	if year.TotalDBPension > 0 {
		fmt.Fprintf(f, `                                <div class="detail-box">
                                    <div class="detail-box-header">DB Pension</div>
                                    <div class="detail-box-value">%s</div>
                                </div>
`, FormatMoney(year.TotalDBPension))
	}

	fmt.Fprintf(f, `                                <div class="detail-box">
                                    <div class="detail-box-header">Mortgage</div>
                                    <div class="detail-box-value">%s</div>
                                </div>
                                <div class="detail-box">
                                    <div class="detail-box-header">Net Needed</div>
                                    <div class="detail-box-value">%s</div>
                                </div>
                                <div class="detail-box">
                                    <div class="detail-box-header">Tax Paid</div>
                                    <div class="detail-box-value negative">%s</div>
                                </div>
                            </div>
`, FormatMoney(year.MortgageCost), FormatMoney(year.NetRequired), FormatMoney(year.TotalTaxPaid))

	// Per-person extraction table
	fmt.Fprintf(f, `                            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
                                <div>
                                    <strong>Extractions by Person</strong>
                                    <table class="detail-table">
                                        <tr>
                                            <th style="text-align:left">Person</th>
                                            <th>From ISA</th>
                                            <th>Pen (Tax-Free)</th>
                                            <th>Pen (Taxable)</th>
                                            <th>To ISA</th>
                                            <th>Tax Paid</th>
                                        </tr>
`)

	for _, name := range names {
		isaWithdraw := year.Withdrawals.TaxFreeFromISA[name]
		penTaxFree := year.Withdrawals.TaxFreeFromPension[name]
		penTaxable := year.Withdrawals.TaxableFromPension[name]
		isaDeposit := year.Withdrawals.ISADeposits[name]
		tax := year.TaxByPerson[name]

		toISAStr := "-"
		if isaDeposit > 0 {
			toISAStr = fmt.Sprintf(`<span class="positive">+%s</span>`, FormatMoney(isaDeposit))
		}

		fmt.Fprintf(f, `                                        <tr>
                                            <td style="text-align:left; font-weight:600">%s</td>
                                            <td class="positive">%s</td>
                                            <td class="positive">%s</td>
                                            <td>%s</td>
                                            <td>%s</td>
                                            <td class="negative">%s</td>
                                        </tr>
`, name, formatOrDash(isaWithdraw), formatOrDash(penTaxFree), formatOrDash(penTaxable), toISAStr, formatOrDash(tax))
	}

	fmt.Fprintf(f, `                                    </table>
                                </div>
                                <div>
                                    <strong>End of Year Balances</strong>
                                    <table class="detail-table">
                                        <tr>
                                            <th style="text-align:left">Person</th>
                                            <th>ISA</th>
                                            <th>Crystallised</th>
                                            <th>Uncrystallised</th>
                                            <th>Total</th>
                                        </tr>
`)

	for _, name := range names {
		bal := year.EndBalances[name]
		total := bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot
		fmt.Fprintf(f, `                                        <tr>
                                            <td style="text-align:left; font-weight:600">%s</td>
                                            <td>%s</td>
                                            <td>%s</td>
                                            <td>%s</td>
                                            <td><strong>%s</strong></td>
                                        </tr>
`, name, FormatMoney(bal.TaxFreeSavings), FormatMoney(bal.CrystallisedPot), FormatMoney(bal.UncrystallisedPot), FormatMoney(total))
	}

	fmt.Fprintf(f, `                                    </table>
                                </div>
                            </div>
`)
}

// writeDrawdownDetailsHTML writes detailed year-by-year extraction breakdown
func writeDrawdownDetailsHTML(f *os.File, result SimulationResult, config *Config, names []string) {
	fmt.Fprintf(f, `
        <div class="card">
            <h2>Year-by-Year Extraction Details</h2>
            <p style="color: var(--text-muted); margin-bottom: 1rem;">Showing key years with detailed breakdown of where money is extracted from.</p>
`)

	for i, year := range result.Years {
		isKeyYear := i == 0 || i == len(result.Years)-1 || year.Year%5 == 0 ||
			year.Year == config.Mortgage.EndYear ||
			year.Year == config.Mortgage.EndYear-1

		if !isKeyYear {
			continue
		}

		shortfallWarning := ""
		if year.NetIncomeReceived < year.TotalRequired-1 {
			shortfall := year.TotalRequired - year.NetIncomeReceived
			shortfallWarning = fmt.Sprintf(` <span style="color: var(--danger); font-weight: 600;">⚠️ Shortfall: %s</span>`, FormatMoney(shortfall))
		}

		fmt.Fprintf(f, `
            <div style="margin-bottom: 1.5rem; border: 1px solid var(--border); border-radius: 8px; overflow: hidden;">
                <div style="background: var(--primary); color: white; padding: 0.75rem 1rem; display: flex; justify-content: space-between; align-items: center;">
                    <strong>Year %d</strong>
                    <span>`, year.Year)

		for _, name := range names {
			fmt.Fprintf(f, "%s: %d  ", name, year.Ages[name])
		}

		fmt.Fprintf(f, `</span>
                </div>
                <div style="padding: 1rem;">`)

		// Determine grid columns based on whether there's DB pension
		gridCols := "repeat(3, 1fr)"
		if year.TotalDBPension > 0 {
			gridCols = "repeat(4, 1fr)"
		}

		fmt.Fprintf(f, `
                    <div style="display: grid; grid-template-columns: %s; gap: 1rem; margin-bottom: 1rem; text-align: center;">
                        <div style="background: var(--bg); padding: 0.5rem; border-radius: 4px;">
                            <div style="font-weight: 600;">%s</div>
                            <div style="font-size: 0.75rem; color: var(--text-muted);">Required`, gridCols, FormatMoney(year.TotalRequired))

		if year.MortgageCost > 0 {
			fmt.Fprintf(f, ` (incl. %s mortgage)`, FormatMoney(year.MortgageCost))
		}

		fmt.Fprintf(f, `</div>
                        </div>
                        <div style="background: var(--bg); padding: 0.5rem; border-radius: 4px;">
                            <div style="font-weight: 600;">%s</div>
                            <div style="font-size: 0.75rem; color: var(--text-muted);">State Pension</div>
                        </div>`, FormatMoney(year.TotalStatePension))

		if year.TotalDBPension > 0 {
			fmt.Fprintf(f, `
                        <div style="background: var(--bg); padding: 0.5rem; border-radius: 4px;">
                            <div style="font-weight: 600;">%s</div>
                            <div style="font-size: 0.75rem; color: var(--text-muted);">DB Pension</div>
                        </div>`, FormatMoney(year.TotalDBPension))
		}

		fmt.Fprintf(f, `
                        <div style="background: var(--bg); padding: 0.5rem; border-radius: 4px;">
                            <div style="font-weight: 600;">%s</div>
                            <div style="font-size: 0.75rem; color: var(--text-muted);">Net Needed from Savings</div>
                        </div>
                    </div>

                    <table style="margin-bottom: 1rem;">
                        <tr style="background: #e0e7ff;">
                            <th style="text-align: left;">Person</th>
                            <th>State Pen</th>
                            <th>DB Pen</th>
                            <th>From ISA</th>
                            <th>Crystallised</th>
                            <th>Pension (Tax-Free)</th>
                            <th>Pension (Taxable)</th>
                            <th>To ISA</th>
                            <th>Tax Paid</th>
                        </tr>
`, FormatMoney(year.NetRequired))

		for _, name := range names {
			statePen := year.StatePensionByPerson[name]
			dbPen := year.DBPensionByPerson[name]
			isaWithdraw := year.Withdrawals.TaxFreeFromISA[name]
			penTaxFree := year.Withdrawals.TaxFreeFromPension[name]
			penTaxable := year.Withdrawals.TaxableFromPension[name]
			isaDeposit := year.Withdrawals.ISADeposits[name]
			tax := year.TaxByPerson[name]

			crystallised := ""
			if penTaxFree > 0 {
				crystallised = FormatMoney(penTaxFree * 4)
			} else {
				crystallised = "-"
			}

			toISAStr := "-"
			if isaDeposit > 0 {
				toISAStr = fmt.Sprintf(`<span style="color: var(--primary);">+%s</span>`, FormatMoney(isaDeposit))
			}

			fmt.Fprintf(f, `                        <tr>
                            <td style="text-align: left; font-weight: 600;">%s</td>
                            <td>%s</td>
                            <td>%s</td>
                            <td style="color: var(--success);">%s</td>
                            <td>%s</td>
                            <td style="color: var(--success);">%s</td>
                            <td>%s</td>
                            <td>%s</td>
                            <td class="negative">%s</td>
                        </tr>
`, name,
				formatOrDash(statePen),
				formatOrDash(dbPen),
				formatOrDash(isaWithdraw),
				crystallised,
				formatOrDash(penTaxFree),
				formatOrDash(penTaxable),
				toISAStr,
				formatOrDash(tax))
		}

		fmt.Fprintf(f, `                    </table>

                    <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
                        <div style="background: #dcfce7; padding: 0.75rem; border-radius: 4px;">
                            <strong>Net Income Received: %s</strong>%s
                        </div>
                        <div style="background: var(--bg); padding: 0.75rem; border-radius: 4px;">
                            <strong>End Balance: %s</strong>
                        </div>
                    </div>

                    <div style="margin-top: 1rem;">
                        <div style="font-size: 0.875rem; font-weight: 600; margin-bottom: 0.5rem;">End of Year Balances:</div>
                        <table>
                            <tr style="background: var(--bg);">
                                <th style="text-align: left;">Person</th>
                                <th>Tax-Free (ISA+PCLS)</th>
                                <th>Crystallised Pension</th>
                                <th>Uncrystallised Pension</th>
                                <th>Total</th>
                            </tr>
`, FormatMoney(year.NetIncomeReceived), shortfallWarning, FormatMoney(year.TotalBalance))

		for _, name := range names {
			bal := year.EndBalances[name]
			total := bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot
			fmt.Fprintf(f, `                            <tr>
                                <td style="text-align: left;">%s</td>
                                <td>%s</td>
                                <td>%s</td>
                                <td>%s</td>
                                <td><strong>%s</strong></td>
                            </tr>
`, name, FormatMoney(bal.TaxFreeSavings), FormatMoney(bal.CrystallisedPot),
				FormatMoney(bal.UncrystallisedPot), FormatMoney(total))
		}

		fmt.Fprintf(f, `                        </table>
                    </div>
                </div>
            </div>
`)
	}

	fmt.Fprintf(f, `        </div>
`)
}

// formatOrDash returns formatted money or "-" if zero
func formatOrDash(amount float64) string {
	if amount < 0.01 {
		return "-"
	}
	return FormatMoney(amount)
}

// GenerateDepletionHTMLReports generates HTML reports for depletion mode
func GenerateDepletionHTMLReports(results []DepletionResult, config *Config, outputDir string, timestamp string) (string, error) {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate combined report
	combinedPath := filepath.Join(outputDir, "summary.html")
	err := generateDepletionCombinedReport(results, config, combinedPath)
	if err != nil {
		return "", err
	}

	// Also generate individual strategy reports
	for _, r := range results {
		filename := fmt.Sprintf("report_%s.html", sanitizeFilename(r.Params.ShortName()))
		reportPath := filepath.Join(outputDir, filename)
		err := GenerateHTMLReport(r.SimulationResult, config, reportPath)
		if err != nil {
			return "", err
		}
	}

	return combinedPath, nil
}

// generateDepletionCombinedReport generates the combined HTML report for depletion mode
func generateDepletionCombinedReport(results []DepletionResult, config *Config, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	bestIdx := FindBestDepletionStrategy(results)
	ic := config.IncomeRequirements
	refPerson := config.GetSimulationReferencePerson()

	// Get names for display
	names := make([]string, len(config.People))
	for i, p := range config.People {
		names[i] = p.Name
	}

	// Write HTML header
	fmt.Fprintf(f, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Pension Depletion Mode Analysis</title>
    <style>
        :root {
            --primary: #4f46e5;
            --primary-dark: #4338ca;
            --success: #10b981;
            --warning: #f59e0b;
            --danger: #ef4444;
            --text: #1e293b;
            --text-muted: #64748b;
            --bg: #f1f5f9;
            --card-bg: #ffffff;
            --border: #e2e8f0;
        }
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: var(--bg);
            color: var(--text);
            line-height: 1.6;
        }
        .header {
            background: linear-gradient(135deg, var(--primary), var(--primary-dark));
            color: white;
            padding: 2rem;
            text-align: center;
        }
        .header h1 { font-size: 1.75rem; margin-bottom: 0.5rem; }
        .header .subtitle { opacity: 0.9; font-size: 0.875rem; }
        .depletion-banner {
            background: linear-gradient(135deg, #059669, #10b981);
            color: white;
            padding: 1.5rem;
            text-align: center;
            font-size: 1.1rem;
        }
        .depletion-banner strong { font-size: 1.3rem; }
        .container { max-width: 1400px; margin: 0 auto; padding: 1.5rem; }
        .tabs {
            display: flex;
            gap: 0.5rem;
            padding: 0.5rem 1.5rem;
            background: var(--card-bg);
            border-bottom: 1px solid var(--border);
            flex-wrap: wrap;
        }
        .tab {
            padding: 0.75rem 1.25rem;
            border: none;
            background: none;
            cursor: pointer;
            border-radius: 8px 8px 0 0;
            font-weight: 500;
            color: var(--text-muted);
            transition: all 0.2s;
        }
        .tab:hover { background: var(--bg); color: var(--text); }
        .tab.active { background: var(--primary); color: white; }
        .tab.recommended { border: 2px solid var(--success); }
        .tab-content { display: none; }
        .tab-content.active { display: block; }
        .card {
            background: var(--card-bg);
            border-radius: 8px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
            padding: 1.5rem;
            margin-bottom: 1.5rem;
        }
        .grid { display: grid; gap: 1rem; }
        .grid-4 { grid-template-columns: repeat(4, 1fr); }
        @media (max-width: 768px) { .grid-4 { grid-template-columns: 1fr; } }
        .metric {
            text-align: center;
            padding: 1rem;
            border-radius: 8px;
            background: var(--bg);
        }
        .metric-value {
            font-size: 1.5rem;
            font-weight: 700;
            color: var(--primary);
        }
        .metric-label {
            font-size: 0.875rem;
            color: var(--text-muted);
        }
        .metric.success .metric-value { color: var(--success); }
        table {
            width: 100%%;
            border-collapse: collapse;
            font-size: 0.875rem;
        }
        th, td {
            padding: 0.75rem 0.5rem;
            text-align: right;
            border-bottom: 1px solid var(--border);
        }
        th {
            background: var(--bg);
            font-weight: 600;
        }
        th:first-child, td:first-child { text-align: left; }
        tr:hover { background: #f1f5f9; }
        .clickable-row { cursor: pointer; transition: background 0.2s; }
        .clickable-row:hover { background: #e0e7ff !important; }
        .best { background: #dcfce7 !important; }
        .best.clickable-row:hover { background: #bbf7d0 !important; }
        .negative { color: var(--danger); }
        .positive { color: var(--success); }
        .comparison-table td { text-align: center; }
        .comparison-table th { text-align: center; }
        .comparison-table td:first-child, .comparison-table th:first-child { text-align: left; }
        .footer {
            text-align: center;
            color: var(--text-muted);
            font-size: 0.75rem;
            margin-top: 2rem;
            padding-top: 1rem;
            border-top: 1px solid var(--border);
        }
        /* Accordion styles for expandable rows */
        .expandable-row { cursor: pointer; }
        .expandable-row:hover { background: #e0e7ff !important; }
        .expandable-row.highlight { background: #fef3c7; }
        .expandable-row.highlight:hover { background: #fde68a !important; }
        .expandable-row td:first-child::before {
            content: '▶';
            display: inline-block;
            margin-right: 0.5rem;
            font-size: 0.75rem;
            transition: transform 0.2s;
        }
        .expandable-row.expanded td:first-child::before {
            transform: rotate(90deg);
        }
        .year-details {
            display: none;
            background: #f8fafc;
        }
        .year-details.show { display: table-row; }
        .year-details td {
            padding: 1rem;
            border-bottom: 2px solid var(--primary);
        }
        .detail-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 1rem;
        }
        .detail-box {
            background: white;
            border-radius: 8px;
            padding: 0.75rem;
            box-shadow: 0 1px 2px rgba(0,0,0,0.05);
        }
        .detail-box-header {
            font-size: 0.75rem;
            color: var(--text-muted);
            margin-bottom: 0.25rem;
        }
        .detail-box-value {
            font-size: 1rem;
            font-weight: 600;
            color: var(--text);
        }
        .highlight { background: #fef3c7 !important; }
        .pension-depleted { background: #fee2e2 !important; border-left: 4px solid var(--danger); }
        .pension-depleted td:first-child::after { content: ' [Pension Depleted]'; color: var(--danger); font-weight: bold; font-size: 0.75rem; }
        .detail-table {
            width: 100%%;
            margin-top: 0.5rem;
            font-size: 0.8rem;
        }
        .detail-table th {
            background: #e0e7ff;
            padding: 0.5rem;
            text-align: right;
        }
        .detail-table th:first-child { text-align: left; }
        .detail-table td {
            padding: 0.5rem;
            border-bottom: 1px solid var(--border);
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>Pension Depletion Mode Analysis</h1>
        <p class="subtitle">Generated %s</p>
    </div>
    <div class="depletion-banner">
        <strong>Target: Deplete by age %d</strong> (%s)<br>
        Income Ratio: %.0f:%.0f (before/after age %d)
    </div>
    <div class="tabs">
        <button class="tab active" onclick="showTab('comparison')">Comparison</button>
`, time.Now().Format("2 January 2006 at 15:04"), ic.TargetDepletionAge, refPerson.Name,
		ic.IncomeRatioPhase1, ic.IncomeRatioPhase2, ic.AgeThreshold)

	// Add tabs for each strategy
	for i, r := range results {
		recommended := ""
		if i == bestIdx {
			recommended = " recommended"
		}
		fmt.Fprintf(f, `        <button class="tab%s" onclick="showTab('strategy%d')">%s</button>
`, recommended, i, r.Params.ShortName())
	}

	fmt.Fprintf(f, `    </div>

    <!-- Comparison Tab -->
    <div id="comparison" class="tab-content active">
        <div class="container">
            <div class="card">
                <h2>Sustainable Income Comparison</h2>
                <p style="margin-bottom: 1rem; color: var(--text-muted);">
                    Each strategy shows the maximum sustainable monthly income that can be drawn while
                    depleting all funds by age %d.
                </p>
                <table class="comparison-table">
                    <thead>
                        <tr>
                            <th>Strategy</th>
                            <th>Monthly (Before %d)</th>
                            <th>Monthly (After %d)</th>
                            <th>Annual (Phase 1)</th>
                            <th>Total Lifetime Tax</th>
                        </tr>
                    </thead>
                    <tbody>
`, ic.TargetDepletionAge, ic.AgeThreshold, ic.AgeThreshold)

	// Write comparison table rows
	for i, r := range results {
		bestClass := "clickable-row"
		if i == bestIdx {
			bestClass = "best clickable-row"
		}
		fmt.Fprintf(f, `                        <tr class="%s" onclick="showTab('strategy%d')" title="Click to view details">
                            <td>%s</td>
                            <td>%s</td>
                            <td>%s</td>
                            <td>%s</td>
                            <td>%s</td>
                        </tr>
`, bestClass, i, r.Params.ShortName(),
			FormatMoney(r.MonthlyBeforeAge), FormatMoney(r.MonthlyAfterAge),
			FormatMoney(r.MonthlyBeforeAge*12), FormatMoney(r.SimulationResult.TotalTaxPaid))
	}

	fmt.Fprintf(f, `                    </tbody>
                </table>
            </div>
`)

	// Add recommendation card
	if bestIdx >= 0 {
		best := results[bestIdx]
		fmt.Fprintf(f, `            <div class="card" style="border-left: 4px solid var(--success);">
                <h2 style="color: var(--success);">Recommended: %s</h2>
                <div class="grid grid-4" style="margin-top: 1rem;">
                    <div class="metric success">
                        <div class="metric-value">%s</div>
                        <div class="metric-label">Monthly Before %d</div>
                    </div>
                    <div class="metric success">
                        <div class="metric-value">%s</div>
                        <div class="metric-label">Monthly After %d</div>
                    </div>
                    <div class="metric">
                        <div class="metric-value">%s</div>
                        <div class="metric-label">Annual (Phase 1)</div>
                    </div>
                    <div class="metric">
                        <div class="metric-value">%s</div>
                        <div class="metric-label">Total Tax Paid</div>
                    </div>
                </div>
            </div>
`, best.Params.String(), FormatMoney(best.MonthlyBeforeAge), ic.AgeThreshold,
			FormatMoney(best.MonthlyAfterAge), ic.AgeThreshold,
			FormatMoney(best.MonthlyBeforeAge*12), FormatMoney(best.SimulationResult.TotalTaxPaid))
	}

	fmt.Fprintf(f, `        </div>
    </div>
`)

	// Write individual strategy tabs
	for i, r := range results {
		result := r.SimulationResult
		fmt.Fprintf(f, `
    <!-- Strategy %d: %s -->
    <div id="strategy%d" class="tab-content">
        <div class="container">
            <div class="card">
                <h2>%s</h2>
                <div class="grid grid-4" style="margin-top: 1rem;">
                    <div class="metric success">
                        <div class="metric-value">%s</div>
                        <div class="metric-label">Monthly Before %d</div>
                    </div>
                    <div class="metric success">
                        <div class="metric-value">%s</div>
                        <div class="metric-label">Monthly After %d</div>
                    </div>
                    <div class="metric">
                        <div class="metric-value">%s</div>
                        <div class="metric-label">Total Tax Paid</div>
                    </div>
                    <div class="metric">
                        <div class="metric-value">%s</div>
                        <div class="metric-label">Balance at Target</div>
                    </div>
                </div>
            </div>
`, i, r.Params.String(), i, r.Params.String(),
			FormatMoney(r.MonthlyBeforeAge), ic.AgeThreshold,
			FormatMoney(r.MonthlyAfterAge), ic.AgeThreshold,
			FormatMoney(result.TotalTaxPaid), FormatMoney(r.ConvergenceError))

		// Year-by-year table
		fmt.Fprintf(f, `            <div class="card">
                <h3>Year-by-Year Breakdown</h3>
                <div style="overflow-x: auto;">
                    <table>
                        <thead>
                            <tr>
                                <th>Year</th>
`)
		for _, name := range names {
			fmt.Fprintf(f, "                                <th>%s Age</th>\n", name)
		}
		fmt.Fprintf(f, `                                <th>Required</th>
                                <th>State Pension</th>
                                <th>DB Pension</th>
                                <th>Tax-Free</th>
                                <th>Taxable</th>
                                <th>Tax Paid</th>
                                <th>Net Income</th>
                                <th>Balance</th>
                            </tr>
                        </thead>
                        <tbody>
`)

		targetYear := GetBirthYear(refPerson.BirthDate) + ic.TargetDepletionAge

		// Determine mortgage payoff year
		mortgagePayoffYear := config.Mortgage.EndYear
		if result.Params.EarlyMortgagePayoff {
			mortgagePayoffYear = config.Mortgage.EarlyPayoffYear
		}

		// Calculate colspan: Year + ages (len(names)) + 8 data columns
		colspan := 1 + len(names) + 8

		// Track previous pension balances to detect depletion
		prevPensionByPerson := make(map[string]float64)
		for _, name := range names {
			prevPensionByPerson[name] = 1.0 // Assume everyone starts with pension
		}

		for _, year := range result.Years {
			// Determine highlight class
			highlightClass := ""
			if year.Year == targetYear || year.Year == mortgagePayoffYear {
				highlightClass = " highlight"
			}

			// Check if any person's pension depleted this year
			for _, name := range names {
				bal := year.EndBalances[name]
				totalPension := bal.CrystallisedPot + bal.UncrystallisedPot
				if prevPensionByPerson[name] > 0 && totalPension <= 0 {
					highlightClass = " pension-depleted"
					break
				}
			}

			// Update previous balances for next iteration
			for _, name := range names {
				bal := year.EndBalances[name]
				prevPensionByPerson[name] = bal.CrystallisedPot + bal.UncrystallisedPot
			}

			rowID := fmt.Sprintf("s%d-%d", i, year.Year)

			// Main row (clickable)
			fmt.Fprintf(f, `                            <tr id="row-%s" class="expandable-row%s" onclick="toggleYear('%s')">
`, rowID, highlightClass, rowID)
			fmt.Fprintf(f, "                                <td>%d</td>\n", year.Year)
			for _, name := range names {
				fmt.Fprintf(f, "                                <td>%d</td>\n", year.Ages[name])
			}
			fmt.Fprintf(f, "                                <td>%s</td>\n", FormatMoney(year.TotalRequired))
			fmt.Fprintf(f, "                                <td>%s</td>\n", FormatMoney(year.TotalStatePension))
			fmt.Fprintf(f, "                                <td>%s</td>\n", formatOrDash(year.TotalDBPension))
			fmt.Fprintf(f, "                                <td>%s</td>\n", FormatMoney(year.Withdrawals.TotalTaxFree))
			fmt.Fprintf(f, "                                <td>%s</td>\n", FormatMoney(year.Withdrawals.TotalTaxable))
			fmt.Fprintf(f, "                                <td class=\"negative\">%s</td>\n", FormatMoney(year.TotalTaxPaid))
			fmt.Fprintf(f, "                                <td class=\"positive\">%s</td>\n", FormatMoney(year.NetIncomeReceived))
			fmt.Fprintf(f, "                                <td>%s</td>\n", FormatMoney(year.TotalBalance))
			fmt.Fprintf(f, "                            </tr>\n")

			// Detail row (hidden, expandable)
			fmt.Fprintf(f, `                            <tr id="details-%s" class="year-details">
                                <td colspan="%d">
                                <div class="detail-grid">
                                    <div class="detail-box">
                                        <div class="detail-box-header">Total Required</div>
                                        <div class="detail-box-value">%s</div>
                                    </div>
                                    <div class="detail-box">
                                        <div class="detail-box-header">State Pension</div>
                                        <div class="detail-box-value">%s</div>
                                    </div>
`, rowID, colspan, FormatMoney(year.TotalRequired), FormatMoney(year.TotalStatePension))

			// Only show DB Pension box if there is any
			if year.TotalDBPension > 0 {
				fmt.Fprintf(f, `                                    <div class="detail-box">
                                        <div class="detail-box-header">DB Pension</div>
                                        <div class="detail-box-value">%s</div>
                                    </div>
`, FormatMoney(year.TotalDBPension))
			}

			fmt.Fprintf(f, `                                    <div class="detail-box">
                                        <div class="detail-box-header">Mortgage</div>
                                        <div class="detail-box-value">%s</div>
                                    </div>
                                    <div class="detail-box">
                                        <div class="detail-box-header">Net Needed</div>
                                        <div class="detail-box-value">%s</div>
                                    </div>
                                    <div class="detail-box">
                                        <div class="detail-box-header">Tax Paid</div>
                                        <div class="detail-box-value negative">%s</div>
                                    </div>
                                </div>
                                <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
                                    <div>
                                        <strong>Extractions by Person</strong>
                                        <table class="detail-table">
                                            <tr>
                                                <th style="text-align:left">Person</th>
                                                <th>From ISA</th>
                                                <th>Pen (Tax-Free)</th>
                                                <th>Pen (Taxable)</th>
                                                <th>To ISA</th>
                                                <th>Tax Paid</th>
                                            </tr>
`, FormatMoney(year.MortgageCost), FormatMoney(year.NetRequired), FormatMoney(year.TotalTaxPaid))

			// Per-person extractions
			for _, name := range names {
				isaWithdraw := year.Withdrawals.TaxFreeFromISA[name]
				penTaxFree := year.Withdrawals.TaxFreeFromPension[name]
				penTaxable := year.Withdrawals.TaxableFromPension[name]
				isaDeposit := year.Withdrawals.ISADeposits[name]
				tax := year.TaxByPerson[name]

				toISAStr := "-"
				if isaDeposit > 0 {
					toISAStr = fmt.Sprintf(`<span class="positive">+%s</span>`, FormatMoney(isaDeposit))
				}

				fmt.Fprintf(f, `                                            <tr>
                                                <td style="text-align:left; font-weight:600">%s</td>
                                                <td class="positive">%s</td>
                                                <td class="positive">%s</td>
                                                <td>%s</td>
                                                <td>%s</td>
                                                <td class="negative">%s</td>
                                            </tr>
`, name, formatOrDash(isaWithdraw), formatOrDash(penTaxFree), formatOrDash(penTaxable), toISAStr, formatOrDash(tax))
			}

			fmt.Fprintf(f, `                                        </table>
                                    </div>
                                    <div>
                                        <strong>End of Year Balances</strong>
                                        <table class="detail-table">
                                            <tr>
                                                <th style="text-align:left">Person</th>
                                                <th>ISA</th>
                                                <th>Crystallised</th>
                                                <th>Uncrystallised</th>
                                                <th>Total</th>
                                            </tr>
`)

			// Per-person balances
			for _, name := range names {
				bal := year.EndBalances[name]
				total := bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot
				fmt.Fprintf(f, `                                            <tr>
                                                <td style="text-align:left; font-weight:600">%s</td>
                                                <td>%s</td>
                                                <td>%s</td>
                                                <td>%s</td>
                                                <td><strong>%s</strong></td>
                                            </tr>
`, name, FormatMoney(bal.TaxFreeSavings), FormatMoney(bal.CrystallisedPot), FormatMoney(bal.UncrystallisedPot), FormatMoney(total))
			}

			fmt.Fprintf(f, `                                        </table>
                                    </div>
                                </div>
                                </td>
                            </tr>
`)
		}

		fmt.Fprintf(f, `                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    </div>
`)
	}

	// Footer and JavaScript
	fmt.Fprintf(f, `
    <div class="container">
        <div class="footer">
            Generated on %s | Pension Depletion Mode Analysis
        </div>
    </div>

    <script>
        function showTab(tabId) {
            document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
            document.querySelectorAll('.tab').forEach(el => el.classList.remove('active'));
            document.getElementById(tabId).classList.add('active');
            // Find the corresponding tab button and activate it
            const tabs = document.querySelectorAll('.tab');
            tabs.forEach(tab => {
                if (tab.getAttribute('onclick') && tab.getAttribute('onclick').includes(tabId)) {
                    tab.classList.add('active');
                }
            });
        }
        function toggleYear(year) {
            const row = document.getElementById('row-' + year);
            const details = document.getElementById('details-' + year);
            if (row && details) {
                row.classList.toggle('expanded');
                details.classList.toggle('show');
            }
        }
    </script>
</body>
</html>
`, time.Now().Format("2006-01-02 15:04:05"))

	return nil
}

// GenerateDepletionSensitivityReport generates an HTML report for depletion sensitivity analysis
func GenerateDepletionSensitivityReport(analysis DepletionSensitivityAnalysis, config *Config) (string, error) {
	timestamp := time.Now().Format("2006-01-02_1504")
	outputDir := fmt.Sprintf("depletion_sensitivity_%s", timestamp)

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate detailed reports for each growth rate combination
	// Build a map for quick lookup of report paths
	reportPaths := make(map[string]string)
	for _, r := range analysis.Results {
		// Create subdirectory for this combination
		subDir := fmt.Sprintf("p%.0f_s%.0f", r.PensionGrowth*100, r.SavingsGrowth*100)
		subPath := filepath.Join(outputDir, subDir)

		// Clone config with these growth rates
		testConfig := *config
		testConfig.Financial.PensionGrowthRate = r.PensionGrowth
		testConfig.Financial.SavingsGrowthRate = r.SavingsGrowth

		// Generate detailed reports in subdirectory
		_, err := GenerateDepletionHTMLReports(r.Results, &testConfig, subPath, timestamp)
		if err != nil {
			return "", fmt.Errorf("failed to generate reports for p%.0f%%/s%.0f%%: %w",
				r.PensionGrowth*100, r.SavingsGrowth*100, err)
		}

		// Store relative path to summary (use forward slashes for HTML/URLs regardless of OS)
		key := fmt.Sprintf("%.2f_%.2f", r.PensionGrowth, r.SavingsGrowth)
		reportPaths[key] = subDir + "/summary.html"
	}
	fmt.Printf("  Generated %d detailed report sets in %s/\n", len(analysis.Results), outputDir)

	filename := filepath.Join(outputDir, "index.html")
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	ic := config.IncomeRequirements
	refPerson := config.GetSimulationReferencePerson()

	// HTML header and styles
	fmt.Fprintf(f, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Depletion Sensitivity Analysis</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f5f5f5; padding: 20px; }
        .container { max-width: 1400px; margin: 0 auto; }
        .header { background: linear-gradient(135deg, #1a5276 0%%, #2980b9 100%%); color: white; padding: 30px; border-radius: 10px; margin-bottom: 20px; }
        .header h1 { font-size: 28px; margin-bottom: 10px; }
        .header .subtitle { opacity: 0.9; font-size: 16px; }
        .info-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin-top: 20px; }
        .info-item { background: rgba(255,255,255,0.1); padding: 15px; border-radius: 8px; }
        .info-label { font-size: 12px; opacity: 0.8; margin-bottom: 5px; }
        .info-value { font-size: 20px; font-weight: bold; }
        .card { background: white; border-radius: 10px; padding: 20px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .card h2 { color: #1a5276; margin-bottom: 15px; font-size: 20px; }
        table { width: 100%%; border-collapse: collapse; font-size: 13px; }
        th, td { padding: 10px 8px; text-align: right; border-bottom: 1px solid #eee; }
        th { background: #f8f9fa; font-weight: 600; color: #333; position: sticky; top: 0; }
        th:first-child, td:first-child { text-align: center; }
        .best { background: #d4edda; font-weight: bold; }
        .heatmap-cell { color: white; font-weight: bold; cursor: pointer; transition: transform 0.1s, box-shadow 0.1s; }
        .heatmap-cell:hover { transform: scale(1.05); box-shadow: 0 2px 8px rgba(0,0,0,0.3); z-index: 10; position: relative; }
        .legend { display: flex; gap: 20px; margin-top: 15px; flex-wrap: wrap; }
        .legend-item { display: flex; align-items: center; gap: 5px; font-size: 12px; }
        .legend-color { width: 20px; height: 20px; border-radius: 4px; }
        .footer { text-align: center; padding: 20px; color: #666; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Depletion Mode Sensitivity Analysis</h1>
            <p class="subtitle">Sustainable income across different growth rate combinations</p>
            <div class="info-grid">
                <div class="info-item">
                    <div class="info-label">Target Depletion Age</div>
                    <div class="info-value">%d</div>
                </div>
                <div class="info-item">
                    <div class="info-label">Reference Person</div>
                    <div class="info-value">%s</div>
                </div>
                <div class="info-item">
                    <div class="info-label">Income Ratio</div>
                    <div class="info-value">%.0f:%.0f</div>
                </div>
                <div class="info-item">
                    <div class="info-label">Age Threshold</div>
                    <div class="info-value">%d</div>
                </div>
            </div>
        </div>
`, ic.TargetDepletionAge, refPerson.Name, ic.IncomeRatioPhase1, ic.IncomeRatioPhase2, ic.AgeThreshold)

	// Find min/max income for color scaling
	minIncome := analysis.Results[0].BestIncome
	maxIncome := analysis.Results[0].BestIncome
	for _, r := range analysis.Results {
		if r.BestIncome < minIncome {
			minIncome = r.BestIncome
		}
		if r.BestIncome > maxIncome {
			maxIncome = r.BestIncome
		}
	}

	// Build a map for easy lookup
	incomeMap := make(map[string]DepletionSensitivityResult)
	for _, r := range analysis.Results {
		key := fmt.Sprintf("%.2f_%.2f", r.PensionGrowth, r.SavingsGrowth)
		incomeMap[key] = r
	}

	// Collect unique growth rates
	pensionRates := []float64{}
	savingsRates := []float64{}
	sens := config.Sensitivity
	for p := sens.PensionGrowthMin; p <= sens.PensionGrowthMax+0.001; p += sens.StepSize {
		pensionRates = append(pensionRates, p)
	}
	for s := sens.SavingsGrowthMin; s <= sens.SavingsGrowthMax+0.001; s += sens.StepSize {
		savingsRates = append(savingsRates, s)
	}

	// Income heatmap
	fmt.Fprintf(f, `
        <div class="card">
            <h2>Sustainable Monthly Income (Phase 1) by Growth Rates</h2>
            <p style="margin-bottom: 15px; color: #666;">Higher income = better. Colors show income levels. <strong>Click any cell for detailed breakdown.</strong></p>
            <div style="overflow-x: auto;">
                <table>
                    <thead>
                        <tr>
                            <th>Pension ↓ / Savings →</th>
`)
	for _, s := range savingsRates {
		fmt.Fprintf(f, "                            <th>%.0f%%</th>\n", s*100)
	}
	fmt.Fprintf(f, "                        </tr>\n                    </thead>\n                    <tbody>\n")

	for _, p := range pensionRates {
		fmt.Fprintf(f, "                        <tr>\n                            <th>%.0f%%</th>\n", p*100)
		for _, s := range savingsRates {
			key := fmt.Sprintf("%.2f_%.2f", p, s)
			if r, ok := incomeMap[key]; ok {
				// Calculate color intensity
				intensity := 0.0
				if maxIncome > minIncome {
					intensity = (r.BestIncome - minIncome) / (maxIncome - minIncome)
				}
				// Green gradient
				red := int(255 - intensity*155)
				green := int(100 + intensity*155)
				blue := int(100 - intensity*50)
				reportPath := reportPaths[key]
				fmt.Fprintf(f, "                            <td class=\"heatmap-cell\" style=\"background: rgb(%d,%d,%d)\" title=\"Pension: %.0f%%, Savings: %.0f%% - Click for details\" onclick=\"window.location='%s'\">£%.0f</td>\n",
					red, green, blue, p*100, s*100, reportPath, r.BestIncome)
			} else {
				fmt.Fprintf(f, "                            <td>-</td>\n")
			}
		}
		fmt.Fprintf(f, "                        </tr>\n")
	}

	fmt.Fprintf(f, `                    </tbody>
                </table>
            </div>
            <div class="legend">
                <div class="legend-item"><div class="legend-color" style="background: rgb(100,100,100)"></div> Lower Income</div>
                <div class="legend-item"><div class="legend-color" style="background: rgb(100,255,50)"></div> Higher Income</div>
            </div>
        </div>
`)

	// Best strategy heatmap
	fmt.Fprintf(f, `
        <div class="card">
            <h2>Best Strategy by Growth Rates</h2>
            <p style="margin-bottom: 15px; color: #666;">Shows which strategy provides highest sustainable income. <strong>Click any cell for detailed breakdown.</strong></p>
            <div style="overflow-x: auto;">
                <table>
                    <thead>
                        <tr>
                            <th>Pension ↓ / Savings →</th>
`)
	for _, s := range savingsRates {
		fmt.Fprintf(f, "                            <th>%.0f%%</th>\n", s*100)
	}
	fmt.Fprintf(f, "                        </tr>\n                    </thead>\n                    <tbody>\n")

	// Strategy colors
	strategyColors := map[string]string{
		"ISA→Pen/Early":  "#3498db",
		"ISA→Pen/2031":   "#2980b9",
		"Pen→ISA/Early":  "#27ae60",
		"Pen→ISA/2031":   "#1e8449",
		"TaxOpt/Early":   "#e74c3c",
		"TaxOpt/2031":    "#c0392b",
		"Pen+ISA/Early":  "#9b59b6",
		"Pen+ISA/2031":   "#8e44ad",
	}

	for _, p := range pensionRates {
		fmt.Fprintf(f, "                        <tr>\n                            <th>%.0f%%</th>\n", p*100)
		for _, s := range savingsRates {
			key := fmt.Sprintf("%.2f_%.2f", p, s)
			if r, ok := incomeMap[key]; ok {
				color := strategyColors[r.BestStrategyName]
				if color == "" {
					color = "#666"
				}
				reportPath := reportPaths[key]
				fmt.Fprintf(f, "                            <td class=\"heatmap-cell\" style=\"background: %s\" title=\"Pension: %.0f%%, Savings: %.0f%% - Click for details\" onclick=\"window.location='%s'\">%s</td>\n",
					color, p*100, s*100, reportPath, r.BestStrategyName)
			} else {
				fmt.Fprintf(f, "                            <td>-</td>\n")
			}
		}
		fmt.Fprintf(f, "                        </tr>\n")
	}

	fmt.Fprintf(f, `                    </tbody>
                </table>
            </div>
            <div class="legend">
`)
	for name, color := range strategyColors {
		fmt.Fprintf(f, "                <div class=\"legend-item\"><div class=\"legend-color\" style=\"background: %s\"></div> %s</div>\n", color, name)
	}
	fmt.Fprintf(f, `            </div>
        </div>
`)

	// Detailed results table
	fmt.Fprintf(f, `
        <div class="card">
            <h2>Detailed Results</h2>
            <p style="margin-bottom: 15px; color: #666;"><strong>Click any row for detailed breakdown.</strong></p>
            <div style="overflow-x: auto; max-height: 500px;">
                <table>
                    <thead>
                        <tr>
                            <th>Pension</th>
                            <th>Savings</th>
                            <th>Best Strategy</th>
                            <th>Monthly (Phase 1)</th>
                            <th>Monthly (Phase 2)</th>
                            <th>Annual (Phase 1)</th>
                        </tr>
                    </thead>
                    <tbody>
`)
	for _, r := range analysis.Results {
		phase2 := r.BestIncome * (ic.IncomeRatioPhase2 / ic.IncomeRatioPhase1)
		key := fmt.Sprintf("%.2f_%.2f", r.PensionGrowth, r.SavingsGrowth)
		reportPath := reportPaths[key]
		fmt.Fprintf(f, `                        <tr style="cursor: pointer;" onclick="window.location='%s'" title="Click for detailed breakdown">
                            <td>%.0f%%</td>
                            <td>%.0f%%</td>
                            <td style="text-align: left;">%s</td>
                            <td>£%.0f</td>
                            <td>£%.0f</td>
                            <td>£%.0f</td>
                        </tr>
`, reportPath, r.PensionGrowth*100, r.SavingsGrowth*100, r.BestStrategyName, r.BestIncome, phase2, r.BestIncome*12)
	}

	fmt.Fprintf(f, `                    </tbody>
                </table>
            </div>
        </div>

        <div class="footer">
            Generated on %s | Depletion Sensitivity Analysis
        </div>
    </div>
</body>
</html>
`, time.Now().Format("2006-01-02 15:04:05"))

	return filename, nil
}

// GeneratePensionOnlyDepletionHTMLReports generates HTML reports for pension-only depletion mode
func GeneratePensionOnlyDepletionHTMLReports(results []DepletionResult, config *Config, outputDir string, timestamp string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}
	combinedPath := filepath.Join(outputDir, "summary.html")
	err := generatePensionOnlyCombinedReport(results, config, combinedPath)
	if err != nil {
		return "", err
	}
	for _, r := range results {
		filename := fmt.Sprintf("report_%s.html", sanitizeFilename(r.Params.ShortName()))
		reportPath := filepath.Join(outputDir, filename)
		if err := GenerateHTMLReport(r.SimulationResult, config, reportPath); err != nil {
			return "", err
		}
	}
	return combinedPath, nil
}

func generatePensionOnlyCombinedReport(results []DepletionResult, config *Config, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	bestIdx := FindBestDepletionStrategy(results)
	ic := config.IncomeRequirements
	refPerson := config.GetSimulationReferencePerson()
	finalISA := 0.0
	if bestIdx >= 0 && len(results[bestIdx].SimulationResult.Years) > 0 {
		lastYear := results[bestIdx].SimulationResult.Years[len(results[bestIdx].SimulationResult.Years)-1]
		for _, bal := range lastYear.EndBalances {
			finalISA += bal.TaxFreeSavings
		}
	}
	fmt.Fprintf(f, `<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><title>Pension-Only Depletion</title>
<style>body{font-family:system-ui;background:#f1f5f9;margin:0;padding:0}.header{background:linear-gradient(135deg,#0d9488,#0f766e);color:#fff;padding:2rem;text-align:center}.container{max-width:1200px;margin:0 auto;padding:1.5rem}.card{background:#fff;border-radius:8px;box-shadow:0 1px 3px rgba(0,0,0,.1);padding:1.5rem;margin-bottom:1.5rem}.metrics{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:1rem;margin-bottom:1.5rem}.metric{text-align:center;padding:1rem;background:#f1f5f9;border-radius:8px}.metric-value{font-size:1.5rem;font-weight:700;color:#0d9488}.metric-label{font-size:.875rem;color:#64748b}table{width:100%%;border-collapse:collapse}th,td{padding:.75rem;text-align:left;border-bottom:1px solid #e2e8f0}th{background:#f1f5f9}.highlight{background:#d1fae5}.footer{text-align:center;padding:1rem;color:#64748b}</style></head>
<body><div class="header"><h1>Pension-Only Depletion Analysis</h1><p>Target: Deplete pensions by age %d (%s) | ISAs preserved</p></div>
<div class="container"><div class="metrics"><div class="metric"><div class="metric-value">%s</div><div class="metric-label">Monthly Income (Phase 1)</div></div>
<div class="metric"><div class="metric-value">%s</div><div class="metric-label">Monthly Income (Phase 2)</div></div>
<div class="metric"><div class="metric-value">%s</div><div class="metric-label">ISA Preserved</div></div></div>
<div class="card"><h2>Strategy Comparison</h2><p style="margin-bottom:1rem;color:#64748b;font-size:.9rem"><strong>PCLS</strong> (Pension Commencement Lump Sum) is the 25%% tax-free amount you can take when crystallising your pension.</p><table><thead><tr><th>Strategy</th><th>Monthly (Before)</th><th>Monthly (After)</th><th>Annual</th><th>Total Tax</th><th>Final ISA</th></tr></thead><tbody>
`, ic.TargetDepletionAge, refPerson.Name, FormatMoney(results[bestIdx].MonthlyBeforeAge), FormatMoney(results[bestIdx].MonthlyAfterAge), FormatMoney(finalISA))
	for i, r := range results {
		rowClass := ""
		if i == bestIdx {
			rowClass = ` class="highlight"`
		}
		fISA := 0.0
		if len(r.SimulationResult.Years) > 0 {
			lastYear := r.SimulationResult.Years[len(r.SimulationResult.Years)-1]
			for _, bal := range lastYear.EndBalances {
				fISA += bal.TaxFreeSavings
			}
		}
		fmt.Fprintf(f, `<tr%s><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>
`, rowClass, r.Params.ShortName(), FormatMoney(r.MonthlyBeforeAge), FormatMoney(r.MonthlyAfterAge), FormatMoney(r.MonthlyBeforeAge*12), FormatMoney(r.SimulationResult.TotalTaxPaid), FormatMoney(fISA))
	}
	fmt.Fprintf(f, `</tbody></table></div><div class="card"><h2>Key Benefits</h2><ul><li><strong>ISA Preserved:</strong> %s remains untouched</li><li><strong>Tax-Free Growth:</strong> ISAs grow tax-free</li><li><strong>Flexibility:</strong> ISAs available for emergencies</li></ul></div>
<div class="footer">Generated on %s | Pension-Only Depletion</div></div></body></html>
`, FormatMoney(finalISA), time.Now().Format("2006-01-02 15:04:05"))
	return nil
}

// GeneratePensionToISADepletionHTMLReports generates HTML reports for PensionToISA depletion mode
func GeneratePensionToISADepletionHTMLReports(results []DepletionResult, config *Config, outputDir string, timestamp string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}
	combinedPath := filepath.Join(outputDir, "summary.html")
	if err := generatePensionToISACombinedReport(results, config, combinedPath); err != nil {
		return "", err
	}
	for _, r := range results {
		filename := fmt.Sprintf("report_%s.html", sanitizeFilename(r.Params.ShortName()))
		reportPath := filepath.Join(outputDir, filename)
		if err := GenerateHTMLReport(r.SimulationResult, config, reportPath); err != nil {
			return "", err
		}
	}
	return combinedPath, nil
}

func generatePensionToISACombinedReport(results []DepletionResult, config *Config, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	bestIdx := FindBestDepletionStrategy(results)
	ic := config.IncomeRequirements
	refPerson := config.GetSimulationReferencePerson()
	finalISA := 0.0
	if bestIdx >= 0 && len(results[bestIdx].SimulationResult.Years) > 0 {
		lastYear := results[bestIdx].SimulationResult.Years[len(results[bestIdx].SimulationResult.Years)-1]
		for _, bal := range lastYear.EndBalances {
			finalISA += bal.TaxFreeSavings
		}
	}
	fmt.Fprintf(f, `<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><title>Pension-to-ISA Depletion</title>
<style>body{font-family:system-ui;background:#f1f5f9;margin:0;padding:0}.header{background:linear-gradient(135deg,#7c3aed,#6d28d9);color:#fff;padding:2rem;text-align:center}.container{max-width:1200px;margin:0 auto;padding:1.5rem}.card{background:#fff;border-radius:8px;box-shadow:0 1px 3px rgba(0,0,0,.1);padding:1.5rem;margin-bottom:1.5rem}.metrics{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:1rem;margin-bottom:1.5rem}.metric{text-align:center;padding:1rem;background:#f1f5f9;border-radius:8px}.metric-value{font-size:1.5rem;font-weight:700;color:#7c3aed}.metric-label{font-size:.875rem;color:#64748b}table{width:100%%;border-collapse:collapse}th,td{padding:.75rem;text-align:left;border-bottom:1px solid #e2e8f0}th{background:#f1f5f9}.highlight{background:#ede9fe}.footer{text-align:center;padding:1rem;color:#64748b}</style></head>
<body><div class="header"><h1>Pension-to-ISA Depletion Analysis</h1><p>Target: Deplete by age %d (%s) | Tax-efficient pension to ISA transfers</p></div>
<div class="container"><div class="metrics"><div class="metric"><div class="metric-value">%s</div><div class="metric-label">Monthly Income (Phase 1)</div></div>
<div class="metric"><div class="metric-value">%s</div><div class="metric-label">Monthly Income (Phase 2)</div></div>
<div class="metric"><div class="metric-value">%s</div><div class="metric-label">Final ISA (after transfers)</div></div></div>
<div class="card"><h2>Strategy Comparison</h2><p style="margin-bottom:1rem;color:#64748b;font-size:.9rem"><strong>PCLS</strong> (Pension Commencement Lump Sum) is the 25%% tax-free amount you can take when crystallising your pension.</p><table><thead><tr><th>Strategy</th><th>Monthly (Before)</th><th>Monthly (After)</th><th>Annual</th><th>Total Tax</th><th>Final ISA</th></tr></thead><tbody>
`, ic.TargetDepletionAge, refPerson.Name, FormatMoney(results[bestIdx].MonthlyBeforeAge), FormatMoney(results[bestIdx].MonthlyAfterAge), FormatMoney(finalISA))
	for i, r := range results {
		rowClass := ""
		if i == bestIdx {
			rowClass = ` class="highlight"`
		}
		fISA := 0.0
		if len(r.SimulationResult.Years) > 0 {
			lastYear := r.SimulationResult.Years[len(r.SimulationResult.Years)-1]
			for _, bal := range lastYear.EndBalances {
				fISA += bal.TaxFreeSavings
			}
		}
		fmt.Fprintf(f, `<tr%s><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>
`, rowClass, r.Params.ShortName(), FormatMoney(r.MonthlyBeforeAge), FormatMoney(r.MonthlyAfterAge), FormatMoney(r.MonthlyBeforeAge*12), FormatMoney(r.SimulationResult.TotalTaxPaid), FormatMoney(fISA))
	}
	fmt.Fprintf(f, `</tbody></table></div><div class="card"><h2>How It Works</h2><ul><li><strong>Tax Band Filling:</strong> Overdraws pension to fill personal allowance and basic rate band</li><li><strong>ISA Transfers:</strong> Excess after spending goes to ISA</li><li><strong>Final ISA:</strong> %s accumulated through transfers</li></ul></div>
<div class="footer">Generated on %s | Pension-to-ISA Depletion</div></div></body></html>
`, FormatMoney(finalISA), time.Now().Format("2006-01-02 15:04:05"))
	return nil
}

// GeneratePensionToISASensitivityReport generates the sensitivity analysis report for PensionToISA mode
func GeneratePensionToISASensitivityReport(analysis PensionToISASensitivityAnalysis, config *Config) (string, error) {
	timestamp := time.Now().Format("2006-01-02_1504")
	outputDir := fmt.Sprintf("reports_%s_pension_to_isa_sensitivity", timestamp)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	filename := filepath.Join(outputDir, "sensitivity.html")
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	ic := config.IncomeRequirements
	sens := config.Sensitivity
	refPerson := config.GetSimulationReferencePerson()
	numSavingsSteps := int((sens.SavingsGrowthMax-sens.SavingsGrowthMin)/sens.StepSize) + 1
	minIncome, maxIncome := 0.0, 0.0
	for i, r := range analysis.Results {
		if i == 0 || r.BestIncome < minIncome {
			minIncome = r.BestIncome
		}
		if r.BestIncome > maxIncome {
			maxIncome = r.BestIncome
		}
	}
	incomeRange := maxIncome - minIncome
	fmt.Fprintf(f, `<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><title>Pension-to-ISA Sensitivity</title>
<style>body{font-family:system-ui;background:#f1f5f9;margin:0}.header{background:linear-gradient(135deg,#7c3aed,#6d28d9);color:#fff;padding:2rem;text-align:center}.container{max-width:1400px;margin:0 auto;padding:1.5rem}.card{background:#fff;border-radius:8px;box-shadow:0 1px 3px rgba(0,0,0,.1);padding:1.5rem;margin-bottom:1.5rem}.matrix{display:grid;gap:2px;margin:1rem 0}.cell{padding:8px;text-align:center;font-size:.75rem;border-radius:4px}.axis-label{font-weight:600;background:#f1f5f9}.income-high{background:#10b981;color:#fff}.income-medium{background:#fbbf24}.income-low{background:#ef4444;color:#fff}.legend{display:flex;gap:1rem;flex-wrap:wrap;margin-top:1rem}.legend-item{display:flex;align-items:center;gap:.5rem}.legend-color{width:20px;height:20px;border-radius:4px}.footer{text-align:center;padding:1rem;color:#64748b}</style></head>
<body><div class="header"><h1>Pension-to-ISA Sensitivity Analysis</h1><p>Target: Deplete by age %d | Ratio %.0f:%.0f</p></div>
<div class="container"><div class="card"><h2>Sustainable Monthly Income by Growth Rate</h2><p>Pension growth (vertical) vs Savings growth (horizontal)</p>
<div class="matrix" style="grid-template-columns:auto repeat(%d,1fr)"><div class="cell axis-label"></div>
`, ic.TargetDepletionAge, ic.IncomeRatioPhase1, ic.IncomeRatioPhase2, numSavingsSteps)
	for savingsGrowth := sens.SavingsGrowthMin; savingsGrowth <= sens.SavingsGrowthMax+0.001; savingsGrowth += sens.StepSize {
		fmt.Fprintf(f, `<div class="cell axis-label">%.0f%%</div>`, savingsGrowth*100)
	}
	resultIdx := 0
	for pensionGrowth := sens.PensionGrowthMin; pensionGrowth <= sens.PensionGrowthMax+0.001; pensionGrowth += sens.StepSize {
		fmt.Fprintf(f, `<div class="cell axis-label">%.0f%%</div>`, pensionGrowth*100)
		for savingsGrowth := sens.SavingsGrowthMin; savingsGrowth <= sens.SavingsGrowthMax+0.001 && resultIdx < len(analysis.Results); savingsGrowth += sens.StepSize {
			r := analysis.Results[resultIdx]
			resultIdx++
			colorClass := "income-medium"
			if incomeRange > 0 {
				ratio := (r.BestIncome - minIncome) / incomeRange
				if ratio > 0.66 {
					colorClass = "income-high"
				} else if ratio < 0.33 {
					colorClass = "income-low"
				}
			}
			fmt.Fprintf(f, `<div class="cell %s" title="P:%.0f%% S:%.0f%% Income:%s ISA:%s">%s</div>`, colorClass, r.PensionGrowth*100, r.SavingsGrowth*100, FormatMoney(r.BestIncome), FormatMoney(r.FinalISABalance), FormatMoney(r.BestIncome))
		}
	}
	fmt.Fprintf(f, `</div><div class="legend"><div class="legend-item"><div class="legend-color income-high"></div><span>High Income</span></div><div class="legend-item"><div class="legend-color income-medium"></div><span>Medium</span></div><div class="legend-item"><div class="legend-color income-low"></div><span>Low</span></div></div></div>
<div class="card"><h2>Summary</h2><p>Reference: %s | Target Age: %d | Income Range: %s - %s/month</p></div>
<div class="footer">Generated on %s</div></div></body></html>
`, refPerson.Name, ic.TargetDepletionAge, FormatMoney(minIncome), FormatMoney(maxIncome), time.Now().Format("2006-01-02 15:04:05"))
	return filename, nil
}
