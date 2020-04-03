package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

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
	stamp := flag.String("stamp", fmt.Sprintf("%04d%02d%02d", time.Now().Year(), time.Now().Month(), time.Now().Day()), "date stamp for output files")
	flag.Parse()

	if *ele {
		var err error
		SrtmClient, err = geoelevations.NewSrtm(http.DefaultClient)
		if err != nil {
			return fmt.Errorf("creating srtm client: %w", err)
		}
	}

	tracksRoot, err := kml.Load(*tracks)
	if err != nil {
		return fmt.Errorf("loading tracks kmz: %w", err)
	}

	pointsRoot, err := kml.Load(*points)
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

	if err := saveGaia(data, *output); err != nil {
		return fmt.Errorf("saving gaia files: %w", err)
	}

	if err := saveGpx(data, *output, *stamp); err != nil {
		return fmt.Errorf("saving generic gps files: %w", err)
	}

	if err := saveKmlTracks(data, *output, *stamp); err != nil {
		return fmt.Errorf("saving generic gps files: %w", err)
	}

	if err := saveKmlWaypoints(data, *output, *stamp); err != nil {
		return fmt.Errorf("saving generic gps files: %w", err)
	}
	return nil
}
