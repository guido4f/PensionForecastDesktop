package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

// SensitivityResult holds the result of a single growth rate combination
type SensitivityResult struct {
	PensionGrowth   float64
	SavingsGrowth   float64
	BestStrategy    string
	BestStrategyIdx int
	LastsUntil      int
	TotalTax        float64
	FinalBalance    float64
	AllRunOut       bool               // True only if ALL strategies truly deplete (no balance left)
	HasShortfall    bool               // True if best strategy has income shortfall but still has balance
	AllResults      []SimulationResult // All strategy results for this combination
	ReportDir       string             // Subdirectory for this combination's reports
	SummaryFile     string             // Path to summary.html for this combination
}

// SensitivityAnalysis holds the complete analysis
type SensitivityAnalysis struct {
	Results            [][]SensitivityResult // [pensionIdx][savingsIdx]
	PensionGrowthRates []float64
	SavingsGrowthRates []float64
	StrategyNames      []string
	Config             *Config
	Timestamp          string
	OutputDir          string // Main output directory
}

// buildGrowthRates generates a slice of growth rates from min to max with given step
func buildGrowthRates(min, max, step float64) []float64 {
	var rates []float64
	for r := min; r <= max+0.0001; r += step { // small epsilon for float comparison
		rates = append(rates, r)
	}
	return rates
}

// RunSensitivityAnalysis runs simulations across a range of growth rates
func RunSensitivityAnalysis(config *Config) *SensitivityAnalysis {
	// Use config for growth rate ranges, with defaults if not set
	pensionMin := config.Sensitivity.PensionGrowthMin
	pensionMax := config.Sensitivity.PensionGrowthMax
	savingsMin := config.Sensitivity.SavingsGrowthMin
	savingsMax := config.Sensitivity.SavingsGrowthMax
	step := config.Sensitivity.StepSize

	// Set defaults if not configured
	if pensionMin == 0 && pensionMax == 0 {
		pensionMin, pensionMax = 0.04, 0.12
	}
	if savingsMin == 0 && savingsMax == 0 {
		savingsMin, savingsMax = 0.04, 0.12
	}
	if step == 0 {
		step = 0.01
	}

	pensionRates := buildGrowthRates(pensionMin, pensionMax, step)
	savingsRates := buildGrowthRates(savingsMin, savingsMax, step)
	timestamp := time.Now().Format("2006-01-02_1504")

	// Get strategies based on whether there's a mortgage
	strategies := GetStrategiesForConfig(config)

	// Apply config settings to strategies
	maximizeCoupleISA := config.Strategy.ShouldMaximizeCoupleISA()
	for i := range strategies {
		strategies[i].MaximizeCoupleISA = maximizeCoupleISA
	}

	strategyNames := make([]string, len(strategies))
	for i, s := range strategies {
		strategyNames[i] = s.ShortName()
	}

	// Initialize results matrix
	results := make([][]SensitivityResult, len(pensionRates))
	for i := range results {
		results[i] = make([]SensitivityResult, len(savingsRates))
	}

	// Run simulations for each combination
	for pi, pensionRate := range pensionRates {
		for si, savingsRate := range savingsRates {
			// Create a copy of config with modified growth rates
			testConfig := *config
			testConfig.Financial.PensionGrowthRate = pensionRate
			testConfig.Financial.SavingsGrowthRate = savingsRate

			// Run all strategies
			var bestIdx int = -1
			var bestBalance float64 = -1
			var longestYear int = 0
			var longestIdx int = 0

			var simResults []SimulationResult
			for _, params := range strategies {
				result := RunSimulation(params, &testConfig)
				simResults = append(simResults, result)

				// Track longest lasting
				if result.RanOutYear > longestYear {
					longestYear = result.RanOutYear
					longestIdx = len(simResults) - 1
				}

				// Find best (highest final balance among those that don't truly deplete)
				// "Shortfall" (RanOutOfMoney=true but still has significant balance) is acceptable
				finalBal := 0.0
				for _, bal := range result.FinalBalances {
					finalBal += bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot
				}
				// Accept if: not ran out, OR ran out but still has significant balance (shortfall not depletion)
				isAcceptable := !result.RanOutOfMoney || finalBal > 1000
				if isAcceptable {
					if bestBalance < 0 || finalBal > bestBalance {
						bestBalance = finalBal
						bestIdx = len(simResults) - 1
					}
				}
			}

			// If all truly depleted, use longest lasting
			allRunOut := bestIdx < 0
			if allRunOut {
				bestIdx = longestIdx
			}

			best := simResults[bestIdx]
			// Calculate total final balance
			finalBalance := 0.0
			for _, bal := range best.FinalBalances {
				finalBalance += bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot
			}

			// Determine if this is a "shortfall" scenario (has income gap but still has money)
			hasShortfall := best.RanOutOfMoney && finalBalance > 1000

			// Generate subdirectory name for this combination
			// Use math.Round to avoid floating point precision issues
			reportDir := fmt.Sprintf("p%02d_s%02d", int(math.Round(pensionRate*100)), int(math.Round(savingsRate*100)))

			results[pi][si] = SensitivityResult{
				PensionGrowth:   pensionRate,
				SavingsGrowth:   savingsRate,
				BestStrategy:    strategies[bestIdx].ShortName(),
				BestStrategyIdx: bestIdx,
				LastsUntil:      best.RanOutYear,
				TotalTax:        best.TotalTaxPaid,
				FinalBalance:    finalBalance,
				AllRunOut:       allRunOut,
				HasShortfall:    hasShortfall,
				AllResults:      simResults,
				ReportDir:       reportDir,
				SummaryFile:     filepath.Join(reportDir, "summary.html"),
			}
		}
	}

	// Create main output directory
	outputDir := fmt.Sprintf("sensitivity_%s", timestamp)

	return &SensitivityAnalysis{
		Results:            results,
		PensionGrowthRates: pensionRates,
		SavingsGrowthRates: savingsRates,
		StrategyNames:      strategyNames,
		Config:             config,
		Timestamp:          timestamp,
		OutputDir:          outputDir,
	}
}

// GenerateSensitivityReport generates the HTML sensitivity analysis report
func GenerateSensitivityReport(analysis *SensitivityAnalysis) (string, error) {
	// Create main output directory
	if err := os.MkdirAll(analysis.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate full detailed reports for each growth rate combination
	totalCombinations := len(analysis.PensionGrowthRates) * len(analysis.SavingsGrowthRates)

	for pi := range analysis.PensionGrowthRates {
		for si := range analysis.SavingsGrowthRates {
			result := &analysis.Results[pi][si]

			// Create subdirectory for this combination
			subDir := filepath.Join(analysis.OutputDir, result.ReportDir)

			// Create a modified config with the growth rates for this combination
			modifiedConfig := *analysis.Config
			modifiedConfig.Financial.PensionGrowthRate = result.PensionGrowth
			modifiedConfig.Financial.SavingsGrowthRate = result.SavingsGrowth

			// Generate full detailed reports using the standard HTML report generator
			GenerateAllHTMLReportsInDir(result.AllResults, &modifiedConfig, subDir, analysis.Timestamp)
		}
	}
	fmt.Printf("  Generated %d reports in %s/\n", totalCombinations, analysis.OutputDir)

	// Generate the main sensitivity matrix page
	filename := filepath.Join(analysis.OutputDir, "index.html")

	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Strategy colors for the heatmap
	strategyColors := map[string]string{
		"ISA→Pen/Early": "#e3f2fd", // Light blue
		"Pen→ISA/Early": "#e8f5e9", // Light green
		"TaxOpt/Early":  "#fff3e0", // Light orange
		"Pen+ISA/Early": "#f3e5f5", // Light purple
		"ISA→Pen/2031":  "#bbdefb", // Blue
		"Pen→ISA/2031":  "#c8e6c9", // Green
		"TaxOpt/2031":   "#ffe0b2", // Orange
		"Pen+ISA/2031":  "#e1bee7", // Purple
	}

	// Write HTML header
	fmt.Fprintf(f, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Pension Strategy Sensitivity Analysis</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0; padding: 20px;
            background: #f5f5f5;
        }
        .container { max-width: 1400px; margin: 0 auto; }
        h1 { color: #1a237e; margin-bottom: 10px; }
        h2 { color: #303f9f; margin-top: 30px; }
        .subtitle { color: #666; margin-bottom: 30px; }

        .config-summary {
            background: white;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 30px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .config-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
        }
        .config-item { }
        .config-label { font-size: 12px; color: #666; }
        .config-value { font-size: 16px; font-weight: 600; color: #333; }

        .matrix-container {
            background: white;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 30px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            overflow-x: auto;
        }

        .matrix {
            border-collapse: collapse;
            margin: 0 auto;
        }
        .matrix th, .matrix td {
            padding: 8px 12px;
            text-align: center;
            border: 1px solid #ddd;
            min-width: 90px;
        }
        .matrix th {
            background: #1a237e;
            color: white;
            font-weight: 600;
        }
        .matrix .row-header {
            background: #303f9f;
            color: white;
            font-weight: 600;
        }
        .matrix td {
            font-size: 11px;
            cursor: pointer;
            transition: transform 0.1s;
        }
        .matrix td:hover {
            transform: scale(1.05);
            box-shadow: 0 2px 8px rgba(0,0,0,0.2);
            z-index: 10;
            position: relative;
        }
        .matrix .strategy-name { font-weight: 600; }
        .matrix .year-info { color: #666; font-size: 10px; }
        .matrix .warning { color: #d32f2f; }

        .legend {
            display: flex;
            flex-wrap: wrap;
            gap: 15px;
            margin-bottom: 20px;
            padding: 15px;
            background: #fafafa;
            border-radius: 8px;
        }
        .legend-item {
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .legend-color {
            width: 24px;
            height: 24px;
            border-radius: 4px;
            border: 1px solid #ddd;
        }
        .legend-label { font-size: 13px; }

        .insights {
            background: white;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 30px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .insight-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
        }
        .insight-card {
            padding: 15px;
            background: #f8f9fa;
            border-radius: 8px;
            border-left: 4px solid #1a237e;
        }
        .insight-title { font-weight: 600; margin-bottom: 8px; color: #1a237e; }
        .insight-value { font-size: 24px; font-weight: 700; color: #333; }
        .insight-detail { font-size: 13px; color: #666; margin-top: 5px; }

        .quadrant-container {
            background: white;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 30px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .quadrant {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 2px;
            max-width: 600px;
            margin: 20px auto;
        }
        .quadrant-cell {
            padding: 30px 20px;
            text-align: center;
            border-radius: 4px;
        }
        .quadrant-title { font-weight: 600; margin-bottom: 10px; }
        .quadrant-strategy { font-size: 18px; font-weight: 700; }
        .quadrant-label {
            text-align: center;
            padding: 10px;
            font-weight: 600;
            color: #666;
        }

        .back-link {
            display: inline-block;
            margin-bottom: 20px;
            color: #1a237e;
            text-decoration: none;
        }
        .back-link:hover { text-decoration: underline; }

        .tax-matrix td {
            font-size: 12px;
        }
        .tax-good { background: #c8e6c9 !important; }
        .tax-medium { background: #fff9c4 !important; }
        .tax-high { background: #ffcdd2 !important; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Pension Strategy Sensitivity Analysis</h1>
        <p class="subtitle">How growth rates affect the optimal drawdown strategy</p>
`)

	// Configuration summary
	fmt.Fprintf(f, `
        <div class="config-summary">
            <h3 style="margin-top:0">Base Configuration</h3>
            <div class="config-grid">
                <div class="config-item">
                    <div class="config-label">Starting Assets</div>
                    <div class="config-value">%s</div>
                </div>
                <div class="config-item">
                    <div class="config-label">Income Need</div>
                    <div class="config-value">%s/month</div>
                </div>
                <div class="config-item">
                    <div class="config-label">Mortgage</div>
                    <div class="config-value">%s balance</div>
                </div>
                <div class="config-item">
                    <div class="config-label">Simulation Period</div>
                    <div class="config-value">%d to age %d</div>
                </div>
            </div>
        </div>
`,
		FormatMoney(getTotalAssets(analysis.Config)),
		FormatMoney(analysis.Config.IncomeRequirements.MonthlyBeforeAge),
		FormatMoney(analysis.Config.GetTotalPayoffAmount(analysis.Config.Mortgage.EndYear)),
		analysis.Config.Simulation.StartYear,
		analysis.Config.Simulation.EndAge)

	// Legend
	fmt.Fprintf(f, `
        <div class="matrix-container">
            <h2 style="margin-top:0">Best Strategy by Growth Rate</h2>
            <div class="legend">
`)
	for strategy, color := range strategyColors {
		fmt.Fprintf(f, `                <div class="legend-item">
                    <div class="legend-color" style="background: %s"></div>
                    <span class="legend-label">%s</span>
                </div>
`, color, strategy)
	}
	fmt.Fprintf(f, `            </div>
`)

	// Main strategy matrix
	fmt.Fprintf(f, `
            <p style="font-size: 13px; color: #666; margin-bottom: 15px;">Click any cell to see detailed breakdown for that growth rate combination.</p>
            <table class="matrix">
                <tr>
                    <th></th>
                    <th colspan="%d">Savings Growth Rate (ISA)</th>
                </tr>
                <tr>
                    <th>Pension Growth</th>
`, len(analysis.SavingsGrowthRates))

	for _, rate := range analysis.SavingsGrowthRates {
		fmt.Fprintf(f, "                    <th>%.0f%%</th>\n", rate*100)
	}
	fmt.Fprintf(f, "                </tr>\n")

	// Data rows
	for pi, pensionRate := range analysis.PensionGrowthRates {
		fmt.Fprintf(f, "                <tr>\n")
		fmt.Fprintf(f, "                    <td class=\"row-header\">%.0f%%</td>\n", pensionRate*100)

		for si := range analysis.SavingsGrowthRates {
			result := analysis.Results[pi][si]
			color := strategyColors[result.BestStrategy]
			if color == "" {
				color = "#ffffff"
			}

			yearInfo := ""
			if result.AllRunOut {
				yearInfo = fmt.Sprintf("<div class=\"year-info warning\">Runs out %d</div>", result.LastsUntil)
			} else {
				yearInfo = fmt.Sprintf("<div class=\"year-info\">Tax: %s</div>", FormatMoney(result.TotalTax))
			}

			fmt.Fprintf(f, `                    <td style="background: %s" title="Pension: %.0f%%, Savings: %.0f%% - Click for details" onclick="window.location='%s'">
                        <div class="strategy-name">%s</div>
                        %s
                    </td>
`, color, pensionRate*100, result.SavingsGrowth*100, result.SummaryFile, result.BestStrategy, yearInfo)
		}
		fmt.Fprintf(f, "                </tr>\n")
	}

	fmt.Fprintf(f, `            </table>
        </div>
`)

	// Quadrant summary
	writeQuadrantSummary(f, analysis)

	// Insights
	writeInsights(f, analysis)

	// Tax matrix
	writeTaxMatrix(f, analysis)

	// Footer
	fmt.Fprintf(f, `
        <p style="text-align: center; color: #666; margin-top: 40px;">
            Generated: %s
        </p>
    </div>
</body>
</html>
`, time.Now().Format("2 January 2006 15:04"))

	return filename, nil
}

func getTotalAssets(config *Config) float64 {
	total := 0.0
	for _, p := range config.People {
		total += p.TaxFreeSavings + p.Pension
	}
	return total
}

func writeQuadrantSummary(f *os.File, analysis *SensitivityAnalysis) {
	// Get corner results for quadrant
	pn := len(analysis.PensionGrowthRates)
	sn := len(analysis.SavingsGrowthRates)
	lowLow := analysis.Results[0][0]             // Low pension, low savings
	lowHigh := analysis.Results[0][sn-1]         // Low pension, high savings
	highLow := analysis.Results[pn-1][0]         // High pension, low savings
	highHigh := analysis.Results[pn-1][sn-1]     // High pension, high savings

	// Get the actual min/max rates for labels
	minPension := analysis.PensionGrowthRates[0] * 100
	maxPension := analysis.PensionGrowthRates[pn-1] * 100
	minSavings := analysis.SavingsGrowthRates[0] * 100
	maxSavings := analysis.SavingsGrowthRates[sn-1] * 100

	fmt.Fprintf(f, `
        <div class="quadrant-container">
            <h2 style="margin-top:0">Quadrant Summary</h2>
            <p>Best strategy at growth rate extremes:</p>

            <div style="display: grid; grid-template-columns: auto 1fr 1fr; gap: 2px; max-width: 500px; margin: 20px auto;">
                <div></div>
                <div class="quadrant-label">Low Savings (%.0f%%%%)</div>
                <div class="quadrant-label">High Savings (%.0f%%%%)</div>

                <div class="quadrant-label" style="writing-mode: vertical-rl; transform: rotate(180deg);">High Pension (%.0f%%%%)</div>
                <div class="quadrant-cell" style="background: #e8f5e9;">
                    <div class="quadrant-title">High Pension / Low Savings</div>
                    <div class="quadrant-strategy">%s</div>
                </div>
                <div class="quadrant-cell" style="background: #c8e6c9;">
                    <div class="quadrant-title">High Pension / High Savings</div>
                    <div class="quadrant-strategy">%s</div>
                </div>
`, minSavings, maxSavings, maxPension, highLow.BestStrategy, highHigh.BestStrategy)

	fmt.Fprintf(f, `
                <div class="quadrant-label" style="writing-mode: vertical-rl; transform: rotate(180deg);">Low Pension (%.0f%%%%)</div>
                <div class="quadrant-cell" style="background: #ffcdd2;">
                    <div class="quadrant-title">Low Pension / Low Savings</div>
                    <div class="quadrant-strategy">%s</div>
                </div>
                <div class="quadrant-cell" style="background: #fff9c4;">
                    <div class="quadrant-title">Low Pension / High Savings</div>
                    <div class="quadrant-strategy">%s</div>
                </div>
            </div>
        </div>
`, minPension, lowLow.BestStrategy, lowHigh.BestStrategy)
}

func writeInsights(f *os.File, analysis *SensitivityAnalysis) {
	// Count strategy wins
	strategyCounts := make(map[string]int)
	allRunOutCount := 0
	bestYear := 0
	worstYear := 9999

	for _, row := range analysis.Results {
		for _, result := range row {
			strategyCounts[result.BestStrategy]++
			if result.AllRunOut {
				allRunOutCount++
				if result.LastsUntil > bestYear {
					bestYear = result.LastsUntil
				}
				if result.LastsUntil < worstYear {
					worstYear = result.LastsUntil
				}
			}
		}
	}

	// Find most common strategy
	mostCommon := ""
	mostCount := 0
	for strategy, count := range strategyCounts {
		if count > mostCount {
			mostCount = count
			mostCommon = strategy
		}
	}

	totalScenarios := len(analysis.PensionGrowthRates) * len(analysis.SavingsGrowthRates)

	fmt.Fprintf(f, `
        <div class="insights">
            <h2 style="margin-top:0">Key Insights</h2>
            <div class="insight-grid">
                <div class="insight-card">
                    <div class="insight-title">Most Robust Strategy</div>
                    <div class="insight-value">%s</div>
                    <div class="insight-detail">Wins in %d of %d scenarios (%.0f%%)</div>
                </div>
                <div class="insight-card">
                    <div class="insight-title">Scenarios Where Money Runs Out</div>
                    <div class="insight-value">%d of %d</div>
                    <div class="insight-detail">%.0f%% of growth rate combinations</div>
                </div>
`, mostCommon, mostCount, totalScenarios, float64(mostCount)/float64(totalScenarios)*100,
		allRunOutCount, totalScenarios, float64(allRunOutCount)/float64(totalScenarios)*100)

	if allRunOutCount > 0 {
		fmt.Fprintf(f, `
                <div class="insight-card">
                    <div class="insight-title">When Money Runs Out</div>
                    <div class="insight-value">%d - %d</div>
                    <div class="insight-detail">Range of years across scenarios</div>
                </div>
`, worstYear, bestYear)
	}

	fmt.Fprintf(f, `
            </div>
        </div>
`)
}

func writeTaxMatrix(f *os.File, analysis *SensitivityAnalysis) {
	// Find min/max tax for color scaling
	minTax := analysis.Results[0][0].TotalTax
	maxTax := analysis.Results[0][0].TotalTax

	for _, row := range analysis.Results {
		for _, result := range row {
			if !result.AllRunOut {
				if result.TotalTax < minTax {
					minTax = result.TotalTax
				}
				if result.TotalTax > maxTax {
					maxTax = result.TotalTax
				}
			}
		}
	}

	fmt.Fprintf(f, `
        <div class="matrix-container">
            <h2 style="margin-top:0">Total Tax Paid by Growth Rate</h2>
            <p>Lower is better. Red indicates scenarios where money runs out.</p>

            <table class="matrix tax-matrix">
                <tr>
                    <th></th>
                    <th colspan="%d">Savings Growth Rate (ISA)</th>
                </tr>
                <tr>
                    <th>Pension Growth</th>
`, len(analysis.SavingsGrowthRates))

	for _, rate := range analysis.SavingsGrowthRates {
		fmt.Fprintf(f, "                    <th>%.0f%%</th>\n", rate*100)
	}
	fmt.Fprintf(f, "                </tr>\n")

	taxRange := maxTax - minTax
	if taxRange == 0 {
		taxRange = 1
	}

	for pi, pensionRate := range analysis.PensionGrowthRates {
		fmt.Fprintf(f, "                <tr>\n")
		fmt.Fprintf(f, "                    <td class=\"row-header\">%.0f%%</td>\n", pensionRate*100)

		for si := range analysis.SavingsGrowthRates {
			result := analysis.Results[pi][si]

			var cellClass string
			if result.AllRunOut {
				cellClass = "tax-high"
			} else {
				// Scale from green to yellow based on tax
				taxRatio := (result.TotalTax - minTax) / taxRange
				if taxRatio < 0.33 {
					cellClass = "tax-good"
				} else if taxRatio < 0.66 {
					cellClass = "tax-medium"
				} else {
					cellClass = "tax-high"
				}
			}

			displayValue := FormatMoney(result.TotalTax)
			if result.AllRunOut {
				displayValue = fmt.Sprintf("%s*", displayValue)
			}

			fmt.Fprintf(f, "                    <td class=\"%s\">%s</td>\n", cellClass, displayValue)
		}
		fmt.Fprintf(f, "                </tr>\n")
	}

	fmt.Fprintf(f, `            </table>
            <p style="font-size: 12px; color: #666; margin-top: 10px;">* Money runs out before end of simulation</p>
        </div>
`)
}
