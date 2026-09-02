package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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

// ESignHandler handles e-signature operations.
type ESignHandler struct {
	db    *database.Pool
	audit *audit.Logger
}

// NewESignHandler creates a new e-sign handler.
func NewESignHandler(db *database.Pool, auditLogger *audit.Logger) *ESignHandler {
	return &ESignHandler{
		db:    db,
		audit: auditLogger,
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

	// Update sent_at
	now := time.Now()
	result, err := tenantDB.Exec(c, `
		UPDATE e_sign_requests SET sent_at = $1
		WHERE id = $2 AND tenant_id = $3 AND status = 'pending'
	`, now, requestID, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to update e-sign request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send e-sign request"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "E-sign request not found or already processed"})
		return
	}

	// TODO: Send email with signing link
	// For now, just mark as sent

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionESignSend, &userID, &tenantID, "e_sign_request", &requestID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "E-sign request sent successfully"})
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

	// In a real implementation, the token would be stored and validated
	// For now, we just return a placeholder
	c.JSON(http.StatusOK, gin.H{
		"message": "Signing page endpoint - token: " + token,
	})
}

// SubmitSignature submits a signature for an e-sign request.
// POST /api/v1/e-sign/sign/:token (public endpoint)
func (h *ESignHandler) SubmitSignature(c *gin.Context) {
	token := c.Param("token")

	var req struct {
		SignatureData map[string]interface{} `json:"signature_data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// In a real implementation, validate token and update the request
	c.JSON(http.StatusOK, gin.H{
		"message": "Signature submitted - token: " + token,
	})
}

// generateToken generates a secure random token.
func generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
