package transform

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// outDir is the gallery path every test writes its visible
// output to. Wiped + recreated once per `go test` invocation so
// the folder always reflects the latest run.
const outDir = "testdata/out"

// TestMain prepares the output gallery. Any test that produces
// a visual artefact should call writeOut so a human can open
// testdata/out/ after the run and eyeball the transformations.
func TestMain(m *testing.M) {
	// Clean slate: stale outputs from a previous run confuse
	// visual inspection.
	_ = os.RemoveAll(outDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic("transform tests: cannot prepare " + outDir + ": " + err.Error())
	}
	os.Exit(m.Run())
}

// writeOut persists `data` under testdata/out/<name>.<ext>. Never
// fails the test on an I/O problem — the gallery is a
// convenience, not a contract.
func writeOut(t *testing.T, name, ext string, data []byte) {
	t.Helper()
	path := filepath.Join(outDir, name+"."+ext)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Logf("gallery write %s: %v", path, err)
	}
}

// writeImageOut encodes img in the requested format and stores
// it under testdata/out. Format must be one Encode supports.
func writeImageOut(t *testing.T, name string, img image.Image, f Format) {
	t.Helper()
	var buf bytes.Buffer
	if err := Encode(&buf, img, f, EncodeOpts{}); err != nil {
		t.Logf("gallery encode %s: %v", name, err)
		return
	}
	writeOut(t, name, f.Ext(), buf.Bytes())
}

// solidRGBA returns a w×h RGBA image filled with c. Handy for
// assertions on solid-colour inputs. Note: color.RGBA is
// premultiplied — pass values understood in that colour space.
// For fully-opaque colours this is indistinguishable from
// straight RGBA, which is what most tests actually want.
func solidRGBA(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// solidNRGBA returns a w×h NRGBA image filled with c. Use this
// when you want to reason about "50% transparent red" in the
// intuitive non-premultiplied sense — RGBA's premultiplied
// encoding makes channel-by-channel assertions surprising after
// resize/blur.
func solidNRGBA(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// gradientRGBA returns a w×h RGBA image with a horizontal red
// gradient (left: black, right: red) and a vertical green
// gradient (top: black, bottom: green). Useful as a rich input
// for visual inspection of resize / crop / effects.
func gradientRGBA(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			rv := uint8((x * 255) / max1(w-1))
			gv := uint8((y * 255) / max1(h-1))
			img.SetRGBA(x, y, color.RGBA{R: rv, G: gv, B: 0, A: 255})
		}
	}
	return img
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// encodePNG returns the PNG bytes of img. Fails the test on a
// real encoder failure (image/png only fails on a nil writer).
func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encodePNG: %v", err)
	}
	return buf.Bytes()
}

// encodeJPEG returns JPEG bytes of img at the given quality.
func encodeJPEG(t *testing.T, img image.Image, quality int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatalf("encodeJPEG: %v", err)
	}
	return buf.Bytes()
}

// encodeGIFAnimated produces a 2-frame animated GIF — frame 0 is
// all red, frame 1 is all green. Used to pin the "first-frame
// only" decode contract.
func encodeGIFAnimated(t *testing.T) []byte {
	t.Helper()
	const w, h = 4, 4
	palette := []color.Color{
		color.RGBA{R: 255, A: 255},
		color.RGBA{G: 255, A: 255},
	}
	frame0 := image.NewPaletted(image.Rect(0, 0, w, h), palette)
	frame1 := image.NewPaletted(image.Rect(0, 0, w, h), palette)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			frame0.SetColorIndex(x, y, 0) // red
			frame1.SetColorIndex(x, y, 1) // green
		}
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, &gif.GIF{
		Image:     []*image.Paletted{frame0, frame1},
		Delay:     []int{5, 5},
		LoopCount: 0,
	}); err != nil {
		t.Fatalf("encodeGIFAnimated: %v", err)
	}
	return buf.Bytes()
}

// nearRGBA asserts channel-wise colour proximity. tolerance is
// in 0..255 space. Alpha is checked strictly unless ignoreAlpha.
func nearRGBA(t *testing.T, got, want color.RGBA, tolerance int, label string) {
	t.Helper()
	diff := func(a, b uint8) int {
		d := int(a) - int(b)
		if d < 0 {
			return -d
		}
		return d
	}
	if diff(got.R, want.R) > tolerance ||
		diff(got.G, want.G) > tolerance ||
		diff(got.B, want.B) > tolerance ||
		diff(got.A, want.A) > tolerance {
		t.Errorf("%s: got %+v want %+v (tolerance %d)", label, got, want, tolerance)
	}
}

// pxAt returns the pixel at (x,y) as an RGBA regardless of the
// underlying image implementation.
func pxAt(img image.Image, x, y int) color.RGBA {
	c := img.At(x, y)
	r, g, b, a := c.RGBA()
	return color.RGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}
}
