package util

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// ResizeImage decodes an image, rescales it to a maximum width of maxWidth
// (retaining aspect ratio), and encodes the result as an optimized JPEG.
func ResizeImage(srcData []byte, maxWidth int) ([]byte, error) {
	// Decode original image
	srcImg, _, err := image.Decode(bytes.NewReader(srcData))
	if err != nil {
		return nil, err
	}

	bounds := srcImg.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width <= 0 || height <= 0 {
		return nil, errors.New("invalid image dimensions")
	}

	// Calculate target dimensions
	targetWidth := width
	targetHeight := height
	if width > maxWidth {
		targetWidth = maxWidth
		targetHeight = int(float64(height) * (float64(maxWidth) / float64(width)))
	}

	// Create canvas for scaled image
	destImg := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))

	// Draw / scale using Bilinear scaling algorithm
	draw.BiLinear.Scale(destImg, destImg.Bounds(), srcImg, bounds, draw.Over, nil)

	// Encode to JPEG format with 80% compression quality
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, destImg, &jpeg.Options{Quality: 80})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DetectImageType examines magic bytes to detect mime type (jpeg, png, webp, gif).
func DetectImageType(data []byte) string {
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return "image/png"
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return "image/gif"
	}
	return "image/jpeg" // Fallback
}
