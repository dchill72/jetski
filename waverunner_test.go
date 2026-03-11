package jetski

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
)

// ansiRE matches a single colored character: ESC[38;2;R;G;Bm<char>ESC[0m
var ansiRE = regexp.MustCompile(`\x1b\[38;2;(\d+);(\d+);(\d+)m(.)\x1b\[0m`)

// parseColorized extracts all (r,g,b,char) tuples from a Colorize output string.
func parseColorized(s string) []struct{ r, g, b uint8; ch byte } {
	matches := ansiRE.FindAllStringSubmatch(s, -1)
	out := make([]struct{ r, g, b uint8; ch byte }, 0, len(matches))
	for _, m := range matches {
		var r, g, b int
		fmt.Sscan(m[1], &r)
		fmt.Sscan(m[2], &g)
		fmt.Sscan(m[3], &b)
		out = append(out, struct {
			r, g, b uint8
			ch      byte
		}{uint8(r), uint8(g), uint8(b), m[4][0]})
	}
	return out
}

func makeGrid(rows, cols int, ch byte) [][]byte {
	grid := make([][]byte, rows)
	for i := range grid {
		grid[i] = make([]byte, cols)
		for j := range grid[i] {
			grid[i][j] = ch
		}
	}
	return grid
}

// TestColorize_outputStructure checks that every cell produces exactly one
// ANSI-colored character and every row ends with a newline.
func TestColorize_outputStructure(t *testing.T) {
	grid := makeGrid(3, 5, '#')
	out := Colorize(grid, WaveOptions{Period: 10})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	cells := parseColorized(out)
	if len(cells) != 15 {
		t.Fatalf("expected 15 colored cells, got %d", len(cells))
	}
	for _, c := range cells {
		if c.ch != '#' {
			t.Errorf("expected char '#', got %q", c.ch)
		}
	}
}

// TestColorize_preservesChars verifies the source character is passed through
// unchanged regardless of the color applied.
func TestColorize_preservesChars(t *testing.T) {
	chars := []byte("@#*+- ")
	grid := [][]byte{chars}
	cells := parseColorized(Colorize(grid, WaveOptions{Period: 100}))

	if len(cells) != len(chars) {
		t.Fatalf("expected %d cells, got %d", len(chars), len(cells))
	}
	for i, c := range cells {
		if c.ch != chars[i] {
			t.Errorf("cell %d: expected %q got %q", i, chars[i], c.ch)
		}
	}
}

// TestColorize_defaultsZeroOptions checks that zero-value Period and Range are
// replaced with their documented defaults (16 and 1.0) and produce valid output.
func TestColorize_defaultsZeroOptions(t *testing.T) {
	grid := makeGrid(1, 4, '*')
	out := Colorize(grid, WaveOptions{}) // all-zero opts
	cells := parseColorized(out)
	if len(cells) != 4 {
		t.Fatalf("expected 4 cells, got %d", len(cells))
	}
	// With default sat=1 the channels should not all be 128 (pure center).
	allCenter := true
	for _, c := range cells {
		if c.r != 128 || c.g != 128 || c.b != 128 {
			allCenter = false
			break
		}
	}
	if allCenter {
		t.Error("all cells are center gray; amplitude may not be applied")
	}
}

// TestColorize_zeroSaturation verifies that Range=0 defaults to 1, not gray.
// (A deliberate design choice: 0 is treated as "unset".)
func TestColorize_zeroSaturationDefaultsToOne(t *testing.T) {
	grid := makeGrid(1, 1, 'X')
	out := Colorize(grid, WaveOptions{Period: 16, Range: 0})
	cells := parseColorized(out)
	if len(cells) != 1 {
		t.Fatalf("expected 1 cell")
	}
	c := cells[0]
	// With full saturation and phase=0 the channels are not all equal to 128.
	if c.r == 128 && c.g == 128 && c.b == 128 {
		t.Error("Range=0 produced flat gray; expected default saturation of 1")
	}
}

// TestColorize_channelPhaseOffset verifies that R, G, B are offset by 120°
// from each other. At phase=0, col=0, row=0: phi=0.
//   R = 128 + 127·sin(0)        = 128
//   G = 128 + 127·sin(2π/3)    ≈ 238
//   B = 128 + 127·sin(4π/3)    ≈  18
func TestColorize_channelPhaseOffset(t *testing.T) {
	grid := makeGrid(1, 1, 'A')
	// period large enough that phi≈0 at col=0
	out := Colorize(grid, WaveOptions{Period: 1e9, Phase: 0, Range: 1})
	cells := parseColorized(out)
	if len(cells) != 1 {
		t.Fatalf("expected 1 cell")
	}
	c := cells[0]

	// clampByte truncates (uint8 cast), so match that behaviour here.
	wantR := clampByte(128 + 127*math.Sin(0))
	wantG := clampByte(128 + 127*math.Sin(2*math.Pi/3))
	wantB := clampByte(128 + 127*math.Sin(4*math.Pi/3))

	if c.r != wantR {
		t.Errorf("R: got %d want %d", c.r, wantR)
	}
	if c.g != wantG {
		t.Errorf("G: got %d want %d", c.g, wantG)
	}
	if c.b != wantB {
		t.Errorf("B: got %d want %d", c.b, wantB)
	}
}

// TestColorize_phaseShift verifies that opts.Phase shifts the cycle uniformly.
// Shifting by 2π should produce the same colors as phase=0.
func TestColorize_phaseShiftFullCycle(t *testing.T) {
	grid := makeGrid(2, 4, '+')
	opts := WaveOptions{Period: 8, Range: 0.8}

	base := parseColorized(Colorize(grid, opts))
	opts.Phase = 2 * math.Pi
	shifted := parseColorized(Colorize(grid, opts))

	if len(base) != len(shifted) {
		t.Fatalf("cell count mismatch")
	}
	// sin(φ + 2π) is not bit-identical to sin(φ) in floating point, so allow
	// a ±1 LSB tolerance in each channel after uint8 truncation.
	diff := func(a, b uint8) uint8 {
		if a > b {
			return a - b
		}
		return b - a
	}
	for i := range base {
		if diff(base[i].r, shifted[i].r) > 1 ||
			diff(base[i].g, shifted[i].g) > 1 ||
			diff(base[i].b, shifted[i].b) > 1 {
			t.Errorf("cell %d: phase+2π gave different color (%d,%d,%d) vs (%d,%d,%d)",
				i, shifted[i].r, shifted[i].g, shifted[i].b,
				base[i].r, base[i].g, base[i].b)
		}
	}
}

// TestColorize_horizontalAngle checks that angle=0 produces identical colors
// within a column (all rows at same column have same phi).
func TestColorize_horizontalAngle(t *testing.T) {
	grid := makeGrid(4, 6, '-')
	cells := parseColorized(Colorize(grid, WaveOptions{Angle: 0, Period: 8}))
	// cells are row-major; col stride = 6
	for col := 0; col < 6; col++ {
		ref := cells[col] // row 0
		for row := 1; row < 4; row++ {
			c := cells[row*6+col]
			if c.r != ref.r || c.g != ref.g || c.b != ref.b {
				t.Errorf("angle=0 col %d row %d: color differs from row 0", col, row)
			}
		}
	}
}

// TestColorize_verticalAngle checks that angle=90 produces identical colors
// within a row (all cols at same row have same phi).
func TestColorize_verticalAngle(t *testing.T) {
	grid := makeGrid(4, 6, '-')
	cells := parseColorized(Colorize(grid, WaveOptions{Angle: 90, Period: 8}))
	for row := 0; row < 4; row++ {
		ref := cells[row*6]
		for col := 1; col < 6; col++ {
			c := cells[row*6+col]
			if c.r != ref.r || c.g != ref.g || c.b != ref.b {
				t.Errorf("angle=90 row %d col %d: color differs from col 0", row, col)
			}
		}
	}
}

// TestColorize_emptyGrid verifies an empty input produces an empty string.
func TestColorize_emptyGrid(t *testing.T) {
	if out := Colorize([][]byte{}, WaveOptions{}); out != "" {
		t.Errorf("expected empty string, got %q", out)
	}
}

// TestGridToString_roundtrip verifies GridToString produces the plain text
// equivalent (same chars, newline-terminated rows, no ANSI codes).
func TestGridToString_roundtrip(t *testing.T) {
	grid := [][]byte{
		[]byte("@#*"),
		[]byte("+- "),
	}
	want := "@#*\n+- \n"
	if got := GridToString(grid); got != want {
		t.Errorf("GridToString: got %q want %q", got, want)
	}
}
