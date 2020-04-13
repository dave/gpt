package main

import (
	"fmt"
	"strings"
)

// Track is a track folder in a section folder
type Track struct {
	*Section
	Raw          string // raw name of the track folder
	Optional     bool   // is this section in the "Optional Tracks" folder?
	Experimental bool   // track folder has "EXP-" prefix
	Code         string // track type code - RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
	Direction    string // direction type - N: north, S: south, "": any
	Year         int    // year in brackets in the track folder
	Variants     bool   // track folder is named "Variants"
	Option       int    // option number if the track folder is named "Option X"
	Name         string // track name for optional tracks
	Segments     []*Segment
}

func (t Track) String() string {
	var b strings.Builder
	if t.Optional {
		if t.Variants {
			b.WriteString("Variants")
		} else {
			b.WriteString("Option ")
			b.WriteString(fmt.Sprint(t.Option))
			b.WriteString(" ")
			b.WriteString(t.Name)
		}
	} else {
		if t.Experimental {
			b.WriteString("EXP")
			b.WriteString("-")
		}
		b.WriteString(t.Code)
		b.WriteString(t.Direction)
	}
	if t.Year > 0 {
		b.WriteString(" (")
		b.WriteString(fmt.Sprintf("%04d", t.Year))
		b.WriteString(")")
	}
	return b.String()
}
