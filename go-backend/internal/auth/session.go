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
// NOTE: The old token is already marked as used atomically by ValidateRefreshToken,
// so we only need to insert the new token here.
func (sm *SessionManager) StoreRotatedRefreshToken(ctx context.Context, userID uuid.UUID, tenantID *uuid.UUID, newToken, oldToken, ipAddress, userAgent string, family uuid.UUID) error {
	newTokenHash := hashToken(newToken)
	oldTokenHash := hashToken(oldToken)
	expiresAt := time.Now().Add(sm.refreshTokenExpire)

	return sm.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		// Insert new token (old token was already marked as used by ValidateRefreshToken)
		_, err := tx.Exec(ctx, `
			INSERT INTO refresh_tokens (user_id, tenant_id, token_hash, family, parent_token_hash, ip_address, user_agent, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, userID, tenantID, newTokenHash, family, oldTokenHash, ipAddress, userAgent, expiresAt)
		return err
	})
}

// ValidateRefreshToken checks if a refresh token is valid and atomically marks it as used.
// This prevents TOCTOU race conditions where two concurrent requests could both validate
// the same token before either marks it as used.
func (sm *SessionManager) ValidateRefreshToken(ctx context.Context, token string) (*RefreshTokenRecord, error) {
	tokenHash := hashToken(token)

	// SECURITY: Atomic UPDATE ... RETURNING to validate AND mark as used in one operation
	// This prevents race conditions between validation and consumption
	atomicQuery := `
		UPDATE refresh_tokens
		SET used_at = NOW()
		WHERE token_hash = $1
		  AND revoked_at IS NULL
		  AND used_at IS NULL
		  AND expires_at > NOW()
		RETURNING id, user_id, tenant_id, family, token_hash, ip_address, user_agent, revoked_at, used_at, created_at, expires_at
	`

	var record RefreshTokenRecord
	var atomicErr error
	if err := sm.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		atomicErr = tx.QueryRow(ctx, atomicQuery, tokenHash).Scan(
			&record.ID, &record.UserID, &record.TenantID, &record.Family,
			&record.TokenHash, &record.IPAddress, &record.UserAgent,
			&record.RevokedAt, &record.UsedAt, &record.CreatedAt, &record.ExpiresAt,
		)
		return nil
	}); err != nil {
		return nil, err
	}

	// If atomic update succeeded, token is valid and now marked as used
	if atomicErr == nil {
		return &record, nil
	}

	// If no rows affected, determine the specific error by checking the token state
	if atomicErr == pgx.ErrNoRows {
		return sm.diagnoseRefreshTokenError(ctx, tokenHash)
	}

	return nil, atomicErr
}

// diagnoseRefreshTokenError determines why a refresh token validation failed.
// Called only when the atomic UPDATE returned no rows.
func (sm *SessionManager) diagnoseRefreshTokenError(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
	query := `
		SELECT id, family, revoked_at, used_at, expires_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`

	var id uuid.UUID
	var family uuid.UUID
	var revokedAt, usedAt *time.Time
	var expiresAt time.Time

	var qErr error
	if err := sm.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		qErr = tx.QueryRow(ctx, query, tokenHash).Scan(&id, &family, &revokedAt, &usedAt, &expiresAt)
		return nil
	}); err != nil {
		return nil, err
	}

	if qErr == pgx.ErrNoRows {
		// Token doesn't exist at all
		return nil, ErrInvalidToken
	}
	if qErr != nil {
		return nil, qErr
	}

	// Check why the token failed validation
	if revokedAt != nil {
		log.Warn().Str("token_id", id.String()).Msg("Attempted to use revoked refresh token")
		return nil, ErrTokenRevoked
	}

	if usedAt != nil {
		// Token reuse detected! Another request already used this token.
		// Revoke entire family for security (potential token theft)
		log.Warn().
			Str("token_id", id.String()).
			Str("family", family.String()).
			Msg("Token reuse detected - revoking entire token family")
		_ = sm.RevokeTokenFamily(ctx, family)
		return nil, ErrTokenReused
	}

	if time.Now().After(expiresAt) {
		return nil, ErrExpiredToken
	}

	// Shouldn't reach here, but return invalid token as fallback
	return nil, ErrInvalidToken
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
