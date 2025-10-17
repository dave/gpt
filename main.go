package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/dave/gpt/globals"
	"github.com/dave/gpt/kml"
	"github.com/dave/gpt/routedata"
	"github.com/tkrajina/go-elevations/geoelevations"
)

func main() {
	if err := Main(); err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}
}

func Main() error {

	cacheDir := path.Join(os.Getenv("HOME"), fmt.Sprintf(".gpt-cache-%04d-%02d", time.Now().Year(), time.Now().Month()))
	elevationCacheDir := path.Join(cacheDir, "elevations")
	descriptionsCacheDir := path.Join(cacheDir, "descriptions")
	_ = os.MkdirAll(elevationCacheDir, 0777)
	_ = os.MkdirAll(descriptionsCacheDir, 0777)

	input := flag.String("input", "./GPT Master.kmz", "input file")
	logger := flag.Bool("log", true, "output logs")
	//tiles := flag.Bool("tiles", false, "output tiles")
	debugger := flag.Bool("debug", false, "debug")
	single := flag.String("single", "", "only process a single section (for testing)")
	ele := flag.Bool("ele", true, "lookup elevations")
	scrape := flag.Bool("scrape", true, "scrape descriptions from wikiexplora")
	output := flag.String("output", "./output", "output dir")
	renames := flag.Bool("renames", false, "create rename log file and RESET legacy names in master file")
	stamp := flag.String("stamp", fmt.Sprintf("%04d%02d%02d", time.Now().Year(), time.Now().Month(), time.Now().Day()), "date stamp for output files")
	version := flag.Bool("version", false, "show version")
	flag.Parse()

	globals.LOG = *logger
	globals.DEBUG = *debugger

	routedata.Initialise()

	if *single != "" {
		key, err := routedata.NewSectionKey(*single)
		if err != nil {
			return fmt.Errorf("parsing single flag: %w", err)
		}
		globals.HAS_SINGLE = true
		globals.SINGLE = key
	}

	if *version {
		fmt.Println(globals.VERSION)
		return nil
	}

	if *ele {
		log.SetOutput(io.Discard)
		var err error
		globals.SrtmClient, err = geoelevations.NewSrtmWithCustomCacheDir(http.DefaultClient, elevationCacheDir)
		if err != nil {
			return fmt.Errorf("creating srtm client: %w", err)
		}
	}

	inputRoot, err := kml.Load(*input)
	if err != nil {
		return fmt.Errorf("loading tracks kmz: %w", err)
	}

	data := &routedata.Data{Sections: map[globals.SectionKey]*routedata.Section{}}

	if err := data.Scan(inputRoot, *ele); err != nil {
		return fmt.Errorf("scanning kml: %w", err)
	}

	if *scrape {
		if err := data.Scrape(descriptionsCacheDir); err != nil {
			return fmt.Errorf("scraping web: %w", err)
		}
	}

	if err := data.Normalise(); err != nil {
		return fmt.Errorf("normalising: %w", err)
	}

	// temporary code - find main optional route and assign name to all other variants in that option
	for _, section := range data.Sections {
		optionNames := map[int]string{}
		for _, route := range section.Routes {
			if route.Option != "" {
				continue
			}
			if route.Key.Required == globals.REGULAR {
				continue
			}
			if route.Key.Option == 0 {
				continue
			}
			name, found := optionNames[route.Key.Option]
			if !found {
				for _, r := range section.Routes {
					if r.Key.Option == route.Key.Option && r.Key.Variant == "" {
						optionNames[route.Key.Option] = r.Name
						name = r.Name
						break
					}
				}
			}
			route.Option = name
		}
	}

	//if *tiles {
	//	fmt.Println("Outputting tiles")
	//	tiler.Output(*output, data)
	//}

	if err := data.SaveMaster(*output, *renames); err != nil {
		return fmt.Errorf("saving master file: %w", err)
	}

	if err := data.SaveGaia(*output); err != nil {
		return fmt.Errorf("saving gaia files: %w", err)
	}

	if err := data.SaveGpx(*output, *stamp); err != nil {
		return fmt.Errorf("saving generic gps files: %w", err)
	}

	if err := data.SaveKmlTracks(*output, *stamp); err != nil {
		return fmt.Errorf("saving generic gps files: %w", err)
	}

	if err := data.SaveKmlWaypoints(*output, *stamp); err != nil {
		return fmt.Errorf("saving generic gps files: %w", err)
	}

	return nil
}
