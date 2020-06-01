package main

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/dave/gpt/geo"
	"github.com/dave/gpt/kml"
)

var elevationCache = map[geo.Pos]float64{}

type AlternativeType int

const NORMAL AlternativeType = 1
const HIKING_ALTERNATIVES AlternativeType = 2

var ALTERNATIVE_TYPES = []AlternativeType{NORMAL, HIKING_ALTERNATIVES}

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

		var required RequiredType
		switch optionalRegularFolder.Name {
		case "Optional Tracks":
			logln("scanning optional tracks")
			required = OPTIONAL
		case "Regular Tracks":
			logln("scanning regular tracks")
			required = REGULAR
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
			name := strings.TrimSpace(matches[3])

			sectionKey := SectionKey{number, suffix}

			if HAS_SINGLE && sectionKey != SINGLE {
				continue
			}

			if d.Sections[sectionKey] == nil {
				d.Keys = append(d.Keys, sectionKey)
				d.Sections[sectionKey] = &Section{
					Raw:       sectionFolder.Name,
					Key:       sectionKey,
					Name:      name,
					Routes:    map[RouteKey]*Route{},
					Waypoints: nil,
					Scraped:   map[ModeType]string{},
				}
			} else {
				if sectionFolder.Name != d.Sections[sectionKey].Raw {
					return fmt.Errorf("regular / optional section name mismatch %q and %q", sectionFolder.Name, d.Sections[sectionKey].Raw)
				}
			}

			section := d.Sections[sectionKey]

			if required == REGULAR {
				var routeKeys []RouteKey
				routeFolders := map[RouteKey]*kml.Folder{}
				for _, folder := range sectionFolder.Folders {
					if folder.Name == "Southbound" {
						key := RouteKey{Required: REGULAR, Direction: "S"}
						routeKeys = append(routeKeys, key)
						routeFolders[key] = folder
					} else if folder.Name == "Northbound" {
						key := RouteKey{Required: REGULAR, Direction: "N"}
						routeKeys = append(routeKeys, key)
						routeFolders[key] = folder
					}
				}
				if len(routeKeys) == 0 {
					key := RouteKey{Required: REGULAR, Direction: ""}
					routeKeys = append(routeKeys, key)
					routeFolders[key] = sectionFolder
				}
				for _, rkey := range routeKeys {
					folder := routeFolders[rkey]

					for _, alternativeType := range ALTERNATIVE_TYPES {

						if alternativeType == HIKING_ALTERNATIVES {
							rkey.Required = OPTIONAL
							rkey.Alternatives = true
							rkey.AlternativesIndex = 1
						}

						var routes []*Route
						route := &Route{
							Section: section,
							Key:     rkey,
							Name:    "",
							All:     []*Segment{},
							Modes:   map[ModeType]*RouteModeData{},
						}

						if alternativeType != HIKING_ALTERNATIVES {
							for _, placemark := range folder.Placemarks {
								segment, err := getSegment(route, placemark, map[string]bool{"RR": true, "RP": true, "RH": true})
								if err != nil {
									return err
								}
								if segment == nil {
									continue
								}
								route.All = append(route.All, segment)
							}
						}

						var modesInUse []ModeType
						if alternativeType == HIKING_ALTERNATIVES {
							modesInUse = []ModeType{RAFT}
						} else {
							modesInUse = []ModeType{HIKE, RAFT}
						}

						for _, mode := range modesInUse {

							codes := map[string]bool{}
							if alternativeType == HIKING_ALTERNATIVES {
								codes["RH"] = true
							} else if mode == RAFT {
								codes["RR"] = true
								codes["RP"] = true
							} else if mode == HIKE {
								codes["RR"] = true
								codes["RH"] = true
							}

							var prev *Segment
							for _, placemark := range folder.Placemarks {
								segment, err := getSegment(route, placemark, codes)
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
										if alternativeType != HIKING_ALTERNATIVES {
											return fmt.Errorf("segments %q and %q in %q are not joined", prev.Raw, segment.Raw, route.Debug())
										}
										if route.Modes[RAFT] != nil {
											for _, s := range route.Modes[RAFT].Segments {
												route.All = append(route.All, s)
											}
										}
										if len(route.All) > 0 {
											routes = append(routes, route)
										}
										newKey := RouteKey{
											Required:          OPTIONAL,
											Direction:         route.Key.Direction,
											Alternatives:      true,
											AlternativesIndex: route.Key.AlternativesIndex + 1,
										}
										route = &Route{
											Section: section,
											Key:     newKey,
											Name:    "",
											All:     []*Segment{},
											Modes:   map[ModeType]*RouteModeData{},
										}
									}
								}
								prev = segment
								if route.Modes[mode] == nil {
									route.Modes[mode] = &RouteModeData{}
								}
								if segment.Modes[mode] == nil {
									segment.Modes[mode] = &SegmentModeData{}
								}
								route.Modes[mode].Segments = append(route.Modes[mode].Segments, segment)
							}
						}
						if route.Key.Alternatives && route.Modes[RAFT] != nil {
							// special case for hiking alternatives routes
							for _, s := range route.Modes[RAFT].Segments {
								route.All = append(route.All, s)
							}
						}
						if len(route.All) == 0 {
							continue
						}
						routes = append(routes, route)

						for _, route := range routes {
							for _, mode := range MODES {
								if route.Modes[mode] != nil {
									route.Modes[mode].Network = &Network{
										Mode:          mode,
										Route:         route,
										RouteModeData: route.Modes[mode],
										Entry:         route.Modes[mode].Segments[0],
									}
								}
							}
							if section.Routes[route.Key] != nil {
								return fmt.Errorf("duplicate regular route %q in %q", route.Debug(), route.Section.Raw)
							}
							section.RouteKeys = append(section.RouteKeys, route.Key)
							section.Routes[route.Key] = route
						}
					}
				}
			} else {
				for _, optionVariantsFolder := range sectionFolder.Folders {
					routes := map[RouteKey]bool{}
					var optionNumber int
					var optionName string
					switch {
					case optionVariantsFolder.Name == "Variants":
						optionNumber = 0
					case strings.HasPrefix(optionVariantsFolder.Name, "Option"):
						matches := optionFolderRegex.FindStringSubmatch(optionVariantsFolder.Name)
						if len(matches) == 0 {
							return fmt.Errorf("incorrect name format %q in %q", optionVariantsFolder.Name, section.Raw)
						}
						num, err := strconv.Atoi(matches[1])
						if err != nil {
							return fmt.Errorf("parsing option number from %q in %q: %w", optionVariantsFolder.Name, section.Raw, err)
						}
						optionNumber = num
						optionName = matches[3]
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

						rkey := RouteKey{
							Required: OPTIONAL,
							Option:   optionNumber,
							Variant:  variantCode,
							Network:  networkCode,
						}
						if routes[rkey] {
							return fmt.Errorf("duplicate variant %q in %q in %q in %q", variantCode, routeFolder.Name, optionVariantsFolder.Name, section.Raw)
						}
						routes[rkey] = true

						route := &Route{
							Section: section,
							Key:     rkey,
							Name:    routeName,
							Option:  optionName,
							All:     []*Segment{},
							Modes:   map[ModeType]*RouteModeData{},
						}

						for _, placemark := range routeFolder.Placemarks {
							segment, err := getSegment(route, placemark, map[string]bool{"OH": true, "OP": true})
							if err != nil {
								return err
							}
							if segment == nil {
								continue
							}
							route.All = append(route.All, segment)
						}

						for _, mode := range MODES {
							for _, segmentPlacemark := range routeFolder.Placemarks {
								codes := map[string]bool{}
								switch mode {
								case HIKE:
									codes["OH"] = true
								case RAFT:
									codes["OH"] = true
									codes["OP"] = true
								}
								segment, err := getSegment(route, segmentPlacemark, codes)
								if err != nil {
									return err
								}
								if segment == nil {
									continue
								}
								if route.Modes[mode] == nil {
									route.Modes[mode] = &RouteModeData{}
								}
								if segment.Modes[mode] == nil {
									segment.Modes[mode] = &SegmentModeData{}
								}
								route.Modes[mode].Segments = append(route.Modes[mode].Segments, segment)
							}
						}
						if len(route.All) == 0 {
							continue
						}
						for _, mode := range MODES {
							if route.Modes[mode] != nil {
								route.Modes[mode].Network = &Network{
									Mode:          mode,
									Route:         route,
									RouteModeData: route.Modes[mode],
									Entry:         route.Modes[mode].Segments[0],
								}
							}
						}
						if section.Routes[route.Key] != nil {
							return fmt.Errorf("duplicate optional route %q in %q", route.Debug(), route.Section.Raw)
						}
						section.RouteKeys = append(section.RouteKeys, route.Key)
						section.Routes[route.Key] = route
					}
				}
			}
		}
	}

	sort.Slice(d.Keys, func(i, j int) bool {
		return d.Keys[i].Code() < d.Keys[j].Code()
	})

	if elevation {
		logln("looking up track elevations")
		for _, sectionKey := range d.Keys {
			section := d.Sections[sectionKey]
			for _, routeKey := range section.RouteKeys {
				route := section.Routes[routeKey]
				for _, segment := range route.All {
					for i := range segment.Line {
						pos := geo.Pos{Lat: segment.Line[i].Lat, Lon: segment.Line[i].Lon}
						ele, found := elevationCache[pos]
						if !found {
							var err error
							ele, err = SrtmClient.GetElevation(http.DefaultClient, pos.Lat, pos.Lon)
							if err != nil {
								return fmt.Errorf("looking up elevation for %q: %w", segment.Raw, err)
							}
							elevationCache[pos] = ele
						}
						segment.Line[i].Ele = ele
					}
				}
			}
		}
	}

	logln("scanning waypoints")
	for _, folder := range pointsFolder.Folders {
		switch folder.Name {
		//case "Start and Finish Points":
		//	for _, inner := range folder.Folders {
		//		switch inner.Name {
		//		case "Regular Routes":
		//			for _, p := range inner.Placemarks {
		//				matches := regularTerminatorName.FindStringSubmatch(p.Name)
		//				if len(matches) != 4 {
		//					return fmt.Errorf("parsing regular route start/end point %q", p.Name)
		//				}
		//				r := Terminator{
		//					Raw:  p.Name,
		//					Name: matches[3],
		//					Pos:  p.Point.Pos(),
		//				}
		//				for _, s := range strings.Split(matches[1], "/") {
		//					key, err := NewSectionKey(s)
		//					if err != nil {
		//						return fmt.Errorf("parsing section key %q from %q: %w", s, p.Name, err)
		//					}
		//					r.Sections = append(r.Sections, key)
		//				}
		//				d.Terminators = append(d.Terminators, r)
		//			}
		//		case "Optional Routes":
		//			for _, p := range inner.Placemarks {
		//				matches := optionsTerminatorName.FindStringSubmatch(p.Name)
		//				if len(matches) != 5 {
		//					return fmt.Errorf("parsing optional route start/end point %q", p.Name)
		//				}
		//				r := Terminator{
		//					Raw:    p.Name,
		//					Name:   matches[4],
		//					Pos:    p.Point.Pos(),
		//					Option: matches[3],
		//				}
		//				if r.Option == "" {
		//					// any optional terminator without a option code should be A?
		//					r.Option = "A"
		//				}
		//				for _, s := range strings.Split(matches[1], "/") {
		//					key, err := NewSectionKey(s)
		//					if err != nil {
		//						return fmt.Errorf("parsing section key %q from %q: %w", s, p.Name, err)
		//					}
		//					r.Sections = append(r.Sections, key)
		//				}
		//				d.Terminators = append(d.Terminators, r)
		//			}
		//		}
		//	}
		case "Geographic Designations":
			for _, p := range folder.Placemarks {
				if p.Point == nil {
					continue
				}
				d.Geographic = append(d.Geographic, Waypoint{
					Pos:  p.Point.Pos(),
					Name: p.Name,
				})
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
			for _, folder := range folder.Folders {
				matches := sectionFolderRegex.FindStringSubmatch(folder.Name)
				if len(matches) == 0 {
					return fmt.Errorf("incorrect format for waypoint section folder %q", folder.Name)
				}
				number, err := strconv.Atoi(matches[1])
				if err != nil {
					return fmt.Errorf("decoding section number for %q: %w", folder.Name, err)
				}
				suffix := matches[2]
				name := strings.TrimSpace(matches[3])
				sectionKey := SectionKey{number, suffix}
				section := d.Sections[sectionKey]
				if name != section.Name {
					return fmt.Errorf("waypoint section name mismatch in GPT%s %q and %q", section.Key.Code(), name, section.Name)
				}
				for _, p := range folder.Placemarks {
					section.Waypoints = append(section.Waypoints, Waypoint{
						Pos:  p.Point.Pos(),
						Name: strings.TrimSuffix(p.Name, "-"), // all waypoint names end with "-"?
					})
				}
				for _, f := range folder.Folders {
					for _, p := range f.Placemarks {
						section.Waypoints = append(section.Waypoints, Waypoint{
							Pos:    p.Point.Pos(),
							Name:   strings.TrimSuffix(p.Name, "-"), // all waypoint names end with "-"?
							Folder: f.Name,
						})
					}
				}
			}
		}
	}

	if elevation {
		logln("looking up waypoint elevations")
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
		if err := waypointElevations(d.Resupplies); err != nil {
			return err
		}
		if err := waypointElevations(d.Important); err != nil {
			return err
		}
		for _, section := range d.Sections {
			if err := waypointElevations(section.Waypoints); err != nil {
				return err
			}
		}
	}

	return nil
}

var segmentCache = map[*kml.Placemark]*Segment{}

func getSegment(route *Route, placemark *kml.Placemark, codes map[string]bool) (*Segment, error) {

	if segment, ok := segmentCache[placemark]; ok {
		if !codes[segment.Code] {
			return nil, nil
		}
		return segment, nil
	}

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
		Legacy:       placemark.Legacy,
		Experimental: matches[2] == "EXP",
		Code:         matches[3],
		Terrains:     strings.Split(matches[4], "&"),
		Verification: matches[7],
		Directional:  matches[8],
		Length:       placemark.GetLineString().Line().Length(),
		Name:         matches[10],
		Line:         placemark.GetLineString().Line(),
		Modes:        map[ModeType]*SegmentModeData{},
	}

	segmentCache[placemark] = segment

	return segment, nil
}

var optionFolderRegex = regexp.MustCompile(`Option ([0-9]+) ?(\((.*)\))?$`)
var sectionFolderRegex = regexp.MustCompile(`^GPT([0-9]{2})([HP])? \((.*)\)$`)
var segmentPlacemarkRegex = regexp.MustCompile(`^((EXP)-)?(RH|RP|RR|OH|OP)-(((BB|CC|MR|PR|TL|FJ|LK|RI|FY)&?)+)-?([VAI])?([12])? {.*} \[.*] ?(\((.*)\))?$`)
var routeFolderRegex = regexp.MustCompile(`^([0-9]{2})?([A-Z]{1,2})?([a-z])? ?(\((.*)\))?$`)
