package main

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/dave/gpt/geo"
)

// Section is a section folder
type Section struct {
	Raw         string // raw name of the section folder
	Key         SectionKey
	Name        string   // name of the section
	Tracks      []*Track // raw tracks from the kml data
	Hiking      *Route
	Packrafting *Route
	Optional    map[OptionalKey]*Route
}

type SectionKey struct {
	Number int
	Suffix string
}

func (k SectionKey) Code() string {
	return fmt.Sprintf("%02d%s", k.Number, k.Suffix)
}

type OptionalKey struct {
	Option  int
	Variant string
}

func (k OptionalKey) Code() string {
	if k.Option > 0 {
		return fmt.Sprintf("%02d%s", k.Option, k.Variant)
	}
	return k.Variant
}

// Route is a continuous path composed of several adjoining segments (maybe from different tracks)
type Route struct {
	*Section
	Hiking, Packrafting bool
	Optional            OptionalKey
	Name                string // track name for optional tracks
	Segments            []*Segment
}

// Normalise finds the first segment (using name data), reorders the other segments and reverses them when needed.
func (r *Route) Normalise() error {

	if len(r.Segments) < 2 {
		// can't normalise a single segment
		return nil
	}

	// TODO: Remove this special case
	spurs := map[string]bool{
		"RP-MR-V@29P-114.5+5.6": true,
		"RP-MR-V@29P-0.0+5.6":   true,
		"OH-TL-V@18-01A-#003":   true,
	}
	var routeSegments []*Segment
	for _, segment := range r.Segments {
		if spurs[segment.Raw] {
			continue
		}
		routeSegments = append(routeSegments, segment)
	}

	findClosest := func(current *Segment, from geo.Pos, exclude map[*Segment]bool) (segment *Segment, start bool, dist float64, err error) {

		hardCoded := map[string]string{
			"RP-MR-V@29P-120.2+8.4": "RP-MR-V@29P-5.6+4.3",
		}

		collection := routeSegments

		if hardCoded[current.Raw] != "" {
			// we have a hardcoded next segment
			for _, s := range routeSegments {
				if s.Raw == hardCoded[current.Raw] {
					collection = []*Segment{s}
					break
				}
			}
		}

		type data struct {
			segment *Segment
			start   bool
			dist    float64
		}
		var measurements []data
		for _, s := range collection {
			if exclude[s] {
				continue
			}
			measurements = append(measurements, data{s, true, from.Distance(s.Start())})
			measurements = append(measurements, data{s, false, from.Distance(s.End())})
		}

		sort.Slice(measurements, func(i, j int) bool { return measurements[i].dist < measurements[j].dist })

		closeSegments := map[*Segment]data{}
		for _, measurement := range measurements {
			if measurement.dist > 0.05 {
				break
			}
			if closeSegments[measurement.segment].segment == nil || closeSegments[measurement.segment].dist > measurement.dist {
				closeSegments[measurement.segment] = measurement
			}
		}
		if len(closeSegments) > 1 {
			var segmentsInSameTrack []*Segment
			for s := range closeSegments {
				if current.Track == s.Track {
					segmentsInSameTrack = append(segmentsInSameTrack, s)
				}
			}
			if len(segmentsInSameTrack) == 0 {
				// are all the close segments in the same track (but different to current)?
				allSameTrack := true
				var first *Segment
				for s := range closeSegments {
					if first == nil {
						first = s
					}
					if s.Track != first.Track {
						allSameTrack = false
						break
					}
				}
				if allSameTrack {
					for s := range closeSegments {
						segmentsInSameTrack = append(segmentsInSameTrack, s)
					}
				}
			}
			if len(segmentsInSameTrack) == 0 {
				message := fmt.Sprintf("%q has %d nearby segments (none in same track):", current.Raw, len(closeSegments))
				for s := range closeSegments {
					message += fmt.Sprintf(" %q", s.Raw)
				}
				return nil, false, 0.0, errors.New(message)
			}
			sort.Slice(segmentsInSameTrack, func(i, j int) bool { return segmentsInSameTrack[i].Index() < segmentsInSameTrack[j].Index() })
			if current.Track == segmentsInSameTrack[0].Track && current.Index() > segmentsInSameTrack[0].Index() {
				message := fmt.Sprintf("%q has %d nearby segments (none in same track with higher index):", current.Raw, len(closeSegments))
				for s := range closeSegments {
					message += fmt.Sprintf(" %q", s.Raw)
				}
				return nil, false, 0.0, errors.New(message)
			}
			measurement := closeSegments[segmentsInSameTrack[0]]
			return measurement.segment, measurement.start, measurement.dist, nil
		}

		if len(measurements) == 0 {
			return nil, false, 0.0, fmt.Errorf("can't find close segment for %q", current.Raw)
		}

		return measurements[0].segment, measurements[0].start, measurements[0].dist, nil
	}
	findFirst := func() *Segment {

		var special string
		switch {
		case r.Section.Key.Number == 22 && r.Hiking:
			special = "RH-PR-V@22-115.8+4.3"
		case r.Section.Key.Number == 22 && r.Packrafting:
			special = "RP-MR-V@22-90.0+0.5"
		case r.Section.Key.Code() == "29P":
			special = "RP-FJ-2@29P-190.3+2.2"
		}
		if special != "" {
			for _, segment := range routeSegments {
				if segment.Raw == special {
					return segment
				}
			}
		}

		for _, segment := range routeSegments {
			if segment.From == 0 && segment.Length > 0 {
				return segment
			}
			if segment.Option > 0 && segment.Count == 1 {
				return segment
			}
			if segment.Variant != "" && segment.Count == 1 {
				return segment
			}
		}
		return nil
	}
	used := map[*Segment]bool{}
	var segments []*Segment

	first := findFirst()

	used[first] = true
	segments = append(segments, first)

	// first might need reversing
	_, _, distFromStart, err := findClosest(first, first.Start(), used)
	if err != nil {
		return err
	}
	_, _, distFromEnd, err := findClosest(first, first.End(), used)
	if err != nil {
		return err
	}

	if math.Min(distFromStart, distFromEnd) > 0.05 {
		return fmt.Errorf("closest segment to %q is %.0f m away", first.Raw, math.Min(distFromStart, distFromEnd)*1000)
	}

	if distFromStart < distFromEnd {
		// reverse the segment
		//fmt.Printf("Reversing: %s\n", first.Raw)
		first.Line.Reverse()
	}

	current := first

	for len(segments) != len(routeSegments) {
		next, start, dist, err := findClosest(current, current.End(), used)
		if err != nil {
			return err
		}
		if dist > 0.05 {

			// TODO: remove special cases
			switch {
			case next.Raw == "EXP-RP-RI-2@90P-152.3+7.6":
			case next.Raw == "RP-LK-1@37P-5.3+1.8":
			case r.Section.Key.Code() == "24H" && r.Optional == OptionalKey{0, "A"}:
			// ignore
			default:
				//fmt.Printf("closest segment to %q is %q - %.0f m away\n", current.Raw, next.Raw, dist*1000)
				return fmt.Errorf("closest segment to %q is %q - %.0f m away", current.Raw, next.Raw, dist*1000)
			}

		}
		if !start {
			// reverse the next segment
			//fmt.Printf("Reversing: %s\n", next.Raw)
			next.Line.Reverse()
		}
		used[next] = true
		segments = append(segments, next)

		current = next

	}

	r.Segments = segments
	return nil
}

// Track is a track folder in a section folder
type Track struct {
	*Section
	Raw          string // raw name of the track folder
	Optional     bool   // is this section in the "Optional Tracks" folder?
	Experimental bool   // track folder has "EXP-" prefix
	Code         string // track type code - RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
	Year         int    // year in brackets in the track folder
	Variants     bool   // track folder is named "Variants"
	Option       int    // option number if the track folder is named "Option X"
	Name         string // track name for optional tracks
	Segments     []*Segment
}

// Segment is a placemark / linestring in a track folder
type Segment struct {
	*Track
	Raw          string  // raw name of the placemark
	Experimental bool    // segment name has "EXP-" prefix
	Code         string  // track code from the segment name - RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
	Terrain      string  // terrain code from segment name - BB: Bush Bashing, CC: Cross Country, MR: Minor Road, PR: Primary or Paved Road, TL: Horse or Hiking Trail, FJ: Fjord Packrafting, LK: Lake Packrafting, RI: River Packrafting
	Verification string  // verification status - V: Verified Route, A: Approximate Route, I: Investigation Route
	Directional  string  // directional status - 1: One-Way Route, 2: Two-Way Route
	Variant      string  // variant from segment name
	Count        int     // counter for optional track
	From         float64 // from km for regular track
	Length       float64 // length km for regular track
	Name         string  // named feature
	Line         geo.Line
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

// Start is the first location in the kml linestring
func (s Segment) Start() geo.Pos {
	return s.Line[0]
}

// End is the last location in the kml linestring
func (s Segment) End() geo.Pos {
	return s.Line[len(s.Line)-1]
}