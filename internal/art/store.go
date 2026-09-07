// Package art is the content-addressed album/artist art cache: local
// providers (embedded tags today; external metadata services later, per the
// plugin design) write blobs in, the HTTP API serves them back out with
// long-lived caching since the URL is the hash.
package art

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"

	"github.com/nfnt/resize"
)

const ThumbWidth = 300

type Store struct {
	dir string
}

func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *Store) fullPath(h string) string  { return filepath.Join(s.dir, h+".full") }
func (s *Store) thumbPath(h string) string { return filepath.Join(s.dir, h+".thumb.jpg") }

// Put stores image bytes under their content hash (idempotent) and
// pre-generates a grid-sized thumbnail so list views never pull full-res
// covers. Returns the hash to reference the art by.
func (s *Store) Put(data []byte) (string, error) {
	h := hash(data)
	if _, err := os.Stat(s.fullPath(h)); err == nil {
		return h, nil // already cached
	}
	if err := os.WriteFile(s.fullPath(h), data, 0o644); err != nil {
		return "", fmt.Errorf("writing art blob: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		// Not decodable as an image we support thumbnailing for; keep the
		// full blob (still servable) and skip the thumbnail.
		return h, nil
	}
	thumb := resize.Resize(ThumbWidth, 0, img, resize.Lanczos3)
	f, err := os.Create(s.thumbPath(h))
	if err != nil {
		return h, fmt.Errorf("creating thumbnail: %w", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, thumb, &jpeg.Options{Quality: 85}); err != nil {
		return h, fmt.Errorf("encoding thumbnail: %w", err)
	}
	return h, nil
}

// Open returns the bytes and content-type for a stored art hash. size is
// "thumb" or "full"; "thumb" falls back to "full" if no thumbnail exists
// (e.g. an undecodable original).
func (s *Store) Open(h, size string) ([]byte, string, error) {
	path := s.fullPath(h)
	if size == "thumb" {
		if _, err := os.Stat(s.thumbPath(h)); err == nil {
			path = s.thumbPath(h)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	ct := http.DetectContentType(data)
	return data, ct, nil
}

func (s *Store) Has(h string) bool {
	_, err := os.Stat(s.fullPath(h))
	return err == nil
}
