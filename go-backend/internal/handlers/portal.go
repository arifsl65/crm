package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/auth"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// PortalHandler handles client portal HTTP requests.
// All endpoints are scoped to the authenticated client's own data only.
type PortalHandler struct {
	db *database.Pool
}

// NewPortalHandler creates a new PortalHandler instance
func NewPortalHandler(db *database.Pool) *PortalHandler {
	return &PortalHandler{db: db}
}

// PortalDashboard represents the client portal dashboard response
type PortalDashboard struct {
	ClientID       uuid.UUID           `json:"client_id"`
	CompanyName    string              `json:"company_name"`
	ContactName    string              `json:"contact_name"`
	AccountantName *string             `json:"accountant_name,omitempty"`
	AccountantEmail *string            `json:"accountant_email,omitempty"`
	ActionNeeded   []PortalDocRequest  `json:"action_needed"`
	Deadlines      []PortalDeadline    `json:"deadlines"`
	RecentDocs     []PortalDocument    `json:"recent_documents"`
}

// PortalDocRequest represents a document request needing client action
type PortalDocRequest struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	RequestNote *string   `json:"request_note,omitempty"`
	RequestedAt time.Time `json:"requested_at"`
	DaysAgo     int       `json:"days_ago"`
}

// PortalDeadline represents an upcoming deadline for the client
type PortalDeadline struct {
	ID          uuid.UUID `json:"id"`
	ServiceName string    `json:"service_name"`
	Deadline    time.Time `json:"deadline"`
	DaysUntil   int       `json:"days_until"`
	Status      string    `json:"status"`
}

// PortalDocument represents a document in the portal
type PortalDocument struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	TypeName   *string    `json:"type_name,omitempty"`
	UploadedAt *time.Time `json:"uploaded_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// PortalProfile represents the client's profile
type PortalProfile struct {
	ID              uuid.UUID `json:"id"`
	CompanyName     string    `json:"company_name"`
	ContactName     string    `json:"contact_name"`
	Email           string    `json:"email"`
	Phone           *string   `json:"phone,omitempty"`
	Address         *string   `json:"address,omitempty"`
	CompanyNumber   *string   `json:"company_number,omitempty"`
	VATNumber       *string   `json:"vat_number,omitempty"`
	YearEnd         *string   `json:"year_end,omitempty"`
}

// PortalService represents a service in the portal
type PortalService struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Deadline    *time.Time `json:"deadline,omitempty"`
	Period      *string    `json:"period,omitempty"`
	DocsRequired int       `json:"docs_required"`
	DocsReceived int       `json:"docs_received"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// UpdatePortalProfileRequest represents profile update request
type UpdatePortalProfileRequest struct {
	ContactName *string `json:"contact_name,omitempty"`
	Phone       *string `json:"phone,omitempty"`
	Address     *string `json:"address,omitempty"`
}

// ChangePasswordRequest represents password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

// getClientID retrieves the client ID associated with the current user.
// Returns error if user is not a client or has no associated client record.
func (h *PortalHandler) getClientID(c *gin.Context, tenantDB *middleware.TenantDB, userID, tenantID uuid.UUID) (uuid.UUID, error) {
	var clientID uuid.UUID
	err := tenantDB.QueryRowScan(c, []interface{}{&clientID}, `
		SELECT id FROM clients
		WHERE user_id = $1 AND tenant_id = $2
		LIMIT 1
	`, userID, tenantID)
	return clientID, err
}

// Dashboard returns the client portal dashboard
// GET /api/v1/portal/dashboard
func (h *PortalHandler) Dashboard(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Get client ID for this user
	clientID, err := h.getClientID(c, tenantDB, userID, tenantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "client_not_found", "message": "No client record associated with this account"})
			return
		}
		log.Error().Err(err).Msg("Failed to get client ID")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var dashboard PortalDashboard
	dashboard.ClientID = clientID

	// Get client info and assigned accountant
	err = tenantDB.QueryRowScan(c, []interface{}{
		&dashboard.CompanyName,
		&dashboard.ContactName,
		&dashboard.AccountantName,
		&dashboard.AccountantEmail,
	}, `
		SELECT
			c.company_name,
			c.contact_name,
			u.name as accountant_name,
			u.email as accountant_email
		FROM clients c
		LEFT JOIN staff_clients sc ON c.id = sc.client_id AND sc.is_primary = true
		LEFT JOIN users u ON sc.staff_id = u.id
		WHERE c.id = $1 AND c.tenant_id = $2
	`, clientID, tenantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get client info")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Get action needed (requested documents)
	dashboard.ActionNeeded = []PortalDocRequest{}
	tenantDB.Query(c, `
		SELECT id, name, request_note, requested_at
		FROM documents
		WHERE client_id = $1 AND tenant_id = $2 AND status = 'requested'
		ORDER BY requested_at ASC
		LIMIT 10
	`, []interface{}{clientID, tenantID}, func(rows pgx.Rows) error {
		var doc PortalDocRequest
		if err := rows.Scan(&doc.ID, &doc.Name, &doc.RequestNote, &doc.RequestedAt); err != nil {
			return err
		}
		doc.DaysAgo = int(time.Since(doc.RequestedAt).Hours() / 24)
		dashboard.ActionNeeded = append(dashboard.ActionNeeded, doc)
		return nil
	})

	// Get upcoming deadlines (services with deadlines)
	dashboard.Deadlines = []PortalDeadline{}
	tenantDB.Query(c, `
		SELECT id, name, deadline, status
		FROM services
		WHERE client_id = $1 AND tenant_id = $2
			AND deadline IS NOT NULL
			AND status NOT IN ('completed', 'cancelled')
		ORDER BY deadline ASC
		LIMIT 5
	`, []interface{}{clientID, tenantID}, func(rows pgx.Rows) error {
		var dl PortalDeadline
		if err := rows.Scan(&dl.ID, &dl.ServiceName, &dl.Deadline, &dl.Status); err != nil {
			return err
		}
		dl.DaysUntil = int(time.Until(dl.Deadline).Hours() / 24)
		dashboard.Deadlines = append(dashboard.Deadlines, dl)
		return nil
	})

	// Get recent documents
	dashboard.RecentDocs = []PortalDocument{}
	tenantDB.Query(c, `
		SELECT d.id, d.name, d.status, dt.name as type_name, d.created_at
		FROM documents d
		LEFT JOIN document_types dt ON d.type_id = dt.id
		WHERE d.client_id = $1 AND d.tenant_id = $2
		ORDER BY d.created_at DESC
		LIMIT 5
	`, []interface{}{clientID, tenantID}, func(rows pgx.Rows) error {
		var doc PortalDocument
		if err := rows.Scan(&doc.ID, &doc.Name, &doc.Status, &doc.TypeName, &doc.CreatedAt); err != nil {
			return err
		}
		dashboard.RecentDocs = append(dashboard.RecentDocs, doc)
		return nil
	})

	c.JSON(http.StatusOK, dashboard)
}

// GetProfile returns the client's profile
// GET /api/v1/portal/me
func (h *PortalHandler) GetProfile(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var profile PortalProfile
	err := tenantDB.QueryRowScan(c, []interface{}{
		&profile.ID,
		&profile.CompanyName,
		&profile.ContactName,
		&profile.Email,
		&profile.Phone,
		&profile.Address,
		&profile.CompanyNumber,
		&profile.VATNumber,
		&profile.YearEnd,
	}, `
		SELECT id, company_name, contact_name, email, phone, address,
			   company_number, vat_number,
			   CASE WHEN year_end IS NOT NULL THEN TO_CHAR(year_end, 'Mon DD') ELSE NULL END
		FROM clients
		WHERE user_id = $1 AND tenant_id = $2
	`, userID, tenantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "client_not_found"})
			return
		}
		log.Error().Err(err).Msg("Failed to get client profile")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// UpdateProfile updates the client's profile
// PATCH /api/v1/portal/me
func (h *PortalHandler) UpdateProfile(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	tenantID, _ := middleware.GetTenantID(c)

	var req UpdatePortalProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "details": err.Error()})
		return
	}

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Build dynamic update query
	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argCount := 0

	if req.ContactName != nil {
		argCount++
		setClauses = append(setClauses, "contact_name = $"+string(rune('0'+argCount)))
		args = append(args, *req.ContactName)
	}
	if req.Phone != nil {
		argCount++
		setClauses = append(setClauses, "phone = $"+string(rune('0'+argCount)))
		args = append(args, *req.Phone)
	}
	if req.Address != nil {
		argCount++
		setClauses = append(setClauses, "address = $"+string(rune('0'+argCount)))
		args = append(args, *req.Address)
	}

	if argCount == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no_fields_to_update"})
		return
	}

	// Add user_id and tenant_id for WHERE clause
	args = append(args, userID, tenantID)

	query := "UPDATE clients SET " + setClauses[0]
	for i := 1; i < len(setClauses); i++ {
		query += ", " + setClauses[i]
	}
	query += " WHERE user_id = $" + string(rune('0'+argCount+1)) + " AND tenant_id = $" + string(rune('0'+argCount+2))
	query += " RETURNING id, company_name, contact_name, email, phone, address"

	var profile PortalProfile
	err := tenantDB.QueryRowScan(c, []interface{}{
		&profile.ID,
		&profile.CompanyName,
		&profile.ContactName,
		&profile.Email,
		&profile.Phone,
		&profile.Address,
	}, query, args...)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "client_not_found"})
			return
		}
		log.Error().Err(err).Msg("Failed to update client profile")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// ListDocuments returns the client's documents
// GET /api/v1/portal/documents
func (h *PortalHandler) ListDocuments(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	tenantID, _ := middleware.GetTenantID(c)
	status := c.Query("status") // optional filter

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Get client ID
	clientID, err := h.getClientID(c, tenantDB, userID, tenantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "client_not_found"})
			return
		}
		log.Error().Err(err).Msg("Failed to get client ID")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Build query with optional status filter
	query := `
		SELECT d.id, d.name, d.status, dt.name as type_name, d.created_at
		FROM documents d
		LEFT JOIN document_types dt ON d.type_id = dt.id
		WHERE d.client_id = $1 AND d.tenant_id = $2
	`
	args := []interface{}{clientID, tenantID}

	if status != "" {
		query += " AND d.status = $3"
		args = append(args, status)
	}
	query += " ORDER BY d.created_at DESC LIMIT 100"

	documents := []PortalDocument{}
	err = tenantDB.Query(c, query, args, func(rows pgx.Rows) error {
		var doc PortalDocument
		if err := rows.Scan(&doc.ID, &doc.Name, &doc.Status, &doc.TypeName, &doc.CreatedAt); err != nil {
			return err
		}
		documents = append(documents, doc)
		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to list documents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": documents,
		"total":     len(documents),
	})
}

// ListServices returns the client's services
// GET /api/v1/portal/services
func (h *PortalHandler) ListServices(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Get client ID
	clientID, err := h.getClientID(c, tenantDB, userID, tenantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "client_not_found"})
			return
		}
		log.Error().Err(err).Msg("Failed to get client ID")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	services := []PortalService{}
	err = tenantDB.Query(c, `
		SELECT id, name, status, deadline, period, docs_required, docs_received, completed_at
		FROM services
		WHERE client_id = $1 AND tenant_id = $2
		ORDER BY
			CASE WHEN status = 'completed' THEN 1 ELSE 0 END,
			deadline NULLS LAST,
			created_at DESC
		LIMIT 50
	`, []interface{}{clientID, tenantID}, func(rows pgx.Rows) error {
		var svc PortalService
		if err := rows.Scan(&svc.ID, &svc.Name, &svc.Status, &svc.Deadline, &svc.Period,
			&svc.DocsRequired, &svc.DocsReceived, &svc.CompletedAt); err != nil {
			return err
		}
		services = append(services, svc)
		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to list services")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"services": services,
		"total":    len(services),
	})
}

// ListDeadlines returns the client's upcoming deadlines
// GET /api/v1/portal/deadlines
func (h *PortalHandler) ListDeadlines(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Get client ID
	clientID, err := h.getClientID(c, tenantDB, userID, tenantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "client_not_found"})
			return
		}
		log.Error().Err(err).Msg("Failed to get client ID")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	deadlines := []PortalDeadline{}
	now := time.Now()
	err = tenantDB.Query(c, `
		SELECT id, name, deadline, status
		FROM services
		WHERE client_id = $1 AND tenant_id = $2
			AND deadline IS NOT NULL
			AND status NOT IN ('completed', 'cancelled')
		ORDER BY deadline ASC
		LIMIT 20
	`, []interface{}{clientID, tenantID}, func(rows pgx.Rows) error {
		var dl PortalDeadline
		if err := rows.Scan(&dl.ID, &dl.ServiceName, &dl.Deadline, &dl.Status); err != nil {
			return err
		}
		dl.DaysUntil = int(dl.Deadline.Sub(now).Hours() / 24)
		deadlines = append(deadlines, dl)
		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to list deadlines")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Group by overdue, this week, upcoming
	overdue := []PortalDeadline{}
	thisWeek := []PortalDeadline{}
	upcoming := []PortalDeadline{}

	for _, dl := range deadlines {
		if dl.DaysUntil < 0 {
			overdue = append(overdue, dl)
		} else if dl.DaysUntil <= 7 {
			thisWeek = append(thisWeek, dl)
		} else {
			upcoming = append(upcoming, dl)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"overdue":   overdue,
		"this_week": thisWeek,
		"upcoming":  upcoming,
		"total":     len(deadlines),
	})
}

// ChangePassword allows the client to change their password
// POST /api/v1/portal/password
func (h *PortalHandler) ChangePassword(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	tenantID, _ := middleware.GetTenantID(c)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "details": err.Error()})
		return
	}

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Get current password hash
	var currentHash string
	err := tenantDB.QueryRowScan(c, []interface{}{&currentHash}, `
		SELECT password FROM users WHERE id = $1 AND tenant_id = $2
	`, userID, tenantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "user_not_found"})
			return
		}
		log.Error().Err(err).Msg("Failed to get user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Verify current password
	valid, err := auth.VerifyPassword(req.CurrentPassword, currentHash)
	if err != nil {
		log.Error().Err(err).Msg("Failed to verify password")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_password", "message": "Current password is incorrect"})
		return
	}

	// Validate new password strength
	if len(req.NewPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "weak_password", "message": "Password must be at least 8 characters"})
		return
	}

	// Hash new password
	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		log.Error().Err(err).Msg("Failed to hash password")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Update password
	_, err = tenantDB.Exec(c, `
		UPDATE users SET password = $1, updated_at = NOW() WHERE id = $2 AND tenant_id = $3
	`, newHash, userID, tenantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update password")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}
