package media

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"sync"
)

// ImageDecoder decodes an image from a byte stream.
type ImageDecoder interface {
	Decode(r io.Reader) (image.Image, error)
}

// ImageEncoder encodes an image into a byte stream with the given quality
// (0–100; decoders that don't use quality ignore the parameter).
type ImageEncoder interface {
	Encode(w io.Writer, img image.Image, quality int) error
}

var (
	decoderMu sync.RWMutex
	decoders  = map[Format]ImageDecoder{}

	encoderMu sync.RWMutex
	encoders  = map[Format]ImageEncoder{}
)

// RegisterDecoder registers an ImageDecoder for the given format.
// It is safe to call from init() functions in multiple files.
func RegisterDecoder(f Format, d ImageDecoder) {
	decoderMu.Lock()
	defer decoderMu.Unlock()
	decoders[f] = d
}

// RegisterEncoder registers an ImageEncoder for the given format.
func RegisterEncoder(f Format, e ImageEncoder) {
	encoderMu.Lock()
	defer encoderMu.Unlock()
	encoders[f] = e
}

// DecodeImage detects the format from the actual byte content, then
// dispatches to the registered decoder for that format.
// Returns the decoded image and the detected format.
func DecodeImage(data []byte) (image.Image, Format, error) {
	f, err := DetectFormat(data)
	if err != nil {
		return nil, "", fmt.Errorf("media: format detection failed: %w", err)
	}

	decoderMu.RLock()
	d, ok := decoders[f]
	decoderMu.RUnlock()

	if !ok {
		return nil, f, fmt.Errorf("media: no decoder registered for format %q", f)
	}

	img, err := d.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, f, fmt.Errorf("media: decode %s: %w", f, err)
	}
	return img, f, nil
}

// EncodeImage encodes img into w using the registered encoder for f.
func EncodeImage(w io.Writer, img image.Image, f Format, quality int) error {
	encoderMu.RLock()
	e, ok := encoders[f]
	encoderMu.RUnlock()

	if !ok {
		return fmt.Errorf("media: no encoder registered for format %q", f)
	}
	return e.Encode(w, img, quality)
}
