package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dave/gpt/kml"
	"github.com/tkrajina/go-elevations/geoelevations"
)

var SrtmClient *geoelevations.Srtm

const VERSION = "v0.1.2"
const DELTA = 0.075 // see https://docs.google.com/spreadsheets/d/1q610i2TkfUTHWvtqVAJ0V8zFtzPMQKBXEm7jiPyuDCQ/edit

var LOG, DEBUG bool
var HAS_SINGLE bool
var SINGLE SectionKey

type ModeType int

const HIKE ModeType = 0
const RAFT ModeType = 1

var MODES = []ModeType{HIKE, RAFT}

type RequiredType int

const REGULAR RequiredType = 0
const OPTIONAL RequiredType = 1

var REQUIRED_TYPES = []RequiredType{REGULAR, OPTIONAL}

func main() {
	if err := Main(); err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}
}

func Main() error {

	input := flag.String("input", "./GPT Master.kmz", "input file")
	logger := flag.Bool("log", true, "output logs")
	debugger := flag.Bool("debug", false, "debug")
	single := flag.String("single", "", "only process a single section (for testing)")
	ele := flag.Bool("ele", true, "lookup elevations")
	scrape := flag.Bool("scrape", true, "scrape descriptions from wikiexplora")
	output := flag.String("output", "./output", "output dir")
	renames := flag.Bool("renames", false, "create rename log file and RESET legacy names in master file")
	stamp := flag.String("stamp", fmt.Sprintf("%04d%02d%02d", time.Now().Year(), time.Now().Month(), time.Now().Day()), "date stamp for output files")
	version := flag.Bool("version", false, "show version")
	flag.Parse()

	LOG = *logger
	DEBUG = *debugger

	logln("initialising")

	if *single != "" {
		key, err := NewSectionKey(*single)
		if err != nil {
			return fmt.Errorf("parsing single flag: %w", err)
		}
		HAS_SINGLE = true
		SINGLE = key
	}

	if *version {
		fmt.Println(VERSION)
		return nil
	}

	if *ele {
		log.SetOutput(ioutil.Discard)
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

	data := &Data{Sections: map[SectionKey]*Section{}}

	if err := data.Scan(inputRoot, *ele); err != nil {
		return fmt.Errorf("scanning kml: %w", err)
	}

	if *scrape {
		logln("web scraping")
		if err := data.Scrape(); err != nil {
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
			if route.Key.Required == REGULAR {
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

func logln(a ...interface{}) {
	if LOG {
		fmt.Println(a...)
	}
}

func logf(format string, a ...interface{}) {
	if LOG {
		fmt.Printf(format, a...)
	}
}

func debugln(a ...interface{}) {
	if DEBUG {
		fmt.Println(a...)
	}
}

func debugf(format string, a ...interface{}) {
	if DEBUG {
		fmt.Printf(format, a...)
	}
}

func debugfln(format string, a ...interface{}) {
	if DEBUG {
		fmt.Printf(format+"\n", a...)
	}
}
