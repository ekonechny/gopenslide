package gopenslide

import (
	"context"
	"errors"
	"image"
	"math"
	"strconv"
)

type (
	// DeepZoomGenerator provides functionality for generating Deep Zoom images from OpenSlide objects
	DeepZoomGenerator struct {
		wsi         WSI
		tileSize    int
		overlap     int
		limitBounds bool

		deepZoomTileLevels []image.Point
		deepZoomLevels     []image.Point
		level0Offset       image.Point
		levels             []image.Point
		downsamples        []downsample
	}

	// Tile wrap openslide raw data
	Tile struct {
		Level int
		Row   int
		Col   int

		tileInfo
	}

	tileInfo struct {
		l0Location image.Point
		lSize      image.Point
		zSize      image.Point
		slideLevel int
	}
)

// Tile returns tile object by coordinates (level, col and row)
func (dz *DeepZoomGenerator) Tile(level, col, row int) (Tile, error) {
	ti, err := dz.tileInfo(level, col, row)
	if err != nil {
		return Tile{}, err
	}

	return Tile{
		Level:    level,
		Row:      row,
		Col:      col,
		tileInfo: ti,
	}, nil
}

func (dz *DeepZoomGenerator) Iter(ctx context.Context) <-chan Tile {
	ch := make(chan Tile)
	go func() {
		defer close(ch)
		for level := 0; level < dz.LevelsCount(); level++ {
			cols, rows := dz.Level(level).X, dz.Level(level).Y
			for row := 0; row < rows; row++ {
				for col := 0; col < cols; col++ {
					tile, _ := dz.Tile(level, col, row)
					// TODO: handle error

					select {
					case <-ctx.Done():
						return
					case ch <- tile:
					}
				}
			}
		}
	}()
	return ch
}

func getDeepZoomLevels(zSize image.Point) []image.Point {
	zDimensions := []image.Point{zSize}
	for {
		if zSize.X <= 1 && zSize.Y <= 1 {
			break
		}
		zSize = image.Point{
			X: int(math.Max(1, math.Ceil(float64(zSize.X)/2))),
			Y: int(math.Max(1, math.Ceil(float64(zSize.Y)/2))),
		}
		zDimensions = append(zDimensions, zSize)
	}
	reverse(zDimensions)
	return zDimensions
}

func tileCount(tileSize, zLim int) int {
	return int(math.Ceil(float64(zLim) / float64(tileSize)))
}

func getDeepZoomTileLevels(tileSize int, dimensions []image.Point) (r []image.Point) {
	for _, d := range dimensions {
		r = append(r, image.Point{X: tileCount(tileSize, d.X), Y: tileCount(tileSize, d.Y)})
	}
	return r
}

type downsample struct {
	slideLevel          levelDownsample
	bestLevelDownsample float64
}

type levelDownsample struct {
	level      int32
	downsample float64
	image.Point
}

func generateDownsamples(slide WSI, dzLevelsCount int) []downsample {
	var downsamples []downsample

	for dzLevel := 0; dzLevel < dzLevelsCount; dzLevel++ {
		l0ZDownsample := math.Pow(2, float64(dzLevelsCount)-float64(dzLevel)-1)
		bestLevel := slide.BestLevelForDownsample(l0ZDownsample)
		X, Y := slide.LevelDimensions(bestLevel)
		downsamples = append(downsamples, downsample{
			slideLevel: levelDownsample{
				level:      bestLevel,
				downsample: slide.LevelDownsample(bestLevel),
				Point: image.Point{
					X: int(X),
					Y: int(Y),
				},
			},
			bestLevelDownsample: l0ZDownsample / slide.LevelDownsample(bestLevel),
		})
	}
	return downsamples
}

func getLevel0Offset(slide WSI, limitBounds bool) image.Point {
	if limitBounds {
		return image.Point{
			X: mustStrToInt(slide.PropertyValue(PropertyBoundsX)),
			Y: mustStrToInt(slide.PropertyValue(PropertyBoundsY)),
		}
	}
	return image.Point{}
}

// NewDeepZoomGenerator creates a DeepZoomGenerator wrapping an OpenSlide object
func NewDeepZoomGenerator(slide WSI, tileSize int, overlap int, limitBounds bool) *DeepZoomGenerator {
	levels := getLevelDimensions(slide, limitBounds)
	deepZoomLevels := getDeepZoomLevels(levels[0])
	deepZoomTileLevels := getDeepZoomTileLevels(tileSize, deepZoomLevels)
	downsamples := generateDownsamples(slide, len(deepZoomTileLevels))
	return &DeepZoomGenerator{
		wsi:                slide,
		tileSize:           tileSize,
		overlap:            overlap,
		deepZoomTileLevels: deepZoomTileLevels,
		deepZoomLevels:     deepZoomLevels,
		levels:             levels,
		level0Offset:       getLevel0Offset(slide, limitBounds),
		downsamples:        downsamples,
	}
}

// Read returns an image.Image for a tile
func (dz *DeepZoomGenerator) Read(_ context.Context, t Tile) (image.Image, error) {
	return ReadTileFromSlide(t, dz.wsi)
}

// LevelsCount provides the number of Deep Zoom levels in the image
func (dz *DeepZoomGenerator) LevelsCount() int {
	return len(dz.downsamples)
}

// LevelTiles provides slide of points for each Deep Zoom level.
func (dz *DeepZoomGenerator) LevelTiles() []image.Point {
	return dz.deepZoomTileLevels
}

// Level provides deep zoom level
func (dz *DeepZoomGenerator) Level(level int) image.Point {
	return dz.deepZoomTileLevels[level]
}

// LevelDimensions provides a list of points for each Deep Zoom level
func (dz *DeepZoomGenerator) LevelDimensions() []image.Point {
	return dz.deepZoomLevels
}

// TileCount provides the total number of Deep Zoom tiles in the image
func (dz *DeepZoomGenerator) TileCount() int {
	var sum int
	for _, dimension := range dz.deepZoomTileLevels {
		sum += dimension.X * dimension.Y
	}
	return sum
}

type deepZoomOverlap struct {
	top    int
	left   int
	bottom int
	right  int
}

func getDeepZoomOverlap(levelDimension image.Point, overlap, col, row int) deepZoomOverlap {
	return deepZoomOverlap{
		left:   overlap * boolToInt(col != 0),
		top:    overlap * boolToInt(row != 0),
		right:  overlap * boolToInt(col != int(levelDimension.X)-1),
		bottom: overlap * boolToInt(row != int(levelDimension.Y)-1),
	}
}

func (dz *DeepZoomGenerator) tileInfo(dzLevel, col, row int) (tileInfo, error) {
	if dzLevel < 0 || dzLevel > len(dz.downsamples) {
		return tileInfo{}, errors.New("invalid levelDownsample")
	}
	// TODO: validate the presence of a column and row in dz.getDeepZoomTileLevels
	if col < 0 || row < 0 {
		return tileInfo{}, errors.New("invalid address")
	}

	level := dz.downsamples[dzLevel]

	// Calculate top/left and bottom/right overlap
	dzOverlap := getDeepZoomOverlap(dz.Level(dzLevel), dz.overlap, col, row)

	zSize := image.Point{
		X: int(math.Min(
			float64(dz.tileSize),
			float64(dz.deepZoomLevels[dzLevel].X-dz.tileSize*col),
		)) + dzOverlap.left + dzOverlap.right,
		Y: int(math.Min(
			float64(dz.tileSize),
			float64(dz.deepZoomLevels[dzLevel].Y-dz.tileSize*row),
		)) + dzOverlap.top + dzOverlap.bottom,
	}
	// Obtain the region coordinates
	zLocation := image.Point{
		X: dz.tileSize * col,
		Y: dz.tileSize * row,
	}
	lLocation := [2]float64{
		level.bestLevelDownsample * (float64(zLocation.X) - float64(dzOverlap.left)),
		level.bestLevelDownsample * (float64(zLocation.Y) - float64(dzOverlap.top)),
	}
	// Round location down and size up, and add offset of active area
	l0Location := image.Point{
		X: int(level.slideLevel.downsample*lLocation[0] + float64(dz.level0Offset.X)),
		Y: int(level.slideLevel.downsample*lLocation[1] + float64(dz.level0Offset.Y)),
	}

	lSize := image.Point{
		X: int(math.Min(
			math.Ceil(level.bestLevelDownsample*float64(zSize.X)),
			float64(level.slideLevel.X)-math.Ceil(lLocation[0]),
		)),
		Y: int(math.Min(
			math.Ceil(level.bestLevelDownsample*float64(zSize.Y)),
			float64(level.slideLevel.Y)-math.Ceil(lLocation[1]),
		)),
	}

	return tileInfo{
		l0Location: l0Location,
		lSize:      lSize,
		zSize:      zSize,
		slideLevel: int(level.slideLevel.level),
	}, nil
}

func getLevelDimensions(slide WSI, limitBounds bool) []image.Point {
	lCount := slide.LevelCount()
	dimensions := make([]image.Point, 0, lCount)
	for i := int32(0); i < lCount; i++ {
		w, h := slide.LevelDimensions(i)
		dimensions = append(dimensions, image.Point{X: int(w), Y: int(h)})
	}

	if !limitBounds {
		return dimensions
	}

	l0width, l0height := dimensions[0].X, dimensions[0].Y
	xRatio := mustStrToInt(slide.PropertyValueWithDefault(PropertyBoundsWidth, strconv.Itoa(l0width))) / l0width
	yRatio := mustStrToInt(slide.PropertyValueWithDefault(PropertyBoundsHeight, strconv.Itoa(l0height))) / l0height
	for i := range dimensions {
		dimensions[i].X *= xRatio
		dimensions[i].Y *= yRatio
	}

	return dimensions
}
