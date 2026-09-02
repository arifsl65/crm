package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// SubscriptionHandler handles subscription operations.
type SubscriptionHandler struct {
	db *database.Pool
}

// NewSubscriptionHandler creates a new subscription handler.
func NewSubscriptionHandler(db *database.Pool) *SubscriptionHandler {
	return &SubscriptionHandler{db: db}
}

// Subscription represents a tenant subscription.
type Subscription struct {
	ID                   uuid.UUID  `json:"id"`
	TenantID             uuid.UUID  `json:"tenant_id"`
	StripeCustomerID     string     `json:"stripe_customer_id"`
	StripeSubscriptionID *string    `json:"stripe_subscription_id,omitempty"`
	Plan                 string     `json:"plan"` // starter, professional, enterprise
	Status               string     `json:"status"` // trialing, active, past_due, canceled, unpaid
	CurrentPeriodStart   *time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd     *time.Time `json:"current_period_end,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// Invoice represents a tenant invoice.
type Invoice struct {
	ID              uuid.UUID  `json:"id"`
	TenantID        uuid.UUID  `json:"tenant_id"`
	StripeInvoiceID string     `json:"stripe_invoice_id"`
	AmountDue       int        `json:"amount_due"`
	AmountPaid      int        `json:"amount_paid"`
	Currency        string     `json:"currency"`
	Status          string     `json:"status"` // draft, open, paid, void, uncollectible
	InvoicePDF      *string    `json:"invoice_pdf,omitempty"`
	PeriodStart     *time.Time `json:"period_start,omitempty"`
	PeriodEnd       *time.Time `json:"period_end,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// Get returns the current tenant's subscription.
// GET /api/v1/subscription
func (h *SubscriptionHandler) Get(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var sub Subscription
	err := tenantDB.QueryRowScan(c, []interface{}{
		&sub.ID, &sub.TenantID, &sub.StripeCustomerID, &sub.StripeSubscriptionID,
		&sub.Plan, &sub.Status, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd,
		&sub.CreatedAt, &sub.UpdatedAt,
	}, `
		SELECT id, tenant_id, stripe_customer_id, stripe_subscription_id,
		       plan, status, current_period_start, current_period_end,
		       created_at, updated_at
		FROM tenant_subscriptions
		WHERE tenant_id = $1
	`, tenantID)

	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "No subscription found"})
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("Failed to get subscription")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"subscription": sub})
}

// ListInvoices returns invoices for the current tenant.
// GET /api/v1/subscription/invoices
func (h *SubscriptionHandler) ListInvoices(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var invoices []Invoice
	err := tenantDB.Query(c, `
		SELECT id, tenant_id, stripe_invoice_id, amount_due, amount_paid,
		       currency, status, invoice_pdf, period_start, period_end, created_at
		FROM tenant_invoices
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT 50
	`, []interface{}{tenantID}, func(rows pgx.Rows) error {
		var inv Invoice
		err := rows.Scan(
			&inv.ID, &inv.TenantID, &inv.StripeInvoiceID, &inv.AmountDue, &inv.AmountPaid,
			&inv.Currency, &inv.Status, &inv.InvoicePDF, &inv.PeriodStart, &inv.PeriodEnd,
			&inv.CreatedAt,
		)
		if err != nil {
			return err
		}
		invoices = append(invoices, inv)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to list invoices")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch invoices"})
		return
	}

	if invoices == nil {
		invoices = []Invoice{}
	}

	c.JSON(http.StatusOK, gin.H{
		"invoices": invoices,
		"count":    len(invoices),
	})
}

// GetUsage returns usage statistics for the current tenant.
// GET /api/v1/subscription/usage
func (h *SubscriptionHandler) GetUsage(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Get counts for various entities
	var clientCount, userCount, serviceCount, documentCount, emailCount int

	// Count clients
	_ = tenantDB.QueryRowScan(c, []interface{}{&clientCount},
		`SELECT COUNT(*) FROM clients WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID)

	// Count users
	_ = tenantDB.QueryRowScan(c, []interface{}{&userCount},
		`SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND deleted_at IS NULL`, tenantID)

	// Count services
	_ = tenantDB.QueryRowScan(c, []interface{}{&serviceCount},
		`SELECT COUNT(*) FROM services WHERE tenant_id = $1`, tenantID)

	// Count documents
	_ = tenantDB.QueryRowScan(c, []interface{}{&documentCount},
		`SELECT COUNT(*) FROM documents WHERE tenant_id = $1`, tenantID)

	// Count emails (this month)
	_ = tenantDB.QueryRowScan(c, []interface{}{&emailCount},
		`SELECT COUNT(*) FROM emails WHERE tenant_id = $1 AND created_at >= date_trunc('month', NOW())`, tenantID)

	c.JSON(http.StatusOK, gin.H{
		"usage": gin.H{
			"clients":        clientCount,
			"users":          userCount,
			"services":       serviceCount,
			"documents":      documentCount,
			"emails_monthly": emailCount,
		},
	})
}

// CreatePortalSession creates a Stripe billing portal session.
// POST /api/v1/subscription/portal
func (h *SubscriptionHandler) CreatePortalSession(c *gin.Context) {
	// This would integrate with Stripe to create a billing portal session
	// For now, return a placeholder response
	c.JSON(http.StatusOK, gin.H{
		"message": "Stripe billing portal integration pending",
		"url":     nil,
	})
}

// CreateCheckoutSession creates a Stripe checkout session for plan upgrade.
// POST /api/v1/subscription/checkout
func (h *SubscriptionHandler) CreateCheckoutSession(c *gin.Context) {
	var req struct {
		Plan string `json:"plan" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate plan
	validPlans := map[string]bool{
		"starter":      true,
		"professional": true,
		"enterprise":   true,
	}

	if !validPlans[req.Plan] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plan"})
		return
	}

	// This would integrate with Stripe to create a checkout session
	// For now, return a placeholder response
	c.JSON(http.StatusOK, gin.H{
		"message": "Stripe checkout integration pending",
		"plan":    req.Plan,
		"url":     nil,
	})
}
