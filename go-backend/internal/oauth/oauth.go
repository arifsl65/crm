// Package oauth provides OAuth 2.0 integration for email providers.
package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/cache"
	"github.com/accountant-crm/go-backend/internal/config"
	"github.com/accountant-crm/go-backend/internal/crypto"
)

var (
	ErrInvalidState       = errors.New("invalid or expired OAuth state")
	ErrProviderNotEnabled = errors.New("OAuth provider not enabled")
	ErrTokenExchange      = errors.New("failed to exchange authorization code for tokens")
	ErrUserInfoFetch      = errors.New("failed to fetch user info from provider")
)

// Provider represents an OAuth provider type.
type Provider string

const (
	ProviderGoogle    Provider = "google"
	ProviderMicrosoft Provider = "microsoft"
)

// Tokens represents OAuth tokens received from a provider.
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        string    `json:"scope"`
}

// UserInfo represents user information from OAuth provider.
type UserInfo struct {
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	EmailVerified bool   `json:"email_verified"`
}

// StateData stores OAuth state information in Redis.
type StateData struct {
	TenantID  string `json:"tenant_id"`
	UserID    string `json:"user_id"`
	Provider  string `json:"provider"`
	CreatedAt int64  `json:"created_at"`
}

// Service handles OAuth operations.
type Service struct {
	redis     *cache.Client
	encryptor *crypto.Encryptor
	config    config.OAuthConfig
	client    *http.Client
}

// NewService creates a new OAuth service.
func NewService(redis *cache.Client, encryptor *crypto.Encryptor, cfg config.OAuthConfig) *Service {
	return &Service{
		redis:     redis,
		encryptor: encryptor,
		config:    cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GenerateState creates a cryptographically secure state token and stores it in Redis.
func (s *Service) GenerateState(ctx context.Context, tenantID, userID string, provider Provider) (string, error) {
	// Generate random state token
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}
	state := base64.URLEncoding.EncodeToString(b)

	// Store state data in Redis with 10-minute TTL
	data := StateData{
		TenantID:  tenantID,
		UserID:    userID,
		Provider:  string(provider),
		CreatedAt: time.Now().Unix(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state data: %w", err)
	}

	key := fmt.Sprintf("oauth:state:%s", state)
	if err := s.redis.Set(ctx, key, string(jsonData), 10*time.Minute).Err(); err != nil {
		return "", fmt.Errorf("failed to store state in Redis: %w", err)
	}

	log.Debug().
		Str("state", state[:8]+"...").
		Str("provider", string(provider)).
		Str("tenant_id", tenantID).
		Msg("OAuth state generated")

	return state, nil
}

// ValidateState validates the state token and returns the stored data.
// The state is deleted after successful validation (one-time use).
func (s *Service) ValidateState(ctx context.Context, state string) (*StateData, error) {
	key := fmt.Sprintf("oauth:state:%s", state)

	// Get and delete atomically
	jsonData, err := s.redis.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return nil, ErrInvalidState
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get state from Redis: %w", err)
	}

	var data StateData
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state data: %w", err)
	}

	// Verify state is not too old (extra safety check)
	if time.Now().Unix()-data.CreatedAt > 600 {
		return nil, ErrInvalidState
	}

	log.Debug().
		Str("state", state[:8]+"...").
		Str("provider", data.Provider).
		Msg("OAuth state validated")

	return &data, nil
}

// GetAuthURL returns the OAuth authorization URL for a provider.
func (s *Service) GetAuthURL(provider Provider, state string) (string, error) {
	switch provider {
	case ProviderGoogle:
		if !s.config.Google.Enabled {
			return "", ErrProviderNotEnabled
		}
		return s.getGoogleAuthURL(state), nil
	case ProviderMicrosoft:
		if !s.config.Microsoft.Enabled {
			return "", ErrProviderNotEnabled
		}
		return s.getMicrosoftAuthURL(state), nil
	default:
		return "", fmt.Errorf("unknown provider: %s", provider)
	}
}

// ExchangeCode exchanges an authorization code for tokens.
func (s *Service) ExchangeCode(ctx context.Context, provider Provider, code string) (*Tokens, error) {
	switch provider {
	case ProviderGoogle:
		return s.exchangeGoogleCode(ctx, code)
	case ProviderMicrosoft:
		return s.exchangeMicrosoftCode(ctx, code)
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

// GetUserInfo fetches user information using the access token.
func (s *Service) GetUserInfo(ctx context.Context, provider Provider, accessToken string) (*UserInfo, error) {
	switch provider {
	case ProviderGoogle:
		return s.getGoogleUserInfo(ctx, accessToken)
	case ProviderMicrosoft:
		return s.getMicrosoftUserInfo(ctx, accessToken)
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

// RefreshToken refreshes an expired access token.
func (s *Service) RefreshToken(ctx context.Context, provider Provider, refreshToken string) (*Tokens, error) {
	switch provider {
	case ProviderGoogle:
		return s.refreshGoogleToken(ctx, refreshToken)
	case ProviderMicrosoft:
		return s.refreshMicrosoftToken(ctx, refreshToken)
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

// EncryptTokens encrypts tokens for secure storage.
func (s *Service) EncryptTokens(tokens *Tokens) (accessEncrypted, refreshEncrypted string, err error) {
	if s.encryptor == nil {
		return "", "", errors.New("encryptor not configured")
	}

	accessEncrypted, err = s.encryptor.Encrypt(tokens.AccessToken)
	if err != nil {
		return "", "", fmt.Errorf("failed to encrypt access token: %w", err)
	}

	if tokens.RefreshToken != "" {
		refreshEncrypted, err = s.encryptor.Encrypt(tokens.RefreshToken)
		if err != nil {
			return "", "", fmt.Errorf("failed to encrypt refresh token: %w", err)
		}
	}

	return accessEncrypted, refreshEncrypted, nil
}

// DecryptTokens decrypts tokens from storage.
func (s *Service) DecryptTokens(accessEncrypted, refreshEncrypted string) (accessToken, refreshToken string, err error) {
	if s.encryptor == nil {
		return "", "", errors.New("encryptor not configured")
	}

	accessToken, err = s.encryptor.Decrypt(accessEncrypted)
	if err != nil {
		return "", "", fmt.Errorf("failed to decrypt access token: %w", err)
	}

	if refreshEncrypted != "" {
		refreshToken, err = s.encryptor.Decrypt(refreshEncrypted)
		if err != nil {
			return "", "", fmt.Errorf("failed to decrypt refresh token: %w", err)
		}
	}

	return accessToken, refreshToken, nil
}

// IsProviderEnabled checks if a provider is enabled.
func (s *Service) IsProviderEnabled(provider Provider) bool {
	switch provider {
	case ProviderGoogle:
		return s.config.Google.Enabled
	case ProviderMicrosoft:
		return s.config.Microsoft.Enabled
	default:
		return false
	}
}

// helper to make POST requests for token exchange
func (s *Service) tokenRequest(ctx context.Context, tokenURL string, data url.Values) (*Tokens, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("OAuth token request failed")
		return nil, ErrTokenExchange
	}

	var tokens Tokens
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Calculate expiry time
	tokens.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)

	return &tokens, nil
}

// helper to make GET requests for user info
func (s *Service) userInfoRequest(ctx context.Context, userInfoURL, accessToken string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user info request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("OAuth user info request failed")
		return nil, ErrUserInfoFetch
	}

	return body, nil
}
