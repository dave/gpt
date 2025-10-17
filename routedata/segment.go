package routedata

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dave/gpt/geo"
	"github.com/dave/gpt/globals"
)

// Segment is a placemark / linestring
type Segment struct {
	Route        *Route
	Raw          string   // raw name of the placemark
	Experimental bool     // segment name has "EXP-" prefix
	Code         string   // track code from the segment name - RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
	Terrains     []string // terrain codes from segment name - BB: Bush Bashing, CC: Cross Country, MR: Minor Road, PR: Primary or Paved Road, TL: Horse or Hiking Trail, FJ: Fjord Packrafting, LK: Lake Packrafting, RI: River Packrafting, FY: Ferry
	Verification string   // verification status - V: Verified Route, A: Approximate Route, I: Investigation Route
	Directional  string   // directional status - 1: One-Way Route, 2: Two-Way Route
	Length       float64  // calculated length km
	Name         string   // named feature
	Legacy       string   // name before last rename job
	Line         geo.Line
	Modes        map[globals.ModeType]*SegmentModeData
}

type SegmentModeData struct {
	From       float64
	StartPoint *Point
	EndPoint   *Point
	MidPoints  []*Point
}

func (s Segment) PlacemarkName() string {
	var b strings.Builder

	if s.Experimental {
		b.WriteString("EXP")
		b.WriteString("-")
	}
	b.WriteString(s.Code)
	b.WriteString("-")
	b.WriteString(strings.Join(s.Terrains, "&"))
	if s.Verification != "" || s.Directional != "" {
		b.WriteString("-")
		b.WriteString(s.Verification)
		b.WriteString(s.Directional)
	}

	b.WriteString(" {")
	b.WriteString(s.Route.Section.Key.Code())
	b.WriteString(s.Route.Key.Direction)
	if s.Route.Key.Required == globals.OPTIONAL {
		b.WriteString("-")
		if s.Route.Key.Option > 0 {
			b.WriteString(fmt.Sprintf("%02d", s.Route.Key.Option))
		}
		b.WriteString(s.Route.Key.Variant)
		if s.Route.Key.Network != "" {
			b.WriteString(s.Route.Key.Network)
		}
	}
	b.WriteString("}")

	b.WriteString(" [")
	if hike, raft := s.Modes[globals.HIKE], s.Modes[globals.RAFT]; hike != nil && raft != nil {

		if fmt.Sprintf("%.1f", raft.From) == fmt.Sprintf("%.1f", hike.From) {
			b.WriteString(fmt.Sprintf("%.1f", raft.From))
		} else {
			b.WriteString(fmt.Sprintf("%.1f", raft.From))
			b.WriteString("/")
			b.WriteString(fmt.Sprintf("%.1f", hike.From))
		}
	} else if raft != nil {
		b.WriteString(fmt.Sprintf("%.1f", raft.From))
	} else if hike != nil {
		b.WriteString(fmt.Sprintf("%.1f", hike.From))
	}
	b.WriteString("+")
	b.WriteString(fmt.Sprintf("%.1f", s.Length))
	b.WriteString("]")

	if s.Name != "" {
		b.WriteString(" (")
		b.WriteString(s.Name)
		b.WriteString(")")
	}
	return b.String()
}

//func (s *Segment) DuplicateForTrack() *Segment {
//	// Segments can't be shared between packrafting and hiking routes because we may need to reverse the segment in one
//	// route not in the other. So when we add a segment to a route, we duplicate it.
//	out := &Segment{
//		Track:        s.Track,
//		Raw:          s.Raw,
//		Experimental: s.Experimental,
//		Code:         s.Code,
//		Verification: s.Verification,
//		Directional:  s.Directional,
//		Variant:      s.Variant,
//		Count:        s.Count,
//		From:         s.From,
//		Length:       s.Length,
//		Name:         s.Name,
//
//		// not assigned yet so no need to copy
//		StartPoint: nil,
//		EndPoint:   nil,
//		MidPoints:  nil,
//
//		// below
//		Terrains: nil,
//		Line:     nil,
//	}
//	line := make(geo.Line, len(s.Line))
//	for i, pos := range s.Line {
//		line[i] = pos
//	}
//	out.Line = line
//
//	terrains := make([]string, len(s.Terrains))
//	for i, t := range s.Terrains {
//		terrains[i] = t
//	}
//	out.Terrains = terrains
//	return out
//}

//func (s *Segment) Reverse() {
//	debugString += fmt.Sprintf("Reversing %s\n", s.String())
//	s.Reversed = !s.Reversed
//	s.Line.Reverse()
//	s.StartPoint, s.EndPoint = s.EndPoint, s.StartPoint
//
//	s.StartPoint.Start = true
//	s.StartPoint.End = false
//	s.StartPoint.Index = 0
//
//	s.EndPoint.Start = false
//	s.EndPoint.End = true
//	s.EndPoint.Index = len(s.Line) - 1
//
//	for _, point := range s.MidPoints {
//		point.Index = len(s.Line) - 1 - point.Index
//	}
//}

func (s *SegmentModeData) Points(reorder bool) []*Point {
	if reorder {
		// Make sure mid points are ordered correctly
		sort.Slice(s.MidPoints, func(i, j int) bool { return s.MidPoints[i].Index < s.MidPoints[j].Index })
	}
	return append([]*Point{s.StartPoint, s.EndPoint}, s.MidPoints...)
}

//func (s Segment) String() string {
//	var b strings.Builder
//	if s.Experimental {
//		b.WriteString("EXP")
//		b.WriteString("-")
//	}
//	b.WriteString(s.Code)
//	b.WriteString("-")
//	b.WriteString(strings.Join(s.Terrains, "&"))
//	if s.Verification != "" || s.Directional != "" {
//		b.WriteString("-")
//		b.WriteString(s.Verification)
//		b.WriteString(s.Directional)
//	}
//	b.WriteString("@")
//	b.WriteString(s.Route.Section.Key.Code())
//	if s.Route.Key.Required == OPTIONAL {
//		b.WriteString("-")
//		if s.Route.Key.Option > 0 {
//			b.WriteString(fmt.Sprintf("%02d", s.Route.Key.Option))
//		}
//		b.WriteString(s.Route.Key.Variant)
//		//b.WriteString("-")
//		//b.WriteString(fmt.Sprintf("#%03d", s.Count))
//	} else {
//		b.WriteString("-")
//		b.WriteString(fmt.Sprintf("%.1f+%.1f", s.From, s.Length))
//	}
//	if s.Name != "" {
//		b.WriteString(" (")
//		b.WriteString(s.Name)
//		b.WriteString(")")
//	}
//	return b.String()
//}

var weights = map[string]float64{
	"thick": 3,
	"thin":  1,
}

var colours = map[string]string{
	"bright-orange": "e3aa71",
	"orange":        "ff8000",
	"bright-violet": "e371e3",
	"violet":        "ff00ff",
	"rose":          "df9f9f",
	"red":           "ff0000",
	"green":         "00ffb1",
	"blue":          "00aaff",
	"white":         "ffffff",
}

func (s Segment) Style() string {
	// Terrain: BB: Bush Bashing, CC: Cross Country, MR: Minor Road, PR: Primary or Paved Road, TL: Horse or Hiking Trail, FJ: Fjord Packrafting, LK: Lake Packrafting, RI: River Packrafting, FY: Ferry
	// Code: RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
	// Verification: V: Verified Route, A: Approximate Route, I: Investigation Route

	// https://colordesigner.io/color-mixer

	/*
	   1. Hiking Land Route (BB, CC, TL, MR, PR): Red
	   2. Investigation (I) Hiking Land Route (BB, CC, TL, MR, PR): Rose (Red + White)
	   3. Packrafting Only Land Route (BB, CC, TL, MR, PR): Violett (Red + Blue)
	   4. Investigation (I) Packrafting Only Land Route (BB, CC, TL, MR, PR): Bright Violett (Red + Blue + White)
	   5. Packrafting Water Route (FJ, LK, RI): Blue
	   6. Ferry Route (FY): White
	   7. Exploration Land Route (Regardless if Hiking or Packrafting Only): Orange (Red + Yellow)
	   8. Investigation Exploration Land Route (Regardless if Hiking or Packrafting Only): Bright Orange (Red + Yellow + White)
	   9. Exploration Packrafting Water Route: Green (Blue + Yellow)
	*/

	var color, weight string
	switch s.Terrains[0] {
	case "BB", "CC", "TL", "MR", "PR": // Land terrain
		switch s.Experimental {
		case true:
			switch s.Verification {
			case "I": // Investigation
				color = "bright-orange" // red + yellow + white - #e3aa71
			default:
				color = "orange" // red + yellow - #ff8000
			}
		case false:
			switch s.Code {
			case "RP", "OP": // Packrafting only segments
				switch s.Verification {
				case "I": // Investigation
					color = "bright-violet" // red + blue + white - #e371e3
				default:
					color = "violet" // red + blue - #ff00ff
				}
			default: // Either hiking specific or dual use segments
				switch s.Verification {
				case "I": // Investigation
					color = "rose" // red + white - #df9f9f
				default:
					color = "red" // red - #ff0000
				}
			}
		}
	case "FJ", "LK", "RI": // Water terrain
		switch s.Experimental {
		case true:
			color = "green" // blue + yellow - #00ffb1
		case false:
			color = "blue" // blue - #0000ff
		}
	case "FY":
		color = "white" // white - #ffffff
	}

	if s.Route.Key.Required == globals.REGULAR {
		weight = "thick"
	} else {
		weight = "thin"
	}

	return fmt.Sprintf("%s-%s", weight, color)

}

// Index in the track folder
//func (s *Segment) Index() int {
//	for i, segment := range s.Track.Segments {
//		if s == segment {
//			return i
//		}
//	}
//	panic("can't find segment in track")
//}

func (s1 Segment) Similar(s2 *Segment) bool {
	return compareTerrain(s1.Terrains, s2.Terrains) &&
		s1.Verification == s2.Verification &&
		s1.Directional == s2.Directional &&
		s1.Experimental == s2.Experimental
}

// Compares two unordered slices of terrain types.
func compareTerrain(a1, a2 []string) bool {
	if len(a1) != len(a2) {
		return false
	}
	for _, t1 := range a1 {
		var found bool
		for _, t2 := range a2 {
			if t1 == t2 {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func Directional(code string) string {
	switch code {
	case "1":
		return "One-Way"
	case "2":
		return "Two-Way"
	}
	return ""
}

func Verification(code string) string {
	switch code {
	case "V":
		return "Verified"
	case "A":
		return "Approximate"
	case "I":
		return "Investigation"
	}
	return ""
}

func HasFerry(terrains []string) bool {
	for _, terrain := range terrains {
		switch terrain {
		case "FY":
			return true
		}
	}
	return false
}

func Terrain(code string) string {
	switch code {
	case "BB":
		return "Bush Bashing"
	case "CC":
		return "Cross Country"
	case "MR":
		return "Minor Road"
	case "PR":
		return "Paved Road"
	case "TL":
		return "Trail"
	case "FJ":
		return "Fjord"
	case "LK":
		return "Lake"
	case "RI":
		return "River"
	case "FY":
		return "Ferry"
	}
	return ""
}
