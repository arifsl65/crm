// Package dataloader provides batched data loading to prevent N+1 queries
package dataloader

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// Loaders holds all DataLoaders for the GraphQL resolvers
type Loaders struct {
	UserLoader     *dataloader.Loader[uuid.UUID, *User]
	ClientLoader   *dataloader.Loader[uuid.UUID, *Client]
	DocumentLoader *dataloader.Loader[uuid.UUID, *Document]
	ServiceLoader  *dataloader.Loader[uuid.UUID, *Service]
}

// User represents a user for DataLoader
type User struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Email       string
	Name        string
	Role        string
	AvatarURL   *string
	IsActive    bool
	LastLoginAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Client represents a client for DataLoader
type Client struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	UserID            *uuid.UUID
	CompanyName       string
	ContactName       string
	Email             string
	Phone             *string
	Address           *string
	YearEnd           *string
	UTR               *string
	CompanyNumber     *string
	CompanyType       *string
	IncorporationDate *string
	VATNumber         *string
	VATQuarter        *string
	Status            string
	RiskScore         *int
	EmailStatus       string
	LastContactAt     *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Document represents a document for DataLoader
type Document struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	ClientID     uuid.UUID
	ServiceID    *uuid.UUID
	TypeID       *uuid.UUID // Maps to type_id in DB (document type)
	Name         string
	OriginalName string
	FilePath     *string // Maps to file_path in DB
	FileSize     *int
	MimeType     *string
	Status       string
	AISummary    *string
	UploadedBy   *uuid.UUID
	ReviewedBy   *uuid.UUID
	ReviewedAt   *time.Time
	ExpiryDate   *string // Maps to expiry_date in DB (DATE stored as string)
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Service represents a service for DataLoader
type Service struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	ClientID     uuid.UUID
	TypeID       *uuid.UUID // Maps to type_id in DB (service type)
	Name         string
	Status       string
	Priority     string
	Deadline     *string // Maps to deadline in DB (DATE stored as string)
	CompletedAt  *time.Time
	DocsRequired int
	DocsReceived int
	StaffID      *uuid.UUID // Maps to staff_id in DB (assigned staff)
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ctxKey is a private type for context keys
type ctxKey string

const loadersKey ctxKey = "dataloaders"

// LoadersKey is exported for use in handler context
var LoadersKey = loadersKey

// NewLoaders creates a new set of DataLoaders using TenantDB (RLS-aware).
// This is the preferred constructor as it enforces Row-Level Security.
func NewLoaders(tenantDB *middleware.TenantDB) *Loaders {
	return &Loaders{
		UserLoader:     newUserLoaderRLS(tenantDB),
		ClientLoader:   newClientLoaderRLS(tenantDB),
		DocumentLoader: newDocumentLoaderRLS(tenantDB),
		ServiceLoader:  newServiceLoaderRLS(tenantDB),
	}
}

// NewLoadersWithPool creates DataLoaders using raw pool with manual tenant filtering.
// This is a fallback when TenantDB is not available (e.g., middleware not applied).
// WARNING: This bypasses RLS and relies on application-level filtering only.
func NewLoadersWithPool(db *database.Pool, tenantID uuid.UUID) *Loaders {
	return &Loaders{
		UserLoader:     newUserLoader(db, tenantID),
		ClientLoader:   newClientLoader(db, tenantID),
		DocumentLoader: newDocumentLoader(db, tenantID),
		ServiceLoader:  newServiceLoader(db, tenantID),
	}
}

// Middleware injects DataLoaders into the request context (legacy, uses raw pool).
// Prefer using NewLoaders(tenantDB) in GinHandler for RLS enforcement.
func Middleware(db *database.Pool) func(ctx context.Context, tenantID uuid.UUID) context.Context {
	return func(ctx context.Context, tenantID uuid.UUID) context.Context {
		loaders := NewLoadersWithPool(db, tenantID)
		return context.WithValue(ctx, loadersKey, loaders)
	}
}

// For retrieves DataLoaders from context.
// Returns nil if no loaders are available (e.g., super_admin without tenant).
// Callers should handle nil gracefully by falling back to direct queries.
func For(ctx context.Context) *Loaders {
	loaders, ok := ctx.Value(loadersKey).(*Loaders)
	if !ok {
		// This is expected for super_admin users without a tenant context
		return nil
	}
	return loaders
}

// =============================================================================
// User Loader
// =============================================================================

func newUserLoader(db *database.Pool, tenantID uuid.UUID) *dataloader.Loader[uuid.UUID, *User] {
	return dataloader.NewBatchedLoader(
		func(ctx context.Context, keys []uuid.UUID) []*dataloader.Result[*User] {
			return userBatchFn(ctx, db, tenantID, keys)
		},
		dataloader.WithWait[uuid.UUID, *User](2*time.Millisecond),
		dataloader.WithBatchCapacity[uuid.UUID, *User](100),
	)
}

func userBatchFn(ctx context.Context, db *database.Pool, tenantID uuid.UUID, keys []uuid.UUID) []*dataloader.Result[*User] {
	results := make([]*dataloader.Result[*User], len(keys))
	userMap := make(map[uuid.UUID]*User)

	// Initialize results with nil
	for i := range results {
		results[i] = &dataloader.Result[*User]{Data: nil}
	}

	// Build query
	query := `
		SELECT id, tenant_id, email, name, role, avatar_url, is_active,
		       last_login_at, created_at, updated_at
		FROM users
		WHERE tenant_id = $1 AND id = ANY($2)
	`

	rows, err := db.Query(ctx, query, tenantID, keys)
	if err != nil {
		log.Error().Err(err).Msg("Failed to batch load users")
		for i := range results {
			results[i] = &dataloader.Result[*User]{Error: err}
		}
		return results
	}
	defer rows.Close()

	for rows.Next() {
		var u User
		if err := rows.Scan(
			&u.ID, &u.TenantID, &u.Email, &u.Name, &u.Role, &u.AvatarURL,
			&u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			log.Error().Err(err).Msg("Failed to scan user")
			continue
		}
		userMap[u.ID] = &u
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating user rows")
	}

	// Map results back to keys in order
	for i, key := range keys {
		if user, ok := userMap[key]; ok {
			results[i] = &dataloader.Result[*User]{Data: user}
		}
	}

	return results
}

// =============================================================================
// Client Loader
// =============================================================================

func newClientLoader(db *database.Pool, tenantID uuid.UUID) *dataloader.Loader[uuid.UUID, *Client] {
	return dataloader.NewBatchedLoader(
		func(ctx context.Context, keys []uuid.UUID) []*dataloader.Result[*Client] {
			return clientBatchFn(ctx, db, tenantID, keys)
		},
		dataloader.WithWait[uuid.UUID, *Client](2*time.Millisecond),
		dataloader.WithBatchCapacity[uuid.UUID, *Client](100),
	)
}

func clientBatchFn(ctx context.Context, db *database.Pool, tenantID uuid.UUID, keys []uuid.UUID) []*dataloader.Result[*Client] {
	results := make([]*dataloader.Result[*Client], len(keys))
	clientMap := make(map[uuid.UUID]*Client)

	for i := range results {
		results[i] = &dataloader.Result[*Client]{Data: nil}
	}

	query := `
		SELECT id, tenant_id, user_id, company_name, contact_name, email,
		       phone, address, year_end, utr, company_number, company_type,
		       incorporation_date, vat_number, vat_quarter, status, risk_score,
		       email_status, last_contact_at, created_at, updated_at
		FROM clients
		WHERE tenant_id = $1 AND id = ANY($2)
	`

	rows, err := db.Query(ctx, query, tenantID, keys)
	if err != nil {
		log.Error().Err(err).Msg("Failed to batch load clients")
		for i := range results {
			results[i] = &dataloader.Result[*Client]{Error: err}
		}
		return results
	}
	defer rows.Close()

	for rows.Next() {
		var c Client
		var yearEnd, incDate *time.Time
		if err := rows.Scan(
			&c.ID, &c.TenantID, &c.UserID, &c.CompanyName, &c.ContactName, &c.Email,
			&c.Phone, &c.Address, &yearEnd, &c.UTR, &c.CompanyNumber, &c.CompanyType,
			&incDate, &c.VATNumber, &c.VATQuarter, &c.Status, &c.RiskScore,
			&c.EmailStatus, &c.LastContactAt, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			log.Error().Err(err).Msg("Failed to scan client")
			continue
		}
		// Format dates as strings
		if yearEnd != nil {
			s := yearEnd.Format("2006-01-02")
			c.YearEnd = &s
		}
		if incDate != nil {
			s := incDate.Format("2006-01-02")
			c.IncorporationDate = &s
		}
		clientMap[c.ID] = &c
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating client rows")
	}

	for i, key := range keys {
		if client, ok := clientMap[key]; ok {
			results[i] = &dataloader.Result[*Client]{Data: client}
		}
	}

	return results
}

// =============================================================================
// Document Loader
// =============================================================================

func newDocumentLoader(db *database.Pool, tenantID uuid.UUID) *dataloader.Loader[uuid.UUID, *Document] {
	return dataloader.NewBatchedLoader(
		func(ctx context.Context, keys []uuid.UUID) []*dataloader.Result[*Document] {
			return documentBatchFn(ctx, db, tenantID, keys)
		},
		dataloader.WithWait[uuid.UUID, *Document](2*time.Millisecond),
		dataloader.WithBatchCapacity[uuid.UUID, *Document](100),
	)
}

func documentBatchFn(ctx context.Context, db *database.Pool, tenantID uuid.UUID, keys []uuid.UUID) []*dataloader.Result[*Document] {
	results := make([]*dataloader.Result[*Document], len(keys))
	docMap := make(map[uuid.UUID]*Document)

	for i := range results {
		results[i] = &dataloader.Result[*Document]{Data: nil}
	}

	query := `
		SELECT id, tenant_id, client_id, service_id, type_id, name,
		       original_name, file_path, file_size, mime_type, status, ai_summary,
		       uploaded_by, reviewed_by, reviewed_at, expiry_date::text,
		       created_at, updated_at
		FROM documents
		WHERE tenant_id = $1 AND id = ANY($2)
	`

	rows, err := db.Query(ctx, query, tenantID, keys)
	if err != nil {
		log.Error().Err(err).Msg("Failed to batch load documents")
		for i := range results {
			results[i] = &dataloader.Result[*Document]{Error: err}
		}
		return results
	}
	defer rows.Close()

	for rows.Next() {
		var d Document
		if err := rows.Scan(
			&d.ID, &d.TenantID, &d.ClientID, &d.ServiceID, &d.TypeID, &d.Name,
			&d.OriginalName, &d.FilePath, &d.FileSize, &d.MimeType, &d.Status, &d.AISummary,
			&d.UploadedBy, &d.ReviewedBy, &d.ReviewedAt, &d.ExpiryDate,
			&d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			log.Error().Err(err).Msg("Failed to scan document")
			continue
		}
		docMap[d.ID] = &d
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating document rows")
	}

	for i, key := range keys {
		if doc, ok := docMap[key]; ok {
			results[i] = &dataloader.Result[*Document]{Data: doc}
		}
	}

	return results
}

// =============================================================================
// Service Loader
// =============================================================================

func newServiceLoader(db *database.Pool, tenantID uuid.UUID) *dataloader.Loader[uuid.UUID, *Service] {
	return dataloader.NewBatchedLoader(
		func(ctx context.Context, keys []uuid.UUID) []*dataloader.Result[*Service] {
			return serviceBatchFn(ctx, db, tenantID, keys)
		},
		dataloader.WithWait[uuid.UUID, *Service](2*time.Millisecond),
		dataloader.WithBatchCapacity[uuid.UUID, *Service](100),
	)
}

func serviceBatchFn(ctx context.Context, db *database.Pool, tenantID uuid.UUID, keys []uuid.UUID) []*dataloader.Result[*Service] {
	results := make([]*dataloader.Result[*Service], len(keys))
	svcMap := make(map[uuid.UUID]*Service)

	for i := range results {
		results[i] = &dataloader.Result[*Service]{Data: nil}
	}

	query := `
		SELECT id, tenant_id, client_id, type_id, name,
		       status, priority, deadline::text, completed_at, docs_required, docs_received,
		       staff_id, created_at, updated_at
		FROM services
		WHERE tenant_id = $1 AND id = ANY($2)
	`

	rows, err := db.Query(ctx, query, tenantID, keys)
	if err != nil {
		log.Error().Err(err).Msg("Failed to batch load services")
		for i := range results {
			results[i] = &dataloader.Result[*Service]{Error: err}
		}
		return results
	}
	defer rows.Close()

	for rows.Next() {
		var s Service
		if err := rows.Scan(
			&s.ID, &s.TenantID, &s.ClientID, &s.TypeID, &s.Name,
			&s.Status, &s.Priority, &s.Deadline, &s.CompletedAt, &s.DocsRequired, &s.DocsReceived,
			&s.StaffID, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			log.Error().Err(err).Msg("Failed to scan service")
			continue
		}
		svcMap[s.ID] = &s
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating service rows")
	}

	for i, key := range keys {
		if svc, ok := svcMap[key]; ok {
			results[i] = &dataloader.Result[*Service]{Data: svc}
		}
	}

	return results
}

// =============================================================================
// RLS-Aware Loaders (using TenantDB with proper RLS context)
// =============================================================================

// newUserLoaderRLS creates a user loader that uses TenantDB for RLS enforcement.
func newUserLoaderRLS(tenantDB *middleware.TenantDB) *dataloader.Loader[uuid.UUID, *User] {
	return dataloader.NewBatchedLoader(
		func(ctx context.Context, keys []uuid.UUID) []*dataloader.Result[*User] {
			return userBatchFnRLS(ctx, tenantDB, keys)
		},
		dataloader.WithWait[uuid.UUID, *User](2*time.Millisecond),
		dataloader.WithBatchCapacity[uuid.UUID, *User](100),
	)
}

func userBatchFnRLS(ctx context.Context, tenantDB *middleware.TenantDB, keys []uuid.UUID) []*dataloader.Result[*User] {
	results := make([]*dataloader.Result[*User], len(keys))
	userMap := make(map[uuid.UUID]*User)

	for i := range results {
		results[i] = &dataloader.Result[*User]{Data: nil}
	}

	// RLS-enforced query - no need for WHERE tenant_id, RLS handles it
	query := `
		SELECT id, tenant_id, email, name, role, avatar_url,
		       CASE WHEN status = 'active' THEN true ELSE false END as is_active,
		       last_login_at, created_at, updated_at
		FROM users
		WHERE id = ANY($1)
	`

	err := tenantDB.TransactionCtx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, keys)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var u User
			if err := rows.Scan(
				&u.ID, &u.TenantID, &u.Email, &u.Name, &u.Role, &u.AvatarURL,
				&u.IsActive, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
			); err != nil {
				log.Error().Err(err).Msg("Failed to scan user")
				continue
			}
			userMap[u.ID] = &u
		}
		return rows.Err()
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to batch load users (RLS)")
		for i := range results {
			results[i] = &dataloader.Result[*User]{Error: err}
		}
		return results
	}

	for i, key := range keys {
		if user, ok := userMap[key]; ok {
			results[i] = &dataloader.Result[*User]{Data: user}
		}
	}

	return results
}

// newClientLoaderRLS creates a client loader that uses TenantDB for RLS enforcement.
func newClientLoaderRLS(tenantDB *middleware.TenantDB) *dataloader.Loader[uuid.UUID, *Client] {
	return dataloader.NewBatchedLoader(
		func(ctx context.Context, keys []uuid.UUID) []*dataloader.Result[*Client] {
			return clientBatchFnRLS(ctx, tenantDB, keys)
		},
		dataloader.WithWait[uuid.UUID, *Client](2*time.Millisecond),
		dataloader.WithBatchCapacity[uuid.UUID, *Client](100),
	)
}

func clientBatchFnRLS(ctx context.Context, tenantDB *middleware.TenantDB, keys []uuid.UUID) []*dataloader.Result[*Client] {
	results := make([]*dataloader.Result[*Client], len(keys))
	clientMap := make(map[uuid.UUID]*Client)

	for i := range results {
		results[i] = &dataloader.Result[*Client]{Data: nil}
	}

	query := `
		SELECT id, tenant_id, user_id, company_name, contact_name, email,
		       phone, address, year_end, utr, company_number, company_type,
		       incorporation_date, vat_number, vat_quarter, status, risk_score,
		       email_status, last_contact_at, created_at, updated_at
		FROM clients
		WHERE id = ANY($1)
	`

	err := tenantDB.TransactionCtx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, keys)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var c Client
			var yearEnd, incDate *time.Time
			if err := rows.Scan(
				&c.ID, &c.TenantID, &c.UserID, &c.CompanyName, &c.ContactName, &c.Email,
				&c.Phone, &c.Address, &yearEnd, &c.UTR, &c.CompanyNumber, &c.CompanyType,
				&incDate, &c.VATNumber, &c.VATQuarter, &c.Status, &c.RiskScore,
				&c.EmailStatus, &c.LastContactAt, &c.CreatedAt, &c.UpdatedAt,
			); err != nil {
				log.Error().Err(err).Msg("Failed to scan client")
				continue
			}
			if yearEnd != nil {
				s := yearEnd.Format("2006-01-02")
				c.YearEnd = &s
			}
			if incDate != nil {
				s := incDate.Format("2006-01-02")
				c.IncorporationDate = &s
			}
			clientMap[c.ID] = &c
		}
		return rows.Err()
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to batch load clients (RLS)")
		for i := range results {
			results[i] = &dataloader.Result[*Client]{Error: err}
		}
		return results
	}

	for i, key := range keys {
		if client, ok := clientMap[key]; ok {
			results[i] = &dataloader.Result[*Client]{Data: client}
		}
	}

	return results
}

// newDocumentLoaderRLS creates a document loader that uses TenantDB for RLS enforcement.
func newDocumentLoaderRLS(tenantDB *middleware.TenantDB) *dataloader.Loader[uuid.UUID, *Document] {
	return dataloader.NewBatchedLoader(
		func(ctx context.Context, keys []uuid.UUID) []*dataloader.Result[*Document] {
			return documentBatchFnRLS(ctx, tenantDB, keys)
		},
		dataloader.WithWait[uuid.UUID, *Document](2*time.Millisecond),
		dataloader.WithBatchCapacity[uuid.UUID, *Document](100),
	)
}

func documentBatchFnRLS(ctx context.Context, tenantDB *middleware.TenantDB, keys []uuid.UUID) []*dataloader.Result[*Document] {
	results := make([]*dataloader.Result[*Document], len(keys))
	docMap := make(map[uuid.UUID]*Document)

	for i := range results {
		results[i] = &dataloader.Result[*Document]{Data: nil}
	}

	query := `
		SELECT id, tenant_id, client_id, service_id, type_id, name,
		       original_name, file_path, file_size, mime_type, status, ai_summary,
		       uploaded_by, reviewed_by, reviewed_at, expiry_date::text,
		       created_at, updated_at
		FROM documents
		WHERE id = ANY($1)
	`

	err := tenantDB.TransactionCtx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, keys)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var d Document
			if err := rows.Scan(
				&d.ID, &d.TenantID, &d.ClientID, &d.ServiceID, &d.TypeID, &d.Name,
				&d.OriginalName, &d.FilePath, &d.FileSize, &d.MimeType, &d.Status, &d.AISummary,
				&d.UploadedBy, &d.ReviewedBy, &d.ReviewedAt, &d.ExpiryDate,
				&d.CreatedAt, &d.UpdatedAt,
			); err != nil {
				log.Error().Err(err).Msg("Failed to scan document")
				continue
			}
			docMap[d.ID] = &d
		}
		return rows.Err()
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to batch load documents (RLS)")
		for i := range results {
			results[i] = &dataloader.Result[*Document]{Error: err}
		}
		return results
	}

	for i, key := range keys {
		if doc, ok := docMap[key]; ok {
			results[i] = &dataloader.Result[*Document]{Data: doc}
		}
	}

	return results
}

// newServiceLoaderRLS creates a service loader that uses TenantDB for RLS enforcement.
func newServiceLoaderRLS(tenantDB *middleware.TenantDB) *dataloader.Loader[uuid.UUID, *Service] {
	return dataloader.NewBatchedLoader(
		func(ctx context.Context, keys []uuid.UUID) []*dataloader.Result[*Service] {
			return serviceBatchFnRLS(ctx, tenantDB, keys)
		},
		dataloader.WithWait[uuid.UUID, *Service](2*time.Millisecond),
		dataloader.WithBatchCapacity[uuid.UUID, *Service](100),
	)
}

func serviceBatchFnRLS(ctx context.Context, tenantDB *middleware.TenantDB, keys []uuid.UUID) []*dataloader.Result[*Service] {
	results := make([]*dataloader.Result[*Service], len(keys))
	svcMap := make(map[uuid.UUID]*Service)

	for i := range results {
		results[i] = &dataloader.Result[*Service]{Data: nil}
	}

	query := `
		SELECT id, tenant_id, client_id, type_id, name,
		       status, priority, deadline::text, completed_at, docs_required, docs_received,
		       staff_id, created_at, updated_at
		FROM services
		WHERE id = ANY($1)
	`

	err := tenantDB.TransactionCtx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, keys)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var s Service
			if err := rows.Scan(
				&s.ID, &s.TenantID, &s.ClientID, &s.TypeID, &s.Name,
				&s.Status, &s.Priority, &s.Deadline, &s.CompletedAt, &s.DocsRequired, &s.DocsReceived,
				&s.StaffID, &s.CreatedAt, &s.UpdatedAt,
			); err != nil {
				log.Error().Err(err).Msg("Failed to scan service")
				continue
			}
			svcMap[s.ID] = &s
		}
		return rows.Err()
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to batch load services (RLS)")
		for i := range results {
			results[i] = &dataloader.Result[*Service]{Error: err}
		}
		return results
	}

	for i, key := range keys {
		if svc, ok := svcMap[key]; ok {
			results[i] = &dataloader.Result[*Service]{Data: svc}
		}
	}

	return results
}
