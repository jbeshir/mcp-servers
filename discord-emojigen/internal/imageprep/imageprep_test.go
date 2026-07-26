package imageprep

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestPrepareProducesDiscordSizedPNG(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 320, 200))
	for y := range 200 {
		for x := range 320 {
			source.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 100, A: 255})
		}
	}
	var input bytes.Buffer
	if err := png.Encode(&input, source); err != nil {
		t.Fatal(err)
	}

	output, err := Prepare(input.Bytes(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(output) > maxUploadBytes {
		t.Fatalf("output is %d bytes", len(output))
	}
	decoded, err := png.Decode(bytes.NewReader(output))
	if err != nil {
		t.Fatal(err)
	}
	if got := decoded.Bounds().Size(); got != (image.Point{X: 128, Y: 128}) {
		t.Fatalf("unexpected output size: %v", got)
	}
}

func TestPrepareRejectsNonImage(t *testing.T) {
	if _, err := Prepare([]byte("not an image"), ""); err == nil {
		t.Fatal("expected error")
	}
}
