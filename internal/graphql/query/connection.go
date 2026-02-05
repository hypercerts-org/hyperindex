// Package query provides GraphQL query type building.
package query

import (
	"github.com/graphql-go/graphql"
)

// PageInfo represents Relay-style pagination info.
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

// ConnectionArgs returns standard Relay connection arguments.
func ConnectionArgs() graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{
		"first": &graphql.ArgumentConfig{
			Type:        graphql.Int,
			Description: "Number of items to return from the start",
		},
		"after": &graphql.ArgumentConfig{
			Type:        graphql.String,
			Description: "Cursor to start after (forward pagination)",
		},
		"last": &graphql.ArgumentConfig{
			Type:        graphql.Int,
			Description: "Number of items to return from the end",
		},
		"before": &graphql.ArgumentConfig{
			Type:        graphql.String,
			Description: "Cursor to start before (backward pagination)",
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
