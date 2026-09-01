package handlers

import (
	"fmt"
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
}

// NewEmailHandler creates a new email handler.
func NewEmailHandler(db *database.Pool, emailClient *email.Client, auditLogger *audit.Logger) *EmailHandler {
	return &EmailHandler{
		db:          db,
		emailClient: emailClient,
		audit:       auditLogger,
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
	Type         string     `json:"type"` // chase, notification, invite, manual
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
	ToEmail    string     `json:"to_email" binding:"required,email"`
	ToName     *string    `json:"to_name,omitempty"`
	Subject    string     `json:"subject" binding:"required"`
	BodyHTML   string     `json:"body_html" binding:"required"`
	BodyText   *string    `json:"body_text,omitempty"`
	ClientID   *uuid.UUID `json:"client_id,omitempty"`
	TemplateID *uuid.UUID `json:"template_id,omitempty"`
	Type       string     `json:"type" binding:"omitempty,oneof=chase notification invite manual"`
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
		       cl.company_name as client_name, u.name as staff_name
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
		       cl.company_name as client_name, u.name as staff_name
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

// Send sends a new email and stores it in the database.
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

	// Send via Resend
	resendID, err := h.emailClient.SendWithID(req.ToEmail, req.Subject, req.BodyHTML, bodyText)
	if err != nil {
		log.Error().Err(err).Str("to", req.ToEmail).Msg("Failed to send email via Resend")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send email"})
		return
	}

	// Store in database
	id := uuid.New()
	now := time.Now()

	var e Email
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		query := `
			INSERT INTO emails (
				id, tenant_id, client_id, staff_id, template_id, direction,
				to_email, to_name, from_email, subject, body_html, body_text,
				type, status, resend_id, is_read, sent_at, created_at
			) VALUES ($1, $2, $3, $4, $5, 'outbound', $6, $7, $8, $9, $10, $11, $12, 'sent', $13, true, $14, $14)
			RETURNING id, tenant_id, client_id, staff_id, template_id, direction,
			          to_email, to_name, from_email, subject, body_html, body_text,
			          type, status, resend_id, is_read, sent_at, created_at
		`
		return tx.QueryRow(ctx, query,
			id, tenantID, req.ClientID, userID, req.TemplateID,
			req.ToEmail, req.ToName, h.emailClient.GetFromEmail(), req.Subject, req.BodyHTML, req.BodyText,
			emailType, resendID, now,
		).Scan(
			&e.ID, &e.TenantID, &e.ClientID, &e.StaffID, &e.TemplateID, &e.Direction,
			&e.ToEmail, &e.ToName, &e.FromEmail, &e.Subject, &e.BodyHTML, &e.BodyText,
			&e.Type, &e.Status, &e.ResendID, &e.IsRead, &e.SentAt, &e.CreatedAt,
		)
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to store sent email")
		// Email was sent but not stored - log but don't fail
		c.JSON(http.StatusCreated, gin.H{
			"message":   "Email sent but failed to store record",
			"resend_id": resendID,
		})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionEmailSend, &userID, &tenantID, "email", &e.ID, c.ClientIP(), map[string]interface{}{
		"to":      req.ToEmail,
		"subject": req.Subject,
		"type":    emailType,
	})

	c.JSON(http.StatusCreated, e)
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
		Unread    int `json:"unread"`
	}

	err := tenantDB.QueryRowScan(c, []interface{}{
		&stats.Total, &stats.Sent, &stats.Received, &stats.Delivered,
		&stats.Opened, &stats.Bounced, &stats.Unread,
	}, query, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to get email stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch email stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// SendFromTemplate sends an email using a template.
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
		TemplateID   uuid.UUID          `json:"template_id" binding:"required"`
		ToEmail      string             `json:"to_email" binding:"required,email"`
		ToName       *string            `json:"to_name,omitempty"`
		ClientID     *uuid.UUID         `json:"client_id,omitempty"`
		Placeholders map[string]string  `json:"placeholders,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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
		subject = strings.ReplaceAll(subject, placeholder, value)
		bodyHTML = strings.ReplaceAll(bodyHTML, placeholder, value)
		bodyText = strings.ReplaceAll(bodyText, placeholder, value)
	}

	// Check if email client is configured
	if h.emailClient == nil || !h.emailClient.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Email service not configured"})
		return
	}

	// Send via Resend
	resendID, err := h.emailClient.SendWithID(req.ToEmail, subject, bodyHTML, bodyText)
	if err != nil {
		log.Error().Err(err).Str("to", req.ToEmail).Msg("Failed to send template email via Resend")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send email"})
		return
	}

	// Store in database
	id := uuid.New()
	now := time.Now()

	var e Email
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		query := `
			INSERT INTO emails (
				id, tenant_id, client_id, staff_id, template_id, direction,
				to_email, to_name, from_email, subject, body_html, body_text,
				type, status, resend_id, is_read, sent_at, created_at
			) VALUES ($1, $2, $3, $4, $5, 'outbound', $6, $7, $8, $9, $10, $11, $12, 'sent', $13, true, $14, $14)
			RETURNING id, tenant_id, client_id, staff_id, template_id, direction,
			          to_email, to_name, from_email, subject, body_html, body_text,
			          type, status, resend_id, is_read, sent_at, created_at
		`
		return tx.QueryRow(ctx, query,
			id, tenantID, req.ClientID, userID, req.TemplateID,
			req.ToEmail, req.ToName, h.emailClient.GetFromEmail(), subject, bodyHTML, &bodyText,
			template.Type, resendID, now,
		).Scan(
			&e.ID, &e.TenantID, &e.ClientID, &e.StaffID, &e.TemplateID, &e.Direction,
			&e.ToEmail, &e.ToName, &e.FromEmail, &e.Subject, &e.BodyHTML, &e.BodyText,
			&e.Type, &e.Status, &e.ResendID, &e.IsRead, &e.SentAt, &e.CreatedAt,
		)
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to store sent template email")
		c.JSON(http.StatusCreated, gin.H{
			"message":   "Email sent but failed to store record",
			"resend_id": resendID,
		})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionEmailSend, &userID, &tenantID, "email", &e.ID, c.ClientIP(), map[string]interface{}{
		"to":          req.ToEmail,
		"subject":     subject,
		"template_id": req.TemplateID.String(),
	})

	c.JSON(http.StatusCreated, e)
}
