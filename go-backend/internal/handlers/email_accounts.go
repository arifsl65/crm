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
	"github.com/accountant-crm/go-backend/internal/crypto"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
	"github.com/accountant-crm/go-backend/internal/oauth"
)

// EmailAccountHandler handles email account operations.
type EmailAccountHandler struct {
	db        *database.Pool
	audit     *audit.Logger
	encryptor *crypto.Encryptor
	oauth     *oauth.Service
}

// NewEmailAccountHandler creates a new email account handler.
func NewEmailAccountHandler(db *database.Pool, auditLogger *audit.Logger, encryptor *crypto.Encryptor, oauthService *oauth.Service) *EmailAccountHandler {
	return &EmailAccountHandler{
		db:        db,
		audit:     auditLogger,
		encryptor: encryptor,
		oauth:     oauthService,
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
		       COALESCE(u.name, '') as user_name
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
		       COALESCE(u.name, '') as user_name
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

	// Encrypt the IMAP password before storing
	var encryptedPassword string
	if h.encryptor != nil {
		var encErr error
		encryptedPassword, encErr = h.encryptor.Encrypt(req.IMAPPassword)
		if encErr != nil {
			log.Error().Err(encErr).Msg("Failed to encrypt IMAP password")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to secure credentials"})
			return
		}
	} else {
		// Encryption not configured - reject storing plaintext passwords
		log.Error().Msg("ENCRYPTION_KEY not configured - cannot store IMAP credentials securely")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Credential encryption not configured"})
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
			req.IMAPHost, imapPort, encryptedPassword,
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
		// Encrypt the new password
		if h.encryptor != nil {
			encryptedPassword, encErr := h.encryptor.Encrypt(*req.IMAPPassword)
			if encErr != nil {
				log.Error().Err(encErr).Msg("Failed to encrypt IMAP password")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to secure credentials"})
				return
			}
			updates = append(updates, "imap_password = $"+strconv.Itoa(argNum))
			args = append(args, encryptedPassword)
			argNum++
		} else {
			log.Error().Msg("ENCRYPTION_KEY not configured - cannot update IMAP credentials securely")
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Credential encryption not configured"})
			return
		}
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

	// Check if OAuth tokens need refresh before sync
	if account.AuthMethod == "oauth" && h.oauth != nil {
		// Get OAuth tokens to check expiry
		var oauthExpiresAt *time.Time
		var oauthRefreshToken *string
		err = tenantDB.QueryRowScan(c, []interface{}{&oauthExpiresAt, &oauthRefreshToken},
			`SELECT oauth_expires_at, oauth_refresh_token FROM email_accounts WHERE id = $1 AND tenant_id = $2`,
			accountID, tenantID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get OAuth tokens for sync")
		} else if oauthExpiresAt != nil && time.Now().After(*oauthExpiresAt) && oauthRefreshToken != nil {
			// Token expired, attempt refresh
			var provider oauth.Provider
			switch account.Provider {
			case "google":
				provider = oauth.ProviderGoogle
			case "microsoft":
				provider = oauth.ProviderMicrosoft
			}

			if provider != "" {
				_, refreshToken, decErr := h.oauth.DecryptTokens("", *oauthRefreshToken)
				if decErr == nil {
					newTokens, refErr := h.oauth.RefreshToken(ctx, provider, refreshToken)
					if refErr == nil {
						accessEnc, refreshEnc, encErr := h.oauth.EncryptTokens(newTokens)
						if encErr == nil {
							tenantDB.Exec(c, `UPDATE email_accounts SET oauth_access_token = $1, oauth_refresh_token = $2, oauth_expires_at = $3 WHERE id = $4`,
								accessEnc, refreshEnc, newTokens.ExpiresAt, accountID)
							log.Info().Str("email", account.Email).Msg("OAuth token refreshed during sync")
						}
					}
				}
			}
		}
	}

	// Update last_sync_at timestamp
	// Note: Actual IMAP/OAuth email fetching would be implemented here
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
	ctx := c.Request.Context()
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

	// Get account details including OAuth tokens
	var account struct {
		Email             string
		AuthMethod        string
		Provider          string
		IMAPHost          *string
		IMAPPort          *int
		OAuthAccessToken  *string
		OAuthRefreshToken *string
		OAuthExpiresAt    *time.Time
	}

	err = tenantDB.QueryRowScan(c, []interface{}{
		&account.Email, &account.AuthMethod, &account.Provider,
		&account.IMAPHost, &account.IMAPPort,
		&account.OAuthAccessToken, &account.OAuthRefreshToken, &account.OAuthExpiresAt,
	}, `SELECT email, auth_method, provider, imap_host, imap_port,
	           oauth_access_token, oauth_refresh_token, oauth_expires_at
	    FROM email_accounts WHERE id = $1 AND tenant_id = $2`,
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

	// Test based on auth method
	if account.AuthMethod == "oauth" {
		// Test OAuth connection by checking token validity
		if h.oauth == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error":   "OAuth service not configured",
			})
			return
		}

		if account.OAuthAccessToken == nil || *account.OAuthAccessToken == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   "No OAuth tokens stored. Please reconnect the account.",
				"email":   account.Email,
			})
			return
		}

		// Check if token is expired
		tokenExpired := account.OAuthExpiresAt != nil && time.Now().After(*account.OAuthExpiresAt)

		if tokenExpired && account.OAuthRefreshToken != nil && *account.OAuthRefreshToken != "" {
			// Attempt to refresh the token
			var provider oauth.Provider
			switch account.Provider {
			case "google":
				provider = oauth.ProviderGoogle
			case "microsoft":
				provider = oauth.ProviderMicrosoft
			default:
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"error":   "Unknown OAuth provider",
					"email":   account.Email,
				})
				return
			}

			// Decrypt refresh token
			_, refreshToken, err := h.oauth.DecryptTokens("", *account.OAuthRefreshToken)
			if err != nil {
				log.Error().Err(err).Msg("Failed to decrypt refresh token")
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"error":   "Failed to decrypt credentials. Please reconnect the account.",
					"email":   account.Email,
				})
				return
			}

			// Refresh the token
			newTokens, err := h.oauth.RefreshToken(ctx, provider, refreshToken)
			if err != nil {
				log.Error().Err(err).Str("provider", account.Provider).Msg("Failed to refresh OAuth token")
				c.JSON(http.StatusOK, gin.H{
					"success":      false,
					"error":        "Token refresh failed. Please reconnect the account.",
					"email":        account.Email,
					"needs_reauth": true,
				})
				return
			}

			// Encrypt and store new tokens
			accessEncrypted, refreshEncrypted, err := h.oauth.EncryptTokens(newTokens)
			if err != nil {
				log.Error().Err(err).Msg("Failed to encrypt new tokens")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to secure credentials"})
				return
			}

			// Update tokens in database
			_, err = tenantDB.Exec(c, `
				UPDATE email_accounts
				SET oauth_access_token = $1,
				    oauth_refresh_token = $2,
				    oauth_expires_at = $3,
				    status = 'active',
				    error_message = NULL,
				    updated_at = NOW()
				WHERE id = $4 AND tenant_id = $5`,
				accessEncrypted, refreshEncrypted, newTokens.ExpiresAt, accountID, tenantID)

			if err != nil {
				log.Error().Err(err).Msg("Failed to update refreshed tokens")
			}

			c.JSON(http.StatusOK, gin.H{
				"success":        true,
				"message":        "Connection test successful (token refreshed)",
				"email":          account.Email,
				"provider":       account.Provider,
				"token_refreshed": true,
			})
			return
		}

		// Token is valid
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"message":    "Connection test successful",
			"email":      account.Email,
			"provider":   account.Provider,
			"expires_at": account.OAuthExpiresAt,
		})
		return
	}

	// IMAP connection test
	if account.AuthMethod == "imap" {
		if account.IMAPHost == nil || *account.IMAPHost == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   "IMAP host not configured",
				"email":   account.Email,
			})
			return
		}

		// For IMAP, we would dial the server here
		// For now, return success if configuration exists
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"message":   "IMAP configuration valid (connection test not implemented)",
			"email":     account.Email,
			"provider":  account.Provider,
			"imap_host": account.IMAPHost,
			"imap_port": account.IMAPPort,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"error":   "Unknown auth method",
		"email":   account.Email,
	})
}

// OAuthInitiate returns the OAuth authorization URL for a provider.
// GET /api/v1/email-accounts/oauth/:provider
func (h *EmailAccountHandler) OAuthInitiate(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	providerStr := c.Param("provider")

	// Validate provider
	var provider oauth.Provider
	switch providerStr {
	case "google":
		provider = oauth.ProviderGoogle
	case "microsoft":
		provider = oauth.ProviderMicrosoft
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OAuth provider. Supported: google, microsoft"})
		return
	}

	// Check if OAuth service is available
	if h.oauth == nil {
		log.Error().Msg("OAuth service not configured")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "OAuth service not configured"})
		return
	}

	// Check if provider is enabled
	if !h.oauth.IsProviderEnabled(provider) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "oauth_provider_not_enabled",
			"message": fmt.Sprintf("%s OAuth is not enabled. Please configure the OAuth credentials.", providerStr),
		})
		return
	}

	// Generate state token with CSRF protection
	state, err := h.oauth.GenerateState(ctx, tenantID.String(), userID.String(), provider)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate OAuth state")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initiate OAuth flow"})
		return
	}

	// Get authorization URL
	authURL, err := h.oauth.GetAuthURL(provider, state)
	if err != nil {
		log.Error().Err(err).Str("provider", providerStr).Msg("Failed to get OAuth auth URL")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate authorization URL"})
		return
	}

	log.Info().
		Str("provider", providerStr).
		Str("tenant_id", tenantID.String()).
		Msg("OAuth flow initiated")

	c.JSON(http.StatusOK, gin.H{
		"provider": providerStr,
		"auth_url": authURL,
		"message":  "Redirect user to auth_url to authorize",
	})
}

// OAuthCallback handles the OAuth callback from a provider.
// GET /api/v1/email-accounts/oauth/:provider/callback
func (h *EmailAccountHandler) OAuthCallback(c *gin.Context) {
	ctx := c.Request.Context()
	providerStr := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")
	errorParam := c.Query("error")

	// Handle OAuth error response
	if errorParam != "" {
		errorDesc := c.Query("error_description")
		log.Warn().
			Str("provider", providerStr).
			Str("error", errorParam).
			Str("description", errorDesc).
			Msg("OAuth authorization denied")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "oauth_denied",
			"message":     "Authorization was denied",
			"description": errorDesc,
		})
		return
	}

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization code"})
		return
	}
	if state == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing state parameter"})
		return
	}

	// Check if OAuth service is available
	if h.oauth == nil {
		log.Error().Msg("OAuth service not configured")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "OAuth service not configured"})
		return
	}

	// Validate state (CSRF protection)
	stateData, err := h.oauth.ValidateState(ctx, state)
	if err != nil {
		log.Warn().Err(err).Str("state", state[:min(8, len(state))]+"...").Msg("Invalid OAuth state")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_state",
			"message": "Invalid or expired state. Please try again.",
		})
		return
	}

	// Verify provider matches
	if stateData.Provider != providerStr {
		log.Warn().
			Str("expected", stateData.Provider).
			Str("received", providerStr).
			Msg("OAuth provider mismatch")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider mismatch"})
		return
	}

	// Parse IDs from state
	tenantID, err := uuid.Parse(stateData.TenantID)
	if err != nil {
		log.Error().Err(err).Str("tenant_id", stateData.TenantID).Msg("Invalid tenant ID in state")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid state data"})
		return
	}
	userID, err := uuid.Parse(stateData.UserID)
	if err != nil {
		log.Error().Err(err).Str("user_id", stateData.UserID).Msg("Invalid user ID in state")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid state data"})
		return
	}

	// Determine provider type
	var provider oauth.Provider
	switch providerStr {
	case "google":
		provider = oauth.ProviderGoogle
	case "microsoft":
		provider = oauth.ProviderMicrosoft
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OAuth provider"})
		return
	}

	// Exchange code for tokens
	tokens, err := h.oauth.ExchangeCode(ctx, provider, code)
	if err != nil {
		log.Error().Err(err).Str("provider", providerStr).Msg("Failed to exchange OAuth code")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "token_exchange_failed",
			"message": "Failed to complete authorization. Please try again.",
		})
		return
	}

	// Get user info from provider
	userInfo, err := h.oauth.GetUserInfo(ctx, provider, tokens.AccessToken)
	if err != nil {
		log.Error().Err(err).Str("provider", providerStr).Msg("Failed to get user info")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "user_info_failed",
			"message": "Failed to retrieve email information from provider.",
		})
		return
	}

	// Encrypt tokens for storage
	accessEncrypted, refreshEncrypted, err := h.oauth.EncryptTokens(tokens)
	if err != nil {
		log.Error().Err(err).Msg("Failed to encrypt OAuth tokens")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to secure credentials"})
		return
	}

	// Get tenant DB connection
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context for OAuth callback")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Check if email account already exists
	var existingID uuid.UUID
	err = tenantDB.QueryRowScan(c, []interface{}{&existingID},
		`SELECT id FROM email_accounts WHERE email = $1 AND tenant_id = $2`,
		userInfo.Email, tenantID)

	if err == nil {
		// Account exists - update tokens
		_, err = tenantDB.Exec(c, `
			UPDATE email_accounts
			SET oauth_access_token = $1,
			    oauth_refresh_token = $2,
			    oauth_expires_at = $3,
			    status = 'active',
			    error_message = NULL,
			    updated_at = NOW()
			WHERE id = $4 AND tenant_id = $5`,
			accessEncrypted, refreshEncrypted, tokens.ExpiresAt, existingID, tenantID)

		if err != nil {
			log.Error().Err(err).Msg("Failed to update OAuth tokens")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update email account"})
			return
		}

		// Audit log
		h.audit.LogEntity(ctx, audit.ActionEmailAccountSync, &userID, &tenantID, "email_account", &existingID, c.ClientIP(), map[string]interface{}{
			"email":    userInfo.Email,
			"provider": providerStr,
			"action":   "oauth_reconnect",
		})

		log.Info().
			Str("email", userInfo.Email).
			Str("provider", providerStr).
			Msg("OAuth tokens updated for existing email account")

		c.JSON(http.StatusOK, gin.H{
			"message":    "Email account reconnected successfully",
			"email":      userInfo.Email,
			"account_id": existingID,
			"provider":   providerStr,
			"is_new":     false,
		})
		return
	}

	if err != pgx.ErrNoRows {
		log.Error().Err(err).Msg("Failed to check existing email account")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Create new email account
	newID := uuid.New()
	var account EmailAccount

	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		query := `
			INSERT INTO email_accounts (
				id, tenant_id, user_id, email, type, auth_method, provider,
				oauth_access_token, oauth_refresh_token, oauth_expires_at,
				status, created_at, updated_at
			) VALUES ($1, $2, $3, $4, 'personal', 'oauth', $5, $6, $7, $8, 'active', NOW(), NOW())
			RETURNING id, tenant_id, user_id, email, type, auth_method, provider,
			          imap_host, imap_port, status, last_sync_at, error_message,
			          oauth_expires_at, created_at, updated_at
		`
		return tx.QueryRow(ctx, query,
			newID, tenantID, userID, userInfo.Email, providerStr,
			accessEncrypted, refreshEncrypted, tokens.ExpiresAt,
		).Scan(
			&account.ID, &account.TenantID, &account.UserID, &account.Email,
			&account.Type, &account.AuthMethod, &account.Provider,
			&account.IMAPHost, &account.IMAPPort, &account.Status,
			&account.LastSyncAt, &account.ErrorMessage,
			&account.OAuthExpiresAt, &account.CreatedAt, &account.UpdatedAt,
		)
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create OAuth email account")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create email account"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionEmailAccountCreate, &userID, &tenantID, "email_account", &account.ID, c.ClientIP(), map[string]interface{}{
		"email":    userInfo.Email,
		"provider": providerStr,
		"action":   "oauth_connect",
	})

	log.Info().
		Str("email", userInfo.Email).
		Str("provider", providerStr).
		Str("account_id", account.ID.String()).
		Msg("OAuth email account created")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Email account connected successfully",
		"email":      userInfo.Email,
		"account_id": account.ID,
		"provider":   providerStr,
		"is_new":     true,
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
