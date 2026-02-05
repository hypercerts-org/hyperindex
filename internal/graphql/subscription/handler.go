package subscription

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/graphql-go/graphql"
)

const (
	// WebSocket subprotocol for GraphQL
	graphqlWSProtocol = "graphql-transport-ws"

	// Message types for graphql-transport-ws protocol
	msgConnectionInit      = "connection_init"
	msgConnectionAck       = "connection_ack"
	msgPing                = "ping"
	msgPong                = "pong"
	msgSubscribe           = "subscribe"
	msgNext                = "next"
	msgError               = "error"
	msgComplete            = "complete"
	msgConnectionTerminate = "connection_terminate"
)

// wsMessage represents a WebSocket message.
type wsMessage struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// subscribePayload is the payload for subscribe messages.
type subscribePayload struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

// Handler handles WebSocket connections for GraphQL subscriptions.
type Handler struct {
	schema   *graphql.Schema
	upgrader websocket.Upgrader
}

// NewHandler creates a new subscription handler.
func NewHandler(schema *graphql.Schema) *Handler {
	return &Handler{
		schema: schema,
		upgrader: websocket.Upgrader{
			Subprotocols: []string{graphqlWSProtocol},
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
	}
}

// ServeHTTP upgrades HTTP to WebSocket and handles subscriptions.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}

	client := &wsClient{
		conn:          conn,
		schema:        h.schema,
		subscriptions: make(map[string]context.CancelFunc),
	}

	go client.run()
}

// wsClient manages a single WebSocket connection.
type wsClient struct {
	conn          *websocket.Conn
	schema        *graphql.Schema
	subscriptions map[string]context.CancelFunc
	mu            sync.Mutex
	initialized   bool
}

// run handles the WebSocket connection lifecycle.
func (c *wsClient) run() {
	defer c.close()

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Debug("WebSocket closed unexpectedly", "error", err)
			}
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			slog.Debug("Invalid WebSocket message", "error", err)
			continue
		}

		c.handleMessage(&msg)
	}
}

// handleMessage processes incoming WebSocket messages.
func (c *wsClient) handleMessage(msg *wsMessage) {
	switch msg.Type {
	case msgConnectionInit:
		c.initialized = true
		c.send(&wsMessage{Type: msgConnectionAck})

	case msgPing:
		c.send(&wsMessage{Type: msgPong})

	case msgSubscribe:
		if !c.initialized {
			c.sendError(msg.ID, "Connection not initialized")
			return
		}
		c.handleSubscribe(msg)

	case msgComplete:
		c.cancelSubscription(msg.ID)

	case msgConnectionTerminate:
		c.close()
	}
}

// handleSubscribe starts a new subscription.
func (c *wsClient) handleSubscribe(msg *wsMessage) {
	var payload subscribePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.sendError(msg.ID, "Invalid subscribe payload")
		return
	}

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	c.mu.Lock()
	c.subscriptions[msg.ID] = cancel
	c.mu.Unlock()

	// Start subscription in goroutine
	go c.runSubscription(ctx, msg.ID, payload)
}

// runSubscription executes a subscription and sends events.
func (c *wsClient) runSubscription(ctx context.Context, id string, payload subscribePayload) {
	defer c.completeSubscription(id)

	// Subscribe to events
	collection := ""
	if vars := payload.Variables; vars != nil {
		if col, ok := vars["collection"].(string); ok {
			collection = col
		}
	}

	sub := Global().Subscribe(collection)
	defer Global().Unsubscribe(sub)

	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-sub.Events:
			if !ok {
				return
			}

			// Convert event to map for GraphQL root object
			rootObject := map[string]interface{}{
				"recordEvents": map[string]interface{}{
					"type":       string(event.Type),
					"uri":        event.URI,
					"cid":        event.CID,
					"did":        event.DID,
					"collection": event.Collection,
					"record":     event.Record,
				},
			}

			// Execute GraphQL query with the event as root value
			result := graphql.Do(graphql.Params{
				Schema:         *c.schema,
				RequestString:  payload.Query,
				OperationName:  payload.OperationName,
				VariableValues: payload.Variables,
				Context:        ctx,
				RootObject:     rootObject,
			})

			// Send result
			if len(result.Errors) > 0 {
				errPayload, _ := json.Marshal(result.Errors)
				c.send(&wsMessage{
					ID:      id,
					Type:    msgError,
					Payload: errPayload,
				})
			} else {
				dataPayload, _ := json.Marshal(map[string]interface{}{
					"data": result.Data,
				})
				c.send(&wsMessage{
					ID:      id,
					Type:    msgNext,
					Payload: dataPayload,
				})
			}
		}
	}
}

// cancelSubscription cancels an active subscription.
func (c *wsClient) cancelSubscription(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cancel, ok := c.subscriptions[id]; ok {
		cancel()
		delete(c.subscriptions, id)
	}
}

// completeSubscription sends a complete message.
func (c *wsClient) completeSubscription(id string) {
	c.send(&wsMessage{
		ID:   id,
		Type: msgComplete,
	})

	c.mu.Lock()
	delete(c.subscriptions, id)
	c.mu.Unlock()
}

// sendError sends an error message.
func (c *wsClient) sendError(id string, message string) {
	errPayload, _ := json.Marshal([]map[string]string{
		{"message": message},
	})
	c.send(&wsMessage{
		ID:      id,
		Type:    msgError,
		Payload: errPayload,
	})
}

// send writes a message to the WebSocket.
func (c *wsClient) send(msg *wsMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		slog.Debug("WebSocket write failed", "error", err)
	}
}

// close closes the WebSocket connection and all subscriptions.
func (c *wsClient) close() {
	c.mu.Lock()
	for id, cancel := range c.subscriptions {
		cancel()
		delete(c.subscriptions, id)
	}
	c.mu.Unlock()

	c.conn.Close()
}
