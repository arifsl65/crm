package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"

	// Google OAuth scopes for email access
	googleScopeEmail   = "https://www.googleapis.com/auth/userinfo.email"
	googleScopeProfile = "https://www.googleapis.com/auth/userinfo.profile"
	googleScopeGmail   = "https://www.googleapis.com/auth/gmail.readonly"
)

// getGoogleAuthURL returns the Google OAuth authorization URL.
func (s *Service) getGoogleAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", s.config.Google.ClientID)
	params.Set("redirect_uri", s.config.Google.RedirectURL)
	params.Set("response_type", "code")
	params.Set("scope", fmt.Sprintf("%s %s %s", googleScopeEmail, googleScopeProfile, googleScopeGmail))
	params.Set("state", state)
	params.Set("access_type", "offline") // Required to get refresh token
	params.Set("prompt", "consent")      // Force consent to get refresh token

	return fmt.Sprintf("%s?%s", googleAuthURL, params.Encode())
}

// exchangeGoogleCode exchanges a Google authorization code for tokens.
func (s *Service) exchangeGoogleCode(ctx context.Context, code string) (*Tokens, error) {
	data := url.Values{}
	data.Set("client_id", s.config.Google.ClientID)
	data.Set("client_secret", s.config.Google.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", s.config.Google.RedirectURL)
	data.Set("grant_type", "authorization_code")

	return s.tokenRequest(ctx, googleTokenURL, data)
}

// getGoogleUserInfo fetches user information from Google.
func (s *Service) getGoogleUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	body, err := s.userInfoRequest(ctx, googleUserInfoURL, accessToken)
	if err != nil {
		return nil, err
	}

	var googleUser struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}

	if err := json.Unmarshal(body, &googleUser); err != nil {
		return nil, fmt.Errorf("failed to parse Google user info: %w", err)
	}

	return &UserInfo{
		Email:         googleUser.Email,
		Name:          googleUser.Name,
		Picture:       googleUser.Picture,
		EmailVerified: googleUser.VerifiedEmail,
	}, nil
}

// refreshGoogleToken refreshes a Google access token.
func (s *Service) refreshGoogleToken(ctx context.Context, refreshToken string) (*Tokens, error) {
	data := url.Values{}
	data.Set("client_id", s.config.Google.ClientID)
	data.Set("client_secret", s.config.Google.ClientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	tokens, err := s.tokenRequest(ctx, googleTokenURL, data)
	if err != nil {
		return nil, err
	}

	// Google doesn't return refresh token on refresh, keep the original
	tokens.RefreshToken = refreshToken

	return tokens, nil
}
