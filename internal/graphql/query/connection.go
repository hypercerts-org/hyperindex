// Package query provides GraphQL query type building.
package query

import (
	"github.com/graphql-go/graphql"
)

const (
	// DefaultPageSize is the number of records returned when no first argument is provided.
	DefaultPageSize = 20
	// MaxPageSize is the maximum number of records that can be requested in a single page.
	MaxPageSize = 100
)

// ClampPageSize returns a valid page size within [1, MaxPageSize], defaulting to DefaultPageSize.
func ClampPageSize(first int) int {
	if first <= 0 {
		return DefaultPageSize
	}
	if first > MaxPageSize {
		return MaxPageSize
	}
	return first
}

// PageInfoType defines the Relay-style pagination info GraphQL type.
var PageInfoType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "PageInfo",
	Description: "Information about pagination in a connection",
	Fields: graphql.Fields{
		"hasNextPage": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.Boolean),
			Description: "Whether there are more items after the last edge",
		},
		"hasPreviousPage": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.Boolean),
			Description: "Whether there are more items before the first edge",
		},
		"startCursor": &graphql.Field{
			Type:        graphql.String,
			Description: "Cursor of the first edge",
		},
		"endCursor": &graphql.Field{
			Type:        graphql.String,
			Description: "Cursor of the last edge",
		},
	},
})

// ConnectionArgs returns standard Relay connection arguments for forward pagination.
func ConnectionArgs() graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{
		"first": &graphql.ArgumentConfig{
			Type:        graphql.Int,
			Description: "Number of items to return (default 20)",
		},
		"after": &graphql.ArgumentConfig{
			Type:        graphql.String,
			Description: "Cursor to start after (forward pagination)",
		},
	}
}

// BuildEdgeType creates an Edge type for a given node type.
func BuildEdgeType(nodeType *graphql.Object) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name:        nodeType.Name() + "Edge",
		Description: "An edge in a " + nodeType.Name() + " connection",
		Fields: graphql.Fields{
			"cursor": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "Cursor for this edge",
			},
			"node": &graphql.Field{
				Type:        graphql.NewNonNull(nodeType),
				Description: "The item at the end of the edge",
			},
		},
	})
}

// BuildConnectionType creates a Connection type for a given node type.
func BuildConnectionType(nodeType *graphql.Object) *graphql.Object {
	edgeType := BuildEdgeType(nodeType)

	return graphql.NewObject(graphql.ObjectConfig{
		Name:        nodeType.Name() + "Connection",
		Description: "A paginated list of " + nodeType.Name() + " items",
		Fields: graphql.Fields{
			"edges": &graphql.Field{
				Type:        graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(edgeType))),
				Description: "List of edges",
			},
			"pageInfo": &graphql.Field{
				Type:        graphql.NewNonNull(PageInfoType),
				Description: "Pagination information",
			},
			"totalCount": &graphql.Field{
				Type:        graphql.Int,
				Description: "Total number of items (if known)",
			},
		},
	})
}
