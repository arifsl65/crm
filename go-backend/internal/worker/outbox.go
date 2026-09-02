// Package worker provides background workers for async tasks.
package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/email"
)

// OutboxWorker processes outbox entries and sends emails.
type OutboxWorker struct {
	db          *database.Pool
	emailClient *email.Client
	interval    time.Duration
	batchSize   int
	stopCh      chan struct{}
}

// OutboxEntry represents an entry in the outbox table.
type OutboxEntry struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	EventType  string
	Payload    json.RawMessage
	CreatedAt  time.Time
	Attempts   int
}

// EmailSendPayload represents the payload for email_send events.
type EmailSendPayload struct {
	EmailID   uuid.UUID  `json:"email_id"`
	ToEmail   string     `json:"to_email"`
	ToName    *string    `json:"to_name,omitempty"`
	FromEmail string     `json:"from_email"`
	Subject   string     `json:"subject"`
	BodyHTML  string     `json:"body_html"`
	BodyText  string     `json:"body_text"`
	ClientID  *uuid.UUID `json:"client_id,omitempty"`
}

// ChaseEmailSendPayload represents the payload for chase_email_send events.
type ChaseEmailSendPayload struct {
	ChaseLogID uuid.UUID `json:"chase_log_id"`
	EmailID    uuid.UUID `json:"email_id"`
	ClientID   uuid.UUID `json:"client_id"`
	ToEmail    string    `json:"to_email"`
	FromEmail  string    `json:"from_email"`
	Subject    string    `json:"subject"`
	BodyHTML   string    `json:"body_html"`
	BodyText   string    `json:"body_text"`
}

// NewOutboxWorker creates a new outbox worker.
func NewOutboxWorker(db *database.Pool, emailClient *email.Client) *OutboxWorker {
	return &OutboxWorker{
		db:          db,
		emailClient: emailClient,
		interval:    5 * time.Second,  // Poll every 5 seconds
		batchSize:   10,               // Process 10 entries per batch
		stopCh:      make(chan struct{}),
	}
}

// Start begins processing the outbox in a background goroutine.
func (w *OutboxWorker) Start() {
	go w.run()
	log.Info().Dur("interval", w.interval).Int("batch_size", w.batchSize).Msg("Outbox worker started")
}

// Stop gracefully stops the worker.
func (w *OutboxWorker) Stop() {
	close(w.stopCh)
	log.Info().Msg("Outbox worker stopped")
}

func (w *OutboxWorker) run() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.processBatch()
		}
	}
}

func (w *OutboxWorker) processBatch() {
	// Create a context that will be cancelled when stopCh is closed
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listen for stop signal and cancel context
	go func() {
		select {
		case <-w.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	// Check if email client is configured
	if w.emailClient == nil || !w.emailClient.IsConfigured() {
		return
	}

	// Fetch pending outbox entries
	entries, err := w.fetchPendingEntries(ctx)
	if err != nil {
		if ctx.Err() != nil {
			log.Debug().Msg("Outbox worker stopping - context cancelled during fetch")
			return
		}
		log.Error().Err(err).Msg("Failed to fetch outbox entries")
		return
	}

	if len(entries) == 0 {
		return
	}

	log.Debug().Int("count", len(entries)).Msg("Processing outbox entries")

	for _, entry := range entries {
		// Check for graceful shutdown before processing each entry
		select {
		case <-ctx.Done():
			log.Info().Int("remaining", len(entries)).Msg("Outbox worker stopping - graceful shutdown during batch")
			return
		default:
		}

		err := w.processEntry(ctx, entry)
		if err != nil {
			if ctx.Err() != nil {
				log.Debug().Msg("Outbox worker stopping - context cancelled during entry processing")
				return
			}
			log.Error().Err(err).Str("id", entry.ID.String()).Msg("Failed to process outbox entry")
			w.incrementAttempts(ctx, entry.ID)
		} else {
			w.markPublished(ctx, entry.ID)
		}
	}
}

func (w *OutboxWorker) fetchPendingEntries(ctx context.Context) ([]OutboxEntry, error) {
	query := `
		SELECT id, tenant_id, event_type, payload, created_at, attempts
		FROM outbox
		WHERE published_at IS NULL AND attempts < 5
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	var entries []OutboxEntry

	// Use SuperAdminTransaction to bypass RLS - worker needs cross-tenant access
	err := w.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, w.batchSize)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var e OutboxEntry
			if err := rows.Scan(&e.ID, &e.TenantID, &e.EventType, &e.Payload, &e.CreatedAt, &e.Attempts); err != nil {
				return err
			}
			entries = append(entries, e)
		}
		return rows.Err()
	})

	return entries, err
}

func (w *OutboxWorker) processEntry(ctx context.Context, entry OutboxEntry) error {
	switch entry.EventType {
	case "email_send":
		return w.processEmailSend(ctx, entry)
	case "chase_email_send":
		return w.processChaseEmailSend(ctx, entry)
	default:
		log.Warn().Str("event_type", entry.EventType).Msg("Unknown outbox event type")
		return nil
	}
}

func (w *OutboxWorker) processEmailSend(ctx context.Context, entry OutboxEntry) error {
	var payload EmailSendPayload
	if err := json.Unmarshal(entry.Payload, &payload); err != nil {
		return err
	}

	// Send via Resend
	resendID, err := w.emailClient.SendWithID(payload.ToEmail, payload.Subject, payload.BodyHTML, payload.BodyText)
	if err != nil {
		return err
	}

	// Update email record with sent status and resend_id
	// Use TenantTransaction with the entry's tenant_id for RLS
	err = w.db.TenantTransaction(ctx, entry.TenantID.String(), "staff", func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE emails
			SET status = 'sent', resend_id = $1, sent_at = NOW()
			WHERE id = $2 AND tenant_id = $3
		`, resendID, payload.EmailID, entry.TenantID)
		return err
	})

	if err != nil {
		log.Error().Err(err).Str("email_id", payload.EmailID.String()).Msg("Failed to update email status after sending")
		// Don't return error - email was sent successfully
	}

	log.Info().
		Str("email_id", payload.EmailID.String()).
		Str("to", payload.ToEmail).
		Str("resend_id", resendID).
		Msg("Email sent successfully")

	return nil
}

func (w *OutboxWorker) processChaseEmailSend(ctx context.Context, entry OutboxEntry) error {
	var payload ChaseEmailSendPayload
	if err := json.Unmarshal(entry.Payload, &payload); err != nil {
		return err
	}

	// Send via Resend
	resendID, err := w.emailClient.SendWithID(payload.ToEmail, payload.Subject, payload.BodyHTML, payload.BodyText)
	if err != nil {
		return err
	}

	// Update email record with sent status and resend_id
	// Use TenantTransaction with the entry's tenant_id for RLS
	err = w.db.TenantTransaction(ctx, entry.TenantID.String(), "staff", func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE emails
			SET status = 'sent', resend_id = $1, sent_at = NOW()
			WHERE id = $2 AND tenant_id = $3
		`, resendID, payload.EmailID, entry.TenantID)
		return err
	})

	if err != nil {
		log.Error().Err(err).Str("email_id", payload.EmailID.String()).Msg("Failed to update chase email status")
	}

	log.Info().
		Str("email_id", payload.EmailID.String()).
		Str("chase_log_id", payload.ChaseLogID.String()).
		Str("to", payload.ToEmail).
		Str("resend_id", resendID).
		Msg("Chase email sent successfully")

	return nil
}

func (w *OutboxWorker) markPublished(ctx context.Context, id uuid.UUID) {
	err := w.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE outbox SET published_at = NOW() WHERE id = $1`, id)
		return err
	})
	if err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to mark outbox entry as published")
	}
}

func (w *OutboxWorker) incrementAttempts(ctx context.Context, id uuid.UUID) {
	err := w.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE outbox SET attempts = attempts + 1 WHERE id = $1`, id)
		return err
	})
	if err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to increment outbox attempts")
	}
}
