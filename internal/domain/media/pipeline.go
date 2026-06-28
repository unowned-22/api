package media

import (
	"bytes"
	"context"
	"fmt"
)

// VariantSpec describes one output variant to derive from a source image.
type VariantSpec struct {
	// Name is an opaque label returned in ProcessedVariant; use it to
	// identify which variant is which (e.g. "mobile", "desktop").
	Name string

	// Crop is the region to extract before resizing.  nil means no crop.
	Crop *CropRect

	// MaxSize is the maximum dimension (width or height) for the output.
	// 0 means no resize.
	MaxSize int

	// Format is the output encoding format.
	Format Format

	// Quality is the encoder quality (0–100).  0 means encoder default.
	Quality int
}

// ProcessedVariant is the output of a single VariantSpec.
type ProcessedVariant struct {
	Name   string
	Format Format
	Data   []byte
}

// Processor decodes a source image once and derives multiple output
// variants from that single decoded image.
type Processor struct{}

// NewProcessor creates a Processor.
func NewProcessor() *Processor {
	return &Processor{}
}

// Process detects the format of data from its bytes, decodes the image
// exactly once, and then encodes each requested variant.
//
// The ctx is available for future cancellation support; it is currently not
// threaded into encoder calls because all built-in encoders are CPU-bound
// and non-blocking.
func (p *Processor) Process(_ context.Context, data []byte, variants []VariantSpec) ([]ProcessedVariant, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("media: Process called with empty data")
	}

	img, _, err := DecodeImage(data)
	if err != nil {
		return nil, fmt.Errorf("media: Process: %w", err)
	}

	out := make([]ProcessedVariant, 0, len(variants))
	for _, v := range variants {
		cur := img

		if v.Crop != nil {
			cur = Crop(cur, *v.Crop)
		}
		if v.MaxSize > 0 {
			cur = ResizeToFit(cur, v.MaxSize)
		}

		var buf bytes.Buffer
		if err := EncodeImage(&buf, cur, v.Format, v.Quality); err != nil {
			return nil, fmt.Errorf("media: Process variant %q: %w", v.Name, err)
		}
		out = append(out, ProcessedVariant{
			Name:   v.Name,
			Format: v.Format,
			Data:   buf.Bytes(),
		})
	}
	return out, nil
}
