// Command cantordctl is a scriptable CLI client for cantord's HTTP API —
// meant both as a day-to-day control tool and as a runnable reference for
// how a frontend exercises the API (list, play, queue, playlists, scan).
package main

import (
	"fmt"
	"os"
)

type command struct {
	fn    func(*Client, []string) error
	usage string
}

var commands map[string]command

func init() {
	commands = map[string]command{
		"artists":         {cmdArtists, "artists"},
		"albums":          {cmdAlbums, "albums [-letter A] [-cursor C] [-limit N] [-all]"},
		"tracks":          {cmdTracks, "tracks <album_id>"},
		"find":            {cmdFind, "find <query>   (searches album/artist names)"},
		"status":          {cmdStatus, "status"},
		"np":              {cmdStatus, "np             (alias for status)"},
		"queue":           {cmdQueue, "queue"},
		"play":            {cmdPlay, "play [track_id]   (no id = resume; with id = enqueue+play)"},
		"play-album":      {cmdPlayAlbum, "play-album <album_id>   (enqueues every track in order)"},
		"pause":           {cmdSimple("/api/playback/pause"), "pause"},
		"stop":            {cmdSimple("/api/playback/stop"), "stop"},
		"next":            {cmdSimple("/api/playback/next"), "next"},
		"prev":            {cmdSimple("/api/playback/previous"), "prev"},
		"seek":            {cmdSeek, "seek <seconds>"},
		"volume":          {cmdVolume, "volume <0-100>"},
		"clear":           {cmdClear, "clear             (empty the queue)"},
		"remove":          {cmdRemove, "remove <queue_index>"},
		"playidx":         {cmdPlayIndex, "playidx <queue_index>   (jump playback to that queue entry)"},
		"playlists":       {cmdPlaylists, "playlists"},
		"playlist-save":   {cmdPlaylistSave, "playlist-save <name> <track_id...>"},
		"playlist-get":    {cmdPlaylistGet, "playlist-get <name>"},
		"playlist-delete": {cmdPlaylistDelete, "playlist-delete <name>"},
		"scan":            {cmdScan, "scan              (trigger a rescan)"},
		"scan-status":     {cmdScanStatus, "scan-status"},
		"events":          {cmdEvents, "events            (stream SSE updates until interrupted)"},
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "cantordctl [-server URL] <command> [args]")
	fmt.Fprintln(os.Stderr, "\nServer defaults to $CANTORD_ADDR or http://localhost:8080")
	fmt.Fprintln(os.Stderr, "\nCommands:")
	for _, name := range []string{
		"artists", "albums", "tracks", "find",
		"status", "np", "queue",
		"play", "play-album", "pause", "stop", "next", "prev", "seek", "volume",
		"clear", "remove", "playidx",
		"playlists", "playlist-save", "playlist-get", "playlist-delete",
		"scan", "scan-status", "events",
	} {
		fmt.Fprintf(os.Stderr, "  %s\n", commands[name].usage)
	}
}

func main() {
	args := os.Args[1:]

	server := os.Getenv("CANTORD_ADDR")
	if server == "" {
		server = "http://localhost:8080"
	}
	if len(args) >= 2 && args[0] == "-server" {
		server = args[1]
		args = args[2:]
	}

	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	cmd, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		usage()
		os.Exit(1)
	}

	client := NewClient(server)
	if err := cmd.fn(client, args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
