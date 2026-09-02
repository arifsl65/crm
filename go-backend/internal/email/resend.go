package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client handles email sending via Resend API.
type Client struct {
	apiKey     string
	fromEmail  string
	fromName   string
	baseURL    string
	httpClient *http.Client
}

// Config holds email client configuration.
type Config struct {
	APIKey    string
	FromEmail string
	FromName  string
}

// NewClient creates a new Resend email client.
func NewClient(cfg Config) *Client {
	return &Client{
		apiKey:    cfg.APIKey,
		fromEmail: cfg.FromEmail,
		fromName:  cfg.FromName,
		baseURL:   "https://api.resend.com",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendRequest represents a Resend email request.
type SendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html,omitempty"`
	Text    string   `json:"text,omitempty"`
}

// SendResponse represents a Resend API response.
type SendResponse struct {
	ID string `json:"id"`
}

// Send sends an email via Resend and returns the Resend ID.
func (c *Client) Send(to, subject, html string) error {
	_, err := c.SendWithID(to, subject, html, "")
	return err
}

// SendWithID sends an email via Resend and returns the Resend ID.
func (c *Client) SendWithID(to, subject, html, text string) (string, error) {
	req := SendRequest{
		From:    fmt.Sprintf("%s <%s>", c.fromName, c.fromEmail),
		To:      []string{to},
		Subject: subject,
		HTML:    html,
		Text:    text,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("resend API returned status %d", resp.StatusCode)
	}

	var sendResp SendResponse
	if err := json.NewDecoder(resp.Body).Decode(&sendResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return sendResp.ID, nil
}

// GetFromEmail returns the configured from email address.
func (c *Client) GetFromEmail() string {
	return c.fromEmail
}

// GetFromName returns the configured from name.
func (c *Client) GetFromName() string {
	return c.fromName
}

// SendPasswordReset sends a password reset email.
func (c *Client) SendPasswordReset(to, resetURL, userName string) error {
	subject := "Reset Your Password"
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
    <div style="background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); padding: 30px; text-align: center; border-radius: 10px 10px 0 0;">
        <h1 style="color: white; margin: 0; font-size: 24px;">Password Reset</h1>
    </div>
    <div style="background: #ffffff; padding: 30px; border: 1px solid #e0e0e0; border-top: none; border-radius: 0 0 10px 10px;">
        <p style="margin-top: 0;">Hi %s,</p>
        <p>We received a request to reset your password. Click the button below to create a new password:</p>
        <div style="text-align: center; margin: 30px 0;">
            <a href="%s" style="background: #667eea; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; font-weight: bold; display: inline-block;">Reset Password</a>
        </div>
        <p style="color: #666; font-size: 14px;">This link will expire in 1 hour. If you didn't request this, you can safely ignore this email.</p>
        <hr style="border: none; border-top: 1px solid #e0e0e0; margin: 20px 0;">
        <p style="color: #999; font-size: 12px; margin-bottom: 0;">If the button doesn't work, copy and paste this link into your browser:</p>
        <p style="color: #667eea; font-size: 12px; word-break: break-all;">%s</p>
    </div>
</body>
</html>
`, userName, resetURL, resetURL)

	return c.Send(to, subject, html)
}

// SendMagicLink sends a passwordless login email.
func (c *Client) SendMagicLink(to, loginURL, userName string) error {
	subject := "Sign in to Accountant CRM"
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
    <div style="background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); padding: 30px; text-align: center; border-radius: 10px 10px 0 0;">
        <h1 style="color: white; margin: 0; font-size: 24px;">Sign In</h1>
    </div>
    <div style="background: #ffffff; padding: 30px; border: 1px solid #e0e0e0; border-top: none; border-radius: 0 0 10px 10px;">
        <p style="margin-top: 0;">Hi %s,</p>
        <p>Click the button below to sign in to your account:</p>
        <div style="text-align: center; margin: 30px 0;">
            <a href="%s" style="background: #667eea; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; font-weight: bold; display: inline-block;">Sign In</a>
        </div>
        <p style="color: #666; font-size: 14px;">This link will expire in 15 minutes. If you didn't request this, you can safely ignore this email.</p>
    </div>
</body>
</html>
`, userName, loginURL)

	return c.Send(to, subject, html)
}

// SendInvite sends a user invitation email.
func (c *Client) SendInvite(to, inviteURL, inviterName, orgName string) error {
	subject := fmt.Sprintf("You've been invited to join %s", orgName)
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
    <div style="background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); padding: 30px; text-align: center; border-radius: 10px 10px 0 0;">
        <h1 style="color: white; margin: 0; font-size: 24px;">You're Invited!</h1>
    </div>
    <div style="background: #ffffff; padding: 30px; border: 1px solid #e0e0e0; border-top: none; border-radius: 0 0 10px 10px;">
        <p style="margin-top: 0;">Hi there,</p>
        <p><strong>%s</strong> has invited you to join <strong>%s</strong> on Accountant CRM.</p>
        <div style="text-align: center; margin: 30px 0;">
            <a href="%s" style="background: #667eea; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; font-weight: bold; display: inline-block;">Accept Invitation</a>
        </div>
        <p style="color: #666; font-size: 14px;">This invitation will expire in 7 days.</p>
    </div>
</body>
</html>
`, inviterName, orgName, inviteURL)

	return c.Send(to, subject, html)
}

// IsConfigured returns true if the email client is properly configured.
func (c *Client) IsConfigured() bool {
	return c.apiKey != "" && c.fromEmail != ""
}

// SendESignRequest sends an e-signature request email to the signer.
func (c *Client) SendESignRequest(to, signerName, clientName, templateType, signingURL string, expiresAt *time.Time) error {
	// Format the template type for display
	templateDisplay := templateType
	switch templateType {
	case "engagement":
		templateDisplay = "Engagement Letter"
	case "service_agreement":
		templateDisplay = "Service Agreement"
	case "gdpr_consent":
		templateDisplay = "GDPR Consent Form"
	}

	// Format expiry date
	expiryText := "14 days"
	if expiresAt != nil {
		expiryText = expiresAt.Format("January 2, 2006")
	}

	subject := fmt.Sprintf("Action Required: Please sign your %s", templateDisplay)
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
    <div style="background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); padding: 30px; text-align: center; border-radius: 10px 10px 0 0;">
        <h1 style="color: white; margin: 0; font-size: 24px;">Document Signing Request</h1>
    </div>
    <div style="background: #ffffff; padding: 30px; border: 1px solid #e0e0e0; border-top: none; border-radius: 0 0 10px 10px;">
        <p style="margin-top: 0;">Hi %s,</p>
        <p>You have been requested to sign a <strong>%s</strong> for <strong>%s</strong>.</p>
        <p>Please review and sign the document by clicking the button below:</p>
        <div style="text-align: center; margin: 30px 0;">
            <a href="%s" style="background: #667eea; color: white; padding: 14px 35px; text-decoration: none; border-radius: 5px; font-weight: bold; display: inline-block; font-size: 16px;">Review & Sign Document</a>
        </div>
        <div style="background: #f8f9fa; padding: 15px; border-radius: 5px; margin: 20px 0;">
            <p style="margin: 0; color: #666; font-size: 14px;">
                <strong>⏰ Expires:</strong> %s<br>
                <strong>📧 Sent to:</strong> %s
            </p>
        </div>
        <p style="color: #666; font-size: 14px;">If you have any questions about this document, please contact the sender directly.</p>
        <hr style="border: none; border-top: 1px solid #e0e0e0; margin: 20px 0;">
        <p style="color: #999; font-size: 12px; margin-bottom: 0;">If the button doesn't work, copy and paste this link into your browser:</p>
        <p style="color: #667eea; font-size: 12px; word-break: break-all;">%s</p>
        <p style="color: #999; font-size: 11px; margin-top: 20px;">This is an automated message from Accountant CRM. Please do not reply to this email.</p>
    </div>
</body>
</html>
`, signerName, templateDisplay, clientName, signingURL, expiryText, to, signingURL)

	return c.Send(to, subject, html)
}
