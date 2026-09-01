package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/audit"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

type ServiceTypeHandler struct {
	db    *database.Pool
	audit *audit.Logger
}

func NewServiceTypeHandler(db *database.Pool, auditLogger *audit.Logger) *ServiceTypeHandler {
	return &ServiceTypeHandler{
		db:    db,
		audit: auditLogger,
	}
}

// ServiceType represents a service type template (e.g., Tax Return, VAT Return)
type ServiceType struct {
	ID                 uuid.UUID  `json:"id"`
	TenantID           uuid.UUID  `json:"tenant_id"`
	Name               string     `json:"name"`
	Category           string     `json:"category"`
	Description        *string    `json:"description,omitempty"`
	DefaultPriority    string     `json:"default_priority"`
	DefaultDeadlineDays *int      `json:"default_deadline_days,omitempty"`
	RequiredDocs       []string   `json:"required_docs,omitempty"`
	ChecklistTemplate  []string   `json:"checklist_template,omitempty"`
	IsRecurring        bool       `json:"is_recurring"`
	RecurrencePattern  *string    `json:"recurrence_pattern,omitempty"`
	HMRCRelevant       bool       `json:"hmrc_relevant"`
	IsActive           bool       `json:"is_active"`
	SortOrder          int        `json:"sort_order"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	// Computed fields
	ServiceCount       int        `json:"service_count,omitempty"`
}

type CreateServiceTypeRequest struct {
	Name               string   `json:"name" binding:"required"`
	Category           string   `json:"category" binding:"required"`
	Description        *string  `json:"description,omitempty"`
	DefaultPriority    *string  `json:"default_priority,omitempty"`
	DefaultDeadlineDays *int    `json:"default_deadline_days,omitempty"`
	RequiredDocs       []string `json:"required_docs,omitempty"`
	ChecklistTemplate  []string `json:"checklist_template,omitempty"`
	IsRecurring        *bool    `json:"is_recurring,omitempty"`
	RecurrencePattern  *string  `json:"recurrence_pattern,omitempty"`
	HMRCRelevant       *bool    `json:"hmrc_relevant,omitempty"`
}

type UpdateServiceTypeRequest struct {
	Name               *string  `json:"name,omitempty"`
	Category           *string  `json:"category,omitempty"`
	Description        *string  `json:"description,omitempty"`
	DefaultPriority    *string  `json:"default_priority,omitempty"`
	DefaultDeadlineDays *int    `json:"default_deadline_days,omitempty"`
	RequiredDocs       []string `json:"required_docs,omitempty"`
	ChecklistTemplate  []string `json:"checklist_template,omitempty"`
	IsRecurring        *bool    `json:"is_recurring,omitempty"`
	RecurrencePattern  *string  `json:"recurrence_pattern,omitempty"`
	HMRCRelevant       *bool    `json:"hmrc_relevant,omitempty"`
	IsActive           *bool    `json:"is_active,omitempty"`
	SortOrder          *int     `json:"sort_order,omitempty"`
}

// List returns all service types for the tenant
// GET /api/v1/service-types
func (h *ServiceTypeHandler) List(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

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
	category := c.Query("category")
	activeOnly := c.DefaultQuery("active", "true") == "true"
	search := c.Query("search")

	var query strings.Builder
	var args []interface{}
	argNum := 1

	query.WriteString(`
		SELECT st.id, st.tenant_id, st.name, st.category, st.description,
		       st.default_priority, st.default_deadline_days, st.required_docs,
		       st.checklist_template, st.is_recurring, st.recurrence_pattern,
		       st.hmrc_relevant, st.is_active, st.sort_order,
		       st.created_at, st.updated_at,
		       COALESCE((SELECT COUNT(*) FROM services s WHERE s.type_id = st.id), 0) as service_count
		FROM service_types st
		WHERE st.tenant_id = $1
	`)
	args = append(args, tenantID)
	argNum++

	if activeOnly {
		query.WriteString(` AND st.is_active = true`)
	}

	if category != "" {
		query.WriteString(` AND st.category = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, category)
		argNum++
	}

	if search != "" {
		query.WriteString(` AND (st.name ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(` OR st.description ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, "%"+search+"%")
		argNum++
	}

	query.WriteString(` ORDER BY st.sort_order ASC, st.name ASC LIMIT $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)
	argNum++

	query.WriteString(` OFFSET $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, offset)

	var serviceTypes []ServiceType
	err := tenantDB.Query(c, query.String(), args, func(rows pgx.Rows) error {
		var st ServiceType
		err := rows.Scan(
			&st.ID, &st.TenantID, &st.Name, &st.Category, &st.Description,
			&st.DefaultPriority, &st.DefaultDeadlineDays, &st.RequiredDocs,
			&st.ChecklistTemplate, &st.IsRecurring, &st.RecurrencePattern,
			&st.HMRCRelevant, &st.IsActive, &st.SortOrder,
			&st.CreatedAt, &st.UpdatedAt, &st.ServiceCount,
		)
		if err != nil {
			return err
		}
		serviceTypes = append(serviceTypes, st)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list service types")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch service types"})
		return
	}

	if serviceTypes == nil {
		serviceTypes = []ServiceType{}
	}

	c.JSON(http.StatusOK, gin.H{
		"service_types": serviceTypes,
		"count":         len(serviceTypes),
	})
}

// Get returns a single service type
// GET /api/v1/service-types/:id
func (h *ServiceTypeHandler) Get(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	stID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service type ID"})
		return
	}

	query := `
		SELECT st.id, st.tenant_id, st.name, st.category, st.description,
		       st.default_priority, st.default_deadline_days, st.required_docs,
		       st.checklist_template, st.is_recurring, st.recurrence_pattern,
		       st.hmrc_relevant, st.is_active, st.sort_order,
		       st.created_at, st.updated_at,
		       COALESCE((SELECT COUNT(*) FROM services s WHERE s.type_id = st.id), 0) as service_count
		FROM service_types st
		WHERE st.id = $1 AND st.tenant_id = $2
	`

	var st ServiceType
	err = tenantDB.QueryRowScan(c, []interface{}{
		&st.ID, &st.TenantID, &st.Name, &st.Category, &st.Description,
		&st.DefaultPriority, &st.DefaultDeadlineDays, &st.RequiredDocs,
		&st.ChecklistTemplate, &st.IsRecurring, &st.RecurrencePattern,
		&st.HMRCRelevant, &st.IsActive, &st.SortOrder,
		&st.CreatedAt, &st.UpdatedAt, &st.ServiceCount,
	}, query, stID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service type not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get service type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch service type"})
		return
	}

	c.JSON(http.StatusOK, st)
}

// Create creates a new service type
// POST /api/v1/service-types
func (h *ServiceTypeHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	// Get tenant-scoped DB for RLS enforcement
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req CreateServiceTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default values
	defaultPriority := "normal"
	if req.DefaultPriority != nil {
		defaultPriority = *req.DefaultPriority
	}

	isRecurring := false
	if req.IsRecurring != nil {
		isRecurring = *req.IsRecurring
	}

	hmrcRelevant := false
	if req.HMRCRelevant != nil {
		hmrcRelevant = *req.HMRCRelevant
	}

	id := uuid.New()
	var st ServiceType
	st.ID = id
	st.TenantID = tenantID
	st.Name = req.Name
	st.Category = req.Category
	st.Description = req.Description
	st.DefaultPriority = defaultPriority
	st.DefaultDeadlineDays = req.DefaultDeadlineDays
	st.RequiredDocs = req.RequiredDocs
	st.ChecklistTemplate = req.ChecklistTemplate
	st.IsRecurring = isRecurring
	st.RecurrencePattern = req.RecurrencePattern
	st.HMRCRelevant = hmrcRelevant
	st.IsActive = true

	err := tenantDB.Transaction(c, func(tx pgx.Tx) error {
		// Get max sort order
		var maxOrder int
		err := tx.QueryRow(ctx, `
			SELECT COALESCE(MAX(sort_order), 0) FROM service_types WHERE tenant_id = $1
		`, tenantID).Scan(&maxOrder)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get max sort order")
		}
		st.SortOrder = maxOrder + 1

		query := `
			INSERT INTO service_types (
				id, tenant_id, name, category, description, default_priority,
				default_deadline_days, required_docs, checklist_template,
				is_recurring, recurrence_pattern, hmrc_relevant, is_active, sort_order,
				created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, true, $13, NOW(), NOW())
			RETURNING id, created_at, updated_at
		`
		return tx.QueryRow(ctx, query,
			id, tenantID, req.Name, req.Category, req.Description, defaultPriority,
			req.DefaultDeadlineDays, req.RequiredDocs, req.ChecklistTemplate,
			isRecurring, req.RecurrencePattern, hmrcRelevant, st.SortOrder,
		).Scan(&st.ID, &st.CreatedAt, &st.UpdatedAt)
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create service type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create service type"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionServiceTypeCreate, &userID, &tenantID, "service_type", &st.ID, c.ClientIP(), map[string]interface{}{
		"name":     req.Name,
		"category": req.Category,
	})

	c.JSON(http.StatusCreated, st)
}

// Update updates a service type
// PATCH /api/v1/service-types/:id
func (h *ServiceTypeHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")

	// Get tenant-scoped DB for RLS enforcement
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	stID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service type ID"})
		return
	}

	var req UpdateServiceTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build dynamic update query
	var updates []string
	var args []interface{}
	argNum := 1

	if req.Name != nil {
		updates = append(updates, "name = $"+strconv.Itoa(argNum))
		args = append(args, *req.Name)
		argNum++
	}
	if req.Category != nil {
		updates = append(updates, "category = $"+strconv.Itoa(argNum))
		args = append(args, *req.Category)
		argNum++
	}
	if req.Description != nil {
		updates = append(updates, "description = $"+strconv.Itoa(argNum))
		args = append(args, *req.Description)
		argNum++
	}
	if req.DefaultPriority != nil {
		updates = append(updates, "default_priority = $"+strconv.Itoa(argNum))
		args = append(args, *req.DefaultPriority)
		argNum++
	}
	if req.DefaultDeadlineDays != nil {
		updates = append(updates, "default_deadline_days = $"+strconv.Itoa(argNum))
		args = append(args, *req.DefaultDeadlineDays)
		argNum++
	}
	if req.RequiredDocs != nil {
		updates = append(updates, "required_docs = $"+strconv.Itoa(argNum))
		args = append(args, req.RequiredDocs)
		argNum++
	}
	if req.ChecklistTemplate != nil {
		updates = append(updates, "checklist_template = $"+strconv.Itoa(argNum))
		args = append(args, req.ChecklistTemplate)
		argNum++
	}
	if req.IsRecurring != nil {
		updates = append(updates, "is_recurring = $"+strconv.Itoa(argNum))
		args = append(args, *req.IsRecurring)
		argNum++
	}
	if req.RecurrencePattern != nil {
		updates = append(updates, "recurrence_pattern = $"+strconv.Itoa(argNum))
		args = append(args, *req.RecurrencePattern)
		argNum++
	}
	if req.HMRCRelevant != nil {
		updates = append(updates, "hmrc_relevant = $"+strconv.Itoa(argNum))
		args = append(args, *req.HMRCRelevant)
		argNum++
	}
	if req.IsActive != nil {
		updates = append(updates, "is_active = $"+strconv.Itoa(argNum))
		args = append(args, *req.IsActive)
		argNum++
	}
	if req.SortOrder != nil {
		updates = append(updates, "sort_order = $"+strconv.Itoa(argNum))
		args = append(args, *req.SortOrder)
		argNum++
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	updates = append(updates, "updated_at = NOW()")

	query := "UPDATE service_types SET " + strings.Join(updates, ", ") +
		" WHERE id = $" + strconv.Itoa(argNum) + " AND tenant_id = $" + strconv.Itoa(argNum+1) +
		" RETURNING id, tenant_id, name, category, description, default_priority, " +
		"default_deadline_days, required_docs, checklist_template, is_recurring, " +
		"recurrence_pattern, hmrc_relevant, is_active, sort_order, created_at, updated_at"

	args = append(args, stID, tenantID)

	var st ServiceType
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, query, args...).Scan(
			&st.ID, &st.TenantID, &st.Name, &st.Category, &st.Description,
			&st.DefaultPriority, &st.DefaultDeadlineDays, &st.RequiredDocs,
			&st.ChecklistTemplate, &st.IsRecurring, &st.RecurrencePattern,
			&st.HMRCRelevant, &st.IsActive, &st.SortOrder,
			&st.CreatedAt, &st.UpdatedAt,
		)
	})

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service type not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to update service type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update service type"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionServiceTypeUpdate, &userID, &tenantID, "service_type", &stID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, st)
}

// Delete deletes a service type (soft delete by setting is_active = false)
// DELETE /api/v1/service-types/:id
func (h *ServiceTypeHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")

	// Get tenant-scoped DB for RLS enforcement
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	stID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service type ID"})
		return
	}

	var serviceCount int
	var rowsAffected int64
	var softDelete bool

	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		// Check if any services are using this type
		err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM services WHERE type_id = $1 AND tenant_id = $2
		`, stID, tenantID).Scan(&serviceCount)
		if err != nil {
			return err
		}

		if serviceCount > 0 {
			// Soft delete - just deactivate
			softDelete = true
			_, err = tx.Exec(ctx, `
				UPDATE service_types SET is_active = false, updated_at = NOW()
				WHERE id = $1 AND tenant_id = $2
			`, stID, tenantID)
			return err
		}

		// Hard delete - no services using it
		result, err := tx.Exec(ctx, `
			DELETE FROM service_types WHERE id = $1 AND tenant_id = $2
		`, stID, tenantID)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to delete service type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete service type"})
		return
	}

	if !softDelete && rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service type not found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionServiceTypeDelete, &userID, &tenantID, "service_type", &stID, c.ClientIP(), map[string]interface{}{
		"soft_delete": softDelete,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Service type deleted"})
}

// GetCategories returns distinct categories for service types
// GET /api/v1/service-types/categories
func (h *ServiceTypeHandler) GetCategories(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	query := `
		SELECT DISTINCT category FROM service_types
		WHERE tenant_id = $1 AND is_active = true
		ORDER BY category ASC
	`

	var categories []string
	err := tenantDB.Query(c, query, []interface{}{tenantID}, func(rows pgx.Rows) error {
		var cat string
		if err := rows.Scan(&cat); err != nil {
			return err
		}
		categories = append(categories, cat)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get categories")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	if categories == nil {
		categories = []string{}
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// Reorder updates the sort order of service types
// PATCH /api/v1/service-types/reorder
func (h *ServiceTypeHandler) Reorder(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req struct {
		Items []struct {
			ID        string `json:"id" binding:"required,uuid"`
			SortOrder int    `json:"sort_order" binding:"required"`
		} `json:"items" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update each item's sort order
	for _, item := range req.Items {
		stID, _ := uuid.Parse(item.ID)
		_, err := tenantDB.Exec(c, `
			UPDATE service_types SET sort_order = $1, updated_at = NOW()
			WHERE id = $2 AND tenant_id = $3
		`, item.SortOrder, stID, tenantID)
		if err != nil {
			log.Error().Err(err).Str("id", item.ID).Msg("Failed to reorder service type")
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Service types reordered"})
}

// ServiceRequirement represents a document requirement for a service type
type ServiceRequirement struct {
	ID             uuid.UUID `json:"id"`
	TenantID       uuid.UUID `json:"tenant_id"`
	ServiceTypeID  uuid.UUID `json:"service_type_id"`
	DocumentTypeID uuid.UUID `json:"document_type_id"`
	IsMandatory    bool      `json:"is_mandatory"`
	CreatedAt      time.Time `json:"created_at"`
	// Joined fields
	DocumentTypeName *string `json:"document_type_name,omitempty"`
}

// GetRequirements returns document requirements for a service type
// GET /api/v1/service-types/:id/requirements
func (h *ServiceTypeHandler) GetRequirements(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	stID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service type ID"})
		return
	}

	var requirements []ServiceRequirement
	err = tenantDB.Query(c, `
		SELECT sr.id, sr.tenant_id, sr.service_type_id, sr.document_type_id,
		       sr.is_mandatory, sr.created_at, dt.name as document_type_name
		FROM service_requirements sr
		LEFT JOIN document_types dt ON sr.document_type_id = dt.id
		WHERE sr.service_type_id = $1 AND sr.tenant_id = $2
		ORDER BY dt.name ASC
	`, []interface{}{stID, tenantID}, func(rows pgx.Rows) error {
		var req ServiceRequirement
		if err := rows.Scan(&req.ID, &req.TenantID, &req.ServiceTypeID, &req.DocumentTypeID,
			&req.IsMandatory, &req.CreatedAt, &req.DocumentTypeName); err != nil {
			return err
		}
		requirements = append(requirements, req)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get service requirements")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if requirements == nil {
		requirements = []ServiceRequirement{}
	}

	c.JSON(http.StatusOK, gin.H{"requirements": requirements})
}

// AddRequirement adds a document requirement to a service type
// POST /api/v1/service-types/:id/requirements
func (h *ServiceTypeHandler) AddRequirement(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	stID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service type ID"})
		return
	}

	var req struct {
		DocumentTypeID string `json:"document_type_id" binding:"required,uuid"`
		IsMandatory    *bool  `json:"is_mandatory,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	docTypeID, _ := uuid.Parse(req.DocumentTypeID)
	isMandatory := true
	if req.IsMandatory != nil {
		isMandatory = *req.IsMandatory
	}

	reqID := uuid.New()
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO service_requirements (id, tenant_id, service_type_id, document_type_id, is_mandatory)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (service_type_id, document_type_id) DO UPDATE SET is_mandatory = $5
		`, reqID, tenantID, stID, docTypeID, isMandatory)
		return err
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to add service requirement")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	h.audit.LogEntity(ctx, audit.ActionServiceTypeUpdate, &userID, &tenantID, "service_requirement", &reqID, c.ClientIP(), nil)

	c.JSON(http.StatusCreated, gin.H{
		"id":      reqID,
		"message": "Requirement added successfully",
	})
}

// RemoveRequirement removes a document requirement from a service type
// DELETE /api/v1/service-types/:id/requirements/:docTypeId
func (h *ServiceTypeHandler) RemoveRequirement(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")
	docTypeIDStr := c.Param("docTypeId")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	stID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service type ID"})
		return
	}

	docTypeID, err := uuid.Parse(docTypeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document type ID"})
		return
	}

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			DELETE FROM service_requirements
			WHERE service_type_id = $1 AND document_type_id = $2 AND tenant_id = $3
		`, stID, docTypeID, tenantID)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to remove service requirement")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "requirement_not_found"})
		return
	}

	h.audit.LogEntity(ctx, audit.ActionServiceTypeUpdate, &userID, &tenantID, "service_requirement", nil, c.ClientIP(), map[string]interface{}{
		"service_type_id":  stID,
		"document_type_id": docTypeID,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Requirement removed successfully"})
}

// Clone creates a copy of an existing service type
// POST /api/v1/service-types/:id/clone
func (h *ServiceTypeHandler) Clone(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	stID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid service type ID"})
		return
	}

	// Get original service type
	var original ServiceType
	err = tenantDB.QueryRowScan(c, []interface{}{
		&original.ID, &original.TenantID, &original.Name, &original.Category,
		&original.Description, &original.DefaultPriority, &original.DefaultDeadlineDays,
		&original.RequiredDocs, &original.ChecklistTemplate,
		&original.IsRecurring, &original.RecurrencePattern, &original.HMRCRelevant,
	}, `
		SELECT id, tenant_id, name, category, description, default_priority,
		       default_deadline_days, required_docs, checklist_template,
		       is_recurring, recurrence_pattern, hmrc_relevant
		FROM service_types WHERE id = $1 AND tenant_id = $2
	`, stID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Service type not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get service type for cloning")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clone service type"})
		return
	}

	// Create clone within transaction
	newID := uuid.New()
	newName := original.Name + " (Copy)"

	var cloned ServiceType
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		// Get max sort order
		var maxOrder int
		_ = tx.QueryRow(ctx, `
			SELECT COALESCE(MAX(sort_order), 0) FROM service_types WHERE tenant_id = $1
		`, tenantID).Scan(&maxOrder)

		return tx.QueryRow(ctx, `
			INSERT INTO service_types (
				id, tenant_id, name, category, description, default_priority,
				default_deadline_days, required_docs, checklist_template,
				is_recurring, recurrence_pattern, hmrc_relevant, is_active, sort_order,
				created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, true, $13, NOW(), NOW())
			RETURNING id, tenant_id, name, category, description, default_priority,
			          default_deadline_days, required_docs, checklist_template,
			          is_recurring, recurrence_pattern, hmrc_relevant, is_active, sort_order,
			          created_at, updated_at
		`, newID, tenantID, newName, original.Category, original.Description,
			original.DefaultPriority, original.DefaultDeadlineDays, original.RequiredDocs,
			original.ChecklistTemplate, original.IsRecurring, original.RecurrencePattern,
			original.HMRCRelevant, maxOrder+1,
		).Scan(
			&cloned.ID, &cloned.TenantID, &cloned.Name, &cloned.Category,
			&cloned.Description, &cloned.DefaultPriority, &cloned.DefaultDeadlineDays,
			&cloned.RequiredDocs, &cloned.ChecklistTemplate,
			&cloned.IsRecurring, &cloned.RecurrencePattern, &cloned.HMRCRelevant,
			&cloned.IsActive, &cloned.SortOrder, &cloned.CreatedAt, &cloned.UpdatedAt,
		)
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to clone service type")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clone service type"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionServiceTypeCreate, &userID, &tenantID, "service_type", &cloned.ID, c.ClientIP(), map[string]interface{}{
		"cloned_from": stID.String(),
	})

	c.JSON(http.StatusCreated, cloned)
}
