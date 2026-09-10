package main

// itemsLoadedMsg carries a fully-loaded item set for a screen (everything
// except the paginated Albums root list and the Queue, which have their own
// message types below because they carry extra bookkeeping).
type itemsLoadedMsg struct {
	screenID int
	items    []item
	err      error
}

type albumsPageLoadedMsg struct {
	screenID   int
	items      []item
	nextCursor string
	append     bool // true when this extends an already-loaded page
	err        error
}

type queueLoadedMsg struct {
	screenID     int
	items        []item
	playingIndex int
	err          error
}

// playerStatusMsg is emitted on every "status" SSE tick (about once a
// second) and on startup, and drives the persistent footer.
type playerStatusMsg Status

// sseMsg wraps one raw server-sent event; decoding happens in Update so a
// reconnect or an event type we don't care about never has to touch the
// model's fields directly.
type sseMsg sseEvent

// connLostMsg marks the footer's connection indicator as disconnected; it
// flips back on its own once the daemon's SSE "status" priming event lands
// after a successful reconnect (see handleSSE).
type connLostMsg struct{}

// actionResultMsg reports the outcome of a fire-and-forget action (enqueue,
// remove, playlist edit, transport control) so it can be surfaced as a
// transient status message on the active screen's list.
type actionResultMsg struct {
	text         string
	err          error
	refreshQueue bool
}
