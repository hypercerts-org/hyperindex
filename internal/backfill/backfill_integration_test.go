//go:build integration

// Package backfill provides integration tests for AT Protocol backfill operations.
//
// Run with: go test -tags=integration -v ./internal/backfill/...
package backfill

import (
	"context"
	"testing"
	"time"
)

const (
	// HypercertsActivityCollection is the collection we're testing.
	HypercertsActivityCollection = "org.hypercerts.claim.activity"

	// TestTimeout is the timeout for integration tests.
	TestTimeout = 2 * time.Minute
)

// TestListReposByCollection_HypercertsActivity tests discovering repos
// that have hypercerts.claim.activity records.
func TestListReposByCollection_HypercertsActivity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), TestTimeout)
	defer cancel()

	client := NewClient("", "") // Use defaults

	repos, err := client.ListReposByCollection(ctx, HypercertsActivityCollection)
	if err != nil {
		t.Fatalf("ListReposByCollection failed: %v", err)
	}

	t.Logf("Found %d repos with %s records", len(repos), HypercertsActivityCollection)

	if len(repos) == 0 {
		t.Fatal("Expected to find at least one repo with hypercerts.claim.activity records")
	}

	// Log first few repos
	limit := 5
	if len(repos) < limit {
		limit = len(repos)
	}
	for i := 0; i < limit; i++ {
		t.Logf("  Repo %d: %s", i+1, repos[i])
	}
}

// TestResolveDID tests DID resolution via PLC directory.
func TestResolveDID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := NewClient("", "")

	// First, get a real DID from the collection
	repos, err := client.ListReposByCollection(ctx, HypercertsActivityCollection)
	if err != nil {
		t.Fatalf("ListReposByCollection failed: %v", err)
	}

	if len(repos) == 0 {
		t.Skip("No repos found for testing")
	}

	did := repos[0]
	t.Logf("Testing DID resolution for: %s", did)

	data, err := client.ResolveDID(ctx, did)
	if err != nil {
		t.Fatalf("ResolveDID failed: %v", err)
	}

	t.Logf("Resolved DID:")
	t.Logf("  DID: %s", data.DID)
	t.Logf("  Handle: %s", data.Handle)
	t.Logf("  PDS: %s", data.PDS)

	if data.DID != did {
		t.Errorf("DID mismatch: got %s, want %s", data.DID, did)
	}

	if data.PDS == "" {
		t.Error("Expected non-empty PDS URL")
	}
}

// TestListRecords_HypercertsActivity tests fetching actual records.
func TestListRecords_HypercertsActivity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), TestTimeout)
	defer cancel()

	client := NewClient("", "")

	// Get repos
	repos, err := client.ListReposByCollection(ctx, HypercertsActivityCollection)
	if err != nil {
		t.Fatalf("ListReposByCollection failed: %v", err)
	}

	if len(repos) == 0 {
		t.Skip("No repos found for testing")
	}

	// Try to find a repo with records (some might be empty or have deleted records)
	var totalRecords int
	var successfulRepos int
	maxRepos := 10
	if len(repos) < maxRepos {
		maxRepos = len(repos)
	}

	for i := 0; i < maxRepos; i++ {
		did := repos[i]

		// Resolve DID to get PDS
		data, err := client.ResolveDID(ctx, did)
		if err != nil {
			t.Logf("Failed to resolve DID %s: %v", did, err)
			continue
		}

		// Fetch records
		records, err := client.ListRecords(ctx, data.PDS, did, HypercertsActivityCollection)
		if err != nil {
			t.Logf("Failed to list records for %s: %v", did, err)
			continue
		}

		if len(records) > 0 {
			successfulRepos++
			totalRecords += len(records)
			t.Logf("Repo %s (%s): %d records", did, data.Handle, len(records))

			// Log first record details
			rec := records[0]
			t.Logf("  First record URI: %s", rec.URI)
			t.Logf("  First record CID: %s", rec.CID)
			t.Logf("  First record value (truncated): %.200s...", string(rec.Value))
		}
	}

	t.Logf("\nSummary:")
	t.Logf("  Repos checked: %d", maxRepos)
	t.Logf("  Repos with records: %d", successfulRepos)
	t.Logf("  Total records found: %d", totalRecords)

	if totalRecords == 0 {
		t.Error("Expected to find at least some records")
	}
}

// TestBackfillClient_EndToEnd tests the full backfill flow without database.
func TestBackfillClient_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), TestTimeout)
	defer cancel()

	client := NewClient("", "")

	t.Log("Step 1: Discovering repos...")
	repos, err := client.ListReposByCollection(ctx, HypercertsActivityCollection)
	if err != nil {
		t.Fatalf("Discovery failed: %v", err)
	}
	t.Logf("  Found %d repos", len(repos))

	if len(repos) == 0 {
		t.Skip("No repos to test")
	}

	// Limit to a few repos for the test
	testRepos := repos
	if len(testRepos) > 5 {
		testRepos = testRepos[:5]
	}

	t.Log("\nStep 2: Resolving DIDs and fetching records...")
	var totalRecords int
	for _, did := range testRepos {
		data, err := client.ResolveDID(ctx, did)
		if err != nil {
			t.Logf("  %s: resolve failed: %v", did, err)
			continue
		}

		records, err := client.ListRecords(ctx, data.PDS, did, HypercertsActivityCollection)
		if err != nil {
			t.Logf("  %s: list failed: %v", did, err)
			continue
		}

		totalRecords += len(records)
		t.Logf("  %s (%s): %d records from %s", did, data.Handle, len(records), data.PDS)
	}

	t.Logf("\nTotal records fetched: %d", totalRecords)

	if totalRecords == 0 {
		t.Error("Expected to fetch at least some records in end-to-end test")
	}
}
