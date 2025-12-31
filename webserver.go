package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// WebServer holds the HTTP server configuration
type WebServer struct {
	config   *Config
	addr     string
	template *template.Template
}

// NewWebServer creates a new web server instance
func NewWebServer(config *Config, addr string) *WebServer {
	return &WebServer{
		config: config,
		addr:   addr,
	}
}

// APISimulationRequest represents a request to run a simulation
type APISimulationRequest struct {
	Mode              string   `json:"mode"`               // "fixed", "depletion", "pension-only", "pension-to-isa"
	OptimizationGoal  string   `json:"optimization_goal"`  // "tax", "income", "balance"
	People            []PersonConfig `json:"people"`
	Financial         FinancialConfig `json:"financial"`
	IncomeRequirements IncomeConfig `json:"income_requirements"`
	Mortgage          MortgageConfig `json:"mortgage"`
	Simulation        SimulationConfig `json:"simulation"`
	TaxBands          []TaxBand `json:"tax_bands,omitempty"`
	Tax               TaxConfig `json:"tax,omitempty"` // Personal allowance tapering settings
}

// APISimulationResponse represents the simulation results
type APISimulationResponse struct {
	Success bool                `json:"success"`
	Error   string              `json:"error,omitempty"`
	Results []APIResultSummary  `json:"results,omitempty"`
	Best    *APIResultSummary   `json:"best,omitempty"`
	// Growth decline info (if enabled)
	GrowthDecline *GrowthDeclineInfo `json:"growth_decline,omitempty"`
}

// GrowthDeclineInfo describes how growth rates decline over time
type GrowthDeclineInfo struct {
	Enabled          bool    `json:"enabled"`
	PensionStartRate float64 `json:"pension_start_rate"`
	PensionEndRate   float64 `json:"pension_end_rate"`
	SavingsStartRate float64 `json:"savings_start_rate"`
	SavingsEndRate   float64 `json:"savings_end_rate"`
	StartYear        int     `json:"start_year"`
	EndYear          int     `json:"end_year"`
	ReferencePerson  string  `json:"reference_person"`
}

// APIResultSummary is a simplified simulation result for API responses
type APIResultSummary struct {
	StrategyIdx    int                `json:"strategy_idx"`    // Original index for PDF export
	Strategy       string             `json:"strategy"`
	ShortName      string             `json:"short_name"`
	TotalTaxPaid   float64            `json:"total_tax_paid"`
	TotalWithdrawn float64            `json:"total_withdrawn"`
	TotalIncome    float64            `json:"total_income"`
	RanOutOfMoney  bool               `json:"ran_out_of_money"`
	RanOutYear     int                `json:"ran_out_year,omitempty"`
	FinalBalance   float64            `json:"final_balance"`
	Years          []APIYearSummary   `json:"years,omitempty"`
	// For depletion mode
	MonthlyIncome  float64            `json:"monthly_income,omitempty"`
	FinalISA       float64            `json:"final_isa,omitempty"`
	// Diagnostic fields
	ISADepletedYear     int     `json:"isa_depleted_year,omitempty"`
	PensionDepletedYear int     `json:"pension_depleted_year,omitempty"`
	PensionDrawdownYear int     `json:"pension_drawdown_year,omitempty"`
	TotalMortgagePaid   float64 `json:"total_mortgage_paid,omitempty"`
	MortgagePaidOffYear int     `json:"mortgage_paid_off_year,omitempty"`
	EarlyPayoff         bool    `json:"early_payoff"`
	MortgageOptionName  string  `json:"mortgage_option_name,omitempty"`
	DescriptiveName     string  `json:"descriptive_name,omitempty"`
}

// APIYearSummary provides year-by-year data
type APIYearSummary struct {
	Year              int                         `json:"year"`
	Ages              map[string]int              `json:"ages"`
	RequiredIncome    float64                     `json:"required_income"`
	MortgageCost      float64                     `json:"mortgage_cost"`
	NetIncomeRequired float64                     `json:"net_income_required"`  // Income portion still needed after pensions
	NetMortgageRequired float64                   `json:"net_mortgage_required"` // Mortgage portion still needed
	StatePension      float64                     `json:"state_pension"`
	DBPension         float64                     `json:"db_pension"`
	TaxPaid           float64                     `json:"tax_paid"`
	NetIncome         float64                     `json:"net_income"`
	TotalBalance      float64                     `json:"total_balance"`
	Balances          map[string]APIPersonBalance `json:"balances"`
	// Withdrawal breakdown
	ISAWithdrawal     float64 `json:"isa_withdrawal"`
	PensionWithdrawal float64 `json:"pension_withdrawal"`
	TaxFreeWithdrawal float64 `json:"tax_free_withdrawal"`
	ISADeposit        float64 `json:"isa_deposit"` // Excess income deposited to ISA
	// Tax band info (inflated for year)
	PersonalAllowance float64 `json:"personal_allowance"`
	BasicRateLimit    float64 `json:"basic_rate_limit"`
}

// APIPersonBalance holds person balance info
type APIPersonBalance struct {
	ISA               float64 `json:"isa"`
	UncrystallisedPot float64 `json:"uncrystallised_pot"`
	CrystallisedPot   float64 `json:"crystallised_pot"`
	Total             float64 `json:"total"`
}

// Start starts the web server
func (ws *WebServer) Start() error {
	mux := http.NewServeMux()

	// Static/UI routes
	mux.HandleFunc("/", ws.handleIndex)
	mux.HandleFunc("/api/config", ws.handleGetConfig)
	mux.HandleFunc("/api/simulate", ws.handleSimulate)
	mux.HandleFunc("/api/simulate/fixed", ws.handleSimulateFixed)
	mux.HandleFunc("/api/simulate/depletion", ws.handleSimulateDepletion)
	mux.HandleFunc("/api/simulate/pension-only", ws.handleSimulatePensionOnly)
	mux.HandleFunc("/api/simulate/pension-to-isa", ws.handleSimulatePensionToISA)
	mux.HandleFunc("/api/simulate/sensitivity", ws.handleSensitivityGrid)
	mux.HandleFunc("/api/export-csv", ws.handleExportCSV)
	mux.HandleFunc("/api/export-pdf", ws.handleExportPDF)
	mux.HandleFunc("/api/download-pdf", ws.handleDownloadPDF)
	mux.HandleFunc("/api/open-folder", ws.handleOpenFolder)

	// Listen on the address (use :0 for auto-assign)
	listener, err := net.Listen("tcp", ws.addr)
	if err != nil {
		return err
	}

	// Get the actual address (with assigned port)
	actualAddr := listener.Addr().String()
	url := fmt.Sprintf("http://%s", actualAddr)

	// If listening on all interfaces, use localhost for the URL
	if strings.HasPrefix(actualAddr, ":") || strings.HasPrefix(actualAddr, "0.0.0.0:") {
		port := actualAddr[strings.LastIndex(actualAddr, ":")+1:]
		url = fmt.Sprintf("http://localhost:%s", port)
	}

	log.Printf("Starting web server on %s", actualAddr)
	log.Printf("Opening %s in your browser...", url)

	// Open browser
	go openBrowser(url)

	return http.Serve(listener, mux)
}

// StartForEmbedded starts the server and returns the URL and a cleanup function.
// Unlike Start(), this does NOT open the browser and does NOT block.
// The caller is responsible for stopping the server via the cleanup function.
func (ws *WebServer) StartForEmbedded() (url string, cleanup func(), err error) {
	mux := http.NewServeMux()

	// Static/UI routes
	mux.HandleFunc("/", ws.handleIndex)
	mux.HandleFunc("/api/config", ws.handleGetConfig)
	mux.HandleFunc("/api/simulate", ws.handleSimulate)
	mux.HandleFunc("/api/simulate/fixed", ws.handleSimulateFixed)
	mux.HandleFunc("/api/simulate/depletion", ws.handleSimulateDepletion)
	mux.HandleFunc("/api/simulate/pension-only", ws.handleSimulatePensionOnly)
	mux.HandleFunc("/api/simulate/pension-to-isa", ws.handleSimulatePensionToISA)
	mux.HandleFunc("/api/simulate/sensitivity", ws.handleSensitivityGrid)
	mux.HandleFunc("/api/export-csv", ws.handleExportCSV)
	mux.HandleFunc("/api/export-pdf", ws.handleExportPDF)
	mux.HandleFunc("/api/download-pdf", ws.handleDownloadPDF)
	mux.HandleFunc("/api/open-folder", ws.handleOpenFolder)

	// Listen on the address (use :0 for auto-assign)
	listener, err := net.Listen("tcp", ws.addr)
	if err != nil {
		return "", nil, err
	}

	// Get the actual address (with assigned port)
	actualAddr := listener.Addr().String()
	url = fmt.Sprintf("http://%s", actualAddr)

	// If listening on all interfaces, use localhost for the URL
	if strings.HasPrefix(actualAddr, ":") || strings.HasPrefix(actualAddr, "0.0.0.0:") {
		port := actualAddr[strings.LastIndex(actualAddr, ":")+1:]
		url = fmt.Sprintf("http://localhost:%s", port)
	}

	log.Printf("Starting embedded web server on %s", actualAddr)

	// Create server with proper shutdown support
	server := &http.Server{Handler: mux}

	// Start server in goroutine
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Return cleanup function
	cleanup = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}

	return url, cleanup, nil
}

// handleIndex serves the main web UI
func (ws *WebServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, webUIHTML)
}

// handleGetConfig returns the current configuration
func (ws *WebServer) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if ws.config == nil {
		// Return default config
		defaultConfig, err := LoadDefaultConfig()
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(defaultConfig)
		return
	}

	json.NewEncoder(w).Encode(ws.config)
}

// handleSimulate is the generic simulation endpoint
func (ws *WebServer) handleSimulate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req APISimulationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "Invalid request body: "+err.Error())
		return
	}

	config := ws.buildConfig(&req)

	// Save config to config.yaml for persistence
	if err := SaveConfig(config, "config.yaml"); err != nil {
		// Log but don't fail the simulation
		log.Printf("Warning: failed to save config: %v", err)
	}

	var response APISimulationResponse
	goal := parseOptimizationGoal(req.OptimizationGoal)

	switch req.Mode {
	case "depletion":
		response = ws.runDepletionSimulation(config, goal)
	case "pension-only":
		response = ws.runPensionOnlySimulation(config, goal)
	case "pension-to-isa":
		response = ws.runPensionToISASimulation(config, goal)
	default: // "fixed" or empty
		response = ws.runFixedSimulation(config, goal)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSimulateFixed runs fixed income mode simulation
func (ws *WebServer) handleSimulateFixed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req APISimulationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "Invalid request body: "+err.Error())
		return
	}

	config := ws.buildConfig(&req)

	// Save config to config.yaml for persistence
	if err := SaveConfig(config, "config.yaml"); err != nil {
		log.Printf("Warning: failed to save config: %v", err)
	}

	goal := parseOptimizationGoal(req.OptimizationGoal)
	response := ws.runFixedSimulation(config, goal)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSimulateDepletion runs depletion mode simulation
func (ws *WebServer) handleSimulateDepletion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req APISimulationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "Invalid request body: "+err.Error())
		return
	}

	config := ws.buildConfig(&req)

	// Save config to config.yaml for persistence
	if err := SaveConfig(config, "config.yaml"); err != nil {
		log.Printf("Warning: failed to save config: %v", err)
	}

	goal := parseOptimizationGoal(req.OptimizationGoal)
	response := ws.runDepletionSimulation(config, goal)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSimulatePensionOnly runs pension-only depletion simulation
func (ws *WebServer) handleSimulatePensionOnly(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req APISimulationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "Invalid request body: "+err.Error())
		return
	}

	config := ws.buildConfig(&req)

	// Save config to config.yaml for persistence
	if err := SaveConfig(config, "config.yaml"); err != nil {
		log.Printf("Warning: failed to save config: %v", err)
	}

	goal := parseOptimizationGoal(req.OptimizationGoal)
	response := ws.runPensionOnlySimulation(config, goal)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSimulatePensionToISA runs pension-to-ISA simulation
func (ws *WebServer) handleSimulatePensionToISA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req APISimulationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "Invalid request body: "+err.Error())
		return
	}

	config := ws.buildConfig(&req)

	// Save config to config.yaml for persistence
	if err := SaveConfig(config, "config.yaml"); err != nil {
		log.Printf("Warning: failed to save config: %v", err)
	}

	goal := parseOptimizationGoal(req.OptimizationGoal)
	response := ws.runPensionToISASimulation(config, goal)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// APISensitivityRequest extends the simulation request with sensitivity-specific fields
type APISensitivityRequest struct {
	APISimulationRequest
	PensionGrowthMin float64 `json:"pension_growth_min"`
	PensionGrowthMax float64 `json:"pension_growth_max"`
	SavingsGrowthMin float64 `json:"savings_growth_min"`
	SavingsGrowthMax float64 `json:"savings_growth_max"`
	StepSize         float64 `json:"step_size"`
}

// APISensitivityCell represents a single cell in the sensitivity grid
type APISensitivityCell struct {
	BestStrategy      string  `json:"best_strategy"`
	SustainableIncome float64 `json:"sustainable_income,omitempty"`
	TotalTax          float64 `json:"total_tax,omitempty"`
	RanOut            bool    `json:"ran_out"`
	HasShortfall      bool    `json:"has_shortfall"`
	RanOutYear        int     `json:"ran_out_year,omitempty"`
}

// APISensitivityResponse returns the grid data
type APISensitivityResponse struct {
	Success      bool                 `json:"success"`
	Error        string               `json:"error,omitempty"`
	PensionRates []float64            `json:"pension_rates"`
	SavingsRates []float64            `json:"savings_rates"`
	Grid         [][]APISensitivityCell `json:"grid"`
}

// handleSensitivityGrid runs sensitivity analysis and returns a grid
func (ws *WebServer) handleSensitivityGrid(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req APISensitivityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APISensitivityResponse{Success: false, Error: "Invalid request body: " + err.Error()})
		return
	}

	config := ws.buildConfig(&req.APISimulationRequest)

	// Set sensitivity ranges
	config.Sensitivity.PensionGrowthMin = req.PensionGrowthMin
	config.Sensitivity.PensionGrowthMax = req.PensionGrowthMax
	config.Sensitivity.SavingsGrowthMin = req.SavingsGrowthMin
	config.Sensitivity.SavingsGrowthMax = req.SavingsGrowthMax
	config.Sensitivity.StepSize = req.StepSize

	// Defaults if not set
	if config.Sensitivity.StepSize == 0 {
		config.Sensitivity.StepSize = 0.01
	}
	if config.Sensitivity.PensionGrowthMin == 0 && config.Sensitivity.PensionGrowthMax == 0 {
		config.Sensitivity.PensionGrowthMin = 0.04
		config.Sensitivity.PensionGrowthMax = 0.12
	}
	if config.Sensitivity.SavingsGrowthMin == 0 && config.Sensitivity.SavingsGrowthMax == 0 {
		config.Sensitivity.SavingsGrowthMin = 0.04
		config.Sensitivity.SavingsGrowthMax = 0.12
	}

	// Save config to config.yaml for persistence
	if err := SaveConfig(config, "config.yaml"); err != nil {
		log.Printf("Warning: failed to save config: %v", err)
	}

	goal := parseOptimizationGoal(req.OptimizationGoal)
	response := ws.runSensitivityGrid(config, req.Mode, goal)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// runSensitivityGrid runs sensitivity analysis based on mode
func (ws *WebServer) runSensitivityGrid(config *Config, mode string, goal OptimizationGoal) APISensitivityResponse {
	// Build growth rate arrays
	pensionRates := buildGrowthRates(config.Sensitivity.PensionGrowthMin, config.Sensitivity.PensionGrowthMax, config.Sensitivity.StepSize)
	savingsRates := buildGrowthRates(config.Sensitivity.SavingsGrowthMin, config.Sensitivity.SavingsGrowthMax, config.Sensitivity.StepSize)

	// Initialize grid
	grid := make([][]APISensitivityCell, len(pensionRates))
	for i := range grid {
		grid[i] = make([]APISensitivityCell, len(savingsRates))
	}

	// Check which mode
	isDepletion := mode == "depletion" || mode == "pension-only" || mode == "pension-to-isa"

	if isDepletion {
		// Run depletion sensitivity analysis
		analysis := RunDepletionSensitivityAnalysis(config)

		// Map results to grid (results are in flat array, need to map to 2D)
		resultIdx := 0
		for pi := range pensionRates {
			for si := range savingsRates {
				if resultIdx < len(analysis.Results) {
					res := analysis.Results[resultIdx]
					cell := APISensitivityCell{
						BestStrategy:      res.BestStrategyName,
						SustainableIncome: res.BestIncome,
					}
					if res.BestStrategyIdx >= 0 && res.BestStrategyIdx < len(res.Results) {
						bestResult := res.Results[res.BestStrategyIdx]
						cell.TotalTax = bestResult.SimulationResult.TotalTaxPaid
						cell.RanOut = bestResult.SimulationResult.RanOutOfMoney
						if bestResult.SimulationResult.RanOutOfMoney {
							cell.RanOutYear = bestResult.SimulationResult.RanOutYear
						}
					}
					grid[pi][si] = cell
					resultIdx++
				}
			}
		}
	} else {
		// Run fixed income sensitivity analysis
		analysis := RunSensitivityAnalysis(config, goal)

		// Map results to grid
		for pi, pensionRow := range analysis.Results {
			for si, res := range pensionRow {
				cell := APISensitivityCell{
					BestStrategy: res.BestStrategy,
					TotalTax:     res.TotalTax,
					RanOut:       res.AllRunOut,
					HasShortfall: res.HasShortfall,
				}
				// Set RanOutYear if there's a shortfall or truly ran out
				if (res.AllRunOut || res.HasShortfall) && len(res.AllResults) > 0 && res.BestStrategyIdx >= 0 {
					cell.RanOutYear = res.AllResults[res.BestStrategyIdx].RanOutYear
				}
				grid[pi][si] = cell
			}
		}
	}

	return APISensitivityResponse{
		Success:      true,
		PensionRates: pensionRates,
		SavingsRates: savingsRates,
		Grid:         grid,
	}
}

// buildConfig creates a Config from the API request
func (ws *WebServer) buildConfig(req *APISimulationRequest) *Config {
	config := &Config{
		People:             req.People,
		Financial:          req.Financial,
		IncomeRequirements: req.IncomeRequirements,
		Mortgage:           req.Mortgage,
		Simulation:         req.Simulation,
		TaxBands:           req.TaxBands,
		Tax:                req.Tax,
	}

	// Use defaults for missing values
	if len(config.People) == 0 && ws.config != nil {
		config.People = ws.config.People
	}
	if config.Financial.PensionGrowthRate == 0 && ws.config != nil {
		config.Financial = ws.config.Financial
	}
	if config.Simulation.StartYear == 0 {
		if ws.config != nil {
			config.Simulation = ws.config.Simulation
		} else {
			config.Simulation.StartYear = time.Now().Year() + 1
			config.Simulation.EndAge = 95
		}
	}
	if len(config.TaxBands) == 0 {
		if ws.config != nil && len(ws.config.TaxBands) > 0 {
			config.TaxBands = ws.config.TaxBands
		} else {
			config.TaxBands = getDefaultTaxBands()
		}
	}

	// Use default tax config if not set (all values zero means not configured)
	if config.Tax.PersonalAllowance == 0 && config.Tax.TaperingThreshold == 0 {
		if ws.config != nil && (ws.config.Tax.PersonalAllowance > 0 || ws.config.Tax.TaperingThreshold > 0) {
			config.Tax = ws.config.Tax
		} else {
			config.Tax = DefaultTaxConfig()
		}
	}

	// Debug: log TaxBandInflation value
	log.Printf("DEBUG: TaxBandInflation = %.4f, StartYear = %d", config.Financial.TaxBandInflation, config.Simulation.StartYear)

	// Set reference person if not set OR if it doesn't exist in People
	// This handles the case where Simulation was copied from ws.config but People came from API request
	if len(config.People) > 0 {
		if config.IncomeRequirements.ReferencePerson == "" || config.FindPerson(config.IncomeRequirements.ReferencePerson) == nil {
			config.IncomeRequirements.ReferencePerson = config.People[0].Name
		}
		if config.Simulation.ReferencePerson == "" || config.FindPerson(config.Simulation.ReferencePerson) == nil {
			config.Simulation.ReferencePerson = config.People[0].Name
		}
	}

	return config
}

// CSVExportRequest represents a request to export CSV
type CSVExportRequest struct {
	Content  string `json:"content"`
	Filename string `json:"filename"`
}

// CSVExportResponse represents the response from CSV export
type CSVExportResponse struct {
	Success  bool   `json:"success"`
	FilePath string `json:"file_path"`
	Message  string `json:"message"`
}

// handleExportCSV saves CSV content to a file and returns the path
func (ws *WebServer) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CSVExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CSVExportResponse{
			Success: false,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	// Create exports directory if it doesn't exist
	exportDir := "exports"
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CSVExportResponse{
			Success: false,
			Message: "Failed to create exports directory: " + err.Error(),
		})
		return
	}

	// Generate filename if not provided
	filename := req.Filename
	if filename == "" {
		filename = fmt.Sprintf("pension-forecast-%s.csv", time.Now().Format("2006-01-02-150405"))
	}

	// Full path for the file
	filePath := filepath.Join(exportDir, filename)
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	// Write the file
	if err := os.WriteFile(filePath, []byte(req.Content), 0644); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CSVExportResponse{
			Success: false,
			Message: "Failed to write file: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CSVExportResponse{
		Success:  true,
		FilePath: absPath,
		Message:  fmt.Sprintf("CSV saved to %s", absPath),
	})
}

// OpenFolderRequest represents a request to open a folder
type OpenFolderRequest struct {
	FilePath string `json:"file_path"`
}

// handleOpenFolder opens the folder containing the specified file in the system file browser
func (ws *WebServer) handleOpenFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req OpenFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Get the directory containing the file
	dir := filepath.Dir(req.FilePath)

	// Open in system file browser based on OS
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", dir)
	case "windows":
		cmd = exec.Command("explorer", dir)
	default: // Linux and others
		cmd = exec.Command("xdg-open", dir)
	}

	if err := cmd.Start(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Failed to open folder: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Folder opened",
	})
}

// APIPDFExportRequest extends APISimulationRequest with strategy index for PDF export
type APIPDFExportRequest struct {
	APISimulationRequest
	StrategyIdx int `json:"strategy_idx"` // Index of the strategy to export
}

// PDFExportResponse represents the response from PDF export
type PDFExportResponse struct {
	Success  bool   `json:"success"`
	FilePath string `json:"file_path,omitempty"`
	Message  string `json:"message"`
}

// handleExportPDF generates a detailed PDF action plan for a strategy and saves to file
func (ws *WebServer) handleExportPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(PDFExportResponse{
			Success: false,
			Message: "Method not allowed",
		})
		return
	}

	// Parse the request
	var req APIPDFExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PDFExportResponse{
			Success: false,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	// Build config using the standard method
	config := ws.buildConfig(&req.APISimulationRequest)

	// Get the strategies based on mode
	var strategies []SimulationParams
	if req.Mode == "pension-to-isa" {
		strategies = GetPensionToISAStrategiesForConfig(config)
	} else {
		strategies = GetStrategiesForConfig(config)
	}
	if req.StrategyIdx < 0 || req.StrategyIdx >= len(strategies) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PDFExportResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid strategy index: %d (max: %d)", req.StrategyIdx, len(strategies)-1),
		})
		return
	}

	// Apply config settings
	maximizeCoupleISA := config.Strategy.ShouldMaximizeCoupleISA()
	strategies[req.StrategyIdx].MaximizeCoupleISA = maximizeCoupleISA

	// Run the simulation for this strategy
	// In depletion mode, we need to run the depletion calculation to get the sustainable income
	var result SimulationResult
	var pdfConfig *Config = config

	if req.Mode == "depletion" || req.Mode == "pension-only" || req.Mode == "pension-to-isa" {
		// For depletion modes, run the appropriate calculation
		var depletionResult DepletionResult
		switch req.Mode {
		case "pension-only":
			depletionResult = CalculatePensionOnlyDepletionIncome(strategies[req.StrategyIdx], config)
		case "pension-to-isa":
			depletionResult = CalculateDepletionIncome(strategies[req.StrategyIdx], config)
		default: // "depletion"
			depletionResult = CalculateDepletionIncome(strategies[req.StrategyIdx], config)
		}
		result = depletionResult.SimulationResult

		// Create a modified config with the calculated income for PDF display
		pdfConfig = &Config{}
		*pdfConfig = *config
		pdfConfig.IncomeRequirements.MonthlyBeforeAge = depletionResult.MonthlyBeforeAge
		pdfConfig.IncomeRequirements.MonthlyAfterAge = depletionResult.MonthlyAfterAge
	} else {
		result = RunSimulation(strategies[req.StrategyIdx], config)
	}

	// Generate the PDF
	pdfBytes, err := GenerateStrategyPDFReport(pdfConfig, result)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PDFExportResponse{
			Success: false,
			Message: "Failed to generate PDF: " + err.Error(),
		})
		return
	}

	// Create exports directory if it doesn't exist
	exportDir := "exports"
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PDFExportResponse{
			Success: false,
			Message: "Failed to create exports directory: " + err.Error(),
		})
		return
	}

	// Generate filename with strategy info and timestamp
	safeStrategyName := strings.ReplaceAll(result.Params.ShortName(), "/", "-")
	safeStrategyName = strings.ReplaceAll(safeStrategyName, " ", "_")
	filename := fmt.Sprintf("action-plan-%s-%s.pdf", safeStrategyName, time.Now().Format("2006-01-02-150405"))
	filePath := filepath.Join(exportDir, filename)

	// Write the file
	if err := os.WriteFile(filePath, pdfBytes, 0644); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PDFExportResponse{
			Success: false,
			Message: "Failed to write PDF: " + err.Error(),
		})
		return
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PDFExportResponse{
		Success:  true,
		FilePath: absPath,
		Message:  fmt.Sprintf("PDF action plan saved to %s", absPath),
	})
}

// handleDownloadPDF returns PDF content directly for browser download
func (ws *WebServer) handleDownloadPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request
	var req APIPDFExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Build config using the standard method
	config := ws.buildConfig(&req.APISimulationRequest)

	// Get the strategies based on mode
	var strategies []SimulationParams
	if req.Mode == "pension-to-isa" {
		strategies = GetPensionToISAStrategiesForConfig(config)
	} else {
		strategies = GetStrategiesForConfig(config)
	}
	if req.StrategyIdx < 0 || req.StrategyIdx >= len(strategies) {
		http.Error(w, fmt.Sprintf("Invalid strategy index: %d", req.StrategyIdx), http.StatusBadRequest)
		return
	}

	// Apply config settings
	maximizeCoupleISA := config.Strategy.ShouldMaximizeCoupleISA()
	strategies[req.StrategyIdx].MaximizeCoupleISA = maximizeCoupleISA

	// Run the simulation for this strategy
	// In depletion mode, we need to run the depletion calculation to get the sustainable income
	var result SimulationResult
	var pdfConfig *Config = config

	if req.Mode == "depletion" || req.Mode == "pension-only" || req.Mode == "pension-to-isa" {
		// For depletion modes, run the appropriate calculation
		var depletionResult DepletionResult
		switch req.Mode {
		case "pension-only":
			depletionResult = CalculatePensionOnlyDepletionIncome(strategies[req.StrategyIdx], config)
		case "pension-to-isa":
			depletionResult = CalculateDepletionIncome(strategies[req.StrategyIdx], config)
		default: // "depletion"
			depletionResult = CalculateDepletionIncome(strategies[req.StrategyIdx], config)
		}
		result = depletionResult.SimulationResult

		// Create a modified config with the calculated income for PDF display
		pdfConfig = &Config{}
		*pdfConfig = *config
		pdfConfig.IncomeRequirements.MonthlyBeforeAge = depletionResult.MonthlyBeforeAge
		pdfConfig.IncomeRequirements.MonthlyAfterAge = depletionResult.MonthlyAfterAge
	} else {
		result = RunSimulation(strategies[req.StrategyIdx], config)
	}

	// Generate the PDF
	pdfBytes, err := GenerateStrategyPDFReport(pdfConfig, result)
	if err != nil {
		http.Error(w, "Failed to generate PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set headers for PDF download
	safeStrategyName := strings.ReplaceAll(result.Params.ShortName(), "/", "-")
	filename := fmt.Sprintf("action-plan-%s.pdf", safeStrategyName)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	w.Write(pdfBytes)
}

// runFixedSimulation runs fixed income mode
// parseOptimizationGoal converts string to OptimizationGoal
func parseOptimizationGoal(s string) OptimizationGoal {
	switch s {
	case "income":
		return OptimizeIncome
	case "balance":
		return OptimizeBalance
	default:
		return OptimizeTax // Default to tax efficiency
	}
}

func (ws *WebServer) runFixedSimulation(config *Config, goal OptimizationGoal) APISimulationResponse {
	// Get strategies based on whether there's a mortgage
	strategies := GetStrategiesForConfig(config)

	// Apply config settings to strategies
	maximizeCoupleISA := config.Strategy.ShouldMaximizeCoupleISA()
	for i := range strategies {
		strategies[i].MaximizeCoupleISA = maximizeCoupleISA
	}

	var results []APIResultSummary
	var simResults []SimulationResult // Store raw results for second pass
	var bestResult *APIResultSummary
	bestScore := -1.0
	bestSecondaryScore := -1.0

	// Helper to calculate primary and secondary scores based on goal
	// Returns: primaryScore, secondaryScore, higherIsBetterPrimary, higherIsBetterSecondary
	calcScores := func(totalTax, totalWithdrawn, totalIncome, finalBalance float64) (primary, secondary float64, higherPrimary, higherSecondary bool) {
		taxEfficiency := 1.0
		if totalWithdrawn > 0 {
			taxEfficiency = totalTax / totalWithdrawn
		}
		switch goal {
		case OptimizeIncome:
			// Primary: income (higher better), Secondary: tax efficiency (lower better)
			return totalIncome, taxEfficiency, true, false
		case OptimizeBalance:
			// Primary: balance (higher better), Secondary: income (higher better)
			return finalBalance, totalIncome, true, true
		default: // OptimizeTax
			// Primary: total tax (lower better), Secondary: income (higher better)
			return totalTax, totalIncome, false, true
		}
	}

	// Check if two scores are "similar" (within 1%)
	isSimilar := func(a, b float64) bool {
		if a == 0 && b == 0 {
			return true
		}
		if a == 0 || b == 0 {
			return false
		}
		diff := (a - b) / a
		if diff < 0 {
			diff = -diff
		}
		return diff < 0.01 // Within 1%
	}

	// First pass: Find best among strategies
	// For income optimization: consider all strategies (max income may come from one that runs out)
	// For tax/balance optimization: only consider strategies that don't run out
	for i, params := range strategies {
		result := RunSimulation(params, config)
		summary := convertToAPISummary(result, true, config.Financial.IncomeInflationRate)
		summary.StrategyIdx = i // Track original index for PDF export
		results = append(results, summary)
		simResults = append(simResults, result)

		// For income optimization, consider all strategies
		// For tax/balance, only consider strategies that don't run out in first pass
		shouldConsider := goal == OptimizeIncome || !result.RanOutOfMoney
		if shouldConsider {
			score, secondary, higherIsBetter, higherSecondary := calcScores(result.TotalTaxPaid, result.TotalWithdrawn, summary.TotalIncome, summary.FinalBalance)

			// Determine if this is better
			isBetter := false
			if bestScore < 0 {
				isBetter = true
			} else if higherIsBetter && score > bestScore {
				isBetter = true
			} else if !higherIsBetter && score < bestScore {
				isBetter = true
			} else if isSimilar(score, bestScore) {
				// Primary scores are similar, use secondary as tiebreaker
				if higherSecondary && secondary > bestSecondaryScore {
					isBetter = true
				} else if !higherSecondary && secondary < bestSecondaryScore {
					isBetter = true
				}
			}

			if isBetter {
				bestScore = score
				bestSecondaryScore = secondary
				bestCopy := summary
				bestResult = &bestCopy
			}
		}
	}

	// Second pass: If all ran out, prefer strategies with positive final balance
	if bestResult == nil {
		bestScore = -1.0
		bestSecondaryScore = -1.0
		for i, r := range results {
			// Only consider strategies with positive final balance
			if r.FinalBalance > 1000 {
				score, secondary, higherIsBetter, higherSecondary := calcScores(simResults[i].TotalTaxPaid, simResults[i].TotalWithdrawn, r.TotalIncome, r.FinalBalance)

				// Determine if this is better
				isBetter := false
				if bestScore < 0 {
					isBetter = true
				} else if higherIsBetter && score > bestScore {
					isBetter = true
				} else if !higherIsBetter && score < bestScore {
					isBetter = true
				} else if isSimilar(score, bestScore) {
					// Primary scores are similar, use secondary as tiebreaker
					if higherSecondary && secondary > bestSecondaryScore {
						isBetter = true
					} else if !higherSecondary && secondary < bestSecondaryScore {
						isBetter = true
					}
				}

				if isBetter {
					bestScore = score
					bestSecondaryScore = secondary
					bestCopy := results[i]
					bestResult = &bestCopy
				}
			}
		}
	}

	// Third pass: If still no best (all have zero balance), pick longest-lasting
	if bestResult == nil {
		longestYear := 0
		for i, r := range results {
			if r.RanOutYear > longestYear {
				longestYear = r.RanOutYear
				bestCopy := results[i]
				bestResult = &bestCopy
			}
		}
	}

	return APISimulationResponse{
		Success:       true,
		Results:       results,
		Best:          bestResult,
		GrowthDecline: buildGrowthDeclineInfo(config),
	}
}

// runDepletionSimulation runs depletion mode
func (ws *WebServer) runDepletionSimulation(config *Config, goal OptimizationGoal) APISimulationResponse {
	if config.IncomeRequirements.TargetDepletionAge <= 0 {
		return APISimulationResponse{
			Success: false,
			Error:   "target_depletion_age must be set for depletion mode",
		}
	}

	depletionResults := RunAllDepletionCalculations(config)

	var results []APIResultSummary
	var simResults []SimulationResult
	var bestResult *APIResultSummary
	bestScore := -1.0
	bestSecondaryScore := -1.0

	// Helper to calculate primary and secondary scores based on goal
	calcScores := func(totalTax, totalWithdrawn, totalIncome, finalBalance float64) (primary, secondary float64, higherPrimary, higherSecondary bool) {
		taxEfficiency := 1.0
		if totalWithdrawn > 0 {
			taxEfficiency = totalTax / totalWithdrawn
		}
		switch goal {
		case OptimizeIncome:
			return totalIncome, taxEfficiency, true, false
		case OptimizeBalance:
			return finalBalance, totalIncome, true, true
		default: // OptimizeTax
			return totalTax, totalIncome, false, true
		}
	}

	// Check if two scores are "similar" (within 1%)
	isSimilar := func(a, b float64) bool {
		if a == 0 && b == 0 {
			return true
		}
		if a == 0 || b == 0 {
			return false
		}
		diff := (a - b) / a
		if diff < 0 {
			diff = -diff
		}
		return diff < 0.01
	}

	for i, dr := range depletionResults {
		summary := convertToAPISummary(dr.SimulationResult, true, config.Financial.IncomeInflationRate)
		summary.StrategyIdx = i // Track original index for PDF export
		summary.MonthlyIncome = dr.MonthlyBeforeAge
		results = append(results, summary)
		simResults = append(simResults, dr.SimulationResult)

		// Use MonthlyBeforeAge for income scoring to match sensitivity grid's FindBestDepletionStrategy
		score, secondary, higherIsBetter, higherSecondary := calcScores(
			dr.SimulationResult.TotalTaxPaid,
			dr.SimulationResult.TotalWithdrawn,
			dr.MonthlyBeforeAge, // Use sustainable monthly income, not TotalIncome
			summary.FinalBalance,
		)

		// Determine if this is better
		isBetter := false
		if bestScore < 0 {
			isBetter = true
		} else if higherIsBetter && score > bestScore {
			isBetter = true
		} else if !higherIsBetter && score < bestScore {
			isBetter = true
		} else if isSimilar(score, bestScore) {
			// Primary scores are similar, use secondary as tiebreaker
			if higherSecondary && secondary > bestSecondaryScore {
				isBetter = true
			} else if !higherSecondary && secondary < bestSecondaryScore {
				isBetter = true
			}
		}

		if isBetter {
			bestScore = score
			bestSecondaryScore = secondary
			bestCopy := summary
			bestResult = &bestCopy
		}
	}

	return APISimulationResponse{
		Success:       true,
		Results:       results,
		Best:          bestResult,
		GrowthDecline: buildGrowthDeclineInfo(config),
	}
}

// runPensionOnlySimulation runs pension-only depletion mode
func (ws *WebServer) runPensionOnlySimulation(config *Config, goal OptimizationGoal) APISimulationResponse {
	if config.IncomeRequirements.TargetDepletionAge <= 0 {
		return APISimulationResponse{
			Success: false,
			Error:   "target_depletion_age must be set for pension-only mode",
		}
	}

	depletionResults := RunPensionOnlyDepletionCalculations(config)

	var results []APIResultSummary
	var bestResult *APIResultSummary
	bestScore := -1.0
	bestSecondaryScore := -1.0

	// Helper to calculate primary and secondary scores based on goal
	calcScores := func(totalTax, totalWithdrawn, totalIncome, finalBalance float64) (primary, secondary float64, higherPrimary, higherSecondary bool) {
		taxEfficiency := 1.0
		if totalWithdrawn > 0 {
			taxEfficiency = totalTax / totalWithdrawn
		}
		switch goal {
		case OptimizeIncome:
			return totalIncome, taxEfficiency, true, false
		case OptimizeBalance:
			return finalBalance, totalIncome, true, true
		default: // OptimizeTax
			return totalTax, totalIncome, false, true
		}
	}

	// Check if two scores are "similar" (within 1%)
	isSimilar := func(a, b float64) bool {
		if a == 0 && b == 0 {
			return true
		}
		if a == 0 || b == 0 {
			return false
		}
		diff := (a - b) / a
		if diff < 0 {
			diff = -diff
		}
		return diff < 0.01
	}

	for i, dr := range depletionResults {
		summary := convertToAPISummary(dr.SimulationResult, true, config.Financial.IncomeInflationRate)
		summary.StrategyIdx = i // Track original index for PDF export
		summary.MonthlyIncome = dr.MonthlyBeforeAge
		// Calculate final ISA from simulation result
		summary.FinalISA = calculateFinalISA(dr.SimulationResult)
		results = append(results, summary)

		// Use MonthlyBeforeAge for income scoring to match sensitivity grid's FindBestDepletionStrategy
		score, secondary, higherIsBetter, higherSecondary := calcScores(
			dr.SimulationResult.TotalTaxPaid,
			dr.SimulationResult.TotalWithdrawn,
			dr.MonthlyBeforeAge, // Use sustainable monthly income, not TotalIncome
			summary.FinalBalance,
		)

		// Determine if this is better
		isBetter := false
		if bestScore < 0 {
			isBetter = true
		} else if higherIsBetter && score > bestScore {
			isBetter = true
		} else if !higherIsBetter && score < bestScore {
			isBetter = true
		} else if isSimilar(score, bestScore) {
			// Primary scores are similar, use secondary as tiebreaker
			if higherSecondary && secondary > bestSecondaryScore {
				isBetter = true
			} else if !higherSecondary && secondary < bestSecondaryScore {
				isBetter = true
			}
		}

		if isBetter {
			bestScore = score
			bestSecondaryScore = secondary
			bestCopy := summary
			bestResult = &bestCopy
		}
	}

	return APISimulationResponse{
		Success:       true,
		Results:       results,
		Best:          bestResult,
		GrowthDecline: buildGrowthDeclineInfo(config),
	}
}

// runPensionToISASimulation runs pension-to-ISA mode
func (ws *WebServer) runPensionToISASimulation(config *Config, goal OptimizationGoal) APISimulationResponse {
	if config.IncomeRequirements.TargetDepletionAge <= 0 {
		return APISimulationResponse{
			Success: false,
			Error:   "target_depletion_age must be set for pension-to-ISA mode",
		}
	}

	depletionResults := RunPensionToISADepletionCalculations(config)

	var results []APIResultSummary
	var bestResult *APIResultSummary
	bestScore := -1.0
	bestSecondaryScore := -1.0

	// Helper to calculate primary and secondary scores based on goal
	calcScores := func(totalTax, totalWithdrawn, totalIncome, finalBalance float64) (primary, secondary float64, higherPrimary, higherSecondary bool) {
		taxEfficiency := 1.0
		if totalWithdrawn > 0 {
			taxEfficiency = totalTax / totalWithdrawn
		}
		switch goal {
		case OptimizeIncome:
			return totalIncome, taxEfficiency, true, false
		case OptimizeBalance:
			return finalBalance, totalIncome, true, true
		default: // OptimizeTax
			return totalTax, totalIncome, false, true
		}
	}

	// Check if two scores are "similar" (within 1%)
	isSimilar := func(a, b float64) bool {
		if a == 0 && b == 0 {
			return true
		}
		if a == 0 || b == 0 {
			return false
		}
		diff := (a - b) / a
		if diff < 0 {
			diff = -diff
		}
		return diff < 0.01
	}

	for i, dr := range depletionResults {
		summary := convertToAPISummary(dr.SimulationResult, true, config.Financial.IncomeInflationRate)
		summary.StrategyIdx = i // Track original index for PDF export
		summary.MonthlyIncome = dr.MonthlyBeforeAge
		// Calculate final ISA from simulation result
		summary.FinalISA = calculateFinalISA(dr.SimulationResult)
		results = append(results, summary)

		// Use MonthlyBeforeAge for income scoring to match sensitivity grid's FindBestDepletionStrategy
		score, secondary, higherIsBetter, higherSecondary := calcScores(
			dr.SimulationResult.TotalTaxPaid,
			dr.SimulationResult.TotalWithdrawn,
			dr.MonthlyBeforeAge, // Use sustainable monthly income, not TotalIncome
			summary.FinalBalance,
		)

		// Determine if this is better
		isBetter := false
		if bestScore < 0 {
			isBetter = true
		} else if higherIsBetter && score > bestScore {
			isBetter = true
		} else if !higherIsBetter && score < bestScore {
			isBetter = true
		} else if isSimilar(score, bestScore) {
			// Primary scores are similar, use secondary as tiebreaker
			if higherSecondary && secondary > bestSecondaryScore {
				isBetter = true
			} else if !higherSecondary && secondary < bestSecondaryScore {
				isBetter = true
			}
		}

		if isBetter {
			bestScore = score
			bestSecondaryScore = secondary
			bestCopy := summary
			bestResult = &bestCopy
		}
	}

	return APISimulationResponse{
		Success:       true,
		Results:       results,
		Best:          bestResult,
		GrowthDecline: buildGrowthDeclineInfo(config),
	}
}

// calculateFinalISA calculates the final ISA balance from a simulation result
func calculateFinalISA(result SimulationResult) float64 {
	var total float64
	if len(result.Years) > 0 {
		lastYear := result.Years[len(result.Years)-1]
		for _, balances := range lastYear.EndBalances {
			total += balances.TaxFreeSavings
		}
	}
	return total
}

// buildGrowthDeclineInfo extracts growth decline information from config
func buildGrowthDeclineInfo(config *Config) *GrowthDeclineInfo {
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
		// Calculate end year based on target age and reference person
		refPerson := config.Financial.GrowthDeclineReferencePerson
		if refPerson == "" {
			refPerson = config.Simulation.ReferencePerson
		}

		// Find reference person's birth year
		var birthYear int
		for _, p := range config.People {
			if p.Name == refPerson || (refPerson == "" && birthYear == 0) {
				birthYear = getBirthYear(p.BirthDate)
			}
		}

		endYear := birthYear + config.Financial.GrowthDeclineTargetAge
		if endYear <= config.Simulation.StartYear {
			endYear = config.Simulation.StartYear + 20 // fallback
		}

		return &GrowthDeclineInfo{
			Enabled:          true,
			PensionStartRate: config.Financial.PensionGrowthRate,
			PensionEndRate:   config.Financial.PensionGrowthEndRate,
			SavingsStartRate: config.Financial.SavingsGrowthRate,
			SavingsEndRate:   config.Financial.SavingsGrowthEndRate,
			StartYear:        config.Simulation.StartYear,
			EndYear:          endYear,
			ReferencePerson:  refPerson,
		}
	}

	// Check for depletion-specific growth decline (will be applied in cloneConfigWithMultiplier)
	if config.Financial.DepletionGrowthDeclineEnabled && config.IncomeRequirements.TargetDepletionAge > 0 {
		// Find reference person's birth year
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
			endYear = config.Simulation.StartYear + 20 // fallback
		}

		return &GrowthDeclineInfo{
			Enabled:          true,
			PensionStartRate: config.Financial.PensionGrowthRate,
			PensionEndRate:   config.Financial.PensionGrowthRate - config.Financial.DepletionGrowthDeclinePercent,
			SavingsStartRate: config.Financial.SavingsGrowthRate,
			SavingsEndRate:   config.Financial.SavingsGrowthRate - config.Financial.DepletionGrowthDeclinePercent,
			StartYear:        config.Simulation.StartYear,
			EndYear:          endYear,
			ReferencePerson:  refPerson,
		}
	}

	return nil // No growth decline
}

// convertToAPISummary converts a SimulationResult to API format
func convertToAPISummary(result SimulationResult, includeYears bool, incomeInflationRate float64) APIResultSummary {
	summary := APIResultSummary{
		Strategy:       result.Params.String(),
		ShortName:      result.Params.ShortName(),
		TotalTaxPaid:   result.TotalTaxPaid,
		TotalWithdrawn: result.TotalWithdrawn,
		RanOutOfMoney:  result.RanOutOfMoney,
		RanOutYear:     result.RanOutYear,
		EarlyPayoff:    result.Params.MortgageOpt == MortgageEarly || result.Params.MortgageOpt == PCLSMortgagePayoff,
	}

	// Set mortgage option name
	switch result.Params.MortgageOpt {
	case MortgageEarly:
		summary.MortgageOptionName = "Early Payoff"
	case MortgageNormal:
		summary.MortgageOptionName = "Normal Payoff"
	case MortgageExtended:
		summary.MortgageOptionName = "Extended +10y"
	case PCLSMortgagePayoff:
		summary.MortgageOptionName = "PCLS Lump Sum" // Pension Commencement Lump Sum
	}

	// Calculate final balance and final ISA
	for _, bal := range result.FinalBalances {
		summary.FinalBalance += bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot
		summary.FinalISA += bal.TaxFreeSavings
	}

	// Calculate total income across all years (deflated to today's purchasing power)
	startYear := 0
	if len(result.Years) > 0 {
		startYear = result.Years[0].Year
	}
	for _, year := range result.Years {
		yearsFromStart := year.Year - startYear
		deflator := math.Pow(1+incomeInflationRate, float64(yearsFromStart))
		summary.TotalIncome += year.NetIncomeReceived / deflator // Real value
	}
	if len(result.Years) > 0 {
		summary.MonthlyIncome = summary.TotalIncome / float64(len(result.Years)) / 12.0
	}

	// Track diagnostic info from year data
	prevTotalISA := -1.0
	prevTotalPension := -1.0
	prevPensionWithdrawal := false
	for _, year := range result.Years {
		// Track total ISA and pension across all people
		totalISA := 0.0
		totalPension := 0.0
		for _, bal := range year.EndBalances {
			totalISA += bal.TaxFreeSavings
			totalPension += bal.UncrystallisedPot + bal.CrystallisedPot
		}

		// Detect ISA depletion (last year ISA goes to near zero - may be refilled and depleted again)
		if prevTotalISA > 1000 && totalISA < 1000 {
			summary.ISADepletedYear = year.Year
		}
		prevTotalISA = totalISA

		// Detect pension depletion (first year pension goes to near zero)
		if summary.PensionDepletedYear == 0 && prevTotalPension > 1000 && totalPension < 1000 {
			summary.PensionDepletedYear = year.Year
		}
		prevTotalPension = totalPension

		// Detect first pension drawdown
		hasPensionWithdrawal := false
		for _, amt := range year.Withdrawals.TaxableFromPension {
			if amt > 0 {
				hasPensionWithdrawal = true
				break
			}
		}
		if summary.PensionDrawdownYear == 0 && hasPensionWithdrawal && !prevPensionWithdrawal {
			summary.PensionDrawdownYear = year.Year
		}
		prevPensionWithdrawal = hasPensionWithdrawal

		// Track mortgage payments
		summary.TotalMortgagePaid += year.MortgageCost
		if year.MortgageCost > 0 {
			summary.MortgagePaidOffYear = year.Year
		}
	}

	if includeYears {
		for _, year := range result.Years {
			// Calculate total ISA withdrawal
			var isaWithdrawal float64
			for _, v := range year.Withdrawals.TaxFreeFromISA {
				isaWithdrawal += v
			}
			// Calculate total pension withdrawal (taxable)
			var pensionWithdrawal float64
			for _, v := range year.Withdrawals.TaxableFromPension {
				pensionWithdrawal += v
			}
			// Calculate total tax-free from pension (PCLS)
			var taxFreeWithdrawal float64
			for _, v := range year.Withdrawals.TaxFreeFromPension {
				taxFreeWithdrawal += v
			}

			yearSummary := APIYearSummary{
				Year:                year.Year,
				Ages:                year.Ages,
				RequiredIncome:      year.RequiredIncome,
				MortgageCost:        year.MortgageCost,
				NetIncomeRequired:   year.NetIncomeRequired,
				NetMortgageRequired: year.NetMortgageRequired,
				StatePension:        year.TotalStatePension,
				DBPension:           year.TotalDBPension,
				TaxPaid:             year.TotalTaxPaid,
				NetIncome:           year.NetIncomeReceived,
				TotalBalance:        year.TotalBalance,
				Balances:            make(map[string]APIPersonBalance),
				ISAWithdrawal:       isaWithdrawal,
				PensionWithdrawal:   pensionWithdrawal,
				TaxFreeWithdrawal:   taxFreeWithdrawal,
				ISADeposit:          year.Withdrawals.TotalISADeposits,
				PersonalAllowance:   year.PersonalAllowance,
				BasicRateLimit:      year.BasicRateLimit,
			}
			for name, bal := range year.EndBalances {
				yearSummary.Balances[name] = APIPersonBalance{
					ISA:               bal.TaxFreeSavings,
					UncrystallisedPot: bal.UncrystallisedPot,
					CrystallisedPot:   bal.CrystallisedPot,
					Total:             bal.TaxFreeSavings + bal.CrystallisedPot + bal.UncrystallisedPot,
				}
			}
			summary.Years = append(summary.Years, yearSummary)
		}
	}

	// Set descriptive name using the mortgage payoff year we found
	summary.DescriptiveName = result.Params.DescriptiveName(summary.MortgagePaidOffYear)

	return summary
}

// getDefaultTaxBands returns the default UK tax bands
func getDefaultTaxBands() []TaxBand {
	return []TaxBand{
		{Name: "Personal Allowance", Lower: 0, Upper: 12570, Rate: 0.00},
		{Name: "Basic Rate", Lower: 12570, Upper: 50270, Rate: 0.20},
		{Name: "Higher Rate", Lower: 50270, Upper: 125140, Rate: 0.40},
		{Name: "Additional Rate", Lower: 125140, Upper: 10000000, Rate: 0.45},
	}
}

// sendJSONError sends a JSON error response
func sendJSONError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(APISimulationResponse{
		Success: false,
		Error:   message,
	})
}

// formatMoney formats a number as currency
func formatMoney(amount float64) string {
	if amount >= 1000000 {
		return fmt.Sprintf("£%.1fM", amount/1000000)
	} else if amount >= 1000 {
		return fmt.Sprintf("£%.0fk", amount/1000)
	}
	return fmt.Sprintf("£%.0f", amount)
}

// formatPercent formats a decimal as percentage
func formatPercent(rate float64) string {
	return strconv.FormatFloat(rate*100, 'f', 1, 64) + "%"
}

// webUIHTML is the embedded web interface HTML
const webUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Pension Forecast Simulator</title>
    <link rel="icon" type="image/svg+xml" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 64 64'%3E%3Crect x='4' y='14' width='56' height='28' rx='3' fill='%23C4956A' transform='rotate(-5, 32, 28)'/%3E%3Ctext x='10' y='33' font-family='Georgia' font-size='14' font-weight='bold' fill='%234A3728' transform='rotate(-5, 32, 28)'%3E£10%3C/text%3E%3Cpolygon points='32,18 52,52 12,52' fill='white' stroke='%23CC0000' stroke-width='4' stroke-linejoin='round'/%3E%3Cg fill='%231a1a1a'%3E%3Ccircle cx='26' cy='32' r='4'/%3E%3Cpath d='M26,36 L24,48 M26,36 L28,48 M22,38 L20,50' stroke='%231a1a1a' stroke-width='2.5' stroke-linecap='round'/%3E%3Ccircle cx='38' cy='32' r='4'/%3E%3Cpath d='M38,36 L36,48 M38,36 L40,48' stroke='%231a1a1a' stroke-width='2.5' stroke-linecap='round'/%3E%3C/g%3E%3C/svg%3E">
    <style>
        :root {
            --primary: #2563eb;
            --primary-dark: #1d4ed8;
            --success: #16a34a;
            --warning: #ea580c;
            --danger: #dc2626;
            --bg: #f1f5f9;
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
            background: linear-gradient(135deg, var(--primary) 0%, var(--primary-dark) 100%);
            color: white;
            padding: 1.5rem 2rem;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .header h1 { font-size: 1.5rem; font-weight: 600; }
        .header p { opacity: 0.9; font-size: 0.875rem; }
        .container {
            display: flex;
            height: calc(100vh - 80px);
            overflow: hidden;
        }
        .config-panel {
            width: 400px;
            min-width: 400px;
            background: var(--card-bg);
            border-right: 1px solid var(--border);
            overflow-y: auto;
            padding: 0.5rem;
            transition: margin-left 0.3s ease, opacity 0.3s ease;
        }
        .config-panel.collapsed {
            margin-left: -400px;
            opacity: 0;
        }
        .results-panel {
            flex: 1;
            overflow-y: auto;
            padding: 1rem;
        }
        .config-toggle {
            position: fixed;
            left: 0;
            top: 50%;
            transform: translateY(-50%);
            z-index: 100;
            background: var(--primary);
            color: white;
            border: none;
            border-radius: 0 8px 8px 0;
            padding: 1rem 0.5rem;
            cursor: pointer;
            font-size: 1.2rem;
            box-shadow: 2px 0 8px rgba(0,0,0,0.2);
            transition: left 0.3s ease;
        }
        .config-toggle.panel-open { left: 400px; }
        .config-toggle:hover { background: var(--primary-dark); }
        .grid { display: grid; gap: 1.5rem; }
        .grid-2 { grid-template-columns: 1fr 1fr; }
        .grid-3 { grid-template-columns: repeat(3, 1fr); }
        @media (max-width: 1024px) {
            .grid-2, .grid-3 { grid-template-columns: 1fr; }
            .config-panel { width: 100%; min-width: 100%; }
            .config-panel.collapsed { margin-left: -100%; }
            .config-toggle.panel-open { left: 100%; }
        }
        .card {
            background: var(--card-bg);
            border-radius: 8px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
            padding: 0.75rem;
            margin-bottom: 0.5rem;
        }
        .card h2 {
            font-size: 0.85rem;
            font-weight: 600;
            margin-bottom: 0.5rem;
            color: var(--primary);
            display: flex;
            align-items: center;
            gap: 0.25rem;
        }
        .form-group { margin-bottom: 0.5rem; }
        .form-group label {
            display: block;
            font-size: 0.7rem;
            font-weight: 500;
            color: var(--text-muted);
            margin-bottom: 0.15rem;
            text-transform: uppercase;
            letter-spacing: 0.3px;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }
        .form-group input, .form-group select {
            width: 100%;
            padding: 0.4rem 0.5rem;
            border: 1px solid var(--border);
            border-radius: 4px;
            font-size: 0.8rem;
            transition: border-color 0.2s, box-shadow 0.2s;
        }
        .form-group input:focus, .form-group select:focus {
            outline: none;
            border-color: var(--primary);
            box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.1);
        }
        .form-row { display: grid; grid-template-columns: repeat(2, 1fr); gap: 0.5rem; align-items: start; }
        .form-row-3 { display: grid; grid-template-columns: repeat(3, 1fr); gap: 0.5rem; align-items: start; }
        .form-row-4 { display: grid; grid-template-columns: repeat(4, 1fr); gap: 0.5rem; align-items: start; }
        @media (max-width: 600px) {
            .form-row-3, .form-row-4 { grid-template-columns: repeat(2, 1fr); }
        }
        .form-hint {
            font-size: 0.65rem;
            color: var(--text-muted);
            margin-top: 0.2rem;
            line-height: 1.3;
        }
        .form-section {
            border: 1px solid var(--border);
            border-radius: 6px;
            padding: 0.75rem;
            margin-bottom: 0.75rem;
        }
        .form-section-title {
            font-size: 0.75rem;
            font-weight: 600;
            color: var(--text);
            margin-bottom: 0.5rem;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        .checkbox-label {
            display: flex;
            align-items: center;
            gap: 0.4rem;
            font-size: 0.75rem;
            font-weight: 500;
            color: var(--text);
            text-transform: none;
            letter-spacing: 0;
            white-space: normal;
            cursor: pointer;
        }
        .checkbox-label input[type="checkbox"] {
            width: auto;
            margin: 0;
        }
        /* Tooltip styling for abbreviations */
        abbr[title] {
            text-decoration: underline dotted;
            text-decoration-color: var(--text-muted);
            cursor: help;
        }
        abbr[title]:hover {
            text-decoration-color: var(--primary);
        }
        .btn {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            gap: 0.3rem;
            padding: 0.5rem 1rem;
            font-size: 0.8rem;
            font-weight: 500;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            transition: all 0.2s;
        }
        .btn-primary {
            background: var(--primary);
            color: white;
        }
        .btn-primary:hover { background: var(--primary-dark); }
        .btn-primary:disabled {
            background: var(--text-muted);
            cursor: not-allowed;
        }
        .btn-secondary {
            background: var(--bg-darker);
            color: var(--text);
            border: 1px solid var(--border);
        }
        .btn-secondary:hover { background: var(--border); }
        .btn-group { display: flex; gap: 0.5rem; flex-wrap: wrap; }
        .mode-selector {
            display: grid;
            grid-template-columns: repeat(4, 1fr);
            gap: 0.3rem;
            margin-bottom: 0.5rem;
        }
        @media (max-width: 768px) {
            .mode-selector { grid-template-columns: repeat(2, 1fr); }
        }
        .mode-btn {
            padding: 0.4rem;
            border: 2px solid var(--border);
            border-radius: 6px;
            background: white;
            cursor: pointer;
            text-align: center;
            transition: all 0.2s;
        }
        .mode-btn:hover { border-color: var(--primary); }
        .mode-btn.active {
            border-color: var(--primary);
            background: rgba(37, 99, 235, 0.05);
        }
        .mode-btn .title { font-weight: 600; font-size: 0.75rem; }
        .mode-btn .desc { font-size: 0.6rem; color: var(--text-muted); }
        .person-card {
            border: 1px solid var(--border);
            border-radius: 6px;
            padding: 0.5rem;
            margin-bottom: 0.5rem;
        }
        .person-card h3 {
            font-size: 0.8rem;
            font-weight: 600;
            margin-bottom: 0.4rem;
            padding-bottom: 0.25rem;
            border-bottom: 1px solid var(--border);
        }
        .advanced-options {
            margin-top: 0.5rem;
            border: 1px solid var(--border);
            border-radius: 4px;
            padding: 0.25rem;
            background: #f8fafc;
        }
        .advanced-options summary {
            font-size: 0.7rem;
            font-weight: 600;
            color: var(--text-muted);
            cursor: pointer;
            padding: 0.25rem;
        }
        .advanced-options summary:hover { color: var(--primary); }
        .advanced-options[open] summary { margin-bottom: 0.5rem; border-bottom: 1px solid var(--border); }
        .advanced-options .form-group { margin-bottom: 0.25rem; }
        .advanced-options label { font-size: 0.6rem; }
        .advanced-options input { font-size: 0.75rem; padding: 0.25rem; }
        .results-card { margin-top: 1.5rem; }
        .results-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
            gap: 1rem;
            margin-bottom: 1.5rem;
        }
        .metric {
            text-align: center;
            padding: 1rem;
            background: var(--bg);
            border-radius: 8px;
        }
        .metric-value {
            font-size: 1.5rem;
            font-weight: 700;
            color: var(--primary);
        }
        .metric-label {
            font-size: 0.75rem;
            color: var(--text-muted);
            text-transform: uppercase;
        }
        .metric.success .metric-value { color: var(--success); }
        .metric.warning .metric-value { color: var(--warning); }
        .metric.danger .metric-value { color: var(--danger); }
        .strategy-table {
            width: 100%;
            border-collapse: collapse;
            font-size: 0.875rem;
        }
        .strategy-table th, .strategy-table td {
            padding: 0.75rem;
            text-align: left;
            border-bottom: 1px solid var(--border);
        }
        .strategy-table th {
            background: var(--bg);
            font-weight: 600;
            font-size: 0.75rem;
            text-transform: uppercase;
            color: var(--text-muted);
        }
        .strategy-table tr:hover { background: rgba(37, 99, 235, 0.02); }
        .strategy-table .best { background: rgba(22, 163, 74, 0.1); }
        .badge {
            display: inline-block;
            padding: 0.25rem 0.5rem;
            font-size: 0.7rem;
            font-weight: 600;
            border-radius: 4px;
        }
        .badge-success { background: rgba(22, 163, 74, 0.15); color: var(--success); }
        .badge-warning { background: rgba(245, 158, 11, 0.15); color: #f59e0b; }
        .badge-danger { background: rgba(220, 38, 38, 0.15); color: var(--danger); }
        .loading {
            display: none;
            text-align: center;
            padding: 2rem;
            color: var(--text-muted);
        }
        .loading.show { display: block; }
        .spinner {
            width: 40px;
            height: 40px;
            border: 3px solid var(--border);
            border-top-color: var(--primary);
            border-radius: 50%;
            animation: spin 1s linear infinite;
            margin: 0 auto 1rem;
        }
        @keyframes spin { to { transform: rotate(360deg); } }
        .hidden { display: none !important; }
        .add-person-btn {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            padding: 0.5rem 1rem;
            border: 2px dashed var(--border);
            border-radius: 8px;
            background: transparent;
            color: var(--text-muted);
            cursor: pointer;
            font-size: 0.875rem;
            width: 100%;
            justify-content: center;
            transition: all 0.2s;
        }
        .add-person-btn:hover {
            border-color: var(--primary);
            color: var(--primary);
        }
        .section-title {
            font-size: 1.125rem;
            font-weight: 600;
            margin-bottom: 1rem;
            padding-bottom: 0.5rem;
            border-bottom: 2px solid var(--primary);
        }
        .collapsible {
            cursor: pointer;
            user-select: none;
        }
        .collapsible::after {
            content: ' ▼';
            font-size: 0.7rem;
        }
        .collapsible.collapsed::after {
            content: ' ▶';
        }
        .collapse-content {
            max-height: 2000px;
            overflow: hidden;
            transition: max-height 0.3s ease;
        }
        .collapse-content.collapsed {
            max-height: 0;
        }
        .strategy-table tr { cursor: pointer; }
        .strategy-table tr:hover { background: rgba(37, 99, 235, 0.08); }
        .detail-view {
            margin-top: 1.5rem;
            padding: 1rem;
            background: var(--bg);
            border-radius: 8px;
            max-height: 600px;
            overflow-y: auto;
        }
        .detail-view h4 {
            margin-bottom: 0.75rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .detail-view .close-btn {
            background: none;
            border: none;
            font-size: 1.25rem;
            cursor: pointer;
            color: var(--text-muted);
        }
        .detail-table {
            width: 100%;
            font-size: 0.75rem;
            border-collapse: collapse;
        }
        .detail-table th, .detail-table td {
            padding: 0.375rem 0.5rem;
            text-align: right;
            border-bottom: 1px solid var(--border);
        }
        .detail-table th {
            background: var(--card-bg);
            font-weight: 600;
            position: sticky;
            top: 0;
        }
        .detail-table td:first-child, .detail-table th:first-child { text-align: left; }
        .detail-table tr:hover { background: rgba(37, 99, 235, 0.05); }
        .detail-table tr.highlight-mortgage { background: #fff3cd; }
        .detail-table tr.highlight-mortgage:hover { background: #ffe69c; }
        .detail-table tr.highlight-retire { background: #d1e7dd; }
        .detail-table tr.highlight-retire:hover { background: #badbcc; }
        .detail-table tr.highlight-spa { background: #cfe2ff; }
        .detail-table tr.highlight-spa:hover { background: #b6d4fe; }
        .detail-table tr.highlight-db { background: #e2d9f3; }
        .detail-table tr.highlight-db:hover { background: #d3c5eb; }
        .detail-table tr.highlight-pension-depleted { background: #f8d7da; }
        .detail-table tr.highlight-pension-depleted:hover { background: #f1aeb5; }
        .detail-table tr.highlight-isa-depleted { background: #ffe5d0; }
        .detail-table tr.highlight-isa-depleted:hover { background: #ffd1b3; }

        /* Expandable year rows */
        .detail-table tr.expandable-row { cursor: pointer; }
        .detail-table tr.expandable-row td:first-child::before {
            content: '▶';
            display: inline-block;
            margin-right: 0.5rem;
            font-size: 0.7rem;
            transition: transform 0.2s;
        }
        .detail-table tr.expandable-row.expanded td:first-child::before {
            transform: rotate(90deg);
        }
        .year-details-row {
            display: none;
            background: #f8fafc;
        }
        .year-details-row.show { display: table-row; }
        .year-details-row td {
            padding: 1rem;
            border-bottom: 2px solid var(--primary);
        }
        .year-detail-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
        }
        .year-detail-section h5 {
            font-size: 0.8rem;
            color: var(--primary);
            margin-bottom: 0.5rem;
            border-bottom: 1px solid var(--border);
            padding-bottom: 0.25rem;
        }
        .year-detail-item {
            display: flex;
            justify-content: space-between;
            font-size: 0.75rem;
            padding: 0.2rem 0;
        }
        .year-detail-item span:last-child { font-weight: 500; }

        /* Summary bar at top of results */
        .summary-bar {
            background: var(--card-bg);
            border-radius: 8px;
            padding: 1rem;
            margin-bottom: 1rem;
            display: flex;
            flex-wrap: wrap;
            gap: 1.5rem;
            align-items: center;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }
        .summary-item {
            display: flex;
            flex-direction: column;
            gap: 0.25rem;
        }
        .summary-item .label { font-size: 0.7rem; color: var(--text-muted); text-transform: uppercase; }
        .summary-item .value { font-size: 1rem; font-weight: 600; color: var(--primary); }
        .summary-divider { width: 1px; height: 2.5rem; background: var(--border); }

        /* Accordion strategy rows */
        .strategy-accordion { border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
        .strategy-accordion-item { border-bottom: 1px solid var(--border); }
        .strategy-accordion-item:last-child { border-bottom: none; }
        .strategy-accordion-header {
            display: flex;
            align-items: center;
            padding: 0.75rem 1rem;
            cursor: pointer;
            background: var(--card-bg);
            transition: background 0.2s;
        }
        .strategy-accordion-header:hover { background: rgba(37, 99, 235, 0.05); }
        .strategy-accordion-header.best { background: rgba(34, 197, 94, 0.1); }
        .strategy-accordion-header .expand-icon {
            margin-right: 0.75rem;
            transition: transform 0.2s;
            font-size: 0.8rem;
            color: var(--text-muted);
        }
        .strategy-accordion-header.expanded .expand-icon { transform: rotate(90deg); }
        .strategy-accordion-header .strategy-name { flex: 1; font-weight: 500; }
        .strategy-accordion-header .strategy-stats {
            display: flex;
            gap: 1.5rem;
            font-size: 0.85rem;
        }
        .strategy-accordion-header .stat { text-align: right; }
        .strategy-accordion-header .stat-label { font-size: 0.65rem; color: var(--text-muted); }
        .strategy-accordion-content {
            max-height: 0;
            overflow: hidden;
            transition: max-height 0.3s ease;
            background: var(--bg);
        }
        .strategy-accordion-content.expanded { max-height: 500px; overflow-y: auto; }

        /* Tax popup modal */
        .tax-popup-overlay {
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0,0,0,0.5);
            display: flex;
            align-items: center;
            justify-content: center;
            z-index: 1000;
            opacity: 0;
            visibility: hidden;
            transition: opacity 0.2s, visibility 0.2s;
        }
        .tax-popup-overlay.visible { opacity: 1; visibility: visible; }
        .tax-popup {
            background: var(--card-bg);
            border-radius: 12px;
            padding: 1.5rem;
            max-width: 500px;
            width: 90%;
            max-height: 80vh;
            overflow-y: auto;
            box-shadow: 0 20px 40px rgba(0,0,0,0.3);
        }
        .tax-popup h3 { margin-bottom: 1rem; display: flex; justify-content: space-between; align-items: center; }
        .tax-popup .close-btn { background: none; border: none; font-size: 1.5rem; cursor: pointer; color: var(--text-muted); }
        .tax-breakdown { font-size: 0.85rem; }
        .tax-breakdown-row { display: flex; justify-content: space-between; padding: 0.5rem 0; border-bottom: 1px solid var(--border); }
        .tax-breakdown-row:last-child { border-bottom: none; }
        .tax-breakdown-row.total { font-weight: 600; background: rgba(37, 99, 235, 0.05); margin: 0.5rem -0.5rem; padding: 0.75rem 0.5rem; border-radius: 4px; }
        .tax-link { color: var(--primary); cursor: pointer; text-decoration: underline; }
        .tax-link:hover { color: var(--primary-dark); }

        /* Comparison modal */
        .compare-modal-overlay {
            position: fixed;
            top: 0; left: 0; right: 0; bottom: 0;
            background: rgba(0,0,0,0.7);
            z-index: 1001;
            display: flex;
            align-items: center;
            justify-content: center;
            opacity: 0;
            visibility: hidden;
            transition: opacity 0.2s, visibility 0.2s;
        }
        .compare-modal-overlay.visible { opacity: 1; visibility: visible; }
        .compare-modal {
            background: var(--card-bg);
            border-radius: 12px;
            width: 95vw;
            max-width: 1400px;
            max-height: 90vh;
            overflow: hidden;
            display: flex;
            flex-direction: column;
        }
        .compare-modal-header {
            padding: 1rem;
            border-bottom: 1px solid var(--border);
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .compare-modal-header h3 { margin: 0; }
        .compare-modal-body {
            flex: 1;
            overflow-y: auto;
            padding: 1rem;
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 1rem;
        }
        .compare-column { border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
        .compare-column-header {
            padding: 0.75rem;
            background: var(--bg-darker);
            font-weight: 600;
            text-align: center;
        }
        .compare-column-header.early { background: #d4edda; color: #155724; }
        .compare-column-header.normal { background: #cce5ff; color: #004085; }
        .compare-stats {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 0.5rem;
            padding: 0.75rem;
            background: var(--bg);
        }
        .compare-stat { text-align: center; }
        .compare-stat .label { font-size: 0.7rem; color: var(--text-muted); }
        .compare-stat .value { font-size: 1rem; font-weight: 600; }
        .compare-stat.better .value { color: var(--success); }
        .compare-stat.worse .value { color: var(--danger); }
        .compare-years { max-height: 400px; overflow-y: auto; }
        .compare-btn { margin-left: auto; background: var(--primary); color: white; border: none; padding: 0.5rem 1rem; border-radius: 6px; cursor: pointer; font-size: 0.8rem; }
        .compare-btn:hover { background: var(--primary-dark); }

        .mortgage-part {
            border: 1px solid var(--border);
            border-radius: 6px;
            padding: 0.5rem;
            margin-bottom: 0.5rem;
        }
        .mortgage-part-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 0.4rem;
            font-weight: 600;
            font-size: 0.8rem;
        }
        .remove-mortgage-btn {
            background: var(--danger);
            color: white;
            border: none;
            border-radius: 4px;
            padding: 0.25rem 0.5rem;
            font-size: 0.7rem;
            cursor: pointer;
        }
        .remove-mortgage-btn:hover { opacity: 0.8; }

        /* Sensitivity Grid Styles */
        .sensitivity-toggle { margin-top: 1rem; display: flex; align-items: center; gap: 0.5rem; }
        .sensitivity-toggle input[type="checkbox"] { width: 1.2rem; height: 1.2rem; cursor: pointer; }
        .sensitivity-toggle label { cursor: pointer; font-size: 0.9rem; }

        .sensitivity-grid-container { margin-top: 1rem; }
        .sensitivity-grid { display: grid; gap: 2px; font-size: 0.85rem; }
        .sensitivity-header { background: var(--bg-darker); padding: 0.5rem; text-align: center; font-weight: 600; }
        .sensitivity-cell {
            padding: 0.5rem 0.3rem;
            text-align: center;
            cursor: pointer;
            transition: transform 0.1s, box-shadow 0.1s;
            border-radius: 2px;
        }
        .sensitivity-cell:hover { transform: scale(1.1); box-shadow: 0 2px 8px rgba(0,0,0,0.3); z-index: 10; position: relative; }
        .sensitivity-cell .income { font-weight: 600; font-size: 0.95rem; }
        .sensitivity-cell .strategy { font-size: 0.75rem; opacity: 0.9; }

        /* Strategy color classes */
        .cell-isa-first { background: #e3f2fd; color: #1565c0; }
        .cell-pen-first { background: #e8f5e9; color: #2e7d32; }
        .cell-tax-opt { background: #fff3e0; color: #e65100; }
        .cell-pen-isa { background: #f3e5f5; color: #7b1fa2; }
        .cell-shortfall { background: #fff8e1; color: #f57c00; }
        .cell-ran-out { background: #ffcdd2; color: #c62828; }
        .cell-unknown { background: var(--bg-darker); color: var(--text-muted); }

        .sensitivity-legend { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 0.5rem; font-size: 0.75rem; }
        .legend-item { display: flex; align-items: center; gap: 0.25rem; }
        .legend-color { width: 1rem; height: 1rem; border-radius: 2px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Pension Forecast Simulator <button onclick="showHelp()" style="background:none;border:none;color:white;font-size:1rem;cursor:pointer;opacity:0.8;" title="Help">?</button></h1>
        <p>Tax-optimised retirement drawdown planning with advanced strategies</p>
    </div>

    <!-- Config Panel Toggle Button -->
    <button class="config-toggle panel-open" id="config-toggle" onclick="toggleConfigPanel()">&#9776;</button>

    <!-- Help Modal -->
    <div class="tax-popup-overlay" id="help-modal" onclick="closeHelp(event)" style="display:none;">
        <div class="tax-popup" onclick="event.stopPropagation()" style="max-width:700px;max-height:80vh;overflow-y:auto;">
            <h3><span>New Features Guide</span><button class="close-btn" onclick="closeHelp()">&times;</button></h3>
            <div style="text-align:left;font-size:0.85rem;line-height:1.6;">
                <h4 style="color:var(--primary);margin:1rem 0 0.5rem;">Income Strategies</h4>
                <p><strong>Guardrails (Guyton-Klinger):</strong> Automatically adjusts withdrawals when portfolio drifts too far from your initial withdrawal rate. Upper limit triggers a reduction, lower limit triggers an increase.</p>
                <p><strong>VPW (Variable Percentage Withdrawal):</strong> Withdrawal rate increases with age (3% at 55 to 35% at 100), naturally adjusting to life expectancy. Optional floor/ceiling limits.</p>
                <p><strong>Emergency Fund Protection:</strong> Preserves a minimum ISA balance (X months of expenses) before drawing from other sources.</p>
                <h4 style="color:var(--primary);margin:1rem 0 0.5rem;">DB Pension Options</h4>
                <p><strong>Early/Late Retirement Factors:</strong> Takes your DB pension early? Reduced by the early factor per year. Taking it late? Enhanced by the late factor per year.</p>
                <p><strong>Commutation:</strong> Trade up to 25% of your DB pension for a tax-free lump sum. The commutation factor determines how much lump sum you get per pound of pension given up.</p>
                <h4 style="color:var(--primary);margin:1rem 0 0.5rem;">State Pension Deferral</h4>
                <p>Defer your state pension for up to 10 years. Each year deferred increases your pension by 5.8% (compounded).</p>
                <h4 style="color:var(--primary);margin:1rem 0 0.5rem;">Phased Retirement</h4>
                <p>Add part-time work income between specified ages. This income is taxable and helps bridge the gap before pensions start.</p>
                <h4 style="color:var(--primary);margin:1rem 0 0.5rem;">Tips</h4>
                <ul style="margin-left:1.5rem;">
                    <li>Expand "Advanced Options" under each person for DB and phased retirement settings</li>
                    <li>Income Strategies section has Guardrails, VPW, and emergency fund settings</li>
                    <li>Hover over input fields for tooltips</li>
                </ul>
            </div>
        </div>
    </div>

    <!-- Tax Popup Overlay -->
    <div class="tax-popup-overlay" id="tax-popup-overlay" onclick="closeTaxPopup(event)">
        <div class="tax-popup" onclick="event.stopPropagation()">
            <h3><span id="tax-popup-title">Tax Breakdown</span><button class="close-btn" onclick="closeTaxPopup()">&times;</button></h3>
            <div id="tax-popup-content" class="tax-breakdown"></div>
        </div>
    </div>

    <!-- Comparison Modal -->
    <div class="compare-modal-overlay" id="compare-modal-overlay" onclick="closeCompareModal(event)">
        <div class="compare-modal" onclick="event.stopPropagation()">
            <div class="compare-modal-header">
                <h3>Early vs Normal vs Extended Mortgage Payoff Comparison</h3>
                <button class="close-btn" onclick="closeCompareModal()">&times;</button>
            </div>
            <div class="compare-modal-body" id="compare-modal-body"></div>
        </div>
    </div>

    <div class="container">
        <!-- Left Column - Configuration Panel -->
        <div class="config-panel" id="config-panel">
            <!-- Mode Selection -->
            <div class="card">
                <h2>Simulation Mode</h2>
                <div class="mode-selector">
                    <div class="mode-btn active" data-mode="fixed">
                        <div class="title">Fixed Income</div>
                        <div class="desc">Specify monthly income needs</div>
                    </div>
                    <div class="mode-btn" data-mode="depletion">
                        <div class="title">Depletion</div>
                        <div class="desc">Calculate sustainable income</div>
                    </div>
                    <div class="mode-btn" data-mode="pension-only">
                        <div class="title">Pension Only</div>
                        <div class="desc">Preserve <abbr title="Individual Savings Account">ISA</abbr>s</div>
                    </div>
                    <div class="mode-btn" data-mode="pension-to-isa">
                        <div class="title">Pension to <abbr title="Individual Savings Account">ISA</abbr></div>
                        <div class="desc">Transfer excess to <abbr title="Individual Savings Account">ISA</abbr>s</div>
                    </div>
                </div>
                <div class="form-group" style="margin-top:1rem;">
                    <label>Optimization Goal</label>
                    <select id="optimization-goal" style="width:100%;padding:0.5rem;border-radius:4px;border:1px solid var(--border);">
                        <option value="tax">Tax Efficiency (minimize tax paid)</option>
                        <option value="income" selected>Total Income (maximize withdrawals)</option>
                        <option value="balance">Final Balance (maximize end wealth)</option>
                    </select>
                </div>
            </div>
                <!-- People -->
                <div class="card">
                    <h2 class="collapsible">People</h2>
                    <div class="collapse-content" id="people-container">
                        <div class="person-card" id="person1">
                            <h3>Person 1</h3>
                            <div class="form-row">
                                <div class="form-group">
                                    <label>Name</label>
                                    <input type="text" id="p1-name" value="Person1">
                                </div>
                                <div class="form-group">
                                    <label>Birth Date</label>
                                    <input type="text" id="p1-birth" value="1970-12-15" placeholder="YYYY-MM-DD" pattern="\d{4}-\d{2}-\d{2}">
                                </div>
                            </div>
                            <div class="form-row-4">
                                <div class="form-group">
                                    <label>Retire Age</label>
                                    <input type="number" id="p1-retire" value="55">
                                </div>
                                <div class="form-group">
                                    <label>SP Age</label>
                                    <input type="number" id="p1-spa" value="67">
                                    <div class="form-hint">State pension</div>
                                </div>
                                <div class="form-group">
                                    <label>Pension</label>
                                    <input type="text" id="p1-pension" value="500000">
                                    <div class="form-hint">DC pot</div>
                                </div>
                                <div class="form-group">
                                    <label>ISA</label>
                                    <input type="text" id="p1-isa" value="100000">
                                    <div class="form-hint">Tax-free</div>
                                </div>
                            </div>
                            <div class="form-row-4">
                                <div class="form-group">
                                    <label>DB Name</label>
                                    <input type="text" id="p1-db-name" value="">
                                    <div class="form-hint">e.g. Teachers</div>
                                </div>
                                <div class="form-group">
                                    <label>DB Amount</label>
                                    <input type="text" id="p1-db-amount" value="0">
                                    <div class="form-hint">Annual</div>
                                </div>
                                <div class="form-group">
                                    <label>DB Age</label>
                                    <input type="number" id="p1-db-age" value="67">
                                    <div class="form-hint">Start age</div>
                                </div>
                                <div class="form-group">
                                    <label>ISA Limit</label>
                                    <input type="text" id="p1-isa-limit" value="20000">
                                    <div class="form-hint">Annual</div>
                                </div>
                            </div>
                            <details class="advanced-options">
                                <summary>Advanced Options</summary>
                                <div class="form-row-4">
                                    <div class="form-group">
                                        <label>SP Defer</label>
                                        <input type="number" id="p1-sp-defer" value="0" min="0" max="10">
                                        <div class="form-hint">Years (5.8%/yr)</div>
                                    </div>
                                    <div class="form-group">
                                        <label>DB Normal</label>
                                        <input type="number" id="p1-db-normal-age" value="65">
                                        <div class="form-hint">NRA age</div>
                                    </div>
                                    <div class="form-group">
                                        <label>Early %</label>
                                        <input type="number" id="p1-db-early-factor" value="4" step="0.5">
                                        <div class="form-hint">Reduction/yr</div>
                                    </div>
                                    <div class="form-group">
                                        <label>Late %</label>
                                        <input type="number" id="p1-db-late-factor" value="5" step="0.5">
                                        <div class="form-hint">Increase/yr</div>
                                    </div>
                                </div>
                                <div class="form-row">
                                    <div class="form-group">
                                        <label>Commute %</label>
                                        <input type="number" id="p1-db-commute" value="0" min="0" max="25">
                                        <div class="form-hint">DB lump sum</div>
                                    </div>
                                    <div class="form-group">
                                        <label>Factor</label>
                                        <input type="number" id="p1-db-commute-factor" value="12">
                                        <div class="form-hint">Commute factor</div>
                                    </div>
                                </div>
                                <div class="form-section-title" style="margin-top: 0.5rem;">Phased Retirement</div>
                                <div class="form-row-3">
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>Income</label>
                                        <input type="text" id="p1-parttime-income" value="0">
                                        <div class="form-hint">Annual</div>
                                    </div>
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>Start</label>
                                        <input type="number" id="p1-parttime-start" value="55">
                                        <div class="form-hint">Age</div>
                                    </div>
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>End</label>
                                        <input type="number" id="p1-parttime-end" value="60">
                                        <div class="form-hint">Age</div>
                                    </div>
                                </div>
                            </details>
                        </div>
                        <div class="person-card" id="person2">
                            <h3>Person 2</h3>
                            <div class="form-row">
                                <div class="form-group">
                                    <label>Name</label>
                                    <input type="text" id="p2-name" value="Person2">
                                </div>
                                <div class="form-group">
                                    <label>Birth Date</label>
                                    <input type="text" id="p2-birth" value="1975-01-13" placeholder="YYYY-MM-DD" pattern="\d{4}-\d{2}-\d{2}">
                                </div>
                            </div>
                            <div class="form-row-4">
                                <div class="form-group">
                                    <label>Retire Age</label>
                                    <input type="number" id="p2-retire" value="57">
                                </div>
                                <div class="form-group">
                                    <label>SP Age</label>
                                    <input type="number" id="p2-spa" value="67">
                                    <div class="form-hint">State pension</div>
                                </div>
                                <div class="form-group">
                                    <label>Pension</label>
                                    <input type="text" id="p2-pension" value="500000">
                                    <div class="form-hint">DC pot</div>
                                </div>
                                <div class="form-group">
                                    <label>ISA</label>
                                    <input type="text" id="p2-isa" value="100000">
                                    <div class="form-hint">Tax-free</div>
                                </div>
                            </div>
                            <div class="form-row-4">
                                <div class="form-group">
                                    <label>DB Name</label>
                                    <input type="text" id="p2-db-name" value="Government Pension">
                                    <div class="form-hint">e.g. Teachers</div>
                                </div>
                                <div class="form-group">
                                    <label>DB Amount</label>
                                    <input type="text" id="p2-db-amount" value="400">
                                    <div class="form-hint">Annual</div>
                                </div>
                                <div class="form-group">
                                    <label>DB Age</label>
                                    <input type="number" id="p2-db-age" value="57">
                                    <div class="form-hint">Start age</div>
                                </div>
                                <div class="form-group">
                                    <label>ISA Limit</label>
                                    <input type="text" id="p2-isa-limit" value="20000">
                                    <div class="form-hint">Annual</div>
                                </div>
                            </div>
                            <details class="advanced-options">
                                <summary>Advanced Options</summary>
                                <div class="form-row-4">
                                    <div class="form-group">
                                        <label>SP Defer</label>
                                        <input type="number" id="p2-sp-defer" value="0" min="0" max="10">
                                        <div class="form-hint">Years (5.8%/yr)</div>
                                    </div>
                                    <div class="form-group">
                                        <label>DB Normal</label>
                                        <input type="number" id="p2-db-normal-age" value="65">
                                        <div class="form-hint">NRA age</div>
                                    </div>
                                    <div class="form-group">
                                        <label>Early %</label>
                                        <input type="number" id="p2-db-early-factor" value="4" step="0.5">
                                        <div class="form-hint">Reduction/yr</div>
                                    </div>
                                    <div class="form-group">
                                        <label>Late %</label>
                                        <input type="number" id="p2-db-late-factor" value="5" step="0.5">
                                        <div class="form-hint">Increase/yr</div>
                                    </div>
                                </div>
                                <div class="form-row">
                                    <div class="form-group">
                                        <label>Commute %</label>
                                        <input type="number" id="p2-db-commute" value="0" min="0" max="25">
                                        <div class="form-hint">DB lump sum</div>
                                    </div>
                                    <div class="form-group">
                                        <label>Factor</label>
                                        <input type="number" id="p2-db-commute-factor" value="12">
                                        <div class="form-hint">Commute factor</div>
                                    </div>
                                </div>
                                <div class="form-section-title" style="margin-top: 0.5rem;">Phased Retirement</div>
                                <div class="form-row-3">
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>Income</label>
                                        <input type="text" id="p2-parttime-income" value="0">
                                        <div class="form-hint">Annual</div>
                                    </div>
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>Start</label>
                                        <input type="number" id="p2-parttime-start" value="55">
                                        <div class="form-hint">Age</div>
                                    </div>
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>End</label>
                                        <input type="number" id="p2-parttime-end" value="60">
                                        <div class="form-hint">Age</div>
                                    </div>
                                </div>
                            </details>
                        </div>
                    </div>
                </div>

                <!-- Income Requirements -->
                <div class="card">
                    <h2 class="collapsible">Income Requirements</h2>
                    <div class="collapse-content">
                        <div id="fixed-income-fields">
                            <div class="form-row">
                                <div class="form-group">
                                    <label>Monthly Before</label>
                                    <input type="text" id="income-before" value="4000">
                                    <div class="form-hint">Before age threshold</div>
                                </div>
                                <div class="form-group">
                                    <label>Monthly After</label>
                                    <input type="text" id="income-after" value="2500">
                                    <div class="form-hint">After age threshold</div>
                                </div>
                            </div>
                        </div>
                        <div id="depletion-fields" class="hidden">
                            <div class="form-row-3">
                                <div class="form-group">
                                    <label>Depletion Age</label>
                                    <input type="number" id="depletion-age" value="80">
                                    <div class="form-hint">Funds run out</div>
                                </div>
                                <div class="form-group">
                                    <label>Ratio Phase 1</label>
                                    <input type="number" id="ratio-phase1" value="5" step="0.5">
                                    <div class="form-hint">Before threshold</div>
                                </div>
                                <div class="form-group">
                                    <label>Ratio Phase 2</label>
                                    <input type="number" id="ratio-phase2" value="3" step="0.5">
                                    <div class="form-hint">After threshold</div>
                                </div>
                            </div>
                            <div class="form-section">
                                <label class="checkbox-label">
                                    <input type="checkbox" id="depletion-growth-decline-enabled">
                                    Growth Rate Decline
                                </label>
                                <div class="form-hint" style="margin-bottom: 0.5rem;">Rates decline linearly to depletion age</div>
                                <div class="form-row">
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>Decline %</label>
                                        <input type="number" id="depletion-growth-decline-percent" value="3" step="0.5" min="0" max="10">
                                        <div class="form-hint">e.g. 3%: 7%→4%</div>
                                    </div>
                                </div>
                            </div>
                        </div>
                        <div class="form-row">
                            <div class="form-group">
                                <label>Age Threshold</label>
                                <input type="number" id="age-threshold" value="67">
                                <div class="form-hint">Income changes at</div>
                            </div>
                            <div class="form-group">
                                <label>Reference Person</label>
                                <select id="ref-person">
                                    <option value="Person1">Person1</option>
                                    <option value="Person2">Person2</option>
                                </select>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Income Strategies -->
                <div class="card">
                    <h2 class="collapsible collapsed">Income Strategies</h2>
                    <div class="collapse-content collapsed">
                        <div class="form-hint" style="margin-bottom: 0.75rem;">Dynamic withdrawal adjustments based on portfolio performance and age</div>

                        <!-- Guardrails Strategy -->
                        <div class="form-section">
                            <label class="checkbox-label">
                                <input type="checkbox" id="guardrails-enabled">
                                Guardrails (Guyton-Klinger)
                            </label>
                            <div class="form-hint">Adjust withdrawals when portfolio drifts from initial rate</div>
                            <div id="guardrails-options" class="hidden" style="margin-top: 0.5rem;">
                                <div class="form-row-3">
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>Upper %</label>
                                        <input type="number" id="guardrails-upper" value="120" step="5">
                                        <div class="form-hint">Reduce if above</div>
                                    </div>
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>Lower %</label>
                                        <input type="number" id="guardrails-lower" value="80" step="5">
                                        <div class="form-hint">Increase if below</div>
                                    </div>
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>Adjust %</label>
                                        <input type="number" id="guardrails-adjust" value="10" step="5">
                                        <div class="form-hint">Adjustment amount</div>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- VPW Strategy -->
                        <div class="form-section">
                            <label class="checkbox-label">
                                <input type="checkbox" id="vpw-enabled">
                                VPW (Variable Percentage)
                            </label>
                            <div class="form-hint">Rate increases with age: 3% at 55 → 35% at 100</div>
                            <div id="vpw-options" class="hidden" style="margin-top: 0.5rem;">
                                <div class="form-row">
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>Floor</label>
                                        <input type="text" id="vpw-floor" value="0">
                                        <div class="form-hint">Min annual (0=none)</div>
                                    </div>
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>Ceiling</label>
                                        <input type="number" id="vpw-ceiling" value="0" step="0.1">
                                        <div class="form-hint">× floor (0=none)</div>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Emergency Fund -->
                        <div class="form-section" style="margin-bottom: 0;">
                            <div class="form-section-title">Emergency Fund</div>
                            <div class="form-row">
                                <div class="form-group" style="margin-bottom: 0;">
                                    <label>ISA Reserve</label>
                                    <input type="number" id="emergency-months" value="0" min="0" max="24">
                                    <div class="form-hint">Months to preserve</div>
                                </div>
                                <div class="form-group" style="margin-bottom: 0;">
                                    <label class="checkbox-label" style="margin-top: 1.2rem;">
                                        <input type="checkbox" id="emergency-inflate">
                                        Inflation Adjust
                                    </label>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Financial Settings -->
                <div class="card">
                    <h2 class="collapsible collapsed">Financial Settings</h2>
                    <div class="collapse-content collapsed">
                        <div class="form-row-4">
                            <div class="form-group">
                                <label>Pension %</label>
                                <input type="number" id="pension-growth" value="5" step="0.5">
                                <div class="form-hint">Growth rate</div>
                            </div>
                            <div class="form-group">
                                <label>ISA %</label>
                                <input type="number" id="savings-growth" value="5" step="0.5">
                                <div class="form-hint">Growth rate</div>
                            </div>
                            <div class="form-group">
                                <label>Inflation %</label>
                                <input type="number" id="income-inflation" value="3" step="0.5">
                                <div class="form-hint">Income needs</div>
                            </div>
                            <div class="form-group">
                                <label>SP Inflation %</label>
                                <input type="number" id="sp-inflation" value="3" step="0.5">
                                <div class="form-hint">State pension</div>
                            </div>
                        </div>
                        <div class="form-row">
                            <div class="form-group">
                                <label>State Pension</label>
                                <input type="text" id="state-pension" value="12547.60">
                                <div class="form-hint">Annual amount</div>
                            </div>
                        </div>

                        <div class="form-section">
                            <div class="form-section-title">Sensitivity Ranges</div>
                            <div class="form-row-4">
                                <div class="form-group" style="margin-bottom: 0;">
                                    <label>Pen Min %</label>
                                    <input type="number" id="pension-growth-min" value="4" step="1">
                                </div>
                                <div class="form-group" style="margin-bottom: 0;">
                                    <label>Pen Max %</label>
                                    <input type="number" id="pension-growth-max" value="12" step="1">
                                </div>
                                <div class="form-group" style="margin-bottom: 0;">
                                    <label>ISA Min %</label>
                                    <input type="number" id="savings-growth-min" value="4" step="1">
                                </div>
                                <div class="form-group" style="margin-bottom: 0;">
                                    <label>ISA Max %</label>
                                    <input type="number" id="savings-growth-max" value="12" step="1">
                                </div>
                            </div>
                        </div>

                        <div class="form-section" style="margin-bottom: 0;">
                            <label class="checkbox-label">
                                <input type="checkbox" id="growth-decline-enabled">
                                Gradual Growth Decline
                            </label>
                            <div class="form-hint">Age in bonds: rates decline linearly to target age</div>
                            <div id="growth-decline-fields" style="display: none; margin-top: 0.5rem;">
                                <div class="form-row-4">
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>Pen End %</label>
                                        <input type="number" id="pension-growth-end" value="4" step="0.5">
                                    </div>
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>ISA End %</label>
                                        <input type="number" id="savings-growth-end" value="4" step="0.5">
                                    </div>
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>Target Age</label>
                                        <input type="number" id="growth-decline-target-age" value="80">
                                    </div>
                                    <div class="form-group" style="margin-bottom: 0;">
                                        <label>Ref Person</label>
                                        <select id="growth-decline-ref-person">
                                            <option value="">Same as sim</option>
                                        </select>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Tax Settings -->
                <div class="card">
                    <h2 class="collapsible collapsed">Tax Settings</h2>
                    <div class="collapse-content collapsed">
                        <div class="form-section">
                            <div class="form-section-title">Personal Allowance Tapering</div>
                            <div class="form-hint" style="margin-bottom: 0.5rem;">For income over threshold, PA reduces by tapering rate</div>
                            <div class="form-row-3">
                                <div class="form-group">
                                    <label>Personal Allowance</label>
                                    <input type="text" id="tax-personal-allowance" value="12570">
                                    <div class="form-hint">Tax-free amount</div>
                                </div>
                                <div class="form-group">
                                    <label>Taper Threshold</label>
                                    <input type="text" id="tax-tapering-threshold" value="100000">
                                    <div class="form-hint">PA starts reducing</div>
                                </div>
                                <div class="form-group">
                                    <label>Taper Rate</label>
                                    <input type="number" id="tax-tapering-rate" value="0.5" step="0.1" min="0" max="1">
                                    <div class="form-hint">PA lost per £1</div>
                                </div>
                            </div>
                            <div class="form-row">
                                <div class="form-group">
                                    <label>Band Inflation %</label>
                                    <input type="number" id="tax-band-inflation" value="3" step="0.5">
                                    <div class="form-hint">Annual tax band adjustment (0 = frozen)</div>
                                </div>
                            </div>
                        </div>

                        <div class="form-section" style="margin-bottom: 0;">
                            <div class="form-section-title">Tax Bands (2024/25)</div>
                            <div id="tax-bands-container">
                                <div class="tax-band-row" data-band="0">
                                    <div class="form-row-4" style="align-items: end;">
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <label>Name</label>
                                            <input type="text" class="tax-band-name" value="Personal Allowance" readonly style="background: var(--bg-darker);">
                                        </div>
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <label>From</label>
                                            <input type="text" class="tax-band-lower" value="0" readonly style="background: var(--bg-darker);">
                                        </div>
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <label>To</label>
                                            <input type="text" class="tax-band-upper" value="12570">
                                        </div>
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <label>Rate %</label>
                                            <input type="number" class="tax-band-rate" value="0" step="1" readonly style="background: var(--bg-darker);">
                                        </div>
                                    </div>
                                </div>
                                <div class="tax-band-row" data-band="1">
                                    <div class="form-row-4" style="align-items: end;">
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <input type="text" class="tax-band-name" value="Basic Rate">
                                        </div>
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <input type="text" class="tax-band-lower" value="12570">
                                        </div>
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <input type="text" class="tax-band-upper" value="50270">
                                        </div>
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <input type="number" class="tax-band-rate" value="20" step="1">
                                        </div>
                                    </div>
                                </div>
                                <div class="tax-band-row" data-band="2">
                                    <div class="form-row-4" style="align-items: end;">
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <input type="text" class="tax-band-name" value="Higher Rate">
                                        </div>
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <input type="text" class="tax-band-lower" value="50270">
                                        </div>
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <input type="text" class="tax-band-upper" value="125140">
                                        </div>
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <input type="number" class="tax-band-rate" value="40" step="1">
                                        </div>
                                    </div>
                                </div>
                                <div class="tax-band-row" data-band="3">
                                    <div class="form-row-4" style="align-items: end;">
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <input type="text" class="tax-band-name" value="Additional Rate">
                                        </div>
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <input type="text" class="tax-band-lower" value="125140">
                                        </div>
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <input type="text" class="tax-band-upper" value="10000000">
                                        </div>
                                        <div class="form-group" style="margin-bottom: 0;">
                                            <input type="number" class="tax-band-rate" value="45" step="1">
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Mortgage -->
                <div class="card">
                    <h2 class="collapsible collapsed">Mortgage</h2>
                    <div class="collapse-content collapsed">
                        <div id="mortgage-parts-container"></div>
                        <button type="button" class="btn" onclick="addMortgagePart()" style="margin-top: 0.25rem;">+ Add Mortgage</button>
                        <div class="form-row" style="margin-top: 0.5rem;">
                            <div class="form-group">
                                <label>Early Payoff Year</label>
                                <input type="number" id="mortgage-early" value="2028">
                                <div class="form-hint">All parts</div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Simulation -->
                <div class="card">
                    <h2 class="collapsible collapsed">Simulation Settings</h2>
                    <div class="collapse-content collapsed">
                        <div class="form-row-3">
                            <div class="form-group">
                                <label>Start Year</label>
                                <input type="number" id="sim-start" value="2026">
                            </div>
                            <div class="form-group">
                                <label>End Age</label>
                                <input type="number" id="sim-end" value="90">
                                <div class="form-hint">Display to</div>
                            </div>
                            <div class="form-group">
                                <label>Ref Person</label>
                                <select id="sim-ref-person">
                                    <option value="Person1">Person1</option>
                                    <option value="Person2">Person2</option>
                                </select>
                            </div>
                        </div>
                        <div style="margin-top: 0.5rem;">
                            <label class="checkbox-label">
                                <input type="checkbox" id="maximize-couple-isa" checked>
                                Maximize couple ISA transfers
                            </label>
                            <div class="form-hint">Combined: fill both ISA allowances (£40k/yr)</div>
                        </div>
                    </div>
                </div>

                <div class="card">
                    <button class="btn btn-primary" id="run-btn" style="width: 100%;">
                        Run Simulation
                    </button>
                    <div class="sensitivity-toggle">
                        <input type="checkbox" id="sensitivity-mode">
                        <label for="sensitivity-mode">Run Sensitivity Grid (vary growth rates)</label>
                    </div>
                </div>
        </div>

        <!-- Right Column - Results Panel -->
        <div class="results-panel" id="results-panel">
            <!-- Summary Bar -->
            <div class="summary-bar" id="summary-bar">
                <div class="summary-item">
                    <span class="label">Mode</span>
                    <span class="value" id="summary-mode">Fixed Income</span>
                </div>
                <div class="summary-divider"></div>
                <div class="summary-item">
                    <span class="label">Pension Growth</span>
                    <span class="value" id="summary-pension-rate">5%</span>
                </div>
                <div class="summary-item">
                    <span class="label">Savings Growth</span>
                    <span class="value" id="summary-savings-rate">5%</span>
                </div>
                <div class="summary-divider"></div>
                <div class="summary-item" id="summary-best-container" style="display:none;">
                    <span class="label">Best Strategy</span>
                    <span class="value" id="summary-best">-</span>
                </div>
            </div>

            <div class="card results-card" id="results-container">
                <div style="display:flex;justify-content:space-between;align-items:center;">
                    <h2 style="margin:0;">Results</h2>
                    <button id="download-btn" class="btn btn-secondary" onclick="downloadStrategyCSV()" style="display:none;">Download CSV</button>
                </div>
                <div class="loading" id="loading">
                    <div class="spinner"></div>
                    <p>Running simulation...</p>
                </div>
                <div id="results-content">
                    <p style="color: var(--text-muted); text-align: center; padding: 2rem;">
                        Configure your settings and click "Run Simulation" to see results.
                    </p>
                </div>
            </div>
        </div>
    </div>

    <script>
        // State
        let currentMode = 'fixed';

        // Toggle config panel
        function toggleConfigPanel() {
            const panel = document.getElementById('config-panel');
            const toggle = document.getElementById('config-toggle');
            panel.classList.toggle('collapsed');
            toggle.classList.toggle('panel-open');
        }

        // Tax popup functions
        function showTaxPopup(year, taxData) {
            const overlay = document.getElementById('tax-popup-overlay');
            const title = document.getElementById('tax-popup-title');
            const content = document.getElementById('tax-popup-content');

            title.textContent = 'Tax Breakdown - ' + year;

            let html = '';
            if (taxData.income_sources) {
                html += '<div class="tax-breakdown-row"><strong>Income Sources</strong></div>';
                if (taxData.income_sources.pension) html += '<div class="tax-breakdown-row"><span>Pension Withdrawals</span><span>' + formatMoney(taxData.income_sources.pension) + '</span></div>';
                if (taxData.income_sources.state_pension) html += '<div class="tax-breakdown-row"><span>State Pension</span><span>' + formatMoney(taxData.income_sources.state_pension) + '</span></div>';
                if (taxData.income_sources.db_pension) html += '<div class="tax-breakdown-row"><span><abbr title="Defined Benefit Pension - Guaranteed pension based on salary and years of service">DB Pension</abbr></span><span>' + formatMoney(taxData.income_sources.db_pension) + '</span></div>';
                if (taxData.income_sources.isa) html += '<div class="tax-breakdown-row"><span><abbr title="Individual Savings Account - Tax-free savings wrapper">ISA</abbr> Withdrawals (tax-free)</span><span>' + formatMoney(taxData.income_sources.isa) + '</span></div>';
            }
            html += '<div class="tax-breakdown-row"><strong>Tax Calculation</strong></div>';
            html += '<div class="tax-breakdown-row"><span>Gross Taxable Income</span><span>' + formatMoney(taxData.gross_income || 0) + '</span></div>';
            html += '<div class="tax-breakdown-row"><span>Personal Allowance</span><span>' + formatMoney(taxData.personal_allowance || 12570) + '</span></div>';
            html += '<div class="tax-breakdown-row total"><span>Tax Paid</span><span>' + formatMoney(taxData.tax_paid || 0) + '</span></div>';

            content.innerHTML = html;
            overlay.classList.add('visible');
        }

        function closeTaxPopup(event) {
            if (!event || event.target === document.getElementById('tax-popup-overlay')) {
                document.getElementById('tax-popup-overlay').classList.remove('visible');
            }
        }

        // Help modal functions
        function showHelp() {
            document.getElementById('help-modal').style.display = 'flex';
        }
        function closeHelp(event) {
            if (!event || event.target === document.getElementById('help-modal')) {
                document.getElementById('help-modal').style.display = 'none';
            }
        }

        // Show tax popup from strategy index and year
        function showTaxPopupFromYear(strategyIdx, year) {
            if (!lastResults || !lastResults.results || !lastResults.results[strategyIdx]) return;
            const r = lastResults.results[strategyIdx];
            const y = r.years.find(yr => yr.year === year);
            if (!y) return;

            const taxData = {
                tax_paid: y.tax_paid,
                gross_income: y.required_income,
                personal_allowance: y.personal_allowance || 12570,
                income_sources: {
                    pension: y.pension_withdrawal || 0,
                    state_pension: y.state_pension || 0,
                    db_pension: y.db_pension || 0,
                    isa: y.isa_withdrawal || 0
                }
            };
            showTaxPopup(year, taxData);
        }

        // Toggle expandable year row
        function toggleYearRow(prefix, year, event) {
            // Prevent toggle when clicking tax link
            if (event && event.target.classList.contains('tax-link')) return;

            const row = document.getElementById('row-' + prefix + '-' + year);
            const details = document.getElementById('details-' + prefix + '-' + year);
            if (row && details) {
                row.classList.toggle('expanded');
                details.classList.toggle('show');
            }
        }

        // Comparison modal functions
        function showCompareModal() {
            if (!lastResults || !lastResults.results) return;

            // Find matching Early vs Normal vs Extended groups (same drawdown strategy)
            const strategies = ['ISAFirst', 'PenFirst', 'TaxOpt', 'Combined', 'PenOnly'];
            let html = '';

            strategies.forEach(base => {
                const early = lastResults.results.find(r => r.short_name === base + '/Early');
                const normal = lastResults.results.find(r => r.short_name === base + '/Normal');
                const extended = lastResults.results.find(r => r.short_name === base + '/Ext+10');
                const pcls = lastResults.results.find(r => r.short_name === base + '/PCLS');
                if (!early && !normal && !extended && !pcls) return;

                // Find best option for highlighting
                const options = [
                    { name: 'Early', data: early },
                    { name: 'Normal', data: normal },
                    { name: 'Ext+10', data: extended },
                    { name: 'PCLS', data: pcls }
                ].filter(o => o.data);
                const lowestTax = options.reduce((min, o) => o.data.total_tax_paid < min.data.total_tax_paid ? o : min, options[0]);
                const lowestMtg = options.reduce((min, o) => o.data.total_mortgage_paid < min.data.total_mortgage_paid ? o : min, options[0]);

                html += '<div style="margin-bottom:1.5rem;border:1px solid var(--border);border-radius:8px;overflow:hidden;">';
                html += '<div style="background:var(--bg-darker);padding:0.75rem;font-weight:600;">' + base + ' Strategy Comparison</div>';

                // Summary stats - show best for each metric
                html += '<div style="display:grid;grid-template-columns:1fr 1fr;gap:1rem;padding:1rem;background:var(--bg);">';

                html += '<div style="text-align:center;"><div style="font-size:0.7rem;color:var(--text-muted);">LOWEST TAX</div>';
                html += '<div style="font-size:1.2rem;font-weight:600;color:var(--success);">' + lowestTax.name + '</div>';
                html += '<div style="font-size:0.8rem;">' + formatMoney(lowestTax.data.total_tax_paid) + '</div></div>';

                html += '<div style="text-align:center;"><div style="font-size:0.7rem;color:var(--text-muted);">LOWEST MORTGAGE COST</div>';
                html += '<div style="font-size:1.2rem;font-weight:600;color:var(--success);">' + lowestMtg.name + '</div>';
                html += '<div style="font-size:0.8rem;">' + formatMoney(lowestMtg.data.total_mortgage_paid) + '</div></div>';

                html += '</div>';

                // Side by side details - 3 columns
                html += '<div style="display:grid;grid-template-columns:1fr 1fr 1fr;font-size:0.8rem;">';

                // Early column
                if (early) {
                    html += '<div style="border-right:1px solid var(--border);">';
                    html += '<div style="background:#d4edda;padding:0.5rem;text-align:center;font-weight:600;">EARLY (' + (early.mortgage_paid_off_year || '-') + ')</div>';
                    html += '<div style="padding:0.5rem;">';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Total Tax:</span><strong>' + formatMoney(early.total_tax_paid) + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Mortgage Paid:</span><strong>' + formatMoney(early.total_mortgage_paid) + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>ISA Depleted:</span><strong>' + (early.isa_depleted_year || 'Never') + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Pension Starts:</span><strong>' + (early.pension_drawdown_year || 'N/A') + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Final Balance:</span><strong>' + formatMoney(early.final_balance) + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Final ISA:</span><strong>' + formatMoney(early.final_isa || 0) + '</strong></div>';
                    html += '</div></div>';
                } else {
                    html += '<div style="border-right:1px solid var(--border);opacity:0.5;"><div style="background:#d4edda;padding:0.5rem;text-align:center;font-weight:600;">EARLY</div><div style="padding:1rem;text-align:center;color:var(--text-muted);">Not available</div></div>';
                }

                // Normal column
                if (normal) {
                    html += '<div style="border-right:1px solid var(--border);">';
                    html += '<div style="background:#cce5ff;padding:0.5rem;text-align:center;font-weight:600;">NORMAL (' + (normal.mortgage_paid_off_year || '-') + ')</div>';
                    html += '<div style="padding:0.5rem;">';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Total Tax:</span><strong>' + formatMoney(normal.total_tax_paid) + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Mortgage Paid:</span><strong>' + formatMoney(normal.total_mortgage_paid) + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>ISA Depleted:</span><strong>' + (normal.isa_depleted_year || 'Never') + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Pension Starts:</span><strong>' + (normal.pension_drawdown_year || 'N/A') + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Final Balance:</span><strong>' + formatMoney(normal.final_balance) + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Final ISA:</span><strong>' + formatMoney(normal.final_isa || 0) + '</strong></div>';
                    html += '</div></div>';
                } else {
                    html += '<div style="border-right:1px solid var(--border);opacity:0.5;"><div style="background:#cce5ff;padding:0.5rem;text-align:center;font-weight:600;">NORMAL</div><div style="padding:1rem;text-align:center;color:var(--text-muted);">Not available</div></div>';
                }

                // Extended column
                if (extended) {
                    html += '<div>';
                    html += '<div style="background:#fff3cd;padding:0.5rem;text-align:center;font-weight:600;">EXTENDED (' + (extended.mortgage_paid_off_year || '-') + ')</div>';
                    html += '<div style="padding:0.5rem;">';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Total Tax:</span><strong>' + formatMoney(extended.total_tax_paid) + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Mortgage Paid:</span><strong>' + formatMoney(extended.total_mortgage_paid) + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>ISA Depleted:</span><strong>' + (extended.isa_depleted_year || 'Never') + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Pension Starts:</span><strong>' + (extended.pension_drawdown_year || 'N/A') + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Final Balance:</span><strong>' + formatMoney(extended.final_balance) + '</strong></div>';
                    html += '<div style="display:flex;justify-content:space-between;padding:0.25rem 0;"><span>Final ISA:</span><strong>' + formatMoney(extended.final_isa || 0) + '</strong></div>';
                    html += '</div></div>';
                } else {
                    html += '<div style="opacity:0.5;"><div style="background:#fff3cd;padding:0.5rem;text-align:center;font-weight:600;">EXTENDED</div><div style="padding:1rem;text-align:center;color:var(--text-muted);">Not available</div></div>';
                }

                html += '</div></div>';
            });

            if (!html) {
                html = '<p style="text-align:center;color:var(--text-muted);">No Early/Normal/Extended groups found to compare.</p>';
            }

            document.getElementById('compare-modal-body').innerHTML = html;
            document.getElementById('compare-modal-overlay').classList.add('visible');
        }

        function closeCompareModal(event) {
            if (!event || event.target === document.getElementById('compare-modal-overlay')) {
                document.getElementById('compare-modal-overlay').classList.remove('visible');
            }
        }

        // Update summary bar
        function updateSummaryBar(data) {
            const modeNames = { 'fixed': 'Fixed Income', 'depletion': 'Depletion', 'pension-only': 'Pension Only', 'pension-to-isa': 'Pension to ISA' };
            document.getElementById('summary-mode').textContent = modeNames[currentMode] || currentMode;
            document.getElementById('summary-pension-rate').textContent = document.getElementById('pension-growth').value + '%';
            document.getElementById('summary-savings-rate').textContent = document.getElementById('savings-growth').value + '%';

            if (data && data.best) {
                document.getElementById('summary-best-container').style.display = 'flex';
                const goalNames = { 'tax': 'Min Tax', 'income': 'Max Income', 'balance': 'Max Balance' };
                const goal = document.getElementById('optimization-goal').value;
                const bestName = data.best.descriptive_name || data.best.short_name;
                document.getElementById('summary-best').textContent = bestName + ' (' + (goalNames[goal] || goal) + ')';
            } else {
                document.getElementById('summary-best-container').style.display = 'none';
            }
        }

        // Mode selection
        document.querySelectorAll('.mode-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                document.querySelectorAll('.mode-btn').forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                currentMode = btn.dataset.mode;
                updateModeFields();
                runSimulation();
            });
        });

        function updateModeFields() {
            const isDepletion = ['depletion', 'pension-only', 'pension-to-isa'].includes(currentMode);
            document.getElementById('fixed-income-fields').classList.toggle('hidden', isDepletion);
            document.getElementById('depletion-fields').classList.toggle('hidden', !isDepletion);

            // Auto-expand Income Requirements section when switching to depletion mode
            if (isDepletion) {
                const incomeHeader = document.querySelector('#fixed-income-fields').closest('.card').querySelector('.collapsible');
                const incomeContent = incomeHeader.nextElementSibling;
                incomeHeader.classList.remove('collapsed');
                incomeContent.classList.remove('collapsed');
            }
        }

        // Collapsible sections
        document.querySelectorAll('.collapsible').forEach(el => {
            el.addEventListener('click', () => {
                el.classList.toggle('collapsed');
                const content = el.nextElementSibling;
                content.classList.toggle('collapsed');
            });
        });

        // Income strategy toggles
        document.getElementById('guardrails-enabled').addEventListener('change', function() {
            document.getElementById('guardrails-options').classList.toggle('hidden', !this.checked);
        });
        document.getElementById('vpw-enabled').addEventListener('change', function() {
            document.getElementById('vpw-options').classList.toggle('hidden', !this.checked);
        });

        // Growth decline toggle
        document.getElementById('growth-decline-enabled').addEventListener('change', function() {
            document.getElementById('growth-decline-fields').style.display = this.checked ? 'block' : 'none';
        });

        // Parse money value (handles k, m suffixes)
        function parseMoney(val) {
            if (!val) return 0;
            val = val.toString().toLowerCase().replace(/[£,\s]/g, '');
            if (val.endsWith('m')) return parseFloat(val) * 1000000;
            if (val.endsWith('k')) return parseFloat(val) * 1000;
            return parseFloat(val) || 0;
        }

        // Format money for display
        function formatMoney(val) {
            if (val === undefined || val === null || isNaN(val)) return 'N/A';
            if (val >= 1000000) return '£' + (val / 1000000).toFixed(1) + 'M';
            if (val >= 1000) return '£' + Math.round(val / 1000) + 'k';
            return '£' + Math.round(val);
        }

        // Mortgage parts management
        let mortgagePartIndex = 0;

        function addMortgagePart(data = null) {
            const container = document.getElementById('mortgage-parts-container');
            const idx = mortgagePartIndex++;
            const part = document.createElement('div');
            part.className = 'mortgage-part';
            part.id = 'mortgage-part-' + idx;
            part.innerHTML =
                '<div class="mortgage-part-header">' +
                    '<span>Part ' + (container.children.length + 1) + '</span>' +
                    '<button type="button" class="remove-mortgage-btn" onclick="removeMortgagePart(' + idx + ')">Remove</button>' +
                '</div>' +
                '<div class="form-row-3">' +
                    '<div class="form-group">' +
                        '<label>Name</label>' +
                        '<input type="text" id="mortgage-name-' + idx + '" value="' + (data?.name || 'Part ' + (container.children.length + 1)) + '">' +
                    '</div>' +
                    '<div class="form-group">' +
                        '<label>Principal</label>' +
                        '<input type="text" id="mortgage-principal-' + idx + '" value="' + (data?.principal || 0) + '">' +
                    '</div>' +
                    '<div class="form-group">' +
                        '<label>Interest Rate (%)</label>' +
                        '<input type="number" id="mortgage-rate-' + idx + '" value="' + (data?.interest_rate ? (data.interest_rate * 100).toFixed(2) : 4) + '" step="0.1">' +
                    '</div>' +
                '</div>' +
                '<div class="form-row-3">' +
                    '<div class="form-group">' +
                        '<label>Type</label>' +
                        '<select id="mortgage-type-' + idx + '">' +
                            '<option value="repayment"' + (data?.is_repayment !== false ? ' selected' : '') + '>Repayment</option>' +
                            '<option value="interest-only"' + (data?.is_repayment === false ? ' selected' : '') + '>Interest Only</option>' +
                        '</select>' +
                    '</div>' +
                    '<div class="form-group">' +
                        '<label>Start Year</label>' +
                        '<input type="number" id="mortgage-start-' + idx + '" value="' + (data?.start_year || 2020) + '">' +
                    '</div>' +
                    '<div class="form-group">' +
                        '<label>Term (Years)</label>' +
                        '<input type="number" id="mortgage-term-' + idx + '" value="' + (data?.term_years || 25) + '">' +
                    '</div>' +
                '</div>';
            container.appendChild(part);
        }

        function removeMortgagePart(idx) {
            const part = document.getElementById('mortgage-part-' + idx);
            if (part) {
                part.remove();
                // Renumber remaining parts
                const container = document.getElementById('mortgage-parts-container');
                Array.from(container.children).forEach((p, i) => {
                    p.querySelector('.mortgage-part-header span').textContent = 'Part ' + (i + 1);
                });
            }
        }

        function getMortgageParts() {
            const container = document.getElementById('mortgage-parts-container');
            const parts = [];
            Array.from(container.children).forEach(part => {
                const idx = part.id.replace('mortgage-part-', '');
                parts.push({
                    name: document.getElementById('mortgage-name-' + idx).value,
                    principal: parseMoney(document.getElementById('mortgage-principal-' + idx).value),
                    interest_rate: parseFloat(document.getElementById('mortgage-rate-' + idx).value) / 100,
                    is_repayment: document.getElementById('mortgage-type-' + idx).value === 'repayment',
                    start_year: parseInt(document.getElementById('mortgage-start-' + idx).value),
                    term_years: parseInt(document.getElementById('mortgage-term-' + idx).value)
                });
            });
            return parts;
        }

        // Build request from form
        function buildRequest() {
            const people = [
                {
                    name: document.getElementById('p1-name').value,
                    birth_date: document.getElementById('p1-birth').value,
                    retirement_age: parseInt(document.getElementById('p1-retire').value),
                    state_pension_age: parseInt(document.getElementById('p1-spa').value),
                    pension: parseMoney(document.getElementById('p1-pension').value),
                    tax_free_savings: parseMoney(document.getElementById('p1-isa').value),
                    db_pension_name: document.getElementById('p1-db-name').value,
                    db_pension_amount: parseMoney(document.getElementById('p1-db-amount').value),
                    db_pension_start_age: parseInt(document.getElementById('p1-db-age').value) || 0,
                    isa_annual_limit: parseMoney(document.getElementById('p1-isa-limit').value),
                    // Advanced options
                    state_pension_defer_years: parseInt(document.getElementById('p1-sp-defer').value) || 0,
                    db_pension_normal_age: parseInt(document.getElementById('p1-db-normal-age').value) || 65,
                    db_pension_early_factor: parseFloat(document.getElementById('p1-db-early-factor').value) / 100 || 0.04,
                    db_pension_late_factor: parseFloat(document.getElementById('p1-db-late-factor').value) / 100 || 0.05,
                    db_pension_commutation: parseFloat(document.getElementById('p1-db-commute').value) / 100 || 0,
                    db_pension_commute_factor: parseFloat(document.getElementById('p1-db-commute-factor').value) || 12,
                    part_time_income: parseMoney(document.getElementById('p1-parttime-income').value),
                    part_time_start_age: parseInt(document.getElementById('p1-parttime-start').value) || 55,
                    part_time_end_age: parseInt(document.getElementById('p1-parttime-end').value) || 60
                },
                {
                    name: document.getElementById('p2-name').value,
                    birth_date: document.getElementById('p2-birth').value,
                    retirement_age: parseInt(document.getElementById('p2-retire').value),
                    state_pension_age: parseInt(document.getElementById('p2-spa').value),
                    pension: parseMoney(document.getElementById('p2-pension').value),
                    tax_free_savings: parseMoney(document.getElementById('p2-isa').value),
                    db_pension_name: document.getElementById('p2-db-name').value,
                    db_pension_amount: parseMoney(document.getElementById('p2-db-amount').value),
                    db_pension_start_age: parseInt(document.getElementById('p2-db-age').value) || 0,
                    isa_annual_limit: parseMoney(document.getElementById('p2-isa-limit').value),
                    // Advanced options
                    state_pension_defer_years: parseInt(document.getElementById('p2-sp-defer').value) || 0,
                    db_pension_normal_age: parseInt(document.getElementById('p2-db-normal-age').value) || 65,
                    db_pension_early_factor: parseFloat(document.getElementById('p2-db-early-factor').value) / 100 || 0.04,
                    db_pension_late_factor: parseFloat(document.getElementById('p2-db-late-factor').value) / 100 || 0.05,
                    db_pension_commutation: parseFloat(document.getElementById('p2-db-commute').value) / 100 || 0,
                    db_pension_commute_factor: parseFloat(document.getElementById('p2-db-commute-factor').value) || 12,
                    part_time_income: parseMoney(document.getElementById('p2-parttime-income').value),
                    part_time_start_age: parseInt(document.getElementById('p2-parttime-start').value) || 55,
                    part_time_end_age: parseInt(document.getElementById('p2-parttime-end').value) || 60
                }
            ];

            const isDepletion = ['depletion', 'pension-only', 'pension-to-isa'].includes(currentMode);
            const optimizationGoal = document.getElementById('optimization-goal').value;

            return {
                mode: currentMode,
                optimization_goal: optimizationGoal,
                people: people,
                financial: {
                    pension_growth_rate: parseFloat(document.getElementById('pension-growth').value) / 100,
                    savings_growth_rate: parseFloat(document.getElementById('savings-growth').value) / 100,
                    income_inflation_rate: parseFloat(document.getElementById('income-inflation').value) / 100,
                    state_pension_amount: parseMoney(document.getElementById('state-pension').value),
                    state_pension_inflation: parseFloat(document.getElementById('sp-inflation').value) / 100,
                    tax_band_inflation: parseFloat(document.getElementById('tax-band-inflation').value) / 100,
                    emergency_fund_months: parseInt(document.getElementById('emergency-months').value) || 0,
                    emergency_fund_inflation_adjust: document.getElementById('emergency-inflate').checked,
                    growth_decline_enabled: document.getElementById('growth-decline-enabled').checked,
                    pension_growth_end_rate: parseFloat(document.getElementById('pension-growth-end').value) / 100,
                    savings_growth_end_rate: parseFloat(document.getElementById('savings-growth-end').value) / 100,
                    growth_decline_target_age: parseInt(document.getElementById('growth-decline-target-age').value) || 80,
                    growth_decline_reference_person: document.getElementById('growth-decline-ref-person').value,
                    depletion_growth_decline_enabled: document.getElementById('depletion-growth-decline-enabled').checked,
                    depletion_growth_decline_percent: parseFloat(document.getElementById('depletion-growth-decline-percent').value) / 100 || 0.03
                },
                income_requirements: {
                    monthly_before_age: isDepletion ? 0 : parseMoney(document.getElementById('income-before').value),
                    monthly_after_age: isDepletion ? 0 : parseMoney(document.getElementById('income-after').value),
                    target_depletion_age: isDepletion ? parseInt(document.getElementById('depletion-age').value) : 0,
                    income_ratio_phase1: isDepletion ? parseFloat(document.getElementById('ratio-phase1').value) : 0,
                    income_ratio_phase2: isDepletion ? parseFloat(document.getElementById('ratio-phase2').value) : 0,
                    age_threshold: parseInt(document.getElementById('age-threshold').value),
                    reference_person: document.getElementById('ref-person').value,
                    // Guardrails strategy
                    guardrails_enabled: document.getElementById('guardrails-enabled').checked,
                    guardrails_upper_limit: parseFloat(document.getElementById('guardrails-upper').value) / 100 || 1.20,
                    guardrails_lower_limit: parseFloat(document.getElementById('guardrails-lower').value) / 100 || 0.80,
                    guardrails_adjustment: parseFloat(document.getElementById('guardrails-adjust').value) / 100 || 0.10,
                    // VPW strategy
                    vpw_enabled: document.getElementById('vpw-enabled').checked,
                    vpw_floor: parseMoney(document.getElementById('vpw-floor').value),
                    vpw_ceiling: parseFloat(document.getElementById('vpw-ceiling').value) || 0
                },
                mortgage: (() => {
                    const parts = getMortgageParts();
                    const endYear = parts.length > 0 ? Math.max(...parts.map(p => p.start_year + p.term_years)) : 2045;
                    return {
                        parts: parts,
                        end_year: endYear,
                        early_payoff_year: parseInt(document.getElementById('mortgage-early').value)
                    };
                })(),
                simulation: {
                    start_year: parseInt(document.getElementById('sim-start').value),
                    end_age: parseInt(document.getElementById('sim-end').value),
                    reference_person: document.getElementById('sim-ref-person').value
                },
                strategy: {
                    maximize_couple_isa: document.getElementById('maximize-couple-isa').checked
                },
                tax: {
                    personal_allowance: parseMoney(document.getElementById('tax-personal-allowance').value),
                    tapering_threshold: parseMoney(document.getElementById('tax-tapering-threshold').value),
                    tapering_rate: parseFloat(document.getElementById('tax-tapering-rate').value)
                },
                tax_bands: getTaxBands()
            };
        }

        // Get tax bands from UI
        function getTaxBands() {
            const bands = [];
            document.querySelectorAll('.tax-band-row').forEach(row => {
                bands.push({
                    name: row.querySelector('.tax-band-name').value,
                    lower: parseMoney(row.querySelector('.tax-band-lower').value),
                    upper: parseMoney(row.querySelector('.tax-band-upper').value),
                    rate: parseFloat(row.querySelector('.tax-band-rate').value) / 100
                });
            });
            return bands;
        }

        // Run simulation function (called on mode change and button click)
        async function runSimulation() {
            const loading = document.getElementById('loading');
            const content = document.getElementById('results-content');
            const btn = document.getElementById('run-btn');

            loading.classList.add('show');
            content.innerHTML = '';
            btn.disabled = true;

            try {
                const req = buildRequest();
                const res = await fetch('/api/simulate', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(req)
                });

                const data = await res.json();
                loading.classList.remove('show');
                btn.disabled = false;

                if (!data.success) {
                    content.innerHTML = '<p style="color: var(--danger);">Error: ' + data.error + '</p>';
                    return;
                }

                renderResults(data);
            } catch (err) {
                loading.classList.remove('show');
                btn.disabled = false;
                content.innerHTML = '<p style="color: var(--danger);">Error: ' + err.message + '</p>';
            }
        }

        // Handle run button - check if sensitivity mode is enabled
        document.getElementById('run-btn').addEventListener('click', () => {
            if (document.getElementById('sensitivity-mode').checked) {
                runSensitivityAnalysis();
            } else {
                runSimulation();
            }
        });

        // Run sensitivity analysis
        async function runSensitivityAnalysis() {
            const loading = document.getElementById('loading');
            const content = document.getElementById('results-content');
            const btn = document.getElementById('run-btn');

            loading.classList.add('show');
            content.innerHTML = '';
            btn.disabled = true;

            try {
                const req = buildRequest();
                // Add sensitivity parameters
                req.pension_growth_min = parseFloat(document.getElementById('pension-growth-min').value) / 100;
                req.pension_growth_max = parseFloat(document.getElementById('pension-growth-max').value) / 100;
                req.savings_growth_min = parseFloat(document.getElementById('savings-growth-min').value) / 100;
                req.savings_growth_max = parseFloat(document.getElementById('savings-growth-max').value) / 100;
                req.step_size = 0.01; // 1% steps as per plan

                const res = await fetch('/api/simulate/sensitivity', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(req)
                });

                const data = await res.json();
                loading.classList.remove('show');
                btn.disabled = false;

                if (!data.success) {
                    content.innerHTML = '<p style="color: var(--danger);">Error: ' + data.error + '</p>';
                    return;
                }

                renderSensitivityGrid(data);
            } catch (err) {
                loading.classList.remove('show');
                btn.disabled = false;
                content.innerHTML = '<p style="color: var(--danger);">Error: ' + err.message + '</p>';
            }
        }

        // Run simulation with specific growth rates (called from sensitivity grid click)
        function runWithRates(pensionRate, savingsRate) {
            // Set the growth rate inputs
            document.getElementById('pension-growth').value = (pensionRate * 100).toFixed(1);
            document.getElementById('savings-growth').value = (savingsRate * 100).toFixed(1);
            // Uncheck sensitivity mode
            document.getElementById('sensitivity-mode').checked = false;
            // Run normal simulation
            runSimulation();
        }

        // Get cell class based on strategy
        // In depletion mode, hitting target year is success (show strategy color, not ran-out)
        function getCellClass(cell, isDepletion, targetYear) {
            // In depletion mode, only show ran-out if depleted significantly BEFORE target
            if (cell.ran_out) {
                if (isDepletion) {
                    // In depletion mode: on-target or after-target = success, show strategy color
                    // Only show ran-out red if depleted more than 2 years before target
                    if (cell.ran_out_year >= targetYear - 2) {
                        // On target or later - fall through to show strategy color
                    } else {
                        return 'cell-ran-out';
                    }
                } else {
                    // Fixed mode: ran out is always bad
                    return 'cell-ran-out';
                }
            }
            if (cell.has_shortfall) return 'cell-shortfall';
            const strategy = (cell.best_strategy || '').toLowerCase();
            if (strategy.includes('isa') && strategy.includes('first')) return 'cell-isa-first';
            if (strategy.includes('pen') && strategy.includes('first')) return 'cell-pen-first';
            if (strategy.includes('tax')) return 'cell-tax-opt';
            if (strategy.includes('pen') && strategy.includes('isa')) return 'cell-pen-isa';
            return 'cell-unknown';
        }

        // Render sensitivity grid
        function renderSensitivityGrid(data) {
            const content = document.getElementById('results-content');
            const isDepletion = ['depletion', 'pension-only', 'pension-to-isa'].includes(currentMode);
            const p1BirthYear = parseInt(document.getElementById('p1-birth').value.split('-')[0]) || 1970;
            const depletionAge = parseInt(document.getElementById('depletion-age').value) || 90;
            const targetYear = p1BirthYear + depletionAge;

            const cols = data.savings_rates.length + 1;
            let html = '<div class="sensitivity-grid-container">';
            html += '<h3>Sensitivity Analysis Grid</h3>';
            html += '<p style="font-size:0.8rem;color:var(--text-muted);margin-bottom:0.5rem;">Rows: Pension Growth | Columns: Savings Growth</p>';
            html += '<div class="sensitivity-grid" style="grid-template-columns: auto repeat(' + data.savings_rates.length + ', 1fr);">';

            // Header row
            html += '<div class="sensitivity-header"></div>';
            data.savings_rates.forEach(rate => {
                html += '<div class="sensitivity-header">' + (rate * 100).toFixed(0) + '%</div>';
            });

            // Data rows
            data.pension_rates.forEach((pensionRate, pi) => {
                html += '<div class="sensitivity-header">' + (pensionRate * 100).toFixed(0) + '%</div>';
                data.grid[pi].forEach((cell, si) => {
                    const cellClass = getCellClass(cell, isDepletion, targetYear);
                    const savingsRate = data.savings_rates[si];
                    html += '<div class="sensitivity-cell ' + cellClass + '" onclick="runWithRates(' + pensionRate + ',' + savingsRate + ')" title="Click to simulate with Pension: ' + (pensionRate * 100).toFixed(0) + '%, Savings: ' + (savingsRate * 100).toFixed(0) + '%">';
                    if (isDepletion && cell.sustainable_income > 0) {
                        html += '<div class="income">' + formatMoney(cell.sustainable_income) + '</div>';
                    }
                    if (isDepletion) {
                        // In depletion mode, hitting target is success
                        if (!cell.ran_out && !cell.has_shortfall) {
                            html += '<div class="strategy">' + (cell.best_strategy || 'Surplus') + '</div>';
                        } else if (cell.ran_out && cell.ran_out_year >= targetYear - 2) {
                            // On target (within 2 years) - show strategy name
                            html += '<div class="strategy">' + (cell.best_strategy || 'On Target') + '</div>';
                        } else if (cell.ran_out && cell.ran_out_year < targetYear - 2) {
                            // Depleted significantly before target - show warning
                            html += '<div class="strategy">Depleted ' + cell.ran_out_year + '</div>';
                        } else if (cell.has_shortfall) {
                            html += '<div class="strategy">Shortfall</div>';
                        } else {
                            html += '<div class="strategy">' + (cell.best_strategy || 'OK') + '</div>';
                        }
                    } else {
                        // Fixed mode
                        if (cell.ran_out) {
                            html += '<div class="strategy">Depleted ' + cell.ran_out_year + '</div>';
                        } else if (cell.has_shortfall) {
                            html += '<div class="strategy">Shortfall ' + cell.ran_out_year + '</div>';
                        } else {
                            html += '<div class="strategy">' + (cell.best_strategy || 'N/A') + '</div>';
                        }
                    }
                    html += '</div>';
                });
            });

            html += '</div>';

            // Legend
            html += '<div class="sensitivity-legend">';
            html += '<div class="legend-item"><div class="legend-color cell-isa-first"></div>ISA First</div>';
            html += '<div class="legend-item"><div class="legend-color cell-pen-first"></div>Pension First</div>';
            html += '<div class="legend-item"><div class="legend-color cell-tax-opt"></div>Tax Optimized</div>';
            html += '<div class="legend-item"><div class="legend-color cell-pen-isa"></div>Pension to ISA</div>';
            html += '<div class="legend-item"><div class="legend-color cell-shortfall"></div>Shortfall</div>';
            html += '<div class="legend-item"><div class="legend-color cell-ran-out"></div>Depleted</div>';
            html += '</div>';

            html += '</div>';
            content.innerHTML = html;
        }

        // Store last results for detail view
        let lastResults = null;

        // Sort state for All Strategies grid
        let strategySortField = 'name';
        let strategySortDir = 'asc';

        function sortStrategies(results, field, dir) {
            return [...results].sort((a, b) => {
                let valA, valB;
                switch(field) {
                    case 'name':
                        valA = (a.descriptive_name || a.short_name).toLowerCase();
                        valB = (b.descriptive_name || b.short_name).toLowerCase();
                        return dir === 'asc' ? valA.localeCompare(valB) : valB.localeCompare(valA);
                    case 'tax':
                        valA = a.total_tax_paid || 0;
                        valB = b.total_tax_paid || 0;
                        break;
                    case 'income':
                        valA = a.total_income || 0;
                        valB = b.total_income || 0;
                        break;
                    case 'monthly':
                        valA = a.monthly_income || 0;
                        valB = b.monthly_income || 0;
                        break;
                    case 'final':
                        valA = a.final_balance || 0;
                        valB = b.final_balance || 0;
                        break;
                    case 'isa_out':
                        valA = a.isa_depleted_year || 9999;
                        valB = b.isa_depleted_year || 9999;
                        break;
                    case 'pen_out':
                        valA = a.pension_depleted_year || 9999;
                        valB = b.pension_depleted_year || 9999;
                        break;
                    case 'mtg_off':
                        valA = a.mortgage_paid_off_year || 9999;
                        valB = b.mortgage_paid_off_year || 9999;
                        break;
                    default:
                        return 0;
                }
                return dir === 'asc' ? valA - valB : valB - valA;
            });
        }

        function handleSortClick(field) {
            if (strategySortField === field) {
                strategySortDir = strategySortDir === 'asc' ? 'desc' : 'asc';
            } else {
                strategySortField = field;
                strategySortDir = 'asc';
            }
            if (lastResults) renderResults(lastResults);
        }

        // Render results
        function renderResults(data) {
            lastResults = data;
            updateSummaryBar(data);
            // Show download button when results are available
            document.getElementById('download-btn').style.display = data.results && data.results.length > 0 ? 'block' : 'none';
            const content = document.getElementById('results-content');
            const best = data.best;
            const isDepletion = ['depletion', 'pension-only', 'pension-to-isa'].includes(currentMode);

            let html = '<div class="results-grid">';

            if (isDepletion && best) {
                const depletionAge = document.getElementById('depletion-age').value;
                html += '<div class="metric"><div class="metric-value">' + depletionAge + '</div><div class="metric-label">Target Depletion Age</div></div>';
                html += '<div class="metric success"><div class="metric-value">' + formatMoney(best.monthly_income) + '/mo</div><div class="metric-label">Sustainable Income</div></div>';
            }

            if (best) {
                const bestDisplayName = best.descriptive_name || best.short_name;
                html += '<div class="metric"><div class="metric-value" style="font-size:0.9rem;">' + bestDisplayName + '</div><div class="metric-label">Best Strategy</div></div>';
                html += '<div class="metric ' + (best.ran_out_of_money ? 'danger' : 'success') + '"><div class="metric-value">' + formatMoney(best.total_tax_paid) + '</div><div class="metric-label">Total Tax Paid</div></div>';
                html += '<div class="metric"><div class="metric-value">' + formatMoney(best.final_balance) + '</div><div class="metric-label">Final Balance</div></div>';
                if (best.final_isa > 0) {
                    html += '<div class="metric success"><div class="metric-value">' + formatMoney(best.final_isa) + '</div><div class="metric-label">Final ISA</div></div>';
                }
            }

            html += '</div>';

            // Growth decline indicator
            if (data.growth_decline && data.growth_decline.enabled) {
                const gd = data.growth_decline;
                const penStart = (gd.pension_start_rate * 100).toFixed(1);
                const penEnd = (gd.pension_end_rate * 100).toFixed(1);
                const savStart = (gd.savings_start_rate * 100).toFixed(1);
                const savEnd = (gd.savings_end_rate * 100).toFixed(1);
                html += '<div class="growth-decline-indicator" style="background:var(--bg-darker);border:1px solid var(--border);border-radius:6px;padding:0.5rem 0.75rem;margin-bottom:0.75rem;font-size:0.75rem;">';
                html += '<div style="font-weight:600;margin-bottom:0.25rem;color:var(--text);">📉 Growth Rate Decline Active</div>';
                html += '<div style="color:var(--text-muted);">';
                if (penStart === savStart && penEnd === savEnd) {
                    html += 'Growth: ' + penStart + '% → ' + penEnd + '% (' + gd.start_year + ' to ' + gd.end_year + ')';
                } else {
                    html += 'Pension: ' + penStart + '% → ' + penEnd + '% · ISA: ' + savStart + '% → ' + savEnd + '% (' + gd.start_year + ' to ' + gd.end_year + ')';
                }
                html += '</div></div>';
            }

            // Strategy accordion
            html += '<div style="display:flex;align-items:center;margin:1rem 0 0.5rem;"><h3 style="margin:0;">All Strategies</h3><span style="font-weight:normal;font-size:0.75rem;color:var(--text-muted);margin-left:0.5rem;">(click to expand, click headers to sort)</span><button class="compare-btn" onclick="showCompareModal()">Compare Early vs Normal vs Extended</button></div>';

            // Sortable header row
            const sortIcon = (field) => strategySortField === field ? (strategySortDir === 'asc' ? ' ▲' : ' ▼') : '';
            html += '<div class="strategy-sort-header" style="display:flex;gap:0.5rem;padding:0.5rem;background:var(--bg-darker);border-radius:4px;margin-bottom:0.5rem;font-size:0.75rem;font-weight:600;">';
            html += '<div style="flex:1;cursor:pointer;" onclick="handleSortClick(\'name\')">Strategy' + sortIcon('name') + '</div>';
            html += '<div style="width:70px;text-align:right;cursor:pointer;" onclick="handleSortClick(\'tax\')">Tax' + sortIcon('tax') + '</div>';
            html += '<div style="width:70px;text-align:right;cursor:pointer;" onclick="handleSortClick(\'income\')">Income' + sortIcon('income') + '</div>';
            html += '<div style="width:70px;text-align:right;cursor:pointer;" onclick="handleSortClick(\'monthly\')">Monthly' + sortIcon('monthly') + '</div>';
            html += '<div style="width:70px;text-align:right;cursor:pointer;" onclick="handleSortClick(\'final\')">Final' + sortIcon('final') + '</div>';
            html += '<div style="width:80px;text-align:right;cursor:pointer;" onclick="handleSortClick(\'isa_out\')">ISA Out' + sortIcon('isa_out') + '</div>';
            html += '<div style="width:80px;text-align:right;cursor:pointer;" onclick="handleSortClick(\'pen_out\')">Pen Out' + sortIcon('pen_out') + '</div>';
            html += '<div style="width:70px;text-align:right;cursor:pointer;" onclick="handleSortClick(\'mtg_off\')">Mtg Off' + sortIcon('mtg_off') + '</div>';
            html += '</div>';

            html += '<div class="strategy-accordion">';

            // Helper to convert year to age
            const p1BirthYear = parseInt(document.getElementById('p1-birth').value.split('-')[0]) || 1970;
            const p2BirthYear = parseInt(document.getElementById('p2-birth').value.split('-')[0]) || 1970;
            function yearToAge(year) {
                if (!year) return '-';
                return year - p1BirthYear;
            }
            function yearToBothAges(year) {
                if (!year) return '-';
                return (year - p1BirthYear) + '/' + (year - p2BirthYear);
            }

            // Sort results using current sort state
            const sortedResults = sortStrategies(data.results, strategySortField, strategySortDir);

            sortedResults.forEach((r, idx) => {
                const isBest = best && r.short_name === best.short_name;
                const displayName = r.descriptive_name || r.short_name;
                html += '<div class="strategy-accordion-item">';
                html += '<div class="strategy-accordion-header' + (isBest ? ' best' : '') + '" onclick="toggleAccordion(' + idx + ')" id="accordion-header-' + idx + '">';
                html += '<span class="expand-icon">&#9654;</span>';
                html += '<span class="strategy-name">' + displayName + (isBest ? ' ⭐' : '') + '</span>';
                // Show appropriate badge based on mode
                if (isDepletion) {
                    // In depletion mode, hitting the target age is SUCCESS
                    const depletionAge = parseInt(document.getElementById('depletion-age').value);
                    const targetYear = p1BirthYear + depletionAge;
                    if (!r.ran_out_of_money) {
                        html += '<span class="badge badge-info">Surplus</span>';
                    } else if (Math.abs(r.ran_out_year - targetYear) <= 1) {
                        html += '<span class="badge badge-success">' + displayName + '</span>';
                    } else if (r.ran_out_year < targetYear) {
                        html += '<span class="badge badge-danger">Depleted ' + r.ran_out_year + '</span>';
                    } else {
                        html += '<span class="badge badge-success">OK</span>';
                    }
                } else {
                    // Fixed mode: OK, Shortfall (income gap but still have money), or Depleted (truly empty)
                    if (!r.ran_out_of_money) {
                        html += '<span class="badge badge-success">OK</span>';
                    } else if (r.final_balance > 1000) {
                        html += '<span class="badge badge-warning">Shortfall ' + r.ran_out_year + '</span>';
                    } else {
                        html += '<span class="badge badge-danger">Depleted ' + r.ran_out_year + '</span>';
                    }
                }
                html += '<div class="strategy-stats">';
                html += '<div class="stat"><div class="stat-label">Tax</div>' + formatMoney(r.total_tax_paid) + '</div>';
                html += '<div class="stat"><div class="stat-label">Income</div>' + formatMoney(r.total_income) + '</div>';
                html += '<div class="stat"><div class="stat-label">Monthly</div>' + formatMoney(r.monthly_income) + '</div>';
                html += '<div class="stat"><div class="stat-label">Final</div>' + formatMoney(r.final_balance) + '</div>';
                // ISA and Pension depletion with both ages and year
                const isaEnd = r.isa_depleted_year ? yearToBothAges(r.isa_depleted_year) + ' (' + r.isa_depleted_year + ')' : 'Never';
                const penEnd = r.pension_depleted_year ? yearToBothAges(r.pension_depleted_year) + ' (' + r.pension_depleted_year + ')' : 'Never';
                html += '<div class="stat"><div class="stat-label">ISA Out</div>' + isaEnd + '</div>';
                html += '<div class="stat"><div class="stat-label">Pen Out</div>' + penEnd + '</div>';
                html += '<div class="stat"><div class="stat-label">Mtg Off</div>' + (r.mortgage_paid_off_year || '-') + '</div>';
                html += '</div></div>';
                html += '<div class="strategy-accordion-content" id="accordion-content-' + idx + '">';
                // Diagnostic summary
                html += '<div style="padding:0.5rem 0.5rem 0;display:flex;gap:1rem;flex-wrap:wrap;font-size:0.8rem;background:var(--bg-darker);border-radius:4px;margin:0.5rem;">';
                const mtgOptName = r.mortgage_option_name || (r.early_payoff ? 'Early Payoff' : 'Normal');
                const mtgTotal = (r.total_mortgage_paid && !isNaN(r.total_mortgage_paid)) ? formatMoney(r.total_mortgage_paid) : 'N/A';
                html += '<span><strong>Mortgage:</strong> ' + mtgOptName + ' ' + (r.mortgage_paid_off_year || '-') + ' (' + mtgTotal + ' total)</span>';
                html += '<span><strong>ISA Depleted:</strong> ' + (r.isa_depleted_year ? r.isa_depleted_year + ' (Age ' + yearToAge(r.isa_depleted_year) + ')' : 'Never') + '</span>';
                html += '<span><strong>Pension Depleted:</strong> ' + (r.pension_depleted_year ? r.pension_depleted_year + ' (Age ' + yearToAge(r.pension_depleted_year) + ')' : 'Never') + '</span>';
                html += '<span><strong>Pension Drawdown Start:</strong> ' + (r.pension_drawdown_year || 'N/A') + '</span>';
                html += '<span><strong>Final ISA:</strong> ' + formatMoney(r.final_isa || 0) + '</span>';
                html += '</div>';
                // PDF Action Plan button - use strategy_idx (original index) not sorted idx
                html += '<div style="padding:0.5rem;display:flex;gap:0.5rem;flex-wrap:wrap;">';
                html += '<button onclick="downloadPDFActionPlan(event, ' + r.strategy_idx + ', \'' + r.short_name.replace(/'/g, "\\'") + '\')" class="btn btn-secondary btn-sm" style="display:flex;align-items:center;gap:0.3rem;">';
                html += '<span style="font-size:1.1em;">📄</span> Download PDF Action Plan';
                html += '</button>';
                html += '</div>';
                html += buildYearTable(r, idx);
                html += '</div></div>';
            });

            html += '</div>';
            content.innerHTML = html;
        }

        // Toggle accordion
        function toggleAccordion(idx) {
            const header = document.getElementById('accordion-header-' + idx);
            const content = document.getElementById('accordion-content-' + idx);
            header.classList.toggle('expanded');
            content.classList.toggle('expanded');
        }

        // Build year-by-year table for accordion
        function buildYearTable(r, idx) {
            if (!r.years || r.years.length === 0) return '<p style="padding:1rem;color:var(--text-muted);">No year details available</p>';

            // Use the actual mortgage payoff year from the strategy result
            const mortgagePayoffYear = r.mortgage_paid_off_year || 0;
            const p1Birth = document.getElementById('p1-birth').value;
            const p2Birth = document.getElementById('p2-birth').value;
            const p1Retire = parseInt(document.getElementById('p1-retire').value) || 0;
            const p2Retire = parseInt(document.getElementById('p2-retire').value) || 0;
            const p1SPA = parseInt(document.getElementById('p1-spa').value) || 0;
            const p2SPA = parseInt(document.getElementById('p2-spa').value) || 0;
            const p1DBAge = parseInt(document.getElementById('p1-db-age').value) || 0;
            const p2DBAge = parseInt(document.getElementById('p2-db-age').value) || 0;

            function yearFromAge(birthDate, age) {
                if (!birthDate || !age) return 0;
                return parseInt(birthDate.split('-')[0]) + age;
            }

            // Find pension and ISA depletion years
            let pensionDepletedYear = 0;
            let isaDepletedYear = 0;
            let prevPensionTotal = -1;
            let prevISATotal = -1;
            r.years.forEach(y => {
                if (y.balances) {
                    let pensionTotal = 0;
                    let isaTotal = 0;
                    Object.values(y.balances).forEach(bal => {
                        pensionTotal += (bal.uncrystallised_pot || 0) + (bal.crystallised_pot || 0);
                        isaTotal += bal.isa || 0;
                    });
                    // Detect pension depletion (was > 0, now 0)
                    if (prevPensionTotal > 0 && pensionTotal <= 0 && !pensionDepletedYear) {
                        pensionDepletedYear = y.year;
                    }
                    // Detect ISA depletion (was > 0, now 0)
                    if (prevISATotal > 0 && isaTotal <= 0 && !isaDepletedYear) {
                        isaDepletedYear = y.year;
                    }
                    prevPensionTotal = pensionTotal;
                    prevISATotal = isaTotal;
                }
            });

            const milestones = {
                mortgage: mortgagePayoffYear,
                retire: [yearFromAge(p1Birth, p1Retire), yearFromAge(p2Birth, p2Retire)],
                spa: [yearFromAge(p1Birth, p1SPA), yearFromAge(p2Birth, p2SPA)],
                db: [yearFromAge(p1Birth, p1DBAge), yearFromAge(p2Birth, p2DBAge)],
                pensionDepleted: pensionDepletedYear,
                isaDepleted: isaDepletedYear
            };

            let html = '<div style="padding:0.5rem;">';
            html += '<div style="font-size:0.7rem;margin-bottom:0.5rem;display:flex;gap:1rem;flex-wrap:wrap;">';
            html += '<span style="background:#fff3cd;padding:2px 6px;border-radius:3px;">Mortgage</span>';
            html += '<span style="background:#d1e7dd;padding:2px 6px;border-radius:3px;">Retirement</span>';
            html += '<span style="background:#cfe2ff;padding:2px 6px;border-radius:3px;">State Pension</span>';
            html += '<span style="background:#e2d9f3;padding:2px 6px;border-radius:3px;"><abbr title="Defined Benefit Pension starts" style="text-decoration:none;">DB Pension</abbr></span>';
            if (pensionDepletedYear) html += '<span style="background:#f8d7da;padding:2px 6px;border-radius:3px;">⚠ Pension Depleted</span>';
            if (isaDepletedYear) html += '<span style="background:#ffe5d0;padding:2px 6px;border-radius:3px;"><abbr title="Individual Savings Account depleted" style="text-decoration:none;">ISA</abbr> Depleted</span>';
            html += '</div>';
            html += '<table class="detail-table"><thead><tr>';
            html += '<th>Year</th><th>Age</th><th>Event</th><th>Required</th><th>Pensions</th><th>Tax</th><th>Net</th><th>Delta</th><th>Balance</th>';
            html += '</tr></thead><tbody>';

            // Calculate colspan for detail rows
            const colspan = 9;

            r.years.forEach((y, yi) => {
                const ages = Object.values(y.ages || {}).join('/');
                let rowClass = 'expandable-row';
                let event = '';

                // Check for warning events first (pension/ISA depleted)
                if (y.year === milestones.pensionDepleted) { rowClass += ' highlight-pension-depleted'; event = '⚠ Pen Out'; }
                else if (y.year === milestones.isaDepleted) { rowClass += ' highlight-isa-depleted'; event = 'ISA Out'; }
                // Then check for milestone events
                else if (y.year === milestones.mortgage) { rowClass += ' highlight-mortgage'; event = 'Mortgage'; }
                else if (milestones.retire.includes(y.year)) { rowClass += ' highlight-retire'; event = 'Retire'; }
                else if (milestones.spa.includes(y.year)) { rowClass += ' highlight-spa'; event = 'State Pen'; }
                else if (milestones.db.includes(y.year)) { rowClass += ' highlight-db'; event = 'DB Pen'; }

                // Main row (clickable to expand)
                html += '<tr id="row-s' + idx + '-' + y.year + '" class="' + rowClass + '" onclick="toggleYearRow(\'s' + idx + '\', ' + y.year + ', event)">';
                html += '<td>' + y.year + '</td>';
                html += '<td>' + ages + '</td>';
                html += '<td>' + event + '</td>';
                html += '<td>' + formatMoney(y.required_income) + '</td>';
                html += '<td>' + formatMoney((y.state_pension || 0) + (y.db_pension || 0)) + '</td>';
                html += '<td><span class="tax-link" onclick="event.stopPropagation(); showTaxPopupFromYear(' + idx + ', ' + y.year + ')">' + formatMoney(y.tax_paid) + '</span></td>';
                html += '<td>' + formatMoney(y.net_income) + '</td>';
                const delta = (y.net_income || 0) - (y.required_income || 0);
                const deltaColor = delta >= 0 ? 'var(--success)' : 'var(--danger)';
                html += '<td style="color:' + deltaColor + ';">' + (delta >= 0 ? '+' : '') + formatMoney(delta) + '</td>';
                html += '<td>' + formatMoney(y.total_balance) + '</td>';
                html += '</tr>';

                // Detail row (hidden by default)
                html += '<tr id="details-s' + idx + '-' + y.year + '" class="year-details-row">';
                html += '<td colspan="' + colspan + '">';
                html += '<div class="year-detail-grid">';

                // Income breakdown section
                html += '<div class="year-detail-section">';
                html += '<h5>Income Sources</h5>';
                if (y.state_pension > 0) html += '<div class="year-detail-item"><span>State Pension</span><span>' + formatMoney(y.state_pension) + '</span></div>';
                if (y.db_pension > 0) html += '<div class="year-detail-item"><span><abbr title="Defined Benefit Pension - Guaranteed pension based on salary and years of service">DB Pension</abbr></span><span>' + formatMoney(y.db_pension) + '</span></div>';
                if (y.isa_withdrawal > 0) html += '<div class="year-detail-item"><span><abbr title="Individual Savings Account - Tax-free savings wrapper">ISA</abbr> Withdrawal</span><span>' + formatMoney(y.isa_withdrawal) + '</span></div>';
                if (y.pension_withdrawal > 0) html += '<div class="year-detail-item"><span>Pension Withdrawal</span><span>' + formatMoney(y.pension_withdrawal) + '</span></div>';
                if (y.tax_free_withdrawal > 0) html += '<div class="year-detail-item"><span>Tax-Free (<abbr title="Pension Commencement Lump Sum - 25% tax-free withdrawal from pension">PCLS</abbr>)</span><span>' + formatMoney(y.tax_free_withdrawal) + '</span></div>';
                if (y.isa_deposit > 0) html += '<div class="year-detail-item"><span style="color:var(--success);">Excess → ISA</span><span style="color:var(--success);">+' + formatMoney(y.isa_deposit) + '</span></div>';
                html += '</div>';

                // Balances section
                if (y.balances) {
                    html += '<div class="year-detail-section">';
                    html += '<h5>End of Year Balances</h5>';
                    Object.keys(y.balances).forEach(name => {
                        const bal = y.balances[name];
                        html += '<div class="year-detail-item"><span>' + name + ' ISA</span><span>' + formatMoney(bal.isa) + '</span></div>';
                        html += '<div class="year-detail-item"><span>' + name + ' Pension</span><span>' + formatMoney(bal.uncrystallised_pot + bal.crystallised_pot) + '</span></div>';
                    });
                    html += '</div>';
                }

                // Tax breakdown section
                html += '<div class="year-detail-section">';
                html += '<h5>Tax Summary</h5>';
                html += '<div class="year-detail-item"><span>Gross Taxable</span><span>' + formatMoney(y.required_income - (y.isa_withdrawal || 0)) + '</span></div>';
                html += '<div class="year-detail-item"><span>Personal Allowance</span><span>' + formatMoney(y.personal_allowance || 12570) + '</span></div>';
                html += '<div class="year-detail-item"><span style="color:var(--danger);">Tax Paid</span><span style="color:var(--danger);">' + formatMoney(y.tax_paid) + '</span></div>';
                html += '</div>';

                html += '</div></td></tr>';
            });

            html += '</tbody></table></div>';
            return html;
        }

        // Show year-by-year detail for a strategy
        function showDetail(idx) {
            if (!lastResults || !lastResults.results[idx]) return;
            const r = lastResults.results[idx];
            const container = document.getElementById('detail-container');

            if (!r.years || r.years.length === 0) {
                container.innerHTML = '<div class="detail-view"><p>No year details available</p></div>';
                return;
            }

            // Calculate important milestone years for highlighting
            // Use the actual mortgage payoff year from the result, not the form field
            const mortgagePayoffYear = r.mortgage_paid_off_year || 0;
            const p1Birth = document.getElementById('p1-birth').value;
            const p2Birth = document.getElementById('p2-birth').value;
            const p1Retire = parseInt(document.getElementById('p1-retire').value) || 0;
            const p2Retire = parseInt(document.getElementById('p2-retire').value) || 0;
            const p1SPA = parseInt(document.getElementById('p1-spa').value) || 0;
            const p2SPA = parseInt(document.getElementById('p2-spa').value) || 0;
            const p1DBAge = parseInt(document.getElementById('p1-db-age').value) || 0;
            const p2DBAge = parseInt(document.getElementById('p2-db-age').value) || 0;

            // Helper to calculate year from birth date and age
            function yearFromAge(birthDate, age) {
                if (!birthDate || !age) return 0;
                const year = parseInt(birthDate.split('-')[0]);
                return year + age;
            }

            // Find pension and ISA depletion years
            let pensionDepletedYear = 0;
            let isaDepletedYear = 0;
            let prevPensionTotal = -1;
            let prevISATotal = -1;
            r.years.forEach(y => {
                if (y.balances) {
                    let pensionTotal = 0;
                    let isaTotal = 0;
                    Object.values(y.balances).forEach(bal => {
                        pensionTotal += (bal.uncrystallised_pot || 0) + (bal.crystallised_pot || 0);
                        isaTotal += bal.isa || 0;
                    });
                    if (prevPensionTotal > 0 && pensionTotal <= 0 && !pensionDepletedYear) {
                        pensionDepletedYear = y.year;
                    }
                    if (prevISATotal > 0 && isaTotal <= 0 && !isaDepletedYear) {
                        isaDepletedYear = y.year;
                    }
                    prevPensionTotal = pensionTotal;
                    prevISATotal = isaTotal;
                }
            });

            const milestones = {
                mortgage: mortgagePayoffYear,
                retire: [yearFromAge(p1Birth, p1Retire), yearFromAge(p2Birth, p2Retire)],
                spa: [yearFromAge(p1Birth, p1SPA), yearFromAge(p2Birth, p2SPA)],
                db: [yearFromAge(p1Birth, p1DBAge), yearFromAge(p2Birth, p2DBAge)],
                pensionDepleted: pensionDepletedYear,
                isaDepleted: isaDepletedYear
            };

            let html = '<div class="detail-view">';
            html += '<h4>' + r.strategy + ' <button class="close-btn" onclick="document.getElementById(\'detail-container\').innerHTML=\'\'">&times;</button></h4>';

            // Legend for highlights
            html += '<div style="font-size:0.7rem;margin-bottom:0.5rem;display:flex;gap:1rem;flex-wrap:wrap;">';
            html += '<span><span style="background:#fff3cd;padding:2px 6px;border-radius:3px;">Mortgage Payoff</span></span>';
            html += '<span><span style="background:#d1e7dd;padding:2px 6px;border-radius:3px;">Retirement</span></span>';
            html += '<span><span style="background:#cfe2ff;padding:2px 6px;border-radius:3px;">State Pension</span></span>';
            html += '<span><span style="background:#e2d9f3;padding:2px 6px;border-radius:3px;"><abbr title="Defined Benefit Pension starts" style="text-decoration:none;">DB Pension</abbr></span></span>';
            if (pensionDepletedYear) html += '<span><span style="background:#f8d7da;padding:2px 6px;border-radius:3px;">⚠ Pension Depleted</span></span>';
            if (isaDepletedYear) html += '<span><span style="background:#ffe5d0;padding:2px 6px;border-radius:3px;"><abbr title="Individual Savings Account depleted" style="text-decoration:none;">ISA</abbr> Depleted</span></span>';
            html += '</div>';

            html += '<table class="detail-table"><thead><tr>';
            html += '<th>Year</th><th>Age</th><th>Event</th><th>Required</th><th>Pensions</th><th>Tax</th><th>Net Income</th><th>Delta</th><th>Balance</th>';
            html += '</tr></thead><tbody>';

            const colspan = 9;

            r.years.forEach(y => {
                const ages = Object.values(y.ages || {}).join('/');
                let rowClass = 'expandable-row';
                let event = '';

                // Check for warning events first (pension/ISA depleted)
                if (y.year === milestones.pensionDepleted) {
                    rowClass += ' highlight-pension-depleted';
                    event = '⚠ Pension Depleted';
                } else if (y.year === milestones.isaDepleted) {
                    rowClass += ' highlight-isa-depleted';
                    event = 'ISA Depleted';
                }
                // Then check for milestone events
                else if (y.year === milestones.mortgage) {
                    rowClass += ' highlight-mortgage';
                    event = 'Mortgage Paid';
                } else if (milestones.retire.includes(y.year)) {
                    rowClass += ' highlight-retire';
                    event = 'Retirement';
                } else if (milestones.spa.includes(y.year)) {
                    rowClass += ' highlight-spa';
                    event = 'State Pension';
                } else if (milestones.db.includes(y.year)) {
                    rowClass += ' highlight-db';
                    event = 'DB Pension';
                }

                // Main row (clickable to expand)
                html += '<tr id="row-d' + idx + '-' + y.year + '" class="' + rowClass + '" onclick="toggleYearRow(\'d' + idx + '\', ' + y.year + ', event)">';
                html += '<td>' + y.year + '</td>';
                html += '<td>' + ages + '</td>';
                html += '<td>' + event + '</td>';
                html += '<td>' + formatMoney(y.required_income) + '</td>';
                html += '<td>' + formatMoney((y.state_pension || 0) + (y.db_pension || 0)) + '</td>';
                html += '<td><span class="tax-link" onclick="event.stopPropagation(); showTaxPopupFromYear(' + idx + ', ' + y.year + ')">' + formatMoney(y.tax_paid) + '</span></td>';
                html += '<td>' + formatMoney(y.net_income) + '</td>';
                const delta2 = (y.net_income || 0) - (y.required_income || 0);
                const deltaColor2 = delta2 >= 0 ? 'var(--success)' : 'var(--danger)';
                html += '<td style="color:' + deltaColor2 + ';">' + (delta2 >= 0 ? '+' : '') + formatMoney(delta2) + '</td>';
                html += '<td>' + formatMoney(y.total_balance) + '</td>';
                html += '</tr>';

                // Detail row (hidden by default)
                html += '<tr id="details-d' + idx + '-' + y.year + '" class="year-details-row">';
                html += '<td colspan="' + colspan + '">';
                html += '<div class="year-detail-grid">';

                // Income breakdown section
                html += '<div class="year-detail-section">';
                html += '<h5>Income Sources</h5>';
                if (y.state_pension > 0) html += '<div class="year-detail-item"><span>State Pension</span><span>' + formatMoney(y.state_pension) + '</span></div>';
                if (y.db_pension > 0) html += '<div class="year-detail-item"><span><abbr title="Defined Benefit Pension - Guaranteed pension based on salary and years of service">DB Pension</abbr></span><span>' + formatMoney(y.db_pension) + '</span></div>';
                if (y.isa_withdrawal > 0) html += '<div class="year-detail-item"><span><abbr title="Individual Savings Account - Tax-free savings wrapper">ISA</abbr> Withdrawal</span><span>' + formatMoney(y.isa_withdrawal) + '</span></div>';
                if (y.pension_withdrawal > 0) html += '<div class="year-detail-item"><span>Pension Withdrawal</span><span>' + formatMoney(y.pension_withdrawal) + '</span></div>';
                if (y.tax_free_withdrawal > 0) html += '<div class="year-detail-item"><span>Tax-Free (<abbr title="Pension Commencement Lump Sum - 25% tax-free withdrawal from pension">PCLS</abbr>)</span><span>' + formatMoney(y.tax_free_withdrawal) + '</span></div>';
                if (y.isa_deposit > 0) html += '<div class="year-detail-item"><span style="color:var(--success);">Excess → ISA</span><span style="color:var(--success);">+' + formatMoney(y.isa_deposit) + '</span></div>';
                html += '</div>';

                // Balances section
                if (y.balances) {
                    html += '<div class="year-detail-section">';
                    html += '<h5>End of Year Balances</h5>';
                    Object.keys(y.balances).forEach(name => {
                        const bal = y.balances[name];
                        html += '<div class="year-detail-item"><span>' + name + ' ISA</span><span>' + formatMoney(bal.isa) + '</span></div>';
                        html += '<div class="year-detail-item"><span>' + name + ' Pension</span><span>' + formatMoney(bal.uncrystallised_pot + bal.crystallised_pot) + '</span></div>';
                    });
                    html += '</div>';
                }

                // Tax breakdown section
                html += '<div class="year-detail-section">';
                html += '<h5>Tax Summary</h5>';
                html += '<div class="year-detail-item"><span>Gross Taxable</span><span>' + formatMoney(y.required_income - (y.isa_withdrawal || 0)) + '</span></div>';
                html += '<div class="year-detail-item"><span>Personal Allowance</span><span>' + formatMoney(y.personal_allowance || 12570) + '</span></div>';
                html += '<div class="year-detail-item"><span style="color:var(--danger);">Tax Paid</span><span style="color:var(--danger);">' + formatMoney(y.tax_paid) + '</span></div>';
                html += '</div>';

                html += '</div></td></tr>';
            });

            html += '</tbody></table></div>';
            container.innerHTML = html;
            container.scrollIntoView({ behavior: 'smooth', block: 'start' });
        }

        // Load config on page load
        async function loadConfig() {
            try {
                const res = await fetch('/api/config');
                const config = await res.json();
                if (config.people && config.people.length > 0) {
                    const p1 = config.people[0];
                    document.getElementById('p1-name').value = p1.name || '';
                    document.getElementById('p1-birth').value = p1.birth_date || '';
                    document.getElementById('p1-retire').value = p1.retirement_age || 55;
                    document.getElementById('p1-spa').value = p1.state_pension_age || 67;
                    document.getElementById('p1-pension').value = p1.pension || 0;
                    document.getElementById('p1-isa').value = p1.tax_free_savings || 0;
                    document.getElementById('p1-db-name').value = p1.db_pension_name || '';
                    document.getElementById('p1-db-amount').value = p1.db_pension_amount || 0;
                    document.getElementById('p1-db-age').value = p1.db_pension_start_age || 67;
                    document.getElementById('p1-isa-limit').value = p1.isa_annual_limit || 20000;

                    if (config.people.length > 1) {
                        const p2 = config.people[1];
                        document.getElementById('p2-name').value = p2.name || '';
                        document.getElementById('p2-birth').value = p2.birth_date || '';
                        document.getElementById('p2-retire').value = p2.retirement_age || 55;
                        document.getElementById('p2-spa').value = p2.state_pension_age || 67;
                        document.getElementById('p2-pension').value = p2.pension || 0;
                        document.getElementById('p2-isa').value = p2.tax_free_savings || 0;
                        document.getElementById('p2-db-name').value = p2.db_pension_name || '';
                        document.getElementById('p2-db-amount').value = p2.db_pension_amount || 0;
                        document.getElementById('p2-db-age').value = p2.db_pension_start_age || 67;
                        document.getElementById('p2-isa-limit').value = p2.isa_annual_limit || 20000;
                    }

                    // Update reference person dropdowns
                    const names = config.people.map(p => p.name);
                    ['ref-person', 'sim-ref-person'].forEach(id => {
                        const select = document.getElementById(id);
                        select.innerHTML = names.map(n => '<option value="' + n + '">' + n + '</option>').join('');
                    });
                    // Growth decline ref person dropdown (with "Same as simulation" option)
                    const growthDeclineRefSelect = document.getElementById('growth-decline-ref-person');
                    growthDeclineRefSelect.innerHTML = '<option value="">Same as simulation</option>' +
                        names.map(n => '<option value="' + n + '">' + n + '</option>').join('');
                }

                // Load income requirements
                if (config.income_requirements) {
                    const ic = config.income_requirements;
                    document.getElementById('income-before').value = ic.monthly_before_age || 6000;
                    document.getElementById('income-after').value = ic.monthly_after_age || 4000;
                    document.getElementById('depletion-age').value = ic.target_depletion_age || 90;
                    document.getElementById('ratio-phase1').value = ic.income_ratio_phase1 || 5;
                    document.getElementById('ratio-phase2').value = ic.income_ratio_phase2 || 3;
                    document.getElementById('age-threshold').value = ic.age_threshold || 67;
                    if (ic.reference_person) {
                        document.getElementById('ref-person').value = ic.reference_person;
                    }
                }

                // Load financial settings
                if (config.financial) {
                    const fin = config.financial;
                    document.getElementById('pension-growth').value = ((fin.pension_growth_rate || 0.05) * 100).toFixed(1);
                    document.getElementById('savings-growth').value = ((fin.savings_growth_rate || 0.05) * 100).toFixed(1);
                    document.getElementById('income-inflation').value = ((fin.income_inflation_rate || 0.03) * 100).toFixed(1);
                    document.getElementById('state-pension').value = fin.state_pension_amount || 12570;
                    document.getElementById('sp-inflation').value = ((fin.state_pension_inflation || 0.03) * 100).toFixed(1);
                    // Growth decline settings
                    document.getElementById('growth-decline-enabled').checked = fin.growth_decline_enabled || false;
                    document.getElementById('pension-growth-end').value = ((fin.pension_growth_end_rate || 0.04) * 100).toFixed(1);
                    document.getElementById('savings-growth-end').value = ((fin.savings_growth_end_rate || 0.04) * 100).toFixed(1);
                    document.getElementById('growth-decline-target-age').value = fin.growth_decline_target_age || 80;
                    if (fin.growth_decline_reference_person) {
                        document.getElementById('growth-decline-ref-person').value = fin.growth_decline_reference_person;
                    }
                    document.getElementById('growth-decline-fields').style.display = fin.growth_decline_enabled ? 'block' : 'none';
                    // Depletion growth decline settings
                    document.getElementById('depletion-growth-decline-enabled').checked = fin.depletion_growth_decline_enabled || false;
                    document.getElementById('depletion-growth-decline-percent').value = ((fin.depletion_growth_decline_percent || 0.03) * 100).toFixed(1);
                }

                // Load simulation settings
                if (config.simulation) {
                    document.getElementById('sim-start').value = config.simulation.start_year || 2026;
                    document.getElementById('sim-end').value = config.simulation.end_age || 95;
                    if (config.simulation.reference_person) {
                        document.getElementById('sim-ref-person').value = config.simulation.reference_person;
                    }
                }

                // Load sensitivity settings
                if (config.sensitivity) {
                    document.getElementById('pension-growth-min').value = ((config.sensitivity.pension_growth_min || 0.04) * 100).toFixed(0);
                    document.getElementById('pension-growth-max').value = ((config.sensitivity.pension_growth_max || 0.12) * 100).toFixed(0);
                    document.getElementById('savings-growth-min').value = ((config.sensitivity.savings_growth_min || 0.04) * 100).toFixed(0);
                    document.getElementById('savings-growth-max').value = ((config.sensitivity.savings_growth_max || 0.12) * 100).toFixed(0);
                }

                // Load strategy settings
                if (config.strategy) {
                    // Default to true if not specified
                    document.getElementById('maximize-couple-isa').checked = config.strategy.maximize_couple_isa !== false;
                }

                // Load tax settings
                if (config.tax) {
                    document.getElementById('tax-personal-allowance').value = config.tax.personal_allowance || 12570;
                    document.getElementById('tax-tapering-threshold').value = config.tax.tapering_threshold || 100000;
                    document.getElementById('tax-tapering-rate').value = config.tax.tapering_rate || 0.5;
                }
                if (config.financial) {
                    document.getElementById('tax-band-inflation').value = ((config.financial.tax_band_inflation || 0.03) * 100).toFixed(1);
                }
                // Load tax bands
                if (config.tax_bands && config.tax_bands.length > 0) {
                    const container = document.getElementById('tax-bands-container');
                    const rows = container.querySelectorAll('.tax-band-row');
                    config.tax_bands.forEach((band, i) => {
                        if (rows[i]) {
                            rows[i].querySelector('.tax-band-name').value = band.name || '';
                            rows[i].querySelector('.tax-band-lower').value = band.lower || 0;
                            rows[i].querySelector('.tax-band-upper').value = band.upper || 0;
                            rows[i].querySelector('.tax-band-rate').value = (band.rate * 100).toFixed(0);
                        }
                    });
                }

                // Load mortgage settings
                if (config.mortgage && config.mortgage.parts && config.mortgage.parts.length > 0) {
                    config.mortgage.parts.forEach(part => addMortgagePart(part));
                    if (config.mortgage.early_payoff_year) {
                        document.getElementById('mortgage-early').value = config.mortgage.early_payoff_year;
                    }
                } else {
                    // Add a default empty mortgage part
                    addMortgagePart();
                }
            } catch (err) {
                console.log('Could not load config:', err);
                // Add a default mortgage part even if config fails
                addMortgagePart();
            }
            // Run initial simulation after config loaded
            runSimulation();
        }

        // Download strategies as CSV
        function downloadStrategyCSV() {
            if (!lastResults || !lastResults.results || lastResults.results.length === 0) {
                alert('No results to download. Run a simulation first.');
                return;
            }

            const modeNames = { 'fixed': 'Fixed Income', 'depletion': 'Depletion', 'pension-only': 'Pension Only', 'pension-to-isa': 'Pension to ISA' };
            const mode = modeNames[currentMode] || currentMode;
            const pensionGrowth = document.getElementById('pension-growth').value + '%';
            const savingsGrowth = document.getElementById('savings-growth').value + '%';

            // Build CSV content
            let csv = '';

            // Add simulation parameters header
            csv += 'Pension Forecast Strategy Export\n';
            csv += 'Mode,' + escapeCSV(mode) + '\n';
            csv += 'Pension Growth Rate,' + pensionGrowth + '\n';
            csv += 'Savings Growth Rate,' + savingsGrowth + '\n';

            // Sort results based on optimization goal
            const isDepletionMode = ['depletion', 'pension-only', 'pension-to-isa'].includes(currentMode);
            const optimizationGoal = document.getElementById('optimization-goal').value;
            const sortedForExport = [...lastResults.results].sort((a, b) => {
                if (isDepletionMode) {
                    // Depletion modes: highest monthly income first
                    return (b.monthly_income || 0) - (a.monthly_income || 0);
                } else {
                    // Fixed mode: sort by optimization goal
                    switch (optimizationGoal) {
                        case 'income':
                            // Higher total income is better
                            return (b.total_income || 0) - (a.total_income || 0);
                        case 'balance':
                            // Higher final balance is better
                            return (b.final_balance || 0) - (a.final_balance || 0);
                        default: // 'tax'
                            // Lower tax is better
                            return (a.total_tax_paid || 0) - (b.total_tax_paid || 0);
                    }
                }
            });

            // Add best strategy header - use server's best if available (respects optimization goal)
            const serverBest = lastResults.best;
            if (serverBest || sortedForExport.length > 0) {
                const best = serverBest || sortedForExport[0];
                const bestName = best.descriptive_name || best.short_name;
                csv += 'Best Strategy,' + escapeCSV(bestName) + '\n';
                if (isDepletionMode && best.monthly_income) {
                    csv += 'Max Monthly Income,' + formatMoneyCSV(best.monthly_income) + '\n';
                } else {
                    // Show metric based on optimization goal
                    switch (optimizationGoal) {
                        case 'income':
                            csv += 'Total Income,' + formatMoneyCSV(best.total_income || 0) + '\n';
                            break;
                        case 'balance':
                            csv += 'Final Balance,' + formatMoneyCSV(best.final_balance || 0) + '\n';
                            break;
                        default: // 'tax'
                            csv += 'Lowest Tax,' + formatMoneyCSV(best.total_tax_paid) + '\n';
                    }
                }
            }
            csv += '\n';

            // Process each strategy (sorted by best first)
            sortedForExport.forEach((r, idx) => {
                const strategyName = r.descriptive_name || r.short_name;

                // Strategy header
                csv += 'Strategy,' + escapeCSV(strategyName) + '\n';
                csv += 'Total Tax Paid,' + formatMoneyCSV(r.total_tax_paid) + '\n';
                csv += 'Total Income,' + formatMoneyCSV(r.total_income || 0) + '\n';
                csv += 'Final Balance,' + formatMoneyCSV(r.final_balance) + '\n';
                csv += 'Final ISA,' + formatMoneyCSV(r.final_isa || 0) + '\n';
                if (r.monthly_income) csv += 'Monthly Income,' + formatMoneyCSV(r.monthly_income) + '\n';
                csv += 'ISA Depleted Year,' + (r.isa_depleted_year || 'Never') + '\n';
                csv += 'Pension Depleted Year,' + (r.pension_depleted_year || 'Never') + '\n';
                csv += 'Mortgage Paid Off Year,' + (r.mortgage_paid_off_year || 'N/A') + '\n';
                csv += '\n';

                // Year-by-year headers
                if (r.years && r.years.length > 0) {
                    csv += 'Year,Ages,Required Income,State Pension,DB Pension,ISA Withdrawal,Pension Withdrawal,Tax Free (PCLS),Tax Paid,Net Income,Total Balance\n';

                    r.years.forEach(y => {
                        const ages = Object.values(y.ages || {}).join('/');
                        csv += y.year + ',';
                        csv += escapeCSV(ages) + ',';
                        csv += formatMoneyCSV(y.required_income) + ',';
                        csv += formatMoneyCSV(y.state_pension || 0) + ',';
                        csv += formatMoneyCSV(y.db_pension || 0) + ',';
                        csv += formatMoneyCSV(y.isa_withdrawal || 0) + ',';
                        csv += formatMoneyCSV(y.pension_withdrawal || 0) + ',';
                        csv += formatMoneyCSV(y.tax_free_withdrawal || 0) + ',';
                        csv += formatMoneyCSV(y.tax_paid || 0) + ',';
                        csv += formatMoneyCSV(y.net_income || 0) + ',';
                        csv += formatMoneyCSV(y.total_balance || 0) + '\n';
                    });
                }

                csv += '\n\n';
            });

            // Save CSV via server API - include parameters in filename
            const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
            const pGrowth = document.getElementById('pension-growth').value;
            const sGrowth = document.getElementById('savings-growth').value;

            // Build descriptive filename with key parameters
            let filenameParts = ['pension', mode.toLowerCase().replace(/\s+/g, '-')];
            filenameParts.push('p' + pGrowth + '-s' + sGrowth);

            // Add mode-specific parameters
            if (currentMode === 'depletion' || currentMode === 'pension-only' || currentMode === 'pension-to-isa') {
                const targetAge = document.getElementById('target-depletion-age');
                if (targetAge) filenameParts.push('age' + targetAge.value);
            }
            if (currentMode === 'fixed') {
                const incomeBefore = document.getElementById('income-before');
                if (incomeBefore) filenameParts.push('inc' + Math.round(parseFloat(incomeBefore.value) || 0));
            }

            filenameParts.push(timestamp);
            const filename = filenameParts.join('-') + '.csv';

            fetch('/api/export-csv', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ content: csv, filename: filename })
            })
            .then(res => res.json())
            .then(data => {
                if (data.success) {
                    showExportNotification(data.file_path);
                } else {
                    alert('Export failed: ' + data.message);
                }
            })
            .catch(err => {
                alert('Export failed: ' + err.message);
            });
        }

        // Show notification with file path and open folder button
        let lastExportPath = '';
        function showExportNotification(filePath) {
            lastExportPath = filePath;

            // Remove existing notification if any
            const existing = document.getElementById('export-notification');
            if (existing) existing.remove();

            const notification = document.createElement('div');
            notification.id = 'export-notification';
            notification.style.cssText = 'position:fixed;bottom:20px;right:20px;background:#065f46;color:white;padding:16px 20px;border-radius:8px;box-shadow:0 4px 12px rgba(0,0,0,0.3);z-index:10000;max-width:500px;font-size:14px;';
            notification.innerHTML = '<div style="display:flex;align-items:flex-start;gap:12px;">' +
                '<div style="flex:1;">' +
                '<div style="font-weight:600;margin-bottom:4px;">CSV Exported Successfully</div>' +
                '<div style="font-size:12px;opacity:0.9;word-break:break-all;">' + filePath + '</div>' +
                '</div>' +
                '<button onclick="this.parentElement.parentElement.remove()" style="background:none;border:none;color:white;font-size:18px;cursor:pointer;padding:0;line-height:1;">&times;</button>' +
                '</div>' +
                '<div style="margin-top:12px;display:flex;gap:8px;">' +
                '<button onclick="openExportFolder()" style="background:white;color:#065f46;border:none;padding:8px 16px;border-radius:4px;cursor:pointer;font-weight:500;">Open Folder</button>' +
                '<button onclick="this.parentElement.parentElement.remove()" style="background:transparent;color:white;border:1px solid rgba(255,255,255,0.5);padding:8px 16px;border-radius:4px;cursor:pointer;">Dismiss</button>' +
                '</div>';
            document.body.appendChild(notification);

            // Auto-dismiss after 15 seconds
            setTimeout(() => {
                const notif = document.getElementById('export-notification');
                if (notif) notif.remove();
            }, 15000);
        }

        function openExportFolder() {
            if (!lastExportPath) return;
            fetch('/api/open-folder', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ file_path: lastExportPath })
            })
            .then(res => res.json())
            .then(data => {
                if (!data.success) {
                    alert('Could not open folder: ' + data.message);
                }
            })
            .catch(err => {
                alert('Could not open folder: ' + err.message);
            });
        }

        // Download PDF Action Plan for a specific strategy
        function downloadPDFActionPlan(event, strategyIdx, strategyName) {
            console.log('PDF export started for strategy:', strategyIdx, strategyName);

            // Show loading indicator
            const btn = event.target.closest('button');
            if (!btn) {
                console.error('Could not find button element');
                alert('Error: Could not find button');
                return;
            }
            const originalText = btn.innerHTML;
            btn.innerHTML = '<span style="font-size:1.1em;">⏳</span> Generating PDF...';
            btn.disabled = true;

            try {
            // Reuse the existing buildRequest function and add strategy_idx
            const requestBody = buildRequest();
            requestBody.strategy_idx = strategyIdx;

            console.log('Sending PDF request:', JSON.stringify(requestBody).substring(0, 200) + '...');

            fetch('/api/export-pdf', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(requestBody)
            })
            .then(res => {
                console.log('PDF response status:', res.status);
                if (!res.ok) {
                    return res.text().then(text => {
                        throw new Error(text || 'Server error: ' + res.status);
                    });
                }
                return res.json();
            })
            .then(data => {
                console.log('PDF response data:', data);
                btn.innerHTML = originalText;
                btn.disabled = false;
                if (data.success) {
                    console.log('Showing PDF notification...');
                    showPDFNotification(data.file_path, strategyName);
                } else {
                    alert('PDF generation failed: ' + (data.message || 'Unknown error'));
                }
            })
            .catch(err => {
                console.error('PDF export error:', err);
                btn.innerHTML = originalText;
                btn.disabled = false;
                alert('PDF generation failed: ' + (err.message || 'Unknown error'));
            });
            } catch (e) {
                console.error('PDF export exception:', e);
                btn.innerHTML = originalText;
                btn.disabled = false;
                alert('PDF generation error: ' + (e.message || 'Unknown error'));
            }
        }

        // Show notification for PDF export
        function showPDFNotification(filePath, strategyName) {
            console.log('showPDFNotification called:', filePath, strategyName);
            lastExportPath = filePath;

            // Remove existing notification if any
            const existing = document.getElementById('export-notification');
            if (existing) existing.remove();

            const notification = document.createElement('div');
            notification.id = 'export-notification';
            notification.style.cssText = 'position:fixed;bottom:20px;right:20px;background:#1e40af;color:white;padding:16px 20px;border-radius:8px;box-shadow:0 4px 12px rgba(0,0,0,0.3);z-index:10000;max-width:500px;font-size:14px;';
            notification.innerHTML = '<div style="display:flex;align-items:flex-start;gap:12px;">' +
                '<div style="flex:1;">' +
                '<div style="font-weight:600;margin-bottom:4px;">PDF Action Plan Generated</div>' +
                '<div style="font-size:12px;opacity:0.9;margin-bottom:4px;">' + strategyName + '</div>' +
                '<div style="font-size:11px;opacity:0.8;word-break:break-all;">' + filePath + '</div>' +
                '</div>' +
                '<button onclick="this.parentElement.parentElement.remove()" style="background:none;border:none;color:white;font-size:18px;cursor:pointer;padding:0;line-height:1;">&times;</button>' +
                '</div>' +
                '<div style="margin-top:12px;display:flex;gap:8px;">' +
                '<button onclick="openExportFolder()" style="background:white;color:#1e40af;border:none;padding:8px 16px;border-radius:4px;cursor:pointer;font-weight:500;">Open Folder</button>' +
                '<button onclick="this.parentElement.parentElement.remove()" style="background:transparent;color:white;border:1px solid rgba(255,255,255,0.5);padding:8px 16px;border-radius:4px;cursor:pointer;">Dismiss</button>' +
                '</div>';
            document.body.appendChild(notification);

            // Auto-dismiss after 15 seconds
            setTimeout(() => {
                const notif = document.getElementById('export-notification');
                if (notif) notif.remove();
            }, 15000);
        }

        // Helper to escape CSV values
        function escapeCSV(val) {
            if (val === null || val === undefined) return '';
            const str = String(val);
            if (str.includes(',') || str.includes('"') || str.includes('\n')) {
                return '"' + str.replace(/"/g, '""') + '"';
            }
            return str;
        }

        // Format money for CSV (no symbol, just number)
        function formatMoneyCSV(val) {
            if (val === null || val === undefined || isNaN(val)) return '0';
            return Math.round(val).toString();
        }

        loadConfig();
    </script>
</body>
</html>
`
