package media

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const ffmpegDefaultTimeout = 15 * time.Second

func init() {
	d := &ffmpegISOBMFFDecoder{
		ffmpegPath: "ffmpeg",
		timeout:    ffmpegDefaultTimeout,
	}
	RegisterDecoder(FormatAVIF, d)
	RegisterDecoder(FormatHEIC, d)
}

// ffmpegISOBMFFDecoder decodes AVIF and HEIC images by invoking ffmpeg as a
// subprocess.  The input is written to a temp file and the output JPEG is
// read back, then decoded with the stdlib jpeg decoder.
type ffmpegISOBMFFDecoder struct {
	ffmpegPath string        // defaults to "ffmpeg" on PATH
	timeout    time.Duration // hard cap per invocation
}

func (d *ffmpegISOBMFFDecoder) Decode(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("ffmpegDecoder: read input: %w", err)
	}

	// Use a temp directory so we fully control the filenames fed to ffmpeg —
	// no user-controlled string ever reaches exec.Command arguments.
	tmpDir, err := os.MkdirTemp("", "media-ffmpeg-*")
	if err != nil {
		return nil, fmt.Errorf("ffmpegDecoder: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.bin")
	outPath := filepath.Join(tmpDir, "output.jpg")

	if err := os.WriteFile(inPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("ffmpegDecoder: write temp input: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()

	// Argument list is fully hardcoded with paths we generated; no
	// user-controlled strings are interpolated.
	cmd := exec.CommandContext(ctx,
		d.ffmpegPath,
		"-v", "error",
		"-i", inPath,
		"-frames:v", "1",
		"-update", "1",
		"-f", "image2",
		"-vcodec", "mjpeg",
		"-q:v", "2",
		outPath,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpegDecoder: ffmpeg failed: %w\nffmpeg output: %s", err, out)
	}

	// Log at warn level: this path is expected to be rare (only reached for
	// AVIF/HEIC formats that the stdlib cannot decode natively).
	log.Printf("[media] warn: ffmpeg fallback decoder used for ISOBMFF file (%d bytes)", len(data))

	jpegData, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("ffmpegDecoder: read ffmpeg output: %w", err)
	}

	img, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, fmt.Errorf("ffmpegDecoder: decode ffmpeg JPEG output: %w", err)
	}
	return img, nil
}
