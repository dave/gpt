package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/dave/gpt/kml"
	"github.com/tkrajina/go-elevations/geoelevations"
)

var SrtmClient *geoelevations.Srtm

func main() {
	if err := Main(); err != nil {
		log.Fatalf("%v", err)
	}
}

func Main() error {

	tracks := flag.String("tracks", "./input/All Tracks.kmz", "all tracks file")
	points := flag.String("points", "./input/All Points.kmz", "all points file")
	ele := flag.Bool("ele", true, "lookup elevations")
	output := flag.String("output", "./output", "output dir")
	flag.Parse()

	if *ele {
		var err error
		SrtmClient, err = geoelevations.NewSrtm(http.DefaultClient)
		if err != nil {
			return fmt.Errorf("creating srtm client: %w", err)
		}
	}

	tracksRoot, err := loadKmz(*tracks)
	if err != nil {
		return fmt.Errorf("loading tracks kmz: %w", err)
	}

	pointsRoot, err := loadKmz(*points)
	if err != nil {
		return fmt.Errorf("loading points kmz: %w", err)
	}

	data, err := scanKml(tracksRoot, pointsRoot, *ele)
	if err != nil {
		return fmt.Errorf("scanning kml: %w", err)
	}

	if err := buildRoutes(data); err != nil {
		return fmt.Errorf("building routes: %w", err)
	}

	if err := saveRoutes(data, *output); err != nil {
		return fmt.Errorf("saving routes: %w", err)
	}

	/*
		for _, id := range keys {
			fmt.Println(sections[id].Raw)
			for _, r := range sections[id].Tracks {
				fmt.Println("-", r.Raw, r.Optional)
			}
		}
	*/

	//fmt.Println("gpt", root.Document.Name)
	return nil
}

func loadKmz(fpath string) (kml.Root, error) {
	zrc, err := zip.OpenReader(fpath)
	if err != nil {
		return kml.Root{}, fmt.Errorf("opening %q: %w", fpath, err)
	}

	defer zrc.Close()

	frc, err := zrc.File[0].Open()
	if err != nil {
		return kml.Root{}, fmt.Errorf("unzipping %q: %w", fpath, err)
	}

	root, err := kml.Decode(frc)
	if err != nil {
		return kml.Root{}, fmt.Errorf("decoding kml %q: %w", fpath, err)
	}

	return root, nil
}
