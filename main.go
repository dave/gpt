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

const VERSION = "v0.0.9"

func main() {
	if err := Main(); err != nil {
		log.Fatalf("%v", err)
	}
}

func Main() error {

	input := flag.String("input", "./GPT Master.kmz", "input file")
	ele := flag.Bool("ele", true, "lookup elevations")
	output := flag.String("output", "./output", "output dir")
	stamp := flag.String("stamp", fmt.Sprintf("%04d%02d%02d", time.Now().Year(), time.Now().Month(), time.Now().Day()), "date stamp for output files")
	version := flag.Bool("version", false, "show version")
	flag.Parse()

	if *version {
		fmt.Println(VERSION)
		return nil
	}

	if *ele {
		var err error
		SrtmClient, err = geoelevations.NewSrtm(http.DefaultClient)
		if err != nil {
			return fmt.Errorf("creating srtm client: %w", err)
		}
	}

	inputRoot, err := kml.Load(*input)
	if err != nil {
		return fmt.Errorf("loading tracks kmz: %w", err)
	}

	data, err := scanKml(inputRoot, *ele)
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
