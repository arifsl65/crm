package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/cache"
	"github.com/accountant-crm/go-backend/internal/config"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// CompaniesHouseHandler handles Companies House API integration.
type CompaniesHouseHandler struct {
	db      *database.Pool
	redis   *cache.Client
	cfg     config.CompaniesHouseConfig
	client  *http.Client
	circuit *CircuitBreaker
}

// CircuitBreaker implements a simple circuit breaker pattern.
type CircuitBreaker struct {
	mu            sync.RWMutex
	failures      int
	lastFailure   time.Time
	state         CircuitState
	threshold     int           // Number of failures before opening
	resetTimeout  time.Duration // Time to wait before trying again
}

type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		threshold:    threshold,
		resetTimeout: resetTimeout,
		state:        CircuitClosed,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			return true // Allow one request to test
		}
		return false
	case CircuitHalfOpen:
		return true
	}
	return false
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = CircuitClosed
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.threshold {
		cb.state = CircuitOpen
	}
}

func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == CircuitOpen && time.Since(cb.lastFailure) > cb.resetTimeout {
		return CircuitHalfOpen
	}
	return cb.state
}

// NewCompaniesHouseHandler creates a new Companies House handler.
func NewCompaniesHouseHandler(db *database.Pool, redis *cache.Client, cfg config.CompaniesHouseConfig) *CompaniesHouseHandler {
	return &CompaniesHouseHandler{
		db:    db,
		redis: redis,
		cfg:   cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		// Circuit breaker: 5 failures -> 30 second timeout
		circuit: NewCircuitBreaker(5, 30*time.Second),
	}
}

// chSearchItem is the internal struct matching CH API response (uses "title" for company name)
type chSearchItem struct {
	CompanyNumber     string   `json:"company_number"`
	Title             string   `json:"title"` // CH search API uses "title"
	CompanyStatus     string   `json:"company_status"`
	CompanyType       string   `json:"company_type"`
	DateOfCreation    string   `json:"date_of_creation,omitempty"`
	AddressSnippet    string   `json:"address_snippet,omitempty"`
	RegisteredAddress *Address `json:"registered_office_address,omitempty"`
}

// chSearchAPIResponse is the internal struct for CH API search response
type chSearchAPIResponse struct {
	Items        []chSearchItem `json:"items"`
	TotalResults int            `json:"total_results"`
	StartIndex   int            `json:"start_index"`
	ItemsPerPage int            `json:"items_per_page"`
}

// CompanySearchResult represents a company in our API response (uses "company_name")
type CompanySearchResult struct {
	CompanyNumber     string   `json:"company_number"`
	CompanyName       string   `json:"company_name"` // Our API uses "company_name"
	CompanyStatus     string   `json:"company_status"`
	CompanyType       string   `json:"company_type"`
	DateOfCreation    string   `json:"date_of_creation,omitempty"`
	AddressSnippet    string   `json:"address_snippet,omitempty"`
	RegisteredAddress *Address `json:"registered_office_address,omitempty"`
}

// CompanyProfile represents detailed company information.
type CompanyProfile struct {
	CompanyNumber           string   `json:"company_number"`
	CompanyName             string   `json:"company_name"`
	CompanyStatus           string   `json:"company_status"`
	CompanyType             string   `json:"company_type"`
	DateOfCreation          string   `json:"date_of_creation,omitempty"`
	RegisteredOfficeAddress *Address `json:"registered_office_address,omitempty"`
	SICCodes                []string `json:"sic_codes,omitempty"`
	Accounts                *Accounts `json:"accounts,omitempty"`
	ConfirmationStatement   *ConfirmationStatement `json:"confirmation_statement,omitempty"`
}

type Address struct {
	AddressLine1 string `json:"address_line_1,omitempty"`
	AddressLine2 string `json:"address_line_2,omitempty"`
	Locality     string `json:"locality,omitempty"`
	Region       string `json:"region,omitempty"`
	PostalCode   string `json:"postal_code,omitempty"`
	Country      string `json:"country,omitempty"`
}

type Accounts struct {
	AccountingReferenceDate *AccountingDate `json:"accounting_reference_date,omitempty"`
	LastAccounts            *LastAccounts   `json:"last_accounts,omitempty"`
	NextDue                 string          `json:"next_due,omitempty"`
	NextMadeUpTo            string          `json:"next_made_up_to,omitempty"`
}

type AccountingDate struct {
	Day   string `json:"day,omitempty"`
	Month string `json:"month,omitempty"`
}

type LastAccounts struct {
	MadeUpTo string `json:"made_up_to,omitempty"`
	Type     string `json:"type,omitempty"`
}

type ConfirmationStatement struct {
	LastMadeUpTo string `json:"last_made_up_to,omitempty"`
	NextDue      string `json:"next_due,omitempty"`
	NextMadeUpTo string `json:"next_made_up_to,omitempty"`
}

// ============================================================================
// Companies House Officers API Types (for Directors sync)
// ============================================================================

// CHOfficersResponse represents the Companies House officers API response.
type CHOfficersResponse struct {
	Items          []CHOfficer `json:"items"`
	TotalResults   int         `json:"total_results"`
	ActiveCount    int         `json:"active_count"`
	ResignedCount  int         `json:"resigned_count"`
	InactiveCount  int         `json:"inactive_count"`
}

// CHOfficer represents an officer from Companies House.
type CHOfficer struct {
	Name          string       `json:"name"`
	OfficerRole   string       `json:"officer_role"`
	AppointedOn   string       `json:"appointed_on,omitempty"`
	ResignedOn    string       `json:"resigned_on,omitempty"`
	Nationality   string       `json:"nationality,omitempty"`
	Occupation    string       `json:"occupation,omitempty"`
	DateOfBirth   *CHDateOfBirth `json:"date_of_birth,omitempty"`
	Address       *Address     `json:"address,omitempty"`
}

// CHDateOfBirth represents partial date of birth from CH.
type CHDateOfBirth struct {
	Month int `json:"month,omitempty"`
	Year  int `json:"year,omitempty"`
}

// ============================================================================
// Companies House PSC API Types (for Persons with Significant Control sync)
// ============================================================================

// CHPSCResponse represents the Companies House PSC API response.
type CHPSCResponse struct {
	Items        []CHPSC `json:"items"`
	TotalResults int     `json:"total_results"`
	ActiveCount  int     `json:"active_count"`
	CeasedCount  int     `json:"ceased_count"`
}

// CHPSC represents a Person with Significant Control from Companies House.
type CHPSC struct {
	Name             string   `json:"name"`
	NaturesOfControl []string `json:"natures_of_control,omitempty"`
	NotifiedOn       string   `json:"notified_on,omitempty"`
	CeasedOn         string   `json:"ceased_on,omitempty"`
	Kind             string   `json:"kind,omitempty"`
}

// CHSearchResponse represents the Companies House search API response.
type CHSearchResponse struct {
	Items        []CompanySearchResult `json:"items"`
	TotalResults int                   `json:"total_results"`
	StartIndex   int                   `json:"start_index"`
	ItemsPerPage int                   `json:"items_per_page"`
}

// Search searches for companies by name or number.
// GET /api/v1/ch/search?q=company+name
func (h *CompaniesHouseHandler) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_query",
			"message": "Search query 'q' is required",
		})
		return
	}

	// Check circuit breaker
	if !h.circuit.Allow() {
		log.Warn().Msg("Companies House circuit breaker is open")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": "Companies House API is temporarily unavailable",
		})
		return
	}

	// Check cache first
	cacheKey := fmt.Sprintf("ch:search:%s", url.QueryEscape(query))
	cached, err := h.redis.CacheGet(c.Request.Context(), cacheKey)
	if err == nil && cached != "" {
		var results CHSearchResponse
		if json.Unmarshal([]byte(cached), &results) == nil {
			log.Debug().Str("query", query).Msg("Companies House search cache hit")
			c.JSON(http.StatusOK, results)
			return
		}
	}

	// Make API request
	apiURL := fmt.Sprintf("%s/search/companies?q=%s", h.cfg.BaseURL, url.QueryEscape(query))
	results, err := h.makeRequest(c.Request.Context(), apiURL)
	if err != nil {
		h.circuit.RecordFailure()
		log.Error().Err(err).Str("query", query).Msg("Companies House search failed")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "api_error",
			"message": "Failed to search Companies House",
		})
		return
	}

	h.circuit.RecordSuccess()

	// Parse CH API response (uses "title" for company name)
	var chResponse chSearchAPIResponse
	if err := json.Unmarshal(results, &chResponse); err != nil {
		log.Error().Err(err).Msg("Failed to parse Companies House search response")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "parse_error",
			"message": "Failed to parse search results",
		})
		return
	}

	// Transform to our API response format (maps "title" -> "company_name")
	searchResponse := CHSearchResponse{
		TotalResults: chResponse.TotalResults,
		StartIndex:   chResponse.StartIndex,
		ItemsPerPage: chResponse.ItemsPerPage,
		Items:        make([]CompanySearchResult, len(chResponse.Items)),
	}
	for i, item := range chResponse.Items {
		searchResponse.Items[i] = CompanySearchResult{
			CompanyNumber:     item.CompanyNumber,
			CompanyName:       item.Title, // Map title -> company_name
			CompanyStatus:     item.CompanyStatus,
			CompanyType:       item.CompanyType,
			DateOfCreation:    item.DateOfCreation,
			AddressSnippet:    item.AddressSnippet,
			RegisteredAddress: item.RegisteredAddress,
		}
	}

	// Cache the results
	if data, err := json.Marshal(searchResponse); err == nil {
		if err := h.redis.CacheSet(c.Request.Context(), cacheKey, string(data), h.cfg.CacheTTL); err != nil {
			log.Warn().Err(err).Msg("Failed to cache Companies House search results")
		}
	}

	log.Info().Str("query", query).Int("results", searchResponse.TotalResults).Msg("Companies House search completed")
	c.JSON(http.StatusOK, searchResponse)
}

// GetCompany retrieves detailed company information by company number.
// GET /api/v1/ch/company/:number
func (h *CompaniesHouseHandler) GetCompany(c *gin.Context) {
	companyNumber := c.Param("number")
	if companyNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_company_number",
			"message": "Company number is required",
		})
		return
	}

	// Check circuit breaker
	if !h.circuit.Allow() {
		log.Warn().Msg("Companies House circuit breaker is open")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": "Companies House API is temporarily unavailable",
		})
		return
	}

	// Check cache first
	cacheKey := fmt.Sprintf("ch:company:%s", companyNumber)
	cached, err := h.redis.CacheGet(c.Request.Context(), cacheKey)
	if err == nil && cached != "" {
		var profile CompanyProfile
		if json.Unmarshal([]byte(cached), &profile) == nil {
			log.Debug().Str("company_number", companyNumber).Msg("Companies House company cache hit")
			c.JSON(http.StatusOK, profile)
			return
		}
	}

	// Make API request
	apiURL := fmt.Sprintf("%s/company/%s", h.cfg.BaseURL, companyNumber)
	results, err := h.makeRequest(c.Request.Context(), apiURL)
	if err != nil {
		h.circuit.RecordFailure()
		log.Error().Err(err).Str("company_number", companyNumber).Msg("Companies House get company failed")

		// Handle 404 specifically
		if err.Error() == "not_found" {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "Company not found",
			})
			return
		}

		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "api_error",
			"message": "Failed to fetch company details",
		})
		return
	}

	h.circuit.RecordSuccess()

	var profile CompanyProfile
	if err := json.Unmarshal(results, &profile); err != nil {
		log.Error().Err(err).Msg("Failed to parse Companies House company response")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "parse_error",
			"message": "Failed to parse company details",
		})
		return
	}

	// Cache the results
	if data, err := json.Marshal(profile); err == nil {
		if err := h.redis.CacheSet(c.Request.Context(), cacheKey, string(data), h.cfg.CacheTTL); err != nil {
			log.Warn().Err(err).Msg("Failed to cache Companies House company details")
		}
	}

	log.Info().Str("company_number", companyNumber).Str("name", profile.CompanyName).Msg("Companies House company lookup completed")
	c.JSON(http.StatusOK, profile)
}

// SyncClient syncs client data with Companies House.
// POST /api/v1/ch/sync/:clientId
func (h *CompaniesHouseHandler) SyncClient(c *gin.Context) {
	clientIDStr := c.Param("clientId")
	clientID, err := uuid.Parse(clientIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_client_id",
			"message": "Invalid client ID format",
		})
		return
	}

	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "Tenant ID not found",
		})
		return
	}

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Get client's company number
	var companyNumber *string
	err = tenantDB.QueryRowScan(c, []interface{}{&companyNumber},
		`SELECT company_number FROM clients WHERE id = $1 AND tenant_id = $2`,
		clientID, tenantID,
	)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("Failed to get client company number")
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "not_found",
			"message": "Client not found",
		})
		return
	}

	if companyNumber == nil || *companyNumber == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "missing_company_number",
			"message": "Client does not have a company number",
		})
		return
	}

	// Check circuit breaker
	if !h.circuit.Allow() {
		log.Warn().Msg("Companies House circuit breaker is open")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "service_unavailable",
			"message": "Companies House API is temporarily unavailable",
		})
		return
	}

	// Fetch company data from Companies House
	apiURL := fmt.Sprintf("%s/company/%s", h.cfg.BaseURL, *companyNumber)
	results, err := h.makeRequest(c.Request.Context(), apiURL)
	if err != nil {
		h.circuit.RecordFailure()
		log.Error().Err(err).Str("company_number", *companyNumber).Msg("Companies House sync failed")
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "api_error",
			"message": "Failed to fetch company details from Companies House",
		})
		return
	}

	h.circuit.RecordSuccess()

	var profile CompanyProfile
	if err := json.Unmarshal(results, &profile); err != nil {
		log.Error().Err(err).Msg("Failed to parse Companies House response")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "parse_error",
			"message": "Failed to parse company details",
		})
		return
	}

	// Build address string
	var address string
	if profile.RegisteredOfficeAddress != nil {
		addr := profile.RegisteredOfficeAddress
		parts := []string{}
		if addr.AddressLine1 != "" {
			parts = append(parts, addr.AddressLine1)
		}
		if addr.AddressLine2 != "" {
			parts = append(parts, addr.AddressLine2)
		}
		if addr.Locality != "" {
			parts = append(parts, addr.Locality)
		}
		if addr.Region != "" {
			parts = append(parts, addr.Region)
		}
		if addr.PostalCode != "" {
			parts = append(parts, addr.PostalCode)
		}
		if len(parts) > 0 {
			for i, p := range parts {
				if i > 0 {
					address += ", "
				}
				address += p
			}
		}
	}

	// Determine year end from accounting reference date (construct as current year date)
	var yearEnd *time.Time
	if profile.Accounts != nil && profile.Accounts.AccountingReferenceDate != nil {
		day := profile.Accounts.AccountingReferenceDate.Day
		month := profile.Accounts.AccountingReferenceDate.Month
		// Construct a date using current year
		yearEndStr := fmt.Sprintf("%d-%s-%s", time.Now().Year(), month, day)
		if t, err := time.Parse("2006-01-02", yearEndStr); err == nil {
			yearEnd = &t
		}
	}

	// Parse incorporation date
	var incorporationDate *time.Time
	if profile.DateOfCreation != "" {
		if t, err := time.Parse("2006-01-02", profile.DateOfCreation); err == nil {
			incorporationDate = &t
		}
	}

	// Update client record
	_, err = tenantDB.Exec(c, `
		UPDATE clients SET
			company_name = COALESCE($1, company_name),
			company_type = COALESCE($2, company_type),
			incorporation_date = COALESCE($3, incorporation_date),
			address = COALESCE(NULLIF($4, ''), address),
			year_end = COALESCE($5, year_end),
			updated_at = NOW()
		WHERE id = $6 AND tenant_id = $7
	`, profile.CompanyName, profile.CompanyType, incorporationDate, address, yearEnd, clientID, tenantID)

	if err != nil {
		log.Error().Err(err).Str("client_id", clientID.String()).Msg("Failed to update client with CH data")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "update_failed",
			"message": "Failed to update client record",
		})
		return
	}

	// Cache the company profile
	cacheKey := fmt.Sprintf("ch:company:%s", *companyNumber)
	if data, err := json.Marshal(profile); err == nil {
		h.redis.CacheSet(c.Request.Context(), cacheKey, string(data), h.cfg.CacheTTL)
	}

	// Sync directors and PSC in parallel
	var directorsCount, pscCount int
	var directorsErr, pscErr error

	// Sync Directors from CH /officers endpoint
	directorsCount, directorsErr = h.syncDirectors(c, clientID, tenantID, *companyNumber)
	if directorsErr != nil {
		log.Warn().Err(directorsErr).Str("company_number", *companyNumber).Msg("Failed to sync directors (non-fatal)")
	}

	// Sync PSC from CH /persons-with-significant-control endpoint
	pscCount, pscErr = h.syncPSC(c, clientID, tenantID, *companyNumber)
	if pscErr != nil {
		log.Warn().Err(pscErr).Str("company_number", *companyNumber).Msg("Failed to sync PSC (non-fatal)")
	}

	log.Info().
		Str("client_id", clientID.String()).
		Str("company_number", *companyNumber).
		Str("company_name", profile.CompanyName).
		Int("directors_synced", directorsCount).
		Int("psc_synced", pscCount).
		Msg("Client synced with Companies House")

	c.JSON(http.StatusOK, gin.H{
		"message":          "Client synced successfully",
		"company":          profile,
		"directors_synced": directorsCount,
		"psc_synced":       pscCount,
	})
}

// syncDirectors fetches officers from Companies House and syncs them to the directors table.
// Returns the count of directors synced and any error.
func (h *CompaniesHouseHandler) syncDirectors(c *gin.Context, clientID, tenantID uuid.UUID, companyNumber string) (int, error) {
	ctx := c.Request.Context()

	// Fetch officers from Companies House
	apiURL := fmt.Sprintf("%s/company/%s/officers", h.cfg.BaseURL, companyNumber)
	results, err := h.makeRequest(ctx, apiURL)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch officers: %w", err)
	}

	var officersResp CHOfficersResponse
	if err := json.Unmarshal(results, &officersResp); err != nil {
		return 0, fmt.Errorf("failed to parse officers response: %w", err)
	}

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		return 0, fmt.Errorf("tenant DB not found")
	}

	// Mark all existing directors as inactive before sync
	_, err = tenantDB.Exec(c, `
		UPDATE directors SET is_active = false
		WHERE client_id = $1 AND tenant_id = $2
	`, clientID, tenantID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to mark existing directors as inactive")
	}

	syncedCount := 0
	for _, officer := range officersResp.Items {
		// Only sync directors and secretaries
		role := "director"
		if officer.OfficerRole == "secretary" || officer.OfficerRole == "corporate-secretary" {
			role = "secretary"
		} else if officer.OfficerRole != "director" && officer.OfficerRole != "corporate-director" {
			continue // Skip other officer types
		}

		// Parse dates
		var appointedDate, resignedDate *time.Time
		if officer.AppointedOn != "" {
			if t, err := time.Parse("2006-01-02", officer.AppointedOn); err == nil {
				appointedDate = &t
			}
		}
		if officer.ResignedOn != "" {
			if t, err := time.Parse("2006-01-02", officer.ResignedOn); err == nil {
				resignedDate = &t
			}
		}

		// Get DOB month/year
		var dobMonth, dobYear *int
		if officer.DateOfBirth != nil {
			if officer.DateOfBirth.Month > 0 {
				dobMonth = &officer.DateOfBirth.Month
			}
			if officer.DateOfBirth.Year > 0 {
				dobYear = &officer.DateOfBirth.Year
			}
		}

		// Determine if active (no resigned date)
		isActive := resignedDate == nil

		// Upsert director (ON CONFLICT based on client_id, name, role)
		_, err := tenantDB.Exec(c, `
			INSERT INTO directors (tenant_id, client_id, name, role, appointed_date, resigned_date, nationality, dob_month, dob_year, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (client_id, name, role) WHERE is_active = true
			DO UPDATE SET
				appointed_date = COALESCE(EXCLUDED.appointed_date, directors.appointed_date),
				resigned_date = COALESCE(EXCLUDED.resigned_date, directors.resigned_date),
				nationality = COALESCE(EXCLUDED.nationality, directors.nationality),
				dob_month = COALESCE(EXCLUDED.dob_month, directors.dob_month),
				dob_year = COALESCE(EXCLUDED.dob_year, directors.dob_year),
				is_active = EXCLUDED.is_active
		`, tenantID, clientID, officer.Name, role, appointedDate, resignedDate, officer.Nationality, dobMonth, dobYear, isActive)

		if err != nil {
			log.Warn().Err(err).Str("name", officer.Name).Msg("Failed to upsert director")
			continue
		}
		syncedCount++
	}

	log.Info().
		Str("client_id", clientID.String()).
		Str("company_number", companyNumber).
		Int("total_officers", officersResp.TotalResults).
		Int("synced", syncedCount).
		Msg("Directors synced from Companies House")

	return syncedCount, nil
}

// syncPSC fetches Persons with Significant Control from Companies House and syncs them.
// Returns the count of PSC records synced and any error.
func (h *CompaniesHouseHandler) syncPSC(c *gin.Context, clientID, tenantID uuid.UUID, companyNumber string) (int, error) {
	ctx := c.Request.Context()

	// Fetch PSC from Companies House
	apiURL := fmt.Sprintf("%s/company/%s/persons-with-significant-control", h.cfg.BaseURL, companyNumber)
	results, err := h.makeRequest(ctx, apiURL)
	if err != nil {
		// PSC endpoint may return 404 for companies without PSC
		if err.Error() == "not_found" {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to fetch PSC: %w", err)
	}

	var pscResp CHPSCResponse
	if err := json.Unmarshal(results, &pscResp); err != nil {
		return 0, fmt.Errorf("failed to parse PSC response: %w", err)
	}

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		return 0, fmt.Errorf("tenant DB not found")
	}

	// Mark all existing PSC as inactive before sync
	_, err = tenantDB.Exec(c, `
		UPDATE psc SET is_active = false
		WHERE client_id = $1 AND tenant_id = $2
	`, clientID, tenantID)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to mark existing PSC as inactive")
	}

	syncedCount := 0
	for _, psc := range pscResp.Items {
		// Skip non-individual PSC for now (corporate entities, etc.)
		if psc.Kind != "" && psc.Kind != "individual-person-with-significant-control" {
			continue
		}

		// Parse dates
		var notifiedDate, ceasedDate *time.Time
		if psc.NotifiedOn != "" {
			if t, err := time.Parse("2006-01-02", psc.NotifiedOn); err == nil {
				notifiedDate = &t
			}
		}
		if psc.CeasedOn != "" {
			if t, err := time.Parse("2006-01-02", psc.CeasedOn); err == nil {
				ceasedDate = &t
			}
		}

		// Determine ownership percentage from natures_of_control
		ownershipPct := determineOwnershipPercentage(psc.NaturesOfControl)

		// Determine if active (no ceased date)
		isActive := ceasedDate == nil

		// Convert natures_of_control to JSONB
		naturesJSON, _ := json.Marshal(psc.NaturesOfControl)

		// Upsert PSC (ON CONFLICT based on client_id, name)
		_, err := tenantDB.Exec(c, `
			INSERT INTO psc (tenant_id, client_id, name, ownership_percentage, notified_date, ceased_date, nature_of_control, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (client_id, name) WHERE is_active = true
			DO UPDATE SET
				ownership_percentage = COALESCE(EXCLUDED.ownership_percentage, psc.ownership_percentage),
				notified_date = COALESCE(EXCLUDED.notified_date, psc.notified_date),
				ceased_date = COALESCE(EXCLUDED.ceased_date, psc.ceased_date),
				nature_of_control = COALESCE(EXCLUDED.nature_of_control, psc.nature_of_control),
				is_active = EXCLUDED.is_active
		`, tenantID, clientID, psc.Name, ownershipPct, notifiedDate, ceasedDate, naturesJSON, isActive)

		if err != nil {
			log.Warn().Err(err).Str("name", psc.Name).Msg("Failed to upsert PSC")
			continue
		}
		syncedCount++
	}

	log.Info().
		Str("client_id", clientID.String()).
		Str("company_number", companyNumber).
		Int("total_psc", pscResp.TotalResults).
		Int("synced", syncedCount).
		Msg("PSC synced from Companies House")

	return syncedCount, nil
}

// determineOwnershipPercentage parses CH natures_of_control to determine ownership percentage.
// Returns one of: "75%+", "50-75%", "25-50%", or nil if undetermined.
func determineOwnershipPercentage(naturesOfControl []string) *string {
	for _, nature := range naturesOfControl {
		// CH uses strings like "ownership-of-shares-75-to-100-percent"
		if strings.Contains(nature, "75-to-100") || strings.Contains(nature, "more-than-75") {
			pct := "75%+"
			return &pct
		}
		if strings.Contains(nature, "50-to-75") {
			pct := "50-75%"
			return &pct
		}
		if strings.Contains(nature, "25-to-50") {
			pct := "25-50%"
			return &pct
		}
	}
	return nil
}

// makeRequest makes an authenticated request to the Companies House API.
func (h *CompaniesHouseHandler) makeRequest(ctx context.Context, apiURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Companies House uses Basic Auth with API key as username and empty password
	req.SetBasicAuth(h.cfg.APIKey, "")
	req.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("not_found")
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized: invalid API key")
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate_limited: too many requests")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return body, nil
}

// Status returns the current state of the circuit breaker.
// GET /api/v1/ch/status
func (h *CompaniesHouseHandler) Status(c *gin.Context) {
	state := h.circuit.State()
	stateStr := "closed"
	switch state {
	case CircuitOpen:
		stateStr = "open"
	case CircuitHalfOpen:
		stateStr = "half-open"
	}

	c.JSON(http.StatusOK, gin.H{
		"circuit_breaker": stateStr,
		"api_configured":  h.cfg.APIKey != "",
	})
}
