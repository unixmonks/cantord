// Package config loads cantord's TOML configuration file.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server   ServerConfig   `toml:"server"`
	Library  LibraryConfig  `toml:"library"`
	Playback PlaybackConfig `toml:"playback"`
	AI       AIConfig       `toml:"ai"`
}

type ServerConfig struct {
	Listen string `toml:"listen"`
}

type LibraryConfig struct {
	MusicDirs   []string `toml:"music_dirs"`
	DBPath      string   `toml:"db_path"`
	ArtCacheDir string   `toml:"art_cache_dir"`

	// RescanInterval is a Go duration string (e.g. "15m") for a periodic
	// full rescan that runs regardless of fsnotify activity. fsnotify only
	// sees local filesystem events, so it won't fire for content added by
	// another NFS client, or for a mount that quietly comes back after
	// being unreachable — this is what actually picks those up.
	RescanInterval string `toml:"rescan_interval"`
	// RescanEvery is RescanInterval parsed by Load; use this at runtime.
	RescanEvery time.Duration `toml:"-"`
}

type PlaybackConfig struct {
	MPVPath     string `toml:"mpv_path"`
	AO          string `toml:"ao"`           // optional: force mpv's --ao (e.g. "alsa", "oss", "null")
	AudioDevice string `toml:"audio_device"` // optional: --audio-device, e.g. "alsa/hw:CARD=DAC,DEV=0"
	IPCSocket   string `toml:"ipc_socket"`
}

// AIConfig configures the optional AI assistant (natural-language queue and
// playlist building). The API key is deliberately left out of the config
// file in the default setup — it's read from $CANTORD_AI_API_KEY instead, so
// a plaintext secret doesn't end up sitting in cantord.toml. Setting APIKey
// here still works for anyone who prefers that.
type AIConfig struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
	APIKey   string `toml:"api_key"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{Listen: ":8080"},
		Library: LibraryConfig{
			DBPath:         "~/.local/share/cantord/library.db",
			ArtCacheDir:    "~/.local/share/cantord/art",
			RescanInterval: "15m",
		},
		Playback: PlaybackConfig{
			MPVPath:   "mpv",
			IPCSocket: "~/.local/share/cantord/mpv.sock",
		},
		AI: AIConfig{
			Provider: "anthropic",
			Model:    "claude-opus-5",
		},
	}
}

// Load reads a TOML config file, falling back to defaults for anything unset.
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		if _, err := toml.DecodeFile(path, &cfg); err != nil {
			return cfg, fmt.Errorf("loading config %s: %w", path, err)
		}
	}

	if cfg.AI.APIKey == "" {
		cfg.AI.APIKey = os.Getenv("CANTORD_AI_API_KEY")
	}

	var err error
	if cfg.Library.DBPath, err = expandHome(cfg.Library.DBPath); err != nil {
		return cfg, err
	}
	if cfg.Library.ArtCacheDir, err = expandHome(cfg.Library.ArtCacheDir); err != nil {
		return cfg, err
	}
	if cfg.Playback.IPCSocket, err = expandHome(cfg.Playback.IPCSocket); err != nil {
		return cfg, err
	}
	for i, d := range cfg.Library.MusicDirs {
		if cfg.Library.MusicDirs[i], err = expandHome(d); err != nil {
			return cfg, err
		}
	}

	if len(cfg.Library.MusicDirs) == 0 {
		return cfg, fmt.Errorf("library.music_dirs must list at least one directory")
	}

	if cfg.Library.RescanEvery, err = time.ParseDuration(cfg.Library.RescanInterval); err != nil {
		return cfg, fmt.Errorf("library.rescan_interval %q: %w", cfg.Library.RescanInterval, err)
	}

	for _, dir := range []string{filepath.Dir(cfg.Library.DBPath), cfg.Library.ArtCacheDir, filepath.Dir(cfg.Playback.IPCSocket)} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return cfg, fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	return cfg, nil
}

func expandHome(p string) (string, error) {
	if !strings.HasPrefix(p, "~") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~")), nil
}
