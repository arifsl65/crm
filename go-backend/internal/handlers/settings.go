package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/audit"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// SettingsHandler handles company settings operations.
type SettingsHandler struct {
	db    *database.Pool
	audit *audit.Logger
}

// NewSettingsHandler creates a new settings handler.
func NewSettingsHandler(db *database.Pool, auditLogger *audit.Logger) *SettingsHandler {
	return &SettingsHandler{
		db:    db,
		audit: auditLogger,
	}
}

// CompanySettings represents company/tenant settings.
type CompanySettings struct {
	ID              uuid.UUID              `json:"id"`
	TenantID        uuid.UUID              `json:"tenant_id"`
	FirmName        string                 `json:"firm_name"`
	Email           string                 `json:"email"`
	Phone           *string                `json:"phone,omitempty"`
	Address         *string                `json:"address,omitempty"`
	LogoURL         *string                `json:"logo_url,omitempty"`
	StripeAccountID *string                `json:"stripe_account_id,omitempty"`
	StripeConnected bool                   `json:"stripe_connected"`
	ReminderRules   map[string]interface{} `json:"reminder_rules,omitempty"`
	UpdatedBy       *uuid.UUID             `json:"updated_by,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// UpdateSettingsRequest represents the request to update settings.
type UpdateSettingsRequest struct {
	FirmName      *string                `json:"firm_name,omitempty"`
	Email         *string                `json:"email,omitempty"`
	Phone         *string                `json:"phone,omitempty"`
	Address       *string                `json:"address,omitempty"`
	LogoURL       *string                `json:"logo_url,omitempty"`
	ReminderRules map[string]interface{} `json:"reminder_rules,omitempty"`
}

// Get returns the company settings for the current tenant.
// GET /api/v1/settings
func (h *SettingsHandler) Get(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var settings CompanySettings
	var reminderRulesJSON []byte

	err := tenantDB.QueryRowScan(c, []interface{}{
		&settings.ID, &settings.TenantID, &settings.FirmName, &settings.Email,
		&settings.Phone, &settings.Address, &settings.LogoURL,
		&settings.StripeAccountID, &settings.StripeConnected,
		&reminderRulesJSON, &settings.UpdatedBy, &settings.CreatedAt, &settings.UpdatedAt,
	}, `
		SELECT id, tenant_id, firm_name, email, phone, address, logo_url,
		       stripe_account_id, stripe_connected, reminder_rules,
		       updated_by, created_at, updated_at
		FROM company_settings
		WHERE tenant_id = $1
	`, tenantID)

	if err == pgx.ErrNoRows {
		// Create default settings if they don't exist
		settings = CompanySettings{
			ID:              uuid.New(),
			TenantID:        tenantID,
			FirmName:        "My Firm",
			Email:           "contact@example.com",
			StripeConnected: false,
			ReminderRules:   map[string]interface{}{"day3": true, "day7": true, "day14": true},
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}

		_, err = tenantDB.Exec(c, `
			INSERT INTO company_settings (id, tenant_id, firm_name, email, stripe_connected, reminder_rules, created_at, updated_at)
			VALUES ($1, $2, $3, $4, false, '{"day3": true, "day7": true, "day14": true}', $5, $6)
		`, settings.ID, tenantID, settings.FirmName, settings.Email, settings.CreatedAt, settings.UpdatedAt)

		if err != nil {
			log.Error().Err(err).Msg("Failed to create default settings")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create settings"})
			return
		}
	} else if err != nil {
		log.Error().Err(err).Msg("Failed to get settings")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	} else {
		// Parse reminder rules JSON
		if len(reminderRulesJSON) > 0 {
			var rules map[string]interface{}
			if err := json.Unmarshal(reminderRulesJSON, &rules); err == nil {
				settings.ReminderRules = rules
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

// Update updates the company settings.
// PATCH /api/v1/settings
func (h *SettingsHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}
	argIdx := 1

	if req.FirmName != nil {
		updates = append(updates, "firm_name = $"+strconv.Itoa(argIdx))
		args = append(args, *req.FirmName)
		argIdx++
	}
	if req.Email != nil {
		updates = append(updates, "email = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Email)
		argIdx++
	}
	if req.Phone != nil {
		updates = append(updates, "phone = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Phone)
		argIdx++
	}
	if req.Address != nil {
		updates = append(updates, "address = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Address)
		argIdx++
	}
	if req.LogoURL != nil {
		updates = append(updates, "logo_url = $"+strconv.Itoa(argIdx))
		args = append(args, *req.LogoURL)
		argIdx++
	}
	if req.ReminderRules != nil {
		rulesJSON, err := json.Marshal(req.ReminderRules)
		if err == nil {
			updates = append(updates, "reminder_rules = $"+strconv.Itoa(argIdx))
			args = append(args, rulesJSON)
			argIdx++
		}
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	// Add updated_by and updated_at
	updates = append(updates, "updated_by = $"+strconv.Itoa(argIdx))
	args = append(args, userID)
	argIdx++
	updates = append(updates, "updated_at = $"+strconv.Itoa(argIdx))
	args = append(args, time.Now())
	argIdx++

	// Add tenant_id to WHERE clause
	args = append(args, tenantID)

	query := "UPDATE company_settings SET " + strings.Join(updates, ", ") + " WHERE tenant_id = $" + strconv.Itoa(argIdx)

	result, err := tenantDB.Exec(c, query, args...)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update settings")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Settings not found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionSettingsUpdate, &userID, &tenantID, "company_settings", nil, c.ClientIP(), map[string]interface{}{
		"fields_updated": len(updates) - 2, // Exclude updated_by and updated_at
	})

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}

// GetBranding returns branding-specific settings (logo, firm name, colors).
// GET /api/v1/settings/branding
func (h *SettingsHandler) GetBranding(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var branding struct {
		FirmName string  `json:"firm_name"`
		LogoURL  *string `json:"logo_url"`
		Email    string  `json:"email"`
		Phone    *string `json:"phone"`
	}

	err := tenantDB.QueryRowScan(c, []interface{}{
		&branding.FirmName, &branding.LogoURL, &branding.Email, &branding.Phone,
	}, `
		SELECT firm_name, logo_url, email, phone
		FROM company_settings
		WHERE tenant_id = $1
	`, tenantID)

	if err == pgx.ErrNoRows {
		branding.FirmName = "My Firm"
		branding.Email = "contact@example.com"
	} else if err != nil {
		log.Error().Err(err).Msg("Failed to get branding")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch branding"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"branding": branding})
}

// UpdateBranding updates branding-specific settings.
// PATCH /api/v1/settings/branding
func (h *SettingsHandler) UpdateBranding(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req struct {
		FirmName *string `json:"firm_name,omitempty"`
		LogoURL  *string `json:"logo_url,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.FirmName == nil && req.LogoURL == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}

	// Build update query
	updates := []string{"updated_by = $1", "updated_at = $2"}
	args := []interface{}{userID, time.Now()}
	argIdx := 3

	if req.FirmName != nil {
		updates = append(updates, "firm_name = $"+strconv.Itoa(argIdx))
		args = append(args, *req.FirmName)
		argIdx++
	}
	if req.LogoURL != nil {
		updates = append(updates, "logo_url = $"+strconv.Itoa(argIdx))
		args = append(args, *req.LogoURL)
		argIdx++
	}

	args = append(args, tenantID)
	query := "UPDATE company_settings SET " + strings.Join(updates, ", ") + " WHERE tenant_id = $" + strconv.Itoa(argIdx)

	result, err := tenantDB.Exec(c, query, args...)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update branding")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update branding"})
		return
	}

	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Settings not found"})
		return
	}

	// Audit log
	h.audit.LogEntity(ctx, audit.ActionSettingsUpdate, &userID, &tenantID, "company_settings", nil, c.ClientIP(), map[string]interface{}{
		"type": "branding",
	})

	c.JSON(http.StatusOK, gin.H{"message": "Branding updated successfully"})
}
