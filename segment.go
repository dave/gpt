package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dave/gpt/geo"
)

// Segment is a placemark / linestring in a track folder
type Segment struct {
	*Track
	Raw          string   // raw name of the placemark
	Reversed     bool     // has the track been reversed? (to compare tracks by Raw)
	Experimental bool     // segment name has "EXP-" prefix
	Code         string   // track code from the segment name - RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
	Terrains     []string // terrain codes from segment name - BB: Bush Bashing, CC: Cross Country, MR: Minor Road, PR: Primary or Paved Road, TL: Horse or Hiking Trail, FJ: Fjord Packrafting, LK: Lake Packrafting, RI: River Packrafting, FY: Ferry
	Verification string   // verification status - V: Verified Route, A: Approximate Route, I: Investigation Route
	Directional  string   // directional status - 1: One-Way Route, 2: Two-Way Route
	Variant      string   // variant from segment name
	Count        int      // counter for optional track
	From         float64  // from km for regular track
	Length       float64  // length km for regular track
	Name         string   // named feature
	Line         geo.Line
	StartPoint   *Point
	EndPoint     *Point
	MidPoints    []*Point
}

func (s *Segment) DuplicateForTrack() *Segment {
	// Segments can't be shared between packrafting and hiking routes because we may need to reverse the segment in one
	// route not in the other. So when we add a segment to a route, we duplicate it.
	out := &Segment{
		Track:        s.Track,
		Raw:          s.Raw,
		Experimental: s.Experimental,
		Code:         s.Code,
		Verification: s.Verification,
		Directional:  s.Directional,
		Variant:      s.Variant,
		Count:        s.Count,
		From:         s.From,
		Length:       s.Length,
		Name:         s.Name,

		// not assigned yet so no need to copy
		StartPoint: nil,
		EndPoint:   nil,
		MidPoints:  nil,

		// below
		Terrains: nil,
		Line:     nil,
	}
	line := make(geo.Line, len(s.Line))
	for i, pos := range s.Line {
		line[i] = pos
	}
	out.Line = line

	terrains := make([]string, len(s.Terrains))
	for i, t := range s.Terrains {
		terrains[i] = t
	}
	out.Terrains = terrains
	return out
}

func (s *Segment) Reverse() {
	debugString += fmt.Sprintf("Reversing %s\n", s.String())
	s.Reversed = !s.Reversed
	s.Line.Reverse()
	s.StartPoint, s.EndPoint = s.EndPoint, s.StartPoint

	s.StartPoint.Start = true
	s.StartPoint.End = false
	s.StartPoint.Index = 0

	s.EndPoint.Start = false
	s.EndPoint.End = true
	s.EndPoint.Index = len(s.Line) - 1

	for _, point := range s.MidPoints {
		point.Index = len(s.Line) - 1 - point.Index
	}
}

func (s *Segment) Points(reorder bool) []*Point {
	if reorder {
		// Make sure mid points are ordered correctly
		sort.Slice(s.MidPoints, func(i, j int) bool { return s.MidPoints[i].Index < s.MidPoints[j].Index })
	}
	return append([]*Point{s.StartPoint, s.EndPoint}, s.MidPoints...)
}

func (s Segment) String() string {
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
	b.WriteString("@")
	b.WriteString(s.Section.Key.Code())
	if s.Optional {
		b.WriteString("-")
		if s.Option > 0 {
			b.WriteString(fmt.Sprintf("%02d", s.Option))
		}
		b.WriteString(s.Variant)
		b.WriteString("-")
		b.WriteString(fmt.Sprintf("#%03d", s.Count))
	} else {
		b.WriteString("-")
		b.WriteString(fmt.Sprintf("%.1f+%.1f", s.From, s.Length))
	}
	if s.Name != "" {
		b.WriteString(" (")
		b.WriteString(s.Name)
		b.WriteString(")")
	}
	return b.String()
}

// Index in the track folder
func (s *Segment) Index() int {
	for i, segment := range s.Track.Segments {
		if s == segment {
			return i
		}
	}
	panic("can't find segment in track")
}

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
