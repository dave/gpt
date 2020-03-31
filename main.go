package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"log"
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

	root, err := loadKmz(*input)
	if err != nil {
		return fmt.Errorf("loading kmz: %w", err)
	}

	keys, sections, err := scanKml(root)
	if err != nil {
		return fmt.Errorf("scanning kml: %w", err)
	}

	if err := buildRoutes(keys, sections); err != nil {
		return fmt.Errorf("building routes: %w", err)
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

func loadKmz(fpath string) (kml.Root, error) {
	zrc, err := zip.OpenReader(fpath)
	if err != nil {
		return kml.Root{}, fmt.Errorf("opening %q: %w", fpath, err)
	}

	defer zrc.Close()

	frc, err := zrc.File[0].Open()
	if err != nil {
		return kml.Root{}, fmt.Errorf("unzipping %q: %w", fpath, err)
	}

	root, err := kml.Decode(frc)
	if err != nil {
		return kml.Root{}, fmt.Errorf("decoding kml %q: %w", fpath, err)
	}

	return root, nil
}

func scanKml(root kml.Root) ([]SectionKey, map[SectionKey]*Section, error) {
	var keys []SectionKey
	sections := map[SectionKey]*Section{}

	for _, rootFolder := range root.Document.Folders[0].Folders {
		optional := rootFolder.Name == "Optional Tracks"
		for _, sectionFolder := range rootFolder.Folders {

			// ^GPT(\d{2})([HP]?)-(PN )?(.*)$
			matches := level2FolderName.FindStringSubmatch(sectionFolder.Name)

			if len(matches) == 0 {
				return nil, nil, fmt.Errorf("section folder regex match for %q", sectionFolder.Name)
			}

			number, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, nil, fmt.Errorf("decoding section number for %q: %w", sectionFolder.Name, err)
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
						return nil, nil, fmt.Errorf("decoding year from %q - %q", trackFolder.Name, matches[3])
					}
					track.Year = year
				} else if matches := level3FolderName2.FindStringSubmatch(trackFolder.Name); len(matches) != 0 {
					// ^Option (\d{1,2}) (.*) \((\d{4}\))$
					option, err := strconv.Atoi(matches[1])
					if err != nil {
						return nil, nil, fmt.Errorf("decoding option number from %q - %q", trackFolder.Name, matches[1])
					}
					track.Option = option
					track.Name = matches[2]
					year, err := strconv.Atoi(matches[3])
					if err != nil {
						return nil, nil, fmt.Errorf("decoding year from %q - %q", trackFolder.Name, matches[3])
					}
					track.Year = year
				} else if matches := level3FolderName3.FindStringSubmatch(trackFolder.Name); len(matches) != 0 {
					// ^Varr?iants \((\d{4}\))$
					track.Variants = true
					year, err := strconv.Atoi(matches[1])
					if err != nil {
						return nil, nil, fmt.Errorf("decoding year from %q - %q", trackFolder.Name, matches[1])
					}
					track.Year = year
				} else if matches := level3FolderName4.FindStringSubmatch(trackFolder.Name); len(matches) != 0 {
					// ^Variants$
					track.Variants = true
				} else {
					return nil, nil, fmt.Errorf("no track folder regex match for %q", trackFolder.Name)
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
								return nil, nil, fmt.Errorf("segment %q is in Optional Tracks folder", segment.Raw)
							}
							if segment.Track.Code != segment.Code {
								// All regular tracks should be in the correct folder
								return nil, nil, fmt.Errorf("segment %q is in %q track folder", segment.Raw, segment.Track.Raw)
							}
						case "OH", "OP":
							if !segment.Track.Optional {
								// All optional tracks should be in the Optional Tracks folder
								return nil, nil, fmt.Errorf("segment %q is not in Optional Tracks folder", segment.Raw)
							}
						}
						segment.Terrain = matches[3]
						segment.Verification = matches[4]
						segment.Directional = matches[5]

						section, err := strconv.Atoi(matches[6])
						if err != nil {
							return nil, nil, fmt.Errorf("decoding section number from %q", segmentPlacemark.Name)
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
								return nil, nil, fmt.Errorf("decoding section number from %q", segmentPlacemark.Name)
							}
						}
						if option != segment.Track.Option {
							// TODO: Put this error back in once Jan has updated the input files
							//fmt.Printf("incorrect option: %q is in %q\n", segment.Raw, segment.Track.Raw)
							//return fmt.Errorf("incorrect option %q is in %q", segment.Raw, segment.Track.Raw)
						}

						segment.Variant = matches[11]
						if segment.Option == 0 && segment.Variant != "" && !segment.Track.Variants {
							return nil, nil, fmt.Errorf("%q is not in variants folder %q", segment.Raw, segment.Track.Raw)
						}

						if matches[12] != "" {
							count, err := strconv.Atoi(matches[12])
							if err != nil {
								return nil, nil, fmt.Errorf("decoding count number from %q", segmentPlacemark.Name)
							}
							segment.Count = count
						}

						if matches[13] != "" {
							from, err := strconv.ParseFloat(matches[13], 64)
							if err != nil {
								return nil, nil, fmt.Errorf("decoding from number from %q", segmentPlacemark.Name)
							}
							segment.From = from
						}

						if matches[14] != "" {
							length, err := strconv.ParseFloat(matches[14], 64)
							if err != nil {
								return nil, nil, fmt.Errorf("decoding length number from %q", segmentPlacemark.Name)
							}
							segment.Length = length
						}

						segment.Name = matches[16]

						var ls kml.LineString
						if segmentPlacemark.LineString == nil {
							ls = *segmentPlacemark.MultiGeometry.LineString
						} else {
							ls = *segmentPlacemark.LineString
						}
						segment.Line = ls.Line()

					} else {
						//fmt.Printf("case %q: placemark.Name = %q\n", placemark.Name, strings.ReplaceAll(placemark.Name, "#", "-#"))
						return nil, nil, fmt.Errorf("no placemark regex match for %q", segmentPlacemark.Name)
					}
				}
			}
		}
	}
	return keys, sections, nil
}

func buildRoutes(keys []SectionKey, sections map[SectionKey]*Section) error {
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
		// optional routes
		section.Optional = map[OptionalKey]*Route{}
		for _, track := range section.Tracks {
			if !track.Optional {
				continue
			}
			for _, segment := range track.Segments {
				key := OptionalKey{track.Option, segment.Variant}
				if section.Optional[key] == nil {
					section.Optional[key] = &Route{
						Section:  section,
						Optional: key,
						Name:     track.Name,
					}
				}
				section.Optional[key].Segments = append(section.Optional[key].Segments, segment)
			}
		}
	}

	for _, key := range keys {
		section := sections[key]
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
		/*
			Optional routes can't be normalised (not continuous enough)
			for key, route := range section.Optional {
				if err := route.Normalise(); err != nil {
					return fmt.Errorf("normalising GPT%v optional route %s: %w", section.Key.Code(), key.Code(), err)
				}
			}
		*/
	}
	return nil
}

var level2FolderName = regexp.MustCompile(`^GPT(\d{2})([HP]?)-(.*)$`)
var level3FolderName1 = regexp.MustCompile(`^(EXP-)?([A-Z]{2}) \((\d{4})\)$`)
var level3FolderName2 = regexp.MustCompile(`^Option (\d{1,2}) (.*) \((\d{4})\)$`)
var level3FolderName3 = regexp.MustCompile(`^Variants \((\d{4})\)$`)
var level3FolderName4 = regexp.MustCompile(`^Variants$`)
var placemarkName = regexp.MustCompile(`^(EXP-)?([A-Z]{2})-([A-Z]{2})-([VAI]?)([12]?)@(\d{2})([A-Z]?)-(((\d{2})?([A-Z]?)-)?#(\d{3})|(\d+\.\d+)\+(\d+\.\d+))( \((.*)\))?$`)
