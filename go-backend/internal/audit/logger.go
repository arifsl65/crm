package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
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
	ActionDocumentUpload     Action = "document_upload"
	ActionDocumentDelete     Action = "document_delete"
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

// LogEntry represents an audit log entry.
type LogEntry struct {
	TenantID    *uuid.UUID
	UserID      *uuid.UUID
	Action      Action
	EntityType  string            // e.g., "user", "client", "document"
	EntityID    *uuid.UUID        // ID of the affected entity
	IPAddress   string
	UserAgent   string
	Metadata    map[string]interface{} // Additional context
	Success     bool
	ErrorMsg    string
}

// Log writes an audit entry to the database.
func (l *Logger) Log(ctx context.Context, entry LogEntry) error {
	var metadataJSON []byte
	var err error
	if entry.Metadata != nil {
		metadataJSON, err = json.Marshal(entry.Metadata)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal audit metadata")
			metadataJSON = []byte("{}")
		}
	} else {
		metadataJSON = []byte("{}")
	}

	query := `
		INSERT INTO audit_logs (
			tenant_id, user_id, action, entity_type, entity_id,
			ip_address, user_agent, metadata, success, error_message, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err = l.db.Exec(ctx, query,
		entry.TenantID,
		entry.UserID,
		string(entry.Action),
		entry.EntityType,
		entry.EntityID,
		entry.IPAddress,
		entry.UserAgent,
		metadataJSON,
		entry.Success,
		entry.ErrorMsg,
		time.Now(),
	)

	if err != nil {
		log.Error().Err(err).
			Str("action", string(entry.Action)).
			Msg("Failed to write audit log")
		return err
	}

	// Also log to structured log for real-time monitoring
	logEvent := log.Info().
		Str("audit_action", string(entry.Action)).
		Bool("success", entry.Success)

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
	if !entry.Success && entry.ErrorMsg != "" {
		logEvent = logEvent.Str("error", entry.ErrorMsg)
	}

	logEvent.Msg("Audit event")

	return nil
}

// LogAuth is a convenience method for auth-related events.
func (l *Logger) LogAuth(ctx context.Context, action Action, userID *uuid.UUID, tenantID *uuid.UUID, ip, userAgent string, success bool, errorMsg string) {
	_ = l.Log(ctx, LogEntry{
		TenantID:   tenantID,
		UserID:     userID,
		Action:     action,
		EntityType: "user",
		EntityID:   userID,
		IPAddress:  ip,
		UserAgent:  userAgent,
		Success:    success,
		ErrorMsg:   errorMsg,
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
		Success:    true,
	})
}

// GetUserAuditLogs retrieves audit logs for a specific user.
func (l *Logger) GetUserAuditLogs(ctx context.Context, userID uuid.UUID, limit, offset int) ([]map[string]interface{}, error) {
	query := `
		SELECT id, action, entity_type, entity_id, ip_address, success, error_message, created_at
		FROM audit_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := l.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var (
			id         uuid.UUID
			action     string
			entityType *string
			entityID   *uuid.UUID
			ipAddress  *string
			success    bool
			errorMsg   *string
			createdAt  time.Time
		)

		err := rows.Scan(&id, &action, &entityType, &entityID, &ipAddress, &success, &errorMsg, &createdAt)
		if err != nil {
			return nil, err
		}

		entry := map[string]interface{}{
			"id":         id.String(),
			"action":     action,
			"success":    success,
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
		if errorMsg != nil {
			entry["error_message"] = *errorMsg
		}

		logs = append(logs, entry)
	}

	return logs, rows.Err()
}
