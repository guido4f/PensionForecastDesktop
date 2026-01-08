package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Validation errors
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

// validateDate checks if input is a valid YYYY-MM-DD date
func validateDate(input string) error {
	input = strings.TrimSpace(input)

	// Check format with regex
	dateRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	if !dateRegex.MatchString(input) {
		return ValidationError{Field: "date", Message: "Invalid format. Use YYYY-MM-DD (e.g., 1975-06-15)"}
	}

	// Parse and validate the actual date
	_, err := time.Parse("2006-01-02", input)
	if err != nil {
		return ValidationError{Field: "date", Message: "Invalid date. Check month (01-12) and day are valid"}
	}

	// Check year is reasonable (1900-2050)
	year, _ := strconv.Atoi(input[:4])
	if year < 1900 || year > 2050 {
		return ValidationError{Field: "date", Message: "Year must be between 1900 and 2050"}
	}

	return nil
}

// validateAge checks if age is reasonable (18-120)
func validateAge(age int, fieldName string) error {
	if age < 18 || age > 120 {
		return ValidationError{Field: fieldName, Message: fmt.Sprintf("Age must be between 18 and 120 (got %d)", age)}
	}
	return nil
}

// validateRetirementAge checks retirement age is reasonable (40-100)
func validateRetirementAge(age int) error {
	if age < 40 || age > 100 {
		return ValidationError{Field: "retirement_age", Message: fmt.Sprintf("Retirement age must be between 40 and 100 (got %d)", age)}
	}
	return nil
}

// validateYear checks if year is reasonable (2000-2100)
func validateYear(year int, fieldName string) error {
	if year < 2000 || year > 2100 {
		return ValidationError{Field: fieldName, Message: fmt.Sprintf("Year must be between 2000 and 2100 (got %d)", year)}
	}
	return nil
}

// validatePercent checks if rate is a valid percentage (0-100% as decimal 0.0-1.0)
func validatePercent(rate float64, fieldName string) error {
	if rate < 0 || rate > 1.0 {
		return ValidationError{Field: fieldName, Message: fmt.Sprintf("Rate must be between 0%% and 100%% (got %.1f%%)", rate*100)}
	}
	return nil
}

// validateMoney checks if amount is non-negative and reasonable
func validateMoney(amount float64, fieldName string) error {
	if amount < 0 {
		return ValidationError{Field: fieldName, Message: "Amount cannot be negative"}
	}
	if amount > 100000000 { // 100 million
		return ValidationError{Field: fieldName, Message: "Amount seems too large. Please check the value"}
	}
	return nil
}

// validateRatio checks if ratio is positive and reasonable
func validateRatio(ratio float64, fieldName string) error {
	if ratio <= 0 {
		return ValidationError{Field: fieldName, Message: "Ratio must be positive"}
	}
	if ratio > 100 {
		return ValidationError{Field: fieldName, Message: "Ratio seems too large. Please check the value"}
	}
	return nil
}

// InteractiveConfigBuilder handles interactive configuration creation
type InteractiveConfigBuilder struct {
	reader        *bufio.Reader
	config        *Config
	defaultConfig *Config
}

// NewInteractiveConfigBuilder creates a new builder
func NewInteractiveConfigBuilder() *InteractiveConfigBuilder {
	builder := &InteractiveConfigBuilder{
		reader: bufio.NewReader(os.Stdin),
		config: &Config{},
	}

	// Try to load defaults from default-config.yaml
	defaultConfig, err := LoadDefaultConfig()
	if err == nil {
		builder.defaultConfig = defaultConfig
	}

	return builder
}

// getDefault returns a default value from the default config, or the fallback
func (b *InteractiveConfigBuilder) getDefault(fieldPath string, fallback string) string {
	if b.defaultConfig != nil {
		val := GetDefaultValue(fieldPath, b.defaultConfig)
		if val != "" {
			return val
		}
	}
	return fallback
}

// getDefaultInt gets an int default from default config
func (b *InteractiveConfigBuilder) getDefaultInt(fieldPath string, fallback int) int {
	val := b.getDefault(fieldPath, "")
	if val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

// getDefaultFloat gets a float default from default config
func (b *InteractiveConfigBuilder) getDefaultFloat(fieldPath string, fallback float64) float64 {
	val := b.getDefault(fieldPath, "")
	if val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return fallback
}

// getDefaultMoney gets a money default from default config (handles "100k" format)
func (b *InteractiveConfigBuilder) getDefaultMoney(fieldPath string, fallback float64) float64 {
	val := b.getDefault(fieldPath, "")
	if val != "" {
		return parseMoney(val, fallback)
	}
	return fallback
}

// getDefaultPercent gets a percent default from default config (handles "5%" format)
func (b *InteractiveConfigBuilder) getDefaultPercent(fieldPath string, fallback float64) float64 {
	val := b.getDefault(fieldPath, "")
	if val != "" {
		if p, err := parsePercentOrDecimal(val); err == nil {
			return p
		}
	}
	return fallback
}

// parseMoney parses money strings like "100k", "1m", "100000"
func parseMoney(input string, fallback float64) float64 {
	input = strings.TrimSpace(strings.ToLower(input))
	input = strings.TrimPrefix(input, "£")
	multiplier := 1.0
	if strings.HasSuffix(input, "k") {
		multiplier = 1000
		input = strings.TrimSuffix(input, "k")
	} else if strings.HasSuffix(input, "m") {
		multiplier = 1000000
		input = strings.TrimSuffix(input, "m")
	}
	val, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return fallback
	}
	return val * multiplier
}

// parsePercentOrDecimal converts "5%" or "0.05" to 0.05
func parsePercentOrDecimal(input string) (float64, error) {
	input = strings.TrimSpace(input)
	if strings.HasSuffix(input, "%") {
		// Remove % and convert
		numStr := strings.TrimSuffix(input, "%")
		num, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, err
		}
		return num / 100.0, nil
	}
	// Try parsing as decimal
	return strconv.ParseFloat(input, 64)
}

// promptString asks for a string with a default value
func (b *InteractiveConfigBuilder) promptString(prompt, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	input, _ := b.reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}

// promptDate asks for a date with validation (YYYY-MM-DD format)
func (b *InteractiveConfigBuilder) promptDate(prompt, defaultVal string) string {
	for {
		fmt.Printf("%s [%s]: ", prompt, defaultVal)
		input, _ := b.reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return defaultVal
		}
		if err := validateDate(input); err != nil {
			fmt.Printf("  ✗ %s\n", err.Error())
			continue
		}
		return input
	}
}

// calculateDefaultRetirementDate calculates a retirement date based on birth date and target age
func calculateDefaultRetirementDate(birthDate string, targetAge int) string {
	t, err := time.Parse("2006-01-02", birthDate)
	if err != nil {
		// Fallback to a reasonable default
		return fmt.Sprintf("%d-07-01", time.Now().Year()+5)
	}
	retirementYear := t.Year() + targetAge
	return fmt.Sprintf("%d-%02d-%02d", retirementYear, t.Month(), t.Day())
}

// promptRetirementDate asks for a retirement date with validation
func (b *InteractiveConfigBuilder) promptRetirementDate(prompt, birthDate string, defaultAge int) string {
	defaultDate := calculateDefaultRetirementDate(birthDate, defaultAge)
	for {
		fmt.Printf("%s [%s]: ", prompt, defaultDate)
		input, _ := b.reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return defaultDate
		}
		if err := validateDate(input); err != nil {
			fmt.Printf("  ✗ %s\n", err.Error())
			continue
		}
		// Validate the retirement date is in the future and reasonable
		retDate, _ := time.Parse("2006-01-02", input)
		if retDate.Before(time.Now()) {
			fmt.Printf("  ✗ Retirement date should be in the future\n")
			continue
		}
		return input
	}
}

// promptInt asks for an integer with a default value
func (b *InteractiveConfigBuilder) promptInt(prompt string, defaultVal int) int {
	fmt.Printf("%s [%d]: ", prompt, defaultVal)
	input, _ := b.reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(input)
	if err != nil {
		fmt.Printf("  ✗ Invalid number, using default: %d\n", defaultVal)
		return defaultVal
	}
	return val
}

// promptAge asks for an age with validation (18-120)
func (b *InteractiveConfigBuilder) promptAge(prompt string, defaultVal int) int {
	for {
		fmt.Printf("%s [%d]: ", prompt, defaultVal)
		input, _ := b.reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return defaultVal
		}
		val, err := strconv.Atoi(input)
		if err != nil {
			fmt.Printf("  ✗ Invalid number. Please enter a whole number\n")
			continue
		}
		if err := validateAge(val, "age"); err != nil {
			fmt.Printf("  ✗ %s\n", err.Error())
			continue
		}
		return val
	}
}

// promptRetirementAge asks for retirement age with validation (40-100)
func (b *InteractiveConfigBuilder) promptRetirementAge(prompt string, defaultVal int) int {
	for {
		fmt.Printf("%s [%d]: ", prompt, defaultVal)
		input, _ := b.reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return defaultVal
		}
		val, err := strconv.Atoi(input)
		if err != nil {
			fmt.Printf("  ✗ Invalid number. Please enter a whole number\n")
			continue
		}
		if err := validateRetirementAge(val); err != nil {
			fmt.Printf("  ✗ %s\n", err.Error())
			continue
		}
		return val
	}
}

// promptYear asks for a year with validation (2000-2100)
func (b *InteractiveConfigBuilder) promptYear(prompt string, defaultVal int) int {
	for {
		fmt.Printf("%s [%d]: ", prompt, defaultVal)
		input, _ := b.reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return defaultVal
		}
		val, err := strconv.Atoi(input)
		if err != nil {
			fmt.Printf("  ✗ Invalid number. Please enter a 4-digit year\n")
			continue
		}
		if err := validateYear(val, "year"); err != nil {
			fmt.Printf("  ✗ %s\n", err.Error())
			continue
		}
		return val
	}
}

// promptFloat asks for a float with a default value
func (b *InteractiveConfigBuilder) promptFloat(prompt string, defaultVal float64) float64 {
	fmt.Printf("%s [%.0f]: ", prompt, defaultVal)
	input, _ := b.reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(input, 64)
	if err != nil {
		fmt.Printf("  ✗ Invalid number, using default: %.0f\n", defaultVal)
		return defaultVal
	}
	return val
}

// promptRatio asks for a ratio with validation (positive, reasonable)
func (b *InteractiveConfigBuilder) promptRatio(prompt string, defaultVal float64) float64 {
	for {
		fmt.Printf("%s [%.0f]: ", prompt, defaultVal)
		input, _ := b.reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return defaultVal
		}
		val, err := strconv.ParseFloat(input, 64)
		if err != nil {
			fmt.Printf("  ✗ Invalid number. Please enter a number\n")
			continue
		}
		if err := validateRatio(val, "ratio"); err != nil {
			fmt.Printf("  ✗ %s\n", err.Error())
			continue
		}
		return val
	}
}

// promptPercent asks for a percentage with validation (accepts "5%" or "0.05")
func (b *InteractiveConfigBuilder) promptPercent(prompt string, defaultVal float64) float64 {
	for {
		fmt.Printf("%s [%.0f%%]: ", prompt, defaultVal*100)
		input, _ := b.reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			return defaultVal
		}
		val, err := parsePercentOrDecimal(input)
		if err != nil {
			fmt.Printf("  ✗ Invalid percentage. Enter as '5%%' or '0.05'\n")
			continue
		}
		if err := validatePercent(val, "rate"); err != nil {
			fmt.Printf("  ✗ %s\n", err.Error())
			continue
		}
		return val
	}
}

// promptMoney asks for a money amount with validation (accepts "100k" or "100000")
func (b *InteractiveConfigBuilder) promptMoney(prompt string, defaultVal float64) float64 {
	defaultStr := formatMoneyShort(defaultVal)
	for {
		fmt.Printf("%s [%s]: ", prompt, defaultStr)
		input, _ := b.reader.ReadString('\n')
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
		// Remove £ if present
		input = strings.TrimPrefix(input, "£")
		val, err := strconv.ParseFloat(input, 64)
		if err != nil {
			fmt.Printf("  ✗ Invalid amount. Enter as '100k', '1.5m', or '100000'\n")
			continue
		}
		amount := val * multiplier
		if err := validateMoney(amount, "amount"); err != nil {
			fmt.Printf("  ✗ %s\n", err.Error())
			continue
		}
		return amount
	}
}

func formatMoneyShort(amount float64) string {
	if amount >= 1000000 {
		return fmt.Sprintf("£%.1fm", amount/1000000)
	} else if amount >= 1000 {
		return fmt.Sprintf("£%.0fk", amount/1000)
	}
	return fmt.Sprintf("£%.0f", amount)
}

// BuildDepletionConfig builds a config with only depletion-required fields
func (b *InteractiveConfigBuilder) BuildDepletionConfig() *Config {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              DEPLETION MODE CONFIGURATION                                     ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	if b.defaultConfig != nil {
		fmt.Println("Defaults loaded from default-config.yaml. Press Enter to accept defaults.")
	} else {
		fmt.Println("Let's set up your pension forecast. Press Enter to accept defaults.")
	}
	fmt.Println("For percentages, enter '5%' or '0.05'. For money, enter '100k' or '100000'.")
	fmt.Println()

	// Person 1 (required)
	fmt.Println("─── Person 1 (Primary) ───")
	person1Name := b.promptString("  Name", b.getDefault("person.name", "Person1"))
	person1BirthDate := b.promptDate("  Birth date (YYYY-MM-DD)", b.getDefault("person.birth_date", "1975-01-15"))
	person1 := PersonConfig{
		Name:             person1Name,
		BirthDate:        person1BirthDate,
		RetirementDate:   b.promptRetirementDate("  Stop work date (YYYY-MM-DD)", person1BirthDate, b.getDefaultInt("person.retirement_age", 55)),
		PensionAccessAge: b.promptAge("  Pension access age (DC pension)", b.getDefaultInt("person.pension_access_age", 55)),
		StatePensionAge:  b.promptAge("  State pension age", b.getDefaultInt("person.state_pension_age", 67)),
		Pension:          b.promptMoney("  Pension pot value", b.getDefaultMoney("person.pension", 500000)),
		TaxFreeSavings:   b.promptMoney("  ISA/savings balance", b.getDefaultMoney("person.tax_free_savings", 100000)),
	}
	// Check for DB pension for Person 1
	hasDB1 := b.promptString("  Has defined benefit pension (e.g., Teachers)? (y/n)", "n")
	if strings.ToLower(hasDB1) == "y" || strings.ToLower(hasDB1) == "yes" {
		person1.DBPensionName = b.promptString("    DB pension name", b.getDefault("person.db_pension_name", "DB Pension"))
		person1.DBPensionAmount = b.promptMoney("    Annual DB pension amount", b.getDefaultMoney("person.db_pension_amount", 5000))
		person1.DBPensionStartAge = b.promptAge("    DB pension start age", b.getDefaultInt("person.db_pension_start_age", 67))
	}
	// Check for current employment (work income before retirement)
	hasWork1 := b.promptString("  Currently employed? (y/n)", "n")
	if strings.ToLower(hasWork1) == "y" || strings.ToLower(hasWork1) == "yes" {
		person1.WorkIncome = b.promptMoney("    Annual salary (gross)", b.getDefaultMoney("person.work_income", 50000))
	}
	b.config.People = append(b.config.People, person1)

	// Person 2 (optional)
	fmt.Println()
	addPerson2 := b.promptString("─── Add a second person? (y/n)", "n")
	if strings.ToLower(addPerson2) == "y" || strings.ToLower(addPerson2) == "yes" {
		person2Name := b.promptString("  Name", b.getDefault("person2.name", "Person2"))
		person2BirthDate := b.promptDate("  Birth date (YYYY-MM-DD)", b.getDefault("person2.birth_date", "1977-06-20"))
		person2 := PersonConfig{
			Name:             person2Name,
			BirthDate:        person2BirthDate,
			RetirementDate:   b.promptRetirementDate("  Stop work date (YYYY-MM-DD)", person2BirthDate, b.getDefaultInt("person2.retirement_age", 57)),
			PensionAccessAge: b.promptAge("  Pension access age (DC pension)", b.getDefaultInt("person2.pension_access_age", 57)),
			StatePensionAge:  b.promptAge("  State pension age", b.getDefaultInt("person2.state_pension_age", 67)),
			Pension:          b.promptMoney("  Pension pot value", b.getDefaultMoney("person2.pension", 100000)),
			TaxFreeSavings:   b.promptMoney("  ISA/savings balance", b.getDefaultMoney("person2.tax_free_savings", 50000)),
		}
		// Check for DB pension
		hasDB := b.promptString("  Has defined benefit pension (e.g., Teachers)? (y/n)", "n")
		if strings.ToLower(hasDB) == "y" || strings.ToLower(hasDB) == "yes" {
			person2.DBPensionName = b.promptString("    DB pension name", b.getDefault("person2.db_pension_name", "Teachers Pension"))
			person2.DBPensionAmount = b.promptMoney("    Annual DB pension amount", b.getDefaultMoney("person2.db_pension_amount", 5000))
			person2.DBPensionStartAge = b.promptAge("    DB pension start age", b.getDefaultInt("person2.db_pension_start_age", 67))
		}
		// Check for current employment (work income before retirement)
		hasWork2 := b.promptString("  Currently employed? (y/n)", "n")
		if strings.ToLower(hasWork2) == "y" || strings.ToLower(hasWork2) == "yes" {
			person2.WorkIncome = b.promptMoney("    Annual salary (gross)", b.getDefaultMoney("person2.work_income", 40000))
		}
		b.config.People = append(b.config.People, person2)
	}

	// Depletion settings
	fmt.Println()
	fmt.Println("─── Depletion Settings ───")
	b.config.IncomeRequirements.TargetDepletionAge = b.promptAge("  Target age to deplete funds by", b.getDefaultInt("income.target_depletion_age", 85))
	b.config.IncomeRequirements.IncomeRatioPhase1 = b.promptRatio("  Income ratio phase 1 (e.g., 5 for 5:3)", b.getDefaultFloat("income.income_ratio_phase1", 5))
	b.config.IncomeRequirements.IncomeRatioPhase2 = b.promptRatio("  Income ratio phase 2 (e.g., 3 for 5:3)", b.getDefaultFloat("income.income_ratio_phase2", 3))
	b.config.IncomeRequirements.AgeThreshold = b.promptAge("  Age when income changes (phase 1 → 2)", b.getDefaultInt("income.age_threshold", 67))
	b.config.IncomeRequirements.ReferencePerson = person1.Name

	// Also set fixed income defaults (in case user switches modes)
	b.config.IncomeRequirements.MonthlyBeforeAge = b.getDefaultMoney("income.monthly_before_age", 4000)
	b.config.IncomeRequirements.MonthlyAfterAge = b.getDefaultMoney("income.monthly_after_age", 2500)

	// Growth rates
	fmt.Println()
	fmt.Println("─── Growth Rates ───")
	b.config.Financial.PensionGrowthRate = b.promptPercent("  Pension growth rate", b.getDefaultPercent("financial.pension_growth_rate", 0.05))
	b.config.Financial.SavingsGrowthRate = b.promptPercent("  Savings/ISA growth rate", b.getDefaultPercent("financial.savings_growth_rate", 0.05))
	b.config.Financial.StatePensionAmount = b.promptMoney("  Annual state pension (per person)", b.getDefaultMoney("financial.state_pension_amount", 12547.60))
	b.config.Financial.StatePensionInflation = b.promptPercent("  State pension inflation", b.getDefaultPercent("financial.state_pension_inflation", 0.03))
	b.config.Financial.IncomeInflationRate = 0.03 // Not used in depletion mode
	b.config.Financial.TaxBandInflation = 0.03

	// Mortgage (optional)
	fmt.Println()
	hasMortgage := b.promptString("─── Do you have a mortgage? (y/n)", "n")
	if strings.ToLower(hasMortgage) == "y" || strings.ToLower(hasMortgage) == "yes" {
		b.config.Mortgage.EndYear = b.promptYear("  Mortgage end year", b.getDefaultInt("mortgage.end_year", 2035))
		b.config.Mortgage.EarlyPayoffYear = b.promptYear("  Earliest payoff year (if tied in)", b.getDefaultInt("mortgage.early_payoff_year", 2030))

		// Add mortgage parts in a loop
		partNum := 1
		for {
			fmt.Printf("\n  ─── Mortgage Part %d ───\n", partNum)
			part := MortgagePartConfig{
				Name:         b.promptString("  Mortgage name", fmt.Sprintf("Mortgage %d", partNum)),
				Principal:    b.promptMoney("  Outstanding balance", b.getDefaultMoney("mortgage.principal", 200000)),
				InterestRate: b.promptPercent("  Interest rate", b.getDefaultPercent("mortgage.interest_rate", 0.04)),
				IsRepayment:  strings.ToLower(b.promptString("  Is repayment mortgage? (y/n)", "y")) == "y",
				StartYear:    b.promptYear("  Start year", b.getDefaultInt("mortgage.start_year", 2020)),
				TermYears:    b.promptInt("  Term in years", b.getDefaultInt("mortgage.term_years", 25)),
			}
			b.config.Mortgage.Parts = append(b.config.Mortgage.Parts, part)

			addAnother := b.promptString("  Add another mortgage part? (y/n)", "n")
			if strings.ToLower(addAnother) != "y" && strings.ToLower(addAnother) != "yes" {
				break
			}
			partNum++
		}
	} else {
		// Set dummy mortgage values to avoid nil issues
		b.config.Mortgage.EndYear = 2025
		b.config.Mortgage.EarlyPayoffYear = 2025
	}

	// Simulation settings
	fmt.Println()
	fmt.Println("─── Simulation Settings ───")
	b.config.Simulation.StartYear = b.promptYear("  Start year", b.getDefaultInt("simulation.start_year", 2026))
	b.config.Simulation.EndAge = b.promptAge("  End age for simulation", b.getDefaultInt("simulation.end_age", 95))
	b.config.Simulation.ReferencePerson = person1.Name

	// Sensitivity analysis settings
	fmt.Println()
	fmt.Println("─── Sensitivity Analysis ───")
	b.config.Sensitivity.PensionGrowthMin = b.promptPercent("  Pension growth min rate", b.getDefaultPercent("sensitivity.pension_growth_min", 0.04))
	b.config.Sensitivity.PensionGrowthMax = b.promptPercent("  Pension growth max rate", b.getDefaultPercent("sensitivity.pension_growth_max", 0.12))
	b.config.Sensitivity.SavingsGrowthMin = b.promptPercent("  Savings growth min rate", b.getDefaultPercent("sensitivity.savings_growth_min", 0.04))
	b.config.Sensitivity.SavingsGrowthMax = b.promptPercent("  Savings growth max rate", b.getDefaultPercent("sensitivity.savings_growth_max", 0.12))
	b.config.Sensitivity.StepSize = b.promptPercent("  Step size for analysis", b.getDefaultPercent("sensitivity.step_size", 0.01))

	// Tax bands (use UK defaults)
	b.config.TaxBands = []TaxBand{
		{Name: "Personal Allowance", Lower: 0, Upper: 12570, Rate: 0.00},
		{Name: "Basic Rate", Lower: 12570, Upper: 50270, Rate: 0.20},
		{Name: "Higher Rate", Lower: 50270, Upper: 125140, Rate: 0.40},
		{Name: "Additional Rate", Lower: 125140, Upper: 10000000, Rate: 0.45},
	}

	return b.config
}

// BuildFixedIncomeConfig builds a config with only fixed-income-required fields
func (b *InteractiveConfigBuilder) BuildFixedIncomeConfig() *Config {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              FIXED INCOME MODE CONFIGURATION                                  ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	if b.defaultConfig != nil {
		fmt.Println("Defaults loaded from default-config.yaml. Press Enter to accept defaults.")
	} else {
		fmt.Println("Let's set up your pension forecast. Press Enter to accept defaults.")
	}
	fmt.Println("For percentages, enter '5%' or '0.05'. For money, enter '100k' or '100000'.")
	fmt.Println()

	// Person 1 (required)
	fmt.Println("─── Person 1 (Primary) ───")
	person1Name := b.promptString("  Name", b.getDefault("person.name", "Person1"))
	person1BirthDate := b.promptDate("  Birth date (YYYY-MM-DD)", b.getDefault("person.birth_date", "1975-01-15"))
	person1 := PersonConfig{
		Name:             person1Name,
		BirthDate:        person1BirthDate,
		RetirementDate:   b.promptRetirementDate("  Stop work date (YYYY-MM-DD)", person1BirthDate, b.getDefaultInt("person.retirement_age", 55)),
		PensionAccessAge: b.promptAge("  Pension access age (DC pension)", b.getDefaultInt("person.pension_access_age", 55)),
		StatePensionAge:  b.promptAge("  State pension age", b.getDefaultInt("person.state_pension_age", 67)),
		Pension:          b.promptMoney("  Pension pot value", b.getDefaultMoney("person.pension", 500000)),
		TaxFreeSavings:   b.promptMoney("  ISA/savings balance", b.getDefaultMoney("person.tax_free_savings", 100000)),
	}
	// Check for DB pension for Person 1
	hasDB1 := b.promptString("  Has defined benefit pension (e.g., Teachers)? (y/n)", "n")
	if strings.ToLower(hasDB1) == "y" || strings.ToLower(hasDB1) == "yes" {
		person1.DBPensionName = b.promptString("    DB pension name", b.getDefault("person.db_pension_name", "DB Pension"))
		person1.DBPensionAmount = b.promptMoney("    Annual DB pension amount", b.getDefaultMoney("person.db_pension_amount", 5000))
		person1.DBPensionStartAge = b.promptAge("    DB pension start age", b.getDefaultInt("person.db_pension_start_age", 67))
	}
	// Check for current employment (work income before retirement)
	hasWork1 := b.promptString("  Currently employed? (y/n)", "n")
	if strings.ToLower(hasWork1) == "y" || strings.ToLower(hasWork1) == "yes" {
		person1.WorkIncome = b.promptMoney("    Annual salary (gross)", b.getDefaultMoney("person.work_income", 50000))
	}
	b.config.People = append(b.config.People, person1)

	// Person 2 (optional)
	fmt.Println()
	addPerson2 := b.promptString("─── Add a second person? (y/n)", "n")
	if strings.ToLower(addPerson2) == "y" || strings.ToLower(addPerson2) == "yes" {
		person2Name := b.promptString("  Name", b.getDefault("person2.name", "Person2"))
		person2BirthDate := b.promptDate("  Birth date (YYYY-MM-DD)", b.getDefault("person2.birth_date", "1977-06-20"))
		person2 := PersonConfig{
			Name:             person2Name,
			BirthDate:        person2BirthDate,
			RetirementDate:   b.promptRetirementDate("  Stop work date (YYYY-MM-DD)", person2BirthDate, b.getDefaultInt("person2.retirement_age", 57)),
			PensionAccessAge: b.promptAge("  Pension access age (DC pension)", b.getDefaultInt("person2.pension_access_age", 57)),
			StatePensionAge:  b.promptAge("  State pension age", b.getDefaultInt("person2.state_pension_age", 67)),
			Pension:          b.promptMoney("  Pension pot value", b.getDefaultMoney("person2.pension", 100000)),
			TaxFreeSavings:   b.promptMoney("  ISA/savings balance", b.getDefaultMoney("person2.tax_free_savings", 50000)),
		}
		// Check for DB pension
		hasDB := b.promptString("  Has defined benefit pension (e.g., Teachers)? (y/n)", "n")
		if strings.ToLower(hasDB) == "y" || strings.ToLower(hasDB) == "yes" {
			person2.DBPensionName = b.promptString("    DB pension name", b.getDefault("person2.db_pension_name", "Teachers Pension"))
			person2.DBPensionAmount = b.promptMoney("    Annual DB pension amount", b.getDefaultMoney("person2.db_pension_amount", 5000))
			person2.DBPensionStartAge = b.promptAge("    DB pension start age", b.getDefaultInt("person2.db_pension_start_age", 67))
		}
		// Check for current employment (work income before retirement)
		hasWork2 := b.promptString("  Currently employed? (y/n)", "n")
		if strings.ToLower(hasWork2) == "y" || strings.ToLower(hasWork2) == "yes" {
			person2.WorkIncome = b.promptMoney("    Annual salary (gross)", b.getDefaultMoney("person2.work_income", 40000))
		}
		b.config.People = append(b.config.People, person2)
	}

	// Income requirements
	fmt.Println()
	fmt.Println("─── Income Requirements ───")
	b.config.IncomeRequirements.MonthlyBeforeAge = b.promptMoney("  Monthly income before age threshold", b.getDefaultMoney("income.monthly_before_age", 4000))
	b.config.IncomeRequirements.MonthlyAfterAge = b.promptMoney("  Monthly income after age threshold", b.getDefaultMoney("income.monthly_after_age", 2500))
	b.config.IncomeRequirements.AgeThreshold = b.promptAge("  Age when income changes", b.getDefaultInt("income.age_threshold", 67))
	b.config.IncomeRequirements.ReferencePerson = person1.Name

	// Set depletion defaults (in case user switches modes)
	b.config.IncomeRequirements.TargetDepletionAge = b.getDefaultInt("income.target_depletion_age", 85)
	b.config.IncomeRequirements.IncomeRatioPhase1 = b.getDefaultFloat("income.income_ratio_phase1", 5)
	b.config.IncomeRequirements.IncomeRatioPhase2 = b.getDefaultFloat("income.income_ratio_phase2", 3)

	// Growth rates
	fmt.Println()
	fmt.Println("─── Growth Rates ───")
	b.config.Financial.PensionGrowthRate = b.promptPercent("  Pension growth rate", b.getDefaultPercent("financial.pension_growth_rate", 0.05))
	b.config.Financial.SavingsGrowthRate = b.promptPercent("  Savings/ISA growth rate", b.getDefaultPercent("financial.savings_growth_rate", 0.05))
	b.config.Financial.IncomeInflationRate = b.promptPercent("  Income inflation rate", b.getDefaultPercent("financial.income_inflation_rate", 0.03))
	b.config.Financial.StatePensionAmount = b.promptMoney("  Annual state pension (per person)", b.getDefaultMoney("financial.state_pension_amount", 12547.60))
	b.config.Financial.StatePensionInflation = b.promptPercent("  State pension inflation", b.getDefaultPercent("financial.state_pension_inflation", 0.03))
	b.config.Financial.TaxBandInflation = 0.03

	// Mortgage (optional)
	fmt.Println()
	hasMortgage := b.promptString("─── Do you have a mortgage? (y/n)", "n")
	if strings.ToLower(hasMortgage) == "y" || strings.ToLower(hasMortgage) == "yes" {
		b.config.Mortgage.EndYear = b.promptYear("  Mortgage end year", b.getDefaultInt("mortgage.end_year", 2035))
		b.config.Mortgage.EarlyPayoffYear = b.promptYear("  Earliest payoff year (if tied in)", b.getDefaultInt("mortgage.early_payoff_year", 2030))

		// Add mortgage parts in a loop
		partNum := 1
		for {
			fmt.Printf("\n  ─── Mortgage Part %d ───\n", partNum)
			part := MortgagePartConfig{
				Name:         b.promptString("  Mortgage name", fmt.Sprintf("Mortgage %d", partNum)),
				Principal:    b.promptMoney("  Outstanding balance", b.getDefaultMoney("mortgage.principal", 200000)),
				InterestRate: b.promptPercent("  Interest rate", b.getDefaultPercent("mortgage.interest_rate", 0.04)),
				IsRepayment:  strings.ToLower(b.promptString("  Is repayment mortgage? (y/n)", "y")) == "y",
				StartYear:    b.promptYear("  Start year", b.getDefaultInt("mortgage.start_year", 2020)),
				TermYears:    b.promptInt("  Term in years", b.getDefaultInt("mortgage.term_years", 25)),
			}
			b.config.Mortgage.Parts = append(b.config.Mortgage.Parts, part)

			addAnother := b.promptString("  Add another mortgage part? (y/n)", "n")
			if strings.ToLower(addAnother) != "y" && strings.ToLower(addAnother) != "yes" {
				break
			}
			partNum++
		}
	} else {
		b.config.Mortgage.EndYear = 2025
		b.config.Mortgage.EarlyPayoffYear = 2025
	}

	// Simulation settings
	fmt.Println()
	fmt.Println("─── Simulation Settings ───")
	b.config.Simulation.StartYear = b.promptYear("  Start year", b.getDefaultInt("simulation.start_year", 2026))
	b.config.Simulation.EndAge = b.promptAge("  End age for simulation", b.getDefaultInt("simulation.end_age", 95))
	b.config.Simulation.ReferencePerson = person1.Name

	// Sensitivity analysis settings
	fmt.Println()
	fmt.Println("─── Sensitivity Analysis ───")
	b.config.Sensitivity.PensionGrowthMin = b.promptPercent("  Pension growth min rate", b.getDefaultPercent("sensitivity.pension_growth_min", 0.04))
	b.config.Sensitivity.PensionGrowthMax = b.promptPercent("  Pension growth max rate", b.getDefaultPercent("sensitivity.pension_growth_max", 0.12))
	b.config.Sensitivity.SavingsGrowthMin = b.promptPercent("  Savings growth min rate", b.getDefaultPercent("sensitivity.savings_growth_min", 0.04))
	b.config.Sensitivity.SavingsGrowthMax = b.promptPercent("  Savings growth max rate", b.getDefaultPercent("sensitivity.savings_growth_max", 0.12))
	b.config.Sensitivity.StepSize = b.promptPercent("  Step size for analysis", b.getDefaultPercent("sensitivity.step_size", 0.01))

	// Tax bands (use UK defaults)
	b.config.TaxBands = []TaxBand{
		{Name: "Personal Allowance", Lower: 0, Upper: 12570, Rate: 0.00},
		{Name: "Basic Rate", Lower: 12570, Upper: 50270, Rate: 0.20},
		{Name: "Higher Rate", Lower: 50270, Upper: 125140, Rate: 0.40},
		{Name: "Additional Rate", Lower: 125140, Upper: 10000000, Rate: 0.45},
	}

	return b.config
}

// SaveConfig saves the configuration to a YAML file
func (b *InteractiveConfigBuilder) SaveConfig(filename string) error {
	return SaveConfig(b.config, filename)
}

// ValidateDepletionConfig checks if config has required depletion fields
func ValidateDepletionConfig(config *Config) []string {
	var missing []string

	if len(config.People) == 0 {
		missing = append(missing, "people")
	} else {
		for i, p := range config.People {
			if p.Name == "" {
				missing = append(missing, fmt.Sprintf("people[%d].name", i))
			}
			if p.BirthDate == "" {
				missing = append(missing, fmt.Sprintf("people[%d].birth_date", i))
			}
		}
	}

	if config.IncomeRequirements.TargetDepletionAge <= 0 {
		missing = append(missing, "target_depletion_age")
	}
	if config.IncomeRequirements.IncomeRatioPhase1 <= 0 {
		missing = append(missing, "income_ratio_phase1")
	}
	if config.IncomeRequirements.IncomeRatioPhase2 <= 0 {
		missing = append(missing, "income_ratio_phase2")
	}

	return missing
}

// ValidateSensitivityConfig checks if config has required sensitivity analysis fields
func ValidateSensitivityConfig(config *Config) []string {
	var missing []string

	if config.Sensitivity.PensionGrowthMin <= 0 {
		missing = append(missing, "sensitivity.pension_growth_min")
	}
	if config.Sensitivity.PensionGrowthMax <= 0 {
		missing = append(missing, "sensitivity.pension_growth_max")
	}
	if config.Sensitivity.SavingsGrowthMin <= 0 {
		missing = append(missing, "sensitivity.savings_growth_min")
	}
	if config.Sensitivity.SavingsGrowthMax <= 0 {
		missing = append(missing, "sensitivity.savings_growth_max")
	}
	if config.Sensitivity.StepSize <= 0 {
		missing = append(missing, "sensitivity.step_size")
	}

	return missing
}

// ValidateFixedIncomeConfig checks if config has required fixed income fields
func ValidateFixedIncomeConfig(config *Config) []string {
	var missing []string

	if len(config.People) == 0 {
		missing = append(missing, "people")
	} else {
		for i, p := range config.People {
			if p.Name == "" {
				missing = append(missing, fmt.Sprintf("people[%d].name", i))
			}
			if p.BirthDate == "" {
				missing = append(missing, fmt.Sprintf("people[%d].birth_date", i))
			}
		}
	}

	// Check income requirements: either tiers or legacy values must be configured
	if !config.IncomeRequirements.HasTiers() {
		if config.IncomeRequirements.MonthlyBeforeAge <= 0 {
			missing = append(missing, "monthly_before_age (or income tiers)")
		}
		if config.IncomeRequirements.MonthlyAfterAge <= 0 {
			missing = append(missing, "monthly_after_age (or income tiers)")
		}
	}

	return missing
}
