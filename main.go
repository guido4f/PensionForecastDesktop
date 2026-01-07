package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func main() {
	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Pension Drawdown Tax Optimisation Simulator

Simulates different pension drawdown strategies to find the most tax-efficient
approach for retirement income. Compares 8 scenarios (4 strategies × 2 mortgage
payment options) and identifies the optimal approach.

MODES:
  This tool supports two main operating modes:

  FIXED INCOME MODE (default)
    You specify how much monthly income you need, and the simulator shows
    how long your funds will last with different drawdown strategies.
    - Set monthly_before_age and monthly_after_age in config
    - Best for: "Can I afford £X/month in retirement?"
    - Output: How long funds last, which strategy minimizes tax

  DEPLETION MODE (-depletion flag)
    You specify a target age to deplete funds, and the simulator calculates
    the maximum sustainable income that depletes funds by that age.
    - Set target_depletion_age and income_ratio_phase1/phase2 in config
    - Uses binary search to find optimal income level
    - Best for: "How much can I spend if I want funds to last until age X?"
    - Output: Maximum sustainable income, optimal drawdown strategy

  Both modes compare 8 strategy combinations:
    - 4 drawdown orders: ISA-first, Pension-first, Tax-optimized, Pension-to-ISA
    - 2 mortgage options: Early payoff vs Normal payoff

SENSITIVITY ANALYSIS (-sensitivity flag)
  Runs simulations across a range of growth rates (pension and savings) to show
  how results change under different market conditions. Requires sensitivity
  settings in config (pension_growth_min/max, savings_growth_min/max, step_size).

Usage:
  %s [options]

Options:
`, os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  %s                           Interactive mode selector
  %s -config my.yaml           Use custom configuration file
  %s -ui                       Embedded browser mode (webview window)
  %s -web                      Web server mode (opens external browser)
  %s -web -addr :8080          Web server on specific port

  Fixed Income Mode:
  %s -html                     Generate HTML reports (how long funds last)
  %s -details                  Show year-by-year console output
  %s -sensitivity              Sensitivity analysis across growth rates

  Depletion Mode:
  %s -depletion                Calculate sustainable income (console output)
  %s -depletion -html          Generate HTML reports with sustainable income
  %s -depletion -sensitivity   Sensitivity analysis (income vs growth rates)

Configuration:
  Edit config.yaml to customize people, assets, income needs, and growth rates.

  Key settings for Fixed Income Mode:
    income_requirements.monthly_before_age: £/month before age threshold
    income_requirements.monthly_after_age:  £/month after age threshold

  Key settings for Depletion Mode:
    income_requirements.target_depletion_age: Age to deplete funds by
    income_requirements.income_ratio_phase1:  Income ratio before threshold (e.g., 5)
    income_requirements.income_ratio_phase2:  Income ratio after threshold (e.g., 3)
    (Ratio 5:3 means phase1 income is 5/3 times phase2 income)

  Key settings for Sensitivity Analysis:
    sensitivity.pension_growth_min/max: Range for pension growth rates
    sensitivity.savings_growth_min/max: Range for ISA growth rates
    sensitivity.step_size: Increment between rates (e.g., 0.01 = 1%%)
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
	}

	// Command line flags
	configFile := flag.String("config", "config.yaml", "Path to YAML configuration file")
	showDetails := flag.Bool("details", false, "Show year-by-year breakdown for each strategy in console")
	showDrawdown := flag.Bool("drawdown", false, "Show detailed drawdown sources (ISA/pension) for best strategy")
	yearDetail := flag.Int("year", 0, "Show detailed breakdown for a specific year (e.g., -year 2030)")
	generateHTML := flag.Bool("html", false, "Generate interactive HTML reports in dated folder")
	runSensitivity := flag.Bool("sensitivity", false, "Run sensitivity analysis across pension/savings growth rates")
	runDepletion := flag.Bool("depletion", false, "Run depletion mode: calculate sustainable income to deplete by target_depletion_age")
	runPensionOnly := flag.Bool("pension-only", false, "Pension-only depletion: deplete pensions only, preserve ISAs")
	runPensionToISA := flag.Bool("pension-to-isa", false, "PensionToISA depletion: efficiently move excess pension to ISAs")
	consoleMode := flag.Bool("console", false, "Use console interface instead of GUI (default is GUI)")
	webMode := flag.Bool("web", false, "Start web server mode (opens external browser)")
	uiMode := flag.Bool("ui", false, "Start embedded browser mode (webview window)")
	webAddr := flag.String("addr", "localhost:0", "Web server address (for -web mode, use :0 for auto port)")
	flag.Parse()

	// Embedded browser mode
	if *uiMode {
		err := runEmbeddedUI(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Embedded UI error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Web server mode (external browser)
	if *webMode {
		config, err := LoadConfig(*configFile)
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}
		server := NewWebServer(config, *webAddr)
		if err := server.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Web server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Determine if we should run in console mode:
	// - Explicit -console flag, OR
	// - Any output/mode flags set (for automation/scripting)
	useConsole := *consoleMode || *runDepletion || *runSensitivity || *generateHTML ||
		*showDetails || *showDrawdown || *yearDetail > 0 || *runPensionOnly || *runPensionToISA

	if useConsole {
		runConsoleMode(*configFile, *showDetails, *showDrawdown, *yearDetail, *generateHTML,
			*runSensitivity, *runDepletion, *runPensionOnly, *runPensionToISA)
		return
	}

	// Default: GUI mode
	err := runGUI(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GUI error: %v\n", err)
		// Fall back to console mode if GUI fails
		fmt.Println("Falling back to console mode...")
		runConsoleMode(*configFile, *showDetails, *showDrawdown, *yearDetail, *generateHTML,
			*runSensitivity, *runDepletion, *runPensionOnly, *runPensionToISA)
	}
}

// runConsoleMode runs the application in console/terminal mode
func runConsoleMode(configFile string, showDetails, showDrawdown bool, yearDetail int,
	generateHTML, runSensitivity, runDepletion, runPensionOnly, runPensionToISA bool) {

	// Load configuration
	config, err := LoadConfig(configFile)
	configMissing := os.IsNotExist(err)

	if err != nil && !configMissing {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// If no specific mode flags set, ask user which mode they want
	if !runDepletion && !runSensitivity && !generateHTML && !showDetails && !showDrawdown && yearDetail == 0 && !runPensionOnly && !runPensionToISA {
		mode := promptForModeInitial(config, configMissing)
		switch mode {
		case "depletion":
			runDepletion = true
		case "depletion-html":
			runDepletion = true
			generateHTML = true
		case "depletion-sensitivity":
			runDepletion = true
			runSensitivity = true
		case "pension-only":
			runPensionOnly = true
		case "pension-only-html":
			runPensionOnly = true
			generateHTML = true
		case "pension-to-isa":
			runPensionToISA = true
		case "pension-to-isa-sensitivity":
			runPensionToISA = true
			runSensitivity = true
		case "fixed":
			// Default mode, continue
		case "fixed-html":
			generateHTML = true
		case "sensitivity":
			runSensitivity = true
		case "quit":
			fmt.Println("Goodbye!")
			return
		}
	}

	// If config is missing, build it interactively based on selected mode
	if configMissing {
		builder := NewInteractiveConfigBuilder()
		if runDepletion {
			config = builder.BuildDepletionConfig()
		} else {
			config = builder.BuildFixedIncomeConfig()
		}
		// Save the config
		err = builder.SaveConfig(configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\nConfiguration saved to %s\n", configFile)
		fmt.Println("You can edit this file to adjust settings for future runs.")
		fmt.Println()
	}

	// Check if pension-only depletion mode is enabled
	if runPensionOnly {
		// Validate depletion config
		missing := ValidateDepletionConfig(config)
		if len(missing) > 0 {
			config = promptForMissingDepletionFields(config, missing, configFile)
		}
		if config.IncomeRequirements.TargetDepletionAge <= 0 {
			fmt.Fprintf(os.Stderr, "Error: target_depletion_age must be set for pension-only depletion mode\n")
			os.Exit(1)
		}
		runPensionOnlyMode(config, showDetails, generateHTML)
		return
	}

	// Check if pension-to-ISA depletion mode is enabled
	if runPensionToISA {
		// Validate depletion config
		missing := ValidateDepletionConfig(config)
		if len(missing) > 0 {
			config = promptForMissingDepletionFields(config, missing, configFile)
		}
		if config.IncomeRequirements.TargetDepletionAge <= 0 {
			fmt.Fprintf(os.Stderr, "Error: target_depletion_age must be set for pension-to-ISA depletion mode\n")
			os.Exit(1)
		}
		if runSensitivity {
			// Validate and prompt for sensitivity config
			sensitivityMissing := ValidateSensitivityConfig(config)
			if len(sensitivityMissing) > 0 {
				config = promptForMissingSensitivityFields(config, sensitivityMissing, configFile)
			}
			runPensionToISASensitivity(config)
			return
		}
		runPensionToISAMode(config, showDetails, generateHTML)
		return
	}

	// Check if depletion mode is enabled via command line flag
	if runDepletion {
		// Validate depletion config
		missing := ValidateDepletionConfig(config)
		if len(missing) > 0 {
			config = promptForMissingDepletionFields(config, missing, configFile)
		}
		if config.IncomeRequirements.TargetDepletionAge <= 0 {
			fmt.Fprintf(os.Stderr, "Error: target_depletion_age must be set for depletion mode\n")
			os.Exit(1)
		}
		if runSensitivity {
			// Validate and prompt for sensitivity config when running sensitivity analysis
			sensitivityMissing := ValidateSensitivityConfig(config)
			if len(sensitivityMissing) > 0 {
				config = promptForMissingSensitivityFields(config, sensitivityMissing, configFile)
			}
			runDepletionSensitivity(config)
			return
		}
		runDepletionMode(config, showDetails, generateHTML)
		return
	}

	// Print header with configuration summary
	PrintHeader(config)

	// Get strategies based on whether there's a mortgage
	strategies := GetStrategiesForConfig(config)

	// Run all strategies
	if config.HasMortgage() {
		fmt.Println("Running 16 scenarios (4 strategies × 4 mortgage options)...")
		fmt.Println()
		annualPayment := config.GetTotalAnnualPayment()
		earlyPayoff := config.GetTotalPayoffAmount(config.Mortgage.EarlyPayoffYear)
		normalPayoff := config.GetTotalPayoffAmount(config.Mortgage.EndYear)
		extendedPayoff := config.GetTotalPayoffAmount(config.Mortgage.EndYear + 10)
		fmt.Println("  Mortgage Options:")
		fmt.Printf("    Early: Pay £%.0f/year until %d, then £%.0fk payoff\n",
			annualPayment, config.Mortgage.EarlyPayoffYear, earlyPayoff/1000)
		fmt.Printf("    Normal: Pay £%.0f/year until %d, then £%.0fk payoff\n",
			annualPayment, config.Mortgage.EndYear, normalPayoff/1000)
		fmt.Printf("    Extended: Pay £%.0f/year until %d, then £%.0fk payoff\n",
			annualPayment, config.Mortgage.EndYear+10, extendedPayoff/1000)
		fmt.Println()
	} else {
		fmt.Println("Running 4 scenarios (no mortgage)...")
		fmt.Println()
	}
	fmt.Println("  Drawdown Strategies:")
	fmt.Println("    1. Savings First (ISA → Pension)")
	fmt.Println("    2. Pension First (Pension → ISA)")
	fmt.Println("    3. Tax Optimized (minimize tax)")
	fmt.Println("    4. Pension to ISA (overdraw to fill tax bands)")
	fmt.Println()

	// Apply config settings to strategies
	maximizeCoupleISA := config.Strategy.ShouldMaximizeCoupleISA()
	for i := range strategies {
		strategies[i].MaximizeCoupleISA = maximizeCoupleISA
	}

	var results []SimulationResult
	for _, params := range strategies {
		result := RunSimulation(params, config)
		results = append(results, result)
	}

	// Print individual results if details requested
	if showDetails {
		for _, result := range results {
			PrintResultSummary(result, config)
		}
	}

	// Print comparison of all strategies
	PrintAllComparison(results)

	// Show detailed drawdown breakdown if requested
	if showDrawdown {
		// Find best strategy (highest final balance among those that don't run out)
		// This matches the criterion used in output.go and html_report.go
		bestIdx := -1
		bestBalance := -1.0
		for i, r := range results {
			if !r.RanOutOfMoney {
				finalBalance := getTotalFinalBalance(r)
				if bestBalance < 0 || finalBalance > bestBalance {
					bestBalance = finalBalance
					bestIdx = i
				}
			}
		}
		if bestIdx >= 0 {
			PrintDrawdownDetails(results[bestIdx], config)
		} else {
			// If all run out, show the one that lasts longest
			longestIdx := 0
			longestYear := 0
			for i, r := range results {
				if r.RanOutYear > longestYear {
					longestYear = r.RanOutYear
					longestIdx = i
				}
			}
			PrintDrawdownDetails(results[longestIdx], config)
		}
	}

	// Generate HTML reports if requested
	if generateHTML {
		timestamp := time.Now().Format("2006-01-02_1504")
		outputDir := fmt.Sprintf("reports_%s", timestamp)
		combinedReport, err := GenerateAllHTMLReportsInDir(results, config, outputDir, timestamp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating HTML reports: %v\n", err)
		} else {
			fmt.Printf("\nGenerated reports in %s/\n", outputDir)
			openBrowser(combinedReport)
		}
	}

	// Run sensitivity analysis if requested
	if runSensitivity {
		// Validate and prompt for sensitivity config
		sensitivityMissing := ValidateSensitivityConfig(config)
		if len(sensitivityMissing) > 0 {
			config = promptForMissingSensitivityFields(config, sensitivityMissing, configFile)
		}
		fmt.Printf("\nRunning sensitivity analysis (pension %.0f%%-%.0f%%, savings %.0f%%-%.0f%%)...\n",
			config.Sensitivity.PensionGrowthMin*100, config.Sensitivity.PensionGrowthMax*100,
			config.Sensitivity.SavingsGrowthMin*100, config.Sensitivity.SavingsGrowthMax*100)
		// Console mode defaults to maximizing final balance
		analysis := RunSensitivityAnalysis(config, OptimizeBalance)
		sensitivityReport, err := GenerateSensitivityReport(analysis)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating sensitivity report: %v\n", err)
		} else {
			openBrowser(sensitivityReport)
		}
	}

	// Show specific year detail if requested
	if yearDetail > 0 {
		for _, result := range results {
			for _, year := range result.Years {
				if year.Year == yearDetail {
					fmt.Printf("\n--- %s ---", result.Params.String())
					PrintDetailedYear(year, config)
					break
				}
			}
		}
	}
}

// openBrowser opens a file in the default browser
func openBrowser(filename string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", filename)
	case "darwin":
		cmd = exec.Command("open", filename)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", filename)
	default:
		fmt.Fprintf(os.Stderr, "Cannot open browser on %s\n", runtime.GOOS)
		return
	}

	err := cmd.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening browser: %v\n", err)
	}
}

// runDepletionMode runs the depletion mode simulation
func runDepletionMode(config *Config, showDetails bool, generateHTML bool) {
	PrintDepletionHeader(config)

	fmt.Println("Running depletion calculations for 8 strategies...")
	fmt.Println()

	results := RunAllDepletionCalculations(config)

	// Print comparison
	PrintDepletionComparison(results, config)

	// Show details if requested
	if showDetails {
		for _, r := range results {
			PrintResultSummary(r.SimulationResult, config)
		}
	}

	// Generate HTML reports if requested
	if generateHTML {
		timestamp := time.Now().Format("2006-01-02_1504")
		outputDir := fmt.Sprintf("reports_%s_depletion", timestamp)
		combinedReport, err := GenerateDepletionHTMLReports(results, config, outputDir, timestamp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating HTML reports: %v\n", err)
		} else {
			fmt.Printf("\nGenerated reports in %s/\n", outputDir)
			openBrowser(combinedReport)
		}
	}
}

// runDepletionSensitivity runs depletion mode across all growth rate combinations
func runDepletionSensitivity(config *Config) {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           DEPLETION MODE SENSITIVITY ANALYSIS                               ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	ic := config.IncomeRequirements
	refPerson := config.GetSimulationReferencePerson()
	fmt.Printf("Target: Deplete funds by age %d (%s)\n", ic.TargetDepletionAge, refPerson.Name)
	fmt.Printf("Income Ratio: %.0f:%.0f (before/after age %d)\n",
		ic.IncomeRatioPhase1, ic.IncomeRatioPhase2, ic.AgeThreshold)
	fmt.Printf("Growth rates: Pension %.0f%%-%.0f%%, Savings %.0f%%-%.0f%% (step %.0f%%)\n",
		config.Sensitivity.PensionGrowthMin*100, config.Sensitivity.PensionGrowthMax*100,
		config.Sensitivity.SavingsGrowthMin*100, config.Sensitivity.SavingsGrowthMax*100,
		config.Sensitivity.StepSize*100)
	fmt.Println()
	fmt.Println("Running sensitivity analysis...")

	analysis := RunDepletionSensitivityAnalysis(config)

	// Generate HTML report
	reportPath, err := GenerateDepletionSensitivityReport(analysis, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating report: %v\n", err)
		return
	}

	fmt.Printf("\nGenerated report: %s\n", reportPath)
	openBrowser(reportPath)
}

// promptForModeInitial asks the user which simulation mode they want to run
// Handles both cases: config exists and config is missing
func promptForModeInitial(config *Config, configMissing bool) string {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              PENSION DRAWDOWN TAX OPTIMISATION SIMULATOR                     ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	if configMissing {
		fmt.Println("No configuration file found. Select a mode to set up interactively:")
		fmt.Println()
		fmt.Println("  Fixed Income Mode (specify monthly income requirements):")
		fmt.Println("    1) Console output      - Run simulation with console output")
		fmt.Println("    2) HTML reports        - Generate interactive browser reports")
		fmt.Println("    3) Sensitivity         - Analyze across growth rate combinations")
		fmt.Println()
		fmt.Println("  Depletion Mode (calculate sustainable income to target age):")
		fmt.Println("    4) Console output      - Calculate sustainable income (all strategies)")
		fmt.Println("    5) HTML reports        - Generate interactive browser reports")
		fmt.Println("    6) Sensitivity         - Analyze across growth rate combinations")
		fmt.Println()
		fmt.Println("  Pension-Only Depletion (deplete pensions, preserve ISAs):")
		fmt.Println("    7) Console output      - Deplete pensions only, ISAs preserved")
		fmt.Println("    8) HTML reports        - Generate reports showing ISA preservation")
		fmt.Println()
		fmt.Println("  Pension-to-ISA Mode (efficiently move excess pension to ISAs):")
		fmt.Println("    9) Console output      - Move excess pension to ISAs")
		fmt.Println("    0) Sensitivity         - Analyze with ISA transfer optimization")
	} else {
		fmt.Println("Select simulation mode:")
		fmt.Println()
		// Calculate initial portfolio for percentage display
		initialPortfolio := 0.0
		for _, p := range config.People {
			initialPortfolio += p.Pension + p.TaxFreeSavings
		}

		if config.IncomeRequirements.HasTiers() {
			tierDesc := config.IncomeRequirements.DescribeTiers(initialPortfolio)
			fmt.Println("  Fixed Income Mode (tiered income):")
			fmt.Printf("    1) Console output      - %s\n", tierDesc)
		} else {
			fmt.Println("  Fixed Income Mode (uses monthly_before_age / monthly_after_age):")
			fmt.Printf("    1) Console output      - £%.0f/month before, £%.0f/month after age %d\n",
				config.IncomeRequirements.MonthlyBeforeAge,
				config.IncomeRequirements.MonthlyAfterAge,
				config.IncomeRequirements.AgeThreshold)
		}
		fmt.Println("    2) HTML reports        - Generate interactive browser reports")
		fmt.Println("    3) Sensitivity         - Analyze across growth rate combinations")
		fmt.Println()

		if config.IncomeRequirements.TargetDepletionAge > 0 {
			if config.IncomeRequirements.HasTiers() {
				fmt.Printf("  Depletion Mode (deplete by age %d, tiered ratios):\n",
					config.IncomeRequirements.TargetDepletionAge)
			} else {
				fmt.Printf("  Depletion Mode (deplete by age %d, ratio %.0f:%.0f):\n",
					config.IncomeRequirements.TargetDepletionAge,
					config.IncomeRequirements.IncomeRatioPhase1,
					config.IncomeRequirements.IncomeRatioPhase2)
			}
			fmt.Println("    4) Console output      - Calculate sustainable income (all strategies)")
			fmt.Println("    5) HTML reports        - Generate interactive browser reports")
			fmt.Println("    6) Sensitivity         - Analyze across growth rate combinations")
			fmt.Println()
			fmt.Println("  Pension-Only Depletion (deplete pensions, preserve ISAs):")
			fmt.Println("    7) Console output      - Deplete pensions only, ISAs preserved")
			fmt.Println("    8) HTML reports        - Generate reports showing ISA preservation")
			fmt.Println()
			fmt.Println("  Pension-to-ISA Mode (efficiently move excess pension to ISAs):")
			fmt.Println("    9) Console output      - Move excess pension to ISAs")
			fmt.Println("    0) Sensitivity         - Analyze with ISA transfer optimization")
		} else {
			fmt.Println("  Depletion Mode (will prompt for settings):")
			fmt.Println("    4) Console output      - Calculate sustainable income")
			fmt.Println("    5) HTML reports        - Generate interactive browser reports")
			fmt.Println("    6) Sensitivity         - Analyze across growth rate combinations")
			fmt.Println()
			fmt.Println("  Pension-Only Depletion (will prompt for settings):")
			fmt.Println("    7) Console output      - Deplete pensions only, ISAs preserved")
			fmt.Println("    8) HTML reports        - Generate reports showing ISA preservation")
			fmt.Println()
			fmt.Println("  Pension-to-ISA Mode (will prompt for settings):")
			fmt.Println("    9) Console output      - Move excess pension to ISAs")
			fmt.Println("    0) Sensitivity         - Analyze with ISA transfer optimization")
		}
	}
	fmt.Println()
	fmt.Println("    q) Quit")
	fmt.Println()
	fmt.Print("Enter choice (0-9 or q): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "fixed"
	}

	input = strings.TrimSpace(strings.ToLower(input))
	switch input {
	case "1":
		return "fixed"
	case "2":
		return "fixed-html"
	case "3":
		return "sensitivity"
	case "4":
		return "depletion"
	case "5":
		return "depletion-html"
	case "6":
		return "depletion-sensitivity"
	case "7":
		return "pension-only"
	case "8":
		return "pension-only-html"
	case "9":
		return "pension-to-isa"
	case "0":
		return "pension-to-isa-sensitivity"
	case "q", "quit", "exit":
		return "quit"
	default:
		fmt.Println("Invalid choice, running fixed income mode.")
		return "fixed"
	}
}

// promptForMissingDepletionFields prompts for any missing depletion mode fields
func promptForMissingDepletionFields(config *Config, missing []string, configFile string) *Config {
	fmt.Println()
	fmt.Println("Some required values for depletion mode are missing.")
	fmt.Println("Please provide the following (press Enter for defaults):")
	fmt.Println("For percentages, enter '5%' or '0.05'. For money, enter '100k' or '100000'.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for _, field := range missing {
		switch {
		case strings.Contains(field, "people") || field == "people":
			if len(config.People) == 0 {
				fmt.Println("─── Person 1 (Primary) ───")
				person := PersonConfig{
					Name:             promptStringSimple(reader, "  Name", "Person1"),
					BirthDate:        promptStringSimple(reader, "  Birth date (YYYY-MM-DD)", "1975-01-15"),
					RetirementAge:    promptIntSimple(reader, "  Stop work age (income starts)", 55),
					PensionAccessAge: promptIntSimple(reader, "  Pension access age (DC pension)", 55),
					StatePensionAge:  promptIntSimple(reader, "  State pension age", 67),
					Pension:          promptMoneySimple(reader, "  Pension pot value", 500000),
					TaxFreeSavings:   promptMoneySimple(reader, "  ISA/savings balance", 100000),
				}
				config.People = append(config.People, person)
				config.IncomeRequirements.ReferencePerson = person.Name
				config.Simulation.ReferencePerson = person.Name
			}
		case field == "target_depletion_age":
			config.IncomeRequirements.TargetDepletionAge = promptIntSimple(reader, "  Target age to deplete funds by", 85)
		case field == "income_ratio_phase1":
			config.IncomeRequirements.IncomeRatioPhase1 = promptFloatSimple(reader, "  Income ratio phase 1 (e.g., 5 for 5:3)", 5)
		case field == "income_ratio_phase2":
			config.IncomeRequirements.IncomeRatioPhase2 = promptFloatSimple(reader, "  Income ratio phase 2 (e.g., 3 for 5:3)", 3)
		}
	}

	// Ensure we have default tax bands if missing
	if len(config.TaxBands) == 0 {
		config.TaxBands = []TaxBand{
			{Name: "Personal Allowance", Lower: 0, Upper: 12570, Rate: 0.00},
			{Name: "Basic Rate", Lower: 12570, Upper: 50270, Rate: 0.20},
			{Name: "Higher Rate", Lower: 50270, Upper: 125140, Rate: 0.40},
			{Name: "Additional Rate", Lower: 125140, Upper: 10000000, Rate: 0.45},
		}
	}

	// Ensure financial defaults
	if config.Financial.PensionGrowthRate == 0 {
		config.Financial.PensionGrowthRate = 0.05
	}
	if config.Financial.SavingsGrowthRate == 0 {
		config.Financial.SavingsGrowthRate = 0.05
	}
	if config.Financial.StatePensionAmount == 0 {
		config.Financial.StatePensionAmount = 12547.60
	}
	if config.Financial.StatePensionInflation == 0 {
		config.Financial.StatePensionInflation = 0.03
	}

	// Ensure income requirements defaults
	if config.IncomeRequirements.AgeThreshold == 0 {
		config.IncomeRequirements.AgeThreshold = 67
	}
	if config.IncomeRequirements.ReferencePerson == "" && len(config.People) > 0 {
		config.IncomeRequirements.ReferencePerson = config.People[0].Name
	}

	// Ensure simulation defaults
	if config.Simulation.StartYear == 0 {
		config.Simulation.StartYear = 2026
	}
	if config.Simulation.EndAge == 0 {
		config.Simulation.EndAge = 95
	}
	if config.Simulation.ReferencePerson == "" && len(config.People) > 0 {
		config.Simulation.ReferencePerson = config.People[0].Name
	}

	// Save the updated config
	err := SaveConfig(config, configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not save config: %v\n", err)
	} else {
		fmt.Printf("\nConfiguration updated and saved to %s\n", configFile)
	}

	return config
}

// promptForMissingSensitivityFields prompts for any missing sensitivity analysis fields
func promptForMissingSensitivityFields(config *Config, missing []string, configFile string) *Config {
	fmt.Println()
	fmt.Println("─── Sensitivity Analysis Settings ───")
	fmt.Println("Some required values for sensitivity analysis are missing.")
	fmt.Println("For percentages, enter '5%' or '0.05'.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for _, field := range missing {
		switch field {
		case "sensitivity.pension_growth_min":
			config.Sensitivity.PensionGrowthMin = promptPercentSimple(reader, "  Pension growth min rate", 0.04)
		case "sensitivity.pension_growth_max":
			config.Sensitivity.PensionGrowthMax = promptPercentSimple(reader, "  Pension growth max rate", 0.12)
		case "sensitivity.savings_growth_min":
			config.Sensitivity.SavingsGrowthMin = promptPercentSimple(reader, "  Savings growth min rate", 0.04)
		case "sensitivity.savings_growth_max":
			config.Sensitivity.SavingsGrowthMax = promptPercentSimple(reader, "  Savings growth max rate", 0.12)
		case "sensitivity.step_size":
			config.Sensitivity.StepSize = promptPercentSimple(reader, "  Step size for analysis", 0.01)
		}
	}

	// Save the updated config
	err := SaveConfig(config, configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not save config: %v\n", err)
	} else {
		fmt.Printf("\nSensitivity settings saved to %s\n", configFile)
	}

	return config
}

// Simple prompt helpers for the missing fields prompts
func promptStringSimple(reader *bufio.Reader, prompt, defaultVal string) string {
	fmt.Printf("%s [%s]: ", prompt, defaultVal)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}

func promptIntSimple(reader *bufio.Reader, prompt string, defaultVal int) int {
	fmt.Printf("%s [%d]: ", prompt, defaultVal)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(input)
	if err != nil {
		return defaultVal
	}
	return val
}

func promptFloatSimple(reader *bufio.Reader, prompt string, defaultVal float64) float64 {
	fmt.Printf("%s [%.0f]: ", prompt, defaultVal)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return defaultVal
	}
	return val
}

func promptPercentSimple(reader *bufio.Reader, prompt string, defaultVal float64) float64 {
	fmt.Printf("%s [%.0f%%]: ", prompt, defaultVal*100)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	// Handle % suffix
	if strings.HasSuffix(input, "%") {
		input = strings.TrimSuffix(input, "%")
		val, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return defaultVal
		}
		return val / 100
	}
	// Assume decimal if no %
	val, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return defaultVal
	}
	// If value > 1, assume percentage
	if val > 1 {
		return val / 100
	}
	return val
}

func promptMoneySimple(reader *bufio.Reader, prompt string, defaultVal float64) float64 {
	defaultStr := fmt.Sprintf("£%.0fk", defaultVal/1000)
	if defaultVal < 1000 {
		defaultStr = fmt.Sprintf("£%.0f", defaultVal)
	}
	fmt.Printf("%s [%s]: ", prompt, defaultStr)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return defaultVal
	}
	// Handle k/m suffix
	multiplier := 1.0
	if strings.HasSuffix(input, "k") {
		multiplier = 1000
		input = strings.TrimSuffix(input, "k")
	} else if strings.HasSuffix(input, "m") {
		multiplier = 1000000
		input = strings.TrimSuffix(input, "m")
	}
	input = strings.TrimPrefix(input, "£")
	val, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return defaultVal
	}
	return val * multiplier
}

// ensureSensitivityDefaults fills in missing sensitivity values from default-config.yaml
func ensureSensitivityDefaults(config *Config) {
	// Load defaults from embedded default-config.yaml
	defaults, err := LoadDefaultConfig()
	if err != nil {
		// Fallback to hardcoded defaults matching default-config.yaml
		if config.Sensitivity.PensionGrowthMin == 0 {
			config.Sensitivity.PensionGrowthMin = 0.04 // 4%
		}
		if config.Sensitivity.PensionGrowthMax == 0 {
			config.Sensitivity.PensionGrowthMax = 0.12 // 12%
		}
		if config.Sensitivity.SavingsGrowthMin == 0 {
			config.Sensitivity.SavingsGrowthMin = 0.04 // 4%
		}
		if config.Sensitivity.SavingsGrowthMax == 0 {
			config.Sensitivity.SavingsGrowthMax = 0.12 // 12%
		}
		if config.Sensitivity.StepSize == 0 {
			config.Sensitivity.StepSize = 0.01 // 1%
		}
		return
	}

	// Use values from default-config.yaml
	if config.Sensitivity.PensionGrowthMin == 0 {
		config.Sensitivity.PensionGrowthMin = defaults.Sensitivity.PensionGrowthMin
	}
	if config.Sensitivity.PensionGrowthMax == 0 {
		config.Sensitivity.PensionGrowthMax = defaults.Sensitivity.PensionGrowthMax
	}
	if config.Sensitivity.SavingsGrowthMin == 0 {
		config.Sensitivity.SavingsGrowthMin = defaults.Sensitivity.SavingsGrowthMin
	}
	if config.Sensitivity.SavingsGrowthMax == 0 {
		config.Sensitivity.SavingsGrowthMax = defaults.Sensitivity.SavingsGrowthMax
	}
	if config.Sensitivity.StepSize == 0 {
		config.Sensitivity.StepSize = defaults.Sensitivity.StepSize
	}
}

// runPensionOnlyMode runs pension-only depletion mode (preserves ISAs)
func runPensionOnlyMode(config *Config, showDetails bool, generateHTML bool) {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           PENSION-ONLY DEPLETION MODE                                       ║")
	fmt.Println("║           (Depletes pensions only, ISAs are preserved)                      ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	ic := config.IncomeRequirements
	refPerson := config.GetSimulationReferencePerson()
	fmt.Printf("Target: Deplete PENSIONS ONLY by age %d (%s)\n", ic.TargetDepletionAge, refPerson.Name)
	fmt.Printf("ISAs will be preserved and continue to grow.\n")
	fmt.Printf("Income Ratio: %.0f:%.0f (before/after age %d)\n",
		ic.IncomeRatioPhase1, ic.IncomeRatioPhase2, ic.AgeThreshold)
	fmt.Println()
	fmt.Println("Running pension-only depletion calculations...")
	fmt.Println()

	results := RunPensionOnlyDepletionCalculations(config)

	// Print comparison
	PrintPensionOnlyDepletionComparison(results, config)

	// Show details if requested
	if showDetails {
		for _, r := range results {
			PrintResultSummary(r.SimulationResult, config)
		}
	}

	// Generate HTML reports if requested
	if generateHTML {
		timestamp := time.Now().Format("2006-01-02_1504")
		outputDir := fmt.Sprintf("reports_%s_pension_only", timestamp)
		combinedReport, err := GeneratePensionOnlyDepletionHTMLReports(results, config, outputDir, timestamp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating HTML reports: %v\n", err)
		} else {
			fmt.Printf("\nGenerated reports in %s/\n", outputDir)
			openBrowser(combinedReport)
		}
	}
}

// runPensionToISAMode runs PensionToISA depletion mode (efficiently moves pension to ISA)
func runPensionToISAMode(config *Config, showDetails bool, generateHTML bool) {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           PENSION-TO-ISA DEPLETION MODE                                     ║")
	fmt.Println("║           (Efficiently moves excess pension to ISAs)                        ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	ic := config.IncomeRequirements
	refPerson := config.GetSimulationReferencePerson()
	fmt.Printf("Target: Deplete funds by age %d (%s)\n", ic.TargetDepletionAge, refPerson.Name)
	fmt.Printf("Strategy: Over-draw pension to fill tax bands, move excess to ISA\n")
	fmt.Printf("Income Ratio: %.0f:%.0f (before/after age %d)\n",
		ic.IncomeRatioPhase1, ic.IncomeRatioPhase2, ic.AgeThreshold)
	fmt.Println()
	fmt.Println("Running PensionToISA depletion calculations...")
	fmt.Println()

	results := RunPensionToISADepletionCalculations(config)

	// Print comparison
	PrintPensionToISADepletionComparison(results, config)

	// Show details if requested
	if showDetails {
		for _, r := range results {
			PrintResultSummary(r.SimulationResult, config)
		}
	}

	// Generate HTML reports if requested
	if generateHTML {
		timestamp := time.Now().Format("2006-01-02_1504")
		outputDir := fmt.Sprintf("reports_%s_pension_to_isa", timestamp)
		combinedReport, err := GeneratePensionToISADepletionHTMLReports(results, config, outputDir, timestamp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating HTML reports: %v\n", err)
		} else {
			fmt.Printf("\nGenerated reports in %s/\n", outputDir)
			openBrowser(combinedReport)
		}
	}
}

// runPensionToISASensitivity runs PensionToISA sensitivity analysis
func runPensionToISASensitivity(config *Config) {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           PENSION-TO-ISA SENSITIVITY ANALYSIS                               ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	ic := config.IncomeRequirements
	refPerson := config.GetSimulationReferencePerson()
	fmt.Printf("Target: Deplete funds by age %d (%s)\n", ic.TargetDepletionAge, refPerson.Name)
	fmt.Printf("Strategy: PensionToISA (efficiently moves excess pension to ISAs)\n")
	fmt.Printf("Income Ratio: %.0f:%.0f (before/after age %d)\n",
		ic.IncomeRatioPhase1, ic.IncomeRatioPhase2, ic.AgeThreshold)
	fmt.Printf("Growth rates: Pension %.0f%%-%.0f%%, Savings %.0f%%-%.0f%% (step %.0f%%)\n",
		config.Sensitivity.PensionGrowthMin*100, config.Sensitivity.PensionGrowthMax*100,
		config.Sensitivity.SavingsGrowthMin*100, config.Sensitivity.SavingsGrowthMax*100,
		config.Sensitivity.StepSize*100)
	fmt.Println()
	fmt.Println("Running sensitivity analysis...")

	analysis := RunPensionToISASensitivityAnalysis(config)

	// Generate HTML report
	reportPath, err := GeneratePensionToISASensitivityReport(analysis, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating report: %v\n", err)
		return
	}

	fmt.Printf("\nGenerated report: %s\n", reportPath)
	openBrowser(reportPath)
}
