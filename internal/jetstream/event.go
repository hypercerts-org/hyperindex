// Package jetstream provides a client for the AT Protocol Jetstream firehose.
package jetstream

import (
	"encoding/json"
	"fmt"
)

// EventType represents the type of Jetstream event.
type EventType string

const (
	EventTypeCommit   EventType = "commit"
	EventTypeIdentity EventType = "identity"
	EventTypeAccount  EventType = "account"
)

// OperationType represents the type of commit operation.
type OperationType string

const (
	OpCreate OperationType = "create"
	OpUpdate OperationType = "update"
	OpDelete OperationType = "delete"
)

// Event represents a Jetstream event envelope.
type Event struct {
	DID    string          `json:"did"`
	TimeUS int64           `json:"time_us"`
	Kind   EventType       `json:"kind"`
	Commit *CommitEvent    `json:"commit,omitempty"`
	Raw    json.RawMessage `json:"-"` // Original message for debugging
}

// CommitEvent represents a commit (record change) event.
type CommitEvent struct {
	Rev        string          `json:"rev"`
	Operation  OperationType   `json:"operation"`
	Collection string          `json:"collection"`
	RKey       string          `json:"rkey"`
	Record     json.RawMessage `json:"record,omitempty"` // Only for create/update
	CID        string          `json:"cid,omitempty"`    // Only for create/update
}

// URI returns the AT-URI for this commit.
func (c *CommitEvent) URI(did string) string {
	return fmt.Sprintf("at://%s/%s/%s", did, c.Collection, c.RKey)
}

// ParseEvent parses a Jetstream event from JSON.
func ParseEvent(data []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to parse event: %w", err)
	}
	event.Raw = data
	return &event, nil
}

// IsCommit returns true if this is a commit event.
func (e *Event) IsCommit() bool {
	return e.Kind == EventTypeCommit && e.Commit != nil
}

// IsCreate returns true if this is a create operation.
func (e *Event) IsCreate() bool {
	return e.IsCommit() && e.Commit.Operation == OpCreate
}

// IsUpdate returns true if this is an update operation.
func (e *Event) IsUpdate() bool {
	return e.IsCommit() && e.Commit.Operation == OpUpdate
}

// IsDelete returns true if this is a delete operation.
func (e *Event) IsDelete() bool {
	return e.IsCommit() && e.Commit.Operation == OpDelete
}
