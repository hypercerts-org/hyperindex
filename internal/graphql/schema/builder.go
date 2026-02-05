// Package schema provides the GraphQL schema builder.
package schema

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/graphql-go/graphql"

	"github.com/GainForest/hypergoat/internal/graphql/query"
	"github.com/GainForest/hypergoat/internal/graphql/resolver"
	"github.com/GainForest/hypergoat/internal/graphql/subscription"
	"github.com/GainForest/hypergoat/internal/graphql/types"
	"github.com/GainForest/hypergoat/internal/lexicon"
)

// Builder builds a GraphQL schema from lexicon definitions.
type Builder struct {
	registry      *lexicon.Registry
	mapper        *types.Mapper
	objectBuilder *types.ObjectBuilder

	// Built types
	recordTypes     map[string]*graphql.Object // lexiconID -> record type
	connectionTypes map[string]*graphql.Object // lexiconID -> connection type
}

// NewBuilder creates a new schema builder.
func NewBuilder(registry *lexicon.Registry) *Builder {
	mapper := types.NewMapper()
	return &Builder{
		registry:        registry,
		mapper:          mapper,
		objectBuilder:   types.NewObjectBuilder(mapper, registry),
		recordTypes:     make(map[string]*graphql.Object),
		connectionTypes: make(map[string]*graphql.Object),
	}
}

// Build builds the complete GraphQL schema.
func (b *Builder) Build() (*graphql.Schema, error) {
	// Phase 1: Build all object types (non-record helper types)
	b.buildObjectTypes()

	// Phase 2: Build all record types
	b.buildRecordTypes()

	// Phase 3: Build connection types
	b.buildConnectionTypes()

	// Phase 4: Build Query type
	queryType := b.buildQueryType()

	// Phase 5: Build Subscription type
	subscriptionType := b.buildSubscriptionType()

	// Create schema
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:        queryType,
		Subscription: subscriptionType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &schema, nil
}

// buildObjectTypes builds GraphQL types for all non-record definitions.
func (b *Builder) buildObjectTypes() {
	// Get all lexicons that only have defs (no main record)
	for _, lex := range b.registry.GetDefsLexicons() {
		for defName, def := range lex.Defs.Others {
			if def.IsObject() {
				ref := lexicon.MakeRef(lex.ID, defName)
				b.objectBuilder.BuildObjectType(ref, def.Object)
			}
		}
	}

	// Also build object defs from collection lexicons
	for _, lex := range b.registry.GetCollectionLexicons() {
		for defName, def := range lex.Defs.Others {
			if def.IsObject() {
				ref := lexicon.MakeRef(lex.ID, defName)
				b.objectBuilder.BuildObjectType(ref, def.Object)
			}
		}
	}
}

// buildRecordTypes builds GraphQL types for all record definitions.
func (b *Builder) buildRecordTypes() {
	for _, lex := range b.registry.GetCollectionLexicons() {
		if lex.Defs.Main != nil {
			recordType := b.objectBuilder.BuildRecordType(lex.ID, lex.Defs.Main)
			b.recordTypes[lex.ID] = recordType
		}
	}
}

// buildConnectionTypes builds Relay connection types for all record types.
func (b *Builder) buildConnectionTypes() {
	for lexiconID, recordType := range b.recordTypes {
		connectionType := query.BuildConnectionType(recordType)
		b.connectionTypes[lexiconID] = connectionType
	}
}

// RecordEvent GraphQL type for subscriptions
var recordEventType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "RecordEvent",
	Description: "A real-time record change event",
	Fields: graphql.Fields{
		"type": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.String),
			Description: "Event type: create, update, or delete",
		},
		"uri": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.String),
			Description: "AT-URI of the record",
		},
		"cid": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.String),
			Description: "CID of the record",
		},
		"did": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.String),
			Description: "DID of the actor who made the change",
		},
		"collection": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.String),
			Description: "Collection NSID",
		},
		"record": &graphql.Field{
			Type:        types.JSONScalar,
			Description: "The record data (null for delete events)",
		},
	},
})

// buildSubscriptionType builds the root Subscription type.
func (b *Builder) buildSubscriptionType() *graphql.Object {
	fields := graphql.Fields{
		// Subscribe to all record events
		"recordEvents": &graphql.Field{
			Type:        recordEventType,
			Description: "Subscribe to all record change events",
			Args: graphql.FieldConfigArgument{
				"collection": &graphql.ArgumentConfig{
					Type:        graphql.String,
					Description: "Filter by collection NSID (optional)",
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				// This is called for each event - just return the event
				return p.Source, nil
			},
		},
	}

	// Add per-collection subscriptions
	for lexiconID, recordType := range b.recordTypes {
		fieldName := lexicon.ToFieldName(lexiconID) + "Events"
		collection := lexiconID // Capture for closure

		fields[fieldName] = &graphql.Field{
			Type:        recordType,
			Description: fmt.Sprintf("Subscribe to %s record changes", lexiconID),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				event, ok := p.Source.(*subscription.RecordEvent)
				if !ok || event == nil {
					return nil, nil
				}
				// Only return if collection matches
				if event.Collection != collection {
					return nil, nil
				}
				return event.Record, nil
			},
		}
	}

	return graphql.NewObject(graphql.ObjectConfig{
		Name:   "Subscription",
		Fields: fields,
	})
}

// buildQueryType builds the root Query type with fields for each collection.
func (b *Builder) buildQueryType() *graphql.Object {
	fields := graphql.Fields{}

	for lexiconID, connectionType := range b.connectionTypes {
		fieldName := lexicon.ToFieldName(lexiconID)

		fields[fieldName] = &graphql.Field{
			Type:        connectionType,
			Description: fmt.Sprintf("Query %s records", lexiconID),
			Args:        query.ConnectionArgs(),
			Resolve:     b.createCollectionResolver(lexiconID),
		}

		// Also add a singular lookup by URI
		recordType := b.recordTypes[lexiconID]
		fields[fieldName+"ByUri"] = &graphql.Field{
			Type:        recordType,
			Description: fmt.Sprintf("Get a single %s by AT-URI", lexiconID),
			Args: graphql.FieldConfigArgument{
				"uri": &graphql.ArgumentConfig{
					Type:        graphql.NewNonNull(graphql.String),
					Description: "AT-URI of the record",
				},
			},
			Resolve: b.createSingleRecordResolver(lexiconID),
		}
	}

	return graphql.NewObject(graphql.ObjectConfig{
		Name:   "Query",
		Fields: fields,
	})
}

// createCollectionResolver creates a resolver for querying a collection.
func (b *Builder) createCollectionResolver(lexiconID string) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		// Get repositories from context
		repos := resolver.GetRepositories(p.Context)
		if repos == nil || repos.Records == nil {
			// Return empty connection if no repos (for tests without DB)
			return emptyConnection(), nil
		}

		// Extract pagination args
		first, _ := p.Args["first"].(int)
		after, _ := p.Args["after"].(string)

		// Default limit
		if first == 0 {
			first = 20
		}

		// Query database (fetch one extra to determine hasNextPage)
		records, err := repos.Records.GetByCollection(p.Context, lexiconID, first+1)
		if err != nil {
			return nil, fmt.Errorf("failed to query records: %w", err)
		}

		// Determine if there are more results
		hasNextPage := len(records) > first
		if hasNextPage {
			records = records[:first] // Trim to requested count
		}

		// Build edges
		edges := make([]interface{}, 0, len(records))
		var startCursor, endCursor string

		for _, rec := range records {
			// Parse JSON to map
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(rec.JSON), &data); err != nil {
				continue // Skip records with invalid JSON
			}

			// Add standard record fields
			data["uri"] = rec.URI
			data["cid"] = rec.CID

			cursor := encodeCursor(rec.URI)
			if startCursor == "" {
				startCursor = cursor
			}
			endCursor = cursor

			edges = append(edges, map[string]interface{}{
				"cursor": cursor,
				"node":   data,
			})
		}

		return map[string]interface{}{
			"edges": edges,
			"pageInfo": map[string]interface{}{
				"hasNextPage":     hasNextPage,
				"hasPreviousPage": after != "",
				"startCursor":     startCursor,
				"endCursor":       endCursor,
			},
			"totalCount": nil, // Could add COUNT query if needed
		}, nil
	}
}

// createSingleRecordResolver creates a resolver for fetching a single record.
func (b *Builder) createSingleRecordResolver(lexiconID string) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		uri, ok := p.Args["uri"].(string)
		if !ok {
			return nil, fmt.Errorf("uri is required")
		}

		// Get repositories from context
		repos := resolver.GetRepositories(p.Context)
		if repos == nil || repos.Records == nil {
			return nil, nil
		}

		// Query database
		rec, err := repos.Records.GetByURI(p.Context, uri)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil // Not found
			}
			return nil, fmt.Errorf("failed to fetch record: %w", err)
		}

		// Parse JSON to map
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(rec.JSON), &data); err != nil {
			return nil, fmt.Errorf("failed to parse record JSON: %w", err)
		}

		// Add standard record fields
		data["uri"] = rec.URI
		data["cid"] = rec.CID

		return data, nil
	}
}

// emptyConnection returns an empty Relay connection.
func emptyConnection() map[string]interface{} {
	return map[string]interface{}{
		"edges": []interface{}{},
		"pageInfo": map[string]interface{}{
			"hasNextPage":     false,
			"hasPreviousPage": false,
			"startCursor":     nil,
			"endCursor":       nil,
		},
		"totalCount": 0,
	}
}

// encodeCursor encodes a URI as a base64 cursor.
func encodeCursor(uri string) string {
	return base64.URLEncoding.EncodeToString([]byte(uri))
}

// decodeCursor decodes a base64 cursor to a URI.
// TODO: Use this for cursor-based pagination with "after" argument
var _ = decodeCursor // Mark as intentionally available for future use

func decodeCursor(cursor string) (string, error) {
	data, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetRecordType returns the GraphQL type for a record.
func (b *Builder) GetRecordType(lexiconID string) *graphql.Object {
	return b.recordTypes[lexiconID]
}

// GetConnectionType returns the connection type for a record.
func (b *Builder) GetConnectionType(lexiconID string) *graphql.Object {
	return b.connectionTypes[lexiconID]
}
