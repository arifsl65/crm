package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
	ErrTokenReused  = errors.New("token reuse detected")
	ErrTokenRevoked = errors.New("token has been revoked")
)

type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

type Claims struct {
	jwt.RegisteredClaims
	UserID   uuid.UUID `json:"user_id"`
	TenantID uuid.UUID `json:"tenant_id"`
	Role     string    `json:"role"`
	Type     TokenType `json:"type"`
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type JWTManager struct {
	secretKey          []byte
	accessTokenExpire  time.Duration
	refreshTokenExpire time.Duration
	issuer             string
}

func NewJWTManager(secretKey string, accessExpire, refreshExpire time.Duration, issuer string) *JWTManager {
	return &JWTManager{
		secretKey:          []byte(secretKey),
		accessTokenExpire:  accessExpire,
		refreshTokenExpire: refreshExpire,
		issuer:             issuer,
	}
}

func (m *JWTManager) GenerateTokenPair(userID, tenantID uuid.UUID, role string) (*TokenPair, error) {
	now := time.Now()
	accessExpiry := now.Add(m.accessTokenExpire)

	accessToken, err := m.generateToken(userID, tenantID, role, AccessToken, accessExpiry)
	if err != nil {
		return nil, err
	}

	refreshExpiry := now.Add(m.refreshTokenExpire)
	refreshToken, err := m.generateToken(userID, tenantID, role, RefreshToken, refreshExpiry)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessExpiry,
	}, nil
}

func (m *JWTManager) generateToken(userID, tenantID uuid.UUID, role string, tokenType TokenType, expiry time.Time) (string, error) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
		UserID:   userID,
		TenantID: tenantID,
		Role:     role,
		Type:     tokenType,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secretKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func (m *JWTManager) RefreshAccessToken(refreshTokenString string) (*TokenPair, error) {
	claims, err := m.ValidateToken(refreshTokenString)
	if err != nil {
		return nil, err
	}

	if claims.Type != RefreshToken {
		return nil, ErrInvalidToken
	}

	return m.GenerateTokenPair(claims.UserID, claims.TenantID, claims.Role)
}
