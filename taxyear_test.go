package main

import "testing"

func TestTaxYearLabel(t *testing.T) {
	tests := []struct {
		year     int
		expected string
	}{
		{2024, "2024/25"},
		{2025, "2025/26"},
		{2026, "2026/27"},
		{2030, "2030/31"},
		{2099, "2099/00"},
		{2000, "2000/01"},
	}

	for _, tt := range tests {
		result := TaxYearLabel(tt.year)
		if result != tt.expected {
			t.Errorf("TaxYearLabel(%d) = %s; want %s", tt.year, result, tt.expected)
		}
	}
}

func TestTaxYearLabelShort(t *testing.T) {
	tests := []struct {
		year     int
		expected string
	}{
		{2024, "24/25"},
		{2025, "25/26"},
		{2026, "26/27"},
		{2099, "99/00"},
		{2000, "00/01"},
	}

	for _, tt := range tests {
		result := TaxYearLabelShort(tt.year)
		if result != tt.expected {
			t.Errorf("TaxYearLabelShort(%d) = %s; want %s", tt.year, result, tt.expected)
		}
	}
}

func TestGetAgeInTaxYear(t *testing.T) {
	// Test cases based on UK tax year (April 6 - April 5)
	tests := []struct {
		name         string
		birthDate    string
		taxYearStart int
		expectedAge  int
	}{
		// Person born July 15, 1971
		// Tax year 2026/27 (Apr 6, 2026 to Apr 5, 2027)
		// They turn 55 on July 15, 2026 - within the tax year
		{"July birthday - turns 55 during TY 2026/27", "1971-07-15", 2026, 55},

		// Person born February 10, 1971
		// Tax year 2026/27 (Apr 6, 2026 to Apr 5, 2027)
		// They turn 56 on Feb 10, 2027 - within the tax year (Jan-Mar part)
		{"February birthday - turns 56 during TY 2026/27", "1971-02-10", 2026, 56},

		// Person born April 1, 1971
		// Tax year 2026/27 (Apr 6, 2026 to Apr 5, 2027)
		// They turn 56 on April 1, 2027 - within the tax year (before Apr 5)
		{"April 1 birthday - turns 56 during TY 2026/27", "1971-04-01", 2026, 56},

		// Person born April 6, 1971
		// Tax year 2026/27 (Apr 6, 2026 to Apr 5, 2027)
		// They turn 55 on April 6, 2026 - first day of tax year
		{"April 6 birthday - turns 55 on first day of TY 2026/27", "1971-04-06", 2026, 55},

		// Person born April 7, 1971
		// Tax year 2026/27 (Apr 6, 2026 to Apr 5, 2027)
		// They turn 55 on April 7, 2026 - second day of tax year
		{"April 7 birthday - turns 55 during TY 2026/27", "1971-04-07", 2026, 55},

		// Person born December 25, 1971
		// Tax year 2026/27 (Apr 6, 2026 to Apr 5, 2027)
		// They turn 55 on December 25, 2026
		{"December birthday - turns 55 during TY 2026/27", "1971-12-25", 2026, 55},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAgeInTaxYear(tt.birthDate, tt.taxYearStart)
			if result != tt.expectedAge {
				t.Errorf("GetAgeInTaxYear(%s, %d) = %d; want %d",
					tt.birthDate, tt.taxYearStart, result, tt.expectedAge)
			}
		})
	}
}

func TestGetTaxYearForAge(t *testing.T) {
	tests := []struct {
		name         string
		birthDate    string
		targetAge    int
		expectedYear int
	}{
		// Person born July 15, 1971, wants to know when they turn 55
		// They turn 55 on July 15, 2026
		// July 15 is after April 6, so tax year 2026/27 (starts 2026)
		{"July birthday - age 55", "1971-07-15", 55, 2026},

		// Person born February 10, 1971, wants to know when they turn 55
		// They turn 55 on February 10, 2026
		// February 10 is before April 6, so tax year 2025/26 (starts 2025)
		{"February birthday - age 55", "1971-02-10", 55, 2025},

		// Person born April 6, 1971
		// They turn 55 on April 6, 2026
		// April 6 is exactly on tax year boundary, tax year 2026/27 (starts 2026)
		{"April 6 birthday - age 55", "1971-04-06", 55, 2026},

		// Person born April 5, 1971
		// They turn 55 on April 5, 2026
		// April 5 is the last day of tax year 2025/26 (starts 2025)
		{"April 5 birthday - age 55", "1971-04-05", 55, 2025},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTaxYearForAge(tt.birthDate, tt.targetAge)
			if result != tt.expectedYear {
				t.Errorf("GetTaxYearForAge(%s, %d) = %d; want %d",
					tt.birthDate, tt.targetAge, result, tt.expectedYear)
			}
		})
	}
}

func TestGetAgeAtTaxYearStart(t *testing.T) {
	tests := []struct {
		name         string
		birthDate    string
		taxYearStart int
		expectedAge  int
	}{
		// Person born July 15, 1971
		// On April 6, 2026, they're still 54 (birthday not until July 15)
		{"July birthday - age at start of TY 2026/27", "1971-07-15", 2026, 54},

		// Person born February 10, 1971
		// On April 6, 2026, they're 55 (birthday was Feb 10)
		{"February birthday - age at start of TY 2026/27", "1971-02-10", 2026, 55},

		// Person born April 6, 1971
		// On April 6, 2026, they just turned 55
		{"April 6 birthday - age at start of TY 2026/27", "1971-04-06", 2026, 55},

		// Person born April 7, 1971
		// On April 6, 2026, they're still 54 (birthday tomorrow)
		{"April 7 birthday - age at start of TY 2026/27", "1971-04-07", 2026, 54},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetAgeAtTaxYearStart(tt.birthDate, tt.taxYearStart)
			if result != tt.expectedAge {
				t.Errorf("GetAgeAtTaxYearStart(%s, %d) = %d; want %d",
					tt.birthDate, tt.taxYearStart, result, tt.expectedAge)
			}
		})
	}
}

func TestGetRetirementTaxYear(t *testing.T) {
	tests := []struct {
		name           string
		retirementDate string
		expectedYear   int
	}{
		// Retirement on July 15, 2026 - tax year 2026/27
		{"July 2026 - TY 2026/27", "2026-07-15", 2026},

		// Retirement on February 10, 2026 - tax year 2025/26 (before April 6)
		{"February 2026 - TY 2025/26", "2026-02-10", 2025},

		// Retirement on April 6, 2026 - tax year 2026/27 (first day)
		{"April 6 2026 - TY 2026/27", "2026-04-06", 2026},

		// Retirement on April 5, 2026 - tax year 2025/26 (last day)
		{"April 5 2026 - TY 2025/26", "2026-04-05", 2025},

		// Retirement on December 25, 2026 - tax year 2026/27
		{"December 2026 - TY 2026/27", "2026-12-25", 2026},

		// Retirement on January 15, 2027 - tax year 2026/27 (Jan-Mar of next year)
		{"January 2027 - TY 2026/27", "2027-01-15", 2026},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetRetirementTaxYear(tt.retirementDate)
			if result != tt.expectedYear {
				t.Errorf("GetRetirementTaxYear(%s) = %d; want %d",
					tt.retirementDate, result, tt.expectedYear)
			}
		})
	}
}

func TestGetEffectiveRetirementAge(t *testing.T) {
	tests := []struct {
		name           string
		birthDate      string
		retirementDate string
		expectedAge    int
	}{
		// Born July 15, 1971, retire July 15, 2026 (exactly 55)
		{"Exact 55th birthday", "1971-07-15", "2026-07-15", 55},

		// Born July 15, 1971, retire April 6, 2026 (still 54)
		{"Before 55th birthday", "1971-07-15", "2026-04-06", 54},

		// Born July 15, 1971, retire December 25, 2026 (55)
		{"After 55th birthday", "1971-07-15", "2026-12-25", 55},

		// Born October 23, 1973, retire October 23, 2030 (exactly 57)
		{"Exact 57th birthday", "1973-10-23", "2030-10-23", 57},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetEffectiveRetirementAge(tt.birthDate, tt.retirementDate)
			if result != tt.expectedAge {
				t.Errorf("GetEffectiveRetirementAge(%s, %s) = %d; want %d",
					tt.birthDate, tt.retirementDate, result, tt.expectedAge)
			}
		})
	}
}

func TestGetPensionAccessMonth(t *testing.T) {
	// Tax year months: Apr=0, May=1, Jun=2, Jul=3, Aug=4, Sep=5, Oct=6, Nov=7, Dec=8, Jan=9, Feb=10, Mar=11
	tests := []struct {
		name             string
		birthDate        string
		pensionAccessAge int
		taxYear          int
		expectedMonth    int // -1 = not this year, 0 = already accessible, 1-11 = specific month
	}{
		// James born July 15, 1971, pension access age 55
		// Turns 55 on July 15, 2026 - that's month 3 (Jul) of tax year 2026/27
		{"July birthday - access in Jul 2026", "1971-07-15", 55, 2026, 3},

		// Same person in tax year 2027/28 - already had access
		{"July birthday - already accessible 2027", "1971-07-15", 55, 2027, 0},

		// Same person in tax year 2025/26 - not yet accessible
		{"July birthday - not yet accessible 2025", "1971-07-15", 55, 2025, -1},

		// Delphine born October 23, 1973, pension access age 57
		// Turns 57 on October 23, 2030 - that's month 6 (Oct) of tax year 2030/31
		{"October birthday - access in Oct 2030", "1973-10-23", 57, 2030, 6},

		// Person born February 15, turns 55 on Feb 15, 2027
		// In tax year 2026/27, Feb is month 10
		{"February birthday - access in Feb 2027", "1972-02-15", 55, 2026, 10},

		// Person born April 6 - first day of tax year
		// Turns 55 on April 6, 2026 - month 0 of tax year 2026/27
		{"April 6 birthday - access from start", "1971-04-06", 55, 2026, 0},

		// Person born April 5 - last day before tax year
		// Turns 55 on April 5, 2026 - this is in tax year 2025/26
		// So in tax year 2026/27, they already have access (turned 55 in previous TY)
		{"April 5 birthday - already accessible", "1971-04-05", 55, 2026, 0},

		// Person born December 25, pension access 55
		// Turns 55 on Dec 25, 2026 - month 8 of tax year 2026/27
		{"December birthday - access in Dec", "1971-12-25", 55, 2026, 8},

		// Person born January 15, pension access 55
		// Turns 55 on Jan 15, 2027 - month 9 of tax year 2026/27
		{"January birthday - access in Jan", "1972-01-15", 55, 2026, 9},

		// Person born March 31, pension access 55
		// Turns 55 on Mar 31, 2027 - month 11 of tax year 2026/27
		{"March birthday - access in Mar", "1972-03-31", 55, 2026, 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPensionAccessMonth(tt.birthDate, tt.pensionAccessAge, tt.taxYear)
			if result != tt.expectedMonth {
				t.Errorf("getPensionAccessMonth(%s, %d, %d) = %d; want %d",
					tt.birthDate, tt.pensionAccessAge, tt.taxYear, result, tt.expectedMonth)
			}
		})
	}
}
