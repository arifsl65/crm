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

// ChaseLogHandler handles chase log operations.
type ChaseLogHandler struct {
	db          *database.Pool
	emailClient *email.Client
	audit       *audit.Logger
}

// NewChaseLogHandler creates a new chase log handler.
func NewChaseLogHandler(db *database.Pool, emailClient *email.Client, auditLogger *audit.Logger) *ChaseLogHandler {
	return &ChaseLogHandler{
		db:          db,
		emailClient: emailClient,
		audit:       auditLogger,
	}
}

// ChaseLog represents a chase log record.
type ChaseLog struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	InitiatedBy uuid.UUID `json:"initiated_by"`
	TotalSent   int       `json:"total_sent"`
	Delivered   int       `json:"delivered"`
	Opened      int       `json:"opened"`
	Bounced     int       `json:"bounced"`
	CreatedAt   time.Time `json:"created_at"`
	// Computed fields
	InitiatedByName *string `json:"initiated_by_name,omitempty"`
}

// ChaseLogClient represents a client in a chase log.
type ChaseLogClient struct {
	ID           uuid.UUID  `json:"id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	ChaseLogID   uuid.UUID  `json:"chase_log_id"`
	ClientID     uuid.UUID  `json:"client_id"`
	EmailID      *uuid.UUID `json:"email_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	// Computed fields from client/email
	ClientName   *string    `json:"client_name,omitempty"`
	ClientEmail  *string    `json:"client_email,omitempty"`
	EmailStatus  *string    `json:"email_status,omitempty"`
	EmailSentAt  *time.Time `json:"email_sent_at,omitempty"`
}

// ChaseLogDetail represents a chase log with its clients.
type ChaseLogDetail struct {
	ChaseLog
	Clients []ChaseLogClient `json:"clients"`
}

// CreateChaseRequest represents the request to create a chase.
type CreateChaseRequest struct {
	TemplateID   uuid.UUID   `json:"template_id" binding:"required"`
	ClientIDs    []uuid.UUID `json:"client_ids" binding:"required,min=1"`
	Placeholders map[string]string `json:"placeholders,omitempty"`
}

// List returns all chase logs for the tenant.
// GET /api/v1/chase-logs
func (h *ChaseLogHandler) List(c *gin.Context) {
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

	query := `
		SELECT cl.id, cl.tenant_id, cl.initiated_by, cl.total_sent, cl.delivered,
		       cl.opened, cl.bounced, cl.created_at, u.name as initiated_by_name
		FROM chase_logs cl
		LEFT JOIN users u ON cl.initiated_by = u.id
		WHERE cl.tenant_id = $1
		ORDER BY cl.created_at DESC
		LIMIT $2 OFFSET $3
	`

	var logs []ChaseLog
	err := tenantDB.Query(c, query, []interface{}{tenantID, limit, offset}, func(rows pgx.Rows) error {
		var l ChaseLog
		err := rows.Scan(
			&l.ID, &l.TenantID, &l.InitiatedBy, &l.TotalSent, &l.Delivered,
			&l.Opened, &l.Bounced, &l.CreatedAt, &l.InitiatedByName,
		)
		if err != nil {
			return err
		}
		logs = append(logs, l)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list chase logs")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chase logs"})
		return
	}

	if logs == nil {
		logs = []ChaseLog{}
	}

	c.JSON(http.StatusOK, gin.H{
		"chase_logs": logs,
		"count":      len(logs),
	})
}

// Get returns a single chase log with its clients.
// GET /api/v1/chase-logs/:id
func (h *ChaseLogHandler) Get(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	chaseLogID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chase log ID"})
		return
	}

	// Get chase log
	var detail ChaseLogDetail
	err = tenantDB.QueryRowScan(c, []interface{}{
		&detail.ID, &detail.TenantID, &detail.InitiatedBy, &detail.TotalSent, &detail.Delivered,
		&detail.Opened, &detail.Bounced, &detail.CreatedAt, &detail.InitiatedByName,
	}, `
		SELECT cl.id, cl.tenant_id, cl.initiated_by, cl.total_sent, cl.delivered,
		       cl.opened, cl.bounced, cl.created_at, u.name as initiated_by_name
		FROM chase_logs cl
		LEFT JOIN users u ON cl.initiated_by = u.id
		WHERE cl.id = $1 AND cl.tenant_id = $2
	`, chaseLogID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chase log not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chase log")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chase log"})
		return
	}

	// Get clients
	clientQuery := `
		SELECT clc.id, clc.tenant_id, clc.chase_log_id, clc.client_id, clc.email_id,
		       clc.created_at, c.company_name as client_name, c.email as client_email,
		       e.status as email_status, e.sent_at as email_sent_at
		FROM chase_log_clients clc
		LEFT JOIN clients c ON clc.client_id = c.id
		LEFT JOIN emails e ON clc.email_id = e.id
		WHERE clc.chase_log_id = $1 AND clc.tenant_id = $2
		ORDER BY clc.created_at ASC
	`

	detail.Clients = []ChaseLogClient{}
	err = tenantDB.Query(c, clientQuery, []interface{}{chaseLogID, tenantID}, func(rows pgx.Rows) error {
		var clc ChaseLogClient
		err := rows.Scan(
			&clc.ID, &clc.TenantID, &clc.ChaseLogID, &clc.ClientID, &clc.EmailID,
			&clc.CreatedAt, &clc.ClientName, &clc.ClientEmail,
			&clc.EmailStatus, &clc.EmailSentAt,
		)
		if err != nil {
			return err
		}
		detail.Clients = append(detail.Clients, clc)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get chase log clients")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chase log clients"})
		return
	}

	c.JSON(http.StatusOK, detail)
}

// Create creates a new chase and sends emails to selected clients.
// POST /api/v1/chase-logs
func (h *ChaseLogHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req CreateChaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if email client is configured
	if h.emailClient == nil || !h.emailClient.IsConfigured() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Email service not configured"})
		return
	}

	// Get template
	var template struct {
		Subject  string
		BodyHTML string
		BodyText *string
	}

	err := tenantDB.QueryRowScan(c, []interface{}{
		&template.Subject, &template.BodyHTML, &template.BodyText,
	}, `SELECT subject, body_html, body_text FROM email_templates WHERE id = $1 AND tenant_id = $2 AND is_active = true AND type = 'chase'`,
		req.TemplateID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chase template not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get chase template")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch template"})
		return
	}

	// Get clients with their emails
	clientPlaceholders := make([]string, len(req.ClientIDs))
	clientArgs := make([]interface{}, len(req.ClientIDs)+1)
	clientArgs[0] = tenantID
	for i, cid := range req.ClientIDs {
		clientPlaceholders[i] = fmt.Sprintf("$%d", i+2)
		clientArgs[i+1] = cid
	}

	type clientInfo struct {
		ID          uuid.UUID
		Email       string
		CompanyName string
	}

	var clients []clientInfo
	err = tenantDB.Query(c, `
		SELECT id, email, company_name FROM clients
		WHERE tenant_id = $1 AND id IN (`+strings.Join(clientPlaceholders, ",")+`)
	`, clientArgs, func(rows pgx.Rows) error {
		var ci clientInfo
		err := rows.Scan(&ci.ID, &ci.Email, &ci.CompanyName)
		if err != nil {
			return err
		}
		clients = append(clients, ci)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get clients for chase")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch clients"})
		return
	}

	if len(clients) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid clients found"})
		return
	}

	// Create chase log
	chaseLogID := uuid.New()
	now := time.Now()
	totalSent := 0
	var chaseLogClients []ChaseLogClient

	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		// Insert chase log
		_, err := tx.Exec(ctx, `
			INSERT INTO chase_logs (id, tenant_id, initiated_by, total_sent, delivered, opened, bounced, created_at)
			VALUES ($1, $2, $3, 0, 0, 0, 0, $4)
		`, chaseLogID, tenantID, userID, now)
		if err != nil {
			return err
		}

		// Send emails and create chase_log_clients records
		for _, client := range clients {
			// Replace placeholders
			subject := template.Subject
			bodyHTML := template.BodyHTML
			bodyText := ""
			if template.BodyText != nil {
				bodyText = *template.BodyText
			}

			// Default placeholders
			subject = strings.ReplaceAll(subject, "{{client_name}}", client.CompanyName)
			bodyHTML = strings.ReplaceAll(bodyHTML, "{{client_name}}", client.CompanyName)
			bodyText = strings.ReplaceAll(bodyText, "{{client_name}}", client.CompanyName)

			// Custom placeholders
			for key, value := range req.Placeholders {
				placeholder := fmt.Sprintf("{{%s}}", key)
				subject = strings.ReplaceAll(subject, placeholder, value)
				bodyHTML = strings.ReplaceAll(bodyHTML, placeholder, value)
				bodyText = strings.ReplaceAll(bodyText, placeholder, value)
			}

			// Send email
			resendID, err := h.emailClient.SendWithID(client.Email, subject, bodyHTML, bodyText)
			if err != nil {
				log.Error().Err(err).Str("to", client.Email).Msg("Failed to send chase email")
				// Continue with other clients
				continue
			}

			// Insert email record
			emailID := uuid.New()
			_, err = tx.Exec(ctx, `
				INSERT INTO emails (
					id, tenant_id, client_id, staff_id, template_id, direction,
					to_email, from_email, subject, body_html, body_text,
					type, status, resend_id, is_read, sent_at, created_at
				) VALUES ($1, $2, $3, $4, $5, 'outbound', $6, $7, $8, $9, $10, 'chase', 'sent', $11, true, $12, $12)
			`, emailID, tenantID, client.ID, userID, req.TemplateID,
				client.Email, h.emailClient.GetFromEmail(), subject, bodyHTML, bodyText,
				resendID, now)
			if err != nil {
				log.Error().Err(err).Msg("Failed to insert chase email record")
				continue
			}

			// Insert chase_log_client
			clcID := uuid.New()
			_, err = tx.Exec(ctx, `
				INSERT INTO chase_log_clients (id, tenant_id, chase_log_id, client_id, email_id, email_created_at, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, clcID, tenantID, chaseLogID, client.ID, emailID, now, now)
			if err != nil {
				log.Error().Err(err).Msg("Failed to insert chase log client")
				continue
			}

			totalSent++
			chaseLogClients = append(chaseLogClients, ChaseLogClient{
				ID:          clcID,
				TenantID:    tenantID,
				ChaseLogID:  chaseLogID,
				ClientID:    client.ID,
				EmailID:     &emailID,
				CreatedAt:   now,
				ClientName:  &client.CompanyName,
				ClientEmail: &client.Email,
			})
		}

		// Update total_sent
		_, err = tx.Exec(ctx, `UPDATE chase_logs SET total_sent = $1 WHERE id = $2`, totalSent, chaseLogID)
		return err
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create chase")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create chase"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionChaseCreate, &userID, &tenantID, "chase_log", &chaseLogID, c.ClientIP(), map[string]interface{}{
		"template_id": req.TemplateID.String(),
		"total_sent":  totalSent,
		"clients":     len(req.ClientIDs),
	})

	c.JSON(http.StatusCreated, gin.H{
		"chase_log": ChaseLogDetail{
			ChaseLog: ChaseLog{
				ID:          chaseLogID,
				TenantID:    tenantID,
				InitiatedBy: userID,
				TotalSent:   totalSent,
				Delivered:   0,
				Opened:      0,
				Bounced:     0,
				CreatedAt:   now,
			},
			Clients: chaseLogClients,
		},
	})
}

// GetStats returns chase statistics for the tenant.
// GET /api/v1/chase-logs/stats
func (h *ChaseLogHandler) GetStats(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	query := `
		SELECT
			COUNT(*) as total_chases,
			COALESCE(SUM(total_sent), 0) as total_sent,
			COALESCE(SUM(delivered), 0) as total_delivered,
			COALESCE(SUM(opened), 0) as total_opened,
			COALESCE(SUM(bounced), 0) as total_bounced
		FROM chase_logs
		WHERE tenant_id = $1 AND created_at > NOW() - INTERVAL '30 days'
	`

	var stats struct {
		TotalChases    int `json:"total_chases"`
		TotalSent      int `json:"total_sent"`
		TotalDelivered int `json:"total_delivered"`
		TotalOpened    int `json:"total_opened"`
		TotalBounced   int `json:"total_bounced"`
	}

	err := tenantDB.QueryRowScan(c, []interface{}{
		&stats.TotalChases, &stats.TotalSent, &stats.TotalDelivered,
		&stats.TotalOpened, &stats.TotalBounced,
	}, query, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to get chase stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chase stats"})
		return
	}

	// Calculate rates
	openRate := 0.0
	bounceRate := 0.0
	if stats.TotalSent > 0 {
		openRate = float64(stats.TotalOpened) / float64(stats.TotalSent) * 100
		bounceRate = float64(stats.TotalBounced) / float64(stats.TotalSent) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"stats": gin.H{
			"total_chases":    stats.TotalChases,
			"total_sent":      stats.TotalSent,
			"total_delivered": stats.TotalDelivered,
			"total_opened":    stats.TotalOpened,
			"total_bounced":   stats.TotalBounced,
			"open_rate":       openRate,
			"bounce_rate":     bounceRate,
		},
	})
}
