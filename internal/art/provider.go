package art

import "context"

type Kind string

const (
	KindAlbumArt Kind = "album_art"
)

type FetchRequest struct {
	Artist string
	Album  string
}

type FetchResult struct {
	Data       []byte
	Confidence float64 // 0..1; Resolve stops at the first result >= its threshold
	Source     string
	License    string
}

// Provider is the plugin boundary for external art/metadata sources. Ship
// this in-process for built-ins (see MusicBrainzProvider); a third-party
// source is expected to run out-of-process behind the same interface (e.g.
// over gRPC via hashicorp/go-plugin) once cantord needs providers it doesn't
// ship itself — Registry doesn't care which.
type Provider interface {
	Name() string
	Capabilities() []Kind
	FetchAlbumArt(ctx context.Context, req FetchRequest) (FetchResult, error)
}

// Registry tries providers in priority order (the order they were
// registered) and returns the first result clearing the confidence
// threshold.
type Registry struct {
	providers []Provider
	threshold float64
}

func NewRegistry(threshold float64) *Registry {
	return &Registry{threshold: threshold}
}

func (r *Registry) Register(p Provider) {
	r.providers = append(r.providers, p)
}

func (r *Registry) Providers() []Provider { return r.providers }

func (r *Registry) ResolveAlbumArt(ctx context.Context, req FetchRequest) (FetchResult, bool) {
	for _, p := range r.providers {
		supported := false
		for _, k := range p.Capabilities() {
			if k == KindAlbumArt {
				supported = true
				break
			}
		}
		if !supported {
			continue
		}
		res, err := p.FetchAlbumArt(ctx, req)
		if err != nil || len(res.Data) == 0 {
			continue
		}
		if res.Confidence >= r.threshold {
			res.Source = p.Name()
			return res, true
		}
	}
	return FetchResult{}, false
}
