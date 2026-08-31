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

type DashboardHandler struct {
	db *database.Pool
}

func NewDashboardHandler(db *database.Pool) *DashboardHandler {
	return &DashboardHandler{db: db}
}

// DashboardStats represents the main dashboard statistics
type DashboardStats struct {
	TotalClients    int `json:"total_clients"`
	ActiveClients   int `json:"active_clients"`
	InactiveClients int `json:"inactive_clients"`

	TotalServices      int `json:"total_services"`
	ServicesInProgress int `json:"services_in_progress"`
	ServicesOverdue    int `json:"services_overdue"`
	ServicesDueSoon    int `json:"services_due_soon"`
	ServicesCompleted  int `json:"services_completed"`

	TotalDocuments      int `json:"total_documents"`
	DocumentsRequested  int `json:"documents_requested"`
	DocumentsPending    int `json:"documents_pending"`
	DocumentsApproved   int `json:"documents_approved"`

	RecentActivity []ActivityItem `json:"recent_activity"`
}

// ActivityItem represents a recent activity entry
type ActivityItem struct {
	ID          uuid.UUID  `json:"id"`
	Action      string     `json:"action"`
	EntityType  string     `json:"entity_type"`
	EntityID    *uuid.UUID `json:"entity_id,omitempty"`
	Description string     `json:"description"`
	UserName    *string    `json:"user_name,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// GetStats returns dashboard statistics
// GET /api/v1/dashboard/stats
func (h *DashboardHandler) GetStats(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := c.Get(middleware.AuthRole)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	roleStr, _ := role.(string)
	isStaff := roleStr == "staff"

	var stats DashboardStats

	// Client stats
	if isStaff {
		// Staff only sees their assigned clients
		err := tenantDB.QueryRowScan(c, []interface{}{&stats.TotalClients, &stats.ActiveClients, &stats.InactiveClients}, `
			SELECT
				COUNT(*) as total,
				COUNT(*) FILTER (WHERE c.status = 'active') as active,
				COUNT(*) FILTER (WHERE c.status = 'inactive') as inactive
			FROM clients c
			INNER JOIN staff_clients sc ON c.id = sc.client_id
			WHERE c.tenant_id = $1 AND sc.staff_id = $2
		`, tenantID, userID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get client stats")
		}
	} else {
		err := tenantDB.QueryRowScan(c, []interface{}{&stats.TotalClients, &stats.ActiveClients, &stats.InactiveClients}, `
			SELECT
				COUNT(*) as total,
				COUNT(*) FILTER (WHERE status = 'active') as active,
				COUNT(*) FILTER (WHERE status = 'inactive') as inactive
			FROM clients
			WHERE tenant_id = $1
		`, tenantID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get client stats")
		}
	}

	// Service stats
	serviceQuery := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE s.status = 'in_progress') as in_progress,
			COUNT(*) FILTER (WHERE s.status NOT IN ('completed', 'cancelled') AND s.deadline < CURRENT_DATE) as overdue,
			COUNT(*) FILTER (WHERE s.status NOT IN ('completed', 'cancelled') AND s.deadline BETWEEN CURRENT_DATE AND CURRENT_DATE + INTERVAL '7 days') as due_soon,
			COUNT(*) FILTER (WHERE s.status = 'completed') as completed
		FROM services s
	`
	if isStaff {
		serviceQuery += `
			INNER JOIN staff_clients sc ON s.client_id = sc.client_id
			WHERE s.tenant_id = $1 AND sc.staff_id = $2
		`
		err := tenantDB.QueryRowScan(c, []interface{}{
			&stats.TotalServices, &stats.ServicesInProgress, &stats.ServicesOverdue,
			&stats.ServicesDueSoon, &stats.ServicesCompleted},
			serviceQuery, tenantID, userID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get service stats")
		}
	} else {
		serviceQuery += ` WHERE s.tenant_id = $1`
		err := tenantDB.QueryRowScan(c, []interface{}{
			&stats.TotalServices, &stats.ServicesInProgress, &stats.ServicesOverdue,
			&stats.ServicesDueSoon, &stats.ServicesCompleted},
			serviceQuery, tenantID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get service stats")
		}
	}

	// Document stats
	docQuery := `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE d.status = 'requested') as requested,
			COUNT(*) FILTER (WHERE d.status = 'pending_review') as pending,
			COUNT(*) FILTER (WHERE d.status = 'approved') as approved
		FROM documents d
	`
	if isStaff {
		docQuery += `
			INNER JOIN staff_clients sc ON d.client_id = sc.client_id
			WHERE d.tenant_id = $1 AND sc.staff_id = $2 AND d.client_id IS NOT NULL
		`
		err := tenantDB.QueryRowScan(c, []interface{}{
			&stats.TotalDocuments, &stats.DocumentsRequested,
			&stats.DocumentsPending, &stats.DocumentsApproved},
			docQuery, tenantID, userID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get document stats")
		}
	} else {
		docQuery += ` WHERE d.tenant_id = $1 AND d.client_id IS NOT NULL`
		err := tenantDB.QueryRowScan(c, []interface{}{
			&stats.TotalDocuments, &stats.DocumentsRequested,
			&stats.DocumentsPending, &stats.DocumentsApproved},
			docQuery, tenantID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get document stats")
		}
	}

	// Recent activity from audit logs
	err := tenantDB.Query(c, `
		SELECT al.id, al.action, al.entity_type, al.entity_id, u.name, al.created_at
		FROM audit_logs al
		LEFT JOIN users u ON al.user_id = u.id
		WHERE al.tenant_id = $1 AND al.success = true
		ORDER BY al.created_at DESC
		LIMIT 10
	`, []interface{}{tenantID}, func(rows pgx.Rows) error {
		var item ActivityItem
		err := rows.Scan(&item.ID, &item.Action, &item.EntityType, &item.EntityID, &item.UserName, &item.CreatedAt)
		if err != nil {
			return err
		}
		item.Description = formatActivityDescription(item.Action, item.EntityType)
		stats.RecentActivity = append(stats.RecentActivity, item)
		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to get recent activity")
	}

	c.JSON(http.StatusOK, stats)
}

// GetDeadlines returns upcoming deadlines
// GET /api/v1/dashboard/deadlines
func (h *DashboardHandler) GetDeadlines(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := c.Get(middleware.AuthRole)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	roleStr, _ := role.(string)
	isStaff := roleStr == "staff"

	query := `
		SELECT s.id, s.name, s.status, s.priority, s.deadline, s.client_id, c.company_name,
		       CASE
		           WHEN s.deadline < CURRENT_DATE THEN 'overdue'
		           WHEN s.deadline = CURRENT_DATE THEN 'today'
		           WHEN s.deadline <= CURRENT_DATE + INTERVAL '3 days' THEN 'urgent'
		           WHEN s.deadline <= CURRENT_DATE + INTERVAL '7 days' THEN 'soon'
		           ELSE 'upcoming'
		       END as urgency
		FROM services s
		LEFT JOIN clients c ON s.client_id = c.id
	`

	var args []interface{}
	if isStaff {
		query += `
			INNER JOIN staff_clients sc ON s.client_id = sc.client_id
			WHERE s.tenant_id = $1 AND sc.staff_id = $2
			  AND s.status NOT IN ('completed', 'cancelled')
			  AND s.deadline IS NOT NULL
		`
		args = []interface{}{tenantID, userID}
	} else {
		query += `
			WHERE s.tenant_id = $1
			  AND s.status NOT IN ('completed', 'cancelled')
			  AND s.deadline IS NOT NULL
		`
		args = []interface{}{tenantID}
	}

	query += ` ORDER BY s.deadline ASC LIMIT 20`

	var deadlines []map[string]interface{}
	err := tenantDB.Query(c, query, args, func(rows pgx.Rows) error {
		var id, clientID uuid.UUID
		var name, status, priority, urgency string
		var deadline time.Time
		var clientName *string

		if err := rows.Scan(&id, &name, &status, &priority, &deadline, &clientID, &clientName, &urgency); err != nil {
			return err
		}

		deadlines = append(deadlines, map[string]interface{}{
			"id":          id,
			"name":        name,
			"status":      status,
			"priority":    priority,
			"deadline":    deadline.Format("2006-01-02"),
			"client_id":   clientID,
			"client_name": clientName,
			"urgency":     urgency,
		})
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get deadlines")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deadlines": deadlines})
}

// GetPendingDocuments returns documents awaiting action
// GET /api/v1/dashboard/pending-documents
func (h *DashboardHandler) GetPendingDocuments(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := c.Get(middleware.AuthRole)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	roleStr, _ := role.(string)
	isStaff := roleStr == "staff"

	query := `
		SELECT d.id, d.name, d.status, d.client_id, c.company_name, d.requested_at, d.created_at
		FROM documents d
		LEFT JOIN clients c ON d.client_id = c.id
	`

	var args []interface{}
	if isStaff {
		query += `
			INNER JOIN staff_clients sc ON d.client_id = sc.client_id
			WHERE d.tenant_id = $1 AND sc.staff_id = $2
			  AND d.status IN ('requested', 'pending_review')
			  AND d.client_id IS NOT NULL
		`
		args = []interface{}{tenantID, userID}
	} else {
		query += `
			WHERE d.tenant_id = $1
			  AND d.status IN ('requested', 'pending_review')
			  AND d.client_id IS NOT NULL
		`
		args = []interface{}{tenantID}
	}

	query += ` ORDER BY d.created_at ASC LIMIT 20`

	var documents []map[string]interface{}
	err := tenantDB.Query(c, query, args, func(rows pgx.Rows) error {
		var id, clientID uuid.UUID
		var name, status string
		var clientName *string
		var requestedAt, createdAt *time.Time

		if err := rows.Scan(&id, &name, &status, &clientID, &clientName, &requestedAt, &createdAt); err != nil {
			return err
		}

		doc := map[string]interface{}{
			"id":          id,
			"name":        name,
			"status":      status,
			"client_id":   clientID,
			"client_name": clientName,
			"created_at":  createdAt,
		}
		if requestedAt != nil {
			doc["requested_at"] = requestedAt
		}

		documents = append(documents, doc)
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get pending documents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"documents": documents})
}

// GetClientWorkload returns staff workload distribution
// GET /api/v1/dashboard/workload
func (h *DashboardHandler) GetClientWorkload(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var workload []map[string]interface{}
	err := tenantDB.Query(c, `
		SELECT u.id, u.name,
		       COUNT(sc.client_id) as client_count,
		       COUNT(s.id) FILTER (WHERE s.status = 'in_progress') as active_services
		FROM users u
		LEFT JOIN staff_clients sc ON u.id = sc.staff_id
		LEFT JOIN services s ON sc.client_id = s.client_id AND s.staff_id = u.id
		WHERE u.tenant_id = $1 AND u.role = 'staff' AND u.deleted_at IS NULL
		GROUP BY u.id, u.name
		ORDER BY client_count DESC
	`, []interface{}{tenantID}, func(rows pgx.Rows) error {
		var id uuid.UUID
		var name string
		var clientCount, activeServices int

		if err := rows.Scan(&id, &name, &clientCount, &activeServices); err != nil {
			return err
		}

		workload = append(workload, map[string]interface{}{
			"staff_id":        id,
			"staff_name":      name,
			"client_count":    clientCount,
			"active_services": activeServices,
		})
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get workload")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"workload": workload})
}

// GetRecentClients returns recently added/modified clients
// GET /api/v1/dashboard/recent-clients
func (h *DashboardHandler) GetRecentClients(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := c.Get(middleware.AuthRole)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	roleStr, _ := role.(string)
	isStaff := roleStr == "staff"

	query := `
		SELECT c.id, c.company_name, c.contact_name, c.email, c.status, c.created_at, c.updated_at
		FROM clients c
	`

	var args []interface{}
	if isStaff {
		query += `
			INNER JOIN staff_clients sc ON c.id = sc.client_id
			WHERE c.tenant_id = $1 AND sc.staff_id = $2
		`
		args = []interface{}{tenantID, userID}
	} else {
		query += ` WHERE c.tenant_id = $1`
		args = []interface{}{tenantID}
	}

	query += ` ORDER BY c.updated_at DESC LIMIT 10`

	var clients []map[string]interface{}
	err := tenantDB.Query(c, query, args, func(rows pgx.Rows) error {
		var id uuid.UUID
		var companyName, contactName, email, status string
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&id, &companyName, &contactName, &email, &status, &createdAt, &updatedAt); err != nil {
			return err
		}

		clients = append(clients, map[string]interface{}{
			"id":           id,
			"company_name": companyName,
			"contact_name": contactName,
			"email":        email,
			"status":       status,
			"created_at":   createdAt,
			"updated_at":   updatedAt,
		})
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get recent clients")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"clients": clients})
}

// GetKanban returns services grouped by status for Kanban board
// GET /api/v1/dashboard/kanban
func (h *DashboardHandler) GetKanban(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)
	role, _ := c.Get(middleware.AuthRole)

	// Get TenantDB for RLS-protected operations
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	roleStr, _ := role.(string)
	isStaff := roleStr == "staff"

	query := `
		SELECT s.id, s.name, s.status, s.priority, s.deadline, s.kanban_position,
		       s.client_id, c.company_name, s.docs_required, s.docs_received
		FROM services s
		LEFT JOIN clients c ON s.client_id = c.id
	`

	var args []interface{}
	if isStaff {
		query += `
			INNER JOIN staff_clients sc ON s.client_id = sc.client_id
			WHERE s.tenant_id = $1 AND sc.staff_id = $2
			  AND s.status NOT IN ('completed', 'cancelled')
		`
		args = []interface{}{tenantID, userID}
	} else {
		query += `
			WHERE s.tenant_id = $1
			  AND s.status NOT IN ('completed', 'cancelled')
		`
		args = []interface{}{tenantID}
	}

	query += ` ORDER BY s.kanban_position ASC, s.created_at DESC`

	// Group by status
	kanban := map[string][]map[string]interface{}{
		"not_started": {},
		"in_progress": {},
		"review":      {},
		"waiting":     {},
	}

	err := tenantDB.Query(c, query, args, func(rows pgx.Rows) error {
		var id, clientID uuid.UUID
		var name, status, priority string
		var deadline *time.Time
		var kanbanPosition, docsRequired, docsReceived int
		var clientName *string

		if err := rows.Scan(&id, &name, &status, &priority, &deadline, &kanbanPosition,
			&clientID, &clientName, &docsRequired, &docsReceived); err != nil {
			return err
		}

		item := map[string]interface{}{
			"id":              id,
			"name":            name,
			"priority":        priority,
			"kanban_position": kanbanPosition,
			"client_id":       clientID,
			"client_name":     clientName,
			"docs_required":   docsRequired,
			"docs_received":   docsReceived,
		}
		if deadline != nil {
			item["deadline"] = deadline.Format("2006-01-02")
		}

		if column, exists := kanban[status]; exists {
			kanban[status] = append(column, item)
		}
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to get kanban data")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	c.JSON(http.StatusOK, kanban)
}

// formatActivityDescription creates a human-readable description for an activity
func formatActivityDescription(action, entityType string) string {
	actionDescriptions := map[string]string{
		"login":            "logged in",
		"logout":           "logged out",
		"client_create":    "created a client",
		"client_update":    "updated a client",
		"client_delete":    "deleted a client",
		"service_create":   "created a service",
		"service_update":   "updated a service",
		"service_complete": "completed a service",
		"document_create":  "requested a document",
		"document_upload":  "uploaded a document",
		"document_approve": "approved a document",
		"document_reject":  "rejected a document",
		"user_create":      "invited a user",
		"user_update":      "updated a user",
	}

	if desc, ok := actionDescriptions[action]; ok {
		return desc
	}
	return action + " " + entityType
}
