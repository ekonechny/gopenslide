package gopenslide

// #cgo CFLAGS: -I/usr/include/openslide
// #cgo LDFLAGS: -lopenslide
// #include <stdio.h>
// #include <stdlib.h>
// #include <stdint.h>
// #include <openslide.h>
// char * str_at(char ** p, int i) { return p[i]; }
import "C"
import (
	"errors"
	"fmt"
	"image"
	"unsafe"
)

const associatedNameLabel = "label"

// WSI wrap openslide object
type WSI struct {
	p *C.openslide_t
}

// Open opens the wsi file
// don't forget to call Close
func Open(filename string) (WSI, error) {
	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))
	data := C.openslide_open(cFilename)
	if data == nil {
		return WSI{nil}, fmt.Errorf("file %s unrecognized", filename)
	}
	return WSI{data}, nil
}

// AssociatedImageDimensions returns associated file dimension by name
func (wsi WSI) AssociatedImageDimensions(name string) (int64, int64, bool) {
	var w, h C.int64_t
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	C.openslide_get_associated_image_dimensions(wsi.p, cName, &w, &h)
	return int64(w), int64(h), int64(w) != -1
}

// AssociatedImageNames returns list of associated files
func (wsi WSI) AssociatedImageNames() []string {
	vendor := wsi.GetVendor()
	associatedNames := mapCStringArr(C.openslide_get_associated_image_names(wsi.p))
	if _, ok := LabelInMacroVendor[vendor]; ok {
		associatedNames = append(associatedNames, associatedNameLabel)
	}

	return associatedNames
}

// BestLevelForDownsample returns best level for downsampling
func (wsi WSI) BestLevelForDownsample(downsample float64) int32 {
	return int32(C.openslide_get_best_level_for_downsample(wsi.p, C.double(downsample)))
}

// Close closes openslide object
func (wsi WSI) Close() {
	C.openslide_close(wsi.p)
}

// DetectVendor return vendor name
func DetectVendor(filename string) (string, error) {
	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))
	slideVendor := C.openslide_detect_vendor(cFilename)
	if slideVendor == nil {
		return "", errors.New("No vendor for " + filename)
	}
	return C.GoString(slideVendor), nil
}

func (wsi WSI) GetVendor() string {
	return wsi.PropertyValue(PropertyVendor)
}

// LargestLevelDimensions return dimension of largest image
func (wsi WSI) LargestLevelDimensions() (int64, int64) {
	var w, h C.int64_t
	C.openslide_get_level0_dimensions(wsi.p, &w, &h)
	return int64(w), int64(h)
}

// LevelCount returns count of level resolutions
func (wsi WSI) LevelCount() int32 {
	return int32(C.openslide_get_level_count(wsi.p))
}

// LevelDimensions returns dimenstion for current level
func (wsi WSI) LevelDimensions(level int32) (int64, int64) {
	var a, b C.int64_t
	C.openslide_get_level_dimensions(wsi.p, C.int32_t(level), &a, &b)
	return int64(a), int64(b)
}

// LevelDownsample return level downsample for current level
func (wsi WSI) LevelDownsample(level int32) float64 {
	return float64(C.openslide_get_level_downsample(wsi.p, C.int32_t(level)))
}

func mapCStringArr(cItems **C.char) []string {
	var strings []string
	for i := 0; C.str_at(cItems, C.int(i)) != nil; i++ {
		strings = append(strings, C.GoString(C.str_at(cItems, C.int(i))))
	}
	return strings
}

// PropertyValue returns value for current property
func (wsi WSI) PropertyValue(propName string) string {
	cname := C.CString(propName)
	defer C.free(unsafe.Pointer(cname))
	cPropertyValue := C.openslide_get_property_value(wsi.p, cname)
	return C.GoString(cPropertyValue)
}

// PropertyValueWithDefault returns value for current property or default value
func (wsi WSI) PropertyValueWithDefault(name string, defaultValue string) string {
	v := wsi.PropertyValue(name)
	if v != "" {
		return v
	}
	return defaultValue
}

// ReadAssociatedImage returns associated image
func (wsi WSI) ReadAssociatedImage(name string, w, h int64) (ImageWithSubImage, error) {
	l := w * h * 4
	rawPtr := C.malloc(C.size_t(l))
	defer C.free(rawPtr)
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	C.openslide_read_associated_image(wsi.p, cName, (*C.uint32_t)(rawPtr))
	if cErrString := C.openslide_get_error(wsi.p); cErrString != nil {
		return nil, errors.New(C.GoString(cErrString))
	}
	bts := C.GoBytes(rawPtr, C.int(l))
	// transform bytes from argb format to rgba
	argb2rgba(bts)
	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	img.Pix = bts
	return img, nil
}

// ReadRegion returns region as image.Image by coordinates
func (wsi WSI) ReadRegion(location image.Point, level int32, bounds image.Point) (image.Image, error) {
	if bounds.X < 0 || bounds.Y < 0 {
		// OpenSlide would catch this, but not before we tried to allocate
		// a negative-size buffer
		return nil, errors.New(fmt.Sprintf("negative width (%d) or negative height (%d) not allowed",
			bounds.X, bounds.Y))
	}
	bts, err := wsi.readRegion(int64(location.X), int64(location.Y), level, int64(bounds.X), int64(bounds.Y))
	if err != nil {
		return nil, err
	}
	// transform bytes from argb format to rgba
	argb2rgba(bts)

	// make tile
	tile := image.NewRGBA(image.Rect(0, 0, bounds.X, bounds.Y))
	tile.Pix = bts
	return tile, nil
}

func (wsi WSI) readRegion(x, y int64, level int32, w, h int64) ([]byte, error) {
	l := w * h * 4
	rawPtr := C.malloc(C.size_t(l))
	defer C.free(rawPtr)
	C.openslide_read_region(
		wsi.p,
		(*C.uint32_t)(rawPtr),
		C.int64_t(x),
		C.int64_t(y),
		C.int32_t(level),
		C.int64_t(w),
		C.int64_t(h),
	)
	if txt := C.openslide_get_error(wsi.p); txt != nil {
		return nil, errors.New(C.GoString(txt))
	}
	return C.GoBytes(rawPtr, C.int(l)), nil
}

// PropertyNames returns list of properties
func (wsi WSI) PropertyNames() []string {
	return mapCStringArr(C.openslide_get_property_names(wsi.p))
}

// Version returns version of openslide
func Version() string {
	cver := C.openslide_get_version()
	return C.GoString(cver)
}
