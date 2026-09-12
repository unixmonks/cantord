package ai

import (
	"fmt"
	"strings"
	"time"
)

// Suggestion is a canned prompt the home screen can offer as a one-click
// shortcut into the assistant — clicking it runs Chat with Prompt exactly
// as if the user had typed it.
type Suggestion struct {
	Label  string `json:"label"`
	Prompt string `json:"prompt"`
}

// historyLookbackDays bounds how far back a listening-history signal can
// come from — recent enough that "what you usually play at this hour"
// still reflects current taste, not a phase from a year ago.
const historyLookbackDays = 60

// historySignalThreshold is the minimum play count within the time bucket
// before a personalized suggestion is offered. Below this, a couple of
// plays would be presented as if they were a habit.
const historySignalThreshold = 3

// Suggestions returns a short, time-of-day-aware list of quick links for
// the home screen: always one generic suggestion for the current time and
// day, plus (when there's a clear enough signal in play history) one or
// two personalized to what this listener actually plays around this hour.
func (s *Service) Suggestions() []Suggestion {
	now := time.Now()
	out := []Suggestion{genericSuggestion(now)}

	hours := bucketHours(timeBucket(now.Hour()))
	if genre, count, err := s.lib.TopGenreByHour(hours, historyLookbackDays); err == nil && genre != "" && count >= historySignalThreshold {
		out = append(out, Suggestion{
			Label:  fmt.Sprintf("More %s", genre),
			Prompt: fmt.Sprintf("play some more %s, like I usually listen to around this time", strings.ToLower(genre)),
		})
	}
	if artist, count, err := s.lib.TopArtistByHour(hours, historyLookbackDays); err == nil && artist != "" && count >= historySignalThreshold {
		out = append(out, Suggestion{
			Label:  fmt.Sprintf("More %s", artist),
			Prompt: fmt.Sprintf("play more %s, like I usually listen to around this time", artist),
		})
	}
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

// genericSuggestion picks a canned time/day-flavored prompt that needs no
// listening history at all, so there's always something to show.
func genericSuggestion(now time.Time) Suggestion {
	weekday := now.Weekday()
	hour := now.Hour()

	switch {
	case weekday == time.Friday && hour >= 17:
		return Suggestion{Label: "Friday night", Prompt: "play something for a Friday night"}
	case weekday == time.Saturday && hour >= 17:
		return Suggestion{Label: "Saturday night", Prompt: "play some Saturday night party music"}
	case weekday == time.Sunday && hour < 12:
		return Suggestion{Label: "Lazy Sunday", Prompt: "play something for a lazy Sunday morning"}
	}

	switch timeBucket(hour) {
	case "morning":
		return Suggestion{Label: "Morning chillout", Prompt: "play some morning chillout"}
	case "midday":
		return Suggestion{Label: "Midday pick-me-up", Prompt: "play something upbeat for the middle of the day"}
	case "afternoon":
		return Suggestion{Label: "Afternoon focus", Prompt: "play some focus music for the afternoon"}
	case "evening":
		return Suggestion{Label: "Evening wind-down", Prompt: "play something to wind down this evening"}
	default:
		return Suggestion{Label: "Late night", Prompt: "play something for late at night"}
	}
}

// timeBucket names the part of the day an hour (0-23) falls in.
func timeBucket(hour int) string {
	switch {
	case hour >= 5 && hour < 11:
		return "morning"
	case hour >= 11 && hour < 14:
		return "midday"
	case hour >= 14 && hour < 18:
		return "afternoon"
	case hour >= 18 && hour < 22:
		return "evening"
	default:
		return "late_night"
	}
}

// bucketHours is the inverse of timeBucket: the hours that make up a
// bucket, for querying play history.
func bucketHours(bucket string) []int {
	switch bucket {
	case "morning":
		return []int{5, 6, 7, 8, 9, 10}
	case "midday":
		return []int{11, 12, 13}
	case "afternoon":
		return []int{14, 15, 16, 17}
	case "evening":
		return []int{18, 19, 20, 21}
	default:
		return []int{22, 23, 0, 1, 2, 3, 4}
	}
}
