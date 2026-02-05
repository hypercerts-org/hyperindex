package jetstream

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/GainForest/hypergoat/internal/database/repositories"
	"github.com/GainForest/hypergoat/internal/graphql/subscription"
)

// ConsumerConfig configures the Jetstream consumer.
type ConsumerConfig struct {
	// JetstreamURL is the Jetstream WebSocket endpoint.
	JetstreamURL string

	// Collections to subscribe to.
	Collections []string

	// DisableCursor disables cursor tracking.
	DisableCursor bool

	// CursorFlushInterval is how often to flush the cursor to database.
	CursorFlushInterval time.Duration
}

// Consumer consumes events from Jetstream and stores them in the database.
type Consumer struct {
	config      ConsumerConfig
	client      *Client
	recordsRepo *repositories.RecordsRepository
	actorsRepo  *repositories.ActorsRepository
	configRepo  *repositories.ConfigRepository

	// Pub/sub for GraphQL subscriptions
	pubsub *subscription.PubSub

	// Cursor tracking
	cursor     int64
	cursorMu   sync.Mutex
	cursorDone chan struct{}

	// Stats
	stats      Stats
	statsMu    sync.RWMutex
	statsStart time.Time
}

// Stats tracks consumer statistics.
type Stats struct {
	EventsReceived int64
	RecordsCreated int64
	RecordsUpdated int64
	RecordsDeleted int64
	Errors         int64
}

// NewConsumer creates a new Jetstream consumer.
func NewConsumer(
	config ConsumerConfig,
	recordsRepo *repositories.RecordsRepository,
	actorsRepo *repositories.ActorsRepository,
	configRepo *repositories.ConfigRepository,
) *Consumer {
	if config.CursorFlushInterval == 0 {
		config.CursorFlushInterval = 5 * time.Second
	}

	return &Consumer{
		config:      config,
		recordsRepo: recordsRepo,
		actorsRepo:  actorsRepo,
		configRepo:  configRepo,
		pubsub:      subscription.Global(), // Use global pub/sub for GraphQL subscriptions
		cursorDone:  make(chan struct{}),
		statsStart:  time.Now(),
	}
}

// Start begins consuming events from Jetstream.
func (c *Consumer) Start(ctx context.Context) error {
	// Load cursor from database
	cursor, err := c.loadCursor(ctx)
	if err != nil {
		slog.Warn("Failed to load cursor, starting from live", "error", err)
	} else if cursor > 0 {
		slog.Info("Resuming from cursor", "cursor", cursor)
	}

	// Create client
	c.client = NewClient(ClientConfig{
		URL:           c.config.JetstreamURL,
		Collections:   c.config.Collections,
		Cursor:        cursor,
		DisableCursor: c.config.DisableCursor,
	})

	// Connect
	if err := c.client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Start cursor flusher
	if !c.config.DisableCursor {
		go c.cursorFlusher(ctx)
	}

	// Start event processor
	go c.processEvents(ctx)

	// Run client (blocking)
	return c.client.Run(ctx)
}

// Stop stops the consumer.
func (c *Consumer) Stop() {
	close(c.cursorDone)
	if c.client != nil {
		c.client.Stop()
	}
}

// Stats returns the current statistics.
func (c *Consumer) Stats() Stats {
	c.statsMu.RLock()
	defer c.statsMu.RUnlock()
	return c.stats
}

// processEvents handles incoming events.
func (c *Consumer) processEvents(ctx context.Context) {
	for event := range c.client.Events() {
		c.statsMu.Lock()
		c.stats.EventsReceived++
		c.statsMu.Unlock()

		// Update cursor
		c.cursorMu.Lock()
		c.cursor = event.TimeUS
		c.cursorMu.Unlock()

		// Process commit events
		if event.IsCommit() {
			if err := c.handleCommit(ctx, event); err != nil {
				slog.Warn("Failed to handle commit",
					"error", err,
					"did", event.DID,
					"collection", event.Commit.Collection,
				)
				c.statsMu.Lock()
				c.stats.Errors++
				c.statsMu.Unlock()
			}
		}
	}
}

// handleCommit processes a commit event.
func (c *Consumer) handleCommit(ctx context.Context, event *Event) error {
	commit := event.Commit
	uri := commit.URI(event.DID)

	switch commit.Operation {
	case OpCreate, OpUpdate:
		// Ensure actor exists (just store the DID, no resolution)
		if err := c.ensureActor(ctx, event.DID); err != nil {
			slog.Warn("Failed to ensure actor", "did", event.DID, "error", err)
			// Continue anyway - record storage is more important
		}

		// Store the record
		result, err := c.recordsRepo.Insert(ctx, uri, commit.CID, event.DID, commit.Collection, string(commit.Record))
		if err != nil {
			return fmt.Errorf("failed to insert record: %w", err)
		}

		c.statsMu.Lock()
		if result == repositories.Inserted {
			if commit.Operation == OpCreate {
				c.stats.RecordsCreated++
			} else {
				c.stats.RecordsUpdated++
			}
		}
		c.statsMu.Unlock()

		// Publish to GraphQL subscriptions
		eventType := subscription.EventCreate
		if commit.Operation == OpUpdate {
			eventType = subscription.EventUpdate
		}
		c.pubsub.PublishRecord(eventType, uri, commit.CID, event.DID, commit.Collection, commit.Record)

		slog.Debug("Stored record",
			"uri", uri,
			"cid", commit.CID,
			"operation", commit.Operation,
		)

	case OpDelete:
		if err := c.recordsRepo.Delete(ctx, uri); err != nil {
			return fmt.Errorf("failed to delete record: %w", err)
		}

		c.statsMu.Lock()
		c.stats.RecordsDeleted++
		c.statsMu.Unlock()

		// Publish delete to GraphQL subscriptions
		c.pubsub.PublishRecord(subscription.EventDelete, uri, commit.CID, event.DID, commit.Collection, nil)

		slog.Debug("Deleted record", "uri", uri)
	}

	return nil
}

// ensureActor ensures the actor exists in the database.
func (c *Consumer) ensureActor(ctx context.Context, did string) error {
	// Check if actor exists
	exists, err := c.actorsRepo.Exists(ctx, did)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	// Upsert actor (without handle resolution per user request)
	return c.actorsRepo.Upsert(ctx, did, "") // Empty handle
}

// cursorFlusher periodically flushes the cursor to the database.
func (c *Consumer) cursorFlusher(ctx context.Context) {
	ticker := time.NewTicker(c.config.CursorFlushInterval)
	defer ticker.Stop()

	var lastFlushed int64

	for {
		select {
		case <-ctx.Done():
			// Final flush
			c.flushCursor(context.Background())
			return
		case <-c.cursorDone:
			// Final flush
			c.flushCursor(context.Background())
			return
		case <-ticker.C:
			c.cursorMu.Lock()
			cursor := c.cursor
			c.cursorMu.Unlock()

			if cursor > lastFlushed {
				if err := c.saveCursor(ctx, cursor); err != nil {
					slog.Warn("Failed to save cursor", "error", err)
				} else {
					lastFlushed = cursor
				}
			}
		}
	}
}

// flushCursor flushes the current cursor immediately.
func (c *Consumer) flushCursor(ctx context.Context) {
	c.cursorMu.Lock()
	cursor := c.cursor
	c.cursorMu.Unlock()

	if cursor > 0 {
		if err := c.saveCursor(ctx, cursor); err != nil {
			slog.Warn("Failed to flush cursor", "error", err)
		}
	}
}

// loadCursor loads the cursor from the config table.
func (c *Consumer) loadCursor(ctx context.Context) (int64, error) {
	value, err := c.configRepo.Get(ctx, "jetstream_cursor")
	if err != nil {
		return 0, err
	}
	if value == "" {
		return 0, nil
	}

	var cursor int64
	if err := json.Unmarshal([]byte(value), &cursor); err != nil {
		return 0, err
	}
	return cursor, nil
}

// saveCursor saves the cursor to the config table.
func (c *Consumer) saveCursor(ctx context.Context, cursor int64) error {
	value, err := json.Marshal(cursor)
	if err != nil {
		return err
	}
	return c.configRepo.Set(ctx, "jetstream_cursor", string(value))
}
