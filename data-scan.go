package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/dave/gpt/kml"
)

func (d *Data) Scan(inputRoot kml.Root, elevation bool) error {

	var tracksFolder, pointsFolder *kml.Folder
	for _, folder := range inputRoot.Document.Folders[0].Folders {
		switch folder.Name {
		case "Tracks":
			tracksFolder = folder
		case "Points":
			pointsFolder = folder
		}
	}

	for _, rootFolder := range tracksFolder.Folders {
		optional := rootFolder.Name == "Optional Tracks"
		for _, sectionFolder := range rootFolder.Folders {

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

			if d.Sections[key] == nil {
				d.Keys = append(d.Keys, key)
				d.Sections[key] = &Section{
					Raw:  sectionFolder.Name,
					Key:  SectionKey{number, suffix},
					Name: matches[3],
				}
			}

			section := d.Sections[key]

			for _, trackFolder := range sectionFolder.Folders {

				track := &Track{
					Raw:      trackFolder.Name,
					Section:  section,
					Optional: optional,
				}
				section.Tracks = append(section.Tracks, track)

				if matches := level3FolderName1.FindStringSubmatch(trackFolder.Name); len(matches) != 0 {
					//debugfln("%#v", matches)
					track.Experimental = matches[1] == "EXP-"
					track.Code = matches[2]
					track.Direction = matches[4]

					year, err := strconv.Atoi(matches[5])
					if err != nil {
						return fmt.Errorf("decoding year from %q - %q", trackFolder.Name, matches[5])
					}
					track.Year = year
				} else if matches := level3FolderName2.FindStringSubmatch(trackFolder.Name); len(matches) != 0 {
					option, err := strconv.Atoi(matches[1])
					if err != nil {
						return fmt.Errorf("decoding option number from %q - %q", trackFolder.Name, matches[1])
					}
					track.Option = option
					track.Name = matches[2]
					if matches[4] != "" {
						year, err := strconv.Atoi(matches[4])
						if err != nil {
							return fmt.Errorf("decoding year from %q - %q", trackFolder.Name, matches[3])
						}
						track.Year = year
					}
				} else if matches := level3FolderName3.FindStringSubmatch(trackFolder.Name); len(matches) != 0 {
					track.Variants = true
					if matches[2] != "" {
						year, err := strconv.Atoi(matches[2])
						if err != nil {
							return fmt.Errorf("decoding year from %q - %q", trackFolder.Name, matches[2])
						}
						track.Year = year
					}
				} else {
					//debugfln("no track folder regex match for %q", trackFolder.Name)
					//return fmt.Errorf("no track folder regex match for %q", trackFolder.Name)
				}
				for _, segmentPlacemark := range trackFolder.Placemarks {

					segment := &Segment{
						Raw:   segmentPlacemark.Name,
						Track: track,
					}

					switch segmentPlacemark.Name {

					// double space
					case "RP-FJ-2@29P(S)-107.7+5.3  (Fiordo Aysen)":
						segmentPlacemark.Name = "RP-FJ-2@29P(S)-107.7+5.3 (Fiordo Aysen)"
					case "OP-FJ-2@28P-01-#002  (Fiordo Pitipalena Brazo Pillan)":
						segmentPlacemark.Name = "OP-FJ-2@28P-01-#002 (Fiordo Pitipalena Brazo Pillan)"
					case "OP-RI-2@28P-04-#001  (Rio Melimoyu)":
						segmentPlacemark.Name = "OP-RI-2@28P-04-#001 (Rio Melimoyu)"
					case "OP-FJ-2@28P-05B-#002  (Estero del Medio)":
						segmentPlacemark.Name = "OP-FJ-2@28P-05B-#002 (Estero del Medio)"
					case "OP-FJ-2@90P-01-#001  (Fiordo Aysen)":
						segmentPlacemark.Name = "OP-FJ-2@90P-01-#001 (Fiordo Aysen)"

					// missing length
					case "RP-LK-2@31P-180.8+ (Lago Riesco)":
						segmentPlacemark.Name = "RP-LK-2@31P-180.8+0.0 (Lago Riesco)"

					case "RP-LK-2@35-53.4+5.2\n (Lago Jeinemeni)":
						segmentPlacemark.Name = "RP-LK-2@35-53.4+5.2 (Lago Jeinemeni)"

					case "Untitled Path", "Untitled Path (Lago Zenteno)":
						continue

					case "OH-MR-V", "OH-PR-V", "OH-TL-V":
						continue

					case "OP-FJ-2@28P-06-#0010 (Canal Puyuhuapi)":
						segmentPlacemark.Name = "OP-FJ-2@28P-06-#010 (Canal Puyuhuapi)"

					case "OH-TL-V@33H-08-#005A":
						segmentPlacemark.Name = "OH-TL-V@33H-08A-#005"

					case "OP-MR-V @33H-11E-#001":
						segmentPlacemark.Name = "OP-MR-V@33H-11E-#001"
					case "OP-MR-V@35-02-$00":
						segmentPlacemark.Name = "OP-MR-V@35-02-#002"

					case "OP-TL&BB@36P-02-#009":
						segmentPlacemark.Name = "OP-TL&BB-V@36P-02-#009"

					case "RH-TL&CC-I@76-B-#001":
						segmentPlacemark.Name = "OH-TL&CC-I@76-B-#001"

					case "LK-2@91P-01-#007 (Lago Gualas)":
						segmentPlacemark.Name = "OP-LK-2@91P-01-#007 (Lago Gualas)"

						// wrong section codes
					case "RP-RI-1@36H- (Rio Cisnes)":
						segmentPlacemark.Name = "RP-RI-1@36P- (Rio Cisnes)" // has wrong section number in "GPT36P-Rio Baker"
					case "RP-LK-2@36H- (Lago Ciervo)":
						segmentPlacemark.Name = "RP-LK-2@36P- (Lago Ciervo)" // has wrong section number in "GPT36P-Rio Baker"
					case "RP-RI-1@36H- (Rio Mayer)":
						segmentPlacemark.Name = "RP-RI-1@36P- (Rio Mayer)" // has wrong section number in "GPT36P-Rio Baker"
					case "OH-TL-V@34P-01A-#001":
						segmentPlacemark.Name = "OH-TL-V@33H-06A-#001" // has wrong section number in "GPT33H-Torres de Avellano" AND OPTION NUMBER
					case "OH-TL-V@34P-01B-#001":
						segmentPlacemark.Name = "OH-TL-V@33H-06B-#001" // has wrong section number in "GPT33H-Torres de Avellano" AND OPTION NUMBER
					case "OH-TL-V@34P-01C-#001":
						segmentPlacemark.Name = "OH-TL-V@33H-06C-#001" // has wrong section number in "GPT33H-Torres de Avellano" AND OPTION NUMBER
					case "OH-TL-V@34P-01E-#001":
						segmentPlacemark.Name = "OH-TL-V@33H-06E-#001" // has wrong section number in "GPT33H-Torres de Avellano" AND OPTION NUMBER

					// Broken options:
					case "OH-MR-V@32-", "OH-MR-I@32-", "OH-CC-I@32-", "OH-TL-I@32-", "OH-TL-V@32-":
						continue

					case "OP-CC-A@36H-C-#002":
						segmentPlacemark.Name = "OP-CC-A@36H-11C-#002" //"Option 11 Rio Salto (2018)"
					case "OP-LK-2@36H-C-#003 (Laguna Esmeralda)":
						segmentPlacemark.Name = "OP-LK-2@36H-11C-#003 (Laguna Esmeralda)" //"Option 11 Rio Salto (2018)"
					case "OP-RI-1@36P-02-#001 (Rio Bravo)":
						segmentPlacemark.Name = "OP-RI-1@36P-03-#001 (Rio Bravo)" //"Option 3 Rio Bravo (2019)"
					case "OP-FJ-2@36P-02-#002 (Fiordo Mitchell)":
						segmentPlacemark.Name = "OP-FJ-2@36P-03-#002 (Fiordo Mitchell)" //"Option 3 Rio Bravo (2019)"

					}

					if matches := placemarkName.FindStringSubmatch(segmentPlacemark.Name); len(matches) == 0 {
						//debugf("no placemark regex match for %q in %q %q\n", segmentPlacemark.Name, section.String(), track.String())
						return fmt.Errorf("no placemark regex match for %q in %q %q", segmentPlacemark.Name, section.String(), track.String())
					} else {
						//debugfln("%#v", matches)
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
						segment.Terrains = strings.Split(matches[3], "&")
						for _, terrain := range segment.Terrains {
							desc := Terrain(terrain)
							if desc == "" {
								return fmt.Errorf("unexpected terrain code %q in %q", terrain, segment.Raw)
							}
						}
						segment.Verification = matches[4]
						if segment.Verification != "" {
							desc := Verification(segment.Verification)
							if desc == "" {
								return fmt.Errorf("unexpected verification code %q in %q", segment.Verification, segment.Raw)
							}
						}
						segment.Directional = matches[5]
						if segment.Directional != "" {
							desc := Directional(segment.Directional)
							if desc == "" {
								return fmt.Errorf("unexpected directional code %q in %q", segment.Directional, segment.Raw)
							}
						}

						key, err := NewSectionKey(matches[6] + matches[7])
						if err != nil {
							return fmt.Errorf("decoding section number from %q: %w", segmentPlacemark.Name, err)
						}
						if key.Number != segment.Track.Section.Key.Number || key.Suffix != segment.Track.Section.Key.Suffix {
							return fmt.Errorf("segment %q has wrong section number in %q", segmentPlacemark.Name, segment.Track.Section.String())
						}

						var option int
						if matches[12] != "" {
							option, err = strconv.Atoi(matches[12])
							if err != nil {
								return fmt.Errorf("decoding section number from %q", segmentPlacemark.Name)
							}
						}
						if option != segment.Track.Option {
							return fmt.Errorf("incorrect option %q is in %q", segment.Raw, segment.Track.Raw)
						}

						segment.Variant = matches[13]
						if segment.Option == 0 && segment.Variant != "" && !segment.Track.Variants {
							return fmt.Errorf("%q is not in variants folder %q", segment.Raw, segment.Track.Raw)
						}

						if matches[14] != "" {
							count, err := strconv.Atoi(matches[14])
							if err != nil {
								return fmt.Errorf("decoding count number from %q", segmentPlacemark.Name)
							}
							segment.Count = count
						}

						if matches[16] != "" {
							from, err := strconv.ParseFloat(matches[16], 64)
							if err != nil {
								return fmt.Errorf("decoding from number %q from %q", matches[16], segmentPlacemark.Name)
							}
							segment.From = from
						} else {
							segment.From = 999 // to stop these from being treated as segment start points
						}

						// Removed length from name - always calculate (below).
						/*
							if matches[15] != "" {
								length, err := strconv.ParseFloat(matches[15], 64)
								if err != nil {
									return fmt.Errorf("decoding length number %q from %q", matches[15], segmentPlacemark.Name)
								}
								segment.Length = length
							}
						*/

						segment.Name = matches[19]

						var ls kml.LineString
						if segmentPlacemark.LineString == nil {
							ls = *segmentPlacemark.MultiGeometry.LineStrings[0]
						} else {
							ls = *segmentPlacemark.LineString
						}
						segment.Line = ls.Line()
						segment.Length = ls.Line().Length()

						if elevation {
							logf("Getting elevations for %s\n", segment.String())
							// lookup elevations
							for i := range segment.Line {
								elevation, err := SrtmClient.GetElevation(http.DefaultClient, segment.Line[i].Lat, segment.Line[i].Lon)
								if err != nil {
									return fmt.Errorf("looking up elevation for %q: %w", segment.Raw, err)
								}
								segment.Line[i].Ele = elevation
							}
						}
					}
					track.Segments = append(track.Segments, segment)
				}
			}
		}
	}

	// TODO: remove this
	// Waypoints are incorrectly tagged as GPT70P / GPT71P / GPT72P - the track is GPT70 / GPT71 / GPT72
	removeSuffix := map[int]bool{
		70: true,
		71: true,
		72: true,
		76: true,
		77: true,
		78: true,
	}

	for _, folder := range pointsFolder.Folders {
		switch folder.Name {
		case "Section Start and End Points":
			for _, inner := range folder.Folders {
				switch inner.Name {
				case "Regular Routes":
					for _, p := range inner.Placemarks {
						matches := regularTerminatorName.FindStringSubmatch(p.Name)
						if len(matches) != 4 {
							return fmt.Errorf("parsing regular route start/end point %q", p.Name)
						}
						r := Terminator{
							Raw:  p.Name,
							Name: matches[3],
							Pos:  p.Point.Pos(),
						}
						for _, s := range strings.Split(matches[1], "/") {
							key, err := NewSectionKey(s)
							if err != nil {
								return fmt.Errorf("parsing section key %q from %q: %w", s, p.Name, err)
							}
							if removeSuffix[key.Number] {
								key.Suffix = ""
							}
							r.Sections = append(r.Sections, key)
						}
						d.Terminators = append(d.Terminators, r)
					}
				case "Optional Routes":
					for _, p := range inner.Placemarks {
						matches := optionsTerminatorName.FindStringSubmatch(p.Name)
						if len(matches) != 5 {
							return fmt.Errorf("parsing optional route start/end point %q", p.Name)
						}
						r := Terminator{
							Raw:    p.Name,
							Name:   matches[4],
							Pos:    p.Point.Pos(),
							Option: matches[3],
						}
						if r.Option == "" {
							// any optional terminator without a option code should be A?
							r.Option = "A"
						}
						for _, s := range strings.Split(matches[1], "/") {
							key, err := NewSectionKey(s)
							if err != nil {
								return fmt.Errorf("parsing section key %q from %q: %w", s, p.Name, err)
							}
							if removeSuffix[key.Number] {
								key.Suffix = ""
							}
							r.Sections = append(r.Sections, key)
						}
						d.Terminators = append(d.Terminators, r)
					}
				}

			}
		case "Resupply Locations":
			for _, p := range folder.Placemarks {
				d.Resupplies = append(d.Resupplies, Waypoint{
					Pos:  p.Point.Pos(),
					Name: p.Name,
				})
			}
		case "Important Infromation", "Important Information":
			for _, p := range folder.Placemarks {
				d.Important = append(d.Important, Waypoint{
					Pos:  p.Point.Pos(),
					Name: p.Name,
				})
			}
		case "Waypoints by Section":
			for _, f := range folder.Folders {
				matches := waypointSectionFolderName.FindStringSubmatch(f.Name)
				if len(matches) != 2 {
					return fmt.Errorf("parsing waypoint folder name %q", f.Name)
				}
				key, err := NewSectionKey(matches[1])
				if err != nil {
					return fmt.Errorf("parsing section key from %q: %w", matches[1], err)
				}
				if removeSuffix[key.Number] {
					key.Suffix = ""
				}
				for _, p := range f.Placemarks {
					d.Sections[key].Waypoints = append(d.Sections[key].Waypoints, Waypoint{
						Pos: p.Point.Pos(),

						// TODO: remove this?
						Name: strings.TrimSuffix(p.Name, "-"), // all waypoint names end with "-"?
					})
				}
			}
		}
	}

	if elevation {
		waypointElevations := func(waypoints []Waypoint) error {
			for i, w := range waypoints {
				elevation, err := SrtmClient.GetElevation(http.DefaultClient, w.Lat, w.Lon)
				if err != nil {
					return fmt.Errorf("looking up waypoint elevation: %w", err)
				}
				waypoints[i].Ele = elevation
			}
			return nil
		}
		logln("Getting elevations for resupplies waypoints")
		if err := waypointElevations(d.Resupplies); err != nil {
			return err
		}
		logln("Getting elevations for important waypoints")
		if err := waypointElevations(d.Important); err != nil {
			return err
		}
		for _, section := range d.Sections {
			logf("Getting elevations for GPT%s\n", section.Key.Code())
			if err := waypointElevations(section.Waypoints); err != nil {
				return err
			}
		}
		logln("Getting elevations for section start/end waypoints")
		for i, terminator := range d.Terminators {
			elevation, err := SrtmClient.GetElevation(http.DefaultClient, terminator.Lat, terminator.Lon)
			if err != nil {
				return fmt.Errorf("looking up terminator elevation: %w", err)
			}
			d.Terminators[i].Ele = elevation
		}
	}

	return nil
}

var level2FolderName = regexp.MustCompile(`^GPT(\d{2})([HP]?)-(.*)$`)
var level3FolderName1 = regexp.MustCompile(`^(EXP-)?([A-Z]{2})(\(?([NS])\)?)? \((\d{4})\)$`)
var level3FolderName2 = regexp.MustCompile(`^Option (\d{1,2}) ([^(]*)( \((\d{4})\))?$`)
var level3FolderName3 = regexp.MustCompile(`^Variants( \((\d{4})\))?$`)
var placemarkName = regexp.MustCompile(`^(EXP-)?([A-Z]{2})-([A-Z&]{2,})-([VAI]?)([12]?)@(\d{2})([PH]?)(\(([NS])\))?-(((\d{2})?([A-Z]?)-)?#(\d{3})|((\d+\.\d+)\+(\d+\.\d+))?)( \((.*)\))?$`)
var regularTerminatorName = regexp.MustCompile(`^([0-9/GTHP]+) (.*)?\((.*)\)$`)
var optionsTerminatorName = regexp.MustCompile(`^([0-9/GTHP]+)(-([A-Z]))? \((.*)\)$`)
var waypointSectionFolderName = regexp.MustCompile(`^([0-9/GTHP]+)-.*$`)
