package imageprep

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
)

const (
	emojiSize       = 128
	maxSourcePixels = 16_000_000
	maxUploadBytes  = 256 * 1024
)

func Prepare(data []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("generated data is not a supported image")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxSourcePixels {
		return nil, errors.New("generated image dimensions exceed limits")
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("decode generated image")
	}

	dst := image.NewNRGBA(image.Rect(0, 0, emojiSize, emojiSize))
	bounds := src.Bounds()
	side := bounds.Dx()
	if bounds.Dy() < side {
		side = bounds.Dy()
	}
	x0 := bounds.Min.X + (bounds.Dx()-side)/2
	y0 := bounds.Min.Y + (bounds.Dy()-side)/2
	crop := image.Rect(x0, y0, x0+side, y0+side)

	for y := range emojiSize {
		sy := crop.Min.Y + y*side/emojiSize
		for x := range emojiSize {
			sx := crop.Min.X + x*side/emojiSize
			dst.SetNRGBA(x, y, color.NRGBAModel.Convert(src.At(sx, sy)).(color.NRGBA))
		}
	}

	var output bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&output, dst); err != nil {
		return nil, err
	}
	if output.Len() > maxUploadBytes {
		return nil, errors.New("prepared emoji exceeds Discord's 256 KiB limit")
	}
	return output.Bytes(), nil
}
