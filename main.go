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
} ImageData;

typedef struct {
    ImageData data;
    char* err;
} Result;

*/
import "C"
import (
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"os"
	"unsafe"
)

//export LoadImage
func LoadImage(path *C.cchar_t) C.Result {
	goPath := C.GoString(path)

	f, err := os.Open(goPath)
	if err != nil {
		return errorResult(fmt.Errorf("failed to open file: %w", err))
	}
	defer f.Close()

	image, err := loadGif(f)
	if err != nil {
		return errorResult(fmt.Errorf("failed to read GIF: %w", err))
	}

	return C.Result{
		data: image,
	}
}

func loadGif(f *os.File) (C.ImageData, error) {
	g, err := gif.DecodeAll(f)
	if err != nil {
		return C.ImageData{}, err
	}

	frameCount := len(g.Image)
	width := g.Config.Width
	height := g.Config.Height

	framesPtr := C.malloc(C.size_t(unsafe.Sizeof(uintptr(0))) * C.size_t(frameCount))
	framePtrs := (*[1 << 30]*C.uchar)(framesPtr)

	for i, frame := range g.Image {
		rgba := image.NewRGBA(image.Rect(0, 0, width, height))
		draw.Draw(rgba, rgba.Bounds(), frame, image.Point{}, draw.Src)

		dataSize := width * height * 4
		data := C.malloc(C.size_t(dataSize))
		out := (*[1 << 30]byte)(data)[:dataSize:dataSize]
		copy(out, rgba.Pix)

		framePtrs[i] = (*C.uchar)(data)
	}

	return C.ImageData{
		width:       C.int(width),
		height:      C.int(height),
		frame_count: C.int(frameCount),
		frames:      (**C.uchar)(framesPtr),
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
}

func errorResult(err error) C.Result {
	cmsg := C.CString(err.Error())
	return C.Result{
		err: cmsg,
	}
}

func main() {}
