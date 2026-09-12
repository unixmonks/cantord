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

// suggestionCap bounds how many chips the home screen shows — enough for
// variety without turning the quick-link row into a wall of buttons.
const suggestionCap = 4

// Lookback windows and signal thresholds for each suggestion source. Each
// is tuned independently: a longer-run aggregate (TopGenreByHour) needs
// more history to mean anything than a "same time last few weeks" callback,
// and a "haven't played it in a while" nudge needs a much longer window
// than either.
const (
	historyLookbackDays        = 60 // TopGenreByHour / TopArtistByHour
	historySignalThreshold     = 3
	weeklyCallbackLookbackDays = 28 // TopGenreByWeekdayHour: ~4 occurrences of this weekday
	weeklyCallbackThreshold    = 2
	staleFavoriteDays          = 30 // StaleFavorite
	deepCutMinPlays            = 5  // TopArtistOverall, before offering a deep cut from it
	unplayedRecentDays         = 14 // UnplayedRecentlyAdded
	streakSampleSize           = 8  // RecentPlayStreakGenre
	streakThreshold            = 5  // out of streakSampleSize
)

// Suggestions returns a short, prioritized list of quick links for the
// home screen: a generic time/day-flavored one first (always present),
// then whichever of several personalization and discovery signals — this
// hour's usual listening, a same-time-last-week callback, a recent-run
// "keep the vibe going", a nearly-empty queue, a favorite gone stale, an
// unheard deep cut from a favorite artist, or an unplayed new addition —
// have a strong enough signal behind them, most specific first.
func (s *Service) Suggestions() []Suggestion {
	now := time.Now()
	hours := bucketHours(timeBucket(now.Hour()))

	var out []Suggestion
	seen := map[string]bool{}
	add := func(sug Suggestion, ok bool) bool {
		if !ok || sug.Label == "" || seen[sug.Label] {
			return false
		}
		seen[sug.Label] = true
		out = append(out, sug)
		return true
	}

	add(genericSuggestion(now), true)

	if sug, ok := s.queueRunningLowSuggestion(); add(sug, ok) && len(out) >= suggestionCap {
		return out
	}

	if genre, count, err := s.lib.TopGenreByWeekdayHour(int(now.Weekday()), hours, weeklyCallbackLookbackDays); err == nil {
		ok := genre != "" && count >= weeklyCallbackThreshold
		sug := Suggestion{
			Label:  fmt.Sprintf("More %s", genre),
			Prompt: fmt.Sprintf("play some more %s — it's what I usually listen to around this time on %ss", strings.ToLower(genre), now.Weekday()),
		}
		if add(sug, ok) && len(out) >= suggestionCap {
			return out
		}
	}

	if genre, count, err := s.lib.TopGenreByHour(hours, historyLookbackDays); err == nil {
		ok := genre != "" && count >= historySignalThreshold
		sug := Suggestion{
			Label:  fmt.Sprintf("More %s", genre),
			Prompt: fmt.Sprintf("play some more %s, like I usually listen to around this time", strings.ToLower(genre)),
		}
		if add(sug, ok) && len(out) >= suggestionCap {
			return out
		}
	}

	if artist, count, err := s.lib.TopArtistByHour(hours, historyLookbackDays); err == nil {
		ok := artist != "" && count >= historySignalThreshold
		sug := Suggestion{
			Label:  fmt.Sprintf("More %s", artist),
			Prompt: fmt.Sprintf("play more %s, like I usually listen to around this time", artist),
		}
		if add(sug, ok) && len(out) >= suggestionCap {
			return out
		}
	}

	if genre, count, err := s.lib.RecentPlayStreakGenre(streakSampleSize); err == nil {
		ok := genre != "" && count >= streakThreshold
		sug := Suggestion{
			Label:  "Keep the vibe going",
			Prompt: fmt.Sprintf("play more %s to keep the vibe going", strings.ToLower(genre)),
		}
		if add(sug, ok) && len(out) >= suggestionCap {
			return out
		}
	}

	if track, ok, err := s.lib.StaleFavorite(staleFavoriteDays); err == nil {
		sug := Suggestion{
			Label:  fmt.Sprintf("Revisit %s", track.Artist),
			Prompt: fmt.Sprintf("play %q by %s and similar tracks — I haven't listened to it in a while", track.Title, track.Artist),
		}
		if add(sug, ok) && len(out) >= suggestionCap {
			return out
		}
	}

	if artist, plays, err := s.lib.TopArtistOverall(); err == nil && artist != "" && plays >= deepCutMinPlays {
		if track, ok, err := s.lib.UnplayedTrackByArtist(artist); err == nil {
			sug := Suggestion{
				Label:  fmt.Sprintf("Deep cut: %s", artist),
				Prompt: fmt.Sprintf("play %q by %s — I haven't heard it before", track.Title, artist),
			}
			if add(sug, ok) && len(out) >= suggestionCap {
				return out
			}
		}
	}

	if track, ok, err := s.lib.UnplayedRecentlyAdded(unplayedRecentDays); err == nil {
		sug := Suggestion{
			Label:  fmt.Sprintf("New: %s", track.Title),
			Prompt: fmt.Sprintf("play %q by %s — I just added it and haven't heard it yet", track.Title, track.Artist),
		}
		add(sug, ok)
	}

	return out
}

// queueRunningLowSuggestion offers to keep the music going when playback
// is active but the queue is about to run out and won't loop on its own.
func (s *Service) queueRunningLowSuggestion() (Suggestion, bool) {
	status := s.engine.Status()
	if status.State == "stopped" || status.Repeat == "all" {
		return Suggestion{}, false
	}
	queue := s.engine.Queue()
	if len(queue) == 0 || len(queue)-1-status.QueueIndex > 1 {
		return Suggestion{}, false
	}
	return Suggestion{
		Label:  "Keep it going",
		Prompt: "the queue's almost done — queue up more music like what's currently playing",
	}, true
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
