// Package cache provides Redis Pub/Sub functionality for real-time events.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// EventType represents the type of real-time event.
type EventType string

const (
	// Document events
	EventDocUploaded  EventType = "doc_uploaded"
	EventDocApproved  EventType = "doc_approved"
	EventDocRejected  EventType = "doc_rejected"
	EventDocExpiring  EventType = "doc_expiring"
	EventDocRenewal   EventType = "doc_renewal_requested"

	// Service events
	EventServiceCreated   EventType = "service_created"
	EventServiceCompleted EventType = "service_completed"
	EventServiceAtRisk    EventType = "service_at_risk"
	EventDeadlineApproach EventType = "deadline_approaching"

	// Client events
	EventClientCreated EventType = "client_created"
	EventClientUpdated EventType = "client_updated"

	// Notification events
	EventNotification EventType = "notification"
)

// Event represents a real-time event to be published.
type Event struct {
	ID         string                 `json:"id"`
	Type       EventType              `json:"type"`
	TenantID   uuid.UUID              `json:"tenant_id"`
	UserID     *uuid.UUID             `json:"user_id,omitempty"`
	EntityType string                 `json:"entity_type"`
	EntityID   *uuid.UUID             `json:"entity_id,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// NewEvent creates a new event with auto-generated ID and timestamp.
func NewEvent(eventType EventType, tenantID uuid.UUID, entityType string, entityID *uuid.UUID) *Event {
	return &Event{
		ID:         uuid.New().String(),
		Type:       eventType,
		TenantID:   tenantID,
		EntityType: entityType,
		EntityID:   entityID,
		Data:       make(map[string]interface{}),
		CreatedAt:  time.Now(),
	}
}

// WithUser sets the user ID on the event.
func (e *Event) WithUser(userID uuid.UUID) *Event {
	e.UserID = &userID
	return e
}

// WithData adds data to the event.
func (e *Event) WithData(key string, value interface{}) *Event {
	if e.Data == nil {
		e.Data = make(map[string]interface{})
	}
	e.Data[key] = value
	return e
}

// TenantChannel returns the Redis channel for a tenant's events.
func TenantChannel(tenantID uuid.UUID) string {
	return fmt.Sprintf("tenant:%s:events", tenantID.String())
}

// UserChannel returns the Redis channel for a user's personal events.
func UserChannel(tenantID, userID uuid.UUID) string {
	return fmt.Sprintf("tenant:%s:user:%s:events", tenantID.String(), userID.String())
}

// Publish publishes an event to the tenant's channel.
func (c *Client) Publish(ctx context.Context, event *Event) error {
	channel := TenantChannel(event.TenantID)

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := c.Client.Publish(ctx, channel, payload).Err(); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	log.Debug().
		Str("channel", channel).
		Str("event_type", string(event.Type)).
		Str("entity_type", event.EntityType).
		Msg("Event published")

	return nil
}

// PublishToUser publishes an event to a specific user's channel.
func (c *Client) PublishToUser(ctx context.Context, event *Event, userID uuid.UUID) error {
	channel := UserChannel(event.TenantID, userID)

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := c.Client.Publish(ctx, channel, payload).Err(); err != nil {
		return fmt.Errorf("failed to publish event to user: %w", err)
	}

	log.Debug().
		Str("channel", channel).
		Str("event_type", string(event.Type)).
		Str("user_id", userID.String()).
		Msg("Event published to user")

	return nil
}

// Subscribe subscribes to a tenant's events channel.
// Returns a channel that receives events and a cleanup function.
func (c *Client) Subscribe(ctx context.Context, tenantID uuid.UUID) (<-chan *Event, func(), error) {
	channel := TenantChannel(tenantID)
	pubsub := c.Client.Subscribe(ctx, channel)

	// Verify subscription
	_, err := pubsub.Receive(ctx)
	if err != nil {
		pubsub.Close()
		return nil, nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	eventChan := make(chan *Event, 100)

	// Start goroutine to process messages
	go func() {
		defer close(eventChan)
		ch := pubsub.Channel()

		for msg := range ch {
			var event Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Error().Err(err).Str("payload", msg.Payload).Msg("Failed to unmarshal event")
				continue
			}

			select {
			case eventChan <- &event:
			case <-ctx.Done():
				return
			default:
				// Channel full, drop event
				log.Warn().Str("event_type", string(event.Type)).Msg("Event channel full, dropping event")
			}
		}
	}()

	cleanup := func() {
		pubsub.Close()
	}

	log.Debug().Str("channel", channel).Msg("Subscribed to events")

	return eventChan, cleanup, nil
}

// SubscribeMultiple subscribes to multiple channels (tenant + user).
func (c *Client) SubscribeMultiple(ctx context.Context, tenantID, userID uuid.UUID) (<-chan *Event, func(), error) {
	channels := []string{
		TenantChannel(tenantID),
		UserChannel(tenantID, userID),
	}

	pubsub := c.Client.Subscribe(ctx, channels...)

	// Verify subscription
	_, err := pubsub.Receive(ctx)
	if err != nil {
		pubsub.Close()
		return nil, nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	eventChan := make(chan *Event, 100)

	// Start goroutine to process messages
	go func() {
		defer close(eventChan)
		ch := pubsub.Channel()

		for msg := range ch {
			var event Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Error().Err(err).Str("payload", msg.Payload).Msg("Failed to unmarshal event")
				continue
			}

			select {
			case eventChan <- &event:
			case <-ctx.Done():
				return
			default:
				log.Warn().Str("event_type", string(event.Type)).Msg("Event channel full, dropping event")
			}
		}
	}()

	cleanup := func() {
		pubsub.Close()
	}

	log.Debug().Strs("channels", channels).Msg("Subscribed to multiple channels")

	return eventChan, cleanup, nil
}

// PubSubStats returns Pub/Sub statistics.
func (c *Client) PubSubStats(ctx context.Context) (map[string]interface{}, error) {
	// Get number of subscribers per channel pattern
	channels, err := c.Client.PubSubChannels(ctx, "tenant:*:events").Result()
	if err != nil {
		return nil, err
	}

	numsub, err := c.Client.PubSubNumSub(ctx, channels...).Result()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"active_channels":  len(channels),
		"channel_subscribers": numsub,
	}, nil
}

// Ensure Client implements PubSubber interface.
var _ PubSubber = (*Client)(nil)

// PubSubber defines the Pub/Sub interface.
type PubSubber interface {
	Publish(ctx context.Context, event *Event) error
	PublishToUser(ctx context.Context, event *Event, userID uuid.UUID) error
	Subscribe(ctx context.Context, tenantID uuid.UUID) (<-chan *Event, func(), error)
	SubscribeMultiple(ctx context.Context, tenantID, userID uuid.UUID) (<-chan *Event, func(), error)
}
