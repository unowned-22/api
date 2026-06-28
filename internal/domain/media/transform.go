package media

import (
	"image"

	"github.com/disintegration/imaging"
)

// CropRect describes a rectangular region in natural pixel coordinates.
type CropRect struct {
	X, Y, Width, Height int
}

// Crop extracts the given rectangle from img.  It clamps the rect to the
// actual image bounds so that out-of-range values (from rounding on the
// client side) never produce a panic.
func Crop(img image.Image, rect CropRect) image.Image {
	bounds := img.Bounds()

	x := rect.X
	y := rect.Y
	w := rect.Width
	h := rect.Height

	if x < bounds.Min.X {
		x = bounds.Min.X
	}
	if y < bounds.Min.Y {
		y = bounds.Min.Y
	}
	if x+w > bounds.Max.X {
		w = bounds.Max.X - x
	}
	if y+h > bounds.Max.Y {
		h = bounds.Max.Y - y
	}

	return imaging.Crop(img, image.Rect(x, y, x+w, y+h))
}

// ResizeToFit scales img down so that neither dimension exceeds maxDimension,
// preserving aspect ratio via Lanczos resampling.  If the image already fits,
// it is returned unchanged.
func ResizeToFit(img image.Image, maxDimension int) image.Image {
	b := img.Bounds()
	if b.Dx() <= maxDimension && b.Dy() <= maxDimension {
		return img
	}
	return imaging.Fit(img, maxDimension, maxDimension, imaging.Lanczos)
}
