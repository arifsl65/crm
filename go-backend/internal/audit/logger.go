package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/database"
)

// Action represents the type of action being audited.
type Action string

const (
	ActionLogin              Action = "login"
	ActionLoginFailed        Action = "login_failed"
	ActionLogout             Action = "logout"
	ActionRegister           Action = "register"
	ActionPasswordReset      Action = "password_reset"
	ActionPasswordResetReq   Action = "password_reset_request"
	ActionPasswordChange     Action = "password_change"
	ActionTokenRefresh       Action = "token_refresh"
	ActionSessionRevoke      Action = "session_revoke"
	ActionSessionRevokeAll   Action = "session_revoke_all"
	ActionProfileUpdate      Action = "profile_update"
	Action2FASetup           Action = "2fa_setup"
	Action2FAVerify          Action = "2fa_verify"
	Action2FADisable         Action = "2fa_disable"
	ActionUserCreate         Action = "user_create"
	ActionUserUpdate         Action = "user_update"
	ActionUserDelete         Action = "user_delete"
	ActionClientCreate       Action = "client_create"
	ActionClientUpdate       Action = "client_update"
	ActionClientDelete       Action = "client_delete"
	ActionServiceCreate      Action = "service_create"
	ActionServiceUpdate      Action = "service_update"
	ActionServiceDelete      Action = "service_delete"
	ActionServiceComplete    Action = "service_complete"
	ActionDocumentCreate     Action = "document_create"
	ActionDocumentUpload     Action = "document_upload"
	ActionDocumentUpdate     Action = "document_update"
	ActionDocumentDelete     Action = "document_delete"
	ActionDocumentApprove    Action = "document_approve"
	ActionDocumentReject     Action = "document_reject"
	ActionDocumentDownload   Action = "document_download"
	ActionServiceTypeCreate  Action = "service_type_create"
	ActionServiceTypeUpdate  Action = "service_type_update"
	ActionServiceTypeDelete  Action = "service_type_delete"
	ActionDocumentTypeCreate Action = "document_type_create"
	ActionDocumentTypeUpdate Action = "document_type_update"
	ActionDocumentTypeDelete Action = "document_type_delete"
	ActionInvoiceCreate      Action = "invoice_create"
	ActionPaymentRecord      Action = "payment_record"
	ActionSettingsUpdate     Action = "settings_update"
	ActionAPIKeyCreate       Action = "api_key_create"
	ActionAPIKeyRevoke       Action = "api_key_revoke"
)

// Logger handles audit logging to the database.
type Logger struct {
	db *database.Pool
}

// NewLogger creates a new audit logger.
func NewLogger(db *database.Pool) *Logger {
	return &Logger{db: db}
}

// Severity levels for audit logs
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// LogEntry represents an audit log entry.
type LogEntry struct {
	TenantID   *uuid.UUID
	UserID     *uuid.UUID
	Action     Action
	EntityType string            // e.g., "user", "client", "document"
	EntityID   *uuid.UUID        // ID of the affected entity
	OldValue   map[string]interface{} // Previous state (for updates)
	NewValue   map[string]interface{} // New state (for creates/updates)
	Metadata   map[string]interface{} // Additional context
	IPAddress  string
	UserAgent  string
	Severity   Severity
}

// Log writes an audit entry to the database.
func (l *Logger) Log(ctx context.Context, entry LogEntry) error {
	// Marshal JSON fields
	metadataJSON := []byte("{}")
	oldValueJSON := []byte(nil)
	newValueJSON := []byte(nil)
	var err error

	if entry.Metadata != nil {
		metadataJSON, err = json.Marshal(entry.Metadata)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal audit metadata")
			metadataJSON = []byte("{}")
		}
	}
	if entry.OldValue != nil {
		oldValueJSON, _ = json.Marshal(entry.OldValue)
	}
	if entry.NewValue != nil {
		newValueJSON, _ = json.Marshal(entry.NewValue)
	}

	// Default severity to info
	severity := entry.Severity
	if severity == "" {
		severity = SeverityInfo
	}

	query := `
		INSERT INTO audit_logs (
			id, tenant_id, user_id, action, entity_type, entity_id,
			old_value, new_value, metadata, ip_address, user_agent, severity, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	// Audit logs must bypass RLS because they may be written from unauthenticated
	// contexts (e.g., failed login) or across tenants.
	err = l.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		_, err = tx.Exec(ctx, query,
			uuid.New(), // id
			entry.TenantID,
			entry.UserID,
			string(entry.Action),
			entry.EntityType,
			entry.EntityID,
			oldValueJSON,
			newValueJSON,
			metadataJSON,
			entry.IPAddress,
			entry.UserAgent,
			string(severity),
			time.Now(),
		)
		return err
	})

	if err != nil {
		log.Error().Err(err).
			Str("action", string(entry.Action)).
			Msg("Failed to write audit log")
		return err
	}

	// Also log to structured log for real-time monitoring
	logEvent := log.Info().
		Str("audit_action", string(entry.Action)).
		Str("severity", string(severity))

	if entry.UserID != nil {
		logEvent = logEvent.Str("user_id", entry.UserID.String())
	}
	if entry.TenantID != nil {
		logEvent = logEvent.Str("tenant_id", entry.TenantID.String())
	}
	if entry.EntityType != "" {
		logEvent = logEvent.Str("entity_type", entry.EntityType)
	}
	if entry.EntityID != nil {
		logEvent = logEvent.Str("entity_id", entry.EntityID.String())
	}
	if entry.IPAddress != "" {
		logEvent = logEvent.Str("ip_address", entry.IPAddress)
	}

	logEvent.Msg("Audit event")

	return nil
}

// LogAuth is a convenience method for auth-related events.
func (l *Logger) LogAuth(ctx context.Context, action Action, userID *uuid.UUID, tenantID *uuid.UUID, ip, userAgent string, success bool, errorMsg string) {
	severity := SeverityInfo
	if !success {
		severity = SeverityWarning
	}

	metadata := map[string]interface{}{
		"success": success,
	}
	if errorMsg != "" {
		metadata["error"] = errorMsg
	}

	_ = l.Log(ctx, LogEntry{
		TenantID:   tenantID,
		UserID:     userID,
		Action:     action,
		EntityType: "user",
		EntityID:   userID,
		IPAddress:  ip,
		UserAgent:  userAgent,
		Severity:   severity,
		Metadata:   metadata,
	})
}

// LogEntity is a convenience method for entity CRUD events.
func (l *Logger) LogEntity(ctx context.Context, action Action, userID, tenantID *uuid.UUID, entityType string, entityID *uuid.UUID, ip string, metadata map[string]interface{}) {
	_ = l.Log(ctx, LogEntry{
		TenantID:   tenantID,
		UserID:     userID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		IPAddress:  ip,
		Metadata:   metadata,
		Severity:   SeverityInfo,
	})
}

// GetUserAuditLogs retrieves audit logs for a specific user.
func (l *Logger) GetUserAuditLogs(ctx context.Context, userID uuid.UUID, limit, offset int) ([]map[string]interface{}, error) {
	query := `
		SELECT id, action, entity_type, entity_id, ip_address, severity, metadata, created_at
		FROM audit_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	var logs []map[string]interface{}
	var qErr error
	if err := l.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, userID, limit, offset)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var (
				id         uuid.UUID
				action     string
				entityType *string
				entityID   *uuid.UUID
				ipAddress  *string
				severity   *string
				metadata   []byte
				createdAt  time.Time
			)

			err := rows.Scan(&id, &action, &entityType, &entityID, &ipAddress, &severity, &metadata, &createdAt)
			if err != nil {
				return err
			}

			entry := map[string]interface{}{
				"id":         id.String(),
				"action":     action,
				"created_at": createdAt,
			}
			if entityType != nil {
				entry["entity_type"] = *entityType
			}
			if entityID != nil {
				entry["entity_id"] = entityID.String()
			}
			if ipAddress != nil {
				entry["ip_address"] = *ipAddress
			}
			if severity != nil {
				entry["severity"] = *severity
			}
			if len(metadata) > 0 {
				var meta map[string]interface{}
				if json.Unmarshal(metadata, &meta) == nil {
					entry["metadata"] = meta
				}
			}

			logs = append(logs, entry)
		}

		qErr = rows.Err()
		return nil
	}); err != nil {
		return nil, err
	}

	return logs, qErr
}
