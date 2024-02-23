package gopenslide

import (
	"image"
	"image/color"
	"strconv"

	"github.com/disintegration/imaging"
)

const (
	vendorHamamatsu = "hamamatsu"
	vendorVentana   = "ventana"
)

const (
	assocImageLabel = "label"
	assocImageMacro = "macro"
)

var LabelInMacroVendor = map[string]struct{}{
	vendorHamamatsu: {},
	vendorVentana:   {},
}

type ImageWithSubImage interface {
	image.Image
	SubImage(rectangle image.Rectangle) image.Image
}

func ReadAssociatedImage(slide WSI, name string) (image.Image, bool, error) {
	if _, ok := LabelInMacroVendor[slide.GetVendor()]; ok {
		if name == assocImageMacro || name == assocImageLabel {
			return getImageFromMacro(slide, name)
		}
	}

	return readAssociatedImage(slide, name)
}

func getImageFromMacro(slide WSI, name string) (image.Image, bool, error) {
	img, ok, err := readAssociatedImage(slide, assocImageMacro)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	return getImageFromMacroByName(img, name), true, nil
}

func getImageFromMacroByName(img ImageWithSubImage, name string) image.Image {
	width := img.Bounds().Max.X
	height := img.Bounds().Max.Y
	baseThreshold := height

	if height > width {
		baseThreshold = width
		img = imaging.Rotate(img, 90, image.Transparent)
	}
	if name == assocImageLabel {
		return img.SubImage(image.Rect(0, 0, baseThreshold, img.Bounds().Dy()))
	}

	return img.SubImage(image.Rect(baseThreshold, 0, img.Bounds().Dx(), img.Bounds().Dy()))
}

func readAssociatedImage(slide WSI, name string) (ImageWithSubImage, bool, error) {
	w, h, ok := slide.AssociatedImageDimensions(name)
	if !ok {
		return nil, false, nil
	}
	img, err := slide.ReadAssociatedImage(name, w, h)
	if err != nil {
		return nil, false, err
	}

	if img.Bounds().Dx() != int(w) || img.Bounds().Dy() != int(h) {
		img = imaging.Fill(img, int(w), int(h), imaging.Center, imaging.Lanczos)
	}

	if h > w && name == assocImageMacro {
		img = imaging.Rotate(img, 90, image.Transparent)
	}

	return img, true, nil
}

// ReadTileFromSlide Return an RGB Image for a tile.
func ReadTileFromSlide(t Tile, slide WSI) (image.Image, error) {
	tile, err := slide.ReadRegion(
		t.tileInfo.l0Location,
		int32(t.tileInfo.slideLevel),
		t.tileInfo.lSize,
	)
	if err != nil {
		return nil, err
	}

	bgImg := imaging.New(tile.Bounds().Dx(), tile.Bounds().Dy(), color.NRGBA{
		R: 255,
		G: 255,
		B: 255,
		A: 255,
	})

	tile = imaging.OverlayCenter(bgImg, tile, 1)

	if tile.Bounds().Dx() != t.tileInfo.zSize.X || tile.Bounds().Dy() != t.tileInfo.zSize.Y {
		tile = imaging.Thumbnail(tile, t.tileInfo.zSize.X, t.tileInfo.zSize.Y, imaging.Lanczos)
	}
	return tile, nil
}

func mustStrToInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func reverse[T comparable](a []T) {
	for i := len(a)/2 - 1; i >= 0; i-- {
		opp := len(a) - 1 - i
		a[i], a[opp] = a[opp], a[i]
	}
}
