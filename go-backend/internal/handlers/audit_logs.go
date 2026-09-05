package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// AuditLogHandler handles audit log viewing operations.
type AuditLogHandler struct {
	db *database.Pool
}

// NewAuditLogHandler creates a new audit log handler.
func NewAuditLogHandler(db *database.Pool) *AuditLogHandler {
	return &AuditLogHandler{db: db}
}

// AuditLogEntry represents an audit log entry for API response.
type AuditLogEntry struct {
	ID         uuid.UUID              `json:"id"`
	TenantID   *uuid.UUID             `json:"tenant_id,omitempty"`
	UserID     *uuid.UUID             `json:"user_id,omitempty"`
	UserName   *string                `json:"user_name,omitempty"`
	Action     string                 `json:"action"`
	EntityType string                 `json:"entity_type"`
	EntityID   *uuid.UUID             `json:"entity_id,omitempty"`
	OldValue   map[string]interface{} `json:"old_value,omitempty"`
	NewValue   map[string]interface{} `json:"new_value,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	IPAddress  *string                `json:"ip_address,omitempty"`
	Severity   string                 `json:"severity"`
	CreatedAt  time.Time              `json:"created_at"`
}

// List returns audit logs for the tenant with filtering and pagination.
// GET /api/v1/audit-logs
func (h *AuditLogHandler) List(c *gin.Context) {
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
	if limit > 200 {
		limit = 200
	}

	// Filters
	userID := c.Query("user_id")
	action := c.Query("action")
	entityType := c.Query("entity_type")
	entityID := c.Query("entity_id")
	severity := c.Query("severity")
	search := c.Query("search")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	var query strings.Builder
	var args []interface{}
	argNum := 1

	query.WriteString(`
		SELECT al.id, al.tenant_id, al.user_id, u.name as user_name,
		       al.action, al.entity_type, al.entity_id,
		       al.old_value, al.new_value, al.metadata,
		       al.ip_address, al.severity, al.created_at
		FROM audit_logs al
		LEFT JOIN users u ON al.user_id = u.id
		WHERE al.tenant_id = $1
	`)
	args = append(args, tenantID)
	argNum++

	if userID != "" {
		if uid, err := uuid.Parse(userID); err == nil {
			query.WriteString(` AND al.user_id = $`)
			query.WriteString(strconv.Itoa(argNum))
			args = append(args, uid)
			argNum++
		}
	}

	if action != "" {
		query.WriteString(` AND al.action = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, action)
		argNum++
	}

	if entityType != "" {
		query.WriteString(` AND al.entity_type = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, entityType)
		argNum++
	}

	if entityID != "" {
		if eid, err := uuid.Parse(entityID); err == nil {
			query.WriteString(` AND al.entity_id = $`)
			query.WriteString(strconv.Itoa(argNum))
			args = append(args, eid)
			argNum++
		}
	}

	if severity != "" {
		query.WriteString(` AND al.severity = $`)
		query.WriteString(strconv.Itoa(argNum))
		args = append(args, severity)
		argNum++
	}

	if search != "" {
		query.WriteString(` AND (al.action ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(` OR al.entity_type ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(` OR u.name ILIKE $`)
		query.WriteString(strconv.Itoa(argNum))
		query.WriteString(`)`)
		args = append(args, "%"+search+"%")
		argNum++
	}

	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			query.WriteString(` AND al.created_at >= $`)
			query.WriteString(strconv.Itoa(argNum))
			args = append(args, t)
			argNum++
		}
	}

	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			// Add 1 day to include the end date fully
			t = t.AddDate(0, 0, 1)
			query.WriteString(` AND al.created_at < $`)
			query.WriteString(strconv.Itoa(argNum))
			args = append(args, t)
			argNum++
		}
	}

	query.WriteString(` ORDER BY al.created_at DESC LIMIT $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, limit)
	argNum++

	query.WriteString(` OFFSET $`)
	query.WriteString(strconv.Itoa(argNum))
	args = append(args, offset)

	var logs []AuditLogEntry
	err := tenantDB.Query(c, query.String(), args, func(rows pgx.Rows) error {
		var entry AuditLogEntry
		var oldValueJSON, newValueJSON, metadataJSON []byte

		err := rows.Scan(
			&entry.ID, &entry.TenantID, &entry.UserID, &entry.UserName,
			&entry.Action, &entry.EntityType, &entry.EntityID,
			&oldValueJSON, &newValueJSON, &metadataJSON,
			&entry.IPAddress, &entry.Severity, &entry.CreatedAt,
		)
		if err != nil {
			return err
		}

		// Parse JSON fields
		if len(oldValueJSON) > 0 {
			_ = json.Unmarshal(oldValueJSON, &entry.OldValue)
		}
		if len(newValueJSON) > 0 {
			_ = json.Unmarshal(newValueJSON, &entry.NewValue)
		}
		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &entry.Metadata)
		}

		logs = append(logs, entry)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list audit logs")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch audit logs"})
		return
	}

	if logs == nil {
		logs = []AuditLogEntry{}
	}

	// Get total count for pagination
	var total int
	countQuery := `SELECT COUNT(*) FROM audit_logs WHERE tenant_id = $1`
	_ = tenantDB.QueryRowScan(c, []interface{}{&total}, countQuery, tenantID)

	c.JSON(http.StatusOK, gin.H{
		"audit_logs": logs,
		"count":      len(logs),
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}

// Get returns a single audit log entry.
// GET /api/v1/audit-logs/:id
func (h *AuditLogHandler) Get(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	id := c.Param("id")

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	logID, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid audit log ID"})
		return
	}

	query := `
		SELECT al.id, al.tenant_id, al.user_id, u.name as user_name,
		       al.action, al.entity_type, al.entity_id,
		       al.old_value, al.new_value, al.metadata,
		       al.ip_address, al.severity, al.created_at
		FROM audit_logs al
		LEFT JOIN users u ON al.user_id = u.id
		WHERE al.id = $1 AND al.tenant_id = $2
	`

	var entry AuditLogEntry
	var oldValueJSON, newValueJSON, metadataJSON []byte

	err = tenantDB.QueryRowScan(c, []interface{}{
		&entry.ID, &entry.TenantID, &entry.UserID, &entry.UserName,
		&entry.Action, &entry.EntityType, &entry.EntityID,
		&oldValueJSON, &newValueJSON, &metadataJSON,
		&entry.IPAddress, &entry.Severity, &entry.CreatedAt,
	}, query, logID, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Audit log not found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get audit log")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch audit log"})
		return
	}

	// Parse JSON fields
	if len(oldValueJSON) > 0 {
		_ = json.Unmarshal(oldValueJSON, &entry.OldValue)
	}
	if len(newValueJSON) > 0 {
		_ = json.Unmarshal(newValueJSON, &entry.NewValue)
	}
	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &entry.Metadata)
	}

	c.JSON(http.StatusOK, entry)
}

// GetActions returns the list of unique actions for filtering.
// GET /api/v1/audit-logs/actions
func (h *AuditLogHandler) GetActions(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	query := `
		SELECT DISTINCT action
		FROM audit_logs
		WHERE tenant_id = $1
		ORDER BY action
	`

	var actions []string
	err := tenantDB.Query(c, query, []interface{}{tenantID}, func(rows pgx.Rows) error {
		var action string
		if err := rows.Scan(&action); err != nil {
			return err
		}
		actions = append(actions, action)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get audit log actions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch actions"})
		return
	}

	if actions == nil {
		actions = []string{}
	}

	c.JSON(http.StatusOK, gin.H{"actions": actions})
}

// GetEntityTypes returns the list of unique entity types for filtering.
// GET /api/v1/audit-logs/entity-types
func (h *AuditLogHandler) GetEntityTypes(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	query := `
		SELECT DISTINCT entity_type
		FROM audit_logs
		WHERE tenant_id = $1 AND entity_type IS NOT NULL AND entity_type != ''
		ORDER BY entity_type
	`

	var entityTypes []string
	err := tenantDB.Query(c, query, []interface{}{tenantID}, func(rows pgx.Rows) error {
		var entityType string
		if err := rows.Scan(&entityType); err != nil {
			return err
		}
		entityTypes = append(entityTypes, entityType)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get audit log entity types")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch entity types"})
		return
	}

	if entityTypes == nil {
		entityTypes = []string{}
	}

	c.JSON(http.StatusOK, gin.H{"entity_types": entityTypes})
}

// GetStats returns audit log statistics.
// GET /api/v1/audit-logs/stats
func (h *AuditLogHandler) GetStats(c *gin.Context) {
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
			COUNT(*) FILTER (WHERE severity = 'info') as info_count,
			COUNT(*) FILTER (WHERE severity = 'warning') as warning_count,
			COUNT(*) FILTER (WHERE severity = 'critical') as critical_count,
			COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '24 hours') as last_24h,
			COUNT(*) FILTER (WHERE created_at > NOW() - INTERVAL '7 days') as last_7d
		FROM audit_logs
		WHERE tenant_id = $1
	`

	var stats struct {
		Total         int `json:"total"`
		InfoCount     int `json:"info_count"`
		WarningCount  int `json:"warning_count"`
		CriticalCount int `json:"critical_count"`
		Last24h      int `json:"last_24h"`
		Last7d       int `json:"last_7d"`
	}

	err := tenantDB.QueryRowScan(c, []interface{}{
		&stats.Total, &stats.InfoCount, &stats.WarningCount,
		&stats.CriticalCount, &stats.Last24h, &stats.Last7d,
	}, query, tenantID)

	if err != nil {
		log.Error().Err(err).Msg("Failed to get audit log stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}
