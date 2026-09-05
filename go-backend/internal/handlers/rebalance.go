package handlers

import (
	"math"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// RebalanceHandler handles staff workload rebalancing operations.
type RebalanceHandler struct {
	db *database.Pool
}

// NewRebalanceHandler creates a new rebalance handler.
func NewRebalanceHandler(db *database.Pool) *RebalanceHandler {
	return &RebalanceHandler{db: db}
}

// StaffWorkload represents a staff member's current workload.
type StaffWorkload struct {
	StaffID          uuid.UUID `json:"staff_id"`
	StaffName        string    `json:"staff_name"`
	Email            string    `json:"email"`
	Specialism       *string   `json:"specialism,omitempty"`
	ClientCount      int       `json:"client_count"`
	ServiceCount     int       `json:"service_count"`
	PendingServices  int       `json:"pending_services"`
	OverdueServices  int       `json:"overdue_services"`
	UpcomingDeadlines int      `json:"upcoming_deadlines"` // Due in next 7 days
	WorkloadScore    float64   `json:"workload_score"`     // Calculated score
	Capacity         string    `json:"capacity"`           // "low", "medium", "high", "overloaded"
}

// RebalanceSuggestion represents a suggested client reassignment.
type RebalanceSuggestion struct {
	ClientID        uuid.UUID `json:"client_id"`
	ClientName      string    `json:"client_name"`
	FromStaffID     uuid.UUID `json:"from_staff_id"`
	FromStaffName   string    `json:"from_staff_name"`
	ToStaffID       uuid.UUID `json:"to_staff_id"`
	ToStaffName     string    `json:"to_staff_name"`
	Reason          string    `json:"reason"`
	ImpactScore     float64   `json:"impact_score"` // How much this improves balance
	ServiceCount    int       `json:"service_count"`
	PendingServices int       `json:"pending_services"`
}

// RebalanceResponse represents the rebalance analysis response.
type RebalanceResponse struct {
	Workloads       []StaffWorkload       `json:"workloads"`
	Suggestions     []RebalanceSuggestion `json:"suggestions"`
	BalanceScore    float64               `json:"balance_score"`    // 0-100, higher is better
	ImprovedScore   float64               `json:"improved_score"`   // Score after applying suggestions
	TotalClients    int                   `json:"total_clients"`
	TotalStaff      int                   `json:"total_staff"`
	OverloadedStaff int                   `json:"overloaded_staff"`
	UnderutilizedStaff int               `json:"underutilized_staff"`
}

// ApplyRebalanceRequest represents a request to apply rebalance suggestions.
type ApplyRebalanceRequest struct {
	Assignments []struct {
		ClientID  uuid.UUID `json:"client_id" binding:"required"`
		ToStaffID uuid.UUID `json:"to_staff_id" binding:"required"`
	} `json:"assignments" binding:"required,min=1"`
}

// GetWorkloads returns current staff workload analysis.
// GET /api/v1/ai/staff/workload
func (h *RebalanceHandler) GetWorkloads(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	workloads, err := h.calculateWorkloads(c, tenantDB, tenantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to calculate workloads")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to analyze workloads"})
		return
	}

	// Calculate balance score
	balanceScore := calculateBalanceScore(workloads)
	overloaded := 0
	underutilized := 0
	for _, w := range workloads {
		if w.Capacity == "overloaded" {
			overloaded++
		} else if w.Capacity == "low" {
			underutilized++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"workloads":           workloads,
		"balance_score":       balanceScore,
		"total_staff":         len(workloads),
		"overloaded_staff":    overloaded,
		"underutilized_staff": underutilized,
	})
}

// Rebalance analyzes workloads and suggests optimal reassignments.
// POST /api/v1/ai/staff/rebalance
func (h *RebalanceHandler) Rebalance(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Get current workloads
	workloads, err := h.calculateWorkloads(c, tenantDB, tenantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to calculate workloads")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to analyze workloads"})
		return
	}

	if len(workloads) < 2 {
		c.JSON(http.StatusOK, RebalanceResponse{
			Workloads:    workloads,
			Suggestions:  []RebalanceSuggestion{},
			BalanceScore: 100,
			TotalStaff:   len(workloads),
		})
		return
	}

	// Get clients with their workload details
	clients, err := h.getClientsWithWorkload(c, tenantDB, tenantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get client workloads")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to analyze clients"})
		return
	}

	// Generate suggestions
	suggestions := h.generateSuggestions(workloads, clients)

	// Calculate scores
	currentScore := calculateBalanceScore(workloads)
	improvedScore := simulateImprovedScore(workloads, suggestions)

	overloaded := 0
	underutilized := 0
	for _, w := range workloads {
		if w.Capacity == "overloaded" {
			overloaded++
		} else if w.Capacity == "low" {
			underutilized++
		}
	}

	c.JSON(http.StatusOK, RebalanceResponse{
		Workloads:          workloads,
		Suggestions:        suggestions,
		BalanceScore:       currentScore,
		ImprovedScore:      improvedScore,
		TotalClients:       len(clients),
		TotalStaff:         len(workloads),
		OverloadedStaff:    overloaded,
		UnderutilizedStaff: underutilized,
	})
}

// ApplyRebalance applies the suggested reassignments.
// POST /api/v1/ai/staff/rebalance/apply
func (h *RebalanceHandler) ApplyRebalance(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)
	userID, _ := middleware.GetUserID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	var req ApplyRebalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	applied := 0
	errors := []string{}

	for _, assignment := range req.Assignments {
		// Update staff_clients table - set new primary staff
		// First, remove existing primary assignment
		_, err := tenantDB.Exec(c, `
			UPDATE staff_clients
			SET is_primary = false, updated_at = NOW()
			WHERE client_id = $1 AND tenant_id = $2 AND is_primary = true
		`, assignment.ClientID, tenantID)
		if err != nil {
			errors = append(errors, "Failed to update client "+assignment.ClientID.String())
			continue
		}

		// Check if new staff already has this client
		var exists bool
		err = tenantDB.QueryRowScan(c, []interface{}{&exists}, `
			SELECT EXISTS(
				SELECT 1 FROM staff_clients
				WHERE client_id = $1 AND staff_id = $2 AND tenant_id = $3
			)
		`, assignment.ClientID, assignment.ToStaffID, tenantID)
		if err != nil {
			errors = append(errors, "Failed to check assignment for "+assignment.ClientID.String())
			continue
		}

		if exists {
			// Update existing assignment to primary
			_, err = tenantDB.Exec(c, `
				UPDATE staff_clients
				SET is_primary = true, updated_at = NOW()
				WHERE client_id = $1 AND staff_id = $2 AND tenant_id = $3
			`, assignment.ClientID, assignment.ToStaffID, tenantID)
		} else {
			// Create new primary assignment
			_, err = tenantDB.Exec(c, `
				INSERT INTO staff_clients (id, tenant_id, staff_id, client_id, is_primary, created_at, updated_at)
				VALUES ($1, $2, $3, $4, true, NOW(), NOW())
			`, uuid.New(), tenantID, assignment.ToStaffID, assignment.ClientID)
		}

		if err != nil {
			errors = append(errors, "Failed to assign client "+assignment.ClientID.String())
			continue
		}

		// Also update services assigned to this client to the new staff
		_, err = tenantDB.Exec(c, `
			UPDATE services
			SET staff_id = $1, updated_at = NOW()
			WHERE client_id = $2 AND tenant_id = $3 AND status IN ('not_started', 'in_progress', 'review', 'waiting')
		`, assignment.ToStaffID, assignment.ClientID, tenantID)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to reassign services - client was assigned")
		}

		applied++
	}

	// Log the rebalance action
	log.Info().
		Str("tenant_id", tenantID.String()).
		Str("user_id", userID.String()).
		Int("applied", applied).
		Int("total", len(req.Assignments)).
		Msg("Applied workload rebalance")

	if len(errors) > 0 {
		c.JSON(http.StatusPartialContent, gin.H{
			"message":  "Partial rebalance applied",
			"applied":  applied,
			"failed":   len(errors),
			"errors":   errors,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Rebalance applied successfully",
		"applied": applied,
	})
}

// calculateWorkloads computes workload metrics for all staff members.
func (h *RebalanceHandler) calculateWorkloads(c *gin.Context, tenantDB *middleware.TenantDB, tenantID uuid.UUID) ([]StaffWorkload, error) {
	query := `
		SELECT
			u.id,
			u.name,
			u.email,
			u.specialism,
			COALESCE(client_counts.cnt, 0) as client_count,
			COALESCE(service_counts.total, 0) as service_count,
			COALESCE(service_counts.pending, 0) as pending_services,
			COALESCE(service_counts.overdue, 0) as overdue_services,
			COALESCE(service_counts.upcoming, 0) as upcoming_deadlines
		FROM users u
		LEFT JOIN (
			SELECT sc.staff_id, COUNT(DISTINCT sc.client_id) as cnt
			FROM staff_clients sc
			JOIN clients c ON sc.client_id = c.id AND c.deleted_at IS NULL
			WHERE sc.tenant_id = $1 AND sc.is_primary = true
			GROUP BY sc.staff_id
		) client_counts ON u.id = client_counts.staff_id
		LEFT JOIN (
			SELECT
				s.staff_id,
				COUNT(*) as total,
				COUNT(*) FILTER (WHERE s.status IN ('not_started', 'in_progress', 'review', 'waiting')) as pending,
				COUNT(*) FILTER (WHERE s.deadline < NOW() AND s.status NOT IN ('completed', 'cancelled')) as overdue,
				COUNT(*) FILTER (WHERE s.deadline BETWEEN NOW() AND NOW() + INTERVAL '7 days' AND s.status NOT IN ('completed', 'cancelled')) as upcoming
			FROM services s
			WHERE s.tenant_id = $1
			GROUP BY s.staff_id
		) service_counts ON u.id = service_counts.staff_id
		WHERE u.tenant_id = $1
		AND u.role = 'staff'
		AND u.status = 'active'
		AND u.deleted_at IS NULL
		ORDER BY u.name
	`

	var workloads []StaffWorkload
	err := tenantDB.Query(c, query, []interface{}{tenantID}, func(rows pgx.Rows) error {
		var w StaffWorkload
		if err := rows.Scan(
			&w.StaffID, &w.StaffName, &w.Email, &w.Specialism,
			&w.ClientCount, &w.ServiceCount, &w.PendingServices,
			&w.OverdueServices, &w.UpcomingDeadlines,
		); err != nil {
			return err
		}

		// Calculate workload score (weighted formula)
		// Overdue services are heavily weighted
		w.WorkloadScore = float64(w.ClientCount)*1.0 +
			float64(w.PendingServices)*2.0 +
			float64(w.OverdueServices)*5.0 +
			float64(w.UpcomingDeadlines)*1.5

		// Determine capacity level
		switch {
		case w.WorkloadScore > 50 || w.OverdueServices > 3:
			w.Capacity = "overloaded"
		case w.WorkloadScore > 30:
			w.Capacity = "high"
		case w.WorkloadScore > 15:
			w.Capacity = "medium"
		default:
			w.Capacity = "low"
		}

		workloads = append(workloads, w)
		return nil
	})

	if err != nil {
		return nil, err
	}

	if workloads == nil {
		workloads = []StaffWorkload{}
	}

	return workloads, nil
}

// clientWorkloadInfo represents a client with workload details.
type clientWorkloadInfo struct {
	ClientID        uuid.UUID
	ClientName      string
	StaffID         uuid.UUID
	StaffName       string
	ServiceCount    int
	PendingServices int
	OverdueServices int
}

// getClientsWithWorkload retrieves clients with their workload metrics.
func (h *RebalanceHandler) getClientsWithWorkload(c *gin.Context, tenantDB *middleware.TenantDB, tenantID uuid.UUID) ([]clientWorkloadInfo, error) {
	query := `
		SELECT
			c.id,
			c.company_name,
			u.id as staff_id,
			u.name as staff_name,
			COALESCE(svc.total, 0) as service_count,
			COALESCE(svc.pending, 0) as pending_services,
			COALESCE(svc.overdue, 0) as overdue_services
		FROM clients c
		INNER JOIN staff_clients sc ON c.id = sc.client_id AND sc.is_primary = true
		INNER JOIN users u ON sc.staff_id = u.id
		LEFT JOIN (
			SELECT
				s.client_id,
				COUNT(*) as total,
				COUNT(*) FILTER (WHERE s.status IN ('not_started', 'in_progress', 'review', 'waiting')) as pending,
				COUNT(*) FILTER (WHERE s.deadline < NOW() AND s.status NOT IN ('completed', 'cancelled')) as overdue
			FROM services s
			WHERE s.tenant_id = $1
			GROUP BY s.client_id
		) svc ON c.id = svc.client_id
		WHERE c.tenant_id = $1 AND c.deleted_at IS NULL
		ORDER BY svc.pending DESC, c.company_name
	`

	var clients []clientWorkloadInfo
	err := tenantDB.Query(c, query, []interface{}{tenantID}, func(rows pgx.Rows) error {
		var client clientWorkloadInfo
		if err := rows.Scan(
			&client.ClientID, &client.ClientName,
			&client.StaffID, &client.StaffName,
			&client.ServiceCount, &client.PendingServices, &client.OverdueServices,
		); err != nil {
			return err
		}
		clients = append(clients, client)
		return nil
	})

	if err != nil {
		return nil, err
	}

	if clients == nil {
		clients = []clientWorkloadInfo{}
	}

	return clients, nil
}

// generateSuggestions creates rebalance suggestions based on workload analysis.
func (h *RebalanceHandler) generateSuggestions(workloads []StaffWorkload, clients []clientWorkloadInfo) []RebalanceSuggestion {
	suggestions := []RebalanceSuggestion{}

	if len(workloads) < 2 || len(clients) == 0 {
		return suggestions
	}

	// Build maps for quick lookup
	workloadMap := make(map[uuid.UUID]*StaffWorkload)
	for i := range workloads {
		workloadMap[workloads[i].StaffID] = &workloads[i]
	}

	// Sort workloads by score (descending - most loaded first)
	sortedByLoad := make([]StaffWorkload, len(workloads))
	copy(sortedByLoad, workloads)
	sort.Slice(sortedByLoad, func(i, j int) bool {
		return sortedByLoad[i].WorkloadScore > sortedByLoad[j].WorkloadScore
	})

	// Find overloaded staff and underutilized staff
	overloaded := []StaffWorkload{}
	underutilized := []StaffWorkload{}
	for _, w := range workloads {
		if w.Capacity == "overloaded" || w.Capacity == "high" {
			overloaded = append(overloaded, w)
		} else if w.Capacity == "low" {
			underutilized = append(underutilized, w)
		}
	}

	if len(overloaded) == 0 || len(underutilized) == 0 {
		return suggestions
	}

	// Calculate average workload for targeting
	avgScore := 0.0
	for _, w := range workloads {
		avgScore += w.WorkloadScore
	}
	avgScore /= float64(len(workloads))

	// For each overloaded staff, suggest moving clients to underutilized staff
	for _, fromStaff := range overloaded {
		// Find clients assigned to this staff
		staffClients := []clientWorkloadInfo{}
		for _, client := range clients {
			if client.StaffID == fromStaff.StaffID {
				staffClients = append(staffClients, client)
			}
		}

		// Sort by pending services (move clients with more pending work first)
		sort.Slice(staffClients, func(i, j int) bool {
			return staffClients[i].PendingServices > staffClients[j].PendingServices
		})

		// Suggest moving clients until workload would be reasonable
		simulatedScore := fromStaff.WorkloadScore
		for _, client := range staffClients {
			if simulatedScore <= avgScore*1.2 { // Don't over-optimize
				break
			}

			// Find best target staff
			var bestTarget *StaffWorkload
			bestTargetScore := math.MaxFloat64
			for i := range underutilized {
				target := &underutilized[i]
				if target.StaffID == fromStaff.StaffID {
					continue
				}
				// Prefer staff with lower workload
				if target.WorkloadScore < bestTargetScore {
					bestTarget = target
					bestTargetScore = target.WorkloadScore
				}
			}

			if bestTarget == nil {
				continue
			}

			// Calculate impact
			clientImpact := float64(client.PendingServices)*2.0 + float64(client.ServiceCount)*0.5
			impactScore := (fromStaff.WorkloadScore - bestTarget.WorkloadScore) / 2.0

			reason := "Balancing workload"
			if fromStaff.OverdueServices > 0 {
				reason = "Reducing overdue backlog"
			} else if fromStaff.UpcomingDeadlines > 5 {
				reason = "Managing upcoming deadline surge"
			}

			suggestions = append(suggestions, RebalanceSuggestion{
				ClientID:        client.ClientID,
				ClientName:      client.ClientName,
				FromStaffID:     fromStaff.StaffID,
				FromStaffName:   fromStaff.StaffName,
				ToStaffID:       bestTarget.StaffID,
				ToStaffName:     bestTarget.StaffName,
				Reason:          reason,
				ImpactScore:     impactScore,
				ServiceCount:    client.ServiceCount,
				PendingServices: client.PendingServices,
			})

			simulatedScore -= clientImpact
			bestTarget.WorkloadScore += clientImpact // Update for next iteration

			// Limit suggestions per overloaded staff
			if len(suggestions) >= 10 {
				break
			}
		}
	}

	// Sort suggestions by impact score (highest impact first)
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].ImpactScore > suggestions[j].ImpactScore
	})

	// Limit total suggestions
	if len(suggestions) > 15 {
		suggestions = suggestions[:15]
	}

	return suggestions
}

// calculateBalanceScore computes how well-balanced the workload distribution is (0-100).
func calculateBalanceScore(workloads []StaffWorkload) float64 {
	if len(workloads) < 2 {
		return 100.0
	}

	// Calculate mean and standard deviation of workload scores
	sum := 0.0
	for _, w := range workloads {
		sum += w.WorkloadScore
	}
	mean := sum / float64(len(workloads))

	if mean == 0 {
		return 100.0 // No workload = perfect balance
	}

	variance := 0.0
	for _, w := range workloads {
		diff := w.WorkloadScore - mean
		variance += diff * diff
	}
	variance /= float64(len(workloads))
	stdDev := math.Sqrt(variance)

	// Coefficient of variation (lower is better)
	cv := stdDev / mean

	// Convert to 0-100 score (100 = perfect balance)
	// CV of 0 = 100, CV of 1+ = 0
	score := math.Max(0, 100-cv*100)
	return math.Round(score*10) / 10
}

// simulateImprovedScore estimates the balance score after applying suggestions.
func simulateImprovedScore(workloads []StaffWorkload, suggestions []RebalanceSuggestion) float64 {
	if len(suggestions) == 0 {
		return calculateBalanceScore(workloads)
	}

	// Create a copy of workloads to simulate changes
	simulated := make([]StaffWorkload, len(workloads))
	copy(simulated, workloads)

	workloadMap := make(map[uuid.UUID]*StaffWorkload)
	for i := range simulated {
		workloadMap[simulated[i].StaffID] = &simulated[i]
	}

	// Apply suggestions
	for _, suggestion := range suggestions {
		fromStaff := workloadMap[suggestion.FromStaffID]
		toStaff := workloadMap[suggestion.ToStaffID]
		if fromStaff == nil || toStaff == nil {
			continue
		}

		// Estimate workload transfer
		transfer := float64(suggestion.PendingServices)*2.0 + 1.0 // Base transfer + pending work

		fromStaff.WorkloadScore = math.Max(0, fromStaff.WorkloadScore-transfer)
		toStaff.WorkloadScore += transfer
	}

	return calculateBalanceScore(simulated)
}
