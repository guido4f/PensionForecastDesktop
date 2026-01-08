package main

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// pdfText converts UTF-8 text to PDF-safe encoding
// The £ sign in UTF-8 is 0xC2 0xA3, but PDF standard fonts expect Latin-1 (just 0xA3)
func pdfText(s string) string {
	// Replace UTF-8 £ (U+00A3) with the Latin-1 byte directly
	return strings.ReplaceAll(s, "£", "\xa3")
}

// FormatMoneyPDF formats money for PDF output with full amounts and comma separators
func FormatMoneyPDF(amount float64) string {
	return pdfText(formatWithCommas(amount))
}

// getPensionAccessMonth returns the first month (0-11) in the tax year when pension becomes accessible.
// Returns -1 if pension is not accessible during this tax year.
// Returns 0 if pension was already accessible before this tax year started.
// Tax year months: Apr=0, May=1, Jun=2, Jul=3, Aug=4, Sep=5, Oct=6, Nov=7, Dec=8, Jan=9, Feb=10, Mar=11
func getPensionAccessMonth(birthDate string, pensionAccessAge int, taxYear int) int {
	t, err := time.Parse("2006-01-02", birthDate)
	if err != nil {
		return 0 // Default to accessible if can't parse
	}

	birthYear := t.Year()
	birthMonth := int(t.Month())
	birthDay := t.Day()

	// Year when they turn pensionAccessAge
	accessYear := birthYear + pensionAccessAge

	// Tax year runs Apr 6 of taxYear to Apr 5 of taxYear+1
	// Check if they gain access before this tax year
	if accessYear < taxYear || (accessYear == taxYear && (birthMonth < 4 || (birthMonth == 4 && birthDay < 6))) {
		return 0 // Already had access before tax year started
	}

	// Check if they gain access after this tax year
	if accessYear > taxYear+1 || (accessYear == taxYear+1 && (birthMonth > 4 || (birthMonth == 4 && birthDay >= 6))) {
		return -1 // Not accessible during this tax year
	}

	// They gain access during this tax year - calculate which month
	// Tax year month mapping: Apr=0, May=1, ..., Dec=8, Jan=9, Feb=10, Mar=11
	if accessYear == taxYear {
		// Birthday is Apr-Dec of taxYear
		// Apr=0, May=1, Jun=2, Jul=3, Aug=4, Sep=5, Oct=6, Nov=7, Dec=8
		return birthMonth - 4
	} else {
		// Birthday is Jan-Mar of taxYear+1
		// Jan=9, Feb=10, Mar=11
		return birthMonth + 8
	}
}

// getLastWorkMonth returns the last month (0-11) in the tax year when a person is still working.
// Returns -1 if the person has already retired before this tax year started.
// Returns 11 if the person works through the entire tax year (retirement is next year or later).
// Tax year months: Apr=0, May=1, Jun=2, Jul=3, Aug=4, Sep=5, Oct=6, Nov=7, Dec=8, Jan=9, Feb=10, Mar=11
func getLastWorkMonth(retirementDate string, taxYear int) int {
	t, err := time.Parse("2006-01-02", retirementDate)
	if err != nil {
		return -1 // Can't parse, assume not working
	}

	retireYear := t.Year()
	retireMonth := int(t.Month())
	retireDay := t.Day()

	// Tax year runs Apr 6 of taxYear to Apr 5 of taxYear+1
	// If retirement is before Apr 6 of taxYear, they weren't working this year
	taxYearStart := time.Date(taxYear, 4, 6, 0, 0, 0, 0, time.UTC)
	if t.Before(taxYearStart) {
		return -1 // Already retired before tax year started
	}

	// If retirement is after Apr 5 of taxYear+1, they work all year
	taxYearEnd := time.Date(taxYear+1, 4, 5, 0, 0, 0, 0, time.UTC)
	if t.After(taxYearEnd) {
		return 11 // Works through entire tax year
	}

	// Retirement falls within this tax year - find the last working month
	// The last working month is the month BEFORE retirement, unless retirement is day 1
	// For simplicity, we'll say they work in the month if retirement is on or after the 15th
	// If retirement is on the 1st, they don't work that month at all

	var lastWorkingMonth int
	if retireDay < 15 {
		// Retiring early in month - last working month is the previous month
		if retireMonth == 1 {
			lastWorkingMonth = 12 // December
			retireYear--
		} else {
			lastWorkingMonth = retireMonth - 1
		}
	} else {
		// Retiring late in month - they work part of this month
		lastWorkingMonth = retireMonth
	}

	// Convert to tax year month (Apr=0, May=1, ..., Dec=8, Jan=9, Feb=10, Mar=11)
	if retireYear == taxYear {
		// Month is Apr-Dec of taxYear
		if lastWorkingMonth >= 4 {
			return lastWorkingMonth - 4
		}
		// Should not happen - would be before tax year start
		return -1
	} else if retireYear == taxYear+1 {
		// Month is Jan-Mar of taxYear+1
		if lastWorkingMonth <= 3 {
			return lastWorkingMonth + 8
		}
		// Should not happen - would be after tax year end
		return 11
	}

	return -1
}

// getGrowthDeclineText returns a text description of growth decline, or empty string if not enabled
func getGrowthDeclineText(config *Config) string {
	// Helper to extract birth year from date string (YYYY-MM-DD)
	getBirthYear := func(dateStr string) int {
		if len(dateStr) >= 4 {
			var year int
			fmt.Sscanf(dateStr, "%d", &year)
			return year
		}
		return 0
	}

	// Check for standard growth decline
	if config.Financial.GrowthDeclineEnabled {
		refPerson := config.Financial.GrowthDeclineReferencePerson
		if refPerson == "" {
			refPerson = config.Simulation.ReferencePerson
		}

		var birthYear int
		for _, p := range config.People {
			if p.Name == refPerson || (refPerson == "" && birthYear == 0) {
				birthYear = getBirthYear(p.BirthDate)
			}
		}
		endYear := birthYear + config.Financial.GrowthDeclineTargetAge
		if endYear <= config.Simulation.StartYear {
			endYear = config.Simulation.StartYear + 20
		}

		return fmt.Sprintf("Growth Decline: %.0f%% to %.0f%% (Pen) / %.0f%% to %.0f%% (ISA) by %d",
			config.Financial.PensionGrowthRate*100, config.Financial.PensionGrowthEndRate*100,
			config.Financial.SavingsGrowthRate*100, config.Financial.SavingsGrowthEndRate*100,
			endYear)
	}

	// Check for depletion-specific growth decline
	if config.Financial.DepletionGrowthDeclineEnabled && config.IncomeRequirements.TargetDepletionAge > 0 {
		refPerson := config.IncomeRequirements.ReferencePerson
		if refPerson == "" {
			refPerson = config.Simulation.ReferencePerson
		}

		var birthYear int
		for _, p := range config.People {
			if p.Name == refPerson || (refPerson == "" && birthYear == 0) {
				birthYear = getBirthYear(p.BirthDate)
			}
		}
		endYear := birthYear + config.IncomeRequirements.TargetDepletionAge
		if endYear <= config.Simulation.StartYear {
			endYear = config.Simulation.StartYear + 20
		}

		pensionEnd := config.Financial.PensionGrowthRate - config.Financial.DepletionGrowthDeclinePercent

		return fmt.Sprintf("Growth Decline: %.0f%% to %.0f%% (%d to %d)",
			config.Financial.PensionGrowthRate*100, pensionEnd*100,
			config.Simulation.StartYear, endYear)
	}

	return ""
}

// getMortgagePayoffYear returns the year the mortgage will be paid off based on strategy
func getMortgagePayoffYear(config *Config, params SimulationParams) int {
	switch params.MortgageOpt {
	case MortgageEarly:
		return config.Mortgage.EarlyPayoffYear
	case MortgageExtended:
		return config.GetExtendedEndYear()
	case PCLSMortgagePayoff:
		// PCLS payoff happens when reference person reaches retirement age
		refPerson := config.GetReferencePerson()
		return GetBirthYear(refPerson.BirthDate) + refPerson.RetirementAge
	default:
		return config.Mortgage.EndYear
	}
}

// PDFActionPlanReport generates a detailed PDF action plan for a simulation result
type PDFActionPlanReport struct {
	pdf    *fpdf.Fpdf
	config *Config
	result SimulationResult
}

// ActionItem represents a specific action to take in a given year
type ActionItem struct {
	Date        string
	Category    string
	Description string
	Amount      float64
	Person      string
	Notes       string
}

// YearActionPlan contains all actions for a specific year
type YearActionPlan struct {
	Year         int
	TaxYearStart string
	TaxYearEnd   string
	Ages         map[string]int
	Actions      []ActionItem
	Summary      YearSummaryPDF
}

// YearSummaryPDF provides totals for the year
type YearSummaryPDF struct {
	StartingBalance   float64
	TotalIncome       float64
	TotalWithdrawals  float64
	MortgageCost      float64
	TotalTaxPaid      float64
	NetIncomeReceived float64
	EndingBalance     float64
	WorkIncome        float64            // Pre-retirement work income
	WorkIncomeByPerson map[string]float64 // Work income per person
}

// MonthlyScheduleItem represents what to do in a specific month
type MonthlyScheduleItem struct {
	Month          string
	NetIncome      float64
	WorkIncome     float64
	ISAWithdrawal  float64
	PensionTaxFree float64
	PensionTaxable float64
	ISADeposit     float64
	Notes          string
}

const (
	pageWidth    = 210.0
	pageHeight   = 297.0
	marginLeft   = 15.0
	marginRight  = 15.0
	marginTop    = 15.0
	marginBottom = 20.0
	contentWidth = pageWidth - marginLeft - marginRight
)

// GenerateStrategyPDFReport creates a detailed PDF action plan for a single strategy
func GenerateStrategyPDFReport(config *Config, result SimulationResult) ([]byte, error) {
	report := &PDFActionPlanReport{
		pdf:    fpdf.New("P", "mm", "A4", ""),
		config: config,
		result: result,
	}

	report.pdf.SetMargins(marginLeft, marginTop, marginRight)
	report.pdf.SetAutoPageBreak(true, marginBottom)

	// Add pages
	report.addTitlePage()
	report.addStrategyOverview()
	report.addYearByYearSummary()
	report.addSummaryPage()

	// Output to buffer
	var buf bytes.Buffer
	err := report.pdf.Output(&buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (r *PDFActionPlanReport) addTitlePage() {
	r.pdf.AddPage()

	// Title
	r.pdf.SetFont("Arial", "B", 28)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.Ln(50)
	r.pdf.CellFormat(contentWidth, 15, "Retirement Action Plan", "", 1, "C", false, 0, "")

	// Strategy name - use descriptive name matching the UI
	mortgagePayoffYear := getMortgagePayoffYear(r.config, r.result.Params)
	strategyName := r.result.Params.DescriptiveName(mortgagePayoffYear)
	r.pdf.SetFont("Arial", "", 14)
	r.pdf.SetTextColor(80, 80, 80)
	r.pdf.Ln(10)
	r.pdf.CellFormat(contentWidth, 10, strategyName, "", 1, "C", false, 0, "")

	// Generation date
	r.pdf.SetFont("Arial", "I", 11)
	r.pdf.Ln(15)
	r.pdf.CellFormat(contentWidth, 8, fmt.Sprintf("Generated: %s", time.Now().Format("2 January 2006")), "", 1, "C", false, 0, "")

	// Participants box
	r.pdf.Ln(20)
	r.pdf.SetFillColor(245, 247, 250)
	r.pdf.SetDrawColor(200, 200, 200)

	r.pdf.SetFont("Arial", "B", 12)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 8, "Plan Participants", "1", 1, "C", true, 0, "")

	r.pdf.SetFont("Arial", "", 11)
	r.pdf.SetTextColor(50, 50, 50)
	for _, person := range r.config.People {
		birthYear := GetBirthYear(person.BirthDate)
		pensionAccessAge := person.PensionAccessAge
		if pensionAccessAge <= 0 {
			pensionAccessAge = person.RetirementAge
		}
		var text string
		if pensionAccessAge != person.RetirementAge {
			text = fmt.Sprintf("%s - Born %d, Stop Work %d, Pension Access %d, State Pension %d",
				person.Name, birthYear, person.RetirementAge, pensionAccessAge, person.StatePensionAge)
		} else {
			text = fmt.Sprintf("%s - Born %d, Pension Access Age %d, State Pension Age %d",
				person.Name, birthYear, pensionAccessAge, person.StatePensionAge)
		}
		r.pdf.CellFormat(contentWidth, 7, text, "LR", 1, "C", true, 0, "")
	}
	r.pdf.CellFormat(contentWidth, 1, "", "LRB", 1, "C", true, 0, "")

	// Simulation period box
	r.pdf.Ln(10)
	r.pdf.SetFont("Arial", "B", 12)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 8, "Simulation Period", "1", 1, "C", true, 0, "")

	r.pdf.SetFont("Arial", "", 11)
	r.pdf.SetTextColor(50, 50, 50)
	startYear := r.config.Simulation.StartYear
	endYear := startYear
	if len(r.result.Years) > 0 {
		endYear = r.result.Years[len(r.result.Years)-1].Year
	}
	// Date range: 6 April start year to 5 April end year+1
	periodText := fmt.Sprintf("6 April %d to 5 April %d (%d years)",
		startYear, endYear+1, endYear-startYear+1)
	r.pdf.CellFormat(contentWidth, 7, periodText, "LR", 1, "C", true, 0, "")

	// Get depletion years
	isaDepletedYear, pensionDepletedYear := r.getDepletionYears()
	refPerson := r.config.GetReferencePerson()
	refBirthYear := GetBirthYear(refPerson.BirthDate)

	// ISA depletion
	isaText := "ISA Depleted: Never"
	if isaDepletedYear > 0 {
		isaAge := GetAgeInTaxYear(refPerson.BirthDate, isaDepletedYear)
		isaText = fmt.Sprintf("ISA Depleted: Tax Year %s (Age %d)", TaxYearLabel(isaDepletedYear), isaAge)
	}
	r.pdf.CellFormat(contentWidth, 7, isaText, "LR", 1, "C", true, 0, "")

	// Pension depletion
	pensionText := "Pension Depleted: Never"
	if pensionDepletedYear > 0 {
		pensionAge := GetAgeInTaxYear(refPerson.BirthDate, pensionDepletedYear)
		pensionText = fmt.Sprintf("Pension Depleted: Tax Year %s (Age %d)", TaxYearLabel(pensionDepletedYear), pensionAge)
	}
	r.pdf.CellFormat(contentWidth, 7, pensionText, "LRB", 1, "C", true, 0, "")

	// Income Requirements box
	r.pdf.Ln(10)
	r.pdf.SetFont("Arial", "B", 12)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 8, "Income Requirements", "1", 1, "C", true, 0, "")

	r.pdf.SetFont("Arial", "", 11)
	r.pdf.SetTextColor(50, 50, 50)

	// Display income tiers
	incomeLines := r.getIncomeRequirementLines(startYear, endYear, refBirthYear)
	for _, line := range incomeLines {
		r.pdf.CellFormat(contentWidth, 7, line, "LR", 1, "C", true, 0, "")
	}
	r.pdf.CellFormat(contentWidth, 1, "", "LRB", 1, "C", true, 0, "")

	// Mortgage box (if applicable)
	if r.config.HasMortgage() {
		r.pdf.Ln(10)
		r.pdf.SetFont("Arial", "B", 12)
		r.pdf.SetTextColor(0, 51, 102)
		r.pdf.CellFormat(contentWidth, 8, "Mortgage Payments", "1", 1, "C", true, 0, "")

		r.pdf.SetFont("Arial", "", 11)
		r.pdf.SetTextColor(50, 50, 50)

		// Calculate mortgage details
		monthlyMortgage := r.config.GetTotalAnnualPayment() / 12
		mortgageEndYear := getMortgagePayoffYear(r.config, r.result.Params)

		// Monthly payment line
		r.pdf.CellFormat(contentWidth, 7,
			fmt.Sprintf("%s/month mortgage payment", FormatMoneyPDF(monthlyMortgage)),
			"LR", 1, "C", true, 0, "")

		// Mortgage end information
		mortgageAge := GetAgeInTaxYear(refPerson.BirthDate, mortgageEndYear)
		mortgageEndText := fmt.Sprintf("Mortgage ends: Tax Year %s (Age %d)", TaxYearLabel(mortgageEndYear), mortgageAge)
		r.pdf.CellFormat(contentWidth, 7, mortgageEndText, "LRB", 1, "C", true, 0, "")
	}

	// Disclaimer
	r.pdf.Ln(15)
	r.pdf.SetFont("Arial", "I", 9)
	r.pdf.SetTextColor(120, 120, 120)
	r.pdf.MultiCell(contentWidth, 4.5,
		"This document is for informational purposes only and does not constitute financial advice. "+
			"Please consult a qualified financial advisor before making any financial decisions. "+
			"Tax rules and allowances are subject to change.", "", "C", false)
}

func (r *PDFActionPlanReport) addStrategyOverview() {
	r.pdf.AddPage()

	// Header
	r.drawSectionHeader("Strategy Overview")

	// Strategy description box
	r.pdf.SetFont("Arial", "B", 11)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 7, "Selected Strategy", "", 1, "L", false, 0, "")

	r.pdf.SetFont("Arial", "", 10)
	r.pdf.SetTextColor(50, 50, 50)
	r.pdf.MultiCell(contentWidth, 5, r.getStrategyDescription(), "", "L", false)
	r.pdf.Ln(5)

	// Simulation Period and Income Requirements
	r.pdf.SetFont("Arial", "B", 11)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 7, "Simulation Period & Income Requirements", "", 1, "L", false, 0, "")

	r.pdf.SetFont("Arial", "", 10)
	r.pdf.SetTextColor(50, 50, 50)

	// Calculate dates
	startYear := r.config.Simulation.StartYear
	endYear := startYear
	if len(r.result.Years) > 0 {
		endYear = r.result.Years[len(r.result.Years)-1].Year
	}
	refPerson := r.config.GetReferencePerson()
	refBirthYear := GetBirthYear(refPerson.BirthDate)

	// Period row
	r.pdf.CellFormat(35, 5, "Simulation Period:", "", 0, "L", false, 0, "")
	r.pdf.CellFormat(contentWidth-35, 5, fmt.Sprintf("6 April %d to 5 April %d (%d years)",
		startYear, endYear+1, endYear-startYear+1), "", 1, "L", false, 0, "")

	// Display income tiers
	incomeLines := r.getIncomeRequirementLines(startYear, endYear, refBirthYear)
	for i, line := range incomeLines {
		label := ""
		if i == 0 {
			label = "Income:"
		}
		r.pdf.CellFormat(35, 5, label, "", 0, "L", false, 0, "")
		r.pdf.CellFormat(contentWidth-35, 5, line, "", 1, "L", false, 0, "")
	}

	// Mortgage details (if applicable)
	if r.config.HasMortgage() {
		monthlyMortgage := r.config.GetTotalAnnualPayment() / 12
		mortgageEndYear := getMortgagePayoffYear(r.config, r.result.Params)
		mortgageEndAge := mortgageEndYear - refBirthYear

		r.pdf.CellFormat(35, 5, "Mortgage:", "", 0, "L", false, 0, "")
		r.pdf.CellFormat(contentWidth-35, 5,
			fmt.Sprintf("%s/month until %d (Age %d)", FormatMoneyPDF(monthlyMortgage), mortgageEndYear, mortgageEndAge),
			"", 1, "L", false, 0, "")
	}

	r.pdf.Ln(5)

	// Strategy Options section
	r.pdf.SetFont("Arial", "B", 11)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 7, "Strategy Options", "", 1, "L", false, 0, "")

	r.pdf.SetFont("Arial", "", 10)
	r.pdf.SetTextColor(50, 50, 50)

	// Helper to add option with explanation
	addOption := func(label, value, explanation string) {
		r.pdf.SetFont("Arial", "", 10)
		r.pdf.SetTextColor(50, 50, 50)
		r.pdf.CellFormat(45, 5, label, "", 0, "L", false, 0, "")
		r.pdf.CellFormat(contentWidth-45, 5, value, "", 1, "L", false, 0, "")
		if explanation != "" {
			r.pdf.SetFont("Arial", "I", 8)
			r.pdf.SetTextColor(100, 100, 100)
			r.pdf.CellFormat(45, 4, "", "", 0, "L", false, 0, "")
			r.pdf.CellFormat(contentWidth-45, 4, explanation, "", 1, "L", false, 0, "")
		}
	}

	// Crystallisation
	crystalName := "Gradual Crystallisation"
	crystalExplain := "Move funds to drawdown as needed, taking 25% tax-free from each withdrawal"
	if r.result.Params.CrystallisationStrategy == UFPLSStrategy {
		crystalName = "UFPLS (Uncrystallised Funds Pension Lump Sum)"
		crystalExplain = "Withdraw directly from uncrystallised pot - 25% tax-free, 75% taxable per withdrawal"
	}
	addOption("Crystallisation:", crystalName, crystalExplain)

	// Drawdown order with explanation
	drawdownName, drawdownExplain := r.getDrawdownOrderNameWithExplanation()
	addOption("Drawdown Order:", drawdownName, drawdownExplain)

	// Guardrails
	if r.result.Params.GuardrailsEnabled {
		addOption("Withdrawal Method:", "Guyton-Klinger Guardrails",
			"Dynamic adjustments based on portfolio performance - cut spending in bad years, increase in good")
	}

	// State Pension Deferral
	if r.result.Params.StatePensionDeferYears > 0 {
		deferText := fmt.Sprintf("%d years (+%.1f%% enhancement)",
			r.result.Params.StatePensionDeferYears,
			float64(r.result.Params.StatePensionDeferYears)*5.8)
		addOption("State Pension Deferral:", deferText,
			"Delay claiming state pension to receive higher payments (5.8% per year deferred)")
	}

	// ISA to SIPP
	if r.result.Params.ISAToSIPPEnabled {
		addOption("ISA to Pension:", "Enabled",
			"Transfer ISA savings to pension while working to gain tax relief on contributions")
	}

	// Maximize Couple ISA
	if r.result.Params.MaximizeCoupleISA && len(r.config.People) >= 2 {
		addOption("Couple ISA Strategy:", "Maximize Both Allowances",
			"Withdraw extra from pension to fill both partners' ISA allowances each year")
	}

	r.pdf.Ln(5)

	// Two-column layout for growth parameters
	colWidth := contentWidth / 2

	// Growth Parameters
	r.pdf.SetFont("Arial", "B", 11)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 7, "Growth & Inflation Assumptions", "", 1, "L", false, 0, "")

	r.pdf.SetFont("Arial", "", 10)
	r.pdf.SetTextColor(50, 50, 50)

	params := [][]string{
		{"Pension Growth:", fmt.Sprintf("%.1f%% p.a.", r.config.Financial.PensionGrowthRate*100)},
		{"ISA Growth:", fmt.Sprintf("%.1f%% p.a.", r.config.Financial.SavingsGrowthRate*100)},
		{"Income Inflation:", fmt.Sprintf("%.1f%% p.a.", r.config.Financial.IncomeInflationRate*100)},
		{"Tax Band Inflation:", fmt.Sprintf("%.1f%% p.a.", r.config.Financial.TaxBandInflation*100)},
	}

	for i := 0; i < len(params); i += 2 {
		r.pdf.CellFormat(colWidth/2, 5, params[i][0], "", 0, "L", false, 0, "")
		r.pdf.CellFormat(colWidth/2, 5, params[i][1], "", 0, "L", false, 0, "")
		if i+1 < len(params) {
			r.pdf.CellFormat(colWidth/2, 5, params[i+1][0], "", 0, "L", false, 0, "")
			r.pdf.CellFormat(colWidth/2, 5, params[i+1][1], "", 1, "L", false, 0, "")
		} else {
			r.pdf.Ln(-1)
		}
	}

	// Growth decline info if enabled
	if gdText := getGrowthDeclineText(r.config); gdText != "" {
		r.pdf.SetTextColor(150, 80, 0) // Orange-brown color
		r.pdf.CellFormat(contentWidth, 5, gdText, "", 1, "L", false, 0, "")
		r.pdf.SetTextColor(50, 50, 50)
	}

	r.pdf.Ln(5)

	// Starting Balances Table
	r.pdf.SetFont("Arial", "B", 11)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 7, "Starting Balances", "", 1, "L", false, 0, "")

	r.drawTableHeader([]string{"Person", "ISA", "Pension", "Total"}, []float64{50, 40, 40, 50})

	totalISA := 0.0
	totalPension := 0.0
	for _, person := range r.config.People {
		totalISA += person.TaxFreeSavings
		totalPension += person.Pension
		r.drawTableRow([]string{
			person.Name,
			FormatMoneyPDF(person.TaxFreeSavings),
			FormatMoneyPDF(person.Pension),
			FormatMoneyPDF(person.TaxFreeSavings + person.Pension),
		}, []float64{50, 40, 40, 50}, false)
	}
	r.drawTableRow([]string{
		"TOTAL",
		FormatMoneyPDF(totalISA),
		FormatMoneyPDF(totalPension),
		FormatMoneyPDF(totalISA + totalPension),
	}, []float64{50, 40, 40, 50}, true)

	r.pdf.Ln(8)

	// Results Summary
	r.pdf.SetFont("Arial", "B", 11)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 7, "Projected Results", "", 1, "L", false, 0, "")

	totalFinal := 0.0
	for _, bal := range r.result.FinalBalances {
		totalFinal += bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot
	}

	// Get depletion years (reuse refPerson and refBirthYear from above)
	isaDepletedYear, pensionDepletedYear := r.getDepletionYears()

	results := [][]string{
		{"Total Tax Paid:", FormatMoneyPDF(r.result.TotalTaxPaid)},
		{"Total Withdrawals:", FormatMoneyPDF(r.result.TotalWithdrawn)},
		{"Final Balance:", FormatMoneyPDF(totalFinal)},
	}

	// Add ISA depletion
	if isaDepletedYear > 0 {
		isaAge := isaDepletedYear - refBirthYear
		results = append(results, []string{"ISA Depleted:", fmt.Sprintf("%d (Age %d)", isaDepletedYear, isaAge)})
	} else {
		results = append(results, []string{"ISA Depleted:", "Never"})
	}

	// Add Pension depletion
	if pensionDepletedYear > 0 {
		pensionAge := pensionDepletedYear - refBirthYear
		results = append(results, []string{"Pension Depleted:", fmt.Sprintf("%d (Age %d)", pensionDepletedYear, pensionAge)})
	} else {
		results = append(results, []string{"Pension Depleted:", "Never"})
	}

	if r.result.RanOutOfMoney {
		results = append(results, []string{"WARNING:", fmt.Sprintf("All funds depleted in %d", r.result.RanOutYear)})
	}

	r.pdf.SetFont("Arial", "", 10)
	r.pdf.SetTextColor(50, 50, 50)
	for _, row := range results {
		if row[0] == "WARNING:" {
			r.pdf.SetTextColor(180, 0, 0)
		} else if row[0] == "ISA Depleted:" || row[0] == "Pension Depleted:" {
			if row[1] != "Never" {
				r.pdf.SetTextColor(200, 100, 0) // Orange for depletion warning
			}
		}
		r.pdf.CellFormat(60, 5, row[0], "", 0, "L", false, 0, "")
		r.pdf.CellFormat(60, 5, row[1], "", 1, "L", false, 0, "")
		r.pdf.SetTextColor(50, 50, 50)
	}

	// Year-by-Year Summary Table
	r.addYearlySummaryTable()
}

// addYearlySummaryTable adds a compact year-by-year summary table to the Strategy Overview
func (r *PDFActionPlanReport) addYearlySummaryTable() {
	if len(r.result.Years) == 0 {
		return
	}

	// Check if we need a new page
	if r.pdf.GetY() > 180 {
		r.pdf.AddPage()
	}

	r.pdf.Ln(8)
	r.pdf.SetFont("Arial", "B", 11)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 7, "Year-by-Year Summary", "", 1, "L", false, 0, "")

	// Column widths for table with full GBP amounts (total must be <= 180mm content width)
	colWidths := []float64{11, 12, 25, 25, 17, 22, 17, 18, 25}
	headers := []string{"Year", "Ages", "Start", "End", "Monthly", "Net Inc", "Mortg", "Tax", "Growth"}

	// Draw header
	r.pdf.SetFillColor(0, 51, 102)
	r.pdf.SetTextColor(255, 255, 255)
	r.pdf.SetFont("Arial", "B", 7)

	for i, header := range headers {
		align := "L"
		if i == 1 { // Ages column centered
			align = "C"
		} else if i > 1 {
			align = "R"
		}
		r.pdf.CellFormat(colWidths[i], 5, header, "1", 0, align, true, 0, "")
	}
	r.pdf.Ln(-1)

	// Track totals
	totalNetIncome := 0.0
	totalMortgage := 0.0
	totalTax := 0.0
	totalGrowth := 0.0

	// Draw rows
	r.pdf.SetFont("Arial", "", 7)
	r.pdf.SetTextColor(50, 50, 50)

	for i, year := range r.result.Years {
		// Check for page break
		if r.pdf.GetY() > 270 {
			r.pdf.AddPage()
			// Redraw header
			r.pdf.SetFont("Arial", "B", 11)
			r.pdf.SetTextColor(0, 51, 102)
			r.pdf.CellFormat(contentWidth, 7, "Year-by-Year Summary (continued)", "", 1, "L", false, 0, "")

			r.pdf.SetFillColor(0, 51, 102)
			r.pdf.SetTextColor(255, 255, 255)
			r.pdf.SetFont("Arial", "B", 7)
			for j, header := range headers {
				align := "L"
				if j == 1 { // Ages column centered
					align = "C"
				} else if j > 1 {
					align = "R"
				}
				r.pdf.CellFormat(colWidths[j], 5, header, "1", 0, align, true, 0, "")
			}
			r.pdf.Ln(-1)
			r.pdf.SetFont("Arial", "", 7)
			r.pdf.SetTextColor(50, 50, 50)
		}

		// Alternate row colors
		if i%2 == 0 {
			r.pdf.SetFillColor(250, 250, 250)
		} else {
			r.pdf.SetFillColor(255, 255, 255)
		}

		// Calculate investment growth applied at start of this year
		// Growth = StartBalance(this year) - EndBalance(previous year)
		// For year 0, no growth is applied (simulation starts fresh)
		var growth float64
		if i == 0 {
			growth = 0
		} else {
			prevYear := r.result.Years[i-1]
			growth = year.StartBalance - prevYear.TotalBalance
		}

		// Monthly required (income portion only, not mortgage)
		monthlyRequired := year.RequiredIncome / 12

		// Accumulate totals
		totalNetIncome += year.NetIncomeReceived
		totalMortgage += year.MortgageCost
		totalTax += year.TotalTaxPaid
		totalGrowth += growth

		// Build ages string (e.g., "53/55" for two people)
		agesStr := ""
		ageValues := make([]int, 0, len(year.Ages))
		for _, age := range year.Ages {
			ageValues = append(ageValues, age)
		}
		sort.Ints(ageValues)
		for i, age := range ageValues {
			if i > 0 {
				agesStr += "/"
			}
			agesStr += fmt.Sprintf("%d", age)
		}

		// Draw row
		r.pdf.CellFormat(colWidths[0], 4, fmt.Sprintf("%d", year.Year), "1", 0, "L", true, 0, "")
		r.pdf.CellFormat(colWidths[1], 4, agesStr, "1", 0, "C", true, 0, "")
		r.pdf.CellFormat(colWidths[2], 4, formatCompactMoney(year.StartBalance), "1", 0, "R", true, 0, "")
		r.pdf.CellFormat(colWidths[3], 4, formatCompactMoney(year.TotalBalance), "1", 0, "R", true, 0, "")
		r.pdf.CellFormat(colWidths[4], 4, formatCompactMoney(monthlyRequired), "1", 0, "R", true, 0, "")
		r.pdf.CellFormat(colWidths[5], 4, formatCompactMoney(year.NetIncomeReceived), "1", 0, "R", true, 0, "")
		r.pdf.CellFormat(colWidths[6], 4, formatCompactMoney(year.MortgageCost), "1", 0, "R", true, 0, "")
		r.pdf.CellFormat(colWidths[7], 4, formatCompactMoney(year.TotalTaxPaid), "1", 0, "R", true, 0, "")

		// Color-code growth (green positive, red negative)
		if growth >= 0 {
			r.pdf.SetTextColor(0, 128, 0) // Green
		} else {
			r.pdf.SetTextColor(180, 0, 0) // Red
		}
		r.pdf.CellFormat(colWidths[8], 4, formatCompactMoney(growth), "1", 1, "R", true, 0, "")
		r.pdf.SetTextColor(50, 50, 50)
	}

	// Totals row
	r.pdf.SetFont("Arial", "B", 7)
	r.pdf.SetFillColor(230, 235, 240)

	startBal := 0.0
	if len(r.result.Years) > 0 {
		startBal = r.result.Years[0].StartBalance
	}
	endBal := 0.0
	if len(r.result.Years) > 0 {
		endBal = r.result.Years[len(r.result.Years)-1].TotalBalance
	}

	r.pdf.CellFormat(colWidths[0], 4, "TOTAL", "1", 0, "L", true, 0, "")
	r.pdf.CellFormat(colWidths[1], 4, "-", "1", 0, "C", true, 0, "")
	r.pdf.CellFormat(colWidths[2], 4, formatCompactMoney(startBal), "1", 0, "R", true, 0, "")
	r.pdf.CellFormat(colWidths[3], 4, formatCompactMoney(endBal), "1", 0, "R", true, 0, "")
	r.pdf.CellFormat(colWidths[4], 4, "-", "1", 0, "R", true, 0, "")
	r.pdf.CellFormat(colWidths[5], 4, formatCompactMoney(totalNetIncome), "1", 0, "R", true, 0, "")
	r.pdf.CellFormat(colWidths[6], 4, formatCompactMoney(totalMortgage), "1", 0, "R", true, 0, "")
	r.pdf.CellFormat(colWidths[7], 4, formatCompactMoney(totalTax), "1", 0, "R", true, 0, "")

	if totalGrowth >= 0 {
		r.pdf.SetTextColor(0, 128, 0)
	} else {
		r.pdf.SetTextColor(180, 0, 0)
	}
	r.pdf.CellFormat(colWidths[8], 4, formatCompactMoney(totalGrowth), "1", 1, "R", true, 0, "")
	r.pdf.SetTextColor(50, 50, 50)

	// Legend
	r.pdf.Ln(2)
	r.pdf.SetFont("Arial", "I", 6)
	r.pdf.SetTextColor(100, 100, 100)
	r.pdf.CellFormat(contentWidth, 3,
		"Ages = P1/P2 | Start/End = Portfolio balance | Monthly = Required income/month | Net Inc = Spendable income | Growth = Investment gains (green) or losses (red)",
		"", 1, "L", false, 0, "")
}

// formatCompactMoney formats money values for tables with full GBP amounts
func formatCompactMoney(amount float64) string {
	if amount == 0 {
		return "-"
	}
	// Format with comma separators
	return pdfText(formatWithCommas(amount))
}

// formatWithCommas formats a number with comma thousand separators and £ symbol
func formatWithCommas(amount float64) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}

	// Round to nearest pound
	rounded := int64(amount + 0.5)

	// Convert to string with commas
	str := fmt.Sprintf("%d", rounded)

	// Add commas from right to left
	n := len(str)
	if n <= 3 {
		if negative {
			return fmt.Sprintf("-\xa3%s", str)
		}
		return fmt.Sprintf("\xa3%s", str)
	}

	// Build string with commas
	var result []byte
	for i, c := range str {
		if i > 0 && (n-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}

	if negative {
		return fmt.Sprintf("-\xa3%s", string(result))
	}
	return fmt.Sprintf("\xa3%s", string(result))
}

func (r *PDFActionPlanReport) addYearByYearSummary() {
	r.pdf.AddPage()
	r.drawSectionHeader("Year-by-Year Action Plan")

	// Get sorted person names for consistent ordering
	personNames := make([]string, 0, len(r.config.People))
	for _, p := range r.config.People {
		personNames = append(personNames, p.Name)
	}
	sort.Strings(personNames)

	for i, yearState := range r.result.Years {
		plan := r.buildYearActionPlan(yearState)

		// Check if we need a new page (leave room for at least the header and a few rows)
		if r.pdf.GetY() > 220 {
			r.pdf.AddPage()
		}

		r.drawYearSection(plan, personNames, i == 0, yearState)
	}
}

func (r *PDFActionPlanReport) drawYearSection(plan YearActionPlan, personNames []string, isFirst bool, yearState YearState) {
	// Year header bar
	r.pdf.SetFillColor(0, 51, 102)
	r.pdf.SetTextColor(255, 255, 255)
	r.pdf.SetFont("Arial", "B", 10)

	// Build ages string
	ageStr := ""
	for i, name := range personNames {
		if i > 0 {
			ageStr += ", "
		}
		ageStr += fmt.Sprintf("%s: %d", name, plan.Ages[name])
	}

	headerText := fmt.Sprintf("Tax Year %d/%d  |  %s to %s  |  Ages: %s",
		plan.Year, plan.Year+1, plan.TaxYearStart, plan.TaxYearEnd, ageStr)
	r.pdf.CellFormat(contentWidth, 7, headerText, "", 1, "L", true, 0, "")

	// Summary row - accounting structure with Start Balance, movements, End Balance
	r.pdf.SetFillColor(240, 248, 255)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.SetFont("Arial", "", 7)

	// First line: Start Balance, Required, Withdrawals
	summaryLine1 := fmt.Sprintf("Start: %s  |  Required: %s  |  Withdrawals: %s",
		FormatMoneyPDF(plan.Summary.StartingBalance),
		FormatMoneyPDF(plan.Summary.TotalIncome),
		FormatMoneyPDF(plan.Summary.TotalWithdrawals))
	if plan.Summary.MortgageCost > 0 {
		summaryLine1 += fmt.Sprintf("  |  Mortgage: %s", FormatMoneyPDF(plan.Summary.MortgageCost))
	}
	r.pdf.CellFormat(contentWidth, 4, summaryLine1, "", 1, "L", true, 0, "")

	// Second line: Tax, Net, End Balance
	summaryLine2 := fmt.Sprintf("Tax: %s  |  Net: %s  |  End: %s",
		FormatMoneyPDF(plan.Summary.TotalTaxPaid),
		FormatMoneyPDF(plan.Summary.NetIncomeReceived),
		FormatMoneyPDF(plan.Summary.EndingBalance))
	r.pdf.CellFormat(contentWidth, 4, summaryLine2, "", 1, "L", true, 0, "")

	// Action items (milestones and key events)
	r.pdf.SetTextColor(50, 50, 50)
	r.pdf.SetFont("Arial", "", 8)

	for i, action := range plan.Actions {
		// Check for page break
		if r.pdf.GetY() > 250 {
			r.pdf.AddPage()
		}

		// Alternate row colors
		if i%2 == 0 {
			r.pdf.SetFillColor(252, 252, 252)
		} else {
			r.pdf.SetFillColor(255, 255, 255)
		}

		// Format amount
		amountStr := ""
		if action.Amount > 0 {
			amountStr = FormatMoneyPDF(action.Amount)
		}

		// Single row with all info
		y := r.pdf.GetY()
		r.pdf.SetX(marginLeft)

		// Category with color
		r.setCategoryColor(action.Category)
		r.pdf.SetFont("Arial", "B", 7)
		r.pdf.CellFormat(20, 4, action.Category, "", 0, "L", true, 0, "")

		// Description
		r.pdf.SetTextColor(50, 50, 50)
		r.pdf.SetFont("Arial", "", 8)
		descWidth := contentWidth - 20 - 25 - 25 // category, amount, person
		r.pdf.CellFormat(descWidth, 4, truncateString(action.Description, 70), "", 0, "L", true, 0, "")

		// Amount
		r.pdf.SetFont("Arial", "", 8)
		r.pdf.CellFormat(25, 4, amountStr, "", 0, "R", true, 0, "")

		// Person
		r.pdf.CellFormat(25, 4, action.Person, "", 1, "C", true, 0, "")

		// Notes on separate line if present
		if action.Notes != "" {
			r.pdf.SetFont("Arial", "I", 7)
			r.pdf.SetTextColor(100, 100, 100)
			r.pdf.SetX(marginLeft + 20)
			r.pdf.CellFormat(contentWidth-20, 3.5, truncateString(action.Notes, 100), "", 1, "L", false, 0, "")
			r.pdf.SetTextColor(50, 50, 50)
		}

		_ = y
	}

	// Add monthly schedule if there are withdrawals
	if plan.Summary.TotalWithdrawals > 0 || plan.Summary.TotalIncome > 0 {
		r.drawMonthlySchedule(plan, yearState)
	}

	// Add mortgage payoff schedule if this is a payoff year
	if yearState.MortgageCost > 0 && yearState.MortgageCost > r.config.GetTotalAnnualPayment()*1.5 {
		r.drawMortgagePayoffSchedule(plan, yearState)
	}

	r.pdf.Ln(4)
}

func (r *PDFActionPlanReport) drawMonthlySchedule(plan YearActionPlan, yearState YearState) {
	// Check for page break before drawing schedule
	if r.pdf.GetY() > 180 {
		r.pdf.AddPage()
	}

	r.pdf.Ln(2)
	r.pdf.SetFont("Arial", "B", 8)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 5, "Monthly Schedule", "", 1, "L", false, 0, "")

	// Show breakdown of requirements including work income
	hasWorkIncome := yearState.TotalWorkIncome > 0
	hasMortgage := yearState.NetMortgageRequired > 0

	if hasWorkIncome || hasMortgage {
		r.pdf.SetFont("Arial", "", 7)
		r.pdf.SetTextColor(80, 80, 80)

		// Build the breakdown string
		grossRequired := yearState.RequiredIncome + yearState.MortgageCost
		breakdown := fmt.Sprintf("Gross: %s/month", FormatMoneyPDF(grossRequired/12))

		// Add components
		components := []string{}
		components = append(components, fmt.Sprintf("Income: %s", FormatMoneyPDF(yearState.RequiredIncome/12)))
		if hasMortgage {
			components = append(components, fmt.Sprintf("Mortgage: %s", FormatMoneyPDF(yearState.MortgageCost/12)))
		}
		breakdown += " (" + strings.Join(components, " + ") + ")"

		// Subtract work income if present
		if hasWorkIncome {
			breakdown += fmt.Sprintf(" - Salary: %s", FormatMoneyPDF(yearState.TotalWorkIncome/12))
		}

		// Show net needed from withdrawals
		breakdown += fmt.Sprintf(" = Net from Withdrawals: %s", FormatMoneyPDF(yearState.NetRequired/12))

		r.pdf.CellFormat(contentWidth, 3.5, breakdown, "", 1, "L", false, 0, "")
		r.pdf.Ln(1)
	}

	// Calculate monthly amounts
	// Net Needed = the required income that must come from withdrawals (excluding work income)
	// This is what must be funded from ISA/pension each month
	monthlyNetRequired := yearState.NetRequired / 12

	// Calculate total ISA withdrawals for this year
	totalISAWithdrawal := 0.0
	for _, amount := range yearState.Withdrawals.TaxFreeFromISA {
		totalISAWithdrawal += amount
	}

	// Calculate total pension tax-free (PCLS)
	totalPensionTaxFree := 0.0
	for _, amount := range yearState.Withdrawals.TaxFreeFromPension {
		totalPensionTaxFree += amount
	}

	// Calculate total pension taxable
	totalPensionTaxable := 0.0
	for _, amount := range yearState.Withdrawals.TaxableFromPension {
		totalPensionTaxable += amount
	}

	// Calculate total ISA deposits (for pension-to-ISA strategy)
	totalISADeposit := 0.0
	for _, amount := range yearState.Withdrawals.ISADeposits {
		totalISADeposit += amount
	}

	// Calculate first month when any person can access pension
	// This determines when pension withdrawals can actually start
	firstPensionMonth := -1 // -1 means no pension access this year
	for _, person := range r.config.People {
		accessMonth := getPensionAccessMonth(person.BirthDate, person.PensionAccessAge, yearState.Year)
		if accessMonth >= 0 {
			if firstPensionMonth < 0 || accessMonth < firstPensionMonth {
				firstPensionMonth = accessMonth
			}
		}
	}

	// If no pension access this year, show all pension columns as 0
	// If pension access starts mid-year, redistribute withdrawals
	pensionMonths := 12 // Number of months pension is accessible
	if firstPensionMonth < 0 {
		pensionMonths = 0
	} else if firstPensionMonth > 0 {
		pensionMonths = 12 - firstPensionMonth
	}

	// Calculate monthly breakdown based on pension access timing
	// The key insight: in pre-pension months, ALL income must come from ISA
	// In pension months, pension provides income and ISA supplements
	var monthlyISAPrePension, monthlyISAPensionPeriod float64
	var monthlyPensionTaxFree, monthlyPensionTaxable float64

	prePensionMonthCount := 12 - pensionMonths

	if pensionMonths > 0 && prePensionMonthCount > 0 {
		// Mixed year: some months without pension, some with
		// Pre-pension months: need to cover full monthly need from ISA/savings
		// The simulation assumed pension available all year, so we need to show
		// what actually needs to happen month by month

		// Pension withdrawals concentrated in pension months only
		monthlyPensionTaxFree = totalPensionTaxFree / float64(pensionMonths)
		monthlyPensionTaxable = totalPensionTaxable / float64(pensionMonths)

		// For pre-pension months, ISA/savings must cover the withdrawal need
		// This is the net required (after work income is accounted for)
		monthlyISAPrePension = monthlyNetRequired

		// During pension months, if there are ISA withdrawals in the simulation,
		// spread them over those months
		if totalISAWithdrawal > 0 {
			monthlyISAPensionPeriod = totalISAWithdrawal / float64(pensionMonths)
		} else {
			monthlyISAPensionPeriod = 0
		}
	} else if pensionMonths > 0 {
		// Full year of pension access
		monthlyPensionTaxFree = totalPensionTaxFree / 12
		monthlyPensionTaxable = totalPensionTaxable / 12
		monthlyISAPrePension = totalISAWithdrawal / 12
		monthlyISAPensionPeriod = totalISAWithdrawal / 12
	} else {
		// No pension access this year - all from ISA
		monthlyPensionTaxFree = 0
		monthlyPensionTaxable = 0
		monthlyISAPrePension = totalISAWithdrawal / 12
		monthlyISAPensionPeriod = totalISAWithdrawal / 12
	}

	// Calculate work income per month based on retirement dates
	// Work income is shown for months BEFORE the person's retirement date
	type workMonthInfo struct {
		lastWorkMonth int     // -1 means not working this year, 0-11 for Apr-Mar
		monthlyAmount float64 // Monthly work income (annual / 12)
	}
	workInfoByPerson := make(map[string]workMonthInfo)
	totalAnnualWorkIncome := plan.Summary.WorkIncome

	for _, person := range r.config.People {
		if person.WorkIncome <= 0 {
			continue
		}
		// Get retirement tax year from retirement date
		retireTaxYear := 0
		if person.RetirementDate != "" {
			retireTaxYear = GetRetirementTaxYear(person.RetirementDate)
		}

		if retireTaxYear == 0 || retireTaxYear > plan.Year {
			// Working all year - retirement is next year or later
			workInfoByPerson[person.Name] = workMonthInfo{
				lastWorkMonth: 11, // Works through March
				monthlyAmount: person.WorkIncome / 12,
			}
		} else if retireTaxYear == plan.Year {
			// Retiring this tax year - find the month
			lastMonth := getLastWorkMonth(person.RetirementDate, plan.Year)
			if lastMonth >= 0 {
				workInfoByPerson[person.Name] = workMonthInfo{
					lastWorkMonth: lastMonth,
					monthlyAmount: person.WorkIncome / 12,
				}
			}
		}
		// If retireTaxYear < plan.Year, person already retired - no work income
	}

	// Tax year months (April to March)
	months := []string{"Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec", "Jan", "Feb", "Mar"}

	// Draw table header with full GBP amounts
	colWidths := []float64{18, 20, 20, 22, 22, 20, 20, 38}
	headers := []string{"Month", "Net Needed", "Work Income", "ISA Withdraw", "Pen Tax-Free", "Pen Taxable", "ISA Deposit", "Notes"}

	r.pdf.SetFillColor(70, 90, 110)
	r.pdf.SetTextColor(255, 255, 255)
	r.pdf.SetFont("Arial", "B", 7)

	for i, header := range headers {
		align := "L"
		if i > 0 && i < 7 {
			align = "R"
		}
		r.pdf.CellFormat(colWidths[i], 4, header, "1", 0, align, true, 0, "")
	}
	r.pdf.Ln(-1)

	// Draw monthly rows
	r.pdf.SetFont("Arial", "", 7)
	r.pdf.SetTextColor(50, 50, 50)

	for i, month := range months {
		// Check for page break
		if r.pdf.GetY() > 275 {
			r.pdf.AddPage()
			// Redraw header after page break
			r.pdf.SetFillColor(70, 90, 110)
			r.pdf.SetTextColor(255, 255, 255)
			r.pdf.SetFont("Arial", "B", 7)
			for j, header := range headers {
				align := "L"
				if j > 0 && j < 7 {
					align = "R"
				}
				r.pdf.CellFormat(colWidths[j], 4, header, "1", 0, align, true, 0, "")
			}
			r.pdf.Ln(-1)
			r.pdf.SetFont("Arial", "", 7)
			r.pdf.SetTextColor(50, 50, 50)
		}

		// Alternate row colors
		if i%2 == 0 {
			r.pdf.SetFillColor(250, 250, 250)
		} else {
			r.pdf.SetFillColor(255, 255, 255)
		}

		// Calculate the actual month/year
		monthNum := ((i + 4 - 1) % 12) + 1 // Apr=4, May=5, ..., Mar=3
		year := plan.Year
		if monthNum <= 3 {
			year = plan.Year + 1
		}
		monthLabel := fmt.Sprintf("%s %d", month, year)

		// Determine notes for special months
		notes := ""
		if i == 0 {
			notes = "Start of tax year"
		} else if i == 11 {
			notes = "End of tax year"
			// If there are ISA deposits, remind about the deadline
			if totalISADeposit > 0 {
				notes = "ISA deadline 5 Apr!"
			}
		}

		// Check if this is a pre-pension access month
		isPensionAccessible := firstPensionMonth >= 0 && i >= firstPensionMonth

		// Add note for the month pension access begins
		if firstPensionMonth > 0 && i == firstPensionMonth {
			notes = "Pension access starts"
		} else if firstPensionMonth > 0 && i < firstPensionMonth {
			if notes == "" {
				notes = "Use ISA/savings bridge"
			} else {
				notes += " - ISA bridge"
			}
		}

		// ISA deposit handling - spread across year or lump sum
		monthlyISADeposit := 0.0
		if totalISADeposit > 0 && isPensionAccessible {
			// For pension-to-ISA, deposits only happen when pension is accessible
			monthlyISADeposit = totalISADeposit / float64(pensionMonths)
		}

		// Determine amounts for this month based on pension accessibility
		var thisMonthISA, thisMonthPensionTaxFree, thisMonthPensionTaxable float64

		if isPensionAccessible {
			thisMonthISA = monthlyISAPensionPeriod
			thisMonthPensionTaxFree = monthlyPensionTaxFree
			thisMonthPensionTaxable = monthlyPensionTaxable
		} else {
			thisMonthISA = monthlyISAPrePension
			thisMonthPensionTaxFree = 0
			thisMonthPensionTaxable = 0
		}

		// Calculate work income for this month
		// Sum up work income from all people who are still working in this month
		thisMonthWorkIncome := 0.0
		for _, info := range workInfoByPerson {
			if info.lastWorkMonth >= 0 && i <= info.lastWorkMonth {
				thisMonthWorkIncome += info.monthlyAmount
			}
		}

		// Draw row
		r.pdf.CellFormat(colWidths[0], 4, monthLabel, "1", 0, "L", true, 0, "")
		r.pdf.CellFormat(colWidths[1], 4, formatMonthlyMoney(monthlyNetRequired), "1", 0, "R", true, 0, "")
		r.pdf.CellFormat(colWidths[2], 4, formatMonthlyMoney(thisMonthWorkIncome), "1", 0, "R", true, 0, "")
		r.pdf.CellFormat(colWidths[3], 4, formatMonthlyMoney(thisMonthISA), "1", 0, "R", true, 0, "")
		r.pdf.CellFormat(colWidths[4], 4, formatMonthlyMoney(thisMonthPensionTaxFree), "1", 0, "R", true, 0, "")
		r.pdf.CellFormat(colWidths[5], 4, formatMonthlyMoney(thisMonthPensionTaxable), "1", 0, "R", true, 0, "")
		r.pdf.CellFormat(colWidths[6], 4, formatMonthlyMoney(monthlyISADeposit), "1", 0, "R", true, 0, "")
		r.pdf.CellFormat(colWidths[7], 4, notes, "1", 1, "L", true, 0, "")
	}

	// Add totals row
	r.pdf.SetFont("Arial", "B", 7)
	r.pdf.SetFillColor(230, 235, 240)
	r.pdf.CellFormat(colWidths[0], 4, "TOTAL", "1", 0, "L", true, 0, "")
	r.pdf.CellFormat(colWidths[1], 4, FormatMoneyPDF(yearState.NetRequired), "1", 0, "R", true, 0, "")
	r.pdf.CellFormat(colWidths[2], 4, FormatMoneyPDF(totalAnnualWorkIncome), "1", 0, "R", true, 0, "")
	r.pdf.CellFormat(colWidths[3], 4, FormatMoneyPDF(totalISAWithdrawal), "1", 0, "R", true, 0, "")
	r.pdf.CellFormat(colWidths[4], 4, FormatMoneyPDF(totalPensionTaxFree), "1", 0, "R", true, 0, "")
	r.pdf.CellFormat(colWidths[5], 4, FormatMoneyPDF(totalPensionTaxable), "1", 0, "R", true, 0, "")
	r.pdf.CellFormat(colWidths[6], 4, FormatMoneyPDF(totalISADeposit), "1", 0, "R", true, 0, "")
	r.pdf.CellFormat(colWidths[7], 4, "", "1", 1, "L", true, 0, "")

	// Add ISA contribution instructions if there are deposits
	if totalISADeposit > 0 {
		r.pdf.Ln(2)
		r.pdf.SetFont("Arial", "B", 8)
		r.pdf.SetTextColor(128, 0, 128) // Purple for ISA instructions
		r.pdf.CellFormat(contentWidth, 4, "ISA Contribution Instructions:", "", 1, "L", false, 0, "")

		r.pdf.SetFont("Arial", "", 7)
		r.pdf.SetTextColor(50, 50, 50)

		// Calculate per-person ISA deposits
		for name, amount := range yearState.Withdrawals.ISADeposits {
			if amount > 0 {
				monthlyAmount := amount / 12
				r.pdf.CellFormat(contentWidth, 3.5,
					fmt.Sprintf("- %s: Deposit %s/month (%s total) from pension withdrawals into ISA",
						name, FormatMoneyPDF(monthlyAmount), FormatMoneyPDF(amount)), "", 1, "L", false, 0, "")
			}
		}

		r.pdf.SetFont("Arial", "I", 7)
		r.pdf.SetTextColor(100, 100, 100)
		r.pdf.CellFormat(contentWidth, 3.5,
			"- Contributions must be made by 5 April to use this tax year's allowance (GBP 20,000 per person)", "", 1, "L", false, 0, "")
		r.pdf.CellFormat(contentWidth, 3.5,
			"- Set up standing order from bank account receiving pension income to ISA", "", 1, "L", false, 0, "")
	}
}

// formatMonthlyMoney formats monthly amounts with full GBP values
func formatMonthlyMoney(amount float64) string {
	if amount == 0 {
		return "-"
	}
	return pdfText(formatWithCommas(amount))
}

// drawMortgagePayoffSchedule draws a detailed mortgage payoff schedule
func (r *PDFActionPlanReport) drawMortgagePayoffSchedule(plan YearActionPlan, yearState YearState) {
	// Check for page break before drawing schedule
	if r.pdf.GetY() > 200 {
		r.pdf.AddPage()
	}

	r.pdf.Ln(3)
	r.pdf.SetFont("Arial", "B", 9)
	r.pdf.SetTextColor(180, 100, 0) // Orange for mortgage
	r.pdf.CellFormat(contentWidth, 5, "Mortgage Payoff Schedule", "", 1, "L", false, 0, "")

	// Determine funding source based on strategy
	fundingSource := "Pension/ISA withdrawals"
	if r.result.Params.MortgageOpt == PCLSMortgagePayoff {
		fundingSource = "25% PCLS tax-free lump sum"
	}

	r.pdf.SetFont("Arial", "", 8)
	r.pdf.SetTextColor(50, 50, 50)
	r.pdf.CellFormat(contentWidth, 4, fmt.Sprintf("Funding source: %s", fundingSource), "", 1, "L", false, 0, "")
	r.pdf.Ln(2)

	// Table header
	colWidths := []float64{50, 35, 25, 35, 35}
	headers := []string{"Mortgage Part", "Original", "Rate", "Outstanding", "Action"}

	r.pdf.SetFillColor(180, 100, 0) // Orange header
	r.pdf.SetTextColor(255, 255, 255)
	r.pdf.SetFont("Arial", "B", 7)

	for i, header := range headers {
		align := "L"
		if i > 0 && i < 4 {
			align = "R"
		}
		r.pdf.CellFormat(colWidths[i], 4, header, "1", 0, align, true, 0, "")
	}
	r.pdf.Ln(-1)

	// Calculate outstanding balances for each mortgage part
	r.pdf.SetFont("Arial", "", 7)
	r.pdf.SetTextColor(50, 50, 50)

	totalOutstanding := 0.0
	rowNum := 0
	for _, part := range r.config.Mortgage.Parts {
		if part.Principal <= 0 {
			continue
		}

		// Calculate outstanding balance
		outstanding := part.CalculateRemainingBalance(plan.Year)

		// Alternate row colors
		if rowNum%2 == 0 {
			r.pdf.SetFillColor(255, 248, 240)
		} else {
			r.pdf.SetFillColor(255, 255, 255)
		}
		rowNum++

		mortgageType := "Repayment"
		if !part.IsRepayment {
			mortgageType = "Interest Only"
		}

		r.pdf.CellFormat(colWidths[0], 4, fmt.Sprintf("%s (%s)", part.Name, mortgageType), "1", 0, "L", true, 0, "")
		r.pdf.CellFormat(colWidths[1], 4, FormatMoneyPDF(part.Principal), "1", 0, "R", true, 0, "")
		r.pdf.CellFormat(colWidths[2], 4, fmt.Sprintf("%.2f%%", part.InterestRate*100), "1", 0, "R", true, 0, "")

		if outstanding <= 0 {
			// Mortgage already paid off through normal payments
			endYear := part.StartYear + part.TermYears
			r.pdf.SetTextColor(100, 100, 100) // Grey for already paid
			r.pdf.CellFormat(colWidths[3], 4, pdfText("£0"), "1", 0, "R", true, 0, "")
			r.pdf.CellFormat(colWidths[4], 4, fmt.Sprintf("Paid off in %d", endYear), "1", 1, "L", true, 0, "")
			r.pdf.SetTextColor(50, 50, 50)
		} else {
			totalOutstanding += outstanding
			r.pdf.CellFormat(colWidths[3], 4, FormatMoneyPDF(outstanding), "1", 0, "R", true, 0, "")
			r.pdf.CellFormat(colWidths[4], 4, "Pay off in full", "1", 1, "L", true, 0, "")
		}
	}

	// Total row
	r.pdf.SetFont("Arial", "B", 7)
	r.pdf.SetFillColor(255, 235, 210)
	r.pdf.CellFormat(colWidths[0]+colWidths[1]+colWidths[2], 4, "TOTAL PAYOFF AMOUNT", "1", 0, "R", true, 0, "")
	r.pdf.CellFormat(colWidths[3], 4, FormatMoneyPDF(totalOutstanding), "1", 0, "R", true, 0, "")
	r.pdf.CellFormat(colWidths[4], 4, "", "1", 1, "L", true, 0, "")

	// Action steps
	r.pdf.Ln(2)
	r.pdf.SetFont("Arial", "B", 8)
	r.pdf.SetTextColor(180, 100, 0)
	r.pdf.CellFormat(contentWidth, 4, "Payoff Action Steps:", "", 1, "L", false, 0, "")

	r.pdf.SetFont("Arial", "", 7)
	r.pdf.SetTextColor(50, 50, 50)

	steps := []string{
		"1. Request redemption statement from lender(s) - valid for specific date",
		"2. Note any early repayment charges (ERCs) that may apply",
		"3. Arrange pension withdrawal/PCLS to cover total amount",
		"4. Instruct solicitor or make direct payment as per lender instructions",
		"5. Obtain confirmation of mortgage discharge and Title Deed update",
	}

	for _, step := range steps {
		r.pdf.CellFormat(contentWidth, 3.5, step, "", 1, "L", false, 0, "")
	}

	// Important notes
	r.pdf.Ln(2)
	r.pdf.SetFont("Arial", "I", 7)
	r.pdf.SetTextColor(100, 100, 100)
	r.pdf.MultiCell(contentWidth, 3.5,
		"Note: Redemption figures change daily due to interest accrual. Request a statement close to your intended payoff date. "+
			"Early repayment charges may apply if paying off during a fixed rate period.", "", "L", false)
}

func (r *PDFActionPlanReport) buildYearActionPlan(yearState YearState) YearActionPlan {
	plan := YearActionPlan{
		Year:         yearState.Year,
		TaxYearStart: fmt.Sprintf("6 Apr %d", yearState.Year),
		TaxYearEnd:   fmt.Sprintf("5 Apr %d", yearState.Year+1),
		Ages:         yearState.Ages,
		Actions:      make([]ActionItem, 0),
		Summary: YearSummaryPDF{
			StartingBalance:    yearState.StartBalance,
			TotalIncome:        yearState.TotalRequired,
			TotalWithdrawals:   yearState.Withdrawals.TotalTaxFree + yearState.Withdrawals.TotalTaxable,
			MortgageCost:       yearState.MortgageCost,
			TotalTaxPaid:       yearState.TotalTaxPaid,
			NetIncomeReceived:  yearState.NetIncomeReceived,
			EndingBalance:      yearState.TotalBalance,
			WorkIncome:         yearState.TotalWorkIncome,
			WorkIncomeByPerson: yearState.WorkIncomeByPerson,
		},
	}

	// Check retirement status
	refPerson := r.config.GetReferencePerson()
	refBirthYear := GetBirthYear(refPerson.BirthDate)
	refAge := yearState.Year - refBirthYear
	isRetired := refAge >= refPerson.RetirementAge

	// Add milestone events
	for _, person := range r.config.People {
		birthYear := GetBirthYear(person.BirthDate)
		age := yearState.Year - birthYear

		if age == person.RetirementAge {
			plan.Actions = append(plan.Actions, ActionItem{
				Category:    "Milestone",
				Description: fmt.Sprintf("%s reaches pension access age %d", person.Name, person.RetirementAge),
				Person:      person.Name,
				Notes:       "25% PCLS tax-free lump sum now available",
			})
		}

		if age == person.StatePensionAge {
			plan.Actions = append(plan.Actions, ActionItem{
				Category:    "Income",
				Description: fmt.Sprintf("%s starts State Pension", person.Name),
				Amount:      r.config.Financial.StatePensionAmount,
				Person:      person.Name,
				Notes:       "Contact DWP to claim - not automatic",
			})
		}

		if person.DBPensionAmount > 0 && age == person.DBPensionStartAge {
			plan.Actions = append(plan.Actions, ActionItem{
				Category:    "Income",
				Description: fmt.Sprintf("%s starts %s", person.Name, person.DBPensionName),
				Amount:      person.DBPensionAmount,
				Person:      person.Name,
			})
		}
	}

	// Check if there are any withdrawals this year (regardless of retirement status)
	hasWithdrawals := yearState.Withdrawals.TotalTaxFree > 0 || yearState.Withdrawals.TotalTaxable > 0

	if !isRetired && !hasWithdrawals {
		plan.Actions = append(plan.Actions, ActionItem{
			Category:    "Info",
			Description: "Pre-retirement - no withdrawals required",
			Notes:       "Continue contributions and review investments",
		})
		return plan
	}

	// State pension income
	for name, amount := range yearState.StatePensionByPerson {
		if amount > 0 {
			plan.Actions = append(plan.Actions, ActionItem{
				Category:    "Income",
				Description: fmt.Sprintf("%s State Pension", name),
				Amount:      amount,
				Person:      name,
				Notes:       fmt.Sprintf("%s/month", FormatMoneyPDF(amount/12)),
			})
		}
	}

	// DB pension income
	for name, amount := range yearState.DBPensionByPerson {
		if amount > 0 {
			plan.Actions = append(plan.Actions, ActionItem{
				Category:    "Income",
				Description: fmt.Sprintf("%s DB Pension", name),
				Amount:      amount,
				Person:      name,
			})
		}
	}

	// ISA withdrawals
	for name, amount := range yearState.Withdrawals.TaxFreeFromISA {
		if amount > 0 {
			plan.Actions = append(plan.Actions, ActionItem{
				Category:    "Withdraw",
				Description: fmt.Sprintf("Withdraw from %s ISA (tax-free)", name),
				Amount:      amount,
				Person:      name,
			})
		}
	}

	// Pension tax-free (PCLS/crystallisation)
	for name, amount := range yearState.Withdrawals.TaxFreeFromPension {
		if amount > 0 {
			plan.Actions = append(plan.Actions, ActionItem{
				Category:    "Withdraw",
				Description: fmt.Sprintf("%s pension crystallisation (25%% tax-free)", name),
				Amount:      amount,
				Person:      name,
			})
		}
	}

	// Pension taxable withdrawals
	for name, amount := range yearState.Withdrawals.TaxableFromPension {
		if amount > 0 {
			plan.Actions = append(plan.Actions, ActionItem{
				Category:    "Withdraw",
				Description: fmt.Sprintf("%s pension withdrawal (taxable)", name),
				Amount:      amount,
				Person:      name,
				Notes:       "Tax deducted via PAYE by provider",
			})
		}
	}

	// ISA deposits (PensionToISA strategy)
	for name, amount := range yearState.Withdrawals.ISADeposits {
		if amount > 0 {
			plan.Actions = append(plan.Actions, ActionItem{
				Category:    "Transfer",
				Description: fmt.Sprintf("Transfer to %s ISA", name),
				Amount:      amount,
				Person:      name,
				Notes:       "From excess pension withdrawal",
			})
		}
	}

	// Mortgage
	if yearState.MortgageCost > 0 {
		isPayoff := yearState.MortgageCost > r.config.GetTotalAnnualPayment()*1.5
		if isPayoff {
			plan.Actions = append(plan.Actions, ActionItem{
				Category:    "Mortgage",
				Description: "Pay off mortgage balance",
				Amount:      yearState.MortgageCost,
				Notes:       "Check for early repayment charges",
			})
		} else {
			plan.Actions = append(plan.Actions, ActionItem{
				Category:    "Mortgage",
				Description: "Mortgage payments",
				Amount:      yearState.MortgageCost,
				Notes:       fmt.Sprintf("%s/month", FormatMoneyPDF(yearState.MortgageCost/12)),
			})
		}
	}

	// Tax
	if yearState.TotalTaxPaid > 0 {
		plan.Actions = append(plan.Actions, ActionItem{
			Category:    "Tax",
			Description: "Income tax on pension withdrawals",
			Amount:      yearState.TotalTaxPaid,
			Notes:       fmt.Sprintf("PA: %s, Basic limit: %s", FormatMoneyPDF(yearState.PersonalAllowance), FormatMoneyPDF(yearState.BasicRateLimit)),
		})
	}

	return plan
}

func (r *PDFActionPlanReport) addSummaryPage() {
	r.pdf.AddPage()
	r.drawSectionHeader("Lifetime Summary")

	// Financial Summary Table
	r.pdf.SetFont("Arial", "B", 11)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 7, "Financial Totals", "", 1, "L", false, 0, "")

	totalIncome := 0.0
	for _, year := range r.result.Years {
		totalIncome += year.NetIncomeReceived
	}

	effectiveTaxRate := 0.0
	if r.result.TotalWithdrawn > 0 {
		effectiveTaxRate = r.result.TotalTaxPaid / r.result.TotalWithdrawn * 100
	}

	r.drawTableHeader([]string{"Metric", "Value"}, []float64{100, 80})
	r.drawTableRow([]string{"Total Net Income Received", FormatMoneyPDF(totalIncome)}, []float64{100, 80}, false)
	r.drawTableRow([]string{"Total Tax Paid", FormatMoneyPDF(r.result.TotalTaxPaid)}, []float64{100, 80}, false)
	r.drawTableRow([]string{"Total Withdrawals", FormatMoneyPDF(r.result.TotalWithdrawn)}, []float64{100, 80}, false)
	r.drawTableRow([]string{"Effective Tax Rate", fmt.Sprintf("%.1f%%", effectiveTaxRate)}, []float64{100, 80}, true)

	r.pdf.Ln(8)

	// Final Balances Table
	r.pdf.SetFont("Arial", "B", 11)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 7, "Final Balances", "", 1, "L", false, 0, "")

	r.drawTableHeader([]string{"Person", "ISA", "Pension", "Total"}, []float64{50, 40, 40, 50})

	totalISA := 0.0
	totalPension := 0.0
	for name, bal := range r.result.FinalBalances {
		pension := bal.CrystallisedPot + bal.UncrystallisedPot
		totalISA += bal.TaxFreeSavings
		totalPension += pension
		r.drawTableRow([]string{
			name,
			FormatMoneyPDF(bal.TaxFreeSavings),
			FormatMoneyPDF(pension),
			FormatMoneyPDF(bal.TaxFreeSavings + pension),
		}, []float64{50, 40, 40, 50}, false)
	}
	r.drawTableRow([]string{
		"TOTAL",
		FormatMoneyPDF(totalISA),
		FormatMoneyPDF(totalPension),
		FormatMoneyPDF(totalISA + totalPension),
	}, []float64{50, 40, 40, 50}, true)

	r.pdf.Ln(8)

	// Key Milestones
	r.pdf.SetFont("Arial", "B", 11)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 7, "Key Milestones Timeline", "", 1, "L", false, 0, "")

	r.drawTableHeader([]string{"Person", "Pension Access", "State Pension", "DB Pension"}, []float64{45, 45, 45, 45})

	for _, person := range r.config.People {
		birthYear := GetBirthYear(person.BirthDate)
		dbStr := "-"
		if person.DBPensionAmount > 0 {
			dbStr = fmt.Sprintf("%d (age %d)", birthYear+person.DBPensionStartAge, person.DBPensionStartAge)
		}
		r.drawTableRow([]string{
			person.Name,
			fmt.Sprintf("%d (age %d)", birthYear+person.RetirementAge, person.RetirementAge),
			fmt.Sprintf("%d (age %d)", birthYear+person.StatePensionAge, person.StatePensionAge),
			dbStr,
		}, []float64{45, 45, 45, 45}, false)
	}

	r.pdf.Ln(10)

	// Important Reminders
	r.pdf.SetFont("Arial", "B", 11)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 7, "Important Reminders", "", 1, "L", false, 0, "")

	r.pdf.SetFont("Arial", "", 9)
	r.pdf.SetTextColor(50, 50, 50)

	reminders := []string{
		"Review your strategy annually - tax rules and personal circumstances change",
		"ISA annual allowance is currently GBP 20,000 per person - use it or lose it",
		"State Pension must be claimed - contact the Pension Service, it is not automatic",
		"Keep records of all withdrawals for your tax return",
		"Pension funds on death before 75 can be passed tax-free to beneficiaries",
		"Consider seeking professional financial advice for major decisions",
	}

	for i, reminder := range reminders {
		r.pdf.CellFormat(contentWidth, 5, fmt.Sprintf("%d. %s", i+1, reminder), "", 1, "L", false, 0, "")
	}

	// Footer
	r.pdf.Ln(15)
	r.pdf.SetFont("Arial", "I", 8)
	r.pdf.SetTextColor(128, 128, 128)
	r.pdf.MultiCell(contentWidth, 4,
		"This report was generated by Pension Forecast Simulator. "+
			"Projections are based on the assumptions provided and actual results may vary. "+
			"This is not financial advice.", "", "C", false)
}

// Helper functions

func (r *PDFActionPlanReport) drawSectionHeader(title string) {
	r.pdf.SetFont("Arial", "B", 16)
	r.pdf.SetTextColor(0, 51, 102)
	r.pdf.CellFormat(contentWidth, 10, title, "", 1, "L", false, 0, "")
	r.pdf.SetDrawColor(0, 51, 102)
	r.pdf.Line(marginLeft, r.pdf.GetY(), marginLeft+contentWidth, r.pdf.GetY())
	r.pdf.Ln(5)
}

func (r *PDFActionPlanReport) drawTableHeader(headers []string, widths []float64) {
	r.pdf.SetFillColor(0, 51, 102)
	r.pdf.SetTextColor(255, 255, 255)
	r.pdf.SetFont("Arial", "B", 9)

	for i, header := range headers {
		align := "L"
		if i > 0 {
			align = "R"
		}
		r.pdf.CellFormat(widths[i], 6, header, "1", 0, align, true, 0, "")
	}
	r.pdf.Ln(-1)
}

func (r *PDFActionPlanReport) drawTableRow(cells []string, widths []float64, isBold bool) {
	r.pdf.SetFillColor(250, 250, 250)
	r.pdf.SetTextColor(50, 50, 50)

	if isBold {
		r.pdf.SetFont("Arial", "B", 9)
		r.pdf.SetFillColor(240, 240, 240)
	} else {
		r.pdf.SetFont("Arial", "", 9)
	}

	for i, cell := range cells {
		align := "L"
		if i > 0 {
			align = "R"
		}
		r.pdf.CellFormat(widths[i], 5, cell, "1", 0, align, true, 0, "")
	}
	r.pdf.Ln(-1)
}

func (r *PDFActionPlanReport) setCategoryColor(category string) {
	switch category {
	case "Milestone":
		r.pdf.SetTextColor(0, 128, 0)
	case "Income":
		r.pdf.SetTextColor(0, 100, 50)
	case "Withdraw":
		r.pdf.SetTextColor(0, 0, 180)
	case "Transfer":
		r.pdf.SetTextColor(128, 0, 128)
	case "Mortgage":
		r.pdf.SetTextColor(180, 100, 0)
	case "Tax":
		r.pdf.SetTextColor(180, 0, 0)
	case "Info":
		r.pdf.SetTextColor(80, 80, 80)
	default:
		r.pdf.SetTextColor(50, 50, 50)
	}
}

func (r *PDFActionPlanReport) getStrategyDescription() string {
	var desc string

	// Use consistent naming with UI
	switch r.result.Params.DrawdownOrder {
	case SavingsFirst:
		desc = "ISA First, Then Pension: Withdraw from tax-free ISA savings first, preserving pension for later growth."
	case PensionFirst:
		desc = "Pension First, Then ISA: Withdraw from pension pots first, preserving ISA for later or inheritance."
	case TaxOptimized:
		desc = "Tax Optimized Withdrawals: Dynamically balance withdrawals between ISA and pension to minimize tax paid."
	case PensionToISA:
		desc = "Combined ISA And Pension: Over-withdraw from pension to fill tax bands, transferring excess to ISA for tax-free growth."
	case PensionToISAProactive:
		desc = "Combined ISA And Pension (Proactive): Aggressively move pension to ISA while working to maximize tax-free growth."
	case PensionOnly:
		desc = "Pension Only: Only withdraw from pension, preserving ISA completely for inheritance."
	case FillBasicRate:
		desc = "Fill Basic Rate: Draw pension up to the basic rate tax threshold each year."
	case StatePensionBridge:
		desc = "State Pension Bridge: Use pension to bridge income gap until state pension begins."
	}

	// Only add mortgage description if there is a mortgage
	if r.config.HasMortgage() {
		mortgagePayoffYear := getMortgagePayoffYear(r.config, r.result.Params)
		switch r.result.Params.MortgageOpt {
		case MortgageEarly:
			desc += fmt.Sprintf(" Mortgage repaid %d.", mortgagePayoffYear)
		case MortgageExtended:
			desc += fmt.Sprintf(" Mortgage extended to %d.", mortgagePayoffYear)
		case PCLSMortgagePayoff:
			desc += fmt.Sprintf(" Using PCLS lump sum for mortgage in %d.", mortgagePayoffYear)
		case MortgageNormal:
			desc += fmt.Sprintf(" Mortgage repaid %d.", mortgagePayoffYear)
		}
	}

	// Add withdrawal adjustment strategies
	if r.result.Params.GuardrailsEnabled {
		desc += " Using Guyton-Klinger Guardrails for dynamic withdrawal adjustments."
	}

	// Add state pension deferral if applicable
	if r.result.Params.StatePensionDeferYears > 0 {
		desc += fmt.Sprintf(" State pension deferred by %d years (+%.1f%% enhancement).",
			r.result.Params.StatePensionDeferYears,
			float64(r.result.Params.StatePensionDeferYears)*5.8)
	}

	// Add ISA to SIPP transfer if enabled
	if r.result.Params.ISAToSIPPEnabled {
		desc += " Transferring ISA to pension while working for tax relief."
	}

	return desc
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// getDrawdownOrderName returns a human-readable name for the drawdown order
func (r *PDFActionPlanReport) getDrawdownOrderName() string {
	name, _ := r.getDrawdownOrderNameWithExplanation()
	return name
}

func (r *PDFActionPlanReport) getDrawdownOrderNameWithExplanation() (string, string) {
	switch r.result.Params.DrawdownOrder {
	case SavingsFirst:
		return "ISA First, Then Pension",
			"Deplete ISA savings first, preserving pension for later (pension benefits from tax-free growth)"
	case PensionFirst:
		return "Pension First, Then ISA",
			"Draw from pension first, preserving ISA (ISA withdrawals are always tax-free)"
	case TaxOptimized:
		return "Tax Optimized",
			"Dynamically choose withdrawals to minimize tax each year based on income and tax bands"
	case PensionToISA:
		return "Pension to ISA Transfer",
			"Over-withdraw from pension to fill ISA allowance, building tax-free savings for later"
	case PensionToISAProactive:
		return "Pension to ISA (Proactive)",
			"Aggressively transfer pension to ISA even when not needed for income"
	case PensionOnly:
		return "Pension Only",
			"Only withdraw from pension, preserving ISA entirely for inheritance or emergencies"
	case FillBasicRate:
		return "Fill Basic Rate Band",
			"Withdraw from pension up to basic rate threshold each year to use tax-efficient allowances"
	case StatePensionBridge:
		return "State Pension Bridge",
			"Higher withdrawals before state pension starts, then reduce when state pension begins"
	default:
		return "Standard", "Default withdrawal strategy"
	}
}

// getDepletionYears calculates when ISA and Pension funds are depleted
func (r *PDFActionPlanReport) getDepletionYears() (isaYear int, pensionYear int) {
	prevISATotal := 0.0
	prevPensionTotal := 0.0

	for i, year := range r.result.Years {
		// Calculate totals for this year
		isaTotal := 0.0
		pensionTotal := 0.0
		for _, bal := range year.EndBalances {
			isaTotal += bal.TaxFreeSavings
			pensionTotal += bal.CrystallisedPot + bal.UncrystallisedPot
		}

		if i > 0 {
			// Check for ISA depletion (crossed from >0 to <=0)
			if isaYear == 0 && prevISATotal > 100 && isaTotal < 100 {
				isaYear = year.Year
			}
			// Check for Pension depletion
			if pensionYear == 0 && prevPensionTotal > 100 && pensionTotal < 100 {
				pensionYear = year.Year
			}
		}

		prevISATotal = isaTotal
		prevPensionTotal = pensionTotal
	}

	// In depletion mode, if we reached the end with very low balance, mark depletion at last year
	if len(r.result.Years) > 0 {
		lastYear := r.result.Years[len(r.result.Years)-1]
		lastISA := 0.0
		lastPension := 0.0
		for _, bal := range lastYear.EndBalances {
			lastISA += bal.TaxFreeSavings
			lastPension += bal.CrystallisedPot + bal.UncrystallisedPot
		}
		// If balance is very low at end (near zero), mark as depleted that year
		if isaYear == 0 && lastISA < 1000 && prevISATotal > 1000 {
			isaYear = lastYear.Year
		}
		if pensionYear == 0 && lastPension < 1000 && prevPensionTotal > 1000 {
			pensionYear = lastYear.Year
		}
	}

	return isaYear, pensionYear
}

// getActualIncomeFromSimulation extracts actual income requirements from simulation years
// This is needed for depletion mode where config values are 0
func (r *PDFActionPlanReport) getActualIncomeFromSimulation() (monthlyBefore, monthlyAfter float64) {
	if len(r.result.Years) == 0 {
		return 0, 0
	}

	refPerson := r.config.GetReferencePerson()
	refBirthYear := GetBirthYear(refPerson.BirthDate)
	thresholdAge := r.config.IncomeRequirements.AgeThreshold

	// Find income for phase 1 (before threshold) and phase 2 (after threshold)
	for _, year := range r.result.Years {
		refAge := year.Year - refBirthYear
		if monthlyBefore == 0 && refAge < thresholdAge && year.RequiredIncome > 0 {
			monthlyBefore = year.RequiredIncome / 12
		}
		if monthlyAfter == 0 && refAge >= thresholdAge && year.RequiredIncome > 0 {
			monthlyAfter = year.RequiredIncome / 12
		}
		if monthlyBefore > 0 && monthlyAfter > 0 {
			break
		}
	}

	// If we only found one phase, use it for both
	if monthlyBefore == 0 && monthlyAfter > 0 {
		monthlyBefore = monthlyAfter
	}
	if monthlyAfter == 0 && monthlyBefore > 0 {
		monthlyAfter = monthlyBefore
	}

	return monthlyBefore, monthlyAfter
}

// getIncomeRequirements returns the income values, using simulation data for depletion mode
func (r *PDFActionPlanReport) getIncomeRequirements() (monthlyBefore, monthlyAfter float64) {
	monthlyBefore = r.config.IncomeRequirements.MonthlyBeforeAge
	monthlyAfter = r.config.IncomeRequirements.MonthlyAfterAge

	// In depletion mode, config values are 0 - extract from simulation
	if monthlyBefore == 0 && monthlyAfter == 0 {
		monthlyBefore, monthlyAfter = r.getActualIncomeFromSimulation()
	}

	return monthlyBefore, monthlyAfter
}

// getIncomeRequirementLines returns formatted strings describing income requirements for PDF display
func (r *PDFActionPlanReport) getIncomeRequirementLines(startYear, endYear, refBirthYear int) []string {
	var lines []string

	// Calculate initial portfolio for percentage display
	initialPortfolio := 0.0
	for _, p := range r.config.People {
		initialPortfolio += p.Pension + p.TaxFreeSavings
	}

	if r.config.IncomeRequirements.HasTiers() {
		// Tiered income system
		for _, tier := range r.config.IncomeRequirements.Tiers {
			// Build age range and date range
			var ageRange, dateRange string
			tierStartYear := startYear
			tierEndYear := endYear + 1

			if tier.StartAge != nil {
				tierStartYear = refBirthYear + *tier.StartAge
			}
			if tier.EndAge != nil {
				tierEndYear = refBirthYear + *tier.EndAge
			}

			if tier.StartAge == nil && tier.EndAge != nil {
				ageRange = fmt.Sprintf("until age %d", *tier.EndAge)
				dateRange = fmt.Sprintf("6 Apr %d to 5 Apr %d", startYear, tierEndYear)
			} else if tier.StartAge != nil && tier.EndAge == nil {
				ageRange = fmt.Sprintf("age %d onwards", *tier.StartAge)
				dateRange = fmt.Sprintf("6 Apr %d to 5 Apr %d", tierStartYear, endYear+1)
			} else if tier.StartAge != nil && tier.EndAge != nil {
				ageRange = fmt.Sprintf("age %d to %d", *tier.StartAge, *tier.EndAge)
				dateRange = fmt.Sprintf("6 Apr %d to 5 Apr %d", tierStartYear, tierEndYear)
			} else {
				ageRange = "all ages"
				dateRange = fmt.Sprintf("6 Apr %d to 5 Apr %d", startYear, endYear+1)
			}

			// Build amount description
			var amount string
			if tier.IsInvestmentGains {
				amount = "Real Returns (growth - inflation)"
			} else if tier.IsPercentage {
				monthly := initialPortfolio * (tier.MonthlyAmount / 100.0) / 12.0
				amount = fmt.Sprintf("%.1f%% of portfolio (%s/month)", tier.MonthlyAmount, FormatMoneyPDF(monthly))
			} else {
				amount = fmt.Sprintf("%s/month", FormatMoneyPDF(tier.MonthlyAmount))
			}

			lines = append(lines, fmt.Sprintf("%s - %s (%s)", amount, ageRange, dateRange))
		}
	} else {
		// Legacy 2-phase income
		thresholdYear := refBirthYear + r.config.IncomeRequirements.AgeThreshold
		monthlyBefore, monthlyAfter := r.getIncomeRequirements()

		lines = append(lines, fmt.Sprintf("%s/month until age %d (6 Apr %d to 5 Apr %d)",
			FormatMoneyPDF(monthlyBefore),
			r.config.IncomeRequirements.AgeThreshold,
			startYear, thresholdYear))
		lines = append(lines, fmt.Sprintf("%s/month age %d+ (6 Apr %d to 5 Apr %d)",
			FormatMoneyPDF(monthlyAfter),
			r.config.IncomeRequirements.AgeThreshold,
			thresholdYear, endYear+1))
	}

	return lines
}
