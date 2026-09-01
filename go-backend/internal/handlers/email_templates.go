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

// EmailTemplateHandler handles email template operations.
type EmailTemplateHandler struct {
	db    *database.Pool
	audit *audit.Logger
}

// NewEmailTemplateHandler creates a new email template handler.
func NewEmailTemplateHandler(db *database.Pool, auditLogger *audit.Logger) *EmailTemplateHandler {
	return &EmailTemplateHandler{
		db:    db,
		audit: auditLogger,
	}
}

// EmailTemplate represents an email template.
type EmailTemplate struct {
	ID           uuid.UUID  `json:"id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	Name         string     `json:"name"`
	Subject      string     `json:"subject"`
	BodyHTML     string     `json:"body_html"`
	BodyText     *string    `json:"body_text,omitempty"`
	Type         string     `json:"type"` // chase, notification, welcome, custom
	Category     *string    `json:"category,omitempty"`
	Placeholders []string   `json:"placeholders,omitempty"`
	IsDefault    bool       `json:"is_default"`
	IsActive     bool       `json:"is_active"`
	CreatedBy    *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// CreateEmailTemplateRequest represents the request to create an email template.
type CreateEmailTemplateRequest struct {
	Name         string   `json:"name" binding:"required"`
	Subject      string   `json:"subject" binding:"required"`
	BodyHTML     string   `json:"body_html" binding:"required"`
	BodyText     *string  `json:"body_text,omitempty"`
	Type         string   `json:"type" binding:"required,oneof=chase notification welcome custom"`
	Category     *string  `json:"category,omitempty"`
	Placeholders []string `json:"placeholders,omitempty"`
	IsDefault    *bool    `json:"is_default,omitempty"`
}

// UpdateEmailTemplateRequest represents the request to update an email template.
type UpdateEmailTemplateRequest struct {
	Name         *string  `json:"name,omitempty"`
	Subject      *string  `json:"subject,omitempty"`
	BodyHTML     *string  `json:"body_html,omitempty"`
	BodyText     *string  `json:"body_text,omitempty"`
	Type         *string  `json:"type,omitempty"`
	Category     *string  `json:"category,omitempty"`
	Placeholders []string `json:"placeholders,omitempty"`
	IsDefault    *bool    `json:"is_default,omitempty"`
	IsActive     *bool    `json:"is_active,omitempty"`
}

// List returns all email templates for the tenant.
// GET /api/v1/email-templates
func (h *EmailTemplateHandler) List(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

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
	templateType := c.Query("type")
	category := c.Query("category")
	activeOnly := c.DefaultQuery("active", "true") == "true"
	search := c.Query("search")

	var query strings.Builder
	var args []interface{}
	argNum := 1

	query.WriteString(`
		SELECT id, tenant_id, name, subject, body_html, body_text,
		       type, category, placeholders, is_default, is_active,
		       created_by, created_at, updated_at
		FROM email_templates
		WHERE tenant_id = $1
	`)
	args = append(args, tenantID)
	argNum++

	if activeOnly {
		query.WriteString(` AND is_active = true`)
	}

	if templateType != "" {
		query.WriteString(` AND type = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, templateType)
		argNum++
	}

	if category != "" {
		query.WriteString(` AND category = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, category)
		argNum++
	}

	if search != "" {
		query.WriteString(` AND (name ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(` OR subject ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, "%"+search+"%")
		argNum++
	}

	query.WriteString(` ORDER BY is_default DESC, name ASC LIMIT $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)
	argNum++

	query.WriteString(` OFFSET $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, offset)

	var templates []EmailTemplate
	err := tenantDB.Query(c, query.String(), args, func(rows pgx.Rows) error {
		var t EmailTemplate
		err := rows.Scan(
			&t.ID, &t.TenantID, &t.Name, &t.Subject, &t.BodyHTML, &t.BodyText,
			&t.Type, &t.Category, &t.Placeholders, &t.IsDefault, &t.IsActive,
			&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return err
		}
		templates = append(templates, t)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list email templates")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch email templates"})
		return
	}

	if templates == nil {
		templates = []EmailTemplate{}
	}

	c.JSON(http.StatusOK, gin.H{
		"email_templates": templates,
		"count":           len(templates),
	})
}

// Get returns a single email template.
// GET /api/v1/email-templates/:id
func (h *EmailTemplateHandler) Get(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	templateID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	query := `
		SELECT id, tenant_id, name, subject, body_html, body_text,
		       type, category, placeholders, is_default, is_active,
		       created_by, created_at, updated_at
		FROM email_templates
		WHERE id = $1 AND tenant_id = $2
	`

	var t EmailTemplate
	err = tenantDB.QueryRowScan(c, []interface{}{
		&t.ID, &t.TenantID, &t.Name, &t.Subject, &t.BodyHTML, &t.BodyText,
		&t.Type, &t.Category, &t.Placeholders, &t.IsDefault, &t.IsActive,
		&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
	}, query, templateID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email template not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get email template")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch email template"})
		return
	}

	c.JSON(http.StatusOK, t)
}

// Create creates a new email template.
// POST /api/v1/email-templates
func (h *EmailTemplateHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req CreateEmailTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default values
	isDefault := false
	if req.IsDefault != nil {
		isDefault = *req.IsDefault
	}

	// Default placeholders if not provided
	placeholders := req.Placeholders
	if placeholders == nil {
		placeholders = []string{}
	}

	id := uuid.New()
	var t EmailTemplate

	err := tenantDB.Transaction(c, func(tx pgx.Tx) error {
		// If this is being set as default, unset other defaults of same type
		if isDefault {
			_, err := tx.Exec(ctx, `
				UPDATE email_templates SET is_default = false
				WHERE tenant_id = $1 AND type = $2 AND is_default = true
			`, tenantID, req.Type)
			if err != nil {
				return err
			}
		}

		query := `
			INSERT INTO email_templates (
				id, tenant_id, name, subject, body_html, body_text,
				type, category, placeholders, is_default, is_active,
				created_by, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, true, $11, NOW(), NOW())
			RETURNING id, tenant_id, name, subject, body_html, body_text,
			          type, category, placeholders, is_default, is_active,
			          created_by, created_at, updated_at
		`
		return tx.QueryRow(ctx, query,
			id, tenantID, req.Name, req.Subject, req.BodyHTML, req.BodyText,
			req.Type, req.Category, placeholders, isDefault, userID,
		).Scan(
			&t.ID, &t.TenantID, &t.Name, &t.Subject, &t.BodyHTML, &t.BodyText,
			&t.Type, &t.Category, &t.Placeholders, &t.IsDefault, &t.IsActive,
			&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
		)
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create email template")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create email template"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionEmailTemplateCreate, &userID, &tenantID, "email_template", &t.ID, c.ClientIP(), map[string]interface{}{
		"name": req.Name,
		"type": req.Type,
	})

	c.JSON(http.StatusCreated, t)
}

// Update updates an email template.
// PATCH /api/v1/email-templates/:id
func (h *EmailTemplateHandler) Update(c *gin.Context) {
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

	templateID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	var req UpdateEmailTemplateRequest
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
	if req.Subject != nil {
		updates = append(updates, "subject = $"+strconv.Itoa(argNum))
		args = append(args, *req.Subject)
		argNum++
	}
	if req.BodyHTML != nil {
		updates = append(updates, "body_html = $"+strconv.Itoa(argNum))
		args = append(args, *req.BodyHTML)
		argNum++
	}
	if req.BodyText != nil {
		updates = append(updates, "body_text = $"+strconv.Itoa(argNum))
		args = append(args, *req.BodyText)
		argNum++
	}
	if req.Type != nil {
		updates = append(updates, "type = $"+strconv.Itoa(argNum))
		args = append(args, *req.Type)
		argNum++
	}
	if req.Category != nil {
		updates = append(updates, "category = $"+strconv.Itoa(argNum))
		args = append(args, *req.Category)
		argNum++
	}
	if req.Placeholders != nil {
		updates = append(updates, "placeholders = $"+strconv.Itoa(argNum))
		args = append(args, req.Placeholders)
		argNum++
	}
	if req.IsDefault != nil {
		updates = append(updates, "is_default = $"+strconv.Itoa(argNum))
		args = append(args, *req.IsDefault)
		argNum++
	}
	if req.IsActive != nil {
		updates = append(updates, "is_active = $"+strconv.Itoa(argNum))
		args = append(args, *req.IsActive)
		argNum++
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	updates = append(updates, "updated_at = NOW()")

	query := "UPDATE email_templates SET " + strings.Join(updates, ", ") +
		" WHERE id = $" + strconv.Itoa(argNum) + " AND tenant_id = $" + strconv.Itoa(argNum+1) +
		" RETURNING id, tenant_id, name, subject, body_html, body_text, " +
		"type, category, placeholders, is_default, is_active, created_by, created_at, updated_at"

	args = append(args, templateID, tenantID)

	var t EmailTemplate
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		// If setting as default, unset others first
		if req.IsDefault != nil && *req.IsDefault {
			// Get the current template type
			var templateType string
			err := tx.QueryRow(ctx, `SELECT type FROM email_templates WHERE id = $1 AND tenant_id = $2`, templateID, tenantID).Scan(&templateType)
			if err != nil {
				return err
			}

			_, err = tx.Exec(ctx, `
				UPDATE email_templates SET is_default = false
				WHERE tenant_id = $1 AND type = $2 AND is_default = true AND id != $3
			`, tenantID, templateType, templateID)
			if err != nil {
				return err
			}
		}

		return tx.QueryRow(ctx, query, args...).Scan(
			&t.ID, &t.TenantID, &t.Name, &t.Subject, &t.BodyHTML, &t.BodyText,
			&t.Type, &t.Category, &t.Placeholders, &t.IsDefault, &t.IsActive,
			&t.CreatedBy, &t.CreatedAt, &t.UpdatedAt,
		)
	})

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email template not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to update email template")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update email template"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionEmailTemplateUpdate, &userID, &tenantID, "email_template", &templateID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, t)
}

// Delete deletes an email template.
// DELETE /api/v1/email-templates/:id
func (h *EmailTemplateHandler) Delete(c *gin.Context) {
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

	templateID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		// Check if template is in use by any emails
		var emailCount int
		err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM emails WHERE template_id = $1
		`, templateID).Scan(&emailCount)
		if err != nil {
			return err
		}

		if emailCount > 0 {
			// Soft delete - just deactivate
			result, err := tx.Exec(ctx, `
				UPDATE email_templates SET is_active = false, updated_at = NOW()
				WHERE id = $1 AND tenant_id = $2
			`, templateID, tenantID)
			if err != nil {
				return err
			}
			rowsAffected = result.RowsAffected()
			return nil
		}

		// Hard delete - not in use
		result, err := tx.Exec(ctx, `
			DELETE FROM email_templates WHERE id = $1 AND tenant_id = $2
		`, templateID, tenantID)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to delete email template")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete email template"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email template not found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionEmailTemplateDelete, &userID, &tenantID, "email_template", &templateID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Email template deleted"})
}
