package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// SearchHandler handles global search operations.
type SearchHandler struct {
	db *database.Pool
}

// NewSearchHandler creates a new search handler.
func NewSearchHandler(db *database.Pool) *SearchHandler {
	return &SearchHandler{db: db}
}

// SearchResult represents a single search result item.
type SearchResult struct {
	ID          uuid.UUID  `json:"id"`
	Type        string     `json:"type"` // client, document, service, user
	Title       string     `json:"title"`
	Subtitle    string     `json:"subtitle,omitempty"`
	Description string     `json:"description,omitempty"`
	Status      string     `json:"status,omitempty"`
	URL         string     `json:"url"` // Frontend route
	CreatedAt   time.Time  `json:"created_at"`
}

// SearchResponse represents the search API response.
type SearchResponse struct {
	Query   string         `json:"query"`
	Results []SearchResult `json:"results"`
	Counts  SearchCounts   `json:"counts"`
	Total   int            `json:"total"`
}

// SearchCounts represents counts per result type.
type SearchCounts struct {
	Clients   int `json:"clients"`
	Documents int `json:"documents"`
	Services  int `json:"services"`
	Users     int `json:"users"`
}

// Search performs a global search across clients, documents, services, and users.
// GET /api/v1/search?q=query&limit=20&type=client,document
func (h *SearchHandler) Search(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := middleware.GetRole(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query 'q' is required"})
		return
	}

	// Minimum query length
	if len(query) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query must be at least 2 characters"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit > 50 {
		limit = 50
	}
	limitPerType := limit / 4
	if limitPerType < 3 {
		limitPerType = 3
	}

	searchPattern := "%" + query + "%"
	isStaff := role == "staff"

	var results []SearchResult
	var counts SearchCounts

	// Search Clients
	clientResults, clientCount := h.searchClients(c, tenantDB, tenantID, userID, searchPattern, limitPerType, isStaff)
	results = append(results, clientResults...)
	counts.Clients = clientCount

	// Search Documents
	docResults, docCount := h.searchDocuments(c, tenantDB, tenantID, userID, searchPattern, limitPerType, isStaff)
	results = append(results, docResults...)
	counts.Documents = docCount

	// Search Services
	svcResults, svcCount := h.searchServices(c, tenantDB, tenantID, userID, searchPattern, limitPerType, isStaff)
	results = append(results, svcResults...)
	counts.Services = svcCount

	// Search Users (staff only sees themselves, admin sees all)
	if role == "super_admin" || role == "tenant_admin" {
		userResults, userCount := h.searchUsers(c, tenantDB, tenantID, searchPattern, limitPerType)
		results = append(results, userResults...)
		counts.Users = userCount
	}

	total := counts.Clients + counts.Documents + counts.Services + counts.Users

	c.JSON(http.StatusOK, SearchResponse{
		Query:   query,
		Results: results,
		Counts:  counts,
		Total:   total,
	})
}

func (h *SearchHandler) searchClients(c *gin.Context, tenantDB *middleware.TenantDB, tenantID, userID uuid.UUID, pattern string, limit int, isStaff bool) ([]SearchResult, int) {
	var results []SearchResult
	var count int

	// Build query based on role
	var query string
	var args []interface{}

	if isStaff {
		// Staff only sees assigned clients
		query = `
			SELECT c.id, c.company_name, c.contact_name, c.email, c.status, c.created_at
			FROM clients c
			INNER JOIN staff_clients sc ON c.id = sc.client_id
			WHERE c.tenant_id = $1 AND sc.staff_id = $2 AND c.deleted_at IS NULL
			AND (c.company_name ILIKE $3 OR c.contact_name ILIKE $3 OR c.email ILIKE $3)
			ORDER BY c.company_name
			LIMIT $4
		`
		args = []interface{}{tenantID, userID, pattern, limit}
	} else {
		query = `
			SELECT c.id, c.company_name, c.contact_name, c.email, c.status, c.created_at
			FROM clients c
			WHERE c.tenant_id = $1 AND c.deleted_at IS NULL
			AND (c.company_name ILIKE $2 OR c.contact_name ILIKE $2 OR c.email ILIKE $2)
			ORDER BY c.company_name
			LIMIT $3
		`
		args = []interface{}{tenantID, pattern, limit}
	}

	err := tenantDB.Query(c, query, args, func(rows pgx.Rows) error {
		var id uuid.UUID
		var companyName, contactName, email, status string
		var createdAt time.Time

		if err := rows.Scan(&id, &companyName, &contactName, &email, &status, &createdAt); err != nil {
			return err
		}

		results = append(results, SearchResult{
			ID:          id,
			Type:        "client",
			Title:       companyName,
			Subtitle:    contactName,
			Description: email,
			Status:      status,
			URL:         "/dashboard/clients/" + id.String(),
			CreatedAt:   createdAt,
		})
		count++
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to search clients")
	}

	return results, count
}

func (h *SearchHandler) searchDocuments(c *gin.Context, tenantDB *middleware.TenantDB, tenantID, userID uuid.UUID, pattern string, limit int, isStaff bool) ([]SearchResult, int) {
	var results []SearchResult
	var count int

	var query string
	var args []interface{}

	if isStaff {
		// Staff sees documents for their assigned clients
		query = `
			SELECT d.id, d.name, d.status, d.created_at, COALESCE(c.company_name, 'Firm Document') as client_name
			FROM documents d
			LEFT JOIN clients c ON d.client_id = c.id
			LEFT JOIN staff_clients sc ON c.id = sc.client_id
			WHERE d.tenant_id = $1
			AND (sc.staff_id = $2 OR d.client_id IS NULL)
			AND d.name ILIKE $3
			ORDER BY d.created_at DESC
			LIMIT $4
		`
		args = []interface{}{tenantID, userID, pattern, limit}
	} else {
		query = `
			SELECT d.id, d.name, d.status, d.created_at, COALESCE(c.company_name, 'Firm Document') as client_name
			FROM documents d
			LEFT JOIN clients c ON d.client_id = c.id
			WHERE d.tenant_id = $1 AND d.name ILIKE $2
			ORDER BY d.created_at DESC
			LIMIT $3
		`
		args = []interface{}{tenantID, pattern, limit}
	}

	err := tenantDB.Query(c, query, args, func(rows pgx.Rows) error {
		var id uuid.UUID
		var name, status, clientName string
		var createdAt time.Time

		if err := rows.Scan(&id, &name, &status, &createdAt, &clientName); err != nil {
			return err
		}

		results = append(results, SearchResult{
			ID:          id,
			Type:        "document",
			Title:       name,
			Subtitle:    clientName,
			Status:      status,
			URL:         "/dashboard/documents/" + id.String(),
			CreatedAt:   createdAt,
		})
		count++
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to search documents")
	}

	return results, count
}

func (h *SearchHandler) searchServices(c *gin.Context, tenantDB *middleware.TenantDB, tenantID, userID uuid.UUID, pattern string, limit int, isStaff bool) ([]SearchResult, int) {
	var results []SearchResult
	var count int

	var query string
	var args []interface{}

	if isStaff {
		// Staff sees their assigned services
		query = `
			SELECT s.id, s.name, s.status, s.deadline, s.created_at, COALESCE(c.company_name, '') as client_name
			FROM services s
			LEFT JOIN clients c ON s.client_id = c.id
			WHERE s.tenant_id = $1 AND s.staff_id = $2
			AND (s.name ILIKE $3 OR c.company_name ILIKE $3)
			ORDER BY s.deadline NULLS LAST, s.created_at DESC
			LIMIT $4
		`
		args = []interface{}{tenantID, userID, pattern, limit}
	} else {
		query = `
			SELECT s.id, s.name, s.status, s.deadline, s.created_at, COALESCE(c.company_name, '') as client_name
			FROM services s
			LEFT JOIN clients c ON s.client_id = c.id
			WHERE s.tenant_id = $1
			AND (s.name ILIKE $2 OR c.company_name ILIKE $2)
			ORDER BY s.deadline NULLS LAST, s.created_at DESC
			LIMIT $3
		`
		args = []interface{}{tenantID, pattern, limit}
	}

	err := tenantDB.Query(c, query, args, func(rows pgx.Rows) error {
		var id uuid.UUID
		var name, status, clientName string
		var deadline *time.Time
		var createdAt time.Time

		if err := rows.Scan(&id, &name, &status, &deadline, &createdAt, &clientName); err != nil {
			return err
		}

		subtitle := clientName
		if deadline != nil {
			subtitle += " • Due: " + deadline.Format("Jan 2, 2006")
		}

		results = append(results, SearchResult{
			ID:        id,
			Type:      "service",
			Title:     name,
			Subtitle:  subtitle,
			Status:    status,
			URL:       "/dashboard/services?id=" + id.String(),
			CreatedAt: createdAt,
		})
		count++
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to search services")
	}

	return results, count
}

func (h *SearchHandler) searchUsers(c *gin.Context, tenantDB *middleware.TenantDB, tenantID uuid.UUID, pattern string, limit int) ([]SearchResult, int) {
	var results []SearchResult
	var count int

	query := `
		SELECT id, name, email, role, status, created_at
		FROM users
		WHERE tenant_id = $1 AND deleted_at IS NULL
		AND (name ILIKE $2 OR email ILIKE $2)
		ORDER BY name
		LIMIT $3
	`

	err := tenantDB.Query(c, query, []interface{}{tenantID, pattern, limit}, func(rows pgx.Rows) error {
		var id uuid.UUID
		var name, email, role, status string
		var createdAt time.Time

		if err := rows.Scan(&id, &name, &email, &role, &status, &createdAt); err != nil {
			return err
		}

		results = append(results, SearchResult{
			ID:          id,
			Type:        "user",
			Title:       name,
			Subtitle:    role,
			Description: email,
			Status:      status,
			URL:         "/dashboard/staff?id=" + id.String(),
			CreatedAt:   createdAt,
		})
		count++
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to search users")
	}

	return results, count
}
