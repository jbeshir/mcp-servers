package imageprep

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"testing"
)

func TestPrepareSupportedFormats(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 64, 96))
	for y := range 96 {
		for x := range 64 {
			source.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 3), G: uint8(y * 2), A: 255})
		}
	}
	var jpegData bytes.Buffer
	if err := jpeg.Encode(&jpegData, source, nil); err != nil {
		t.Fatal(err)
	}
	var gifData bytes.Buffer
	if err := gif.Encode(&gifData, source, nil); err != nil {
		t.Fatal(err)
	}
	for name, input := range map[string][]byte{"jpeg": jpegData.Bytes(), "gif": gifData.Bytes()} {
		t.Run(name, func(t *testing.T) {
			output, err := Prepare(input, "")
			if err != nil {
				t.Fatal(err)
			}
			if len(output) == 0 || len(output) > maxUploadBytes {
				t.Fatalf("prepared size %d violates Discord limit", len(output))
			}
		})
	}
}

func TestPrepareRejectsOverflowScaleDimensionsBeforeDecode(t *testing.T) {
	tests := map[string][]byte{
		"png": oversizedPNGHeader(),
		"gif": {
			'G', 'I', 'F', '8', '9', 'a',
			0xff, 0xff, 0xff, 0xff,
			0, 0, 0,
		},
		"jpeg": {
			0xff, 0xd8,
			0xff, 0xc0, 0, 17, 8,
			0xff, 0xff, 0xff, 0xff,
			3, 1, 0x11, 0, 2, 0x11, 0, 3, 0x11, 0,
			0xff, 0xda, 0, 12,
			3, 1, 0, 2, 0x11, 3, 0x11, 0, 0x3f, 0,
			0xff, 0xd9,
		},
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("crafted header must pass configuration decode: %v", err)
			}
			if cfg.Width <= 0 || cfg.Height <= 0 {
				t.Fatalf("unexpected dimensions: %v", cfg)
			}
			if _, err := Prepare(data, ""); err == nil || err.Error() != "image dimensions exceed limits" {
				t.Fatalf("expected dimension rejection before full decode, got %v", err)
			}
		})
	}
}

func oversizedPNGHeader() []byte {
	data := []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0, 0, 0, 13, 'I', 'H', 'D', 'R',
		0, 0, 0xff, 0xff,
		0, 0, 0xff, 0xff,
		8, 6, 0, 0, 0,
		0, 0, 0, 0,
	}
	checksum := crc32.ChecksumIEEE(data[12:29])
	binary.BigEndian.PutUint32(data[29:33], checksum)
	return data
}
