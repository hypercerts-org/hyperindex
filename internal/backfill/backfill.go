package backfill

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/GainForest/hypergoat/internal/database/repositories"
)

// Config configures the backfill operation.
type Config struct {
	// RelayURL is the AT Protocol relay URL for discovering repos.
	RelayURL string

	// PLCURL is the PLC directory URL for resolving DIDs.
	PLCURL string

	// Collections to backfill.
	Collections []string

	// MaxConcurrentRepos is the max concurrent repo fetches.
	MaxConcurrentRepos int

	// MaxConcurrentPerPDS is the max concurrent requests per PDS.
	MaxConcurrentPerPDS int
}

// DefaultConfig returns a default backfill configuration.
func DefaultConfig() Config {
	return Config{
		RelayURL:            DefaultRelayURL,
		PLCURL:              DefaultPLCURL,
		MaxConcurrentRepos:  10,
		MaxConcurrentPerPDS: 4,
	}
}

// Stats tracks backfill statistics.
type Stats struct {
	ReposDiscovered int64
	ReposProcessed  int64
	RecordsInserted int64
	Errors          int64
	StartTime       time.Time
	EndTime         time.Time
}

// Duration returns the backfill duration.
func (s *Stats) Duration() time.Duration {
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

// Backfiller coordinates historical data backfill.
type Backfiller struct {
	config      Config
	client      *Client
	recordsRepo *repositories.RecordsRepository
	actorsRepo  *repositories.ActorsRepository

	stats Stats
}

// NewBackfiller creates a new backfiller.
func NewBackfiller(
	config Config,
	recordsRepo *repositories.RecordsRepository,
	actorsRepo *repositories.ActorsRepository,
) *Backfiller {
	return &Backfiller{
		config:      config,
		client:      NewClient(config.RelayURL, config.PLCURL),
		recordsRepo: recordsRepo,
		actorsRepo:  actorsRepo,
	}
}

// Run executes the backfill operation.
func (b *Backfiller) Run(ctx context.Context) (*Stats, error) {
	b.stats = Stats{StartTime: time.Now()}

	slog.Info("[backfill] Starting backfill operation",
		"collections", b.config.Collections,
		"relay", b.config.RelayURL,
	)

	// Step 1: Discover all repos for all collections
	allRepos := make(map[string]struct{})
	for _, collection := range b.config.Collections {
		slog.Info("[backfill] Discovering repos for collection", "collection", collection)

		repos, err := b.client.ListReposByCollection(ctx, collection)
		if err != nil {
			slog.Warn("[backfill] Failed to list repos for collection",
				"collection", collection,
				"error", err,
			)
			atomic.AddInt64(&b.stats.Errors, 1)
			continue
		}

		for _, repo := range repos {
			allRepos[repo] = struct{}{}
		}

		slog.Info("[backfill] Found repos for collection",
			"collection", collection,
			"count", len(repos),
		)
	}

	repoList := make([]string, 0, len(allRepos))
	for repo := range allRepos {
		repoList = append(repoList, repo)
	}
	atomic.StoreInt64(&b.stats.ReposDiscovered, int64(len(repoList)))

	slog.Info("[backfill] Total unique repos discovered", "count", len(repoList))

	if len(repoList) == 0 {
		slog.Info("[backfill] No repos found, nothing to backfill")
		b.stats.EndTime = time.Now()
		return &b.stats, nil
	}

	// Step 2: Resolve DIDs and group by PDS
	slog.Info("[backfill] Resolving DIDs...")
	reposByPDS := b.resolveAndGroupByPDS(ctx, repoList)

	// Step 3: Process each PDS concurrently
	slog.Info("[backfill] Processing repos by PDS",
		"pds_count", len(reposByPDS),
		"max_concurrent", b.config.MaxConcurrentRepos,
	)

	b.processReposByPDS(ctx, reposByPDS)

	b.stats.EndTime = time.Now()

	slog.Info("[backfill] Backfill complete",
		"repos_discovered", b.stats.ReposDiscovered,
		"repos_processed", b.stats.ReposProcessed,
		"records_inserted", b.stats.RecordsInserted,
		"errors", b.stats.Errors,
		"duration", b.stats.Duration(),
	)

	return &b.stats, nil
}

// resolveAndGroupByPDS resolves DIDs and groups repos by their PDS.
func (b *Backfiller) resolveAndGroupByPDS(ctx context.Context, repos []string) map[string][]*AtprotoData {
	result := make(map[string][]*AtprotoData)
	var mu sync.Mutex

	// Use a semaphore to limit concurrent DID resolutions
	sem := make(chan struct{}, b.config.MaxConcurrentRepos)
	var wg sync.WaitGroup

	for _, repo := range repos {
		wg.Add(1)
		go func(did string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			data, err := b.client.ResolveDID(ctx, did)
			if err != nil {
				slog.Debug("[backfill] Failed to resolve DID",
					"did", did,
					"error", err,
				)
				atomic.AddInt64(&b.stats.Errors, 1)
				return
			}

			// Ensure actor exists in database
			if err := b.actorsRepo.Upsert(ctx, data.DID, data.Handle); err != nil {
				slog.Debug("[backfill] Failed to upsert actor",
					"did", data.DID,
					"error", err,
				)
			}

			mu.Lock()
			result[data.PDS] = append(result[data.PDS], data)
			mu.Unlock()
		}(repo)
	}

	wg.Wait()

	return result
}

// processReposByPDS processes repos grouped by PDS.
func (b *Backfiller) processReposByPDS(ctx context.Context, reposByPDS map[string][]*AtprotoData) {
	// Process PDS endpoints concurrently, but limit per-PDS concurrency
	var wg sync.WaitGroup

	for pdsURL, repos := range reposByPDS {
		wg.Add(1)
		go func(pds string, repoList []*AtprotoData) {
			defer wg.Done()
			b.processPDS(ctx, pds, repoList)
		}(pdsURL, repos)
	}

	wg.Wait()
}

// processPDS processes all repos for a single PDS.
func (b *Backfiller) processPDS(ctx context.Context, pdsURL string, repos []*AtprotoData) {
	slog.Debug("[backfill] Processing PDS",
		"pds", pdsURL,
		"repo_count", len(repos),
	)

	// Use a semaphore to limit concurrent requests to this PDS
	sem := make(chan struct{}, b.config.MaxConcurrentPerPDS)
	var wg sync.WaitGroup

	for _, repo := range repos {
		wg.Add(1)
		go func(data *AtprotoData) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			b.processRepo(ctx, pdsURL, data)
		}(repo)
	}

	wg.Wait()
}

// processRepo processes a single repo, fetching all collections.
func (b *Backfiller) processRepo(ctx context.Context, pdsURL string, data *AtprotoData) {
	for _, collection := range b.config.Collections {
		records, err := b.client.ListRecords(ctx, pdsURL, data.DID, collection)
		if err != nil {
			slog.Debug("[backfill] Failed to list records",
				"did", data.DID,
				"collection", collection,
				"error", err,
			)
			atomic.AddInt64(&b.stats.Errors, 1)
			continue
		}

		for _, rec := range records {
			result, err := b.recordsRepo.Insert(ctx, rec.URI, rec.CID, data.DID, collection, string(rec.Value))
			if err != nil {
				slog.Debug("[backfill] Failed to insert record",
					"uri", rec.URI,
					"error", err,
				)
				atomic.AddInt64(&b.stats.Errors, 1)
				continue
			}

			if result == repositories.Inserted {
				atomic.AddInt64(&b.stats.RecordsInserted, 1)
			}
		}
	}

	atomic.AddInt64(&b.stats.ReposProcessed, 1)
}

// BackfillActor backfills all collections for a single actor.
func (b *Backfiller) BackfillActor(ctx context.Context, did string) (int, error) {
	slog.Info("[backfill] Starting actor backfill", "did", did)

	// Resolve DID
	data, err := b.client.ResolveDID(ctx, did)
	if err != nil {
		return 0, err
	}

	// Ensure actor exists
	if err := b.actorsRepo.Upsert(ctx, data.DID, data.Handle); err != nil {
		slog.Warn("[backfill] Failed to upsert actor", "did", did, "error", err)
	}

	// Fetch records for all collections
	var totalRecords int
	for _, collection := range b.config.Collections {
		records, err := b.client.ListRecords(ctx, data.PDS, data.DID, collection)
		if err != nil {
			slog.Warn("[backfill] Failed to list records for actor",
				"did", did,
				"collection", collection,
				"error", err,
			)
			continue
		}

		for _, rec := range records {
			result, err := b.recordsRepo.Insert(ctx, rec.URI, rec.CID, data.DID, collection, string(rec.Value))
			if err != nil {
				slog.Debug("[backfill] Failed to insert record", "uri", rec.URI, "error", err)
				continue
			}

			if result == repositories.Inserted {
				totalRecords++
			}
		}
	}

	slog.Info("[backfill] Actor backfill complete",
		"did", did,
		"records", totalRecords,
	)

	return totalRecords, nil
}

// ParseCollections parses a comma-separated list of collections.
func ParseCollections(s string) []string {
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
