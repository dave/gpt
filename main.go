package main

import (
	"archive/zip"
	"encoding/xml"
	"flag"
	"fmt"
	"gpt/kml"
	"log"
	"regexp"
	"sort"
	"strconv"
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

	fmt.Println(zrc.File[0].Name)
	frc, err := zrc.File[0].Open()
	if err != nil {
		return fmt.Errorf("unzipping %q: %w", *input, err)
	}

	var root kml.Root
	if err := xml.NewDecoder(frc).Decode(&root); err != nil {
		return fmt.Errorf("decoding %q: %w", *input, err)
	}

	var sections []*Section

	for _, rootFolder := range root.Document.Folders[0].Folders {
		optional := rootFolder.Name == "Optional Tracks"
		for _, sectionFolder := range rootFolder.Folders {

			section := &Section{
				Raw:      sectionFolder.Name,
				Optional: optional,
			}
			sections = append(sections, section)

			// ^GPT(\d{2})([HP]?)-(PN )?(.*)$
			matches := level2FolderName.FindStringSubmatch(sectionFolder.Name)

			if len(matches) == 0 {
				return fmt.Errorf("section folder regex match for %q", sectionFolder.Name)
			}

			id, err := strconv.Atoi(matches[1])
			if err != nil {
				return fmt.Errorf("decoding section number for %q: %w", sectionFolder.Name, err)
			}
			section.Section = id
			section.Suffix = matches[2]
			section.Name = matches[3]

			for _, routeFolder := range sectionFolder.Folders {

				switch routeFolder.Name {
				case "Varriants (2018)":
					routeFolder.Name = "Variants (2018)"
				case "Option 1 (Puerto Montt)":
					routeFolder.Name = "Option 1 Puerto Montt (0000)"
				case "Option 2 (Quellon)":
					routeFolder.Name = "Option 2 Quellon (0000)"
				}

				route := &Route{
					Raw:     routeFolder.Name,
					Section: section,
				}
				section.Routes = append(section.Routes, route)

				if matches := level3FolderName1.FindStringSubmatch(routeFolder.Name); len(matches) != 0 {
					// ^(EXP-)?([A-Z]{2}) \((\d{4})\)$
					route.Experimental = matches[1] == "EXP-"
					route.Code = matches[2]
					year, err := strconv.Atoi(matches[3])
					if err != nil {
						return fmt.Errorf("decoding year from %q - %q", routeFolder.Name, matches[3])
					}
					route.Year = year
				} else if matches := level3FolderName2.FindStringSubmatch(routeFolder.Name); len(matches) != 0 {
					// ^Option (\d{1,2}) (.*) \((\d{4}\))$
					option, err := strconv.Atoi(matches[1])
					if err != nil {
						return fmt.Errorf("decoding option number from %q - %q", routeFolder.Name, matches[1])
					}
					route.Option = option
					route.Name = matches[2]
					year, err := strconv.Atoi(matches[3])
					if err != nil {
						return fmt.Errorf("decoding year from %q - %q", routeFolder.Name, matches[3])
					}
					route.Year = year
				} else if matches := level3FolderName3.FindStringSubmatch(routeFolder.Name); len(matches) != 0 {
					// ^Varr?iants \((\d{4}\))$
					route.Variants = true
					year, err := strconv.Atoi(matches[1])
					if err != nil {
						return fmt.Errorf("decoding year from %q - %q", routeFolder.Name, matches[1])
					}
					route.Year = year
				} else if matches := level3FolderName4.FindStringSubmatch(routeFolder.Name); len(matches) != 0 {
					// ^Variants$
					route.Variants = true
				} else {
					return fmt.Errorf("no route folder regex match for %q", routeFolder.Name)
				}
				for _, segmentPlacemark := range routeFolder.Placemarks {

					segment := &Segment{
						Raw:   segmentPlacemark.Name,
						Route: route,
					}
					route.Segments = append(route.Segments, segment)

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
							if segment.Route.Section.Optional {
								// All regular routes should be in the Regular Tracks folder
								return fmt.Errorf("segment %q is in Optional Tracks folder", segment.Raw)
							}
							if segment.Route.Code != segment.Code {
								// All regular routes should be in the correct folder
								return fmt.Errorf("segment %q is in %q route folder", segment.Raw, segment.Route.Raw)
							}
						case "OH", "OP":
							if !segment.Route.Section.Optional {
								// All optional routes should be in the Optional Tracks folder
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
						if section != segment.Route.Section.Section || matches[7] != segment.Route.Section.Suffix {
							// TODO: Put this error back in once Jan has updated the input files
							//fmt.Printf("%q is in %q\n", segment.Raw, segment.Route.Section.Raw)
							//return fmt.Errorf("segment %q has wrong section number", segmentPlacemark.Name)
						}

						var option int
						if matches[10] != "" {
							option, err = strconv.Atoi(matches[10])
							if err != nil {
								return fmt.Errorf("decoding section number from %q", segmentPlacemark.Name)
							}
						}
						if option != segment.Route.Option {
							// TODO: Put this error back in once Jan has updated the input files
							//fmt.Printf("incorrect option: %q is in %q\n", segment.Raw, segment.Route.Raw)
							//return fmt.Errorf("incorrect option %q is in %q", segment.Raw, segment.Route.Raw)
						}

						segment.Variant = matches[11]
						if segment.Option == 0 && segment.Variant != "" && !segment.Route.Variants {
							return fmt.Errorf("%q is not in variants folder %q", segment.Raw, segment.Route.Raw)
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

	for _, section := range sections {
		if section.Section == 1 && !section.Optional {
			for _, route := range section.Routes {
				for i, segment := range route.Segments {
					if len(route.Segments) == 0 {
						break
					}
					if i == 0 {
						// special case for first segment... it might be reversed.
						// work out distance between start and end points of segment 0 and 1
						s1 := segment.Locations
						s2 := route.Segments[i+1].Locations
						ary := []struct {
							start1, start2 bool
							dist           float64
						}{
							{true, true, s1[0].Distance(s2[0])},
							{true, false, s1[0].Distance(s2[len(s2)-1])},
							{false, true, s1[len(s1)-1].Distance(s2[0])},
							{false, false, s1[len(s1)-1].Distance(s2[len(s2)-1])},
						}
						//for _, s := range ary {
						//	fmt.Printf("%#v\n", s)
						//}
						sort.Slice(ary, func(i, j int) bool { return ary[i].dist < ary[j].dist })
						if ary[0].start1 {
							// first segment is reversed
							fmt.Printf("segment %d: %v is reversed\n", i, segment.Raw)
							segment.Line.Coordinates = segment.Line.Reverse()
							segment.Locations = segment.Line.Locations()
						}
					} else {
						s1 := route.Segments[i-1].Locations
						s2 := segment.Locations
						start := s1[len(s1)-1].Distance(s2[0])
						end := s1[len(s1)-1].Distance(s2[len(s2)-1])
						if end < start {
							// this segment is reversed
							fmt.Printf("segment %d: %v is reversed\n", i, segment.Raw)
							segment.Line.Coordinates = segment.Line.Reverse()
							segment.Locations = segment.Line.Locations()
						}
					}
					//fmt.Println(segment.Raw, len(segment.Line.Locations()))
				}
				//fmt.Println(route.Raw)
			}
		}
	}

	fmt.Println("gpt", root.Document.Name)
	return nil
}

// Section is a section folder
type Section struct {
	Raw      string // raw name of the section folder
	Optional bool   // is this section in the "Optional Tracks" folder?
	Section  int    // section number
	Suffix   string // section number suffix e.g. H = Hiking, P = Packrafting
	Name     string // name of the section
	Routes   []*Route
}

// Route is a route folder inn a section folder
type Route struct {
	*Section
	Raw          string // raw name of the route folder
	Experimental bool   // route folder has "EXP-" prefix
	Code         string // route type code - RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
	Year         int    // year in brackets in the route folder
	Variants     bool   // route folder is named "Variants"
	Option       int    // option number if the route folder is named "Option X"
	Name         string // route name for optional routes
	Segments     []*Segment
}

// Segment is a placemark / linestring in a route folder
type Segment struct {
	*Route
	Raw          string  // raw name of the placemark
	Experimental bool    // segment name has "EXP-" prefix
	Code         string  // route code from the segment name - RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
	Terrain      string  // terrain code from segment name - BB: Bush Bashing, CC: Cross Country, MR: Minor Road, PR: Primary or Paved Road, TL: Horse or Hiking Trail, FJ: Fjord Packrafting, LK: Lake Packrafting, RI: River Packrafting
	Verification string  // verification status - V: Verified Route, A: Approximate Route, I: Investigation Route
	Directional  string  // directional status - 1: One-Way Route, 2: Two-Way Route
	Variant      string  // variant from segment name
	Count        int     // counter for optional routes
	From         float64 // from km for regular routes
	Length       float64 // length km for regular routes
	Name         string  // named feature
	Line         kml.LineString
	Locations    []kml.Location
}

var level2FolderName = regexp.MustCompile(`^GPT(\d{2})([HP]?)-(.*)$`)
var level3FolderName1 = regexp.MustCompile(`^(EXP-)?([A-Z]{2}) \((\d{4})\)$`)
var level3FolderName2 = regexp.MustCompile(`^Option (\d{1,2}) (.*) \((\d{4})\)$`)
var level3FolderName3 = regexp.MustCompile(`^Variants \((\d{4})\)$`)
var level3FolderName4 = regexp.MustCompile(`^Variants$`)
var placemarkName = regexp.MustCompile(`^(EXP-)?([A-Z]{2})-([A-Z]{2})-([VAI]?)([12]?)@(\d{2})([A-Z]?)-(((\d{2})?([A-Z]?)-)?#(\d{3})|(\d+\.\d+)\+(\d+\.\d+))( \((.*)\))?$`)
