package media

import (
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/webp"
)

func init() {
	RegisterDecoder(FormatJPEG, nativeDecoder{jpeg.Decode})
	RegisterDecoder(FormatPNG, nativeDecoder{png.Decode})
	RegisterDecoder(FormatGIF, nativeDecoder{func(r io.Reader) (image.Image, error) {
		return gif.Decode(r)
	}})
	RegisterDecoder(FormatWebP, nativeDecoder{webp.Decode})
}

// nativeDecoder wraps a standard-library decode function.
type nativeDecoder struct {
	fn func(r io.Reader) (image.Image, error)
}

func (d nativeDecoder) Decode(r io.Reader) (image.Image, error) {
	return d.fn(r)
}
