package imaging

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

var (
	errCreate = errors.New("failed to create file")
	errClose  = errors.New("failed to close file")
	errOpen   = errors.New("failed to open file")
)

type badFS struct{}

func (badFS) Create(name string) (io.WriteCloser, error) {
	if name == "badFile.jpg" {
		return badFile{ioutil.Discard}, nil
	}
	return nil, errCreate
}

func (badFS) Open(name string) (io.ReadCloser, error) {
	return nil, errOpen
}

type badFile struct {
	io.Writer
}

func (badFile) Close() error {
	return errClose
}

func TestOpenSave(t *testing.T) {
	imgWithoutAlpha := image.NewNRGBA(image.Rect(0, 0, 4, 6))
	imgWithoutAlpha.Pix = []uint8{
		0x00, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0x00, 0x00, 0xff, 0xff, 0x00, 0x00, 0xff, 0x00, 0xff, 0x00, 0xff, 0x00, 0xff, 0x00, 0xff,
		0xff, 0x00, 0x00, 0xff, 0xff, 0x00, 0x00, 0xff, 0x00, 0xff, 0x00, 0xff, 0x00, 0xff, 0x00, 0xff,
		0x00, 0x00, 0xff, 0xff, 0x00, 0x00, 0xff, 0xff, 0x88, 0x88, 0x88, 0xff, 0x88, 0x88, 0x88, 0xff,
		0x00, 0x00, 0xff, 0xff, 0x00, 0x00, 0xff, 0xff, 0x88, 0x88, 0x88, 0xff, 0x88, 0x88, 0x88, 0xff,
	}
	imgWithAlpha := image.NewNRGBA(image.Rect(0, 0, 4, 6))
	imgWithAlpha.Pix = []uint8{
		0x00, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0x00, 0x00, 0x80, 0xff, 0x00, 0x00, 0x80, 0x00, 0xff, 0x00, 0x80, 0x00, 0xff, 0x00, 0x80,
		0xff, 0x00, 0x00, 0x80, 0xff, 0x00, 0x00, 0x80, 0x00, 0xff, 0x00, 0x80, 0x00, 0xff, 0x00, 0x80,
		0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0xff, 0x00, 0x88, 0x88, 0x88, 0x00, 0x88, 0x88, 0x88, 0x00,
		0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0xff, 0x00, 0x88, 0x88, 0x88, 0x00, 0x88, 0x88, 0x88, 0x00,
	}

	options := []EncodeOption{
		JPEGQuality(100),
	}

	dir, err := ioutil.TempDir("", "imaging")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(dir)

	for _, ext := range []string{"jpg", "jpeg", "png", "gif", "bmp", "tif", "tiff"} {
		filename := filepath.Join(dir, "test."+ext)

		img := imgWithoutAlpha
		if ext == "png" {
			img = imgWithAlpha
		}

		err := Save(img, filename, options...)
		if err != nil {
			t.Fatalf("failed to save image (%q): %v", filename, err)
		}

		img2, err := Open(filename)
		if err != nil {
			t.Fatalf("failed to open image (%q): %v", filename, err)
		}
		got := Clone(img2)

		delta := 0
		if ext == "jpg" || ext == "jpeg" || ext == "gif" {
			delta = 3
		}

		if !compareNRGBA(got, img, delta) {
			t.Fatalf("bad encode-decode result (ext=%q): got %#v want %#v", ext, got, img)
		}
	}

	buf := &bytes.Buffer{}
	err = Encode(buf, imgWithAlpha, JPEG)
	if err != nil {
		t.Fatalf("failed to encode alpha to JPEG: %v", err)
	}

	buf = &bytes.Buffer{}
	err = Encode(buf, imgWithAlpha, Format(100))
	if err != ErrUnsupportedFormat {
		t.Fatalf("got %v want ErrUnsupportedFormat", err)
	}

	buf = bytes.NewBuffer([]byte("bad data"))
	_, err = Decode(buf)
	if err == nil {
		t.Fatalf("decoding bad data: expected error got nil")
	}

	err = Save(imgWithAlpha, filepath.Join(dir, "test.unknown"))
	if err != ErrUnsupportedFormat {
		t.Fatalf("got %v want ErrUnsupportedFormat", err)
	}

	prevFS := fs
	fs = badFS{}
	defer func() { fs = prevFS }()

	err = Save(imgWithAlpha, "test.jpg")
	if err != errCreate {
		t.Fatalf("got error %v want errCreate", err)
	}

	err = Save(imgWithAlpha, "badFile.jpg")
	if err != errClose {
		t.Fatalf("got error %v want errClose", err)
	}

	_, err = Open("test.jpg")
	if err != errOpen {
		t.Fatalf("got error %v want errOpen", err)
	}
}

func TestNew(t *testing.T) {
	testCases := []struct {
		name      string
		w, h      int
		c         color.Color
		dstBounds image.Rectangle
		dstPix    []uint8
	}{
		{
			"New 1x1 black",
			1, 1,
			color.NRGBA{0, 0, 0, 0},
			image.Rect(0, 0, 1, 1),
			[]uint8{0x00, 0x00, 0x00, 0x00},
		},
		{
			"New 1x2 red",
			1, 2,
			color.NRGBA{255, 0, 0, 255},
			image.Rect(0, 0, 1, 2),
			[]uint8{0xff, 0x00, 0x00, 0xff, 0xff, 0x00, 0x00, 0xff},
		},
		{
			"New 2x1 white",
			2, 1,
			color.NRGBA{255, 255, 255, 255},
			image.Rect(0, 0, 2, 1),
			[]uint8{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		},
		{
			"New 3x3 with alpha",
			3, 3,
			color.NRGBA{0x01, 0x23, 0x45, 0x67},
			image.Rect(0, 0, 3, 3),
			[]uint8{
				0x01, 0x23, 0x45, 0x67, 0x01, 0x23, 0x45, 0x67, 0x01, 0x23, 0x45, 0x67,
				0x01, 0x23, 0x45, 0x67, 0x01, 0x23, 0x45, 0x67, 0x01, 0x23, 0x45, 0x67,
				0x01, 0x23, 0x45, 0x67, 0x01, 0x23, 0x45, 0x67, 0x01, 0x23, 0x45, 0x67,
			},
		},
		{
			"New 0x0 white",
			0, 0,
			color.NRGBA{255, 255, 255, 255},
			image.Rect(0, 0, 0, 0),
			nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := New(tc.w, tc.h, tc.c)
			want := image.NewNRGBA(tc.dstBounds)
			want.Pix = tc.dstPix
			if !compareNRGBA(got, want, 0) {
				t.Fatalf("got result %#v want %#v", got, want)
			}
		})
	}
}

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		New(1024, 1024, color.White)
	}
}

func TestFormats(t *testing.T) {
	formatNames := map[Format]string{
		JPEG:       "JPEG",
		PNG:        "PNG",
		GIF:        "GIF",
		BMP:        "BMP",
		TIFF:       "TIFF",
		Format(-1): "Unsupported",
	}
	for format, name := range formatNames {
		got := format.String()
		if got != name {
			t.Fatalf("got format name %q want %q", got, name)
		}
	}
}

func TestClone(t *testing.T) {
	testCases := []struct {
		name string
		src  image.Image
		want *image.NRGBA
	}{
		{
			"Clone NRGBA",
			&image.NRGBA{
				Rect:   image.Rect(-1, -1, 0, 1),
				Stride: 1 * 4,
				Pix:    []uint8{0x00, 0x11, 0x22, 0x33, 0xcc, 0xdd, 0xee, 0xff},
			},
			&image.NRGBA{
				Rect:   image.Rect(0, 0, 1, 2),
				Stride: 1 * 4,
				Pix:    []uint8{0x00, 0x11, 0x22, 0x33, 0xcc, 0xdd, 0xee, 0xff},
			},
		},
		{
			"Clone NRGBA64",
			&image.NRGBA64{
				Rect:   image.Rect(-1, -1, 0, 1),
				Stride: 1 * 8,
				Pix: []uint8{
					0x00, 0x00, 0x11, 0x11, 0x22, 0x22, 0x33, 0x33,
					0xcc, 0xcc, 0xdd, 0xdd, 0xee, 0xee, 0xff, 0xff,
				},
			},
			&image.NRGBA{
				Rect:   image.Rect(0, 0, 1, 2),
				Stride: 1 * 4,
				Pix:    []uint8{0x00, 0x11, 0x22, 0x33, 0xcc, 0xdd, 0xee, 0xff},
			},
		},
		{
			"Clone RGBA",
			&image.RGBA{
				Rect:   image.Rect(-1, -1, 0, 2),
				Stride: 1 * 4,
				Pix:    []uint8{0x00, 0x00, 0x00, 0x00, 0x00, 0x11, 0x22, 0x33, 0xcc, 0xdd, 0xee, 0xff},
			},
			&image.NRGBA{
				Rect:   image.Rect(0, 0, 1, 3),
				Stride: 1 * 4,
				Pix:    []uint8{0x00, 0x00, 0x00, 0x00, 0x00, 0x55, 0xaa, 0x33, 0xcc, 0xdd, 0xee, 0xff},
			},
		},
		{
			"Clone RGBA64",
			&image.RGBA64{
				Rect:   image.Rect(-1, -1, 0, 2),
				Stride: 1 * 8,
				Pix: []uint8{
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x11, 0x11, 0x22, 0x22, 0x33, 0x33,
					0xcc, 0xcc, 0xdd, 0xdd, 0xee, 0xee, 0xff, 0xff,
				},
			},
			&image.NRGBA{
				Rect:   image.Rect(0, 0, 1, 3),
				Stride: 1 * 4,
				Pix:    []uint8{0x00, 0x00, 0x00, 0x00, 0x00, 0x55, 0xaa, 0x33, 0xcc, 0xdd, 0xee, 0xff},
			},
		},
		{
			"Clone Gray",
			&image.Gray{
				Rect:   image.Rect(-1, -1, 0, 1),
				Stride: 1 * 1,
				Pix:    []uint8{0x11, 0xee},
			},
			&image.NRGBA{
				Rect:   image.Rect(0, 0, 1, 2),
				Stride: 1 * 4,
				Pix:    []uint8{0x11, 0x11, 0x11, 0xff, 0xee, 0xee, 0xee, 0xff},
			},
		},
		{
			"Clone Gray16",
			&image.Gray16{
				Rect:   image.Rect(-1, -1, 0, 1),
				Stride: 1 * 2,
				Pix:    []uint8{0x11, 0x11, 0xee, 0xee},
			},
			&image.NRGBA{
				Rect:   image.Rect(0, 0, 1, 2),
				Stride: 1 * 4,
				Pix:    []uint8{0x11, 0x11, 0x11, 0xff, 0xee, 0xee, 0xee, 0xff},
			},
		},
		{
			"Clone Alpha",
			&image.Alpha{
				Rect:   image.Rect(-1, -1, 0, 1),
				Stride: 1 * 1,
				Pix:    []uint8{0x11, 0xee},
			},
			&image.NRGBA{
				Rect:   image.Rect(0, 0, 1, 2),
				Stride: 1 * 4,
				Pix:    []uint8{0xff, 0xff, 0xff, 0x11, 0xff, 0xff, 0xff, 0xee},
			},
		},
		{
			"Clone YCbCr",
			&image.YCbCr{
				Rect:           image.Rect(-1, -1, 5, 0),
				SubsampleRatio: image.YCbCrSubsampleRatio444,
				YStride:        6,
				CStride:        6,
				Y:              []uint8{0x00, 0xff, 0x7f, 0x26, 0x4b, 0x0e},
				Cb:             []uint8{0x80, 0x80, 0x80, 0x6b, 0x56, 0xc0},
				Cr:             []uint8{0x80, 0x80, 0x80, 0xc0, 0x4b, 0x76},
			},
			&image.NRGBA{
				Rect:   image.Rect(0, 0, 6, 1),
				Stride: 6 * 4,
				Pix: []uint8{
					0x00, 0x00, 0x00, 0xff,
					0xff, 0xff, 0xff, 0xff,
					0x7f, 0x7f, 0x7f, 0xff,
					0x7f, 0x00, 0x00, 0xff,
					0x00, 0x7f, 0x00, 0xff,
					0x00, 0x00, 0x7f, 0xff,
				},
			},
		},
		{
			"Clone YCbCr 444",
			&image.YCbCr{
				Y:              []uint8{0x4c, 0x69, 0x1d, 0xb1, 0x96, 0xe2, 0x26, 0x34, 0xe, 0x59, 0x4b, 0x71, 0x0, 0x4c, 0x99, 0xff},
				Cb:             []uint8{0x55, 0xd4, 0xff, 0x8e, 0x2c, 0x01, 0x6b, 0xaa, 0xc0, 0x95, 0x56, 0x40, 0x80, 0x80, 0x80, 0x80},
				Cr:             []uint8{0xff, 0xeb, 0x6b, 0x36, 0x15, 0x95, 0xc0, 0xb5, 0x76, 0x41, 0x4b, 0x8c, 0x80, 0x80, 0x80, 0x80},
				YStride:        4,
				CStride:        4,
				SubsampleRatio: image.YCbCrSubsampleRatio444,
				Rect:           image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: 4, Y: 4}},
			},
			&image.NRGBA{
				Pix:    []uint8{0xff, 0x0, 0x0, 0xff, 0xff, 0x0, 0xff, 0xff, 0x0, 0x0, 0xff, 0xff, 0x49, 0xe1, 0xca, 0xff, 0x0, 0xff, 0x0, 0xff, 0xff, 0xff, 0x0, 0xff, 0x7f, 0x0, 0x0, 0xff, 0x7f, 0x0, 0x7f, 0xff, 0x0, 0x0, 0x7f, 0xff, 0x0, 0x7f, 0x7f, 0xff, 0x0, 0x7f, 0x0, 0xff, 0x82, 0x7f, 0x0, 0xff, 0x0, 0x0, 0x0, 0xff, 0x4c, 0x4c, 0x4c, 0xff, 0x99, 0x99, 0x99, 0xff, 0xff, 0xff, 0xff, 0xff},
				Stride: 16,
				Rect:   image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: 4, Y: 4}},
			},
		},
		{
			"Clone YCbCr 440",
			&image.YCbCr{
				Y:              []uint8{0x4c, 0x69, 0x1d, 0xb1, 0x96, 0xe2, 0x26, 0x34, 0xe, 0x59, 0x4b, 0x71, 0x0, 0x4c, 0x99, 0xff},
				Cb:             []uint8{0x2c, 0x01, 0x6b, 0xaa, 0x80, 0x80, 0x80, 0x80},
				Cr:             []uint8{0x15, 0x95, 0xc0, 0xb5, 0x80, 0x80, 0x80, 0x80},
				YStride:        4,
				CStride:        4,
				SubsampleRatio: image.YCbCrSubsampleRatio440,
				Rect:           image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: 4, Y: 4}},
			},
			&image.NRGBA{
				Pix:    []uint8{0x0, 0xb5, 0x0, 0xff, 0x86, 0x86, 0x0, 0xff, 0x77, 0x0, 0x0, 0xff, 0xfb, 0x7d, 0xfb, 0xff, 0x0, 0xff, 0x1, 0xff, 0xff, 0xff, 0x1, 0xff, 0x80, 0x0, 0x1, 0xff, 0x7e, 0x0, 0x7e, 0xff, 0xe, 0xe, 0xe, 0xff, 0x59, 0x59, 0x59, 0xff, 0x4b, 0x4b, 0x4b, 0xff, 0x71, 0x71, 0x71, 0xff, 0x0, 0x0, 0x0, 0xff, 0x4c, 0x4c, 0x4c, 0xff, 0x99, 0x99, 0x99, 0xff, 0xff, 0xff, 0xff, 0xff},
				Stride: 16,
				Rect:   image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: 4, Y: 4}},
			},
		},
		{
			"Clone YCbCr 422",
			&image.YCbCr{
				Y:              []uint8{0x4c, 0x69, 0x1d, 0xb1, 0x96, 0xe2, 0x26, 0x34, 0xe, 0x59, 0x4b, 0x71, 0x0, 0x4c, 0x99, 0xff},
				Cb:             []uint8{0xd4, 0x8e, 0x01, 0xaa, 0x95, 0x40, 0x80, 0x80},
				Cr:             []uint8{0xeb, 0x36, 0x95, 0xb5, 0x41, 0x8c, 0x80, 0x80},
				YStride:        4,
				CStride:        2,
				SubsampleRatio: image.YCbCrSubsampleRatio422,
				Rect:           image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: 4, Y: 4}},
			},
			&image.NRGBA{
				Pix:    []uint8{0xe2, 0x0, 0xe1, 0xff, 0xff, 0x0, 0xfe, 0xff, 0x0, 0x4d, 0x36, 0xff, 0x49, 0xe1, 0xca, 0xff, 0xb3, 0xb3, 0x0, 0xff, 0xff, 0xff, 0x1, 0xff, 0x70, 0x0, 0x70, 0xff, 0x7e, 0x0, 0x7e, 0xff, 0x0, 0x34, 0x33, 0xff, 0x1, 0x7f, 0x7e, 0xff, 0x5c, 0x58, 0x0, 0xff, 0x82, 0x7e, 0x0, 0xff, 0x0, 0x0, 0x0, 0xff, 0x4c, 0x4c, 0x4c, 0xff, 0x99, 0x99, 0x99, 0xff, 0xff, 0xff, 0xff, 0xff},
				Stride: 16,
				Rect:   image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: 4, Y: 4}},
			},
		},
		{
			"Clone YCbCr 420",
			&image.YCbCr{
				Y:       []uint8{0x4c, 0x69, 0x1d, 0xb1, 0x96, 0xe2, 0x26, 0x34, 0xe, 0x59, 0x4b, 0x71, 0x0, 0x4c, 0x99, 0xff},
				Cb:      []uint8{0x01, 0xaa, 0x80, 0x80},
				Cr:      []uint8{0x95, 0xb5, 0x80, 0x80},
				YStride: 4, CStride: 2,
				SubsampleRatio: image.YCbCrSubsampleRatio420,
				Rect:           image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: 4, Y: 4}},
			},
			&image.NRGBA{
				Pix:    []uint8{0x69, 0x69, 0x0, 0xff, 0x86, 0x86, 0x0, 0xff, 0x67, 0x0, 0x67, 0xff, 0xfb, 0x7d, 0xfb, 0xff, 0xb3, 0xb3, 0x0, 0xff, 0xff, 0xff, 0x1, 0xff, 0x70, 0x0, 0x70, 0xff, 0x7e, 0x0, 0x7e, 0xff, 0xe, 0xe, 0xe, 0xff, 0x59, 0x59, 0x59, 0xff, 0x4b, 0x4b, 0x4b, 0xff, 0x71, 0x71, 0x71, 0xff, 0x0, 0x0, 0x0, 0xff, 0x4c, 0x4c, 0x4c, 0xff, 0x99, 0x99, 0x99, 0xff, 0xff, 0xff, 0xff, 0xff},
				Stride: 16,
				Rect:   image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: 4, Y: 4}},
			},
		},
		{
			"Clone Paletted",
			&image.Paletted{
				Rect:   image.Rect(-1, -1, 5, 0),
				Stride: 6 * 1,
				Palette: color.Palette{
					color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff},
					color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
					color.NRGBA{R: 0x7f, G: 0x7f, B: 0x7f, A: 0xff},
					color.NRGBA{R: 0x7f, G: 0x00, B: 0x00, A: 0xff},
					color.NRGBA{R: 0x00, G: 0x7f, B: 0x00, A: 0xff},
					color.NRGBA{R: 0x00, G: 0x00, B: 0x7f, A: 0xff},
				},
				Pix: []uint8{0x0, 0x1, 0x2, 0x3, 0x4, 0x5},
			},
			&image.NRGBA{
				Rect:   image.Rect(0, 0, 6, 1),
				Stride: 6 * 4,
				Pix: []uint8{
					0x00, 0x00, 0x00, 0xff,
					0xff, 0xff, 0xff, 0xff,
					0x7f, 0x7f, 0x7f, 0xff,
					0x7f, 0x00, 0x00, 0xff,
					0x00, 0x7f, 0x00, 0xff,
					0x00, 0x00, 0x7f, 0xff,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := Clone(tc.src)
			delta := 0
			if _, ok := tc.src.(*image.YCbCr); ok {
				delta = 1
			}
			if !compareNRGBA(got, tc.want, delta) {
				t.Fatalf("got result %#v want %#v", got, tc.want)
			}
		})
	}
}
