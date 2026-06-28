package media

// WebP output encoding is registered here but deliberately not used by the
// upload pipelines yet (avatar/cover produce JPEG output only).
//
// Enabling WebP output requires a follow-up decision on <picture>/fallback
// support in the frontend before it can be turned on by default.
//
// To activate: pass FormatWebP as the Format in a VariantSpec.

// NOTE: The standard library has no WebP encoder.  When WebP output is
// needed, add a cgo-free encoder (e.g. github.com/chai2010/webp) or an
// ffmpeg-backed encoder following the same pattern as decode_ffmpeg.go,
// then uncomment the init() below and register it.

/*
import (
	"image"
	"io"
)

func init() {
	RegisterEncoder(FormatWebP, webpEncoder{})
}

type webpEncoder struct{}

func (webpEncoder) Encode(w io.Writer, img image.Image, quality int) error {
	// TODO: implement via ffmpeg subprocess or pure-Go encoder library.
	return fmt.Errorf("media: WebP encoder not yet implemented")
}
*/
