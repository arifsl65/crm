package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// CompanySearchResult represents a company from search results.
type CompanySearchResult struct {
	CompanyNumber     string  `json:"company_number"`
	CompanyName       string  `json:"company_name"`
	CompanyStatus     string  `json:"company_status"`
	CompanyType       string  `json:"company_type"`
	DateOfCreation    string  `json:"date_of_creation,omitempty"`
	AddressSnippet    string  `json:"address_snippet,omitempty"`
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

	var searchResponse CHSearchResponse
	if err := json.Unmarshal(results, &searchResponse); err != nil {
		log.Error().Err(err).Msg("Failed to parse Companies House search response")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "parse_error",
			"message": "Failed to parse search results",
		})
		return
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

	log.Info().
		Str("client_id", clientID.String()).
		Str("company_number", *companyNumber).
		Str("company_name", profile.CompanyName).
		Msg("Client synced with Companies House")

	c.JSON(http.StatusOK, gin.H{
		"message": "Client synced successfully",
		"company": profile,
	})
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
