package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/audit"
	"github.com/accountant-crm/go-backend/internal/cache"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
	"github.com/accountant-crm/go-backend/internal/storage"
)

// Magic byte signatures for file type validation
// These are the first few bytes that identify file types
var magicBytes = map[string][]byte{
	"application/pdf":  {0x25, 0x50, 0x44, 0x46}, // %PDF
	"image/jpeg":       {0xFF, 0xD8, 0xFF},
	"image/png":        {0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
	"image/gif":        {0x47, 0x49, 0x46, 0x38}, // GIF8
	"application/zip":  {0x50, 0x4B, 0x03, 0x04},
	"image/webp":       {0x52, 0x49, 0x46, 0x46}, // RIFF (need to check WEBP later)
	"application/msword": {0xD0, 0xCF, 0x11, 0xE0}, // OLE compound doc
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {0x50, 0x4B, 0x03, 0x04}, // DOCX (ZIP-based)
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       {0x50, 0x4B, 0x03, 0x04}, // XLSX (ZIP-based)
	"application/vnd.ms-excel": {0xD0, 0xCF, 0x11, 0xE0}, // XLS (OLE)
	"text/csv":                 nil,                      // No magic bytes for text files
	"text/plain":               nil,
}

// Allowed MIME types for document uploads
var allowedMimeTypes = map[string]bool{
	"application/pdf":  true,
	"image/jpeg":       true,
	"image/png":        true,
	"image/gif":        true,
	"image/webp":       true,
	"application/zip":  true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
	"application/vnd.ms-excel": true,
	"text/csv":                 true,
	"text/plain":               true,
}

// validateMagicBytes checks if the file content matches the claimed MIME type
func validateMagicBytes(data []byte, claimedMime string) bool {
	expected, exists := magicBytes[claimedMime]

	// No magic bytes to check for text files
	if expected == nil || !exists {
		// For text files, do a basic check for binary content
		if strings.HasPrefix(claimedMime, "text/") {
			// Check first 512 bytes for binary content
			checkLen := 512
			if len(data) < checkLen {
				checkLen = len(data)
			}
			for i := 0; i < checkLen; i++ {
				// Allow common text characters
				if data[i] < 0x09 || (data[i] > 0x0D && data[i] < 0x20 && data[i] != 0x1B) {
					if data[i] != 0x00 { // Allow UTF-16 BOM
						return false // Binary content found
					}
				}
			}
		}
		return true
	}

	if len(data) < len(expected) {
		return false
	}

	return bytes.Equal(data[:len(expected)], expected)
}

// MaxUploadSize is the maximum file size (50MB)
const MaxUploadSize = 50 * 1024 * 1024

type DocumentHandler struct {
	db    *database.Pool
	audit *audit.Logger
	redis *cache.Client
	oss   *storage.OSSClient
}

func NewDocumentHandler(db *database.Pool, auditLogger *audit.Logger, redisClient *cache.Client, ossClient *storage.OSSClient) *DocumentHandler {
	return &DocumentHandler{
		db:    db,
		audit: auditLogger,
		redis: redisClient,
		oss:   ossClient,
	}
}

// Document represents a document record
type Document struct {
	ID           uuid.UUID  `json:"id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	ClientID     *uuid.UUID `json:"client_id,omitempty"`
	ServiceID    *uuid.UUID `json:"service_id,omitempty"`
	UploadedBy   *uuid.UUID `json:"uploaded_by,omitempty"`
	TypeID       *uuid.UUID `json:"type_id,omitempty"`
	Name         string     `json:"name"`
	OriginalName string     `json:"original_name"`
	FilePath     *string    `json:"file_path,omitempty"`
	FileSize     *int       `json:"file_size,omitempty"`
	MimeType     *string    `json:"mime_type,omitempty"`
	Status       string     `json:"status"`
	Access       string     `json:"access"`
	Version      int        `json:"version"`
	ParentID     *uuid.UUID `json:"parent_id,omitempty"`
	RequestedAt  *time.Time `json:"requested_at,omitempty"`
	ExpiryDate   *string    `json:"expiry_date,omitempty"`
	RequestNote  *string    `json:"request_note,omitempty"`
	UploadNote   *string    `json:"upload_note,omitempty"`
	ReviewNote   *string    `json:"review_note,omitempty"`
	ReviewedBy   *uuid.UUID `json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty"`
	ChaseCount   int        `json:"chase_count"`
	LastChasedAt *time.Time `json:"last_chased_at,omitempty"`
	AISummary    *string    `json:"ai_summary,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	// Joined fields
	ClientName   *string `json:"client_name,omitempty"`
	TypeName     *string `json:"type_name,omitempty"`
	UploadedName *string `json:"uploaded_by_name,omitempty"`
}

type CreateDocumentRequest struct {
	ClientID    *string `json:"client_id,omitempty"`
	ServiceID   *string `json:"service_id,omitempty"`
	TypeID      *string `json:"type_id,omitempty"`
	Name        string  `json:"name" binding:"required"`
	ExpiryDate  *string `json:"expiry_date,omitempty"`
	RequestNote *string `json:"request_note,omitempty"`
	Status      *string `json:"status,omitempty"`
}

type UpdateDocumentRequest struct {
	Name        *string `json:"name,omitempty"`
	TypeID      *string `json:"type_id,omitempty"`
	ExpiryDate  *string `json:"expiry_date,omitempty"`
	RequestNote *string `json:"request_note,omitempty"`
	UploadNote  *string `json:"upload_note,omitempty"`
	Status      *string `json:"status,omitempty"`
}

type GenerateUploadURLRequest struct {
	ClientID    *string `json:"client_id,omitempty"`
	ServiceID   *string `json:"service_id,omitempty"`
	TypeID      *string `json:"type_id,omitempty"`
	Name        string  `json:"name" binding:"required"`
	ExpiryDate  *string `json:"expiry_date,omitempty"`
	RequestNote *string `json:"request_note,omitempty"`
}

// List returns all documents for the tenant (staff-scoped)
// GET /api/v1/documents
func (h *DocumentHandler) List(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := c.Get(middleware.AuthRole)

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
	status := c.Query("status")
	clientID := c.Query("client_id")
	typeID := c.Query("type_id")
	search := c.Query("search")

	roleStr, _ := role.(string)
	isSuperAdmin := roleStr == "super_admin"

	var query strings.Builder
	var args []interface{}
	argNum := 1

	// Don't fetch ai_extracted (TOAST performance)
	query.WriteString(`
		SELECT d.id, d.tenant_id, d.client_id, d.service_id, d.uploaded_by, d.type_id,
		       d.name, d.original_name, d.file_size, d.mime_type, d.status, d.access,
		       d.version, d.expiry_date, d.chase_count, d.last_chased_at, d.ai_summary,
		       d.created_at, d.updated_at,
		       c.company_name as client_name, dt.name as type_name, u.name as uploaded_by_name
		FROM documents d
		LEFT JOIN clients c ON d.client_id = c.id
		LEFT JOIN document_types dt ON d.type_id = dt.id
		LEFT JOIN users u ON d.uploaded_by = u.id
	`)
	if isSuperAdmin {
		query.WriteString(`WHERE d.client_id IS NOT NULL`)
	} else {
		query.WriteString(`WHERE d.tenant_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(` AND d.client_id IS NOT NULL`)
		args = append(args, tenantID)
		argNum++
	}

	// Staff scoping - only see documents for assigned clients unless admin
	if roleStr == "staff" {
		query.WriteString(` AND d.client_id IN (SELECT client_id FROM staff_clients WHERE staff_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, userID)
		argNum++
	}

	// Status filter
	if status != "" {
		query.WriteString(` AND d.status = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, status)
		argNum++
	}

	// Client filter
	if clientID != "" {
		query.WriteString(` AND d.client_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, clientID)
		argNum++
	}

	// Type filter
	if typeID != "" {
		query.WriteString(` AND d.type_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, typeID)
		argNum++
	}

	// Search filter
	if search != "" {
		query.WriteString(` AND (d.name ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(` OR d.original_name ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, "%"+search+"%")
		argNum++
	}

	query.WriteString(` ORDER BY d.created_at DESC LIMIT $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)
	argNum++

	query.WriteString(` OFFSET $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, offset)

	documents := []Document{}
	err := tenantDB.Query(c, query.String(), args, func(rows pgx.Rows) error {
		var doc Document
		var expiryDate *time.Time
		err := rows.Scan(
			&doc.ID, &doc.TenantID, &doc.ClientID, &doc.ServiceID, &doc.UploadedBy, &doc.TypeID,
			&doc.Name, &doc.OriginalName, &doc.FileSize, &doc.MimeType, &doc.Status, &doc.Access,
			&doc.Version, &expiryDate, &doc.ChaseCount, &doc.LastChasedAt, &doc.AISummary,
			&doc.CreatedAt, &doc.UpdatedAt, &doc.ClientName, &doc.TypeName, &doc.UploadedName,
		)
		if err != nil {
			return err
		}
		if expiryDate != nil {
			s := expiryDate.Format("2006-01-02")
			doc.ExpiryDate = &s
		}
		documents = append(documents, doc)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list documents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": documents,
		"limit":     limit,
		"offset":    offset,
	})
}

// Get returns a single document by ID
// GET /api/v1/documents/:id
func (h *DocumentHandler) Get(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	var doc Document
	var expiryDate *time.Time
	err = tenantDB.QueryRowScan(c, []interface{}{
		&doc.ID, &doc.TenantID, &doc.ClientID, &doc.ServiceID, &doc.UploadedBy, &doc.TypeID,
		&doc.Name, &doc.OriginalName, &doc.FilePath, &doc.FileSize, &doc.MimeType, &doc.Status, &doc.Access,
		&doc.Version, &doc.ParentID, &doc.RequestedAt, &expiryDate, &doc.RequestNote,
		&doc.UploadNote, &doc.ReviewNote, &doc.ReviewedBy, &doc.ReviewedAt,
		&doc.ChaseCount, &doc.LastChasedAt, &doc.AISummary, &doc.CreatedAt, &doc.UpdatedAt,
		&doc.ClientName, &doc.TypeName, &doc.UploadedName,
	}, `
		SELECT d.id, d.tenant_id, d.client_id, d.service_id, d.uploaded_by, d.type_id,
		       d.name, d.original_name, d.file_path, d.file_size, d.mime_type, d.status, d.access,
		       d.version, d.parent_id, d.requested_at, d.expiry_date, d.request_note,
		       d.upload_note, d.review_note, d.reviewed_by, d.reviewed_at,
		       d.chase_count, d.last_chased_at, d.ai_summary, d.created_at, d.updated_at,
		       c.company_name as client_name, dt.name as type_name, u.name as uploaded_by_name
		FROM documents d
		LEFT JOIN clients c ON d.client_id = c.id
		LEFT JOIN document_types dt ON d.type_id = dt.id
		LEFT JOIN users u ON d.uploaded_by = u.id
		WHERE d.id = $1 AND d.tenant_id = $2
	`, documentID, tenantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found"})
			return
		}
		log.Error().Err(err).Msg("Failed to get document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if expiryDate != nil {
		s := expiryDate.Format("2006-01-02")
		doc.ExpiryDate = &s
	}

	c.JSON(http.StatusOK, doc)
}

// Create creates a new document (metadata only)
// POST /api/v1/documents
func (h *DocumentHandler) Create(c *gin.Context) {
	var req CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

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

	documentID := uuid.New()

	// Parse optional UUIDs
	var clientID, serviceID, typeID *uuid.UUID
	if req.ClientID != nil && *req.ClientID != "" {
		cid, _ := uuid.Parse(*req.ClientID)
		clientID = &cid
	}
	if req.ServiceID != nil && *req.ServiceID != "" {
		s, _ := uuid.Parse(*req.ServiceID)
		serviceID = &s
	}
	if req.TypeID != nil && *req.TypeID != "" {
		t, _ := uuid.Parse(*req.TypeID)
		typeID = &t
	}

	// Parse expiry date
	var expiryDate *time.Time
	if req.ExpiryDate != nil && *req.ExpiryDate != "" {
		t, err := time.Parse("2006-01-02", *req.ExpiryDate)
		if err == nil {
			expiryDate = &t
		}
	}

	status := "requested"
	if req.Status != nil {
		status = *req.Status
	}

	err := tenantDB.Transaction(c, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO documents (
				id, tenant_id, client_id, service_id, uploaded_by, type_id,
				name, original_name, status, access, expiry_date, request_note, requested_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $7, $8, 'all_staff', $9, $10, NOW()
			)
		`, documentID, tenantID, clientID, serviceID, userID, typeID,
			req.Name, status, expiryDate, req.RequestNote)
		return err
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentCreate, &userID, &tenantID, "document", &documentID, c.ClientIP(), nil)

	c.JSON(http.StatusCreated, gin.H{
		"id":      documentID,
		"message": "Document created successfully",
	})
}

// Update updates an existing document
// PATCH /api/v1/documents/:id
func (h *DocumentHandler) Update(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	var req UpdateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

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

	// Build dynamic update query
	var setClauses []string
	var args []interface{}
	argNum := 1

	if req.Name != nil {
		setClauses = append(setClauses, "name = $"+strconv.Itoa(argNum))
		args = append(args, *req.Name)
		argNum++
	}
	if req.TypeID != nil {
		var typeID *uuid.UUID
		if *req.TypeID != "" {
			t, _ := uuid.Parse(*req.TypeID)
			typeID = &t
		}
		setClauses = append(setClauses, "type_id = $"+strconv.Itoa(argNum))
		args = append(args, typeID)
		argNum++
	}
	if req.ExpiryDate != nil {
		var expiryDate *time.Time
		if *req.ExpiryDate != "" {
			t, _ := time.Parse("2006-01-02", *req.ExpiryDate)
			expiryDate = &t
		}
		setClauses = append(setClauses, "expiry_date = $"+strconv.Itoa(argNum))
		args = append(args, expiryDate)
		argNum++
	}
	if req.RequestNote != nil {
		setClauses = append(setClauses, "request_note = $"+strconv.Itoa(argNum))
		args = append(args, *req.RequestNote)
		argNum++
	}
	if req.UploadNote != nil {
		setClauses = append(setClauses, "upload_note = $"+strconv.Itoa(argNum))
		args = append(args, *req.UploadNote)
		argNum++
	}
	if req.Status != nil {
		setClauses = append(setClauses, "status = $"+strconv.Itoa(argNum))
		args = append(args, *req.Status)
		argNum++
	}

	if len(setClauses) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no_fields_to_update"})
		return
	}

	setClauses = append(setClauses, "updated_at = NOW()")

	query := "UPDATE documents SET " + strings.Join(setClauses, ", ") +
		" WHERE id = $" + strconv.Itoa(argNum) + " AND tenant_id = $" + strconv.Itoa(argNum+1)
	args = append(args, documentID, tenantID)

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, query, args...)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to update document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentUpdate, &userID, &tenantID, "document", &documentID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Document updated successfully"})
}

// Approve approves a document
// POST /api/v1/documents/:id/approve
func (h *DocumentHandler) Approve(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	var req struct {
		Note string `json:"note,omitempty"`
	}
	_ = c.ShouldBindJSON(&req)

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

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE documents SET status = 'approved', review_note = $1,
			       reviewed_by = $2, reviewed_at = NOW(), updated_at = NOW()
			WHERE id = $3 AND tenant_id = $4
		`, req.Note, userID, documentID, tenantID)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to approve document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentApprove, &userID, &tenantID, "document", &documentID, c.ClientIP(), nil)

	// Publish real-time event
	if h.redis != nil {
		event := cache.NewEvent(cache.EventDocApproved, tenantID, "document", &documentID).
			WithUser(userID).
			WithData("document_id", documentID.String())
		if err := h.redis.Publish(ctx, event); err != nil {
			log.Warn().Err(err).Msg("Failed to publish doc_approved event")
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Document approved successfully"})
}

// Reject rejects a document
// POST /api/v1/documents/:id/reject
func (h *DocumentHandler) Reject(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	var req struct {
		Note string `json:"note" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

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

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE documents SET status = 'rejected', review_note = $1,
			       reviewed_by = $2, reviewed_at = NOW(), updated_at = NOW()
			WHERE id = $3 AND tenant_id = $4
		`, req.Note, userID, documentID, tenantID)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to reject document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentReject, &userID, &tenantID, "document", &documentID, c.ClientIP(), nil)

	// Publish real-time event
	if h.redis != nil {
		event := cache.NewEvent(cache.EventDocRejected, tenantID, "document", &documentID).
			WithUser(userID).
			WithData("document_id", documentID.String()).
			WithData("reason", req.Note)
		if err := h.redis.Publish(ctx, event); err != nil {
			log.Warn().Err(err).Msg("Failed to publish doc_rejected event")
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Document rejected successfully"})
}

// RequestRenewal requests a renewal for an expiring document
// POST /api/v1/documents/:id/request-renewal
func (h *DocumentHandler) RequestRenewal(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	var req struct {
		Note string `json:"note,omitempty"`
	}
	_ = c.ShouldBindJSON(&req)

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE documents SET
				renewal_requested = true,
				renewal_requested_at = NOW(),
				renewal_requested_by = $1,
				renewal_note = $2,
				updated_at = NOW()
			WHERE id = $3 AND tenant_id = $4 AND renewal_requested = false
		`, userID, req.Note, documentID, tenantID)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to request document renewal")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found_or_already_requested"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, "document_renewal_request", &userID, &tenantID, "document", &documentID, c.ClientIP(), nil)

	// Publish real-time event
	if h.redis != nil {
		event := cache.NewEvent(cache.EventDocRenewal, tenantID, "document", &documentID).
			WithUser(userID).
			WithData("document_id", documentID.String())
		if err := h.redis.Publish(ctx, event); err != nil {
			log.Warn().Err(err).Msg("Failed to publish doc_renewal_requested event")
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Renewal requested successfully"})
}

// CancelRenewal cancels a pending renewal request
// DELETE /api/v1/documents/:id/renewal
func (h *DocumentHandler) CancelRenewal(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE documents SET
				renewal_requested = false,
				renewal_requested_at = NULL,
				renewal_requested_by = NULL,
				renewal_note = NULL,
				updated_at = NOW()
			WHERE id = $1 AND tenant_id = $2 AND renewal_requested = true
		`, documentID, tenantID)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to cancel document renewal")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found_or_not_pending_renewal"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, "document_renewal_cancel", &userID, &tenantID, "document", &documentID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Renewal cancelled successfully"})
}

// Delete soft-deletes a document
// DELETE /api/v1/documents/:id
func (h *DocumentHandler) Delete(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := middleware.GetRole(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var rowsAffected int64
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		var sql string
		var args []interface{}

		// Super admin can delete any document
		if role == "super_admin" {
			sql = `
				UPDATE documents SET deleted_at = NOW(), updated_at = NOW()
				WHERE id = $1 AND deleted_at IS NULL
			`
			args = []interface{}{documentID}
		} else {
			sql = `
				UPDATE documents SET deleted_at = NOW(), updated_at = NOW()
				WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
			`
			args = []interface{}{documentID, tenantID}
		}

		result, err := tx.Exec(ctx, sql, args...)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to delete document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentDelete, &userID, &tenantID, "document", &documentID, c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{"message": "Document deleted successfully"})
}

// ListExpiring returns documents that are expiring soon
// GET /api/v1/documents/expiring
func (h *DocumentHandler) ListExpiring(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Default to 30 days
	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}

	var documents []map[string]interface{}
	err := tenantDB.Query(c, `
		SELECT d.id, d.name, d.expiry_date, d.status, d.renewal_requested,
		       c.id as client_id, c.company_name,
		       (d.expiry_date - CURRENT_DATE) as days_until_expiry
		FROM documents d
		LEFT JOIN clients c ON d.client_id = c.id
		WHERE d.tenant_id = $1
		  AND d.expiry_date IS NOT NULL
		  AND d.expiry_date <= CURRENT_DATE + INTERVAL '1 day' * $2
		  AND d.expiry_date >= CURRENT_DATE
		  AND d.status = 'approved'
		ORDER BY d.expiry_date ASC
	`, []interface{}{tenantID, days}, func(rows pgx.Rows) error {
		var id uuid.UUID
		var name string
		var expiryDate time.Time
		var status string
		var renewalRequested bool
		var clientID *uuid.UUID
		var clientName *string
		var daysUntil int

		if err := rows.Scan(&id, &name, &expiryDate, &status, &renewalRequested,
			&clientID, &clientName, &daysUntil); err != nil {
			return err
		}

		doc := map[string]interface{}{
			"id":                id,
			"name":              name,
			"expiry_date":       expiryDate.Format("2006-01-02"),
			"status":            status,
			"renewal_requested": renewalRequested,
			"days_until_expiry": int(daysUntil),
		}
		if clientID != nil {
			doc["client_id"] = clientID
		}
		if clientName != nil {
			doc["client_name"] = clientName
		}

		documents = append(documents, doc)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list expiring documents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if documents == nil {
		documents = []map[string]interface{}{}
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": documents,
		"count":     len(documents),
		"days":      days,
	})
}

// GetVersions returns version history for a document
// GET /api/v1/documents/:id/versions
func (h *DocumentHandler) GetVersions(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	tenantID, _ := middleware.GetTenantID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var versions []map[string]interface{}
	err = tenantDB.Query(c, `
		WITH RECURSIVE version_chain AS (
			SELECT id, parent_id, version, name, file_size, created_at, uploaded_by
			FROM documents
			WHERE id = $1 AND tenant_id = $2
			UNION ALL
			SELECT d.id, d.parent_id, d.version, d.name, d.file_size, d.created_at, d.uploaded_by
			FROM documents d
			INNER JOIN version_chain vc ON d.id = vc.parent_id
			WHERE d.tenant_id = $2
		)
		SELECT vc.id, vc.version, vc.name, vc.file_size, vc.created_at, u.name as uploaded_by_name
		FROM version_chain vc
		LEFT JOIN users u ON vc.uploaded_by = u.id
		ORDER BY vc.version DESC
	`, []interface{}{documentID, tenantID}, func(rows pgx.Rows) error {
		var id uuid.UUID
		var version int
		var name string
		var fileSize *int
		var createdAt time.Time
		var uploadedByName *string

		if err := rows.Scan(&id, &version, &name, &fileSize, &createdAt, &uploadedByName); err != nil {
			return err
		}

		versions = append(versions, map[string]interface{}{
			"id":               id,
			"version":          version,
			"name":             name,
			"file_size":        fileSize,
			"created_at":       createdAt,
			"uploaded_by_name": uploadedByName,
		})
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get document versions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"versions": versions})
}

// RestoreVersion restores a previous version
// POST /api/v1/documents/:id/versions/:versionId/restore
func (h *DocumentHandler) RestoreVersion(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	versionID, err := uuid.Parse(c.Param("versionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_version_id"})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Get current document version
	var currentVersion int
	err = tenantDB.QueryRowScan(c, []interface{}{&currentVersion}, `
		SELECT version FROM documents WHERE id = $1 AND tenant_id = $2
	`, documentID, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found"})
		return
	}

	// Fix #4: Get the version to restore, validating it belongs to this document's version chain
	// A version is valid if: it IS the parent document, OR its parent_id matches the document
	var oldFilePath, oldMimeType *string
	var oldFileSize *int
	err = tenantDB.QueryRowScan(c, []interface{}{&oldFilePath, &oldFileSize, &oldMimeType}, `
		SELECT file_path, file_size, mime_type FROM documents
		WHERE id = $1 AND tenant_id = $2
		  AND (id = $3 OR parent_id = $3)
	`, versionID, tenantID, documentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "version_not_found_or_not_in_chain"})
		return
	}

	// Create new version with restored content
	newID := uuid.New()
	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO documents (
				id, tenant_id, client_id, service_id, uploaded_by, type_id,
				name, original_name, file_path, file_size, mime_type, status, access,
				version, parent_id
			)
			SELECT $1, tenant_id, client_id, service_id, $2, type_id,
			       name, original_name, $3, $4, $5, 'uploaded', access,
			       $6, $7
			FROM documents WHERE id = $7 AND tenant_id = $8
		`, newID, userID, oldFilePath, oldFileSize, oldMimeType, currentVersion+1, documentID, tenantID)
		return err
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to restore document version")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentUpdate, &userID, &tenantID, "document", &newID, c.ClientIP(), map[string]interface{}{
		"action":           "version_restored",
		"restored_from_id": versionID,
	})

	c.JSON(http.StatusOK, gin.H{
		"id":      newID,
		"message": "Version restored successfully",
	})
}

// BulkRequest requests multiple documents at once
// POST /api/v1/documents/bulk-request
func (h *DocumentHandler) BulkRequest(c *gin.Context) {
	var req struct {
		ClientID string   `json:"client_id" binding:"required,uuid"`
		TypeIDs  []string `json:"type_ids" binding:"required"`
		Note     *string  `json:"note,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	clientID, _ := uuid.Parse(req.ClientID)
	var created []uuid.UUID

	for _, typeIDStr := range req.TypeIDs {
		typeID, err := uuid.Parse(typeIDStr)
		if err != nil {
			continue
		}

		// Get type name for document name
		var typeName string
		err = tenantDB.QueryRowScan(c, []interface{}{&typeName}, `SELECT name FROM document_types WHERE id = $1`, typeID)
		if err != nil {
			continue
		}

		documentID := uuid.New()
		err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO documents (
					id, tenant_id, client_id, uploaded_by, type_id,
					name, original_name, status, access, request_note, requested_at
				) VALUES (
					$1, $2, $3, $4, $5, $6, $6, 'requested', 'all_staff', $7, NOW()
				)
			`, documentID, tenantID, clientID, userID, typeID, typeName, req.Note)
			return err
		})

		if err != nil {
			log.Error().Err(err).Str("type_id", typeIDStr).Msg("Failed to create document request")
			continue
		}

		created = append(created, documentID)
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentCreate, &userID, &tenantID, "document", nil, c.ClientIP(), map[string]interface{}{
		"bulk":      true,
		"count":     len(created),
		"client_id": req.ClientID,
	})

	c.JSON(http.StatusCreated, gin.H{
		"created": created,
		"count":   len(created),
		"message": "Document requests created successfully",
	})
}

// BulkApprove approves multiple documents at once
// POST /api/v1/documents/bulk-approve
func (h *DocumentHandler) BulkApprove(c *gin.Context) {
	var req struct {
		DocumentIDs []string `json:"document_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var documentIDs []uuid.UUID
	for _, id := range req.DocumentIDs {
		if uid, err := uuid.Parse(id); err == nil {
			documentIDs = append(documentIDs, uid)
		}
	}

	result, err := tenantDB.Exec(c, `
		UPDATE documents SET status = 'approved',
		       reviewed_by = $1, reviewed_at = NOW(), updated_at = NOW()
		WHERE id = ANY($2) AND tenant_id = $3 AND status = 'pending_review'
	`, userID, documentIDs, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to bulk approve documents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentApprove, &userID, &tenantID, "document", nil, c.ClientIP(), map[string]interface{}{
		"bulk":  true,
		"count": result.RowsAffected(),
	})

	c.JSON(http.StatusOK, gin.H{
		"approved": result.RowsAffected(),
		"message":  "Documents approved successfully",
	})
}

// ListFirm returns firm-level documents (no client_id)
// GET /api/v1/documents/firm
func (h *DocumentHandler) ListFirm(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := c.Get(middleware.AuthRole)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit > 100 {
		limit = 100
	}

	var query strings.Builder
	var args []interface{}
	argNum := 1

	query.WriteString(`
		SELECT d.id, d.name, d.original_name, d.file_size, d.mime_type, d.status, d.access,
		       d.created_at, d.updated_at, u.name as uploaded_by_name
		FROM documents d
		LEFT JOIN users u ON d.uploaded_by = u.id
		WHERE d.tenant_id = $1 AND d.client_id IS NULL
	`)
	args = append(args, tenantID)
	argNum++

	// Check access permissions for non-admin
	roleStr, _ := role.(string)
	if roleStr != "tenant_admin" && roleStr != "super_admin" {
		query.WriteString(` AND (d.access = 'all_staff' OR d.access = 'admin' AND EXISTS (
			SELECT 1 FROM document_access da WHERE da.document_id = d.id AND da.staff_id = $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`))`)
		args = append(args, userID)
		argNum++
	}

	query.WriteString(` ORDER BY d.created_at DESC LIMIT $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)
	argNum++

	query.WriteString(` OFFSET $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, offset)

	var documents []map[string]interface{}
	err := tenantDB.Query(c, query.String(), args, func(rows pgx.Rows) error {
		var id uuid.UUID
		var name, originalName, status, access string
		var fileSize *int
		var mimeType, uploadedByName *string
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&id, &name, &originalName, &fileSize, &mimeType, &status, &access,
			&createdAt, &updatedAt, &uploadedByName); err != nil {
			return err
		}

		documents = append(documents, map[string]interface{}{
			"id":               id,
			"name":             name,
			"original_name":    originalName,
			"file_size":        fileSize,
			"mime_type":        mimeType,
			"status":           status,
			"access":           access,
			"created_at":       createdAt,
			"updated_at":       updatedAt,
			"uploaded_by_name": uploadedByName,
		})
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list firm documents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": documents,
		"limit":     limit,
		"offset":    offset,
	})
}

// GenerateUploadURL creates a pending document record and returns an upload URL.
// POST /api/v1/documents/upload-url
func (h *DocumentHandler) GenerateUploadURL(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req GenerateUploadURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}

	var clientID, serviceID, typeID *uuid.UUID
	if req.ClientID != nil {
		id, err := uuid.Parse(*req.ClientID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_id"})
			return
		}
		clientID = &id
	}
	if req.ServiceID != nil {
		id, err := uuid.Parse(*req.ServiceID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_service_id"})
			return
		}
		serviceID = &id
	}
	if req.TypeID != nil {
		id, err := uuid.Parse(*req.TypeID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_type_id"})
			return
		}
		typeID = &id
	}

	docID := uuid.New()
	var doc Document
	err := tenantDB.Transaction(c, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO documents (id, tenant_id, client_id, service_id, type_id, name, original_name, status, access, requested_at, request_note, expiry_date, uploaded_by)
			VALUES ($1, $2, $3, $4, $5, $6, $6, 'pending_upload', 'private', NOW(), $7, $8, $9)
			RETURNING id, tenant_id, client_id, service_id, type_id, name, original_name, status, access, version, requested_at, request_note, expiry_date, created_at, updated_at
		`, docID, tenantID, clientID, serviceID, typeID, req.Name, req.RequestNote, req.ExpiryDate, userID).Scan(
			&doc.ID, &doc.TenantID, &doc.ClientID, &doc.ServiceID, &doc.TypeID, &doc.Name, &doc.OriginalName,
			&doc.Status, &doc.Access, &doc.Version, &doc.RequestedAt, &doc.RequestNote, &doc.ExpiryDate, &doc.CreatedAt, &doc.UpdatedAt,
		)
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create pending document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	h.audit.LogEntity(ctx, audit.ActionDocumentCreate, &userID, &tenantID, "document", &doc.ID, c.ClientIP(), map[string]interface{}{
		"name":   req.Name,
		"status": "pending_upload",
	})

	c.JSON(http.StatusCreated, gin.H{
		"document":   doc,
		"upload_url": fmt.Sprintf("/api/v1/documents/%s/upload", doc.ID.String()),
	})
}

// GenerateQRToken generates a secure upload token for QR code
// POST /api/v1/documents/qr
func (h *DocumentHandler) GenerateQRToken(c *gin.Context) {
	var req struct {
		ClientID  string   `json:"client_id" binding:"required,uuid"`
		TypeIDs   []string `json:"type_ids,omitempty"`
		Note      *string  `json:"note,omitempty"`
		ExpiresIn *int     `json:"expires_in,omitempty"` // minutes
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation_error", "message": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	clientID, _ := uuid.Parse(req.ClientID)

	// Generate secure token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	// Default expiry: 24 hours
	expiresIn := 24 * 60 // minutes
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		expiresIn = *req.ExpiresIn
	}

	tokenID := uuid.New()
	err := tenantDB.Transaction(c, func(tx pgx.Tx) error {
		// Get client name for the token (stored for public lookup without join)
		var clientName string
		err := tx.QueryRow(ctx, `SELECT company_name FROM clients WHERE id = $1`, clientID).Scan(&clientName)
		if err != nil {
			return fmt.Errorf("client not found: %w", err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO upload_tokens (id, tenant_id, client_id, client_name, token, created_by, note, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + $8 * INTERVAL '1 minute')
		`, tokenID, tenantID, clientID, clientName, token, userID, req.Note, expiresIn)
		return err
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create upload token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token":      token,
		"expires_in": expiresIn,
		"upload_url": "/api/v1/documents/qr/" + token + "/upload",
	})
}

// GetQRToken validates and returns QR token info (PUBLIC)
// GET /api/v1/documents/qr/:token
func (h *DocumentHandler) GetQRToken(c *gin.Context) {
	token := c.Param("token")
	ctx := c.Request.Context()

	var clientName string
	var expiresAt time.Time
	var note *string

	// Use SuperAdminTransaction to bypass RLS for public endpoint
	err := h.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT client_name, expires_at, note
			FROM upload_tokens
			WHERE token = $1 AND expires_at > NOW() AND used_at IS NULL
		`, token).Scan(&clientName, &expiresAt, &note)
	})

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invalid_or_expired_token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":       true,
		"client_name": clientName,
		"expires_at":  expiresAt,
		"note":        note,
	})
}

// Upload handles file upload for a document
// POST /api/v1/documents/:id/upload
func (h *DocumentHandler) Upload(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

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

	// Check file size limit (50MB)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadSize)

	// Parse multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		if err.Error() == "http: request body too large" {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":   "file_too_large",
				"message": fmt.Sprintf("File size exceeds maximum allowed size of %d MB", MaxUploadSize/(1024*1024)),
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "no_file_provided", "message": err.Error()})
		return
	}
	defer file.Close()

	// Read file content for validation
	fileContent, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_read_file"})
		return
	}

	// Detect MIME type from content
	detectedMime := http.DetectContentType(fileContent)

	// Get claimed MIME type from header
	claimedMime := header.Header.Get("Content-Type")
	if claimedMime == "" {
		claimedMime = detectedMime
	}

	// Validate MIME type is allowed
	if !allowedMimeTypes[detectedMime] && !allowedMimeTypes[claimedMime] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_file_type",
			"message": fmt.Sprintf("File type '%s' is not allowed", detectedMime),
		})
		return
	}

	// Validate magic bytes match claimed type
	if !validateMagicBytes(fileContent, detectedMime) {
		log.Warn().
			Str("document_id", documentID.String()).
			Str("claimed_mime", claimedMime).
			Str("detected_mime", detectedMime).
			Msg("Magic byte validation failed - possible file type spoofing")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_file_content",
			"message": "File content does not match its declared type",
		})
		return
	}

	// Verify document exists and belongs to tenant
	var currentStatus string
	err = tenantDB.QueryRowScan(c, []interface{}{&currentStatus}, `
		SELECT status FROM documents WHERE id = $1 AND tenant_id = $2
	`, documentID, tenantID)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found"})
			return
		}
		log.Error().Err(err).Msg("Failed to get document")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Generate secure file path
	// Format: tenant_id/year/month/document_id_random.ext
	now := time.Now()
	randomSuffix := make([]byte, 8)
	rand.Read(randomSuffix)
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		// Fallback extension based on MIME type
		switch detectedMime {
		case "application/pdf":
			ext = ".pdf"
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/gif":
			ext = ".gif"
		default:
			ext = ".bin"
		}
	}

	filePath := fmt.Sprintf("%s/%d/%02d/%s_%s%s",
		tenantID.String(),
		now.Year(),
		now.Month(),
		documentID.String(),
		hex.EncodeToString(randomSuffix),
		ext,
	)

	fileSize := len(fileContent)

	// Upload to OSS if configured
	if h.oss != nil && h.oss.IsConfigured() {
		if err := h.oss.Upload(ctx, filePath, fileContent, detectedMime); err != nil {
			log.Error().Err(err).Str("document_id", documentID.String()).Msg("Failed to upload file to OSS")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "storage_error", "message": "Failed to upload file to storage"})
			return
		}
		log.Info().Str("document_id", documentID.String()).Str("path", filePath).Int("size", fileSize).Msg("File uploaded to OSS")
	} else {
		log.Warn().Str("document_id", documentID.String()).Msg("OSS not configured - file metadata stored but content not persisted")
	}
	originalName := header.Filename

	err = tenantDB.Transaction(c, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE documents SET
				file_path = $1,
				file_size = $2,
				mime_type = $3,
				original_name = $4,
				status = 'uploaded',
				uploaded_by = $5,
				updated_at = NOW()
			WHERE id = $6 AND tenant_id = $7
		`, filePath, fileSize, detectedMime, originalName, userID, documentID, tenantID)
		return err
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to update document with file info")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentUpload, &userID, &tenantID, "document", &documentID, c.ClientIP(), map[string]interface{}{
		"file_size":     fileSize,
		"mime_type":     detectedMime,
		"original_name": originalName,
	})

	// Publish real-time event
	if h.redis != nil {
		event := cache.NewEvent(cache.EventDocUploaded, tenantID, "document", &documentID).
			WithUser(userID).
			WithData("document_id", documentID.String()).
			WithData("file_size", fileSize).
			WithData("mime_type", detectedMime)
		if err := h.redis.Publish(ctx, event); err != nil {
			log.Warn().Err(err).Msg("Failed to publish doc_uploaded event")
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "File uploaded successfully",
		"document_id":   documentID,
		"file_size":     fileSize,
		"mime_type":     detectedMime,
		"original_name": originalName,
	})
}

// Download generates a signed URL for document download
// GET /api/v1/documents/:id/download
func (h *DocumentHandler) Download(c *gin.Context) {
	documentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_document_id"})
		return
	}

	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Get document metadata
	var filePath, mimeType, originalName *string
	var fileSize *int
	err = tenantDB.QueryRowScan(c, []interface{}{&filePath, &mimeType, &originalName, &fileSize}, `
		SELECT file_path, mime_type, original_name, file_size
		FROM documents
		WHERE id = $1 AND tenant_id = $2
	`, documentID, tenantID)

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "document_not_found"})
			return
		}
		log.Error().Err(err).Msg("Failed to get document for download")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	if filePath == nil || *filePath == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "no_file",
			"message": "Document has no uploaded file",
		})
		return
	}

	// Audit the download request
	h.audit.LogEntity(c.Request.Context(), audit.ActionDocumentDownload, &userID, &tenantID, "document", &documentID, c.ClientIP(), nil)

	// Generate OSS signed URL if configured
	if h.oss != nil && h.oss.IsConfigured() {
		signedURL, err := h.oss.GetSignedURL(c.Request.Context(), *filePath, 15*time.Minute)
		if err != nil {
			log.Error().Err(err).Str("document_id", documentID.String()).Msg("Failed to generate signed URL")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "storage_error", "message": "Failed to generate download URL"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"document_id":   documentID,
			"mime_type":     mimeType,
			"original_name": originalName,
			"file_size":     fileSize,
			"download_url":  signedURL,
			"expires_in":    900, // 15 minutes in seconds
		})
		return
	}

	// Fallback: OSS not configured - return proxied URL
	// Fix #43: Don't expose internal file_path - use opaque document ID in URL
	c.JSON(http.StatusOK, gin.H{
		"document_id":   documentID,
		"mime_type":     mimeType,
		"original_name": originalName,
		"file_size":     fileSize,
		"download_url":  fmt.Sprintf("/api/v1/documents/%s/stream", documentID), // Proxied URL - no path leakage
		"expires_in":    900, // 15 minutes in seconds
		"message":       "OSS not configured - using proxy URL",
	})
}

// UploadViaQR handles file upload via QR token (PUBLIC - no auth required)
// POST /api/v1/documents/qr/:token/upload
func (h *DocumentHandler) UploadViaQR(c *gin.Context) {
	token := c.Param("token")
	ctx := c.Request.Context()

	// Validate token and get associated info
	// Use SuperAdminTransaction to bypass RLS for public endpoint
	var tokenID, tenantID, clientID, createdBy uuid.UUID
	var expiresAt time.Time
	err := h.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT id, tenant_id, client_id, created_by, expires_at
			FROM upload_tokens
			WHERE token = $1 AND expires_at > NOW() AND used_at IS NULL
		`, token).Scan(&tokenID, &tenantID, &clientID, &createdBy, &expiresAt)
	})

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invalid_or_expired_token"})
		return
	}

	// Check file size limit
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadSize)

	// Parse multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		if err.Error() == "http: request body too large" {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":   "file_too_large",
				"message": fmt.Sprintf("File size exceeds maximum allowed size of %d MB", MaxUploadSize/(1024*1024)),
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "no_file_provided"})
		return
	}
	defer file.Close()

	// Read and validate file content
	fileContent, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_to_read_file"})
		return
	}

	detectedMime := http.DetectContentType(fileContent)

	if !allowedMimeTypes[detectedMime] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_file_type",
			"message": fmt.Sprintf("File type '%s' is not allowed", detectedMime),
		})
		return
	}

	if !validateMagicBytes(fileContent, detectedMime) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_file_content",
			"message": "File content does not match its declared type",
		})
		return
	}

	// Generate file path
	now := time.Now()
	randomSuffix := make([]byte, 8)
	rand.Read(randomSuffix)
	ext := filepath.Ext(header.Filename)
	if ext == "" {
		ext = ".bin"
	}

	documentID := uuid.New()
	filePath := fmt.Sprintf("%s/%d/%02d/%s_%s%s",
		tenantID.String(),
		now.Year(),
		now.Month(),
		documentID.String(),
		hex.EncodeToString(randomSuffix),
		ext,
	)

	fileSize := len(fileContent)
	originalName := header.Filename

	// Upload to OSS if configured
	if h.oss != nil && h.oss.IsConfigured() {
		if err := h.oss.Upload(ctx, filePath, fileContent, detectedMime); err != nil {
			log.Error().Err(err).Str("document_id", documentID.String()).Msg("Failed to upload QR file to OSS")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "storage_error", "message": "Failed to upload file to storage"})
			return
		}
		log.Info().Str("document_id", documentID.String()).Str("path", filePath).Int("size", fileSize).Msg("QR file uploaded to OSS")
	} else {
		log.Warn().Str("document_id", documentID.String()).Msg("OSS not configured - QR file metadata stored but content not persisted")
	}

	// Create document record using TenantTransaction with the token's tenant context
	err = h.db.TenantTransaction(ctx, tenantID.String(), "staff", func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO documents (
				id, tenant_id, client_id, uploaded_by,
				name, original_name, file_path, file_size, mime_type,
				status, access
			) VALUES (
				$1, $2, $3, $4, $5, $5, $6, $7, $8, 'pending_review', 'all_staff'
			)
		`, documentID, tenantID, clientID, createdBy, originalName, filePath, fileSize, detectedMime)
		if err != nil {
			return err
		}

		// Fix #2: Mark token as used to prevent replay attacks
		// Single-use tokens are invalidated immediately after successful upload
		_, err = tx.Exec(ctx, `UPDATE upload_tokens SET used_at = NOW() WHERE id = $1`, tokenID)
		return err
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to create document from QR upload")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionDocumentUpload, &createdBy, &tenantID, "document", &documentID, c.ClientIP(), map[string]interface{}{
		"via":           "qr_upload",
		"file_size":     fileSize,
		"mime_type":     detectedMime,
		"original_name": originalName,
	})

	c.JSON(http.StatusCreated, gin.H{
		"message":       "File uploaded successfully",
		"document_id":   documentID,
		"file_size":     fileSize,
		"mime_type":     detectedMime,
		"original_name": originalName,
	})
}
