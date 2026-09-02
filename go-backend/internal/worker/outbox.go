// Package worker provides background workers for async tasks.
package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
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
	ctx := context.Background()

	// Check if email client is configured
	if w.emailClient == nil || !w.emailClient.IsConfigured() {
		return
	}

	// Fetch pending outbox entries
	entries, err := w.fetchPendingEntries(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch outbox entries")
		return
	}

	if len(entries) == 0 {
		return
	}

	log.Debug().Int("count", len(entries)).Msg("Processing outbox entries")

	for _, entry := range entries {
		err := w.processEntry(ctx, entry)
		if err != nil {
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

	rows, err := w.db.Query(ctx, query, w.batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []OutboxEntry
	for rows.Next() {
		var e OutboxEntry
		err := rows.Scan(&e.ID, &e.TenantID, &e.EventType, &e.Payload, &e.CreatedAt, &e.Attempts)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
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
	_, err = w.db.Exec(ctx, `
		UPDATE emails
		SET status = 'sent', resend_id = $1, sent_at = NOW()
		WHERE id = $2 AND tenant_id = $3
	`, resendID, payload.EmailID, entry.TenantID)

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
	_, err = w.db.Exec(ctx, `
		UPDATE emails
		SET status = 'sent', resend_id = $1, sent_at = NOW()
		WHERE id = $2 AND tenant_id = $3
	`, resendID, payload.EmailID, entry.TenantID)

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
	_, err := w.db.Exec(ctx, `UPDATE outbox SET published_at = NOW() WHERE id = $1`, id)
	if err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to mark outbox entry as published")
	}
}

func (w *OutboxWorker) incrementAttempts(ctx context.Context, id uuid.UUID) {
	_, err := w.db.Exec(ctx, `UPDATE outbox SET attempts = attempts + 1 WHERE id = $1`, id)
	if err != nil {
		log.Error().Err(err).Str("id", id.String()).Msg("Failed to increment outbox attempts")
	}
}
