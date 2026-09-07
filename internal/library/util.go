package library

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

func hashHex(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// sortKey normalizes an album/artist name for stable alphabetical ordering:
// lowercased, leading "the/a/an" dropped, so sorting and letter-bucketing
// behave the way a listener expects.
func sortKey(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	for _, prefix := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
		}
	}
	return s
}

// letterBucket returns the sidebar letter ("A".."Z" or "#") for a sort key.
func letterBucket(key string) string {
	for _, r := range key {
		if unicode.IsLetter(r) {
			return strings.ToUpper(string(r))
		}
		if unicode.IsDigit(r) {
			return "#"
		}
		break
	}
	return "#"
}
