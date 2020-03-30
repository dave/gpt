package main

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"regexp"
	"sort"
	"strconv"

	"github.com/dave/gpt/kml"
)

func main() {
	if err := Main(); err != nil {
		log.Fatalf("%v", err)
	}
}

func Main() error {

	input := flag.String("input", "./input/All Tracks.kmz", "input file")
	//output := flag.String("output", "./output", "output dir")
	flag.Parse()

	zrc, err := zip.OpenReader(*input)
	if err != nil {
		return fmt.Errorf("opening %q: %w", *input, err)
	}

	frc, err := zrc.File[0].Open()
	if err != nil {
		return fmt.Errorf("unzipping %q: %w", *input, err)
	}

	var root kml.Root
	if err := xml.NewDecoder(frc).Decode(&root); err != nil {
		return fmt.Errorf("decoding %q: %w", *input, err)
	}

	var keys []SectionKey
	sections := map[SectionKey]*Section{}

	for _, rootFolder := range root.Document.Folders[0].Folders {
		optional := rootFolder.Name == "Optional Tracks"
		for _, sectionFolder := range rootFolder.Folders {

			// ^GPT(\d{2})([HP]?)-(PN )?(.*)$
			matches := level2FolderName.FindStringSubmatch(sectionFolder.Name)

			if len(matches) == 0 {
				return fmt.Errorf("section folder regex match for %q", sectionFolder.Name)
			}

			number, err := strconv.Atoi(matches[1])
			if err != nil {
				return fmt.Errorf("decoding section number for %q: %w", sectionFolder.Name, err)
			}
			suffix := matches[2]
			key := SectionKey{number, suffix}

			if sections[key] == nil {
				keys = append(keys, key)
				sections[key] = &Section{
					Raw:  sectionFolder.Name,
					Key:  SectionKey{number, suffix},
					Name: matches[3],
				}
			}

			section := sections[key]

			for _, trackFolder := range sectionFolder.Folders {

				switch trackFolder.Name {
				case "Varriants (2018)":
					trackFolder.Name = "Variants (2018)"
				case "Option 1 (Puerto Montt)":
					trackFolder.Name = "Option 1 Puerto Montt (0000)"
				case "Option 2 (Quellon)":
					trackFolder.Name = "Option 2 Quellon (0000)"
				}

				track := &Track{
					Raw:      trackFolder.Name,
					Section:  section,
					Optional: optional,
				}
				section.Tracks = append(section.Tracks, track)

				if matches := level3FolderName1.FindStringSubmatch(trackFolder.Name); len(matches) != 0 {
					// ^(EXP-)?([A-Z]{2}) \((\d{4})\)$
					track.Experimental = matches[1] == "EXP-"
					track.Code = matches[2]
					year, err := strconv.Atoi(matches[3])
					if err != nil {
						return fmt.Errorf("decoding year from %q - %q", trackFolder.Name, matches[3])
					}
					track.Year = year
				} else if matches := level3FolderName2.FindStringSubmatch(trackFolder.Name); len(matches) != 0 {
					// ^Option (\d{1,2}) (.*) \((\d{4}\))$
					option, err := strconv.Atoi(matches[1])
					if err != nil {
						return fmt.Errorf("decoding option number from %q - %q", trackFolder.Name, matches[1])
					}
					track.Option = option
					track.Name = matches[2]
					year, err := strconv.Atoi(matches[3])
					if err != nil {
						return fmt.Errorf("decoding year from %q - %q", trackFolder.Name, matches[3])
					}
					track.Year = year
				} else if matches := level3FolderName3.FindStringSubmatch(trackFolder.Name); len(matches) != 0 {
					// ^Varr?iants \((\d{4}\))$
					track.Variants = true
					year, err := strconv.Atoi(matches[1])
					if err != nil {
						return fmt.Errorf("decoding year from %q - %q", trackFolder.Name, matches[1])
					}
					track.Year = year
				} else if matches := level3FolderName4.FindStringSubmatch(trackFolder.Name); len(matches) != 0 {
					// ^Variants$
					track.Variants = true
				} else {
					return fmt.Errorf("no track folder regex match for %q", trackFolder.Name)
				}
				for _, segmentPlacemark := range trackFolder.Placemarks {

					segment := &Segment{
						Raw:   segmentPlacemark.Name,
						Track: track,
					}
					track.Segments = append(track.Segments, segment)

					switch segmentPlacemark.Name {
					case "RH-MR-V@24H-75.8¦1.7":
						segmentPlacemark.Name = "RH-MR-V@24H-75.8+1.7"
					case "EXP-OH-CC-A@T28H-05-#003":
						segmentPlacemark.Name = "EXP-OH-CC-A@28H-05-#003"
					case "EXP-OP-BB-I@#36P-04A-#001":
						segmentPlacemark.Name = "EXP-OP-BB-I@36P-04A-#001"
					case "EXP-OP-TL-V@82P-1-#001":
						segmentPlacemark.Name = "EXP-OP-TL-V@82P-01-#001"

					// missing hyphens
					case "OH-CC-A@03-02#016":
						segmentPlacemark.Name = "OH-CC-A@03-02-#016"
					case "OH-MR-V@08-F#001":
						segmentPlacemark.Name = "OH-MR-V@08-F-#001"
					case "OH-CC-A@11-02A#001":
						segmentPlacemark.Name = "OH-CC-A@11-02A-#001"
					case "OH-CC-A@12-K#001":
						segmentPlacemark.Name = "OH-CC-A@12-K-#001"
					case "OH-MR-V@12-M#001":
						segmentPlacemark.Name = "OH-MR-V@12-M-#001"
					case "OH-TL-V@12-M#003":
						segmentPlacemark.Name = "OH-TL-V@12-M-#003"
					case "OH-MR-V@12-M#004":
						segmentPlacemark.Name = "OH-MR-V@12-M-#004"
					case "OH-TL-V@12-02A#001":
						segmentPlacemark.Name = "OH-TL-V@12-02A-#001"
					case "OP-MR-V@22-G#007":
						segmentPlacemark.Name = "OP-MR-V@22-G-#007"
					case "OP-PR-V@27P-E#001":
						segmentPlacemark.Name = "OP-PR-V@27P-E-#001"
					case "OP-TL-V@27P-E#002":
						segmentPlacemark.Name = "OP-TL-V@27P-E-#002"
					case "EXP-OP-TL-V@90P-01#011":
						segmentPlacemark.Name = "EXP-OP-TL-V@90P-01-#011"
					}

					if matches := placemarkName.FindStringSubmatch(segmentPlacemark.Name); len(matches) != 0 {
						//fmt.Printf("%v %#v\n", segmentPlacemark.Name, matches)

						if matches[1] == "EXP-" {
							segment.Experimental = true
						}
						segment.Code = matches[2]
						switch segment.Code {
						/*
							RR: Regular Route
							RH: Regular Hiking Route
							RP: Regular Packrafting Route
							OH: Optional Hiking Route
							OP: Optional Packrafting Route
						*/
						case "RR", "RH", "RP":
							if segment.Track.Optional {
								// All regular tracks should be in the Regular Tracks folder
								return fmt.Errorf("segment %q is in Optional Tracks folder", segment.Raw)
							}
							if segment.Track.Code != segment.Code {
								// All regular tracks should be in the correct folder
								return fmt.Errorf("segment %q is in %q track folder", segment.Raw, segment.Track.Raw)
							}
						case "OH", "OP":
							if !segment.Track.Optional {
								// All optional tracks should be in the Optional Tracks folder
								return fmt.Errorf("segment %q is not in Optional Tracks folder", segment.Raw)
							}
						}
						segment.Terrain = matches[3]
						segment.Verification = matches[4]
						segment.Directional = matches[5]

						section, err := strconv.Atoi(matches[6])
						if err != nil {
							return fmt.Errorf("decoding section number from %q", segmentPlacemark.Name)
						}
						if section != segment.Track.Section.Key.Number || matches[7] != segment.Track.Section.Key.Suffix {
							// TODO: Put this error back in once Jan has updated the input files
							//fmt.Printf("%q is in %q\n", segment.Raw, segment.Track.Section.Raw)
							//return fmt.Errorf("segment %q has wrong section number", segmentPlacemark.Name)
						}

						var option int
						if matches[10] != "" {
							option, err = strconv.Atoi(matches[10])
							if err != nil {
								return fmt.Errorf("decoding section number from %q", segmentPlacemark.Name)
							}
						}
						if option != segment.Track.Option {
							// TODO: Put this error back in once Jan has updated the input files
							//fmt.Printf("incorrect option: %q is in %q\n", segment.Raw, segment.Track.Raw)
							//return fmt.Errorf("incorrect option %q is in %q", segment.Raw, segment.Track.Raw)
						}

						segment.Variant = matches[11]
						if segment.Option == 0 && segment.Variant != "" && !segment.Track.Variants {
							return fmt.Errorf("%q is not in variants folder %q", segment.Raw, segment.Track.Raw)
						}

						if matches[12] != "" {
							count, err := strconv.Atoi(matches[12])
							if err != nil {
								return fmt.Errorf("decoding count number from %q", segmentPlacemark.Name)
							}
							segment.Count = count
						}

						if matches[13] != "" {
							from, err := strconv.ParseFloat(matches[13], 64)
							if err != nil {
								return fmt.Errorf("decoding from number from %q", segmentPlacemark.Name)
							}
							segment.From = from
						}

						if matches[14] != "" {
							length, err := strconv.ParseFloat(matches[14], 64)
							if err != nil {
								return fmt.Errorf("decoding length number from %q", segmentPlacemark.Name)
							}
							segment.Length = length
						}

						segment.Name = matches[16]

						if segmentPlacemark.LineString == nil {
							segment.Line = *segmentPlacemark.MultiGeometry.LineString
						} else {
							segment.Line = *segmentPlacemark.LineString
						}
						segment.Locations = segment.Line.Locations()

					} else {
						//fmt.Printf("case %q: placemark.Name = %q\n", placemark.Name, strings.ReplaceAll(placemark.Name, "#", "-#"))
						return fmt.Errorf("no placemark regex match for %q", segmentPlacemark.Name)
					}
				}
			}
		}
	}

	// Build routes
	for _, key := range keys {
		section := sections[key]
		// build a list of segments for hiking route
		var hasHiking, hasPackrafting bool
		if section.Key.Suffix == "P" {
			hasPackrafting = true
		} else if section.Key.Suffix == "H" {
			hasHiking = true
		} else {
			hasHiking = true
		}
		// Regular sections sometimes have a hiking / packrafting route without having the prefix. We need to search
		// the tracks to find out.
		for _, track := range section.Tracks {
			// Regular sections without H or P suffix may have packrafting tracks. Scan to find out.
			if track.Code == "RP" {
				// RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
				hasPackrafting = true
			}
			if track.Code == "RH" {
				// RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
				hasHiking = true
			}
		}

		collect := func(route *Route, code string) {
			for _, track := range section.Tracks {
				// RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
				if track.Code == "RR" || track.Code == code {
					for _, segment := range track.Segments {
						route.Segments = append(route.Segments, segment)
					}
				}
			}
			// order the segments by From km
			sort.Slice(route.Segments, func(i, j int) bool { return route.Segments[i].From < route.Segments[j].From })
		}
		if hasHiking {
			section.Hiking = &Route{Section: section, Hiking: true}
			collect(section.Hiking, "RH")
		}
		if hasPackrafting {
			section.Packrafting = &Route{Section: section, Packrafting: true}
			collect(section.Packrafting, "RP")
		}
	}

	for _, section := range sections {
		if section.Hiking != nil {
			if err := section.Hiking.Normalise(); err != nil {
				return fmt.Errorf("normalising GPT%v regular hiking route: %w", section.Key.Code(), err)
			}
		}
		if section.Packrafting != nil {
			if err := section.Packrafting.Normalise(); err != nil {
				return fmt.Errorf("normalising GPT%v regular packrafting route: %w", section.Key.Code(), err)
			}
		}
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
	findClosest := func(current *Segment, from kml.Location, exclude map[*Segment]bool) (segment *Segment, start bool, dist float64, err error) {

		hardCoded := map[string]string{
			"RP-MR-V@29P-120.2+8.4": "RP-MR-V@29P-5.6+4.3",
		}

		collection := r.Segments

		if hardCoded[current.Raw] != "" {
			// we have a hardcoded next segment
			for _, s := range r.Segments {
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
			for _, segment := range r.Segments {
				if segment.Raw == special {
					return segment
				}
			}
		}

		for _, segment := range r.Segments {
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
		first.Line.Coordinates = first.Line.Reverse()
		first.Locations = first.Line.Locations()
	}

	current := first

	for len(segments) != len(r.Segments) {
		next, start, dist, err := findClosest(current, current.End(), used)
		if err != nil {
			return err
		}
		if dist > 0.05 {

			// TODO: remove special cases
			switch next.Raw {
			case "EXP-RP-RI-2@90P-152.3+7.6":
			case "RP-LK-1@37P-5.3+1.8":
			// ignore
			default:
				//fmt.Printf("closest segment to %q is %q - %.0f m away\n", current.Raw, next.Raw, dist*1000)
				return fmt.Errorf("closest segment to %q is %q - %.0f m away", current.Raw, next.Raw, dist*1000)
			}

		}
		if !start {
			// reverse the next segment
			//fmt.Printf("Reversing: %s\n", next.Raw)
			next.Line.Coordinates = next.Line.Reverse()
			next.Locations = next.Line.Locations()
		}
		used[next] = true
		segments = append(segments, next)

		if next.Raw == "RP-PR-V@29P-111.6+2.9" {
			// TODO: Remove this special case
			// There is a branch in 29P so  "RP-MR-V@29P-0.0+5.6" will be left over when we reach the end of the route
			break
		}

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
	Line         kml.LineString
	Locations    []kml.Location
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
func (s Segment) Start() kml.Location {
	return s.Locations[0]
}

// End is the last location in the kml linestring
func (s Segment) End() kml.Location {
	return s.Locations[len(s.Locations)-1]
}

var level2FolderName = regexp.MustCompile(`^GPT(\d{2})([HP]?)-(.*)$`)
var level3FolderName1 = regexp.MustCompile(`^(EXP-)?([A-Z]{2}) \((\d{4})\)$`)
var level3FolderName2 = regexp.MustCompile(`^Option (\d{1,2}) (.*) \((\d{4})\)$`)
var level3FolderName3 = regexp.MustCompile(`^Variants \((\d{4})\)$`)
var level3FolderName4 = regexp.MustCompile(`^Variants$`)
var placemarkName = regexp.MustCompile(`^(EXP-)?([A-Z]{2})-([A-Z]{2})-([VAI]?)([12]?)@(\d{2})([A-Z]?)-(((\d{2})?([A-Z]?)-)?#(\d{3})|(\d+\.\d+)\+(\d+\.\d+))( \((.*)\))?$`)
