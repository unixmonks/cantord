package art

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// MusicBrainzProvider is the built-in, no-API-key-required external art
// source: MusicBrainz for release lookup, Cover Art Archive for the image
// itself. Both services ask for a descriptive User-Agent and MusicBrainz
// throttles to ~1 req/sec, so this provider self-rate-limits rather than
// relying on the caller to.
type MusicBrainzProvider struct {
	client    *http.Client
	userAgent string

	mu       sync.Mutex
	lastCall time.Time
}

func NewMusicBrainzProvider(userAgent string) *MusicBrainzProvider {
	return &MusicBrainzProvider{
		client:    &http.Client{Timeout: 15 * time.Second},
		userAgent: userAgent,
	}
}

func (p *MusicBrainzProvider) Name() string         { return "musicbrainz" }
func (p *MusicBrainzProvider) Capabilities() []Kind { return []Kind{KindAlbumArt} }

func (p *MusicBrainzProvider) throttle() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if wait := time.Second - time.Since(p.lastCall); wait > 0 {
		time.Sleep(wait)
	}
	p.lastCall = time.Now()
}

func (p *MusicBrainzProvider) get(ctx context.Context, rawURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", p.userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("%s: status %d", rawURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	return body, resp.Header.Get("Content-Type"), err
}

type mbSearchResult struct {
	Releases []struct {
		ID string `json:"id"`
	} `json:"releases"`
}

func (p *MusicBrainzProvider) FetchAlbumArt(ctx context.Context, req FetchRequest) (FetchResult, error) {
	if req.Artist == "" || req.Album == "" {
		return FetchResult{}, fmt.Errorf("musicbrainz: artist and album required")
	}

	p.throttle()
	query := fmt.Sprintf(`artist:"%s" AND release:"%s"`, req.Artist, req.Album)
	searchURL := "https://musicbrainz.org/ws/2/release/?" + url.Values{
		"query": {query},
		"fmt":   {"json"},
		"limit": {"1"},
	}.Encode()

	body, _, err := p.get(ctx, searchURL)
	if err != nil {
		return FetchResult{}, fmt.Errorf("musicbrainz search: %w", err)
	}
	var parsed mbSearchResult
	if err := json.Unmarshal(body, &parsed); err != nil {
		return FetchResult{}, fmt.Errorf("musicbrainz search response: %w", err)
	}
	if len(parsed.Releases) == 0 {
		return FetchResult{}, nil
	}
	mbid := parsed.Releases[0].ID

	p.throttle()
	imgURL := fmt.Sprintf("https://coverartarchive.org/release/%s/front-500", mbid)
	data, _, err := p.get(ctx, imgURL)
	if err != nil {
		return FetchResult{}, nil // no cover art on file for this release — not an error condition
	}

	return FetchResult{
		Data:       data,
		Confidence: 0.7,
		License:    "Cover Art Archive contributor upload; verify per-image license before redistribution",
	}, nil
}
