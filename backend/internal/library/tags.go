package library

import (
	"fmt"
	"os"

	"github.com/dhowden/tag"
)

// TagInfo is the subset of embedded metadata we care about, plus the raw
// embedded-art bytes if present (used by the "embedded" art provider).
type TagInfo struct {
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Genre       string
	Year        int
	TrackNo     int
	DiscNo      int
	ArtBytes    []byte
	ArtMIME     string
}

func ReadTags(path string) (TagInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return TagInfo{}, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return TagInfo{}, fmt.Errorf("reading tags %s: %w", path, err)
	}

	info := TagInfo{
		Title:       m.Title(),
		Artist:      m.Artist(),
		Album:       m.Album(),
		AlbumArtist: m.AlbumArtist(),
		Genre:       m.Genre(),
		Year:        m.Year(),
	}
	info.TrackNo, _ = m.Track()
	info.DiscNo, _ = m.Disc()
	if pic := m.Picture(); pic != nil {
		info.ArtBytes = pic.Data
		info.ArtMIME = pic.MIMEType
	}
	return info, nil
}
