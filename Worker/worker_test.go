package worker

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestApplyFilterGrayscaleProducesGrayPixels(t *testing.T) {
	input := testPNG(t)

	output, err := applyFilter(input, "grayscale")
	if err != nil {
		t.Fatalf("applyFilter returned error: %v", err)
	}

	img, _, err := image.Decode(bytes.NewReader(output))
	if err != nil {
		t.Fatalf("filtered output is not decodable: %v", err)
	}

	r, g, b, _ := img.At(0, 0).RGBA()
	if r != g || g != b {
		t.Fatalf("expected grayscale pixel, got r=%d g=%d b=%d", r, g, b)
	}
}

func TestApplyFilterBlurProducesDecodablePNG(t *testing.T) {
	output, err := applyFilter(testPNG(t), "blur")
	if err != nil {
		t.Fatalf("applyFilter returned error: %v", err)
	}

	if _, _, err := image.Decode(bytes.NewReader(output)); err != nil {
		t.Fatalf("blurred output is not decodable: %v", err)
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("could not encode test image: %v", err)
	}
	return buffer.Bytes()
}
