package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/dave/gpt/kml"
)

func scanKml(tracksRoot, pointsRoot kml.Root, elevation bool) (*Data, error) {
	data := &Data{
		Keys:      nil,
		Sections:  map[SectionKey]*Section{},
		Waypoints: map[SectionKey][]Waypoint{},
	}

	//var startEndPoints, resupplies, important, waypoints *kml.Folder
	for _, folder := range pointsRoot.Document.Folders[0].Folders {
		switch folder.Name {
		case "Section Start and End Points":
			for _, inner := range folder.Folders {
				switch inner.Name {
				case "Regular Routes":
					for _, p := range inner.Placemarks {
						matches := regularNodeName.FindStringSubmatch(p.Name)
						if len(matches) != 4 {
							return nil, fmt.Errorf("parsing regular route start/end point %q", p.Name)
						}
						r := SectionNode{
							Name: matches[3],
							Pos:  p.Point.Pos(),
						}
						for _, s := range strings.Split(matches[1], "/") {
							key, err := NewSectionKey(s)
							if err != nil {
								return nil, fmt.Errorf("parsting section key %q from %q: %w", s, p.Name, err)
							}
							r.Sections = append(r.Sections, key)
						}
						data.Nodes = append(data.Nodes, r)
					}
				case "Optional Routes":
					for _, p := range inner.Placemarks {
						matches := optionsNodeName.FindStringSubmatch(p.Name)
						if len(matches) != 5 {
							return nil, fmt.Errorf("parsing optional route start/end point %q", p.Name)
						}
						r := SectionNode{
							Name:   matches[4],
							Pos:    p.Point.Pos(),
							Option: matches[3],
						}
						if r.Option == "" {
							// any optional node without a option code should be A?
							r.Option = "A"
						}
						for _, s := range strings.Split(matches[1], "/") {
							key, err := NewSectionKey(s)
							if err != nil {
								return nil, fmt.Errorf("parsting section key %q from %q: %w", s, p.Name, err)
							}
							r.Sections = append(r.Sections, key)
						}
						data.Nodes = append(data.Nodes, r)
					}
				}

			}
		case "Resupply Locations":
			for _, p := range folder.Placemarks {
				data.Resupplies = append(data.Resupplies, Waypoint{
					Pos:  p.Point.Pos(),
					Name: p.Name,
				})
			}
		case "Important Infromation", "Important Information":
			for _, p := range folder.Placemarks {
				data.Important = append(data.Important, Waypoint{
					Pos:  p.Point.Pos(),
					Name: p.Name,
				})
			}
		case "Waypoints by Section":
			for _, f := range folder.Folders {
				matches := waypointSectionFolderName.FindStringSubmatch(f.Name)
				if len(matches) != 2 {
					return nil, fmt.Errorf("parsing waypoint folder name %q", f.Name)
				}
				key, err := NewSectionKey(matches[1])
				if err != nil {
					return nil, fmt.Errorf("parsing section key from %q: %w", matches[1], err)
				}
				for _, p := range f.Placemarks {
					data.Waypoints[key] = append(data.Waypoints[key], Waypoint{
						Pos:  p.Point.Pos(),
						Name: strings.TrimSuffix(p.Name, "-"), // all waypoint names end with "-"?
					})
				}
			}
		}
	}

	for _, rootFolder := range tracksRoot.Document.Folders[0].Folders {
		optional := rootFolder.Name == "Optional Tracks"
		for _, sectionFolder := range rootFolder.Folders {

			// ^GPT(\d{2})([HP]?)-(PN )?(.*)$
			matches := level2FolderName.FindStringSubmatch(sectionFolder.Name)

			if len(matches) == 0 {
				return nil, fmt.Errorf("section folder regex match for %q", sectionFolder.Name)
			}

			number, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, fmt.Errorf("decoding section number for %q: %w", sectionFolder.Name, err)
			}
			suffix := matches[2]
			key := SectionKey{number, suffix}

			if data.Sections[key] == nil {
				data.Keys = append(data.Keys, key)
				data.Sections[key] = &Section{
					Raw:  sectionFolder.Name,
					Key:  SectionKey{number, suffix},
					Name: matches[3],
				}
			}

			section := data.Sections[key]

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

					// TODO: Remove this special case
					if section.Key.Code() == "24H" && !track.Optional {
						// Tracks in 24P are RH but should be RR
						track.Code = "RR"
					}

					year, err := strconv.Atoi(matches[3])
					if err != nil {
						return nil, fmt.Errorf("decoding year from %q - %q", trackFolder.Name, matches[3])
					}
					track.Year = year
				} else if matches := level3FolderName2.FindStringSubmatch(trackFolder.Name); len(matches) != 0 {
					// ^Option (\d{1,2}) (.*) \((\d{4}\))$
					option, err := strconv.Atoi(matches[1])
					if err != nil {
						return nil, fmt.Errorf("decoding option number from %q - %q", trackFolder.Name, matches[1])
					}
					track.Option = option
					track.Name = matches[2]
					year, err := strconv.Atoi(matches[3])
					if err != nil {
						return nil, fmt.Errorf("decoding year from %q - %q", trackFolder.Name, matches[3])
					}
					track.Year = year
				} else if matches := level3FolderName3.FindStringSubmatch(trackFolder.Name); len(matches) != 0 {
					// ^Varr?iants \((\d{4}\))$
					track.Variants = true
					year, err := strconv.Atoi(matches[1])
					if err != nil {
						return nil, fmt.Errorf("decoding year from %q - %q", trackFolder.Name, matches[1])
					}
					track.Year = year
				} else if matches := level3FolderName4.FindStringSubmatch(trackFolder.Name); len(matches) != 0 {
					// ^Variants$
					track.Variants = true
				} else {
					return nil, fmt.Errorf("no track folder regex match for %q", trackFolder.Name)
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

						// TODO: Remove this special case
						if section.Key.Code() == "24H" && !track.Optional {
							// Tracks in 24P are RH but should be RR
							segment.Code = "RR"
						}

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
								return nil, fmt.Errorf("segment %q is in Optional Tracks folder", segment.Raw)
							}
							if segment.Track.Code != segment.Code {
								// All regular tracks should be in the correct folder
								return nil, fmt.Errorf("segment %q is in %q track folder", segment.Raw, segment.Track.Raw)
							}
						case "OH", "OP":
							if !segment.Track.Optional {
								// All optional tracks should be in the Optional Tracks folder
								return nil, fmt.Errorf("segment %q is not in Optional Tracks folder", segment.Raw)
							}
						}
						segment.Terrain = matches[3]
						if segment.Terrain != "" {
							desc := Terrain(segment.Terrain)
							if desc == "" {
								return nil, fmt.Errorf("unexpected terrain code %q in %q", segment.Terrain, segment.Raw)
							}
						}
						segment.Verification = matches[4]
						if segment.Verification != "" {
							desc := Verification(segment.Verification)
							if desc == "" {
								return nil, fmt.Errorf("unexpected verification code %q in %q", segment.Verification, segment.Raw)
							}
						}
						segment.Directional = matches[5]
						if segment.Directional != "" {
							desc := Directional(segment.Directional)
							if desc == "" {
								return nil, fmt.Errorf("unexpected directional code %q in %q", segment.Directional, segment.Raw)
							}
						}

						section, err := strconv.Atoi(matches[6])
						if err != nil {
							return nil, fmt.Errorf("decoding section number from %q", segmentPlacemark.Name)
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
								return nil, fmt.Errorf("decoding section number from %q", segmentPlacemark.Name)
							}
						}
						if option != segment.Track.Option {
							// TODO: Put this error back in once Jan has updated the input files
							//fmt.Printf("incorrect option: %q is in %q\n", segment.Raw, segment.Track.Raw)
							//return fmt.Errorf("incorrect option %q is in %q", segment.Raw, segment.Track.Raw)
						}

						segment.Variant = matches[11]
						if segment.Option == 0 && segment.Variant != "" && !segment.Track.Variants {
							return nil, fmt.Errorf("%q is not in variants folder %q", segment.Raw, segment.Track.Raw)
						}

						if matches[12] != "" {
							count, err := strconv.Atoi(matches[12])
							if err != nil {
								return nil, fmt.Errorf("decoding count number from %q", segmentPlacemark.Name)
							}
							segment.Count = count
						}

						if matches[13] != "" {
							from, err := strconv.ParseFloat(matches[13], 64)
							if err != nil {
								return nil, fmt.Errorf("decoding from number from %q", segmentPlacemark.Name)
							}
							segment.From = from
						}

						if matches[14] != "" {
							length, err := strconv.ParseFloat(matches[14], 64)
							if err != nil {
								return nil, fmt.Errorf("decoding length number from %q", segmentPlacemark.Name)
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

						if elevation {
							// lookup elevations
							for i := range segment.Line {
								elevation, err := SrtmClient.GetElevation(http.DefaultClient, segment.Line[i].Lat, segment.Line[i].Lon)
								if err != nil {
									panic(err.Error())
								}
								segment.Line[i].Ele = elevation
							}
						}

					} else {
						//fmt.Printf("case %q: placemark.Name = %q\n", placemark.Name, strings.ReplaceAll(placemark.Name, "#", "-#"))
						return nil, fmt.Errorf("no placemark regex match for %q", segmentPlacemark.Name)
					}
				}
			}
		}
	}
	return data, nil
}

var level2FolderName = regexp.MustCompile(`^GPT(\d{2})([HP]?)-(.*)$`)
var level3FolderName1 = regexp.MustCompile(`^(EXP-)?([A-Z]{2}) \((\d{4})\)$`)
var level3FolderName2 = regexp.MustCompile(`^Option (\d{1,2}) (.*) \((\d{4})\)$`)
var level3FolderName3 = regexp.MustCompile(`^Variants \((\d{4})\)$`)
var level3FolderName4 = regexp.MustCompile(`^Variants$`)
var placemarkName = regexp.MustCompile(`^(EXP-)?([A-Z]{2})-([A-Z]{2})-([VAI]?)([12]?)@(\d{2})([A-Z]?)-(((\d{2})?([A-Z]?)-)?#(\d{3})|(\d+\.\d+)\+(\d+\.\d+))( \((.*)\))?$`)
var regularNodeName = regexp.MustCompile(`^([0-9/GTHP]+) (.*)?\((.*)\)$`)
var optionsNodeName = regexp.MustCompile(`^([0-9/GTHP]+)(-([A-Z]))? \((.*)\)$`)
var waypointSectionFolderName = regexp.MustCompile(`^([0-9/GTHP]+)-.*$`)
