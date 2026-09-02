package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/audit"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

type ServiceHandler struct {
	db    *database.Pool
	audit *audit.Logger
}

func NewServiceHandler(db *database.Pool, auditLogger *audit.Logger) *ServiceHandler {
	return &ServiceHandler{
		db:    db,
		audit: auditLogger,
	}
}

// Service represents a service/task record
type Service struct {
	ID              uuid.UUID  `json:"id"`
	TenantID        uuid.UUID  `json:"tenant_id"`
	ClientID        *uuid.UUID `json:"client_id,omitempty"`
	StaffID         *uuid.UUID `json:"staff_id,omitempty"`
	TypeID          *uuid.UUID `json:"type_id,omitempty"`
	Name            string     `json:"name"`
	Period          *string    `json:"period,omitempty"`
	Status          string     `json:"status"`
	Priority        string     `json:"priority"`
	RiskLevel       *string    `json:"risk_level,omitempty"`
	Deadline        *string    `json:"deadline,omitempty"`
	KanbanPosition  int        `json:"kanban_position"`
	DocsRequired    int        `json:"docs_required"`
	DocsReceived    int        `json:"docs_received"`
	HMRCReference   *string    `json:"hmrc_reference,omitempty"`
	FiledAt         *time.Time `json:"filed_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CompletionNotes *string    `json:"completion_notes,omitempty"`
	Version         int        `json:"version"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	// Joined fields
	ClientName *string `json:"client_name,omitempty"`
	StaffName  *string `json:"staff_name,omitempty"`
}

type CreateServiceRequest struct {
	ClientID     string  `json:"client_id" binding:"required,uuid"`
	TypeID       *string `json:"type_id,omitempty"`
	Name         string  `json:"name" binding:"required"`
	Period       *string `json:"period,omitempty"`
	Priority     *string `json:"priority,omitempty"`
	RiskLevel    *string `json:"risk_level,omitempty"`
	Deadline     *string `json:"deadline,omitempty"`
	DocsRequired *int    `json:"docs_required,omitempty"`
	StaffID      *string `json:"staff_id,omitempty"`
}

type UpdateServiceRequest struct {
	Name            *string `json:"name,omitempty"`
	Period          *string `json:"period,omitempty"`
	Status          *string `json:"status,omitempty"`
	Priority        *string `json:"priority,omitempty"`
	RiskLevel       *string `json:"risk_level,omitempty"`
	Deadline        *string `json:"deadline,omitempty"`
	DocsRequired    *int    `json:"docs_required,omitempty"`
	DocsReceived    *int    `json:"docs_received,omitempty"`
	StaffID         *string `json:"staff_id,omitempty"`
	CompletionNotes *string `json:"completion_notes,omitempty"`
}

// List returns all services for the tenant (staff-scoped)
// GET /api/v1/services
func (h *ServiceHandler) List(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := c.Get(middleware.AuthRole)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	// Filters
	status := c.Query("status")
	priority := c.Query("priority")
	clientID := c.Query("client_id")
	search := c.Query("search")

	roleStr, _ := role.(string)
	isSuperAdmin := roleStr == "super_admin"

	var query strings.Builder
	var args []interface{}
	argNum := 1

	query.WriteString(`
		SELECT s.id, s.tenant_id, s.client_id, s.staff_id, s.type_id, s.name, s.period,
		       s.status, s.priority, s.risk_level, s.deadline, s.kanban_position,
		       s.docs_required, s.docs_received, s.hmrc_reference, s.filed_at,
		       s.completed_at, s.completion_notes, s.version, s.created_at, s.updated_at,
		       c.company_name as client_name, COALESCE(u.name, '') as staff_name
		FROM services s
		LEFT JOIN clients c ON s.client_id = c.id
		LEFT JOIN users u ON s.staff_id = u.id
	`)
	if isSuperAdmin {
		query.WriteString(`WHERE 1=1`)
	} else {
		query.WriteString(`WHERE s.tenant_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, tenantID)
		argNum++
	}

	// Staff scoping - only see services for assigned clients unless admin
	if roleStr == "staff" {
		query.WriteString(` AND s.client_id IN (SELECT client_id FROM staff_clients WHERE staff_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, userID)
		argNum++
	}

	// Status filter
	if status != "" {
		query.WriteString(` AND s.status = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, status)
		argNum++
	}

	// Priority filter
	if priority != "" {
		query.WriteString(` AND s.priority = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, priority)
		argNum++
	}

	// Client filter
	if clientID != "" {
		query.WriteString(` AND s.client_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, clientID)
		argNum++
	}

	// Search filter
	if search != "" {
		query.WriteString(` AND (s.name ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(` OR c.company_name ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, "%"+search+"%")
		argNum++
	}

	query.WriteString(` ORDER BY s.deadline ASC NULLS LAST, s.kanban_position ASC, s.created_at DESC LIMIT $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)
	argNum++

	query.WriteString(` OFFSET $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, offset)

	services := []Service{}
	err := tenantDB.Query(c, query.String(), args, func(rows pgx.Rows) error {
		var service Service
		var deadline *time.Time
		err := rows.Scan(
			&service.ID, &service.TenantID, &service.ClientID, &service.StaffID, &service.TypeID,
			&service.Name, &service.Period, &service.Status, &service.Priority, &service.RiskLevel,
			&deadline, &service.KanbanPosition, &service.DocsRequired, &service.DocsReceived,
			&service.HMRCReference, &service.FiledAt, &service.CompletedAt, &service.CompletionNotes,
			&service.Version, &service.CreatedAt, &service.UpdatedAt, &service.ClientName, &service.StaffName,
		)
		if err != nil {
			return err
		}
		if deadline != nil {
			s := deadline.Format("2006-01-02")
			service.Deadline = &s
		}
		services = append(services, service)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list services")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"services": services,
		"limit":    limit,
		"offset":   offset,
	})
}

// Get returns a single service by ID
// GET /api/v1/services/:id
func (h *ServiceHandler) Get(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	serviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_service_id"})
		return
	}

	var service Service
	var deadline *time.Time
	err = tenantDB.QueryRowScan(c, []interface{}{
		&service.ID, &service.TenantID, &service.ClientID, &service.StaffID, &service.TypeID,
		&service.Name, &service.Period, &service.Status, &service.Priority, &service.RiskLevel,
		&deadline, &service.KanbanPosition, &service.DocsRequired, &service.DocsReceived,
		&service.HMRCReference, &service.FiledAt, &service.CompletedAt, &service.CompletionNotes,
		&service.Version, &service.CreatedAt, &service.UpdatedAt, &service.ClientName, &service.StaffName,
	}, `
		SELECT s.id, s.tenant_id, s.client_id, s.staff_id, s.type_id, s.name, s.period,
		       s.status, s.priority, s.risk_level, s.deadline, s.kanban_position,
		       s.docs_required, s.docs_received, s.hmrc_reference, s.filed_at,
		       s.completed_at, s.completion_notes, s.version, s.created_at, s.updated_at,
		       c.company_name as client_name, COALESCE(u.name, '') as staff_name
		FROM services s
		LEFT JOIN clients c ON s.client_id = c.id
		LEFT JOIN users u ON s.staff_id = u.id
		WHERE s.id = $1 AND s.tenant_id = $2
	`, serviceID, tenantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "service_not_found"})
			return
		}
		log.Error().Err(err).Msg("Failed to get service")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if deadline != nil {
		s := deadline.Format("2006-01-02")
		service.Deadline = &s
	}

	c.JSON(http.StatusOK, service)
}

// Create creates a new service
// POST /api/v1/services
func (h *ServiceHandler) Create(c *gin.Context) {
	var req CreateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	clientID, _ := uuid.Parse(req.ClientID)
	serviceID := uuid.New()

	// Parse deadline
	var deadline *time.Time
	if req.Deadline != nil && *req.Deadline != "" {
		t, err := time.Parse("2006-01-02", *req.Deadline)
		if err == nil {
			deadline = &t
		}
	}

	// Parse optional UUIDs
	var typeID, staffID *uuid.UUID
	if req.TypeID != nil && *req.TypeID != "" {
		t, _ := uuid.Parse(*req.TypeID)
		typeID = &t
	}
	if req.StaffID != nil && *req.StaffID != "" {
		s, _ := uuid.Parse(*req.StaffID)
		staffID = &s
	}

	priority := "normal"
	if req.Priority != nil {
		priority = *req.Priority
	}

	docsRequired := 0
	if req.DocsRequired != nil {
		docsRequired = *req.DocsRequired
	}

	err := tenantDB.Transaction(c, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO services (
				id, tenant_id, client_id, staff_id, type_id, name, period,
				status, priority, risk_level, deadline, docs_required
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, 'not_started', $8, $9, $10, $11
			)
		`, serviceID, tenantID, clientID, staffID, typeID, req.Name, req.Period,
			priority, req.RiskLevel, deadline, docsRequired)
		return err
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create service")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionServiceCreate, &userID, &tenantID, "service", &serviceID, c.ClientIP(), nil)

	c.JSON(http.StatusCreated, gin.H{
		"id":      serviceID,
		"message": "Service created successfully",
	})
}

// Update updates an existing service
// PATCH /api/v1/services/:id
func (h *ServiceHandler) Update(c *gin.Context) {
	serviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_service_id"})
		return
	}

	var req UpdateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	// Build dynamic update query
	var setClauses []string
	var args []interface{}
	argNum := 1

	if req.Name != nil {
		setClauses = append(setClauses, "name = $"+strconv.Itoa(argNum))
		args = append(args, *req.Name)
		argNum++
	}
	if req.Period != nil {
		setClauses = append(setClauses, "period = $"+strconv.Itoa(argNum))
		args = append(args, *req.Period)
		argNum++
	}
	if req.Status != nil {
		setClauses = append(setClauses, "status = $"+strconv.Itoa(argNum))
		args = append(args, *req.Status)
		argNum++
	}
	if req.Priority != nil {
		setClauses = append(setClauses, "priority = $"+strconv.Itoa(argNum))
		args = append(args, *req.Priority)
		argNum++
	}
	if req.RiskLevel != nil {
		setClauses = append(setClauses, "risk_level = $"+strconv.Itoa(argNum))
		args = append(args, *req.RiskLevel)
		argNum++
	}
	if req.Deadline != nil {
		var deadline *time.Time
		if *req.Deadline != "" {
			t, _ := time.Parse("2006-01-02", *req.Deadline)
			deadline = &t
		}
		setClauses = append(setClauses, "deadline = $"+strconv.Itoa(argNum))
		args = append(args, deadline)
		argNum++
	}
	if req.DocsRequired != nil {
		setClauses = append(setClauses, "docs_required = $"+strconv.Itoa(argNum))
		args = append(args, *req.DocsRequired)
		argNum++
	}
	if req.DocsReceived != nil {
		setClauses = append(setClauses, "docs_received = $"+strconv.Itoa(argNum))
		args = append(args, *req.DocsReceived)
		argNum++
	}
	if req.StaffID != nil {
		var staffID *uuid.UUID
		if *req.StaffID != "" {
			s, _ := uuid.Parse(*req.StaffID)
			staffID = &s
		}
		setClauses = append(setClauses, "staff_id = $"+strconv.Itoa(argNum))
		args = append(args, staffID)
		argNum++
	}
	if req.CompletionNotes != nil {
		setClauses = append(setClauses, "completion_notes = $"+strconv.Itoa(argNum))
		args = append(args, *req.CompletionNotes)
		argNum++
	}

	if len(setClauses) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no_fields_to_update"})
		return
	}

	// Increment version for optimistic locking
	setClauses = append(setClauses, "version = version + 1")
	setClauses = append(setClauses, "updated_at = NOW()")

	query := "UPDATE services SET " + strings.Join(setClauses, ", ") +
		" WHERE id = $" + strconv.Itoa(argNum) + " AND tenant_id = $" + strconv.Itoa(argNum+1)
	args = append(args, serviceID, tenantID)
	argNum += 2

	// Optimistic locking: If-Match header check
	// Strip W/ prefix and quotes per RFC 7232 (e.g., W/"123" or "123" -> 123)
	ifMatch := c.GetHeader("If-Match")
	ifMatch = strings.TrimPrefix(ifMatch, "W/")
	ifMatch = strings.Trim(ifMatch, `"`)
	var expectedVersion int
	hasIfMatch := false
	if ifMatch != "" {
		v, err := strconv.Atoi(ifMatch)
		if err == nil {
			hasIfMatch = true
			expectedVersion = v
			query += " AND version = $" + strconv.Itoa(argNum)
			args = append(args, expectedVersion)
		}
	}

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, query, args...)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to update service")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if rowsAffected == 0 {
		// Check if this was due to version mismatch (If-Match conflict)
		if hasIfMatch {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "version_conflict",
				"message": "Resource was modified by another request. Refresh and try again.",
			})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "service_not_found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionServiceUpdate, &userID, &tenantID, "service", &serviceID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Service updated successfully"})
}

// UpdateStatus updates just the status (for Kanban)
// PATCH /api/v1/services/:id/status
func (h *ServiceHandler) UpdateStatus(c *gin.Context) {
	serviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_service_id"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		"not_started": true, "in_progress": true, "review": true,
		"waiting": true, "completed": true, "cancelled": true,
	}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_status"})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Set completed_at if transitioning to completed
	var result pgconn.CommandTag
	if req.Status == "completed" {
		result, err = tenantDB.Exec(c, `
			UPDATE services SET status = $1, completed_at = NOW(), version = version + 1, updated_at = NOW()
			WHERE id = $2 AND tenant_id = $3
		`, req.Status, serviceID, tenantID)
	} else {
		result, err = tenantDB.Exec(c, `
			UPDATE services SET status = $1, version = version + 1, updated_at = NOW()
			WHERE id = $2 AND tenant_id = $3
		`, req.Status, serviceID, tenantID)
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to update service status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "service_not_found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionServiceUpdate, &userID, &tenantID, "service", &serviceID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Status updated successfully"})
}

// Reorder updates kanban positions in bulk
// PATCH /api/v1/services/reorder
func (h *ServiceHandler) Reorder(c *gin.Context) {
	var req struct {
		Items []struct {
			ID             string `json:"id" binding:"required,uuid"`
			KanbanPosition int    `json:"kanban_position"`
		} `json:"items" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	tenantID, _ := middleware.GetTenantID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Update each service's position
	for _, item := range req.Items {
		serviceID, _ := uuid.Parse(item.ID)
		_, err := tenantDB.Exec(c, `
			UPDATE services SET kanban_position = $1, updated_at = NOW()
			WHERE id = $2 AND tenant_id = $3
		`, item.KanbanPosition, serviceID, tenantID)
		if err != nil {
			log.Error().Err(err).Str("service_id", item.ID).Msg("Failed to reorder service")
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Services reordered successfully"})
}

// Complete marks a service as completed
// POST /api/v1/services/:id/complete
func (h *ServiceHandler) Complete(c *gin.Context) {
	serviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_service_id"})
		return
	}

	var req struct {
		Notes string `json:"notes,omitempty"`
	}
	_ = c.ShouldBindJSON(&req)

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	result, err := tenantDB.Exec(c, `
		UPDATE services SET status = 'completed', completed_at = NOW(),
		       completion_notes = $1, version = version + 1, updated_at = NOW()
		WHERE id = $2 AND tenant_id = $3
	`, req.Notes, serviceID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to complete service")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "service_not_found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionServiceComplete, &userID, &tenantID, "service", &serviceID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Service completed successfully"})
}

// GetDeadlines returns services sorted by deadline
// GET /api/v1/services/deadlines
func (h *ServiceHandler) GetDeadlines(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := c.Get(middleware.AuthRole)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit > 100 {
		limit = 100
	}

	var query strings.Builder
	var args []interface{}
	argNum := 1

	query.WriteString(`
		SELECT s.id, s.name, s.status, s.priority, s.deadline, s.client_id,
		       c.company_name as client_name
		FROM services s
		LEFT JOIN clients c ON s.client_id = c.id
		WHERE s.tenant_id = $1
		  AND s.status NOT IN ('completed', 'cancelled')
		  AND s.deadline IS NOT NULL
	`)
	args = append(args, tenantID)
	argNum++

	// Staff scoping
	roleStr, _ := role.(string)
	if roleStr == "staff" {
		query.WriteString(` AND s.client_id IN (SELECT client_id FROM staff_clients WHERE staff_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, userID)
		argNum++
	}

	query.WriteString(` ORDER BY s.deadline ASC LIMIT $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)

	var services []map[string]interface{}
	err := tenantDB.Query(c, query.String(), args, func(rows pgx.Rows) error {
		var id, clientID uuid.UUID
		var name, status, priority string
		var deadline time.Time
		var clientName *string

		if err := rows.Scan(&id, &name, &status, &priority, &deadline, &clientID, &clientName); err != nil {
			return err
		}

		services = append(services, map[string]interface{}{
			"id":          id,
			"name":        name,
			"status":      status,
			"priority":    priority,
			"deadline":    deadline.Format("2006-01-02"),
			"client_id":   clientID,
			"client_name": clientName,
		})
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get service deadlines")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"services": services})
}

// GetAlerts returns at-risk and overdue services
// GET /api/v1/services/alerts
func (h *ServiceHandler) GetAlerts(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := c.Get(middleware.AuthRole)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var query strings.Builder
	var args []interface{}
	argNum := 1

	query.WriteString(`
		SELECT s.id, s.name, s.status, s.priority, s.deadline, s.risk_level,
		       s.client_id, c.company_name as client_name,
		       CASE
		           WHEN s.deadline < CURRENT_DATE THEN 'overdue'
		           WHEN s.deadline <= CURRENT_DATE + INTERVAL '7 days' THEN 'due_soon'
		           WHEN s.risk_level = 'high' THEN 'high_risk'
		           ELSE 'normal'
		       END as alert_type
		FROM services s
		LEFT JOIN clients c ON s.client_id = c.id
		WHERE s.tenant_id = $1
		  AND s.status NOT IN ('completed', 'cancelled')
		  AND (
		      s.deadline < CURRENT_DATE
		      OR s.deadline <= CURRENT_DATE + INTERVAL '7 days'
		      OR s.risk_level = 'high'
		  )
	`)
	args = append(args, tenantID)
	argNum++

	// Staff scoping
	roleStr, _ := role.(string)
	if roleStr == "staff" {
		query.WriteString(` AND s.client_id IN (SELECT client_id FROM staff_clients WHERE staff_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, userID)
	}

	query.WriteString(` ORDER BY s.deadline ASC NULLS LAST LIMIT 100`)

	var overdue, dueSoon, highRisk []map[string]interface{}
	err := tenantDB.Query(c, query.String(), args, func(rows pgx.Rows) error {
		var id, clientID uuid.UUID
		var name, status, priority string
		var deadline *time.Time
		var riskLevel, clientName *string
		var alertType string

		if err := rows.Scan(&id, &name, &status, &priority, &deadline, &riskLevel, &clientID, &clientName, &alertType); err != nil {
			return err
		}

		item := map[string]interface{}{
			"id":          id,
			"name":        name,
			"status":      status,
			"priority":    priority,
			"risk_level":  riskLevel,
			"client_id":   clientID,
			"client_name": clientName,
		}
		if deadline != nil {
			item["deadline"] = deadline.Format("2006-01-02")
		}

		switch alertType {
		case "overdue":
			overdue = append(overdue, item)
		case "due_soon":
			dueSoon = append(dueSoon, item)
		case "high_risk":
			highRisk = append(highRisk, item)
		}
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get service alerts")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"overdue":   overdue,
		"due_soon":  dueSoon,
		"high_risk": highRisk,
	})
}

// Delete cancels a service
// DELETE /api/v1/services/:id
func (h *ServiceHandler) Delete(c *gin.Context) {
	serviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_service_id"})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := middleware.GetRole(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		var sql string
		var args []interface{}
		if role == "super_admin" {
			sql = `
				UPDATE services SET status = 'cancelled', version = version + 1, updated_at = NOW()
				WHERE id = $1
			`
			args = []interface{}{serviceID}
		} else {
			sql = `
				UPDATE services SET status = 'cancelled', version = version + 1, updated_at = NOW()
				WHERE id = $1 AND tenant_id = $2
			`
			args = []interface{}{serviceID, tenantID}
		}
		result, err := tx.Exec(ctx, sql, args...)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to cancel service")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "service_not_found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionServiceDelete, &userID, &tenantID, "service", &serviceID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Service cancelled successfully"})
}

// MarkHMRC marks a service with an HMRC reference
// POST /api/v1/services/:id/hmrc-mark
func (h *ServiceHandler) MarkHMRC(c *gin.Context) {
	serviceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_service_id"})
		return
	}

	var req struct {
		Reference string `json:"reference" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	tenantID, _ := middleware.GetTenantID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	result, err := tenantDB.Exec(c, `
		UPDATE services SET hmrc_reference = $1, filed_at = NOW(), version = version + 1, updated_at = NOW()
		WHERE id = $2 AND tenant_id = $3
	`, req.Reference, serviceID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to mark HMRC reference")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "service_not_found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "HMRC reference marked successfully"})
}

// BulkUpdate updates multiple services at once
// POST /api/v1/services/bulk-update
func (h *ServiceHandler) BulkUpdate(c *gin.Context) {
	var req struct {
		ServiceIDs []string `json:"service_ids" binding:"required"`
		Status     *string  `json:"status,omitempty"`
		Priority   *string  `json:"priority,omitempty"`
		StaffID    *string  `json:"staff_id,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Build update query
	var setClauses []string
	var args []interface{}
	argNum := 1

	if req.Status != nil {
		setClauses = append(setClauses, "status = $"+strconv.Itoa(argNum))
		args = append(args, *req.Status)
		argNum++
	}
	if req.Priority != nil {
		setClauses = append(setClauses, "priority = $"+strconv.Itoa(argNum))
		args = append(args, *req.Priority)
		argNum++
	}
	if req.StaffID != nil {
		var staffID *uuid.UUID
		if *req.StaffID != "" {
			s, _ := uuid.Parse(*req.StaffID)
			staffID = &s
		}
		setClauses = append(setClauses, "staff_id = $"+strconv.Itoa(argNum))
		args = append(args, staffID)
		argNum++
	}

	if len(setClauses) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no_fields_to_update"})
		return
	}

	setClauses = append(setClauses, "version = version + 1")
	setClauses = append(setClauses, "updated_at = NOW()")

	// Convert string IDs to UUIDs
	var serviceIDs []uuid.UUID
	for _, id := range req.ServiceIDs {
		if uid, err := uuid.Parse(id); err == nil {
			serviceIDs = append(serviceIDs, uid)
		}
	}

	query := "UPDATE services SET " + strings.Join(setClauses, ", ") +
		" WHERE id = ANY($" + strconv.Itoa(argNum) + ") AND tenant_id = $" + strconv.Itoa(argNum+1)
	args = append(args, serviceIDs, tenantID)

	result, err := tenantDB.Exec(c, query, args...)
	if err != nil {
		log.Error().Err(err).Msg("Failed to bulk update services")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionServiceUpdate, &userID, &tenantID, "service", nil, c.ClientIP(), map[string]interface{}{
		"count":       result.RowsAffected(),
		"service_ids": req.ServiceIDs,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Services updated successfully",
		"updated": result.RowsAffected(),
	})
}
