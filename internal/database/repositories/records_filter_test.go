// Package repositories contains data access layer implementations.
package repositories

import (
	"strings"
	"testing"

	"github.com/GainForest/hypergoat/internal/database"
	"github.com/GainForest/hypergoat/internal/database/sqlite"
)

// newTestRepo creates a RecordsRepository backed by an in-memory SQLite executor for unit tests.
func newTestRepo(t *testing.T) *RecordsRepository {
	t.Helper()
	exec, err := sqlite.NewExecutor("sqlite::memory:")
	if err != nil {
		t.Fatalf("failed to create sqlite executor: %v", err)
	}
	t.Cleanup(func() { exec.Close() })
	return NewRecordsRepository(exec)
}

func TestBuildFilterClause_EmptyFilters(t *testing.T) {
	repo := newTestRepo(t)
	clause, params := repo.buildFilterClause(nil, 1)
	if clause != "" {
		t.Errorf("empty filters: clause = %q, want empty string", clause)
	}
	if params != nil {
		t.Errorf("empty filters: params = %v, want nil", params)
	}

	clause2, params2 := repo.buildFilterClause([]FieldFilter{}, 1)
	if clause2 != "" {
		t.Errorf("empty slice: clause = %q, want empty string", clause2)
	}
	if params2 != nil {
		t.Errorf("empty slice: params = %v, want nil", params2)
	}
}

func TestBuildFilterClause_Operators(t *testing.T) {
	repo := newTestRepo(t)

	tests := []struct {
		name         string
		filter       FieldFilter
		wantContains string // substring expected in the clause
		wantParams   int    // number of params expected
	}{
		{
			name:         "eq operator",
			filter:       FieldFilter{Field: "title", Operator: "eq", Value: "hello", FieldType: "string"},
			wantContains: "= ?",
			wantParams:   1,
		},
		{
			name:         "neq operator",
			filter:       FieldFilter{Field: "title", Operator: "neq", Value: "hello", FieldType: "string"},
			wantContains: "!= ?",
			wantParams:   1,
		},
		{
			name:         "gt operator",
			filter:       FieldFilter{Field: "score", Operator: "gt", Value: 5, FieldType: "integer"},
			wantContains: "> ?",
			wantParams:   1,
		},
		{
			name:         "lt operator",
			filter:       FieldFilter{Field: "score", Operator: "lt", Value: 10, FieldType: "integer"},
			wantContains: "< ?",
			wantParams:   1,
		},
		{
			name:         "gte operator",
			filter:       FieldFilter{Field: "score", Operator: "gte", Value: 5, FieldType: "number"},
			wantContains: ">= ?",
			wantParams:   1,
		},
		{
			name:         "lte operator",
			filter:       FieldFilter{Field: "score", Operator: "lte", Value: 10, FieldType: "number"},
			wantContains: "<= ?",
			wantParams:   1,
		},
		{
			name:         "contains operator wraps value in percent",
			filter:       FieldFilter{Field: "body", Operator: "contains", Value: "world", FieldType: "string"},
			wantContains: "LIKE ?",
			wantParams:   1,
		},
		{
			name:         "startsWith operator appends percent",
			filter:       FieldFilter{Field: "body", Operator: "startsWith", Value: "hello", FieldType: "string"},
			wantContains: "LIKE ?",
			wantParams:   1,
		},
		{
			name:         "isNull true generates IS NULL",
			filter:       FieldFilter{Field: "deletedAt", Operator: "isNull", Value: true, FieldType: "string"},
			wantContains: "IS NULL",
			wantParams:   0,
		},
		{
			name:         "isNull false generates IS NOT NULL",
			filter:       FieldFilter{Field: "deletedAt", Operator: "isNull", Value: false, FieldType: "string"},
			wantContains: "IS NOT NULL",
			wantParams:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clause, params := repo.buildFilterClause([]FieldFilter{tt.filter}, 1)
			if clause == "" {
				t.Fatalf("clause is empty, want non-empty")
			}
			if !strings.Contains(clause, tt.wantContains) {
				t.Errorf("clause = %q, want to contain %q", clause, tt.wantContains)
			}
			if len(params) != tt.wantParams {
				t.Errorf("params count = %d, want %d", len(params), tt.wantParams)
			}
		})
	}
}

func TestBuildFilterClause_ContainsWrapsValue(t *testing.T) {
	repo := newTestRepo(t)
	filters := []FieldFilter{
		{Field: "body", Operator: "contains", Value: "world", FieldType: "string"},
	}
	_, params := repo.buildFilterClause(filters, 1)
	if len(params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(params))
	}
	tv, ok := params[0].(database.TextValue)
	if !ok {
		t.Fatalf("param is not TextValue, got %T", params[0])
	}
	if string(tv) != "%world%" {
		t.Errorf("contains param = %q, want %%world%%", string(tv))
	}
}

func TestBuildFilterClause_StartsWithAppendsPercent(t *testing.T) {
	repo := newTestRepo(t)
	filters := []FieldFilter{
		{Field: "body", Operator: "startsWith", Value: "hello", FieldType: "string"},
	}
	_, params := repo.buildFilterClause(filters, 1)
	if len(params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(params))
	}
	tv, ok := params[0].(database.TextValue)
	if !ok {
		t.Fatalf("param is not TextValue, got %T", params[0])
	}
	if string(tv) != "hello%" {
		t.Errorf("startsWith param = %q, want hello%%", string(tv))
	}
}

func TestBuildFilterClause_InOperator(t *testing.T) {
	repo := newTestRepo(t)
	filters := []FieldFilter{
		{Field: "status", Operator: "in", Value: []interface{}{"active", "pending", "closed"}, FieldType: "string"},
	}
	clause, params := repo.buildFilterClause(filters, 1)
	if !strings.Contains(clause, "IN (") {
		t.Errorf("clause = %q, want to contain IN (", clause)
	}
	if len(params) != 3 {
		t.Errorf("params count = %d, want 3", len(params))
	}
}

func TestBuildFilterClause_NumericCast(t *testing.T) {
	repo := newTestRepo(t)

	t.Run("integer type uses CAST AS REAL in SQLite", func(t *testing.T) {
		filters := []FieldFilter{
			{Field: "score", Operator: "gt", Value: 5, FieldType: "integer"},
		}
		clause, _ := repo.buildFilterClause(filters, 1)
		if !strings.Contains(clause, "CAST(") {
			t.Errorf("integer filter clause = %q, want CAST(...)", clause)
		}
		if !strings.Contains(clause, "AS REAL") {
			t.Errorf("integer filter clause = %q, want AS REAL", clause)
		}
	})

	t.Run("number type uses CAST AS REAL in SQLite", func(t *testing.T) {
		filters := []FieldFilter{
			{Field: "price", Operator: "lte", Value: 99.99, FieldType: "number"},
		}
		clause, _ := repo.buildFilterClause(filters, 1)
		if !strings.Contains(clause, "CAST(") {
			t.Errorf("number filter clause = %q, want CAST(...)", clause)
		}
		if !strings.Contains(clause, "AS REAL") {
			t.Errorf("number filter clause = %q, want AS REAL", clause)
		}
	})

	t.Run("string type does not use CAST", func(t *testing.T) {
		filters := []FieldFilter{
			{Field: "title", Operator: "eq", Value: "hello", FieldType: "string"},
		}
		clause, _ := repo.buildFilterClause(filters, 1)
		if strings.Contains(clause, "CAST(") {
			t.Errorf("string filter clause = %q, should not contain CAST", clause)
		}
	})
}

func TestBuildFilterClause_MultipleFilters(t *testing.T) {
	repo := newTestRepo(t)
	filters := []FieldFilter{
		{Field: "title", Operator: "eq", Value: "hello", FieldType: "string"},
		{Field: "score", Operator: "gt", Value: 5, FieldType: "integer"},
		{Field: "deletedAt", Operator: "isNull", Value: true, FieldType: "string"},
	}
	clause, params := repo.buildFilterClause(filters, 1)

	// Should be joined with AND
	parts := strings.Split(clause, " AND ")
	if len(parts) != 3 {
		t.Errorf("expected 3 AND-joined conditions, got %d in clause: %q", len(parts), clause)
	}
	// Two params: eq and gt (isNull has no param)
	if len(params) != 2 {
		t.Errorf("params count = %d, want 2", len(params))
	}
}
