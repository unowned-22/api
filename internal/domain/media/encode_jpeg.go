package media

import (
	"image"
	"image/jpeg"
	"io"
)

func init() {
	RegisterEncoder(FormatJPEG, jpegEncoder{})
}

type jpegEncoder struct{}

func (jpegEncoder) Encode(w io.Writer, img image.Image, quality int) error {
	if quality <= 0 || quality > 100 {
		quality = 85 // sensible default
	}
	return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
}
