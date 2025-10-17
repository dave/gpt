package globals

import (
	"fmt"

	"github.com/tkrajina/go-elevations/geoelevations"
)

var SrtmClient *geoelevations.Srtm

const VERSION = "v0.3.4"
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

type SectionKey struct {
	Number int
	Suffix string
}

func (k SectionKey) Code() string {
	return fmt.Sprintf("%02d%s", k.Number, k.Suffix)
}
