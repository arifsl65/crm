package handlers

import (
	"encoding/json"
	"fmt"
	"html"
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
	"github.com/accountant-crm/go-backend/internal/email"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// EmailHandler handles email operations.
type EmailHandler struct {
	db          *database.Pool
	emailClient *email.Client
	audit       *audit.Logger
	rateLimiter *middleware.AuthRateLimiter
}

// NewEmailHandler creates a new email handler.
func NewEmailHandler(db *database.Pool, emailClient *email.Client, auditLogger *audit.Logger, rateLimiter *middleware.AuthRateLimiter) *EmailHandler {
	return &EmailHandler{
		db:          db,
		emailClient: emailClient,
		audit:       auditLogger,
		rateLimiter: rateLimiter,
	}
}

// Email represents an email record.
type Email struct {
	ID           uuid.UUID  `json:"id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	ClientID     *uuid.UUID `json:"client_id,omitempty"`
	StaffID      *uuid.UUID `json:"staff_id,omitempty"`
	TemplateID   *uuid.UUID `json:"template_id,omitempty"`
	ThreadID     *string    `json:"thread_id,omitempty"`
	ReplyToID    *uuid.UUID `json:"reply_to_id,omitempty"`
	Direction    string     `json:"direction"` // inbound, outbound
	ToEmail      string     `json:"to_email"`
	ToName       *string    `json:"to_name,omitempty"`
	FromEmail    string     `json:"from_email"`
	Subject      string     `json:"subject"`
	BodyHTML     string     `json:"body_html"`
	BodyText     *string    `json:"body_text,omitempty"`
	Type         string     `json:"type"`   // chase, notification, invite, manual
	Status       string     `json:"status"` // queued, sent, delivered, opened, clicked, bounced, complained
	ResendID     *string    `json:"resend_id,omitempty"`
	IsRead       bool       `json:"is_read"`
	AISummary    *string    `json:"ai_summary,omitempty"`
	Sentiment    *string    `json:"sentiment,omitempty"`
	SentAt       *time.Time `json:"sent_at,omitempty"`
	OpenedAt     *time.Time `json:"opened_at,omitempty"`
	BouncedAt    *time.Time `json:"bounced_at,omitempty"`
	BounceReason *string    `json:"bounce_reason,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	// Computed fields
	ClientName *string `json:"client_name,omitempty"`
	StaffName  *string `json:"staff_name,omitempty"`
}

// SendEmailRequest represents the request to send an email.
type SendEmailRequest struct {
	ToEmail        string     `json:"to_email" binding:"required,email"`
	ToName         *string    `json:"to_name,omitempty"`
	Subject        string     `json:"subject" binding:"required"`
	BodyHTML       string     `json:"body_html" binding:"required"`
	BodyText       *string    `json:"body_text,omitempty"`
	ClientID       *uuid.UUID `json:"client_id,omitempty"`
	TemplateID     *uuid.UUID `json:"template_id,omitempty"`
	Type           string     `json:"type" binding:"omitempty,oneof=chase notification invite manual"`
	IdempotencyKey string     `json:"idempotency_key,omitempty"` // Optional idempotency key
}

// OutboxPayload represents the email payload stored in the outbox.
type OutboxPayload struct {
	EmailID   uuid.UUID  `json:"email_id"`
	ToEmail   string     `json:"to_email"`
	ToName    *string    `json:"to_name,omitempty"`
	FromEmail string     `json:"from_email"`
	Subject   string     `json:"subject"`
	BodyHTML  string     `json:"body_html"`
	BodyText  string     `json:"body_text"`
	ClientID  *uuid.UUID `json:"client_id,omitempty"`
}

// queueEmailParams contains parameters for queueing an email.
type queueEmailParams struct {
	TenantID   uuid.UUID
	UserID     uuid.UUID
	ToEmail    string
	ToName     *string
	FromEmail  string
	Subject    string
	BodyHTML   string
	BodyText   string
	ClientID   *uuid.UUID
	TemplateID *uuid.UUID
	EmailType  string
}

// queueEmail creates an email record and outbox entry in a single transaction.
// This is the common logic shared between Send and SendFromTemplate.
func (h *EmailHandler) queueEmail(c *gin.Context, tenantDB *middleware.TenantDB, params queueEmailParams) (*Email, error) {
	ctx := c.Request.Context()
	id := uuid.New()
	now := time.Now()

	var e Email
	err := tenantDB.Transaction(c, func(tx pgx.Tx) error {
		// Insert email record with status 'queued'
		query := `
			INSERT INTO emails (
				id, tenant_id, client_id, staff_id, template_id, direction,
				to_email, to_name, from_email, subject, body_html, body_text,
				type, status, is_read, created_at
			) VALUES ($1, $2, $3, $4, $5, 'outbound', $6, $7, $8, $9, $10, $11, $12, 'queued', true, $13)
			RETURNING id, tenant_id, client_id, staff_id, template_id, direction,
			          to_email, to_name, from_email, subject, body_html, body_text,
			          type, status, is_read, created_at
		`
		err := tx.QueryRow(ctx, query,
			id, params.TenantID, params.ClientID, params.UserID, params.TemplateID,
			params.ToEmail, params.ToName, params.FromEmail, params.Subject, params.BodyHTML, &params.BodyText,
			params.EmailType, now,
		).Scan(
			&e.ID, &e.TenantID, &e.ClientID, &e.StaffID, &e.TemplateID, &e.Direction,
			&e.ToEmail, &e.ToName, &e.FromEmail, &e.Subject, &e.BodyHTML, &e.BodyText,
			&e.Type, &e.Status, &e.IsRead, &e.CreatedAt,
		)
		if err != nil {
			return err
		}

		// Insert into outbox for async processing
		payload := OutboxPayload{
			EmailID:   id,
			ToEmail:   params.ToEmail,
			ToName:    params.ToName,
			FromEmail: params.FromEmail,
			Subject:   params.Subject,
			BodyHTML:  params.BodyHTML,
			BodyText:  params.BodyText,
			ClientID:  params.ClientID,
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal outbox payload: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO outbox (tenant_id, event_type, payload, created_at)
			VALUES ($1, 'email_send', $2, $3)
		`, params.TenantID, payloadJSON, now)

		return err
	})

	if err != nil {
		return nil, err
	}
	return &e, nil
}

// List returns all emails for the tenant.
// GET /api/v1/emails
func (h *EmailHandler) List(c *gin.Context) {
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
	clientID := c.Query("client_id")
	direction := c.Query("direction")
	status := c.Query("status")
	emailType := c.Query("type")
	search := c.Query("search")

	var query strings.Builder
	var args []interface{}
	argNum := 1

	query.WriteString(`
		SELECT e.id, e.tenant_id, e.client_id, e.staff_id, e.template_id,
		       e.thread_id, e.reply_to_id, e.direction, e.to_email, e.to_name,
		       e.from_email, e.subject, e.body_html, e.body_text, e.type, e.status,
		       e.resend_id, e.is_read, e.ai_summary, e.sentiment,
		       e.sent_at, e.opened_at, e.bounced_at, e.bounce_reason, e.created_at,
		       cl.company_name as client_name, COALESCE(u.name, '') as staff_name
		FROM emails e
		LEFT JOIN clients cl ON e.client_id = cl.id
		LEFT JOIN users u ON e.staff_id = u.id
		WHERE e.tenant_id = $1
	`)
	args = append(args, tenantID)
	argNum++

	if clientID != "" {
		if cid, err := uuid.Parse(clientID); err == nil {
			query.WriteString(` AND e.client_id = $`)
			query.WriteString(strconv.Itoa(argNum))
			args = append(args, cid)
			argNum++
		}
	}

	if direction != "" {
		query.WriteString(` AND e.direction = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, direction)
		argNum++
	}

	if status != "" {
		query.WriteString(` AND e.status = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, status)
		argNum++
	}

	if emailType != "" {
		query.WriteString(` AND e.type = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, emailType)
		argNum++
	}

	if search != "" {
		query.WriteString(` AND (e.subject ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(` OR e.to_email ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(` OR e.from_email ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, "%"+search+"%")
		argNum++
	}

	query.WriteString(` ORDER BY e.created_at DESC LIMIT $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)
	argNum++

	query.WriteString(` OFFSET $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, offset)

	var emails []Email
	err := tenantDB.Query(c, query.String(), args, func(rows pgx.Rows) error {
		var e Email
		err := rows.Scan(
			&e.ID, &e.TenantID, &e.ClientID, &e.StaffID, &e.TemplateID,
			&e.ThreadID, &e.ReplyToID, &e.Direction, &e.ToEmail, &e.ToName,
			&e.FromEmail, &e.Subject, &e.BodyHTML, &e.BodyText, &e.Type, &e.Status,
			&e.ResendID, &e.IsRead, &e.AISummary, &e.Sentiment,
			&e.SentAt, &e.OpenedAt, &e.BouncedAt, &e.BounceReason, &e.CreatedAt,
			&e.ClientName, &e.StaffName,
		)
		if err != nil {
			return err
		}
		emails = append(emails, e)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list emails")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch emails"})
		return
	}

	if emails == nil {
		emails = []Email{}
	}

	c.JSON(http.StatusOK, gin.H{
		"emails": emails,
		"count":  len(emails),
	})
}

// Get returns a single email.
// GET /api/v1/emails/:id
func (h *EmailHandler) Get(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	emailID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email ID"})
		return
	}

	query := `
		SELECT e.id, e.tenant_id, e.client_id, e.staff_id, e.template_id,
		       e.thread_id, e.reply_to_id, e.direction, e.to_email, e.to_name,
		       e.from_email, e.subject, e.body_html, e.body_text, e.type, e.status,
		       e.resend_id, e.is_read, e.ai_summary, e.sentiment,
		       e.sent_at, e.opened_at, e.bounced_at, e.bounce_reason, e.created_at,
		       cl.company_name as client_name, COALESCE(u.name, '') as staff_name
		FROM emails e
		LEFT JOIN clients cl ON e.client_id = cl.id
		LEFT JOIN users u ON e.staff_id = u.id
		WHERE e.id = $1 AND e.tenant_id = $2
	`

	var e Email
	err = tenantDB.QueryRowScan(c, []interface{}{
		&e.ID, &e.TenantID, &e.ClientID, &e.StaffID, &e.TemplateID,
		&e.ThreadID, &e.ReplyToID, &e.Direction, &e.ToEmail, &e.ToName,
		&e.FromEmail, &e.Subject, &e.BodyHTML, &e.BodyText, &e.Type, &e.Status,
		&e.ResendID, &e.IsRead, &e.AISummary, &e.Sentiment,
		&e.SentAt, &e.OpenedAt, &e.BouncedAt, &e.BounceReason, &e.CreatedAt,
		&e.ClientName, &e.StaffName,
	}, query, emailID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get email")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch email"})
		return
	}

	// Mark as read if viewing
	if !e.IsRead {
		_, _ = tenantDB.Exec(c, `UPDATE emails SET is_read = true WHERE id = $1 AND tenant_id = $2`, emailID, tenantID)
		e.IsRead = true
	}

	c.JSON(http.StatusOK, e)
}

// Send queues a new email for sending via the outbox pattern.
// POST /api/v1/emails
func (h *EmailHandler) Send(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req SendEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check rate limit
	if h.rateLimiter != nil {
		allowed, count, ttl, err := h.rateLimiter.CheckEmailSendRate(ctx, tenantID.String())
		if err != nil {
			log.Error().Err(err).Msg("Failed to check email rate limit")
		} else if !allowed {
			middleware.RateLimitExceededWithLog(c, "email-send", tenantID.String(), count, ttl)
			return
		}
	}

	// Check idempotency key if provided
	if req.IdempotencyKey != "" && h.rateLimiter != nil {
		isDuplicate, err := h.rateLimiter.CheckIdempotencyKey(ctx, tenantID.String(), req.IdempotencyKey)
		if err != nil {
			log.Error().Err(err).Msg("Failed to check idempotency key")
		} else if isDuplicate {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "duplicate_request",
				"message": "This request has already been processed",
			})
			return
		}
	}

	// Default type
	emailType := req.Type
	if emailType == "" {
		emailType = "manual"
	}

	// Check if email client is configured
	if h.emailClient == nil || !h.emailClient.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Email service not configured"})
		return
	}

	// Prepare body text if not provided
	bodyText := ""
	if req.BodyText != nil {
		bodyText = *req.BodyText
	}

	// Queue email using common helper
	e, err := h.queueEmail(c, tenantDB, queueEmailParams{
		TenantID:   tenantID,
		UserID:     userID,
		ToEmail:    req.ToEmail,
		ToName:     req.ToName,
		FromEmail:  h.emailClient.GetFromEmail(),
		Subject:    req.Subject,
		BodyHTML:   req.BodyHTML,
		BodyText:   bodyText,
		ClientID:   req.ClientID,
		TemplateID: req.TemplateID,
		EmailType:  emailType,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to queue email")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue email"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionEmailSend, &userID, &tenantID, "email", &e.ID, c.ClientIP(), map[string]interface{}{
		"to":      req.ToEmail,
		"subject": req.Subject,
		"type":    emailType,
		"status":  "queued",
	})

	c.JSON(http.StatusAccepted, gin.H{
		"email":   e,
		"message": "Email queued for delivery",
	})
}

// MarkRead marks an email as read.
// PATCH /api/v1/emails/:id/read
func (h *EmailHandler) MarkRead(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	emailID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email ID"})
		return
	}

	result, err := tenantDB.Exec(c, `UPDATE emails SET is_read = true WHERE id = $1 AND tenant_id = $2`, emailID, tenantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to mark email as read")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update email"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email marked as read"})
}

// Claim claims an email for the current user (for triage).
// PATCH /api/v1/emails/:id/claim
func (h *EmailHandler) Claim(c *gin.Context) {
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

	emailID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email ID"})
		return
	}

	// Check if email exists and is not already claimed
	var existingClaimedBy *uuid.UUID
	err = tenantDB.QueryRowScan(c, []interface{}{&existingClaimedBy},
		`SELECT claimed_by FROM emails WHERE id = $1 AND tenant_id = $2`, emailID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to check email claim status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check email"})
		return
	}

	// If already claimed by someone else, return conflict
	if existingClaimedBy != nil && *existingClaimedBy != userID {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "already_claimed",
			"message": "Email is already claimed by another user",
		})
		return
	}

	// Claim the email
	now := time.Now()
	result, err := tenantDB.Exec(c,
		`UPDATE emails SET claimed_by = $1, claimed_at = $2 WHERE id = $3 AND tenant_id = $4`,
		userID, now, emailID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to claim email")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to claim email"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionUpdate, &userID, &tenantID, "email", &emailID, c.ClientIP(), map[string]interface{}{
		"action":     "claim",
		"claimed_by": userID.String(),
		"claimed_at": now,
	})

	c.JSON(http.StatusOK, gin.H{
		"message":    "Email claimed successfully",
		"claimed_by": userID,
		"claimed_at": now,
	})
}

// Unclaim removes the claim on an email.
// PATCH /api/v1/emails/:id/unclaim
func (h *EmailHandler) Unclaim(c *gin.Context) {
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

	emailID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email ID"})
		return
	}

	// Only allow unclaim if user is the one who claimed it (or admin)
	role, _ := middleware.GetRole(c)
	var existingClaimedBy *uuid.UUID
	err = tenantDB.QueryRowScan(c, []interface{}{&existingClaimedBy},
		`SELECT claimed_by FROM emails WHERE id = $1 AND tenant_id = $2`, emailID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to check email claim status")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check email"})
		return
	}

	// Check permission: only the claimer or admin can unclaim
	if existingClaimedBy != nil && *existingClaimedBy != userID && role != "super_admin" && role != "tenant_admin" {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "not_authorized",
			"message": "You can only unclaim emails you have claimed",
		})
		return
	}

	// Unclaim the email
	result, err := tenantDB.Exec(c,
		`UPDATE emails SET claimed_by = NULL, claimed_at = NULL WHERE id = $1 AND tenant_id = $2`,
		emailID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to unclaim email")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unclaim email"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionUpdate, &userID, &tenantID, "email", &emailID, c.ClientIP(), map[string]interface{}{
		"action": "unclaim",
	})

	c.JSON(http.StatusOK, gin.H{"message": "Email unclaimed successfully"})
}

// GetStats returns email statistics for the tenant.
// GET /api/v1/emails/stats
func (h *EmailHandler) GetStats(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	query := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE direction = 'outbound') as sent,
			COUNT(*) FILTER (WHERE direction = 'inbound') as received,
			COUNT(*) FILTER (WHERE status = 'delivered') as delivered,
			COUNT(*) FILTER (WHERE status = 'opened') as opened,
			COUNT(*) FILTER (WHERE status = 'bounced') as bounced,
			COUNT(*) FILTER (WHERE status = 'queued') as queued,
			COUNT(*) FILTER (WHERE is_read = false AND direction = 'inbound') as unread
		FROM emails
		WHERE tenant_id = $1 AND created_at > NOW() - INTERVAL '30 days'
	`

	var stats struct {
		Total     int `json:"total"`
		Sent      int `json:"sent"`
		Received  int `json:"received"`
		Delivered int `json:"delivered"`
		Opened    int `json:"opened"`
		Bounced   int `json:"bounced"`
		Queued    int `json:"queued"`
		Unread    int `json:"unread"`
	}

	err := tenantDB.QueryRowScan(c, []interface{}{
		&stats.Total, &stats.Sent, &stats.Received, &stats.Delivered,
		&stats.Opened, &stats.Bounced, &stats.Queued, &stats.Unread,
	}, query, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to get email stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch email stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// SendFromTemplate queues an email using a template via the outbox pattern.
// POST /api/v1/emails/send-template
func (h *EmailHandler) SendFromTemplate(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req struct {
		TemplateID     uuid.UUID         `json:"template_id" binding:"required"`
		ToEmail        string            `json:"to_email" binding:"required,email"`
		ToName         *string           `json:"to_name,omitempty"`
		ClientID       *uuid.UUID        `json:"client_id,omitempty"`
		Placeholders   map[string]string `json:"placeholders,omitempty"`
		IdempotencyKey string            `json:"idempotency_key,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check rate limit
	if h.rateLimiter != nil {
		allowed, count, ttl, err := h.rateLimiter.CheckEmailSendRate(ctx, tenantID.String())
		if err != nil {
			log.Error().Err(err).Msg("Failed to check email rate limit")
		} else if !allowed {
			middleware.RateLimitExceededWithLog(c, "email-send-template", tenantID.String(), count, ttl)
			return
		}
	}

	// Check idempotency key if provided
	if req.IdempotencyKey != "" && h.rateLimiter != nil {
		isDuplicate, err := h.rateLimiter.CheckIdempotencyKey(ctx, tenantID.String(), req.IdempotencyKey)
		if err != nil {
			log.Error().Err(err).Msg("Failed to check idempotency key")
		} else if isDuplicate {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "duplicate_request",
				"message": "This request has already been processed",
			})
			return
		}
	}

	// Get template
	var template struct {
		Subject  string
		BodyHTML string
		BodyText *string
		Type     string
	}

	err := tenantDB.QueryRowScan(c, []interface{}{
		&template.Subject, &template.BodyHTML, &template.BodyText, &template.Type,
	}, `SELECT subject, body_html, body_text, type FROM email_templates WHERE id = $1 AND tenant_id = $2 AND is_active = true`,
		req.TemplateID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get email template")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch template"})
		return
	}

	// Replace placeholders
	subject := template.Subject
	bodyHTML := template.BodyHTML
	bodyText := ""
	if template.BodyText != nil {
		bodyText = *template.BodyText
	}

	for key, value := range req.Placeholders {
		placeholder := fmt.Sprintf("{{%s}}", key)
		// Subject and bodyText are plain text - no escaping needed
		subject = strings.ReplaceAll(subject, placeholder, value)
		bodyText = strings.ReplaceAll(bodyText, placeholder, value)
		// SECURITY: Escape HTML in bodyHTML to prevent XSS
		// User-provided values could contain <script> tags or other malicious HTML
		bodyHTML = strings.ReplaceAll(bodyHTML, placeholder, html.EscapeString(value))
	}

	// Check if email client is configured
	if h.emailClient == nil || !h.emailClient.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Email service not configured"})
		return
	}

	// Queue email using common helper
	e, err := h.queueEmail(c, tenantDB, queueEmailParams{
		TenantID:   tenantID,
		UserID:     userID,
		ToEmail:    req.ToEmail,
		ToName:     req.ToName,
		FromEmail:  h.emailClient.GetFromEmail(),
		Subject:    subject,
		BodyHTML:   bodyHTML,
		BodyText:   bodyText,
		ClientID:   req.ClientID,
		TemplateID: &req.TemplateID,
		EmailType:  template.Type,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to queue template email")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue email"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionEmailSend, &userID, &tenantID, "email", &e.ID, c.ClientIP(), map[string]interface{}{
		"to":          req.ToEmail,
		"subject":     subject,
		"template_id": req.TemplateID.String(),
		"status":      "queued",
	})

	c.JSON(http.StatusAccepted, gin.H{
		"email":   e,
		"message": "Email queued for delivery",
	})
}

// ============================================================================
// Email Thread Handlers
// ============================================================================

// EmailThread represents an email thread/conversation.
type EmailThread struct {
	ID            uuid.UUID       `json:"id"`
	TenantID      uuid.UUID       `json:"tenant_id"`
	ThreadKey     string          `json:"thread_key"`
	ClientID      *uuid.UUID      `json:"client_id,omitempty"`
	FirstEmailID  *uuid.UUID      `json:"first_email_id,omitempty"`
	Subject       string          `json:"subject"`
	Participants  json.RawMessage `json:"participants"`
	LastMessageAt *time.Time      `json:"last_message_at,omitempty"`
	MessageCount  int             `json:"message_count"`
	AISummary     *string         `json:"ai_summary,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	// Computed fields
	ClientName *string `json:"client_name,omitempty"`
}

// ListThreads returns all email threads for the tenant.
// GET /api/v1/emails/threads
func (h *EmailHandler) ListThreads(c *gin.Context) {
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
	clientID := c.Query("client_id")
	search := c.Query("search")

	var query strings.Builder
	var args []interface{}
	argNum := 1

	query.WriteString(`
		SELECT t.id, t.tenant_id, t.thread_key, t.client_id, t.first_email_id,
		       t.subject, t.participants, t.last_message_at, t.message_count,
		       t.ai_summary, t.created_at, t.updated_at,
		       cl.company_name as client_name
		FROM email_threads t
		LEFT JOIN clients cl ON t.client_id = cl.id
		WHERE t.tenant_id = $1
	`)
	args = append(args, tenantID)
	argNum++

	if clientID != "" {
		if cid, err := uuid.Parse(clientID); err == nil {
			query.WriteString(` AND t.client_id = $`)
			query.WriteString(strconv.Itoa(argNum))
			args = append(args, cid)
			argNum++
		}
	}

	if search != "" {
		query.WriteString(` AND (t.subject ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(` OR t.ai_summary ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, "%"+search+"%")
		argNum++
	}

	query.WriteString(` ORDER BY t.last_message_at DESC NULLS LAST, t.created_at DESC LIMIT $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)
	argNum++

	query.WriteString(` OFFSET $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, offset)

	var threads []EmailThread
	err := tenantDB.Query(c, query.String(), args, func(rows pgx.Rows) error {
		var t EmailThread
		err := rows.Scan(
			&t.ID, &t.TenantID, &t.ThreadKey, &t.ClientID, &t.FirstEmailID,
			&t.Subject, &t.Participants, &t.LastMessageAt, &t.MessageCount,
			&t.AISummary, &t.CreatedAt, &t.UpdatedAt,
			&t.ClientName,
		)
		if err != nil {
			return err
		}
		threads = append(threads, t)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list email threads")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch email threads"})
		return
	}

	if threads == nil {
		threads = []EmailThread{}
	}

	c.JSON(http.StatusOK, gin.H{
		"threads": threads,
		"count":   len(threads),
	})
}

// GetThread returns a single email thread.
// GET /api/v1/emails/threads/:id
func (h *EmailHandler) GetThread(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	threadID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid thread ID"})
		return
	}

	query := `
		SELECT t.id, t.tenant_id, t.thread_key, t.client_id, t.first_email_id,
		       t.subject, t.participants, t.last_message_at, t.message_count,
		       t.ai_summary, t.created_at, t.updated_at,
		       cl.company_name as client_name
		FROM email_threads t
		LEFT JOIN clients cl ON t.client_id = cl.id
		WHERE t.id = $1 AND t.tenant_id = $2
	`

	var t EmailThread
	err = tenantDB.QueryRowScan(c, []interface{}{
		&t.ID, &t.TenantID, &t.ThreadKey, &t.ClientID, &t.FirstEmailID,
		&t.Subject, &t.Participants, &t.LastMessageAt, &t.MessageCount,
		&t.AISummary, &t.CreatedAt, &t.UpdatedAt,
		&t.ClientName,
	}, query, threadID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Thread not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get email thread")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch thread"})
		return
	}

	c.JSON(http.StatusOK, t)
}

// GetThreadMessages returns all emails in a thread.
// GET /api/v1/emails/threads/:id/messages
func (h *EmailHandler) GetThreadMessages(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	threadID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid thread ID"})
		return
	}

	// Pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	// First verify the thread exists and belongs to this tenant
	var threadKey string
	err = tenantDB.QueryRowScan(c, []interface{}{&threadKey},
		`SELECT thread_key FROM email_threads WHERE id = $1 AND tenant_id = $2`,
		threadID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Thread not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to verify thread")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch thread"})
		return
	}

	// Get emails in this thread (by thread_key since emails.thread_id stores thread_key)
	query := `
		SELECT e.id, e.tenant_id, e.client_id, e.staff_id, e.template_id,
		       e.thread_id, e.reply_to_id, e.direction, e.to_email, e.to_name,
		       e.from_email, e.subject, e.body_html, e.body_text, e.type, e.status,
		       e.resend_id, e.is_read, e.ai_summary, e.sentiment,
		       e.sent_at, e.opened_at, e.bounced_at, e.bounce_reason, e.created_at,
		       cl.company_name as client_name, COALESCE(u.name, '') as staff_name
		FROM emails e
		LEFT JOIN clients cl ON e.client_id = cl.id
		LEFT JOIN users u ON e.staff_id = u.id
		WHERE e.thread_id = $1 AND e.tenant_id = $2
		ORDER BY e.created_at ASC
		LIMIT $3 OFFSET $4
	`

	var emails []Email
	err = tenantDB.Query(c, query, []interface{}{threadKey, tenantID, limit, offset}, func(rows pgx.Rows) error {
		var e Email
		err := rows.Scan(
			&e.ID, &e.TenantID, &e.ClientID, &e.StaffID, &e.TemplateID,
			&e.ThreadID, &e.ReplyToID, &e.Direction, &e.ToEmail, &e.ToName,
			&e.FromEmail, &e.Subject, &e.BodyHTML, &e.BodyText, &e.Type, &e.Status,
			&e.ResendID, &e.IsRead, &e.AISummary, &e.Sentiment,
			&e.SentAt, &e.OpenedAt, &e.BouncedAt, &e.BounceReason, &e.CreatedAt,
			&e.ClientName, &e.StaffName,
		)
		if err != nil {
			return err
		}
		emails = append(emails, e)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list thread messages")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}

	if emails == nil {
		emails = []Email{}
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": emails,
		"count":    len(emails),
	})
}
