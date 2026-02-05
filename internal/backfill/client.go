// Package backfill provides historical data fetching from AT Protocol relays and PDS servers.
package backfill

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	// DefaultRelayURL is the default Bluesky relay.
	DefaultRelayURL = "https://relay1.us-west.bsky.network"

	// DefaultPLCURL is the default PLC directory.
	DefaultPLCURL = "https://plc.directory"

	// DefaultTimeout is the default HTTP timeout.
	DefaultTimeout = 30 * time.Second
)

// Client handles HTTP requests to AT Protocol services.
type Client struct {
	httpClient *http.Client
	relayURL   string
	plcURL     string
}

// NewClient creates a new backfill client.
func NewClient(relayURL, plcURL string) *Client {
	if relayURL == "" {
		relayURL = DefaultRelayURL
	}
	if plcURL == "" {
		plcURL = DefaultPLCURL
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		relayURL: relayURL,
		plcURL:   plcURL,
	}
}

// RepoInfo contains basic repository information.
type RepoInfo struct {
	DID string `json:"did"`
}

// ListReposByCollectionResponse is the response from listReposByCollection.
type ListReposByCollectionResponse struct {
	Repos  []RepoInfo `json:"repos"`
	Cursor string     `json:"cursor,omitempty"`
}

// ListReposByCollection fetches all repos that have records for a collection.
func (c *Client) ListReposByCollection(ctx context.Context, collection string) ([]string, error) {
	var allRepos []string
	var cursor string

	for {
		repos, nextCursor, err := c.listReposByCollectionPage(ctx, collection, cursor)
		if err != nil {
			return nil, err
		}

		allRepos = append(allRepos, repos...)

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return allRepos, nil
}

func (c *Client) listReposByCollectionPage(ctx context.Context, collection, cursor string) ([]string, string, error) {
	u, err := url.Parse(c.relayURL + "/xrpc/com.atproto.sync.listReposByCollection")
	if err != nil {
		return nil, "", err
	}

	q := u.Query()
	q.Set("collection", collection)
	q.Set("limit", "1000")
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result ListReposByCollectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", fmt.Errorf("failed to decode response: %w", err)
	}

	repos := make([]string, len(result.Repos))
	for i, r := range result.Repos {
		repos[i] = r.DID
	}

	return repos, result.Cursor, nil
}

// AtprotoData contains resolved DID information.
type AtprotoData struct {
	DID    string
	Handle string
	PDS    string
}

// ResolveDID resolves a DID to get PDS endpoint and handle.
func (c *Client) ResolveDID(ctx context.Context, did string) (*AtprotoData, error) {
	u := c.plcURL + "/" + did

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var doc PLCDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return doc.ToAtprotoData(did), nil
}

// PLCDocument represents a DID document from PLC directory.
type PLCDocument struct {
	Service     []PLCService `json:"service"`
	AlsoKnownAs []string     `json:"alsoKnownAs"`
}

// PLCService represents a service in the DID document.
type PLCService struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	ServiceEndpoint string `json:"serviceEndpoint"`
}

// ToAtprotoData converts a PLC document to AtprotoData.
func (d *PLCDocument) ToAtprotoData(did string) *AtprotoData {
	data := &AtprotoData{
		DID:    did,
		Handle: did,                   // Default to DID if no handle found
		PDS:    "https://bsky.social", // Default PDS
	}

	// Find AtprotoPersonalDataServer service
	for _, svc := range d.Service {
		if svc.Type == "AtprotoPersonalDataServer" {
			data.PDS = svc.ServiceEndpoint
			break
		}
	}

	// Find handle from alsoKnownAs
	for _, aka := range d.AlsoKnownAs {
		if len(aka) > 5 && aka[:5] == "at://" {
			data.Handle = aka[5:]
			break
		}
	}

	return data
}

// ListRecordsRecord represents a single record from listRecords.
type ListRecordsRecord struct {
	URI   string          `json:"uri"`
	CID   string          `json:"cid"`
	Value json.RawMessage `json:"value"`
}

// ListRecordsResponse is the response from listRecords.
type ListRecordsResponse struct {
	Records []ListRecordsRecord `json:"records"`
	Cursor  string              `json:"cursor,omitempty"`
}

// ListRecords fetches all records for a repo and collection from a PDS.
func (c *Client) ListRecords(ctx context.Context, pdsURL, repo, collection string) ([]ListRecordsRecord, error) {
	var allRecords []ListRecordsRecord
	var cursor string

	for {
		records, nextCursor, err := c.listRecordsPage(ctx, pdsURL, repo, collection, cursor)
		if err != nil {
			return nil, err
		}

		allRecords = append(allRecords, records...)

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return allRecords, nil
}

func (c *Client) listRecordsPage(ctx context.Context, pdsURL, repo, collection, cursor string) ([]ListRecordsRecord, string, error) {
	u, err := url.Parse(pdsURL + "/xrpc/com.atproto.repo.listRecords")
	if err != nil {
		return nil, "", err
	}

	q := u.Query()
	q.Set("repo", repo)
	q.Set("collection", collection)
	q.Set("limit", "100")
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result ListRecordsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Records, result.Cursor, nil
}
