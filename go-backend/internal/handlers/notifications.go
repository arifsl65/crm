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

// NotificationHandler handles notification operations.
type NotificationHandler struct {
	db    *database.Pool
	audit *audit.Logger
}

// NewNotificationHandler creates a new notification handler.
func NewNotificationHandler(db *database.Pool, auditLogger *audit.Logger) *NotificationHandler {
	return &NotificationHandler{
		db:    db,
		audit: auditLogger,
	}
}

// Notification represents a notification record.
type Notification struct {
	ID          uuid.UUID  `json:"id"`
	TenantID    uuid.UUID  `json:"tenant_id"`
	UserID      uuid.UUID  `json:"user_id"`
	Type        string     `json:"type"` // document, deadline, email, system, reminder
	Title       string     `json:"title"`
	Message     string     `json:"message"`
	EntityType  *string    `json:"entity_type,omitempty"`
	EntityID    *uuid.UUID `json:"entity_id,omitempty"`
	Link        *string    `json:"link,omitempty"`
	IsRead      bool       `json:"is_read"`
	RemindAt    *time.Time `json:"remind_at,omitempty"`
	DismissedAt *time.Time `json:"dismissed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// CreateNotificationRequest represents the request to create a notification.
type CreateNotificationRequest struct {
	UserID     uuid.UUID  `json:"user_id" binding:"required"`
	Type       string     `json:"type" binding:"required,oneof=document deadline email system reminder"`
	Title      string     `json:"title" binding:"required,max=255"`
	Message    string     `json:"message" binding:"required"`
	EntityType *string    `json:"entity_type,omitempty"`
	EntityID   *uuid.UUID `json:"entity_id,omitempty"`
	Link       *string    `json:"link,omitempty"`
	RemindAt   *time.Time `json:"remind_at,omitempty"`
}

// UpdateNotificationRequest represents the request to update a notification.
type UpdateNotificationRequest struct {
	IsRead   *bool      `json:"is_read,omitempty"`
	RemindAt *time.Time `json:"remind_at,omitempty"`
}

// List returns all notifications for the current user.
// GET /api/v1/notifications
func (h *NotificationHandler) List(c *gin.Context) {
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
	unreadOnly := c.Query("unread") == "true"
	notificationType := c.Query("type")

	query := `
		SELECT id, tenant_id, user_id, type, title, message, entity_type, entity_id,
		       link, is_read, remind_at, dismissed_at, created_at
		FROM notifications
		WHERE tenant_id = $1 AND user_id = $2 AND dismissed_at IS NULL
	`
	args := []interface{}{tenantID, userID}
	argIdx := 3

	if unreadOnly {
		query += ` AND is_read = false`
	}

	if notificationType != "" {
		query += ` AND type = $` + strconv.Itoa(argIdx)
		args = append(args, notificationType)
		argIdx++
	}

	query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, limit, offset)

	var notifications []Notification
	err := tenantDB.Query(c, query, args, func(rows pgx.Rows) error {
		var n Notification
		err := rows.Scan(
			&n.ID, &n.TenantID, &n.UserID, &n.Type, &n.Title, &n.Message,
			&n.EntityType, &n.EntityID, &n.Link, &n.IsRead, &n.RemindAt,
			&n.DismissedAt, &n.CreatedAt,
		)
		if err != nil {
			return err
		}
		notifications = append(notifications, n)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list notifications")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notifications"})
		return
	}

	if notifications == nil {
		notifications = []Notification{}
	}

	// Get unread count
	var unreadCount int
	err = tenantDB.QueryRowScan(c, []interface{}{&unreadCount},
		`SELECT COUNT(*) FROM notifications WHERE tenant_id = $1 AND user_id = $2 AND is_read = false AND dismissed_at IS NULL`,
		tenantID, userID)
	if err != nil {
		unreadCount = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications,
		"count":         len(notifications),
		"unread_count":  unreadCount,
	})
}

// Get returns a single notification.
// GET /api/v1/notifications/:id
func (h *NotificationHandler) Get(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	notificationID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	var n Notification
	err = tenantDB.QueryRowScan(c, []interface{}{
		&n.ID, &n.TenantID, &n.UserID, &n.Type, &n.Title, &n.Message,
		&n.EntityType, &n.EntityID, &n.Link, &n.IsRead, &n.RemindAt,
		&n.DismissedAt, &n.CreatedAt,
	}, `
		SELECT id, tenant_id, user_id, type, title, message, entity_type, entity_id,
		       link, is_read, remind_at, dismissed_at, created_at
		FROM notifications
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3
	`, notificationID, tenantID, userID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get notification")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notification"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"notification": n})
}

// Create creates a new notification (typically called internally or by admins).
// POST /api/v1/notifications
func (h *NotificationHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	currentUserID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req CreateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify target user belongs to the same tenant AND check role authorization
	var targetUserRole string
	err := tenantDB.QueryRowScan(c, []interface{}{&targetUserRole},
		`SELECT role FROM users WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`,
		req.UserID, tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User not found in tenant"})
		return
	}

	// SECURITY: Staff cannot send notifications to admins (privilege escalation prevention)
	// Only tenant_admin or super_admin can notify anyone
	currentRole, _ := middleware.GetRole(c)
	if currentRole == "staff" {
		if targetUserRole == "tenant_admin" || targetUserRole == "super_admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "Staff cannot send notifications to administrators",
			})
			return
		}
	}

	notificationID := uuid.New()
	now := time.Now()

	_, err = tenantDB.Exec(c, `
		INSERT INTO notifications (id, tenant_id, user_id, type, title, message, entity_type, entity_id, link, is_read, remind_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, false, $10, $11)
	`, notificationID, tenantID, req.UserID, req.Type, req.Title, req.Message,
		req.EntityType, req.EntityID, req.Link, req.RemindAt, now)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create notification")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create notification"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionNotificationCreate, &currentUserID, &tenantID, "notification", &notificationID, c.ClientIP(), map[string]interface{}{
		"target_user_id": req.UserID.String(),
		"type":           req.Type,
	})

	c.JSON(http.StatusCreated, gin.H{
		"notification": Notification{
			ID:         notificationID,
			TenantID:   tenantID,
			UserID:     req.UserID,
			Type:       req.Type,
			Title:      req.Title,
			Message:    req.Message,
			EntityType: req.EntityType,
			EntityID:   req.EntityID,
			Link:       req.Link,
			IsRead:     false,
			RemindAt:   req.RemindAt,
			CreatedAt:  now,
		},
	})
}

// MarkRead marks a notification as read.
// PATCH /api/v1/notifications/:id/read
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	notificationID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	result, err := tenantDB.Exec(c, `
		UPDATE notifications SET is_read = true
		WHERE id = $1 AND tenant_id = $2 AND user_id = $3
	`, notificationID, tenantID, userID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to mark notification as read")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update notification"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification marked as read"})
}

// MarkAllRead marks all notifications as read for the current user.
// POST /api/v1/notifications/read-all
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	result, err := tenantDB.Exec(c, `
		UPDATE notifications SET is_read = true
		WHERE tenant_id = $1 AND user_id = $2 AND is_read = false
	`, tenantID, userID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to mark all notifications as read")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update notifications"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "All notifications marked as read",
		"count":   result.RowsAffected(),
	})
}

// Dismiss dismisses a notification (soft delete).
// DELETE /api/v1/notifications/:id
func (h *NotificationHandler) Dismiss(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	notificationID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	now := time.Now()
	result, err := tenantDB.Exec(c, `
		UPDATE notifications SET dismissed_at = $1
		WHERE id = $2 AND tenant_id = $3 AND user_id = $4 AND dismissed_at IS NULL
	`, now, notificationID, tenantID, userID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to dismiss notification")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to dismiss notification"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification dismissed"})
}

// DismissAll dismisses all notifications for the current user.
// POST /api/v1/notifications/dismiss-all
func (h *NotificationHandler) DismissAll(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	now := time.Now()
	result, err := tenantDB.Exec(c, `
		UPDATE notifications SET dismissed_at = $1
		WHERE tenant_id = $2 AND user_id = $3 AND dismissed_at IS NULL
	`, now, tenantID, userID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to dismiss all notifications")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to dismiss notifications"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "All notifications dismissed",
		"count":   result.RowsAffected(),
	})
}

// GetUnreadCount returns the count of unread notifications.
// GET /api/v1/notifications/unread-count
func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var count int
	err := tenantDB.QueryRowScan(c, []interface{}{&count},
		`SELECT COUNT(*) FROM notifications WHERE tenant_id = $1 AND user_id = $2 AND is_read = false AND dismissed_at IS NULL`,
		tenantID, userID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to get unread count")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch unread count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"unread_count": count})
}
