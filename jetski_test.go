package jetski_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/dchill72/jetski"
)

// makePNG creates an in-memory PNG with the given pixel grid.
func makePNG(t *testing.T, pixels [][]color.Gray) []byte {
	t.Helper()
	h := len(pixels)
	w := len(pixels[0])
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y, row := range pixels {
		for x, c := range row {
			img.SetGray(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestConvert_outputDimensions(t *testing.T) {
	// 40×20 image → width 20, height should be ~5 (20 * (20/40) * 0.5 = 5).
	pixels := make([][]color.Gray, 20)
	for i := range pixels {
		pixels[i] = make([]color.Gray, 40)
	}

	data := makePNG(t, pixels)
	result, err := jetski.Convert(bytes.NewReader(data), jetski.Options{Width: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 rows, got %d", len(lines))
	}
	for i, line := range lines {
		if len(line) != 20 {
			t.Errorf("row %d: expected 20 columns, got %d", i, len(line))
		}
	}
}

func TestConvert_blackPixelIsDark(t *testing.T) {
	pixels := [][]color.Gray{{{Y: 0}}}
	data := makePNG(t, pixels)

	result, err := jetski.Convert(bytes.NewReader(data), jetski.Options{Width: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ch := strings.TrimSpace(result)
	if ch == " " || ch == "." || ch == "`" {
		t.Errorf("black pixel mapped to light char %q", ch)
	}
}

func TestConvert_whitePixelIsLight(t *testing.T) {
	pixels := [][]color.Gray{{{Y: 255}}}
	data := makePNG(t, pixels)

	result, err := jetski.Convert(bytes.NewReader(data), jetski.Options{Width: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The last char in DefaultChars is a space; white → lightest character.
	if !strings.Contains(result, " ") {
		t.Errorf("white pixel did not map to space, got %q", result)
	}
}

func TestConvert_customRenderer(t *testing.T) {
	pixels := [][]color.Gray{{{Y: 128}}}
	data := makePNG(t, pixels)

	called := false
	result, err := jetski.Convert(bytes.NewReader(data), jetski.Options{
		Width: 1,
		Renderer: func(r, g, b uint8) string {
			called = true
			return "X"
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("custom renderer was not called")
	}
	if !strings.Contains(result, "X") {
		t.Errorf("expected 'X' in output, got %q", result)
	}
}

func TestConvert_defaultWidth(t *testing.T) {
	pixels := make([][]color.Gray, 10)
	for i := range pixels {
		pixels[i] = make([]color.Gray, 10)
	}
	data := makePNG(t, pixels)

	result, err := jetski.Convert(bytes.NewReader(data), jetski.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if len(lines[0]) != jetski.DefaultWidth {
		t.Errorf("expected default width %d, got %d", jetski.DefaultWidth, len(lines[0]))
	}
}
