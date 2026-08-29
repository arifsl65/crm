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
	err := sm.db.QueryRow(ctx, query, userID, tenantID, tokenHash, family, ipAddress, userAgent, expiresAt).Scan(&id)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store refresh token")
		return uuid.Nil, err
	}

	return family, nil
}

// StoreRotatedRefreshToken stores a new refresh token as part of token rotation.
func (sm *SessionManager) StoreRotatedRefreshToken(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID, newToken, oldToken, ipAddress, userAgent string, family uuid.UUID) error {
	newTokenHash := hashToken(newToken)
	oldTokenHash := hashToken(oldToken)

	// Mark old token as used and store new token in a transaction
	tx, err := sm.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Mark old token as used
	_, err = tx.Exec(ctx, `
		UPDATE refresh_tokens SET used_at = NOW() WHERE token_hash = $1
	`, oldTokenHash)
	if err != nil {
		return err
	}

	// Insert new token
	expiresAt := time.Now().Add(sm.refreshTokenExpire)
	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, tenant_id, token_hash, family, parent_token_hash, ip_address, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, userID, tenantID, newTokenHash, family, oldTokenHash, ipAddress, userAgent, expiresAt)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
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
	err := sm.db.QueryRow(ctx, query, tokenHash).Scan(
		&record.ID, &record.UserID, &record.TenantID, &record.Family,
		&record.TokenHash, &record.IPAddress, &record.UserAgent,
		&record.RevokedAt, &record.UsedAt, &record.CreatedAt, &record.ExpiresAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	// Check if token is revoked
	if record.RevokedAt != nil {
		log.Warn().Str("token_id", record.ID.String()).Msg("Attempted to use revoked refresh token")
		return nil, ErrInvalidToken
	}

	// Check if token is already used (replay attack detection)
	if record.UsedAt != nil {
		// Token reuse detected! Revoke entire family
		log.Warn().
			Str("token_id", record.ID.String()).
			Str("family", record.Family.String()).
			Msg("Token reuse detected - revoking entire token family")
		_ = sm.RevokeTokenFamily(ctx, record.Family)
		return nil, ErrInvalidToken
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
	_, err := sm.db.Exec(ctx, query, tokenHash)
	return err
}

// RevokeTokenFamily revokes all tokens in a family (for token theft detection).
func (sm *SessionManager) RevokeTokenFamily(ctx context.Context, family uuid.UUID) error {
	query := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE family = $1 AND revoked_at IS NULL`
	_, err := sm.db.Exec(ctx, query, family)
	if err != nil {
		log.Error().Err(err).Str("family", family.String()).Msg("Failed to revoke token family")
	}
	return err
}

// RevokeAllUserTokens revokes all refresh tokens for a user.
func (sm *SessionManager) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`
	result, err := sm.db.Exec(ctx, query, userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("Failed to revoke all user tokens")
		return err
	}

	log.Info().
		Str("user_id", userID.String()).
		Int64("tokens_revoked", result.RowsAffected()).
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

	rows, err := sm.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []RefreshTokenRecord
	for rows.Next() {
		var record RefreshTokenRecord
		err := rows.Scan(
			&record.ID, &record.UserID, &record.TenantID, &record.Family,
			&record.TokenHash, &record.IPAddress, &record.UserAgent,
			&record.RevokedAt, &record.UsedAt, &record.CreatedAt, &record.ExpiresAt,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

// CleanupExpiredTokens removes expired tokens (should be run periodically).
func (sm *SessionManager) CleanupExpiredTokens(ctx context.Context) (int64, error) {
	query := `DELETE FROM refresh_tokens WHERE expires_at < NOW() - INTERVAL '7 days'`
	result, err := sm.db.Exec(ctx, query)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
