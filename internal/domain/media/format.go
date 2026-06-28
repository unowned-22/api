package media

import (
	"encoding/binary"
	"errors"
	"net/http"
)

// Format identifies a media container/codec format.
type Format string

const (
	FormatJPEG Format = "jpeg"
	FormatPNG  Format = "png"
	FormatWebP Format = "webp"
	FormatGIF  Format = "gif"
	FormatAVIF Format = "avif"
	FormatHEIC Format = "heic"
)

// ErrUnknownFormat is returned when the file bytes do not match any
// supported format signature.
var ErrUnknownFormat = errors.New("media: unknown or unsupported format")

// DetectFormat inspects the actual file bytes — never a client-supplied
// Content-Type header — and returns the real format.
//
// Detection order:
//  1. ISOBMFF ftyp-box brand sniffing for AVIF / HEIC (which
//     http.DetectContentType returns "application/octet-stream" for).
//  2. http.DetectContentType for JPEG / PNG / WebP / GIF.
func DetectFormat(data []byte) (Format, error) {
	if f, ok := detectISOBMFF(data); ok {
		return f, nil
	}

	switch ct := http.DetectContentType(data); ct {
	case "image/jpeg":
		return FormatJPEG, nil
	case "image/png":
		return FormatPNG, nil
	case "image/webp":
		return FormatWebP, nil
	case "image/gif":
		return FormatGIF, nil
	default:
		return "", ErrUnknownFormat
	}
}

// FormatExtension returns the conventional file extension for a Format
// (without the leading dot).
func FormatExtension(f Format) string {
	switch f {
	case FormatJPEG:
		return "jpg"
	case FormatPNG:
		return "png"
	case FormatWebP:
		return "webp"
	case FormatGIF:
		return "gif"
	case FormatAVIF:
		return "avif"
	case FormatHEIC:
		return "heic"
	default:
		return "bin"
	}
}

// detectISOBMFF checks for an ISOBMFF ftyp box at the start of data to
// identify AVIF or HEIC files.  It never panics on short or malformed input.
//
// ISOBMFF layout (all offsets from byte 0):
//
//	[0:4]  box size (big-endian uint32)
//	[4:8]  box type — must be "ftyp"
//	[8:12] major brand (4-byte ASCII)
//	[12:16] minor version (ignored)
//	[16:]  compatible brands, 4 bytes each
func detectISOBMFF(data []byte) (Format, bool) {
	if len(data) < 12 {
		return "", false
	}
	if string(data[4:8]) != "ftyp" {
		return "", false
	}

	majorBrand := string(data[8:12])
	if f, ok := isobmffBrand(majorBrand); ok {
		return f, true
	}

	// Walk compatible brands starting at offset 16.
	boxSize := binary.BigEndian.Uint32(data[0:4])
	end := int(boxSize)
	if end > len(data) {
		end = len(data)
	}
	for i := 16; i+4 <= end; i += 4 {
		if f, ok := isobmffBrand(string(data[i : i+4])); ok {
			return f, true
		}
	}
	return "", false
}

func isobmffBrand(brand string) (Format, bool) {
	switch brand {
	case "avif", "avis":
		return FormatAVIF, true
	case "heic", "heix", "heim", "heis", "hevc", "hevx", "mif1":
		return FormatHEIC, true
	}
	return "", false
}
