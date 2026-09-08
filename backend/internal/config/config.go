// Package config loads cantord's TOML configuration file.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server   ServerConfig   `toml:"server"`
	Library  LibraryConfig  `toml:"library"`
	Playback PlaybackConfig `toml:"playback"`
}

type ServerConfig struct {
	Listen string `toml:"listen"`
}

type LibraryConfig struct {
	MusicDirs   []string `toml:"music_dirs"`
	DBPath      string   `toml:"db_path"`
	ArtCacheDir string   `toml:"art_cache_dir"`
}

type PlaybackConfig struct {
	MPVPath     string `toml:"mpv_path"`
	AO          string `toml:"ao"`           // optional: force mpv's --ao (e.g. "alsa", "oss", "null")
	AudioDevice string `toml:"audio_device"` // optional: --audio-device, e.g. "alsa/hw:CARD=DAC,DEV=0"
	IPCSocket   string `toml:"ipc_socket"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{Listen: ":8080"},
		Library: LibraryConfig{
			DBPath:      "~/.local/share/cantord/library.db",
			ArtCacheDir: "~/.local/share/cantord/art",
		},
		Playback: PlaybackConfig{
			MPVPath:   "mpv",
			IPCSocket: "~/.local/share/cantord/mpv.sock",
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
