package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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

// ESignHandler handles e-signature operations.
type ESignHandler struct {
	db          *database.Pool
	audit       *audit.Logger
	email       *email.Client
	frontendURL string
}

// NewESignHandler creates a new e-sign handler.
func NewESignHandler(db *database.Pool, auditLogger *audit.Logger, emailClient *email.Client, frontendURL string) *ESignHandler {
	return &ESignHandler{
		db:          db,
		audit:       auditLogger,
		email:       emailClient,
		frontendURL: frontendURL,
	}
}

// ESignRequest represents an e-signature request.
type ESignRequest struct {
	ID                uuid.UUID              `json:"id"`
	TenantID          uuid.UUID              `json:"tenant_id"`
	ClientID          uuid.UUID              `json:"client_id"`
	DocumentID        *uuid.UUID             `json:"document_id,omitempty"`
	TemplateType      string                 `json:"template_type"`
	Status            string                 `json:"status"` // pending, signed, expired, declined
	SignerEmail       string                 `json:"signer_email"`
	SignerName        *string                `json:"signer_name,omitempty"`
	SigningToken      *string                `json:"signing_token,omitempty"`
	SentAt            *time.Time             `json:"sent_at,omitempty"`
	SignedAt          *time.Time             `json:"signed_at,omitempty"`
	ExpiresAt         *time.Time             `json:"expires_at,omitempty"`
	SignatureData     map[string]interface{} `json:"signature_data,omitempty"`
	AutoCreateService bool                   `json:"auto_create_service"`
	ServiceTypeID     *uuid.UUID             `json:"service_type_id,omitempty"`
	CreatedServiceID  *uuid.UUID             `json:"created_service_id,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	// Computed fields
	ClientName *string `json:"client_name,omitempty"`
}

// CreateESignRequest represents the request to create an e-signature request.
type CreateESignRequest struct {
	ClientID          uuid.UUID  `json:"client_id" binding:"required"`
	DocumentID        *uuid.UUID `json:"document_id,omitempty"`
	TemplateType      string     `json:"template_type" binding:"required"`
	SignerEmail       string     `json:"signer_email" binding:"required,email"`
	SignerName        *string    `json:"signer_name,omitempty"`
	ExpiresInDays     int        `json:"expires_in_days,omitempty"` // Default 14 days
	AutoCreateService bool       `json:"auto_create_service,omitempty"`
	ServiceTypeID     *uuid.UUID `json:"service_type_id,omitempty"`
}

// List returns all e-sign requests for the tenant.
// GET /api/v1/e-sign
func (h *ESignHandler) List(c *gin.Context) {
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
	status := c.Query("status")
	clientID := c.Query("client_id")

	query := `
		SELECT e.id, e.tenant_id, e.client_id, e.document_id, e.template_type,
		       e.status, e.signer_email, e.signer_name, e.sent_at, e.signed_at,
		       e.expires_at, e.auto_create_service, e.service_type_id,
		       e.created_service_id, e.created_at, c.company_name as client_name
		FROM e_sign_requests e
		LEFT JOIN clients c ON e.client_id = c.id
		WHERE e.tenant_id = $1
	`
	args := []interface{}{tenantID}
	argIdx := 2

	if status != "" {
		query += " AND e.status = $" + strconv.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}
	if clientID != "" {
		if cid, err := uuid.Parse(clientID); err == nil {
			query += " AND e.client_id = $" + strconv.Itoa(argIdx)
			args = append(args, cid)
			argIdx++
		}
	}

	query += " ORDER BY e.created_at DESC LIMIT $" + strconv.Itoa(argIdx) + " OFFSET $" + strconv.Itoa(argIdx+1)
	args = append(args, limit, offset)

	var requests []ESignRequest
	err := tenantDB.Query(c, query, args, func(rows pgx.Rows) error {
		var r ESignRequest
		err := rows.Scan(
			&r.ID, &r.TenantID, &r.ClientID, &r.DocumentID, &r.TemplateType,
			&r.Status, &r.SignerEmail, &r.SignerName, &r.SentAt, &r.SignedAt,
			&r.ExpiresAt, &r.AutoCreateService, &r.ServiceTypeID,
			&r.CreatedServiceID, &r.CreatedAt, &r.ClientName,
		)
		if err != nil {
			return err
		}
		requests = append(requests, r)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list e-sign requests")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch e-sign requests"})
		return
	}

	if requests == nil {
		requests = []ESignRequest{}
	}

	c.JSON(http.StatusOK, gin.H{
		"e_sign_requests": requests,
		"count":           len(requests),
	})
}

// Get returns a single e-sign request.
// GET /api/v1/e-sign/:id
func (h *ESignHandler) Get(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	requestID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	var r ESignRequest
	var signatureDataJSON []byte
	err = tenantDB.QueryRowScan(c, []interface{}{
		&r.ID, &r.TenantID, &r.ClientID, &r.DocumentID, &r.TemplateType,
		&r.Status, &r.SignerEmail, &r.SignerName, &r.SentAt, &r.SignedAt,
		&r.ExpiresAt, &signatureDataJSON, &r.AutoCreateService, &r.ServiceTypeID,
		&r.CreatedServiceID, &r.CreatedAt, &r.ClientName,
	}, `
		SELECT e.id, e.tenant_id, e.client_id, e.document_id, e.template_type,
		       e.status, e.signer_email, e.signer_name, e.sent_at, e.signed_at,
		       e.expires_at, e.signature_data, e.auto_create_service, e.service_type_id,
		       e.created_service_id, e.created_at, c.company_name as client_name
		FROM e_sign_requests e
		LEFT JOIN clients c ON e.client_id = c.id
		WHERE e.id = $1 AND e.tenant_id = $2
	`, requestID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "E-sign request not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get e-sign request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch e-sign request"})
		return
	}

	if len(signatureDataJSON) > 0 {
		json.Unmarshal(signatureDataJSON, &r.SignatureData)
	}

	c.JSON(http.StatusOK, gin.H{"e_sign_request": r})
}

// Create creates a new e-sign request.
// POST /api/v1/e-sign
func (h *ESignHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req CreateESignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify client exists
	var clientExists bool
	err := tenantDB.QueryRowScan(c, []interface{}{&clientExists},
		`SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1 AND tenant_id = $2)`,
		req.ClientID, tenantID)
	if err != nil || !clientExists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Client not found"})
		return
	}

	// Calculate expiry
	expiresInDays := req.ExpiresInDays
	if expiresInDays <= 0 {
		expiresInDays = 14
	}
	expiresAt := time.Now().AddDate(0, 0, expiresInDays)

	requestID := uuid.New()
	now := time.Now()

	_, err = tenantDB.Exec(c, `
		INSERT INTO e_sign_requests (
			id, tenant_id, client_id, document_id, template_type, status,
			signer_email, signer_name, expires_at, auto_create_service,
			service_type_id, created_at
		) VALUES ($1, $2, $3, $4, $5, 'pending', $6, $7, $8, $9, $10, $11)
	`, requestID, tenantID, req.ClientID, req.DocumentID, req.TemplateType,
		req.SignerEmail, req.SignerName, expiresAt, req.AutoCreateService,
		req.ServiceTypeID, now)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create e-sign request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create e-sign request"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionESignCreate, &userID, &tenantID, "e_sign_request", &requestID, c.ClientIP(), map[string]interface{}{
		"client_id":     req.ClientID.String(),
		"template_type": req.TemplateType,
		"signer_email":  req.SignerEmail,
	})

	c.JSON(http.StatusCreated, gin.H{
		"e_sign_request": ESignRequest{
			ID:                requestID,
			TenantID:          tenantID,
			ClientID:          req.ClientID,
			DocumentID:        req.DocumentID,
			TemplateType:      req.TemplateType,
			Status:            "pending",
			SignerEmail:       req.SignerEmail,
			SignerName:        req.SignerName,
			ExpiresAt:         &expiresAt,
			AutoCreateService: req.AutoCreateService,
			ServiceTypeID:     req.ServiceTypeID,
			CreatedAt:         now,
		},
	})
}

// Send sends the e-sign request to the signer via email.
// POST /api/v1/e-sign/:id/send
func (h *ESignHandler) Send(c *gin.Context) {
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

	requestID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	// Get request details for email
	var signerEmail, signerName, templateType string
	var clientName string
	var expiresAt *time.Time
	err = tenantDB.QueryRowScan(c, []interface{}{
		&signerEmail, &signerName, &templateType, &clientName, &expiresAt,
	}, `
		SELECT e.signer_email, COALESCE(e.signer_name, ''), e.template_type,
		       COALESCE(c.company_name, 'Client'), e.expires_at
		FROM e_sign_requests e
		LEFT JOIN clients c ON e.client_id = c.id
		WHERE e.id = $1 AND e.tenant_id = $2 AND e.status = 'pending'
	`, requestID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "E-sign request not found or already processed"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get e-sign request details")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send e-sign request"})
		return
	}

	// Generate a secure signing token
	signingToken := generateToken()
	now := time.Now()

	// Update the request with token and sent_at
	result, err := tenantDB.Exec(c, `
		UPDATE e_sign_requests SET sent_at = $1, signing_token = $2
		WHERE id = $3 AND tenant_id = $4 AND status = 'pending'
	`, now, signingToken, requestID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to update e-sign request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send e-sign request"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "E-sign request not found or already processed"})
		return
	}

	// Send email with signing link
	if h.email != nil && h.email.IsConfigured() {
		signingURL := fmt.Sprintf("%s/sign/%s", h.frontendURL, signingToken)
		displayName := signerName
		if displayName == "" {
			displayName = signerEmail
		}

		err = h.email.SendESignRequest(signerEmail, displayName, clientName, templateType, signingURL, expiresAt)
		if err != nil {
			log.Error().Err(err).Str("email", signerEmail).Msg("Failed to send e-sign email")
			// Don't fail the request, email is sent async in most cases
			// The signing token is already saved, they can be re-sent
		} else {
			log.Info().
				Str("email", signerEmail).
				Str("request_id", requestID.String()).
				Msg("E-sign request email sent")
		}
	} else {
		log.Warn().Msg("Email client not configured, e-sign email not sent")
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionESignSend, &userID, &tenantID, "e_sign_request", &requestID, c.ClientIP(), map[string]interface{}{
		"signer_email":  signerEmail,
		"template_type": templateType,
	})

	c.JSON(http.StatusOK, gin.H{
		"message":      "E-sign request sent successfully",
		"signing_url":  fmt.Sprintf("%s/sign/%s", h.frontendURL, signingToken),
		"signer_email": signerEmail,
	})
}

// Delete cancels an e-sign request.
// DELETE /api/v1/e-sign/:id
func (h *ESignHandler) Delete(c *gin.Context) {
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

	requestID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID"})
		return
	}

	result, err := tenantDB.Exec(c, `
		DELETE FROM e_sign_requests
		WHERE id = $1 AND tenant_id = $2 AND status = 'pending'
	`, requestID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to delete e-sign request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete e-sign request"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "E-sign request not found or cannot be deleted"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionESignDelete, &userID, &tenantID, "e_sign_request", &requestID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "E-sign request cancelled"})
}

// GetSigningPage returns the public signing page data.
// GET /api/v1/e-sign/sign/:token (public endpoint)
func (h *ESignHandler) GetSigningPage(c *gin.Context) {
	token := c.Param("token")

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing signing token"})
		return
	}

	ctx := c.Request.Context()

	// Query the e-sign request by token (across all tenants - public endpoint)
	// Use a transaction to set super_admin role to bypass RLS
	var request struct {
		ID           uuid.UUID  `json:"id"`
		TemplateType string     `json:"template_type"`
		Status       string     `json:"status"`
		SignerEmail  string     `json:"signer_email"`
		SignerName   *string    `json:"signer_name"`
		ExpiresAt    *time.Time `json:"expires_at"`
		ClientName   string     `json:"client_name"`
		TenantName   string     `json:"tenant_name"`
	}

	// Use a transaction to set session variables to bypass RLS
	tx, err := h.db.Begin(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to begin transaction")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load signing page"})
		return
	}
	defer tx.Rollback(ctx)

	// Set super_admin role and dummy tenant_id to bypass RLS for this public query
	// TODO: Remove hardcoded UUID when RLS policy is updated to handle empty tenant_id
	_, err = tx.Exec(ctx, "SET LOCAL app.role = 'super_admin'; SET LOCAL app.tenant_id = '00000000-0000-0000-0000-000000000000'")
	if err != nil {
		log.Error().Err(err).Msg("Failed to set session role")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load signing page"})
		return
	}

	err = tx.QueryRow(ctx, `
		SELECT e.id, e.template_type, e.status, e.signer_email, e.signer_name, e.expires_at,
		       COALESCE(c.company_name, 'Client') as client_name,
		       COALESCE(t.name, 'Organization') as tenant_name
		FROM e_sign_requests e
		LEFT JOIN clients c ON e.client_id = c.id
		LEFT JOIN tenants t ON e.tenant_id = t.id
		WHERE e.signing_token = $1
	`, token).Scan(
		&request.ID, &request.TemplateType, &request.Status,
		&request.SignerEmail, &request.SignerName, &request.ExpiresAt,
		&request.ClientName, &request.TenantName,
	)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or expired signing link"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get signing page data")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load signing page"})
		return
	}

	// Check if already signed or expired
	if request.Status == "signed" {
		c.JSON(http.StatusGone, gin.H{
			"error":   "already_signed",
			"message": "This document has already been signed",
		})
		return
	}

	if request.Status == "expired" || (request.ExpiresAt != nil && time.Now().After(*request.ExpiresAt)) {
		c.JSON(http.StatusGone, gin.H{
			"error":   "expired",
			"message": "This signing link has expired",
		})
		return
	}

	if request.Status == "declined" {
		c.JSON(http.StatusGone, gin.H{
			"error":   "declined",
			"message": "This document signing was declined",
		})
		return
	}

	// Format template type for display
	templateDisplay := request.TemplateType
	switch request.TemplateType {
	case "engagement":
		templateDisplay = "Engagement Letter"
	case "service_agreement":
		templateDisplay = "Service Agreement"
	case "gdpr_consent":
		templateDisplay = "GDPR Consent Form"
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id":     request.ID,
		"template_type":  request.TemplateType,
		"template_title": templateDisplay,
		"signer_email":   request.SignerEmail,
		"signer_name":    request.SignerName,
		"client_name":    request.ClientName,
		"tenant_name":    request.TenantName,
		"expires_at":     request.ExpiresAt,
	})
}

// SubmitSignature submits a signature for an e-sign request.
// POST /api/v1/e-sign/sign/:token (public endpoint)
func (h *ESignHandler) SubmitSignature(c *gin.Context) {
	token := c.Param("token")

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing signing token"})
		return
	}

	var req struct {
		SignatureData map[string]interface{} `json:"signature_data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Add metadata to signature data
	req.SignatureData["signed_at"] = time.Now().UTC().Format(time.RFC3339)
	req.SignatureData["ip_address"] = c.ClientIP()
	req.SignatureData["user_agent"] = c.GetHeader("User-Agent")

	signatureJSON, err := json.Marshal(req.SignatureData)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal signature data")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process signature"})
		return
	}

	ctx := c.Request.Context()

	// Use a transaction to set session variables to bypass RLS for public endpoint
	tx, err := h.db.Begin(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to begin transaction")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process signature"})
		return
	}
	defer tx.Rollback(ctx)

	// Set super_admin role and dummy tenant_id to bypass RLS for this public query
	// TODO: Remove hardcoded UUID when RLS policy is updated to handle empty tenant_id
	_, err = tx.Exec(ctx, "SET LOCAL app.role = 'super_admin'; SET LOCAL app.tenant_id = '00000000-0000-0000-0000-000000000000'")
	if err != nil {
		log.Error().Err(err).Msg("Failed to set session role")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process signature"})
		return
	}

	// Get request details first to check status and get IDs for auto-create service
	var requestID, tenantID, clientID uuid.UUID
	var status string
	var expiresAt *time.Time
	var autoCreateService bool
	var serviceTypeID *uuid.UUID

	err = tx.QueryRow(ctx, `
		SELECT id, tenant_id, client_id, status, expires_at, auto_create_service, service_type_id
		FROM e_sign_requests
		WHERE signing_token = $1
	`, token).Scan(
		&requestID, &tenantID, &clientID, &status, &expiresAt, &autoCreateService, &serviceTypeID,
	)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or expired signing link"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get e-sign request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process signature"})
		return
	}

	// Check if already signed or expired
	if status == "signed" {
		c.JSON(http.StatusConflict, gin.H{"error": "This document has already been signed"})
		return
	}
	if status == "expired" || (expiresAt != nil && time.Now().After(*expiresAt)) {
		c.JSON(http.StatusGone, gin.H{"error": "This signing link has expired"})
		return
	}
	if status == "declined" {
		c.JSON(http.StatusGone, gin.H{"error": "This document signing was declined"})
		return
	}

	// Update the request with signature
	now := time.Now()
	_, err = tx.Exec(ctx, `
		UPDATE e_sign_requests
		SET status = 'signed', signed_at = $1, signature_data = $2, signing_token = NULL
		WHERE signing_token = $3 AND status = 'pending'
	`, now, signatureJSON, token)

	if err != nil {
		log.Error().Err(err).Msg("Failed to update e-sign request with signature")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save signature"})
		return
	}

	// Auto-create service if enabled
	var createdServiceID *uuid.UUID
	if autoCreateService && serviceTypeID != nil {
		serviceID := uuid.New()
		_, err = tx.Exec(ctx, `
			INSERT INTO services (id, tenant_id, client_id, service_type_id, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'new', NOW(), NOW())
		`, serviceID, tenantID, clientID, serviceTypeID)

		if err != nil {
			log.Error().Err(err).Msg("Failed to auto-create service after signing")
			// Don't fail the signing, just log the error
		} else {
			createdServiceID = &serviceID
			// Update the e-sign request with created service ID
			tx.Exec(ctx, `UPDATE e_sign_requests SET created_service_id = $1 WHERE id = $2`, serviceID, requestID)
			log.Info().Str("service_id", serviceID.String()).Msg("Auto-created service after e-sign")
		}
	}

	// Commit the transaction
	if err = tx.Commit(ctx); err != nil {
		log.Error().Err(err).Msg("Failed to commit transaction")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save signature"})
		return
	}

	// Audit log
	h.audit.LogEntity(c.Request.Context(), audit.ActionESignSigned, nil, &tenantID, "e_sign_request", &requestID, c.ClientIP(), map[string]interface{}{
		"auto_created_service": createdServiceID != nil,
	})

	log.Info().
		Str("request_id", requestID.String()).
		Str("ip", c.ClientIP()).
		Msg("Document signed successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":            "Document signed successfully",
		"signed_at":          now,
		"created_service_id": createdServiceID,
	})
}

// generateToken generates a secure random token.
func generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
