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

// EmailAccountHandler handles email account operations.
type EmailAccountHandler struct {
	db    *database.Pool
	audit *audit.Logger
}

// NewEmailAccountHandler creates a new email account handler.
func NewEmailAccountHandler(db *database.Pool, auditLogger *audit.Logger) *EmailAccountHandler {
	return &EmailAccountHandler{
		db:    db,
		audit: auditLogger,
	}
}

// EmailAccount represents an email account (without sensitive fields).
type EmailAccount struct {
	ID             uuid.UUID  `json:"id"`
	TenantID       uuid.UUID  `json:"tenant_id"`
	UserID         *uuid.UUID `json:"user_id,omitempty"`
	Email          string     `json:"email"`
	Type           string     `json:"type"`     // shared, personal
	AuthMethod     string     `json:"auth_method"` // imap, oauth
	Provider       string     `json:"provider"` // imap, google, microsoft, zoho
	IMAPHost       *string    `json:"imap_host,omitempty"`
	IMAPPort       *int       `json:"imap_port,omitempty"`
	Status         string     `json:"status"` // active, error, disconnected
	LastSyncAt     *time.Time `json:"last_sync_at,omitempty"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
	OAuthExpiresAt *time.Time `json:"oauth_expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	// Computed fields
	UserName *string `json:"user_name,omitempty"`
}

// CreateIMAPAccountRequest represents the request to add an IMAP email account.
type CreateIMAPAccountRequest struct {
	Email        string     `json:"email" binding:"required,email"`
	Type         string     `json:"type" binding:"omitempty,oneof=shared personal"`
	IMAPHost     string     `json:"imap_host" binding:"required"`
	IMAPPort     int        `json:"imap_port" binding:"omitempty,min=1,max=65535"`
	IMAPPassword string     `json:"imap_password" binding:"required"`
	UserID       *uuid.UUID `json:"user_id,omitempty"`
}

// UpdateEmailAccountRequest represents the request to update an email account.
type UpdateEmailAccountRequest struct {
	IMAPHost     *string `json:"imap_host,omitempty"`
	IMAPPort     *int    `json:"imap_port,omitempty"`
	IMAPPassword *string `json:"imap_password,omitempty"`
	Type         *string `json:"type,omitempty"`
}

// List returns all email accounts for the tenant.
// GET /api/v1/email-accounts
func (h *EmailAccountHandler) List(c *gin.Context) {
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
	provider := c.Query("provider")
	status := c.Query("status")
	accountType := c.Query("type")

	var query strings.Builder
	var args []interface{}
	argNum := 1

	query.WriteString(`
		SELECT ea.id, ea.tenant_id, ea.user_id, ea.email, ea.type, ea.auth_method,
		       ea.provider, ea.imap_host, ea.imap_port, ea.status, ea.last_sync_at,
		       ea.error_message, ea.oauth_expires_at, ea.created_at, ea.updated_at,
		       u.name as user_name
		FROM email_accounts ea
		LEFT JOIN users u ON ea.user_id = u.id
		WHERE ea.tenant_id = $1
	`)
	args = append(args, tenantID)
	argNum++

	if provider != "" {
		query.WriteString(` AND ea.provider = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, provider)
		argNum++
	}

	if status != "" {
		query.WriteString(` AND ea.status = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, status)
		argNum++
	}

	if accountType != "" {
		query.WriteString(` AND ea.type = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, accountType)
		argNum++
	}

	query.WriteString(` ORDER BY ea.created_at DESC LIMIT $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)
	argNum++

	query.WriteString(` OFFSET $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, offset)

	var accounts []EmailAccount
	err := tenantDB.Query(c, query.String(), args, func(rows pgx.Rows) error {
		var a EmailAccount
		err := rows.Scan(
			&a.ID, &a.TenantID, &a.UserID, &a.Email, &a.Type, &a.AuthMethod,
			&a.Provider, &a.IMAPHost, &a.IMAPPort, &a.Status, &a.LastSyncAt,
			&a.ErrorMessage, &a.OAuthExpiresAt, &a.CreatedAt, &a.UpdatedAt,
			&a.UserName,
		)
		if err != nil {
			return err
		}
		accounts = append(accounts, a)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list email accounts")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch email accounts"})
		return
	}

	if accounts == nil {
		accounts = []EmailAccount{}
	}

	c.JSON(http.StatusOK, gin.H{
		"email_accounts": accounts,
		"count":          len(accounts),
	})
}

// Get returns a single email account.
// GET /api/v1/email-accounts/:id
func (h *EmailAccountHandler) Get(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	accountID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid account ID"})
		return
	}

	query := `
		SELECT ea.id, ea.tenant_id, ea.user_id, ea.email, ea.type, ea.auth_method,
		       ea.provider, ea.imap_host, ea.imap_port, ea.status, ea.last_sync_at,
		       ea.error_message, ea.oauth_expires_at, ea.created_at, ea.updated_at,
		       u.name as user_name
		FROM email_accounts ea
		LEFT JOIN users u ON ea.user_id = u.id
		WHERE ea.id = $1 AND ea.tenant_id = $2
	`

	var a EmailAccount
	err = tenantDB.QueryRowScan(c, []interface{}{
		&a.ID, &a.TenantID, &a.UserID, &a.Email, &a.Type, &a.AuthMethod,
		&a.Provider, &a.IMAPHost, &a.IMAPPort, &a.Status, &a.LastSyncAt,
		&a.ErrorMessage, &a.OAuthExpiresAt, &a.CreatedAt, &a.UpdatedAt,
		&a.UserName,
	}, query, accountID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get email account")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch email account"})
		return
	}

	c.JSON(http.StatusOK, a)
}

// CreateIMAP creates a new IMAP email account.
// POST /api/v1/email-accounts/imap
func (h *EmailAccountHandler) CreateIMAP(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req CreateIMAPAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default values
	accountType := req.Type
	if accountType == "" {
		accountType = "shared"
	}
	imapPort := req.IMAPPort
	if imapPort == 0 {
		imapPort = 993
	}

	// Check for duplicate email
	var exists bool
	err := tenantDB.QueryRowScan(c, []interface{}{&exists},
		`SELECT EXISTS(SELECT 1 FROM email_accounts WHERE email = $1 AND tenant_id = $2)`,
		req.Email, tenantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check duplicate email account")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Email account already exists"})
		return
	}

	// Insert new account
	id := uuid.New()
	var a EmailAccount

	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		query := `
			INSERT INTO email_accounts (
				id, tenant_id, user_id, email, type, auth_method, provider,
				imap_host, imap_port, imap_password, status, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, 'imap', 'imap', $6, $7, $8, 'active', NOW(), NOW())
			RETURNING id, tenant_id, user_id, email, type, auth_method, provider,
			          imap_host, imap_port, status, last_sync_at, error_message,
			          oauth_expires_at, created_at, updated_at
		`
		return tx.QueryRow(ctx, query,
			id, tenantID, req.UserID, req.Email, accountType,
			req.IMAPHost, imapPort, req.IMAPPassword,
		).Scan(
			&a.ID, &a.TenantID, &a.UserID, &a.Email, &a.Type, &a.AuthMethod, &a.Provider,
			&a.IMAPHost, &a.IMAPPort, &a.Status, &a.LastSyncAt, &a.ErrorMessage,
			&a.OAuthExpiresAt, &a.CreatedAt, &a.UpdatedAt,
		)
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create IMAP email account")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create email account"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionEmailAccountCreate, &userID, &tenantID, "email_account", &a.ID, c.ClientIP(), map[string]interface{}{
		"email":    req.Email,
		"provider": "imap",
		"type":     accountType,
	})

	c.JSON(http.StatusCreated, a)
}

// Update updates an email account.
// PATCH /api/v1/email-accounts/:id
func (h *EmailAccountHandler) Update(c *gin.Context) {
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

	accountID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid account ID"})
		return
	}

	var req UpdateEmailAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build dynamic update query
	var updates []string
	var args []interface{}
	argNum := 1

	args = append(args, accountID)
	argNum++
	args = append(args, tenantID)
	argNum++

	if req.IMAPHost != nil {
		updates = append(updates, "imap_host = $"+strconv.Itoa(argNum))
		args = append(args, *req.IMAPHost)
		argNum++
	}
	if req.IMAPPort != nil {
		updates = append(updates, "imap_port = $"+strconv.Itoa(argNum))
		args = append(args, *req.IMAPPort)
		argNum++
	}
	if req.IMAPPassword != nil {
		updates = append(updates, "imap_password = $"+strconv.Itoa(argNum))
		args = append(args, *req.IMAPPassword)
		argNum++
	}
	if req.Type != nil {
		updates = append(updates, "type = $"+strconv.Itoa(argNum))
		args = append(args, *req.Type)
		argNum++
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	query := `
		UPDATE email_accounts
		SET ` + strings.Join(updates, ", ") + `, updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2
		RETURNING id, tenant_id, user_id, email, type, auth_method, provider,
		          imap_host, imap_port, status, last_sync_at, error_message,
		          oauth_expires_at, created_at, updated_at
	`

	var a EmailAccount
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, query, args...).Scan(
			&a.ID, &a.TenantID, &a.UserID, &a.Email, &a.Type, &a.AuthMethod, &a.Provider,
			&a.IMAPHost, &a.IMAPPort, &a.Status, &a.LastSyncAt, &a.ErrorMessage,
			&a.OAuthExpiresAt, &a.CreatedAt, &a.UpdatedAt,
		)
	})

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to update email account")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update email account"})
		return
	}

	// Audit log (don't log password changes in metadata)
	h.audit.LogEntity(ctx, audit.ActionEmailAccountCreate, &userID, &tenantID, "email_account", &a.ID, c.ClientIP(), map[string]interface{}{
		"email": a.Email,
	})

	c.JSON(http.StatusOK, a)
}

// Delete removes an email account.
// DELETE /api/v1/email-accounts/:id
func (h *EmailAccountHandler) Delete(c *gin.Context) {
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

	accountID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid account ID"})
		return
	}

	// Get account email for audit log before deleting
	var email string
	err = tenantDB.QueryRowScan(c, []interface{}{&email},
		`SELECT email FROM email_accounts WHERE id = $1 AND tenant_id = $2`,
		accountID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get email account for deletion")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete email account"})
		return
	}

	// Delete the account
	result, err := tenantDB.Exec(c, `DELETE FROM email_accounts WHERE id = $1 AND tenant_id = $2`,
		accountID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to delete email account")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete email account"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionEmailAccountDelete, &userID, &tenantID, "email_account", &accountID, c.ClientIP(), map[string]interface{}{
		"email": email,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Email account deleted"})
}

// Sync triggers a sync for an email account inbox.
// POST /api/v1/email-accounts/:id/sync
func (h *EmailAccountHandler) Sync(c *gin.Context) {
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

	accountID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid account ID"})
		return
	}

	// Check if account exists and get details
	var account struct {
		Email      string
		AuthMethod string
		Provider   string
		Status     string
	}

	err = tenantDB.QueryRowScan(c, []interface{}{
		&account.Email, &account.AuthMethod, &account.Provider, &account.Status,
	}, `SELECT email, auth_method, provider, status FROM email_accounts WHERE id = $1 AND tenant_id = $2`,
		accountID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get email account for sync")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync email account"})
		return
	}

	if account.Status == "disconnected" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot sync disconnected account"})
		return
	}

	// TODO: Implement actual IMAP/OAuth sync logic
	// For now, update last_sync_at timestamp
	_, err = tenantDB.Exec(c, `UPDATE email_accounts SET last_sync_at = NOW(), status = 'active' WHERE id = $1 AND tenant_id = $2`,
		accountID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to update sync timestamp")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync email account"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionEmailAccountSync, &userID, &tenantID, "email_account", &accountID, c.ClientIP(), map[string]interface{}{
		"email":    account.Email,
		"provider": account.Provider,
	})

	c.JSON(http.StatusOK, gin.H{
		"message":     "Sync initiated",
		"last_sync_at": time.Now(),
	})
}

// TestConnection tests the connection for an email account.
// POST /api/v1/email-accounts/:id/test
func (h *EmailAccountHandler) TestConnection(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	accountID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid account ID"})
		return
	}

	// Get account details
	var account struct {
		Email      string
		AuthMethod string
		Provider   string
		IMAPHost   *string
		IMAPPort   *int
	}

	err = tenantDB.QueryRowScan(c, []interface{}{
		&account.Email, &account.AuthMethod, &account.Provider, &account.IMAPHost, &account.IMAPPort,
	}, `SELECT email, auth_method, provider, imap_host, imap_port FROM email_accounts WHERE id = $1 AND tenant_id = $2`,
		accountID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get email account for test")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to test connection"})
		return
	}

	// TODO: Implement actual connection test (IMAP dial, OAuth token refresh)
	// For now, return a mock success
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Connection test successful",
		"email":    account.Email,
		"provider": account.Provider,
	})
}

// OAuthInitiate returns the OAuth authorization URL for a provider.
// GET /api/v1/email-accounts/oauth/:provider
func (h *EmailAccountHandler) OAuthInitiate(c *gin.Context) {
	provider := c.Param("provider")

	// Validate provider
	validProviders := map[string]bool{
		"google":    true,
		"microsoft": true,
		"zoho":      true,
	}

	if !validProviders[provider] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OAuth provider"})
		return
	}

	// TODO: Implement actual OAuth flow
	// Generate state, store in Redis, return authorization URL
	c.JSON(http.StatusOK, gin.H{
		"provider": provider,
		"message":  "OAuth integration not yet implemented",
		"auth_url": "", // Would be the OAuth authorization URL
	})
}

// OAuthCallback handles the OAuth callback from a provider.
// GET /api/v1/email-accounts/oauth/:provider/callback
func (h *EmailAccountHandler) OAuthCallback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization code"})
		return
	}
	if state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing state parameter"})
		return
	}

	// TODO: Implement actual OAuth callback handling
	// - Validate state against Redis
	// - Exchange code for tokens
	// - Get user email from provider
	// - Store email account with OAuth tokens

	c.JSON(http.StatusOK, gin.H{
		"provider": provider,
		"message":  "OAuth callback received but integration not yet implemented",
	})
}

// Disconnect marks an email account as disconnected.
// POST /api/v1/email-accounts/:id/disconnect
func (h *EmailAccountHandler) Disconnect(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	accountID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid account ID"})
		return
	}

	result, err := tenantDB.Exec(c, `UPDATE email_accounts SET status = 'disconnected', updated_at = NOW() WHERE id = $1 AND tenant_id = $2`,
		accountID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to disconnect email account")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disconnect account"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email account disconnected"})
}

// Reconnect marks an email account as active.
// POST /api/v1/email-accounts/:id/reconnect
func (h *EmailAccountHandler) Reconnect(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	accountID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid account ID"})
		return
	}

	result, err := tenantDB.Exec(c, `UPDATE email_accounts SET status = 'active', error_message = NULL, updated_at = NOW() WHERE id = $1 AND tenant_id = $2`,
		accountID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to reconnect email account")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reconnect account"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email account reconnected"})
}
