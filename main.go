// gifwrapper.go
package main

/*
#include <stdlib.h>
typedef const char cchar_t;
typedef struct {
    int width;
    int height;
    int frame_count;
    unsigned char **frames;
	unsigned long *frame_delays;
} ImageData;

typedef struct {
    ImageData data;
    char* err;
	int is_unsupported_type;
} Result;

*/
import "C"
import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/png"
	"io"
	"net/http"
	"os"
	"time"
	"unsafe"

	"github.com/ingridhq/zebrash"
	"github.com/ingridhq/zebrash/drawers"
)

var errUnsupportedContentType = fmt.Errorf("unsupported content type")

//export LoadImage
func LoadImage(path *C.cchar_t) C.Result {
	goPath := C.GoString(path)

	f, err := os.Open(goPath)
	if err != nil {
		return errorResult(fmt.Errorf("failed to open file: %w", err))
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return errorResult(fmt.Errorf("failed to read file content: %w", err))
	}

	var image C.ImageData

	switch contentType := http.DetectContentType(content); {
	case contentType == "image/gif":
		image, err = loadGif(content)
		if err != nil {
			return errorResult(fmt.Errorf("failed to read GIF: %w", err))
		}
	case bytes.Contains(content, []byte("^XA")) && bytes.Contains(content, []byte("^XZ")):
		image, err = loadZpl(content)
		if err != nil {
			return errorResult(fmt.Errorf("failed to read ZPL: %w", err))
		}
	default:
		return errorResult(fmt.Errorf("%w: %v", errUnsupportedContentType, contentType))
	}

	return C.Result{
		data: image,
	}
}

func loadZpl(content []byte) (C.ImageData, error) {
	parser := zebrash.NewParser()

	res, err := parser.Parse(content)
	if err != nil {
		return C.ImageData{}, fmt.Errorf("failed to parse ZPL label: %w", err)
	}

	var buff bytes.Buffer

	drawer := zebrash.NewDrawer()

	if len(res) == 0 {
		return C.ImageData{}, fmt.Errorf("no ZPL labels to draw")
	}

	err = drawer.DrawLabelAsPng(res[0], &buff, drawers.DrawerOptions{})
	if err != nil {
		return C.ImageData{}, fmt.Errorf("failed to draw ZPL label: %w", err)
	}

	frame, err := png.Decode(&buff)
	if err != nil {
		return C.ImageData{}, err
	}

	return imageData([]image.Image{frame}, []int{0})
}

func loadGif(content []byte) (C.ImageData, error) {
	g, err := gif.DecodeAll(bytes.NewReader(content))
	if err != nil {
		return C.ImageData{}, err
	}

	frames := make([]image.Image, 0, len(g.Image))
	for _, frame := range g.Image {
		frames = append(frames, frame)
	}

	return imageData(frames, g.Delay)
}

func imageData(frames []image.Image, delays []int) (C.ImageData, error) {
	if len(frames) == 0 {
		return C.ImageData{}, fmt.Errorf("no frames to return")
	}

	frameCount := len(frames)
	width := frames[0].Bounds().Dx()
	height := frames[0].Bounds().Dy()

	framesPtr := C.malloc(C.size_t(unsafe.Sizeof(uintptr(0))) * C.size_t(frameCount))
	framePtrs := (*[1 << 30]*C.uchar)(framesPtr)

	frameDelaysPtr := C.malloc(C.size_t(unsafe.Sizeof(C.ulong(0))) * C.size_t(frameCount))
	frameDelaysPtrs := (*[1 << 30]C.ulong)(frameDelaysPtr)

	rgba := image.NewRGBA(image.Rect(0, 0, width, height))

	for i, frame := range frames {
		draw.Draw(rgba, rgba.Bounds(), frame, image.Point{}, draw.Over)

		dataSize := width * height * 4
		data := C.malloc(C.size_t(dataSize))
		out := (*[1 << 30]byte)(data)[:dataSize:dataSize]
		copy(out, rgba.Pix)

		framePtrs[i] = (*C.uchar)(data)
		frameDelaysPtrs[i] = C.ulong(time.Duration(delays[i]) * time.Second / 100)
	}

	return C.ImageData{
		width:        C.int(width),
		height:       C.int(height),
		frame_count:  C.int(frameCount),
		frames:       (**C.uchar)(framesPtr),
		frame_delays: (*C.ulong)(frameDelaysPtr),
	}, nil
}

//export FreeImageFrame
func FreeImageFrame(frame *C.uchar) {
	C.free(unsafe.Pointer(frame))
}

//export FreeResult
func FreeResult(result C.Result) {
	if result.err != nil {
		C.free(unsafe.Pointer(result.err))
	}

	if result.data.frames != nil {
		C.free(unsafe.Pointer(result.data.frames))
	}

	if result.data.frame_delays != nil {
		C.free(unsafe.Pointer(result.data.frame_delays))
	}
}

func errorResult(err error) C.Result {
	unsupported := 0
	if errors.Is(err, errUnsupportedContentType) {
		unsupported = 1
	}

	cmsg := C.CString(err.Error())
	return C.Result{
		err:                 cmsg,
		is_unsupported_type: C.int(unsupported),
	}
}

func main() {}
