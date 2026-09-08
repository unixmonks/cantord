package library

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

// StreamInfo carries the audio characteristics ffprobe reports — the source
// of truth for hi-res facts (sample rate, bit depth, channels) that tag
// readers don't expose.
type StreamInfo struct {
	Codec      string
	SampleRate int
	BitDepth   int
	Channels   int
	DurationMS int
}

type ffprobeOutput struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType        string `json:"codec_type"`
		CodecName        string `json:"codec_name"`
		SampleRate       string `json:"sample_rate"`
		Channels         int    `json:"channels"`
		BitsPerRawSample string `json:"bits_per_raw_sample"`
		BitsPerSample    int    `json:"bits_per_sample"`
	} `json:"streams"`
}

// Probe runs ffprobe against an audio file and extracts stream-level facts.
func Probe(ctx context.Context, ffprobePath, path string) (StreamInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return StreamInfo{}, fmt.Errorf("ffprobe %s: %w", path, err)
	}

	var parsed ffprobeOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return StreamInfo{}, fmt.Errorf("ffprobe output %s: %w", path, err)
	}

	var info StreamInfo
	for _, s := range parsed.Streams {
		if s.CodecType != "audio" {
			continue
		}
		info.Codec = s.CodecName
		info.Channels = s.Channels
		if rate, err := strconv.Atoi(s.SampleRate); err == nil {
			info.SampleRate = rate
		}
		if bits, err := strconv.Atoi(s.BitsPerRawSample); err == nil {
			info.BitDepth = bits
		} else if s.BitsPerSample > 0 {
			info.BitDepth = s.BitsPerSample
		}
		break
	}
	if seconds, err := strconv.ParseFloat(parsed.Format.Duration, 64); err == nil {
		info.DurationMS = int(seconds * 1000)
	}
	return info, nil
}
