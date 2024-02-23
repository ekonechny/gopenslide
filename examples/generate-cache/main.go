package main

import (
	"context"
	"flag"
	"fmt"
	"image/jpeg"
	"log"
	"os"

	"github.com/ekonechny/gopenslide"
)

func main() {
	var fp string
	flag.StringVar(&fp, "file", "", "file for deepzoom")
	flag.Parse()
	f, err := gopenslide.Open(fp)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	generator := gopenslide.NewDeepZoomGenerator(f, 512, 0, false)
	for tile := range generator.Iter(context.Background()) {
		if err := os.MkdirAll(fmt.Sprintf("tiles/%d", tile.Level), 0777); err != nil {
			log.Fatal(err)
		}
		image, err := generator.Read(context.Background(), tile)
		if err != nil {
			log.Fatal(err)
		}
		tf, err := os.OpenFile(fmt.Sprintf("tiles/%d/%d_%d.jpg", tile.Level, tile.Col, tile.Row), os.O_CREATE|os.O_RDWR, 0777)
		if err != nil {
			log.Fatal(err)
		}
		if err = jpeg.Encode(tf, image, &jpeg.Options{
			Quality: 95,
		}); err != nil {
			log.Fatal(err)
		}
		if err = tf.Close(); err != nil {
			log.Fatal(err)
		}
	}
}
