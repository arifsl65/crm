package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/audit"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// ReminderHandler handles reminder operations.
type ReminderHandler struct {
	db    *database.Pool
	audit *audit.Logger
}

// NewReminderHandler creates a new reminder handler.
func NewReminderHandler(db *database.Pool, auditLogger *audit.Logger) *ReminderHandler {
	return &ReminderHandler{
		db:    db,
		audit: auditLogger,
	}
}

// Reminder represents a reminder entity.
type Reminder struct {
	ID         uuid.UUID  `json:"id"`
	TenantID   uuid.UUID  `json:"tenant_id"`
	UserID     uuid.UUID  `json:"user_id"`
	ClientID   *uuid.UUID `json:"client_id,omitempty"`
	EmailID    *uuid.UUID `json:"email_id,omitempty"`
	DocumentID *uuid.UUID `json:"document_id,omitempty"`
	ServiceID  *uuid.UUID `json:"service_id,omitempty"`
	RemindAt   time.Time  `json:"remind_at"`
	Reason     *string    `json:"reason,omitempty"`
	Status     string     `json:"status"` // pending, completed, dismissed
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	// Computed fields
	ClientName   *string `json:"client_name,omitempty"`
	DocumentName *string `json:"document_name,omitempty"`
	ServiceName  *string `json:"service_name,omitempty"`
}

// CreateReminderRequest represents the request to create a reminder.
type CreateReminderRequest struct {
	ClientID   *uuid.UUID `json:"client_id,omitempty"`
	EmailID    *uuid.UUID `json:"email_id,omitempty"`
	DocumentID *uuid.UUID `json:"document_id,omitempty"`
	ServiceID  *uuid.UUID `json:"service_id,omitempty"`
	RemindAt   time.Time  `json:"remind_at" binding:"required"`
	Reason     *string    `json:"reason,omitempty"`
}

// List returns all reminders for the current user.
// GET /api/v1/reminders
func (h *ReminderHandler) List(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

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
	upcoming := c.Query("upcoming") == "true"

	query := `
		SELECT r.id, r.tenant_id, r.user_id, r.client_id, r.email_id, r.document_id,
		       r.service_id, r.remind_at, r.reason, r.status, r.created_at, r.updated_at,
		       c.company_name as client_name, d.name as document_name, s.name as service_name
		FROM reminders r
		LEFT JOIN clients c ON r.client_id = c.id
		LEFT JOIN documents d ON r.document_id = d.id
		LEFT JOIN services s ON r.service_id = s.id
		WHERE r.tenant_id = $1 AND r.user_id = $2
	`
	args := []interface{}{tenantID, userID}
	argIdx := 3

	if status != "" {
		query += " AND r.status = $" + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}

	if upcoming {
		query += " AND r.remind_at >= NOW() AND r.status = 'pending'"
	}

	query += " ORDER BY r.remind_at ASC LIMIT $" + strconv.Itoa(argIdx) + " OFFSET $" + strconv.Itoa(argIdx+1)
	args = append(args, limit, offset)

	var reminders []Reminder
	err := tenantDB.Query(c, query, args, func(rows pgx.Rows) error {
		var r Reminder
		err := rows.Scan(
			&r.ID, &r.TenantID, &r.UserID, &r.ClientID, &r.EmailID, &r.DocumentID,
			&r.ServiceID, &r.RemindAt, &r.Reason, &r.Status, &r.CreatedAt, &r.UpdatedAt,
			&r.ClientName, &r.DocumentName, &r.ServiceName,
		)
		if err != nil {
			return err
		}
		reminders = append(reminders, r)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list reminders")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reminders"})
		return
	}

	if reminders == nil {
		reminders = []Reminder{}
	}

	c.JSON(http.StatusOK, gin.H{
		"reminders": reminders,
		"count":     len(reminders),
	})
}

// Get returns a single reminder.
// GET /api/v1/reminders/:id
func (h *ReminderHandler) Get(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	reminderID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reminder ID"})
		return
	}

	var r Reminder
	err = tenantDB.QueryRowScan(c, []interface{}{
		&r.ID, &r.TenantID, &r.UserID, &r.ClientID, &r.EmailID, &r.DocumentID,
		&r.ServiceID, &r.RemindAt, &r.Reason, &r.Status, &r.CreatedAt, &r.UpdatedAt,
		&r.ClientName, &r.DocumentName, &r.ServiceName,
	}, `
		SELECT r.id, r.tenant_id, r.user_id, r.client_id, r.email_id, r.document_id,
		       r.service_id, r.remind_at, r.reason, r.status, r.created_at, r.updated_at,
		       c.company_name as client_name, d.name as document_name, s.name as service_name
		FROM reminders r
		LEFT JOIN clients c ON r.client_id = c.id
		LEFT JOIN documents d ON r.document_id = d.id
		LEFT JOIN services s ON r.service_id = s.id
		WHERE r.id = $1 AND r.tenant_id = $2 AND r.user_id = $3
	`, reminderID, tenantID, userID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Reminder not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get reminder")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reminder"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reminder": r})
}

// Create creates a new reminder.
// POST /api/v1/reminders
func (h *ReminderHandler) Create(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req CreateReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate at least one entity is provided
	if req.ClientID == nil && req.EmailID == nil && req.DocumentID == nil && req.ServiceID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one of client_id, email_id, document_id, or service_id is required"})
		return
	}

	reminderID := uuid.New()
	now := time.Now()

	_, err := tenantDB.Exec(c, `
		INSERT INTO reminders (
			id, tenant_id, user_id, client_id, email_id, document_id, service_id,
			remind_at, reason, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending', $10, $10)
	`, reminderID, tenantID, userID, req.ClientID, req.EmailID, req.DocumentID,
		req.ServiceID, req.RemindAt, req.Reason, now)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create reminder")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create reminder"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"reminder": Reminder{
			ID:         reminderID,
			TenantID:   tenantID,
			UserID:     userID,
			ClientID:   req.ClientID,
			EmailID:    req.EmailID,
			DocumentID: req.DocumentID,
			ServiceID:  req.ServiceID,
			RemindAt:   req.RemindAt,
			Reason:     req.Reason,
			Status:     "pending",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	})
}

// Complete marks a reminder as completed.
// POST /api/v1/reminders/:id/complete
func (h *ReminderHandler) Complete(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	reminderID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reminder ID"})
		return
	}

	result, err := tenantDB.Exec(c, `
		UPDATE reminders SET status = 'completed', updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3 AND status = 'pending'
	`, reminderID, tenantID, userID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to complete reminder")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete reminder"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Reminder not found or already completed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reminder marked as completed"})
}

// Dismiss dismisses a reminder.
// POST /api/v1/reminders/:id/dismiss
func (h *ReminderHandler) Dismiss(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	reminderID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reminder ID"})
		return
	}

	result, err := tenantDB.Exec(c, `
		UPDATE reminders SET status = 'dismissed', updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3 AND status = 'pending'
	`, reminderID, tenantID, userID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to dismiss reminder")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to dismiss reminder"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Reminder not found or already processed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reminder dismissed"})
}

// Delete deletes a reminder.
// DELETE /api/v1/reminders/:id
func (h *ReminderHandler) Delete(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	reminderID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reminder ID"})
		return
	}

	result, err := tenantDB.Exec(c, `
		DELETE FROM reminders WHERE id = $1 AND tenant_id = $2 AND user_id = $3
	`, reminderID, tenantID, userID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to delete reminder")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete reminder"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Reminder not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reminder deleted"})
}

// GetUpcoming returns upcoming reminders for the current user.
// GET /api/v1/reminders/upcoming
func (h *ReminderHandler) GetUpcoming(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Get reminders due in the next 24 hours
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	var reminders []Reminder
	err := tenantDB.Query(c, `
		SELECT r.id, r.tenant_id, r.user_id, r.client_id, r.email_id, r.document_id,
		       r.service_id, r.remind_at, r.reason, r.status, r.created_at, r.updated_at,
		       c.company_name as client_name, d.name as document_name, s.name as service_name
		FROM reminders r
		LEFT JOIN clients c ON r.client_id = c.id
		LEFT JOIN documents d ON r.document_id = d.id
		LEFT JOIN services s ON r.service_id = s.id
		WHERE r.tenant_id = $1 AND r.user_id = $2
		  AND r.status = 'pending'
		  AND r.remind_at <= NOW() + INTERVAL '24 hours'
		ORDER BY r.remind_at ASC
		LIMIT $3
	`, []interface{}{tenantID, userID, limit}, func(rows pgx.Rows) error {
		var r Reminder
		err := rows.Scan(
			&r.ID, &r.TenantID, &r.UserID, &r.ClientID, &r.EmailID, &r.DocumentID,
			&r.ServiceID, &r.RemindAt, &r.Reason, &r.Status, &r.CreatedAt, &r.UpdatedAt,
			&r.ClientName, &r.DocumentName, &r.ServiceName,
		)
		if err != nil {
			return err
		}
		reminders = append(reminders, r)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get upcoming reminders")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch upcoming reminders"})
		return
	}

	if reminders == nil {
		reminders = []Reminder{}
	}

	c.JSON(http.StatusOK, gin.H{
		"reminders": reminders,
		"count":     len(reminders),
	})
}
