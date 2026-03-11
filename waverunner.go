package jetski

import (
	"fmt"
	"math"
	"strings"
)

// WaveOptions controls the RGB sine-wave gradient applied by Colorize.
type WaveOptions struct {
	// Angle is the direction the wave travels, in degrees.
	// 0 = horizontal (left→right), 90 = vertical (top→bottom).
	Angle float64

	// Phase shifts the entire gradient cycle, in radians.
	// Useful for animating: increment Phase each frame.
	Phase float64

	// Period is the wavelength of the gradient in character cells.
	// Smaller values produce tighter color bands; larger values a slow sweep.
	// Defaults to 16 when zero.
	Period float64

	// Range controls color saturation as an amplitude in [0, 1].
	// 0 = gray (no color), 1 = fully saturated. Defaults to 1 when zero.
	Range float64
}

// Colorize wraps every character in grid with a 24-bit ANSI foreground color
// driven by three sine waves (120° apart) projected along the wave direction.
// The result is a terminal-printable string with embedded escape sequences.
//
// grid is typically obtained from ConvertGrid.
func Colorize(grid [][]byte, opts WaveOptions) string {
	period := opts.Period
	if period == 0 {
		period = 16
	}
	sat := opts.Range
	if sat == 0 {
		sat = 1
	}

	angle := opts.Angle * math.Pi / 180
	cosA := math.Cos(angle)
	sinA := math.Sin(angle)
	amplitude := sat * 127.0
	center := 128.0

	// Pre-compute the two-thirds-pi offset once.
	const tau23 = 2 * math.Pi / 3

	var sb strings.Builder
	for row, rowBytes := range grid {
		for col, ch := range rowBytes {
			// Project cell position onto the wave direction.
			d := float64(col)*cosA + float64(row)*sinA
			phi := 2*math.Pi*d/period + opts.Phase

			r := clampByte(center + amplitude*math.Sin(phi))
			g := clampByte(center + amplitude*math.Sin(phi+tau23))
			b := clampByte(center + amplitude*math.Sin(phi+2*tau23))

			fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm%c\x1b[0m", r, g, b, ch)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func clampByte(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// GridToString converts a raw char grid to a plain (uncolored) string.
// This is a convenience inverse of ConvertGrid for callers who want to
// switch between plain and colored output without re-decoding the image.
func GridToString(grid [][]byte) string {
	var sb strings.Builder
	for _, row := range grid {
		sb.Write(row)
		sb.WriteByte('\n')
	}
	return sb.String()
}
