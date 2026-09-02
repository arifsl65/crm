package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

const (
	// Microsoft OAuth endpoints (common tenant for personal + work accounts)
	microsoftAuthURL     = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	microsoftTokenURL    = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	microsoftUserInfoURL = "https://graph.microsoft.com/v1.0/me"

	// Microsoft OAuth scopes for email access
	microsoftScopeOpenID  = "openid"
	microsoftScopeProfile = "profile"
	microsoftScopeEmail   = "email"
	microsoftScopeMail    = "Mail.Read"
	microsoftScopeOffline = "offline_access" // Required for refresh token
)

// getMicrosoftAuthURL returns the Microsoft OAuth authorization URL.
func (s *Service) getMicrosoftAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", s.config.Microsoft.ClientID)
	params.Set("redirect_uri", s.config.Microsoft.RedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", fmt.Sprintf("%s %s %s %s %s",
		microsoftScopeOpenID,
		microsoftScopeProfile,
		microsoftScopeEmail,
		microsoftScopeMail,
		microsoftScopeOffline,
	))
	params.Set("state", state)
	params.Set("response_mode", "query")

	return fmt.Sprintf("%s?%s", microsoftAuthURL, params.Encode())
}

// exchangeMicrosoftCode exchanges a Microsoft authorization code for tokens.
func (s *Service) exchangeMicrosoftCode(ctx context.Context, code string) (*Tokens, error) {
	data := url.Values{}
	data.Set("client_id", s.config.Microsoft.ClientID)
	data.Set("client_secret", s.config.Microsoft.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", s.config.Microsoft.RedirectURL)
	data.Set("grant_type", "authorization_code")

	return s.tokenRequest(ctx, microsoftTokenURL, data)
}

// getMicrosoftUserInfo fetches user information from Microsoft Graph.
func (s *Service) getMicrosoftUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	body, err := s.userInfoRequest(ctx, microsoftUserInfoURL, accessToken)
	if err != nil {
		return nil, err
	}

	var msUser struct {
		ID                string `json:"id"`
		DisplayName       string `json:"displayName"`
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
	}

	if err := json.Unmarshal(body, &msUser); err != nil {
		return nil, fmt.Errorf("failed to parse Microsoft user info: %w", err)
	}

	// Microsoft uses 'mail' for email, fallback to userPrincipalName
	email := msUser.Mail
	if email == "" {
		email = msUser.UserPrincipalName
	}

	return &UserInfo{
		Email:         email,
		Name:          msUser.DisplayName,
		EmailVerified: true, // Microsoft accounts are always verified
	}, nil
}

// refreshMicrosoftToken refreshes a Microsoft access token.
func (s *Service) refreshMicrosoftToken(ctx context.Context, refreshToken string) (*Tokens, error) {
	data := url.Values{}
	data.Set("client_id", s.config.Microsoft.ClientID)
	data.Set("client_secret", s.config.Microsoft.ClientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	tokens, err := s.tokenRequest(ctx, microsoftTokenURL, data)
	if err != nil {
		return nil, err
	}

	// Microsoft returns a new refresh token, use it if provided
	if tokens.RefreshToken == "" {
		tokens.RefreshToken = refreshToken
	}

	return tokens, nil
}
