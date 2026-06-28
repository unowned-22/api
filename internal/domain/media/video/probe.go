package video

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

const probeTimeout = 30 * time.Second

// Metadata holds basic information about a video file returned by ffprobe.
type Metadata struct {
	// Duration in seconds.
	Duration float64
	// Width and Height of the primary video stream in pixels.
	Width, Height int
	// VideoCodec is the codec name of the first video stream (e.g. "h264").
	VideoCodec string
	// AudioCodec is the codec name of the first audio stream (empty if none).
	AudioCodec string
	// Size is the file size in bytes reported by ffprobe.
	Size int64
}

// Probe runs ffprobe on the file at path and returns its metadata.
// The path must be a local filesystem path (not a URL); callers are
// responsible for writing the file before calling Probe.
func Probe(ctx context.Context, path string) (Metadata, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	// All arguments are hardcoded except the final path, which is the caller-
	// controlled local temp path — never a user-supplied string from an HTTP
	// request.
	cmd := exec.CommandContext(ctx,
		"ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)

	out, err := cmd.Output()
	if err != nil {
		return Metadata{}, fmt.Errorf("video: ffprobe failed: %w", err)
	}

	var raw struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			Size     string `json:"size"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return Metadata{}, fmt.Errorf("video: ffprobe JSON parse: %w", err)
	}

	var meta Metadata
	fmt.Sscanf(raw.Format.Duration, "%f", &meta.Duration)
	fmt.Sscanf(raw.Format.Size, "%d", &meta.Size)

	for _, s := range raw.Streams {
		switch s.CodecType {
		case "video":
			if meta.VideoCodec == "" {
				meta.VideoCodec = s.CodecName
				meta.Width = s.Width
				meta.Height = s.Height
			}
		case "audio":
			if meta.AudioCodec == "" {
				meta.AudioCodec = s.CodecName
			}
		}
	}
	return meta, nil
}
