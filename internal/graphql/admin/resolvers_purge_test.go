package admin

import (
	"context"
	"testing"

	"github.com/GainForest/hypergoat/internal/testutil"
)

func TestResolver_PurgeActor_RemoveFromTapCallback(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx := context.Background()

	repos := &Repositories{
		Records: db.Records,
		Actors:  db.Actors,
	}
	r := NewResolver(repos, "did:plc:test-labeler")

	if err := db.Actors.Upsert(ctx, "did:plc:target", "target.bsky.social"); err != nil {
		t.Fatalf("failed to seed actor: %v", err)
	}
	if _, err := db.Records.Insert(ctx,
		"at://did:plc:target/app.certified.actor.profile/1",
		"cid1",
		"did:plc:target",
		"app.certified.actor.profile",
		`{"displayName":"Target"}`,
	); err != nil {
		t.Fatalf("failed to seed record: %v", err)
	}

	called := 0
	calledDID := ""
	r.SetRemoveRepoCallback(func(_ context.Context, did string) error {
		called++
		calledDID = did
		return nil
	})

	ok, err := r.PurgeActor(ctx, "did:plc:target", "PURGE", true)
	if err != nil {
		t.Fatalf("PurgeActor() error = %v", err)
	}
	if !ok {
		t.Fatal("PurgeActor() = false, want true")
	}
	if called != 1 {
		t.Fatalf("expected callback called once, got %d", called)
	}
	if calledDID != "did:plc:target" {
		t.Fatalf("callback DID = %q, want %q", calledDID, "did:plc:target")
	}
}

func TestResolver_PurgeActor_RemoveFromTapFalse_DoesNotCallCallback(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx := context.Background()

	repos := &Repositories{
		Records: db.Records,
		Actors:  db.Actors,
	}
	r := NewResolver(repos, "did:plc:test-labeler")

	if err := db.Actors.Upsert(ctx, "did:plc:target", "target.bsky.social"); err != nil {
		t.Fatalf("failed to seed actor: %v", err)
	}

	called := 0
	r.SetRemoveRepoCallback(func(_ context.Context, _ string) error {
		called++
		return nil
	})

	ok, err := r.PurgeActor(ctx, "did:plc:target", "PURGE", false)
	if err != nil {
		t.Fatalf("PurgeActor() error = %v", err)
	}
	if !ok {
		t.Fatal("PurgeActor() = false, want true")
	}
	if called != 0 {
		t.Fatalf("expected callback not called, got %d", called)
	}
}

func TestResolver_PurgeActor_TrimsDID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx := context.Background()

	repos := &Repositories{
		Records: db.Records,
		Actors:  db.Actors,
	}
	r := NewResolver(repos, "did:plc:test-labeler")

	if err := db.Actors.Upsert(ctx, "did:plc:target", "target.bsky.social"); err != nil {
		t.Fatalf("failed to seed actor: %v", err)
	}
	if _, err := db.Records.Insert(ctx,
		"at://did:plc:target/app.certified.actor.profile/1",
		"cid1",
		"did:plc:target",
		"app.certified.actor.profile",
		`{"displayName":"Target"}`,
	); err != nil {
		t.Fatalf("failed to seed record: %v", err)
	}

	calledDID := ""
	r.SetRemoveRepoCallback(func(_ context.Context, did string) error {
		calledDID = did
		return nil
	})

	ok, err := r.PurgeActor(ctx, "  did:plc:target  ", "PURGE", true)
	if err != nil {
		t.Fatalf("PurgeActor() error = %v", err)
	}
	if !ok {
		t.Fatal("PurgeActor() = false, want true")
	}
	if calledDID != "did:plc:target" {
		t.Fatalf("callback DID = %q, want %q", calledDID, "did:plc:target")
	}
}
