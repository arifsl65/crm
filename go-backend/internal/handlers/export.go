package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// ExportHandler handles CSV export operations.
type ExportHandler struct {
	db *database.Pool
}

// NewExportHandler creates a new export handler.
func NewExportHandler(db *database.Pool) *ExportHandler {
	return &ExportHandler{db: db}
}

// ExportClients exports all clients as CSV.
// GET /api/v1/export/clients
func (h *ExportHandler) ExportClients(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Optional filter by status
	status := c.Query("status")

	query := `
		SELECT
			c.id, c.company_name, c.contact_name, c.email, c.phone,
			c.address, c.year_end, c.utr, c.company_number, c.company_type,
			c.vat_number, c.vat_quarter, c.status, c.risk_score,
			c.last_contact_at, c.created_at,
			COALESCE(u.name, '') as assigned_staff
		FROM clients c
		LEFT JOIN staff_clients sc ON c.id = sc.client_id AND sc.is_primary = true
		LEFT JOIN users u ON sc.staff_id = u.id
		WHERE c.tenant_id = $1 AND c.deleted_at IS NULL
	`
	args := []interface{}{tenantID}

	if status != "" {
		query += ` AND c.status = $2`
		args = append(args, status)
	}

	query += ` ORDER BY c.company_name`

	// Set CSV headers
	filename := fmt.Sprintf("clients_export_%s.csv", time.Now().Format("2006-01-02"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// Write header row
	headers := []string{
		"ID", "Company Name", "Contact Name", "Email", "Phone",
		"Address", "Year End", "UTR", "Company Number", "Company Type",
		"VAT Number", "VAT Quarter", "Status", "Risk Score",
		"Last Contact", "Created At", "Assigned Staff",
	}
	if err := writer.Write(headers); err != nil {
		log.Error().Err(err).Msg("Failed to write CSV header")
		return
	}

	// Stream rows directly to CSV
	err := tenantDB.Query(c, query, args, func(rows pgx.Rows) error {
		var (
			id            uuid.UUID
			companyName   string
			contactName   string
			email         string
			phone         *string
			address       *string
			yearEnd       *time.Time
			utr           *string
			companyNumber *string
			companyType   *string
			vatNumber     *string
			vatQuarter    *string
			status        string
			riskScore     *int
			lastContact   *time.Time
			createdAt     time.Time
			assignedStaff string
		)

		err := rows.Scan(
			&id, &companyName, &contactName, &email, &phone,
			&address, &yearEnd, &utr, &companyNumber, &companyType,
			&vatNumber, &vatQuarter, &status, &riskScore,
			&lastContact, &createdAt, &assignedStaff,
		)
		if err != nil {
			return err
		}

		row := []string{
			id.String(),
			companyName,
			contactName,
			email,
			ptrToString(phone),
			ptrToString(address),
			timeToString(yearEnd, "2006-01-02"),
			ptrToString(utr),
			ptrToString(companyNumber),
			ptrToString(companyType),
			ptrToString(vatNumber),
			ptrToString(vatQuarter),
			status,
			intPtrToString(riskScore),
			timeToString(lastContact, "2006-01-02 15:04"),
			createdAt.Format("2006-01-02 15:04"),
			assignedStaff,
		}

		return writer.Write(row)
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to export clients")
		// Can't return JSON error since we've already started writing CSV
		return
	}
}

// ExportServices exports all services as CSV.
// GET /api/v1/export/services
func (h *ExportHandler) ExportServices(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	// Optional filters
	status := c.Query("status")
	clientID := c.Query("client_id")

	query := `
		SELECT
			s.id, s.name, s.period, s.status, s.priority, s.risk_level,
			s.deadline, s.docs_required, s.docs_received,
			s.hmrc_reference, s.filed_at, s.completed_at,
			s.created_at, s.updated_at,
			COALESCE(c.company_name, '') as client_name,
			COALESCE(u.name, '') as staff_name,
			COALESCE(st.name, '') as service_type
		FROM services s
		LEFT JOIN clients c ON s.client_id = c.id
		LEFT JOIN users u ON s.staff_id = u.id
		LEFT JOIN service_types st ON s.type_id = st.id
		WHERE s.tenant_id = $1
	`
	args := []interface{}{tenantID}
	argNum := 2

	if status != "" {
		query += fmt.Sprintf(` AND s.status = $%d`, argNum)
		args = append(args, status)
		argNum++
	}

	if clientID != "" {
		if cid, err := uuid.Parse(clientID); err == nil {
			query += fmt.Sprintf(` AND s.client_id = $%d`, argNum)
			args = append(args, cid)
			argNum++
		}
	}

	query += ` ORDER BY s.deadline NULLS LAST, s.created_at DESC`

	// Set CSV headers
	filename := fmt.Sprintf("services_export_%s.csv", time.Now().Format("2006-01-02"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// Write header row
	headers := []string{
		"ID", "Service Name", "Period", "Status", "Priority", "Risk Level",
		"Deadline", "Docs Required", "Docs Received",
		"HMRC Reference", "Filed At", "Completed At",
		"Created At", "Updated At",
		"Client Name", "Staff Name", "Service Type",
	}
	if err := writer.Write(headers); err != nil {
		log.Error().Err(err).Msg("Failed to write CSV header")
		return
	}

	// Stream rows directly to CSV
	err := tenantDB.Query(c, query, args, func(rows pgx.Rows) error {
		var (
			id            uuid.UUID
			name          string
			period        *string
			status        string
			priority      string
			riskLevel     string
			deadline      *time.Time
			docsRequired  int
			docsReceived  int
			hmrcReference *string
			filedAt       *time.Time
			completedAt   *time.Time
			createdAt     time.Time
			updatedAt     time.Time
			clientName    string
			staffName     string
			serviceType   string
		)

		err := rows.Scan(
			&id, &name, &period, &status, &priority, &riskLevel,
			&deadline, &docsRequired, &docsReceived,
			&hmrcReference, &filedAt, &completedAt,
			&createdAt, &updatedAt,
			&clientName, &staffName, &serviceType,
		)
		if err != nil {
			return err
		}

		row := []string{
			id.String(),
			name,
			ptrToString(period),
			status,
			priority,
			riskLevel,
			timeToString(deadline, "2006-01-02"),
			fmt.Sprintf("%d", docsRequired),
			fmt.Sprintf("%d", docsReceived),
			ptrToString(hmrcReference),
			timeToString(filedAt, "2006-01-02 15:04"),
			timeToString(completedAt, "2006-01-02 15:04"),
			createdAt.Format("2006-01-02 15:04"),
			updatedAt.Format("2006-01-02 15:04"),
			clientName,
			staffName,
			serviceType,
		}

		return writer.Write(row)
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to export services")
		return
	}
}

// ExportChase exports chase logs as CSV.
// GET /api/v1/export/chase
func (h *ExportHandler) ExportChase(c *gin.Context) {
	tenantID, _ := middleware.GetTenantID(c)

	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		log.Error().Msg("TenantDB not found in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
		return
	}

	query := `
		SELECT
			cl.id, cl.total_sent, cl.delivered, cl.opened, cl.bounced,
			cl.created_at,
			COALESCE(u.name, '') as initiated_by_name,
			(
				SELECT STRING_AGG(c.company_name, ', ')
				FROM chase_log_clients clc
				JOIN clients c ON clc.client_id = c.id
				WHERE clc.chase_log_id = cl.id
			) as clients_chased
		FROM chase_logs cl
		LEFT JOIN users u ON cl.initiated_by = u.id
		WHERE cl.tenant_id = $1
		ORDER BY cl.created_at DESC
	`

	// Set CSV headers
	filename := fmt.Sprintf("chase_logs_export_%s.csv", time.Now().Format("2006-01-02"))
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// Write header row
	headers := []string{
		"ID", "Total Sent", "Delivered", "Opened", "Bounced",
		"Created At", "Initiated By", "Clients Chased",
	}
	if err := writer.Write(headers); err != nil {
		log.Error().Err(err).Msg("Failed to write CSV header")
		return
	}

	// Stream rows directly to CSV
	err := tenantDB.Query(c, query, []interface{}{tenantID}, func(rows pgx.Rows) error {
		var (
			id              uuid.UUID
			totalSent       int
			delivered       int
			opened          int
			bounced         int
			createdAt       time.Time
			initiatedByName string
			clientsChased   *string
		)

		err := rows.Scan(
			&id, &totalSent, &delivered, &opened, &bounced,
			&createdAt, &initiatedByName, &clientsChased,
		)
		if err != nil {
			return err
		}

		row := []string{
			id.String(),
			fmt.Sprintf("%d", totalSent),
			fmt.Sprintf("%d", delivered),
			fmt.Sprintf("%d", opened),
			fmt.Sprintf("%d", bounced),
			createdAt.Format("2006-01-02 15:04"),
			initiatedByName,
			ptrToString(clientsChased),
		}

		return writer.Write(row)
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to export chase logs")
		return
	}
}

// Helper functions for CSV formatting

func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func timeToString(t *time.Time, format string) string {
	if t == nil {
		return ""
	}
	return t.Format(format)
}

func intPtrToString(i *int) string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%d", *i)
}
