package main

// ReturnPeriod represents historical returns for a specific time period
type ReturnPeriod struct {
	Years  int     // Number of years in this period
	Label  string  // Human-readable label (e.g., "3 Year", "Since 1984")
	Return float64 // Annualized return as decimal (0.10 = 10%)
}

// StockIndex represents a stock market index with historical return data
type StockIndex struct {
	ID            string         // Unique identifier (e.g., "ftse100")
	Name          string         // Full name (e.g., "FTSE 100")
	ShortName     string         // Short display name
	Country       string         // Country/region (e.g., "UK", "US", "Global")
	Returns       []ReturnPeriod // Returns over different time periods
	DefaultReturn float64        // Default/long-term return to use
	Volatility    string         // "low", "medium", "high"
	Description   string         // Brief description
	InceptionYear int            // Year the index was created
}

// StockIndices contains all available stock market indices
// Data as of end 2024
// Sources: MSCI, FTSE Russell, S&P Dow Jones Indices, Bloomberg, Morningstar
// Note: Returns are nominal (not inflation-adjusted). Real returns typically 2-3% lower.
// Past performance does not guarantee future results.
var StockIndices = []StockIndex{
	// UK Indices
	{
		ID:        "ftse100",
		Name:      "FTSE 100",
		ShortName: "FTSE 100",
		Country:   "UK",
		Returns: []ReturnPeriod{
			{Years: 3, Label: "3 Year", Return: 0.082},
			{Years: 5, Label: "5 Year", Return: 0.058},
			{Years: 10, Label: "10 Year", Return: 0.056},
			{Years: 25, Label: "25 Year", Return: 0.052},
			{Years: 40, Label: "Since 1984", Return: 0.074},
		},
		DefaultReturn: 0.074,
		Volatility:    "medium",
		Description:   "UK large cap - top 100 companies",
		InceptionYear: 1984,
	},
	{
		ID:        "ftse250",
		Name:      "FTSE 250",
		ShortName: "FTSE 250",
		Country:   "UK",
		Returns: []ReturnPeriod{
			{Years: 3, Label: "3 Year", Return: 0.045},
			{Years: 5, Label: "5 Year", Return: 0.052},
			{Years: 10, Label: "10 Year", Return: 0.068},
			{Years: 25, Label: "25 Year", Return: 0.085},
			{Years: 32, Label: "Since 1992", Return: 0.095},
		},
		DefaultReturn: 0.095,
		Volatility:    "medium",
		Description:   "UK mid cap - companies ranked 101-350",
		InceptionYear: 1992,
	},
	{
		ID:        "ftseAim100",
		Name:      "FTSE AIM 100",
		ShortName: "AIM 100",
		Country:   "UK",
		Returns: []ReturnPeriod{
			{Years: 3, Label: "3 Year", Return: -0.08},
			{Years: 5, Label: "5 Year", Return: -0.02},
			{Years: 10, Label: "10 Year", Return: 0.02},
			{Years: 19, Label: "Since 2005", Return: 0.03},
		},
		DefaultReturn: 0.03,
		Volatility:    "high",
		Description:   "UK small cap - Alternative Investment Market",
		InceptionYear: 2005,
	},
	{
		ID:        "ftseAllShare",
		Name:      "FTSE All-Share",
		ShortName: "All-Share",
		Country:   "UK",
		Returns: []ReturnPeriod{
			{Years: 3, Label: "3 Year", Return: 0.075},
			{Years: 5, Label: "5 Year", Return: 0.055},
			{Years: 10, Label: "10 Year", Return: 0.058},
			{Years: 25, Label: "25 Year", Return: 0.055},
			{Years: 62, Label: "Since 1962", Return: 0.078},
		},
		DefaultReturn: 0.078,
		Volatility:    "medium",
		Description:   "Broad UK market - ~600 companies",
		InceptionYear: 1962,
	},

	// US Indices
	{
		ID:        "sp500",
		Name:      "S&P 500",
		ShortName: "S&P 500",
		Country:   "US",
		Returns: []ReturnPeriod{
			{Years: 3, Label: "3 Year", Return: 0.089},
			{Years: 5, Label: "5 Year", Return: 0.145},
			{Years: 10, Label: "10 Year", Return: 0.128},
			{Years: 25, Label: "25 Year", Return: 0.078},
			{Years: 67, Label: "Since 1957", Return: 0.104},
		},
		DefaultReturn: 0.104,
		Volatility:    "medium",
		Description:   "US large cap - 500 largest companies",
		InceptionYear: 1957,
	},
	{
		ID:        "nasdaq",
		Name:      "NASDAQ Composite",
		ShortName: "NASDAQ",
		Country:   "US",
		Returns: []ReturnPeriod{
			{Years: 3, Label: "3 Year", Return: 0.092},
			{Years: 5, Label: "5 Year", Return: 0.188},
			{Years: 10, Label: "10 Year", Return: 0.165},
			{Years: 25, Label: "25 Year", Return: 0.095},
			{Years: 53, Label: "Since 1971", Return: 0.105},
		},
		DefaultReturn: 0.105,
		Volatility:    "high",
		Description:   "US tech-heavy - higher growth, more volatile",
		InceptionYear: 1971,
	},
	{
		ID:        "dowJones",
		Name:      "Dow Jones Industrial Average",
		ShortName: "Dow Jones",
		Country:   "US",
		Returns: []ReturnPeriod{
			{Years: 3, Label: "3 Year", Return: 0.072},
			{Years: 5, Label: "5 Year", Return: 0.105},
			{Years: 10, Label: "10 Year", Return: 0.108},
			{Years: 25, Label: "25 Year", Return: 0.072},
			{Years: 128, Label: "Since 1896", Return: 0.075},
		},
		DefaultReturn: 0.075,
		Volatility:    "medium",
		Description:   "US blue chip - 30 large industrial companies",
		InceptionYear: 1896,
	},

	// European Indices
	{
		ID:        "dax",
		Name:      "Xetra DAX",
		ShortName: "DAX",
		Country:   "Germany",
		Returns: []ReturnPeriod{
			{Years: 3, Label: "3 Year", Return: 0.095},
			{Years: 5, Label: "5 Year", Return: 0.092},
			{Years: 10, Label: "10 Year", Return: 0.078},
			{Years: 25, Label: "25 Year", Return: 0.055},
			{Years: 36, Label: "Since 1988", Return: 0.080},
		},
		DefaultReturn: 0.080,
		Volatility:    "medium",
		Description:   "German large cap - 40 largest companies",
		InceptionYear: 1988,
	},

	// Asian Indices
	{
		ID:        "nikkei225",
		Name:      "Nikkei 225",
		ShortName: "Nikkei",
		Country:   "Japan",
		Returns: []ReturnPeriod{
			{Years: 3, Label: "3 Year", Return: 0.145},
			{Years: 5, Label: "5 Year", Return: 0.128},
			{Years: 10, Label: "10 Year", Return: 0.105},
			{Years: 25, Label: "25 Year", Return: 0.055},
			{Years: 74, Label: "Since 1950", Return: 0.045},
		},
		DefaultReturn: 0.045,
		Volatility:    "medium",
		Description:   "Japanese large cap - 225 companies (includes lost decades)",
		InceptionYear: 1950,
	},
	{
		ID:        "hangSeng",
		Name:      "Hang Seng Index",
		ShortName: "Hang Seng",
		Country:   "Hong Kong",
		Returns: []ReturnPeriod{
			{Years: 3, Label: "3 Year", Return: -0.05},
			{Years: 5, Label: "5 Year", Return: -0.02},
			{Years: 10, Label: "10 Year", Return: 0.015},
			{Years: 25, Label: "25 Year", Return: 0.045},
			{Years: 55, Label: "Since 1969", Return: 0.090},
		},
		DefaultReturn: 0.090,
		Volatility:    "high",
		Description:   "Hong Kong large cap - recent China concerns",
		InceptionYear: 1969,
	},

	// Global Indices
	{
		ID:        "msciWorld",
		Name:      "MSCI World",
		ShortName: "MSCI World",
		Country:   "Global",
		Returns: []ReturnPeriod{
			{Years: 3, Label: "3 Year", Return: 0.082},
			{Years: 5, Label: "5 Year", Return: 0.125},
			{Years: 10, Label: "10 Year", Return: 0.102},
			{Years: 25, Label: "25 Year", Return: 0.072},
			{Years: 54, Label: "Since 1970", Return: 0.085},
		},
		DefaultReturn: 0.085,
		Volatility:    "medium",
		Description:   "Developed markets - ~1,500 companies, 23 countries",
		InceptionYear: 1970,
	},
	{
		ID:        "ftseAllWorld",
		Name:      "FTSE All-World",
		ShortName: "All-World",
		Country:   "Global",
		Returns: []ReturnPeriod{
			{Years: 3, Label: "3 Year", Return: 0.078},
			{Years: 5, Label: "5 Year", Return: 0.115},
			{Years: 10, Label: "10 Year", Return: 0.095},
			{Years: 24, Label: "Since 2000", Return: 0.080},
		},
		DefaultReturn: 0.080,
		Volatility:    "medium",
		Description:   "Global all markets - ~4,000 companies, 50 countries",
		InceptionYear: 2000,
	},
}

// GetStockIndexByID returns a stock index by its ID, or nil if not found
func GetStockIndexByID(id string) *StockIndex {
	for i := range StockIndices {
		if StockIndices[i].ID == id {
			return &StockIndices[i]
		}
	}
	return nil
}

// GetReturnForPeriod returns the return for a specific period, or the default return if not found
func GetReturnForPeriod(index *StockIndex, years int) float64 {
	for _, r := range index.Returns {
		if r.Years == years {
			return r.Return
		}
	}
	return index.DefaultReturn
}

// GetIndicesByRegion groups indices by their country/region
func GetIndicesByRegion() map[string][]StockIndex {
	result := make(map[string][]StockIndex)
	for _, idx := range StockIndices {
		region := idx.Country
		// Group European countries under "Europe"
		if region == "Germany" {
			region = "Europe"
		}
		// Group Asian countries under "Asia"
		if region == "Japan" || region == "Hong Kong" {
			region = "Asia"
		}
		result[region] = append(result[region], idx)
	}
	return result
}

// GetAllReturnPeriods returns all unique return periods available across indices
func GetAllReturnPeriods() []int {
	seen := make(map[int]bool)
	periods := []int{}

	for _, idx := range StockIndices {
		for _, r := range idx.Returns {
			if !seen[r.Years] {
				seen[r.Years] = true
				periods = append(periods, r.Years)
			}
		}
	}

	// Sort periods
	for i := 0; i < len(periods)-1; i++ {
		for j := i + 1; j < len(periods); j++ {
			if periods[i] > periods[j] {
				periods[i], periods[j] = periods[j], periods[i]
			}
		}
	}

	return periods
}
