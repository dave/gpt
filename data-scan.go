package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/dave/gpt/geo"
	"github.com/dave/gpt/kml"
)

type Mode int

const HIKING Mode = 1
const PACKRAFTING Mode = 2

var MODES = []Mode{HIKING, PACKRAFTING}

var elevationCache = map[geo.Pos]float64{}

func (d *Data) Scan(inputRoot kml.Root, elevation bool) error {

	folders := inputRoot.Document.Folders
	if len(folders) == 1 && folders[0].Name == "GPT Master" {
		folders = folders[0].Folders
	}

	var tracksFolder, pointsFolder *kml.Folder
	for _, folder := range folders {
		switch folder.Name {
		case "Tracks":
			tracksFolder = folder
		case "Points":
			pointsFolder = folder
		}
	}
	if tracksFolder == nil {
		return fmt.Errorf("input file doesn't contain a folder called 'Tracks'")
	}
	if pointsFolder == nil {
		return fmt.Errorf("input file doesn't contain a folder called 'Points'")
	}

	for _, optionalRegularFolder := range tracksFolder.Folders {

		var optional bool
		switch optionalRegularFolder.Name {
		case "Optional Tracks":
			optional = true
		case "Regular Tracks":
			optional = false
		default:
			return fmt.Errorf("incorrect name in %q", optionalRegularFolder.Name)
		}

		for _, sectionFolder := range optionalRegularFolder.Folders {
			matches := sectionFolderRegex.FindStringSubmatch(sectionFolder.Name)
			if len(matches) == 0 {
				return fmt.Errorf("incorrect format for section folder %q", sectionFolder.Name)
			}

			number, err := strconv.Atoi(matches[1])
			if err != nil {
				return fmt.Errorf("decoding section number for %q: %w", sectionFolder.Name, err)
			}
			suffix := matches[2]
			name := matches[3]

			key := SectionKey{number, suffix}

			if HAS_SINGLE && key != SINGLE {
				continue
			}

			if d.Sections[key] == nil {
				d.Keys = append(d.Keys, key)
				d.Sections[key] = &Section{
					Raw:  sectionFolder.Name,
					Key:  key,
					Name: name,

					Hiking:      nil,
					Packrafting: nil,
					Waypoints:   nil,
				}
			} else {
				if sectionFolder.Name != d.Sections[key].Raw {
					return fmt.Errorf("regular / optional section name mismatch %q and %q", sectionFolder.Name, d.Sections[key].Raw)
				}
			}

			section := d.Sections[key]

			if !optional {
				var routes []RegularKey
				folders := map[RegularKey]*kml.Folder{}
				for _, folder := range sectionFolder.Folders {
					if folder.Name == "Southbound" {
						key := RegularKey{Direction: "S"}
						routes = append(routes, key)
						folders[key] = folder
					} else if folder.Name == "Northbound" {
						key := RegularKey{Direction: "N"}
						routes = append(routes, key)
						folders[key] = folder
					}
				}
				if len(routes) == 0 {
					key := RegularKey{Direction: ""}
					routes = append(routes, key)
					folders[key] = sectionFolder
				}
				for _, regularKey := range routes {
					folder := folders[regularKey]

					for _, mode := range MODES {

						type AlternativeMode int
						const NORMAL AlternativeMode = 1
						const HIKING_ALTERNATIVES AlternativeMode = 2
						var ALTERNATIVES_MODES []AlternativeMode

						if mode == HIKING {
							ALTERNATIVES_MODES = []AlternativeMode{NORMAL}
						} else {
							ALTERNATIVES_MODES = []AlternativeMode{NORMAL, HIKING_ALTERNATIVES}
						}

						for _, altMode := range ALTERNATIVES_MODES {

							optionalKey := OptionalKey{}
							regular := true

							if altMode == HIKING_ALTERNATIVES {
								optionalKey.Alternatives = true
								optionalKey.Direction = regularKey.Direction
								optionalKey.AlternativesIndex = 0
								regularKey = RegularKey{}
								regular = false
							}

							var routes []*Route
							route := &Route{
								Section:     section,
								Hiking:      mode == HIKING,
								Packrafting: mode == PACKRAFTING,
								Regular:     regular,
								RegularKey:  regularKey,
								OptionalKey: optionalKey,
								Name:        "",
								Segments:    nil,
							}
							var prev *Segment
							for _, placemark := range folder.Placemarks {
								codes := map[string]bool{}
								if altMode == HIKING_ALTERNATIVES {
									codes["RH"] = true
								} else if mode == PACKRAFTING {
									codes["RR"] = true
									codes["RP"] = true
								} else if mode == HIKING {
									codes["RR"] = true
									codes["RH"] = true
								}

								segment, err := getSegment(route, placemark, elevation, codes)
								if err != nil {
									return err
								}
								if segment == nil {
									continue
								}
								if prev != nil {
									// check that segment adjoins prev.
									adjoins := prev.Line.End().IsClose(segment.Line.Start(), DELTA)
									if !adjoins {
										if altMode != HIKING_ALTERNATIVES {
											return fmt.Errorf("segments %q and %q in %q are not joined", prev.Raw, segment.Raw, route.Debug())
										}
										if len(route.Segments) > 0 {
											routes = append(routes, route)
										}
										optionalKey = OptionalKey{
											Alternatives:      true,
											Direction:         optionalKey.Direction,
											AlternativesIndex: optionalKey.AlternativesIndex + 1,
										}
										route = &Route{
											Section:     section,
											Hiking:      mode == HIKING,
											Packrafting: mode == PACKRAFTING,
											Regular:     regular,
											RegularKey:  regularKey,
											OptionalKey: optionalKey,
											Name:        "",
											Segments:    nil,
										}
									}
								}
								prev = segment
								route.Segments = append(route.Segments, segment)
							}
							if len(route.Segments) == 0 {
								continue
							}
							routes = append(routes, route)

							for _, route := range routes {

								route.Network = &Network{
									Route: route,
									Entry: route.Segments[0],
								}
								if altMode == HIKING_ALTERNATIVES {
									if section.Packrafting == nil {
										section.Packrafting = &Bundle{
											Regular: map[RegularKey]*Route{},
											Options: map[OptionalKey]*Route{},
										}
									}
									if section.Packrafting.Options[route.OptionalKey] != nil {
										fmt.Printf("%#v\n", route.OptionalKey)
										return fmt.Errorf("duplicate route HA %q in %q", route.Raw, route.Section.Raw)
									}
									section.Packrafting.Options[route.OptionalKey] = route
								} else if mode == PACKRAFTING {
									if section.Packrafting == nil {
										section.Packrafting = &Bundle{
											Regular: map[RegularKey]*Route{},
											Options: map[OptionalKey]*Route{},
										}
									}
									if section.Packrafting.Regular[route.RegularKey] != nil {
										return fmt.Errorf("duplicate route PR %q in %q", route.Raw, route.Section.Raw)
									}
									section.Packrafting.Regular[route.RegularKey] = route
								} else if mode == HIKING {
									if section.Hiking == nil {
										section.Hiking = &Bundle{
											Regular: map[RegularKey]*Route{},
											Options: map[OptionalKey]*Route{},
										}
									}
									if section.Hiking.Regular[route.RegularKey] != nil {
										return fmt.Errorf("duplicate route HR %q in %q", route.Raw, route.Section.Raw)
									}
									section.Hiking.Regular[route.RegularKey] = route
								}
							}

						}
					}
				}
			} else {
				for _, optionVariantsFolder := range sectionFolder.Folders {
					routes := map[OptionalKey]bool{}
					var optionNumber int
					switch {
					case optionVariantsFolder.Name == "Variants":
						optionNumber = 0
					case strings.HasPrefix(optionVariantsFolder.Name, "Option"):
						num, err := strconv.Atoi(strings.TrimPrefix(optionVariantsFolder.Name, "Option "))
						if err != nil {
							return fmt.Errorf("parsing option number from %q in %q: %w", optionVariantsFolder.Name, section.Raw, err)
						}
						optionNumber = num
					default:
						return fmt.Errorf("incorrect folder name %q in %q", optionVariantsFolder.Name, section.Raw)
					}
					for _, routeFolder := range optionVariantsFolder.Folders {
						matches := routeFolderRegex.FindStringSubmatch(routeFolder.Name)

						if len(matches) == 0 {
							return fmt.Errorf("incorrect name format %q in %q in %q", routeFolder.Name, optionVariantsFolder.Name, section.Raw)
						}

						variantCode := matches[2]
						networkCode := matches[3]
						routeName := matches[5]

						optionalKey := OptionalKey{
							Option:  optionNumber,
							Variant: variantCode,
							Network: networkCode,
						}
						if routes[optionalKey] {
							return fmt.Errorf("duplicate variant %q in %q in %q in %q", variantCode, routeFolder.Name, optionVariantsFolder.Name, section.Raw)
						}
						routes[optionalKey] = true

						for _, mode := range MODES {
							route := &Route{
								Section:     section,
								Hiking:      mode == HIKING,
								Packrafting: mode == PACKRAFTING,
								Regular:     false,
								RegularKey:  RegularKey{},
								OptionalKey: optionalKey,
								Name:        routeName,
								Segments:    nil,
							}
							for _, segmentPlacemark := range routeFolder.Placemarks {
								codes := map[string]bool{}
								switch mode {
								case HIKING:
									codes["OH"] = true
								case PACKRAFTING:
									codes["OH"] = true
									codes["OP"] = true
								}
								segment, err := getSegment(route, segmentPlacemark, elevation, codes)
								if err != nil {
									return err
								}
								if segment == nil {
									continue
								}
								route.Segments = append(route.Segments, segment)
							}
							if len(route.Segments) == 0 {
								continue
							}
							route.Network = &Network{
								Route: route,
								Entry: route.Segments[0],
							}
							if mode == PACKRAFTING {
								if section.Packrafting == nil {
									section.Packrafting = &Bundle{
										Regular: map[RegularKey]*Route{},
										Options: map[OptionalKey]*Route{},
									}
								}
								if section.Packrafting.Options[optionalKey] != nil {
									return fmt.Errorf("duplicate route PO %q in %q", route.Raw, route.Section.Raw)
								}
								section.Packrafting.Options[optionalKey] = route
							} else if mode == HIKING {
								if section.Hiking == nil {
									section.Hiking = &Bundle{
										Regular: map[RegularKey]*Route{},
										Options: map[OptionalKey]*Route{},
									}
								}
								if section.Hiking.Options[optionalKey] != nil {
									return fmt.Errorf("duplicate route HO %q in %q", route.Raw, route.Section.Raw)
								}
								section.Hiking.Options[optionalKey] = route
							}
						}
					}
				}
			}
		}
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
		case "Important Information":
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

func getSegment(route *Route, placemark *kml.Placemark, elevation bool, codes map[string]bool) (*Segment, error) {
	matches := segmentPlacemarkRegex.FindStringSubmatch(placemark.Name)

	if len(matches) == 0 {
		return nil, fmt.Errorf("unknown format in placemark %q", placemark.Name)
	}

	if placemark.GetLineString() == nil {
		return nil, fmt.Errorf("placemark %q has no line string", placemark.Name)
	}

	if !codes[matches[3]] {
		return nil, nil
	}

	segment := &Segment{
		Route:        route,
		Raw:          placemark.Name,
		Experimental: matches[2] == "EXP",
		Code:         matches[3],
		Terrains:     strings.Split(matches[4], "&"),
		Verification: matches[7],
		Directional:  matches[8],
		From:         0, // calculated later
		Length:       placemark.GetLineString().Line().Length(),
		Name:         matches[10],
		Line:         placemark.GetLineString().Line(),
		StartPoint:   nil,
		EndPoint:     nil,
		MidPoints:    nil,
	}

	if elevation {
		logf("Getting elevations for %s\n", segment.String())
		for i := range segment.Line {
			// SrtmClient.GetElevation is slow even when cached, so we make our own cache.
			pos := geo.Pos{Lat: segment.Line[i].Lat, Lon: segment.Line[i].Lon}
			ele, found := elevationCache[pos]
			if !found {
				var err error
				ele, err = SrtmClient.GetElevation(http.DefaultClient, pos.Lat, pos.Lon)
				if err != nil {
					return nil, fmt.Errorf("looking up elevation for %q: %w", segment.Raw, err)
				}
				elevationCache[pos] = ele
			}
			segment.Line[i].Ele = ele
		}
	}
	return segment, nil
}

var sectionFolderRegex = regexp.MustCompile(`^GPT([0-9]{2})([HP])? \((.*)\)$`)
var segmentPlacemarkRegex = regexp.MustCompile(`^((EXP)-)?(RH|RP|RR|OH|OP)-(((BB|CC|MR|PR|TL|FJ|LK|RI|FY)&?)+)-?([VAI])?([12])? {.*} \[.*] ?(\((.*)\))?$`)
var routeFolderRegex = regexp.MustCompile(`^([0-9]{2})?([A-Z]{1,2})?([a-z])? ?(\((.*)\))?$`)

var regularTerminatorName = regexp.MustCompile(`^([0-9/GTHP]+) (.*)?\((.*)\)$`)
var optionsTerminatorName = regexp.MustCompile(`^([0-9/GTHP]+)(-([A-Z]))? \((.*)\)$`)
var waypointSectionFolderName = regexp.MustCompile(`^([0-9/GTHP]+)-.*$`)
