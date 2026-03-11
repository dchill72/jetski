// Package jetski converts images (PNG, JPEG, BMP) into ASCII art.
package jetski

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"strings"

	_ "golang.org/x/image/bmp"
)

// DefaultChars is the luminance ramp from dark to light.
// 12 characters chosen so each step has a clearly distinct visual weight.
//
//	@ # $ * + = ~ - : . ` (space)
//	█ ▓ ▒ ▒ ░ ░ ░ ░
const DefaultChars = "@#$*+=~-:." + "` "

// DefaultWidth is the fallback column width when Options.Width is zero.
const DefaultWidth = 80

// fontAspect corrects for the fact that terminal characters are roughly twice
// as tall as they are wide, so the rendered height must be halved.
const fontAspect = 0.5

type pixel struct{ r, g, b uint8 }

// Renderer converts a single pixel (r, g, b components 0–255) into the
// string fragment that represents it in the output. For grayscale output the
// default renderer ignores colour; a colour-aware renderer can emit ANSI
// escape sequences around the chosen character without changing any other part
// of the pipeline.
type Renderer func(r, g, b uint8) string

// Options controls the conversion.
type Options struct {
	// Width is the target number of columns in the output. Height is derived
	// automatically to preserve the image's aspect ratio. Defaults to
	// DefaultWidth when zero.
	Width int

	// Chars is the luminance ramp used by the default grayscale renderer.
	// Index 0 is the darkest character; the last index is the lightest.
	// Ignored when Renderer is set. Defaults to DefaultChars when empty.
	Chars string

	// Renderer overrides the per-pixel rendering function. When nil the
	// package uses a grayscale renderer built from Chars.
	//
	// Note: Gamma and Equalize are applied before this function is called,
	// but only affect the luminance used for character selection by the
	// default renderer. Custom renderers receive the original r, g, b values.
	Renderer Renderer

	// Gamma applies a power-curve to the luminance before character selection.
	// Values less than 1 brighten the output; values greater than 1 darken it.
	// 0 and 1 are both treated as identity (no adjustment).
	// Only applies to the default grayscale renderer.
	Gamma float64

	// Equalize applies histogram equalization to the luminance distribution
	// across all output pixels before character selection. This redistributes
	// the character ramp evenly across the actual tonal range of the image,
	// improving contrast when brightness is concentrated in a narrow band.
	// Only applies to the default grayscale renderer.
	Equalize bool

	// Brightness shifts the overall luminance of the output.
	// 0.5 is neutral (no change); 0.0 is darkest; 1.0 is lightest.
	// Zero value is treated as 0.5 (no adjustment).
	// Only applies to the default grayscale renderer.
	Brightness float64
}

// Convert reads an image from r and returns an ASCII-art string.
// Supported formats: PNG, JPEG, BMP.
func Convert(r io.Reader, opts Options) (string, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return "", err
	}

	chars, samples, charFor := prepare(img, opts)
	renderer := opts.Renderer

	width := len(charFor[0])
	height := len(charFor)
	var sb strings.Builder
	sb.Grow(height * (width + 1))
	for row, rowPx := range samples {
		for col, px := range rowPx {
			if renderer != nil {
				sb.WriteString(renderer(px.r, px.g, px.b))
			} else {
				_ = chars // used inside buildCharSelector; silence unused warning
				sb.WriteByte(charFor[row][col])
			}
		}
		sb.WriteByte('\n')
	}

	return sb.String(), nil
}

// ConvertGrid reads an image from r and returns the ASCII-art as a 2-D byte
// grid. Each cell contains the character chosen for that pixel position.
// The grid can be passed to Colorize to apply an RGB gradient.
// Supported formats: PNG, JPEG, BMP.
func ConvertGrid(r io.Reader, opts Options) ([][]byte, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}
	_, _, grid := prepare(img, opts)
	return grid, nil
}

// prepare decodes common options, samples the image, and builds the char grid.
func prepare(img image.Image, opts Options) (chars string, samples [][]pixel, grid [][]byte) {
	width := opts.Width
	if width <= 0 {
		width = DefaultWidth
	}

	chars = opts.Chars
	if chars == "" {
		chars = DefaultChars
	}

	bounds := img.Bounds()
	srcW := bounds.Max.X - bounds.Min.X
	srcH := bounds.Max.Y - bounds.Min.Y

	height := int(float64(width) * float64(srcH) / float64(srcW) * fontAspect)
	if height < 1 {
		height = 1
	}

	samples = make([][]pixel, height)
	for row := range samples {
		samples[row] = make([]pixel, width)
		for col := range samples[row] {
			srcX := bounds.Min.X + col*srcW/width
			srcY := bounds.Min.Y + row*srcH/height
			r32, g32, b32, _ := img.At(srcX, srcY).RGBA()
			samples[row][col] = pixel{uint8(r32 >> 8), uint8(g32 >> 8), uint8(b32 >> 8)}
		}
	}

	grid = buildCharSelector(samples, chars, opts)
	return
}

// buildCharSelector returns a 2-D grid of pre-selected characters for the
// default grayscale renderer, applying Equalize and Gamma as requested.
func buildCharSelector(samples [][]pixel, chars string, opts Options) [][]byte {
	height := len(samples)
	width := 0
	if height > 0 {
		width = len(samples[0])
	}

	// Compute raw lumas.
	lumas := make([][]float64, height)
	for row, rowPx := range samples {
		lumas[row] = make([]float64, width)
		for col, px := range rowPx {
			lumas[row][col] = luma(px.r, px.g, px.b)
		}
	}

	if opts.Equalize {
		equalizeInPlace(lumas)
	}

	gamma := opts.Gamma
	if gamma != 0 && gamma != 1.0 {
		for row := range lumas {
			for col := range lumas[row] {
				lumas[row][col] = 255 * math.Pow(lumas[row][col]/255, gamma)
			}
		}
	}

	brightness := opts.Brightness
	if brightness == 0 {
		brightness = 0.5
	}
	if brightness != 0.5 {
		offset := (brightness - 0.5) * 255
		for row := range lumas {
			for col := range lumas[row] {
				lumas[row][col] = math.Max(0, math.Min(255, lumas[row][col]+offset))
			}
		}
	}

	n := len(chars)
	out := make([][]byte, height)
	for row := range lumas {
		out[row] = make([]byte, width)
		for col, l := range lumas[row] {
			idx := int(l) * (n - 1) / 255
			out[row][col] = chars[idx]
		}
	}
	return out
}

// luma computes BT.601 luminance in [0, 255].
func luma(r, g, b uint8) float64 {
	return 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
}

// equalizeInPlace applies CDF-based histogram equalization to a 2-D luma grid.
func equalizeInPlace(lumas [][]float64) {
	var hist [256]int
	total := 0
	for _, row := range lumas {
		for _, l := range row {
			hist[int(l)]++
			total++
		}
	}
	if total == 0 {
		return
	}

	// Find the first non-zero CDF value (cdfMin).
	cdfMin := 0
	for _, h := range hist {
		if h > 0 {
			cdfMin = h
			break
		}
	}

	// Build look-up table: luma → equalized luma.
	var lut [256]float64
	cdf := 0
	denom := float64(total - cdfMin)
	for i, h := range hist {
		cdf += h
		if denom > 0 {
			lut[i] = math.Round(float64(cdf-cdfMin) / denom * 255)
		}
	}

	for row := range lumas {
		for col := range lumas[row] {
			lumas[row][col] = lut[int(lumas[row][col])]
		}
	}
}
