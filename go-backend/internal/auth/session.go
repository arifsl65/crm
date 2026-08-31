package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/database"
)

// SessionManager handles session and refresh token operations with the database.
type SessionManager struct {
	db                 *database.Pool
	refreshTokenExpire time.Duration
}

// NewSessionManager creates a new session manager.
func NewSessionManager(db *database.Pool, refreshTokenExpire time.Duration) *SessionManager {
	return &SessionManager{
		db:                 db,
		refreshTokenExpire: refreshTokenExpire,
	}
}

// RefreshTokenRecord represents a refresh token stored in the database.
type RefreshTokenRecord struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TenantID  *uuid.UUID
	Family    uuid.UUID
	TokenHash string
	IPAddress string
	UserAgent string
	RevokedAt *time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
	ExpiresAt time.Time
}

// hashToken creates a SHA256 hash of the token for secure storage.
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// StoreRefreshToken stores a new refresh token in the database.
func (sm *SessionManager) StoreRefreshToken(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID, token, ipAddress, userAgent string) (uuid.UUID, error) {
	tokenHash := hashToken(token)
	family := uuid.New()

	query := `
		INSERT INTO refresh_tokens (user_id, tenant_id, token_hash, family, ip_address, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	expiresAt := time.Now().Add(sm.refreshTokenExpire)
	var id uuid.UUID
	var qErr error
	if err := sm.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		qErr = tx.QueryRow(ctx, query, userID, tenantID, tokenHash, family, ipAddress, userAgent, expiresAt).Scan(&id)
		return nil
	}); err != nil {
		log.Error().Err(err).Msg("Failed to store refresh token")
		return uuid.Nil, err
	}
	if qErr != nil {
		log.Error().Err(qErr).Msg("Failed to store refresh token")
		return uuid.Nil, qErr
	}

	return family, nil
}

// StoreRotatedRefreshToken stores a new refresh token as part of token rotation.
func (sm *SessionManager) StoreRotatedRefreshToken(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID, newToken, oldToken, ipAddress, userAgent string, family uuid.UUID) error {
	newTokenHash := hashToken(newToken)
	oldTokenHash := hashToken(oldToken)
	expiresAt := time.Now().Add(sm.refreshTokenExpire)

	return sm.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		// Mark old token as used
		_, err := tx.Exec(ctx, `
			UPDATE refresh_tokens SET used_at = NOW() WHERE token_hash = $1
		`, oldTokenHash)
		if err != nil {
			return err
		}

		// Insert new token
		_, err = tx.Exec(ctx, `
			INSERT INTO refresh_tokens (user_id, tenant_id, token_hash, family, parent_token_hash, ip_address, user_agent, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, userID, tenantID, newTokenHash, family, oldTokenHash, ipAddress, userAgent, expiresAt)
		return err
	})
}

// ValidateRefreshToken checks if a refresh token is valid (exists, not revoked, not expired, not used).
func (sm *SessionManager) ValidateRefreshToken(ctx context.Context, token string) (*RefreshTokenRecord, error) {
	tokenHash := hashToken(token)

	query := `
		SELECT id, user_id, tenant_id, family, token_hash, ip_address, user_agent, revoked_at, used_at, created_at, expires_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`

	var record RefreshTokenRecord
	var qErr error
	if err := sm.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		qErr = tx.QueryRow(ctx, query, tokenHash).Scan(
			&record.ID, &record.UserID, &record.TenantID, &record.Family,
			&record.TokenHash, &record.IPAddress, &record.UserAgent,
			&record.RevokedAt, &record.UsedAt, &record.CreatedAt, &record.ExpiresAt,
		)
		return nil
	}); err != nil {
		return nil, err
	}
	if qErr != nil {
		if qErr == pgx.ErrNoRows {
			return nil, ErrInvalidToken
		}
		return nil, qErr
	}

	// Check if token is revoked
	if record.RevokedAt != nil {
		log.Warn().Str("token_id", record.ID.String()).Msg("Attempted to use revoked refresh token")
		return nil, ErrTokenRevoked
	}

	// Check if token is already used (replay attack detection)
	if record.UsedAt != nil {
		// Token reuse detected! Revoke entire family for security
		log.Warn().
			Str("token_id", record.ID.String()).
			Str("family", record.Family.String()).
			Msg("Token reuse detected - revoking entire token family")
		_ = sm.RevokeTokenFamily(ctx, record.Family)
		return nil, ErrTokenReused
	}

	// Check if token is expired
	if time.Now().After(record.ExpiresAt) {
		return nil, ErrExpiredToken
	}

	return &record, nil
}

// RevokeRefreshToken revokes a specific refresh token.
func (sm *SessionManager) RevokeRefreshToken(ctx context.Context, token string) error {
	tokenHash := hashToken(token)

	query := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1`
	return sm.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, query, tokenHash)
		return err
	})
}

// RevokeTokenFamily revokes all tokens in a family (for token theft detection).
func (sm *SessionManager) RevokeTokenFamily(ctx context.Context, family uuid.UUID) error {
	query := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE family = $1 AND revoked_at IS NULL`
	return sm.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, query, family)
		if err != nil {
			log.Error().Err(err).Str("family", family.String()).Msg("Failed to revoke token family")
		}
		return err
	})
}

// RevokeAllUserTokens revokes all refresh tokens for a user.
func (sm *SessionManager) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`
	var rowsAffected int64
	if err := sm.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, query, userID)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	}); err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("Failed to revoke all user tokens")
		return err
	}

	log.Info().
		Str("user_id", userID.String()).
		Int64("tokens_revoked", rowsAffected).
		Msg("Revoked all user tokens")
	return nil
}

// GetUserActiveSessions returns all active (non-revoked, non-expired) sessions for a user.
func (sm *SessionManager) GetUserActiveSessions(ctx context.Context, userID uuid.UUID) ([]RefreshTokenRecord, error) {
	query := `
		SELECT id, user_id, tenant_id, family, token_hash, ip_address, user_agent, revoked_at, used_at, created_at, expires_at
		FROM refresh_tokens
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW() AND used_at IS NULL
		ORDER BY created_at DESC
	`

	var records []RefreshTokenRecord
	var qErr error
	if err := sm.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, userID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var record RefreshTokenRecord
			if err := rows.Scan(
				&record.ID, &record.UserID, &record.TenantID, &record.Family,
				&record.TokenHash, &record.IPAddress, &record.UserAgent,
				&record.RevokedAt, &record.UsedAt, &record.CreatedAt, &record.ExpiresAt,
			); err != nil {
				return err
			}
			records = append(records, record)
		}

		qErr = rows.Err()
		return nil
	}); err != nil {
		return nil, err
	}

	return records, qErr
}

// CleanupExpiredTokens removes expired tokens (should be run periodically).
func (sm *SessionManager) CleanupExpiredTokens(ctx context.Context) (int64, error) {
	query := `DELETE FROM refresh_tokens WHERE expires_at < NOW() - INTERVAL '7 days'`
	var rowsAffected int64
	if err := sm.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, query)
		if err != nil {
			return err
		}
		rowsAffected = result.RowsAffected()
		return nil
	}); err != nil {
		return 0, err
	}
	return rowsAffected, nil
}
