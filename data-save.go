package main

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dave/gpt/geo"
	"github.com/dave/gpt/gpx"
	"github.com/dave/gpt/kml"
)

type LegacyRenameHolder struct {
	update    bool
	segments  []struct{ from, to string }
	waypoints []struct{ from, to string }
}

func (r *LegacyRenameHolder) segment(legacy, name string) string {
	if !r.update {
		return legacy
	}
	if legacy != name {
		r.segments = append(r.segments, struct{ from, to string }{legacy, name})
	}
	return name
}

func (r *LegacyRenameHolder) waypoint(legacy, name string) string {
	if !r.update {
		return legacy
	}
	if legacy != name {
		r.waypoints = append(r.waypoints, struct{ from, to string }{legacy, name})
	}
	return name
}

func (d *Data) SaveMaster(dpath string, updateLegacy bool) error {
	logln("saving kml master")
	legacy := &LegacyRenameHolder{update: updateLegacy}

	tracksFolder := &kml.Folder{Name: "Tracks"}

	for _, required := range REQUIRED_TYPES {
		rootFolder := &kml.Folder{}
		switch required {
		case REGULAR:
			rootFolder.Name = "Regular Tracks"
		case OPTIONAL:
			rootFolder.Name = "Optional Tracks"
		}
		tracksFolder.Folders = append(tracksFolder.Folders, rootFolder)
		for _, sectionKey := range d.Keys {
			section := d.Sections[sectionKey]
			sectionFolder := &kml.Folder{Name: section.FolderName()}
			rootFolder.Folders = append(rootFolder.Folders, sectionFolder)
			optionalRouteHolderFolders := map[int]*kml.Folder{} // option number

			for _, routeKey := range section.RouteKeys {
				route := section.Routes[routeKey]
				if route.Key.Alternatives {
					// all hiking alternatives are already in the regular routes
					continue
				}
				if route.Key.Required != required {
					continue
				}
				var routeFolder *kml.Folder
				switch route.Key.Required {
				case REGULAR:
					switch route.Key.Direction {
					case "S":
						routeFolder = &kml.Folder{Name: "Southbound"}
						sectionFolder.Folders = append(sectionFolder.Folders, routeFolder)
					case "N":
						routeFolder = &kml.Folder{Name: "Northbound"}
						sectionFolder.Folders = append(sectionFolder.Folders, routeFolder)
					default:
						routeFolder = sectionFolder
					}
				case OPTIONAL:
					var routeHolderFolder *kml.Folder
					if f := optionalRouteHolderFolders[route.Key.Option]; f != nil {
						routeHolderFolder = f
					} else {
						if route.Key.Option == 0 {
							routeHolderFolder = &kml.Folder{Name: "Variants"}
						} else {
							var name string
							if route.Option != "" {
								name = fmt.Sprintf("Option %d (%s)", route.Key.Option, route.Option)
							} else {
								name = fmt.Sprintf("Option %d", route.Key.Option)
							}
							routeHolderFolder = &kml.Folder{Name: name}
						}
						optionalRouteHolderFolders[route.Key.Option] = routeHolderFolder
						sectionFolder.Folders = append(sectionFolder.Folders, routeHolderFolder)
					}
					routeFolder = &kml.Folder{Name: route.FolderName()}
					routeHolderFolder.Folders = append(routeHolderFolder.Folders, routeFolder)
				default:
					panic("")
				}
				for _, segment := range route.All {
					routeFolder.Placemarks = append(routeFolder.Placemarks, &kml.Placemark{
						Name:       segment.PlacemarkName(),
						Legacy:     legacy.segment(segment.Legacy, segment.PlacemarkName()),
						Visibility: 1,
						Open:       0,
						StyleUrl:   fmt.Sprintf("#%s", segment.Style()),
						LineString: &kml.LineString{
							Tessellate:  true,
							Coordinates: kml.LineCoordinates(segment.Line),
						},
					})
				}
			}
		}
	}

	//
	//sort.Slice(rf.Folders, func(i, j int) bool {
	//	return rf.Folders[i].Name < rf.Folders[j].Name
	//})

	//	sort.Slice(sec.Folders, func(i, j int) bool {
	//		extractInt := func(s string) int {
	//			var i int
	//			if s != "Variants" {
	//				i, _ = strconv.Atoi(strings.TrimPrefix(s, "Option "))
	//			}
	//			return i
	//		}
	//		return extractInt(sec.Folders[i].Name) < extractInt(sec.Folders[j].Name)
	//	})
	//
	//sort.Slice(of.Folders, func(i, j int) bool {
	//	return of.Folders[i].Name < of.Folders[j].Name
	//})

	regularStartEndFolder, optionalStartEndFolder, resupplyFolder, geographicFolder, importantFolder, waypointsFolder := d.getWaypointFolders(legacy)

	pointsFolder := &kml.Folder{
		Name: "Points",
		Folders: []*kml.Folder{
			importantFolder,
			waypointsFolder,
			regularStartEndFolder,
			optionalStartEndFolder,
			resupplyFolder,
			geographicFolder,
		},
	}

	doc := kml.Document{
		Name:    "GPT Master.kmz",
		Folders: []*kml.Folder{tracksFolder, pointsFolder},
	}
	addSegmentStyles(&doc)

	root := kml.Root{
		Xmlns:    "http://www.opengis.net/kml/2.2",
		Document: doc,
	}
	if err := root.Save(filepath.Join(dpath, "GPT Master.kmz")); err != nil {
		return fmt.Errorf("saving master: %w", err)
	}

	if updateLegacy {
		var sbSegments strings.Builder
		for _, rename := range legacy.segments {
			sbSegments.WriteString(fmt.Sprintf("%q, %q\n", rename.from, rename.to))
		}
		if err := ioutil.WriteFile(filepath.Join(dpath, "segment-renames.txt"), []byte(sbSegments.String()), 0666); err != nil {
			return fmt.Errorf("writing segment renames file: %w", err)
		}

		var sbWaypoints strings.Builder
		for _, rename := range legacy.waypoints {
			sbWaypoints.WriteString(fmt.Sprintf("%q, %q\n", rename.from, rename.to))
		}
		if err := ioutil.WriteFile(filepath.Join(dpath, "waypoint-renames.txt"), []byte(sbWaypoints.String()), 0666); err != nil {
			return fmt.Errorf("writing waypoint renames file: %w", err)
		}
	}
	return nil
}

func (d *Data) getWaypointFolders(legacy *LegacyRenameHolder) (regularStartEndFolder, optionalStartEndFolder, resupplyFolder, geographicFolder, importantFolder, waypointsFolder *kml.Folder) {

	collect := func(waypoints []Waypoint, style string) []*kml.Placemark {
		var placemarks []*kml.Placemark
		for _, w := range waypoints {
			placemarks = append(placemarks, &kml.Placemark{
				Visibility: 1,
				Open:       0,
				StyleUrl:   style,
				Name:       w.Name,
				Legacy:     legacy.waypoint(w.Legacy, w.Name),
				Point:      kml.PosPoint(w.Pos),
			})
		}
		return placemarks
	}

	/*
		// startFolder: go
		// geographicFolder: wht-blank
		// waypointsFolder: ylw-blank
		// importantFolder: red-stars
		// resupplyFolder: ylw-circle
	*/

	importantFolder = &kml.Folder{
		Name:       "Important Information",
		Placemarks: collect(d.Important, "#red-stars"),
	}
	resupplyFolder = &kml.Folder{
		Name:       "Resupply Locations",
		Placemarks: collect(d.Resupplies, "#ylw-circle"),
	}
	geographicFolder = &kml.Folder{
		Name:       "Geographic Designations",
		Placemarks: collect(d.Geographic, "#wht-blank"),
	}
	regularStartEndFolder = &kml.Folder{
		Name: "Regular Start / End Points",
	}
	optionalStartEndFolder = &kml.Folder{
		Name: "Optional Start / End Points",
	}

	for _, key := range d.Keys {
		section := d.Sections[key]
		var routes []*Route
		for _, routeKey := range section.RouteKeys {
			routes = append(routes, section.Routes[routeKey])
		}
		for _, route := range routes {
			var folder *kml.Folder
			var nameFormat string
			if route.Key.Required == REGULAR {
				folder = regularStartEndFolder
				nameFormat = fmt.Sprintf("GPT%s%s%%s", section.Key.Code(), route.Key.Direction)
				//nameFormat = fmt.Sprintf("GPT%s%s%%s (%s)", section.Key.Code(), route.Key.Direction, section.Name)
			} else {
				if route.Key.Alternatives {
					continue
				}
				folder = optionalStartEndFolder
				nameFormat = fmt.Sprintf("%s%%s", route.String())
				//if route.Name == "" {
				//	nameFormat = fmt.Sprintf("%s%%s", route.String())
				//} else {
				//	nameFormat = fmt.Sprintf("%s%%s (%s)", route.String(), route.Name)
				//}
			}
			if route.Modes[RAFT] != nil && route.Modes[HIKE] != nil && !route.Modes[RAFT].Segments[0].Line.Start().IsClose(route.Modes[HIKE].Segments[0].Line.Start(), DELTA) {
				// needs separate points for hiking and packrafting
				folder.Placemarks = append(folder.Placemarks, &kml.Placemark{
					Visibility: 1,
					Open:       0,
					StyleUrl:   fmt.Sprintf("#go"),
					Name:       fmt.Sprintf(nameFormat, " hiking") + " start",
					Point:      kml.PosPoint(route.Modes[HIKE].Segments[0].Line.Start()),
				})
				folder.Placemarks = append(folder.Placemarks, &kml.Placemark{
					Visibility: 1,
					Open:       0,
					StyleUrl:   fmt.Sprintf("#go"),
					Name:       fmt.Sprintf(nameFormat, " packrafting") + " start",
					Point:      kml.PosPoint(route.Modes[RAFT].Segments[0].Line.Start()),
				})
			} else {
				folder.Placemarks = append(folder.Placemarks, &kml.Placemark{
					Visibility: 1,
					Open:       0,
					StyleUrl:   fmt.Sprintf("#go"),
					Name:       fmt.Sprintf(nameFormat, "") + " start",
					Point:      kml.PosPoint(route.All[0].Line.Start()),
				})
			}
			if route.Modes[RAFT] != nil && route.Modes[HIKE] != nil && !route.Modes[RAFT].Segments[len(route.Modes[RAFT].Segments)-1].Line.End().IsClose(route.Modes[HIKE].Segments[len(route.Modes[HIKE].Segments)-1].Line.End(), DELTA) {
				// needs separate points for hiking and packrafting
				folder.Placemarks = append(folder.Placemarks, &kml.Placemark{
					Visibility: 1,
					Open:       0,
					StyleUrl:   fmt.Sprintf("#grn-square"),
					Name:       fmt.Sprintf(nameFormat, " hiking") + " end",
					Point:      kml.PosPoint(route.Modes[HIKE].Segments[len(route.Modes[HIKE].Segments)-1].Line.End()),
				})
				folder.Placemarks = append(folder.Placemarks, &kml.Placemark{
					Visibility: 1,
					Open:       0,
					StyleUrl:   fmt.Sprintf("#grn-square"),
					Name:       fmt.Sprintf(nameFormat, " packrafting") + " end",
					Point:      kml.PosPoint(route.Modes[RAFT].Segments[len(route.Modes[RAFT].Segments)-1].Line.End()),
				})
			} else {
				folder.Placemarks = append(folder.Placemarks, &kml.Placemark{
					Visibility: 1,
					Open:       0,
					StyleUrl:   fmt.Sprintf("#grn-square"),
					Name:       fmt.Sprintf(nameFormat, "") + " end",
					Point:      kml.PosPoint(route.All[len(route.All)-1].Line.End()),
				})
			}
		}
	}

	waypointsFolder = &kml.Folder{
		Name: "Waypoints by Section",
	}
	for _, key := range d.Keys {
		if HAS_SINGLE && key != SINGLE {
			continue
		}
		section := d.Sections[key]
		sectionFolder := &kml.Folder{
			Name: section.FolderName(),
		}
		subfolders := map[string]*kml.Folder{}
		for _, w := range section.Waypoints {
			if w.Folder == "" {
				sectionFolder.Placemarks = append(sectionFolder.Placemarks, &kml.Placemark{
					Visibility: 1,
					Open:       0,
					StyleUrl:   "#ylw-blank",
					Name:       w.Name,
					Legacy:     legacy.waypoint(w.Legacy, w.Name),
					Point:      kml.PosPoint(w.Pos),
				})
			} else {
				if subfolders[w.Folder] == nil {
					f := &kml.Folder{
						Name: w.Folder,
					}
					subfolders[w.Folder] = f
					sectionFolder.Folders = append(sectionFolder.Folders, f)
				}
				subfolders[w.Folder].Placemarks = append(subfolders[w.Folder].Placemarks, &kml.Placemark{
					Visibility: 1,
					Open:       0,
					StyleUrl:   "#ylw-blank",
					Name:       w.Name,
					Legacy:     legacy.waypoint(w.Legacy, w.Name),
					Point:      kml.PosPoint(w.Pos),
				})
			}
		}
		waypointsFolder.Folders = append(waypointsFolder.Folders, sectionFolder)
	}

	return
}

func (d *Data) SaveKmlWaypoints(dpath string, stamp string) error {
	logln("saving kml waypoints")
	/*
		debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Waypoints/All Points.kmz", "input-all.txt")
		debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Waypoints/Important Infromation.kmz", "input-important.txt")
		debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Waypoints/Optional Start and End Points.kmz", "input-optional-start.txt")
		debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Waypoints/Regular Start and End Points.kmz", "input-regular-start.txt")
		debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Waypoints/Resupply Locations.kmz", "input-resupply.txt")
		debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Waypoints/Waypoints.kmz", "input-waypoints.txt")
	*/

	legacy := &LegacyRenameHolder{update: false}

	regularStartEndFolder, optionalStartEndFolder, resupplyFolder, geographicFolder, importantFolder, waypointsFolder := d.getWaypointFolders(legacy)

	all := kml.Root{
		Xmlns: "http://www.opengis.net/kml/2.2",
		Document: kml.Document{
			Name: "All Points.kmz",
			Folders: []*kml.Folder{
				{
					Name: "Points",
					Folders: []*kml.Folder{
						importantFolder,
						waypointsFolder,
						regularStartEndFolder,
						optionalStartEndFolder,
						resupplyFolder,
						geographicFolder,
					},
				},
			},
		},
	}
	if err := all.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "All Points.kmz")); err != nil {
		return fmt.Errorf("saving All Points.kmz: %w", err)
	}

	imp := kml.Root{
		Xmlns: "http://www.opengis.net/kml/2.2",
		Document: kml.Document{
			Name:    "Important Information.kmz",
			Folders: []*kml.Folder{importantFolder},
		},
	}
	if err := imp.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "Important Information.kmz")); err != nil {
		return fmt.Errorf("saving Important Information.kmz: %w", err)
	}

	res := kml.Root{
		Xmlns: "http://www.opengis.net/kml/2.2",
		Document: kml.Document{
			Name:    "Resupply Locations.kmz",
			Folders: []*kml.Folder{resupplyFolder},
		},
	}
	if err := res.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "Resupply Locations.kmz")); err != nil {
		return fmt.Errorf("saving Resupply Locations.kmz: %w", err)
	}

	regStart := kml.Root{
		Xmlns: "http://www.opengis.net/kml/2.2",
		Document: kml.Document{
			Name:    "Section Start Points (regular).kmz",
			Folders: []*kml.Folder{regularStartEndFolder},
		},
	}
	if err := regStart.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "Section Start Points (regular).kmz")); err != nil {
		return fmt.Errorf("saving Section Start Points (regular).kmz: %w", err)
	}

	optStart := kml.Root{
		Xmlns: "http://www.opengis.net/kml/2.2",
		Document: kml.Document{
			Name:    "Section Start Points (optional).kmz",
			Folders: []*kml.Folder{optionalStartEndFolder},
		},
	}
	if err := optStart.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "Section Start Points (optional).kmz")); err != nil {
		return fmt.Errorf("saving Section Start Points (optional).kmz: %w", err)
	}

	way := kml.Root{
		Xmlns: "http://www.opengis.net/kml/2.2",
		Document: kml.Document{
			Name:    "Waypoints.kmz",
			Folders: []*kml.Folder{waypointsFolder},
		},
	}
	if err := way.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "Waypoints.kmz")); err != nil {
		return fmt.Errorf("saving Waypoints.kmz: %w", err)
	}

	/*
		debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "All Points.kmz"), "output-all.txt")
		debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "Important Information.kmz"), "output-important.txt")
		debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "Resupply Locations.kmz"), "output-resupply.txt")
		debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "Optional Start and End Points.kmz"), "output-optional-start.txt")
		debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "Regular Start and End Points.kmz"), "output-regular-start.txt")
		debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "Waypoints.kmz"), "output-waypoints.txt")
	*/

	return nil
}

func addSegmentStyles(d *kml.Document) {
	for weightName, weightValue := range weights {
		for colourName, colourValue := range colours {
			r, g, b := colourValue[0:2], colourValue[2:4], colourValue[4:6]
			d.Styles = append(d.Styles, &kml.Style{
				Id: fmt.Sprintf("%s-%s", weightName, colourName),
				LineStyle: &kml.LineStyle{
					Color: fmt.Sprintf("ff%s%s%s", b, g, r),
					Width: geo.FloatOne(weightValue),
				},
			})
		}
	}

	addStyle := func(fname string) *kml.Style {
		return &kml.Style{
			Id: fname,
			IconStyle: &kml.IconStyle{
				Scale:   0.8,
				Icon:    &kml.Icon{Href: fmt.Sprintf("http://maps.google.com/mapfiles/kml/paddle/%s.png", fname)},
				HotSpot: &kml.HotSpot{X: 32, Y: 1, Xunits: "pixels", Yunits: "pixels"},
			},
			ListStyle: &kml.ListStyle{
				Scale:    0.5,
				ItemIcon: &kml.Icon{Href: fmt.Sprintf("http://maps.google.com/mapfiles/kml/paddle/%s-lv.png", fname)},
			},
		}
	}

	colours := []string{"blu", "wht", "ylw", "red", "grn", "pink", "orange"}
	decorations := []string{"blank", "stars", "circle", "square"}
	for _, colour := range colours {
		for _, decoration := range decorations {
			d.Styles = append(d.Styles, addStyle(fmt.Sprintf("%s-%s", colour, decoration)))
		}
	}
	d.Styles = append(d.Styles, addStyle("go"))
}

func (d *Data) SaveKmlTracks(dpath string, stamp string) error {
	logln("saving kml tracks")
	//debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Tracks/Regular Tracks.kmz", "input-regular.txt")
	//debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Tracks/Optional Tracks.kmz", "input-optional.txt")
	//debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Tracks/All Tracks.kmz", "input-all.txt")
	do := func(name string, filter func(*Route) bool) (*kml.Folder, error) {
		f := &kml.Folder{Name: name}
		for _, key := range d.Keys {
			if HAS_SINGLE && key != SINGLE {
				continue
			}
			section := d.Sections[key]
			sectionFolder := &kml.Folder{
				Name: section.FolderName(),
			}
			for _, route := range section.Routes {
				if name == "Regular Tracks" && route.Key.Required == OPTIONAL {
					continue
				} else if name == "Optional Tracks" && route.Key.Required == REGULAR {
					continue
				}
				var trackFolder *kml.Folder
				if route.Key.Required == REGULAR {
					if route.Key.Direction == "" {
						trackFolder = sectionFolder
					} else {
						var name string
						if route.Key.Direction == "S" {
							name = "Southbound"
						} else {
							name = "Northbound"
						}
						trackFolder = &kml.Folder{
							Name: name,
						}
						sectionFolder.Folders = append(sectionFolder.Folders, trackFolder)
					}
				} else {
					trackFolder = &kml.Folder{
						Name: route.FolderName(),
					}
					sectionFolder.Folders = append(sectionFolder.Folders, trackFolder)
				}
				for _, segment := range route.All {
					trackFolder.Placemarks = append(trackFolder.Placemarks, &kml.Placemark{
						Visibility: 1,
						Open:       0,
						Name:       segment.PlacemarkName(),
						StyleUrl:   fmt.Sprintf("#%s", segment.Style()),
						LineString: &kml.LineString{
							Tessellate:  true,
							Coordinates: kml.LineCoordinates(segment.Line),
						},
					})
				}
			}
			f.Folders = append(f.Folders, sectionFolder)
		}
		return f, nil
	}
	regularFolder, err := do("Regular Tracks", func(r *Route) bool { return r.Key.Required == REGULAR })
	if err != nil {
		return fmt.Errorf("building regular tracks kml: %w", err)
	}
	optionalFolder, err := do("Optional Tracks", func(r *Route) bool { return r.Key.Required == OPTIONAL })
	if err != nil {
		return fmt.Errorf("building optional tracks kml: %w", err)
	}
	all := kml.Root{
		Xmlns: "http://www.opengis.net/kml/2.2",
		Document: kml.Document{
			Name: "All Tracks.kmz",
			Open: 1,
			Folders: []*kml.Folder{
				{
					Name:    "Tracks",
					Folders: []*kml.Folder{regularFolder, optionalFolder},
				},
			},
		},
	}
	addSegmentStyles(&all.Document)
	if err := all.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "All Tracks.kmz")); err != nil {
		return fmt.Errorf("saving All Tracks.kmz: %w", err)
	}
	regular := kml.Root{
		Xmlns: "http://www.opengis.net/kml/2.2",
		Document: kml.Document{
			Name:    "Regular Tracks.kmz",
			Open:    1,
			Folders: []*kml.Folder{regularFolder},
		},
	}
	addSegmentStyles(&regular.Document)
	if err := regular.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "Regular Tracks.kmz")); err != nil {
		return fmt.Errorf("saving Regular Tracks.kmz: %w", err)
	}
	optional := kml.Root{
		Xmlns: "http://www.opengis.net/kml/2.2",
		Document: kml.Document{
			Name:    "Optional Tracks.kmz",
			Open:    1,
			Folders: []*kml.Folder{optionalFolder},
		},
	}
	addSegmentStyles(&optional.Document)
	if err := optional.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "Optional Tracks.kmz")); err != nil {
		return fmt.Errorf("saving Optional Tracks.kmz: %w", err)
	}

	//debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "All Tracks.kmz"), "output-all.txt")
	//debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "Regular Tracks.kmz"), "output-regular.txt")
	//debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "Optional Tracks.kmz"), "output-optional.txt")

	return nil
}

func (d *Data) SaveGpx(dpath string, stamp string) error {
	logln("saving gpx files")
	type matcher struct {
		path     []string
		match    func(*Segment) bool
		segments []*Segment
	}
	matchers := []*matcher{
		{
			path:  []string{"Combined Tracks", fmt.Sprintf("All Optional and Regular Tracks (%s).gpx", stamp)},
			match: func(s *Segment) bool { return true },
		},
		{
			path:  []string{"Combined Tracks", fmt.Sprintf("Optional Tracks (%s).gpx", stamp)},
			match: func(s *Segment) bool { return s.Route.Key.Required == OPTIONAL },
		},
		{
			path:  []string{"Combined Tracks", fmt.Sprintf("Regular Tracks (%s).gpx", stamp)},
			match: func(s *Segment) bool { return s.Route.Key.Required == REGULAR },
		},

		{
			path: []string{"Exploration Hiking Tracks", "Optional Tracks", "EXP-OH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OH" && s.Verification == "A"
			},
		},
		{
			path: []string{"Exploration Hiking Tracks", "Optional Tracks", "EXP-OH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OH" && s.Verification == "I"
			},
		},
		{
			path: []string{"Exploration Hiking Tracks", "Optional Tracks", "EXP-OH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OH" && s.Verification == "V"
			},
		},

		{
			path: []string{"Exploration Hiking Tracks", "Regular Tracks", "EXP-RH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RH" && s.Verification == "A"
			},
		},
		{
			path: []string{"Exploration Hiking Tracks", "Regular Tracks", "EXP-RH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RH" && s.Verification == "I"
			},
		},
		{
			path: []string{"Exploration Hiking Tracks", "Regular Tracks", "EXP-RH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RH" && s.Verification == "V"
			},
		},
		{
			path: []string{"Exploration Hiking Tracks", "Regular Tracks", "EXP-RR-LD-A.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RR" && s.Verification == "A"
			},
		},
		{
			path: []string{"Exploration Hiking Tracks", "Regular Tracks", "EXP-RR-LD-I.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RR" && s.Verification == "I"
			},
		},
		{
			path: []string{"Exploration Hiking Tracks", "Regular Tracks", "EXP-RR-LD-V.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RR" && s.Verification == "V"
			},
		},

		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OH" && s.Verification == "A"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OH" && s.Verification == "I"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OH" && s.Verification == "V"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-LD-A.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OP" && s.Verification == "A"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-LD-I.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OP" && s.Verification == "I"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-LD-V.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OP" && s.Verification == "V"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-WR-1.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OP" && s.Directional == "1"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-WR-2.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OP" && s.Directional == "2"
			},
		},

		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-RH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RH" && s.Verification == "A"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-RH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RH" && s.Verification == "I"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-RH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RH" && s.Verification == "V"
			},
		},

		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-LD-A.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RP" && s.Verification == "A"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-LD-I.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RP" && s.Verification == "I"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-LD-V.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RP" && s.Verification == "V"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-WR-1.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RP" && s.Directional == "1"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-WR-2.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RP" && s.Directional == "2"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RR-LD-A.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RR" && s.Verification == "A"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RR-LD-I.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RR" && s.Verification == "I"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RR-LD-V.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RR" && s.Verification == "V"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RR-WR-1.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RR" && s.Directional == "1"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RR-WR-2.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RR" && s.Directional == "2"
			},
		},

		{
			path: []string{"Hiking Tracks", "Optional Tracks", "OH-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && HasFerry(s.Terrains) && s.Directional == "1"
			},
		},
		{
			path: []string{"Hiking Tracks", "Optional Tracks", "OH-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && HasFerry(s.Terrains) && s.Directional == "2"
			},
		},
		{
			path: []string{"Hiking Tracks", "Optional Tracks", "OH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && s.Verification == "A"
			},
		},
		{
			path: []string{"Hiking Tracks", "Optional Tracks", "OH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && s.Verification == "I"
			},
		},
		{
			path: []string{"Hiking Tracks", "Optional Tracks", "OH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && s.Verification == "V"
			},
		},

		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RH-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && HasFerry(s.Terrains) && s.Directional == "1"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RH-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && HasFerry(s.Terrains) && s.Directional == "2"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && s.Verification == "A"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && s.Verification == "I"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && s.Verification == "V"
			},
		},

		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RR-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && HasFerry(s.Terrains) && s.Directional == "1"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RR-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && HasFerry(s.Terrains) && s.Directional == "2"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RR-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && s.Verification == "A"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RR-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && s.Verification == "I"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RR-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && s.Verification == "V"
			},
		},

		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && HasFerry(s.Terrains) && s.Directional == "1"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && HasFerry(s.Terrains) && s.Directional == "2"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && s.Verification == "A"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && s.Verification == "I"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && s.Verification == "V"
			},
		},

		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OP" && HasFerry(s.Terrains) && s.Directional == "1"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OP" && HasFerry(s.Terrains) && s.Directional == "2"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OP" && s.Verification == "A"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OP" && s.Verification == "I"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OP" && s.Verification == "V"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-WR-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OP" && s.Directional == "1"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-WR-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OP" && s.Directional == "2"
			},
		},

		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && HasFerry(s.Terrains) && s.Directional == "1"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && HasFerry(s.Terrains) && s.Directional == "2"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && s.Verification == "A"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && s.Verification == "I"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && s.Verification == "V"
			},
		},

		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RP" && HasFerry(s.Terrains) && s.Directional == "1"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RP" && HasFerry(s.Terrains) && s.Directional == "2"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RP" && s.Verification == "A"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RP" && s.Verification == "I"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RP" && s.Verification == "V"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-WR-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RP" && s.Directional == "1"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-WR-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RP" && s.Directional == "2"
			},
		},

		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && HasFerry(s.Terrains) && s.Directional == "1"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && HasFerry(s.Terrains) && s.Directional == "2"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && s.Verification == "A"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && s.Verification == "I"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && s.Verification == "V"
			},
		},
	}

	for _, sectionKey := range d.Keys {
		section := d.Sections[sectionKey]
		for _, routeKey := range section.RouteKeys {
			route := section.Routes[routeKey]
			if route.Key.Alternatives {
				continue
			}
			for _, segment := range route.All {
				for _, m := range matchers {
					if m.match(segment) {
						m.segments = append(m.segments, segment)
					}
				}
			}
		}
	}

	for _, m := range matchers {

		if len(m.segments) == 0 {
			continue
		}

		g := gpx.Root{}
		for _, segment := range m.segments {
			g.Tracks = append(g.Tracks, gpx.Track{
				Name:     segment.PlacemarkName(),
				Segments: []gpx.TrackSegment{{Points: gpx.LineTrackPoints(segment.Line)}},
			})
		}
		fpath := filepath.Join(append([]string{dpath, "GPX Files (For Smartphones and Basecamp)"}, m.path...)...)
		if err := g.Save(fpath); err != nil {
			return fmt.Errorf("saving %q: %w", m.path[len(m.path)-1], err)
		}
	}

	wpAll := gpx.Root{}
	for _, key := range d.Keys {
		if HAS_SINGLE && key != SINGLE {
			continue
		}
		section := d.Sections[key]
		for _, w := range section.Waypoints {
			wpAll.Waypoints = append(wpAll.Waypoints, gpx.Waypoint{
				Point: gpx.PosPoint(w.Pos),
				Name:  w.Name,
			})
		}
	}

	if err := wpAll.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("All Other Waypoints (%s).gpx", stamp))); err != nil {
		return fmt.Errorf("saving waypoints: %w", err)
	}

	wpImp := gpx.Root{}
	for _, w := range d.Important {
		wpImp.Waypoints = append(wpImp.Waypoints, gpx.Waypoint{
			Point: gpx.PosPoint(w.Pos),
			Name:  w.Name,
		})
	}
	if err := wpImp.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("Important Infromation (%s).gpx", stamp))); err != nil {
		return fmt.Errorf("saving waypoints: %w", err)
	}

	wpRes := gpx.Root{}
	for _, w := range d.Resupplies {
		wpRes.Waypoints = append(wpRes.Waypoints, gpx.Waypoint{
			Point: gpx.PosPoint(w.Pos),
			Name:  w.Name,
		})
	}
	if err := wpRes.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("Resupply Locations (%s).gpx", stamp))); err != nil {
		return fmt.Errorf("saving waypoints: %w", err)
	}

	startPoints := gpx.Root{}

	for _, sectionKey := range d.Keys {
		section := d.Sections[sectionKey]
		var routes []*Route
		for _, routeKey := range section.RouteKeys {
			if routeKey.Required == OPTIONAL {
				continue
			}
			routes = append(routes, section.Routes[routeKey])
		}
		for _, route := range routes {
			if route.Modes[RAFT] != nil && route.Modes[HIKE] != nil && !route.Modes[RAFT].Segments[0].Line.Start().IsClose(route.Modes[HIKE].Segments[0].Line.Start(), DELTA) {
				// needs separate points for hiking and packrafting
				wp1 := gpx.Waypoint{
					Point: gpx.PosPoint(route.Modes[HIKE].Segments[0].Line.Start()),
					Name:  fmt.Sprintf("GPT%s%s (%s) hiking", section.Key.Code(), route.Key.Direction, section.Name),
				}
				startPoints.Waypoints = append(startPoints.Waypoints, wp1)
				wp2 := gpx.Waypoint{
					Point: gpx.PosPoint(route.Modes[RAFT].Segments[0].Line.Start()),
					Name:  fmt.Sprintf("GPT%s%s (%s) packrafting", section.Key.Code(), route.Key.Direction, section.Name),
				}
				startPoints.Waypoints = append(startPoints.Waypoints, wp2)

			} else {
				wp := gpx.Waypoint{
					Point: gpx.PosPoint(route.All[0].Line.Start()),
					Name:  fmt.Sprintf("GPT%s%s (%s)", section.Key.Code(), route.Key.Direction, section.Name),
				}
				startPoints.Waypoints = append(startPoints.Waypoints, wp)
			}
		}
	}
	if err := startPoints.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("Section Start Points (%s).gpx", stamp))); err != nil {
		return fmt.Errorf("saving waypoints: %w", err)
	}

	if err := ioutil.WriteFile(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Nomenclature.txt"), []byte(Nomenclature), 0666); err != nil {
		return fmt.Errorf("saving Nomenclature.txt: %w", err)
	}

	return nil
}

func (d *Data) SaveGaia(dpath string) error {
	logln("saving gaia files")
	type groupStruct struct {
		letter, name string
		from, to     int
	}
	type clusterStruct struct {
		name                       string
		from, to                   int
		modes                      map[ModeType]map[string]*gpx.Root
		routes, waypoints, options bool
	}
	newModes := func() map[ModeType]map[string]*gpx.Root {
		return map[ModeType]map[string]*gpx.Root{
			HIKE: {"routes": {}, "options": {}, "waypoints": {}},
			RAFT: {"routes": {}, "options": {}, "waypoints": {}},
		}
	}
	groups := []groupStruct{
		{letter: "A", name: "Precordillera", from: 1, to: 4},
		{letter: "B", name: "Arrieros", from: 5, to: 9},
		{letter: "C", name: "Pehuenche", from: 10, to: 16},
		{letter: "D", name: "Lagos Chilenos", from: 17, to: 22},
		{letter: "E", name: "Lagos Argentinos", from: 23, to: 26},
		{letter: "F", name: "Sector Palena", from: 27, to: 28},
		{letter: "G", name: "Aysen Norte", from: 29, to: 31},
		{letter: "H", name: "Aysen Sur", from: 32, to: 37},
		{letter: "I", name: "Campo de Hielo Sur 1", from: 38, to: 40},
		{letter: "J", name: "Campo de Hielo Sur 2", from: 41, to: 46},
		{letter: "K", name: "Magallanes", from: 47, to: 50},
		{letter: "L", name: "Tierra del Fuego", from: 60, to: 69},
		{letter: "M", name: "Sector Yelcho", from: 70, to: 75},
		{letter: "N", name: "Fiordos Norte", from: 76, to: 78},
		{letter: "O", name: "Campo de Hielo Norte", from: 80, to: 83},
		{letter: "P", name: "Fjordos Sur", from: 90, to: 92},
	}
	clusters := []clusterStruct{
		{
			name:   "",
			from:   1,
			to:     99,
			modes:  newModes(),
			routes: true, waypoints: true, options: false,
		},
		//{name: "main-north", from: 1, to: 16, modes: newModes()},
		//{name: "main-south", from: 17, to: 40, modes: newModes()},
		//{name: "extensions-south", from: 41, to: 69, modes: newModes()},
		//{name: "extensions-west", from: 72, to: 99, modes: newModes()},
	}
	for _, group := range groups {
		clusters = append(clusters, clusterStruct{
			name:   fmt.Sprintf("Group %s (GPT%02d-GPT%02d) %s", group.letter, group.from, group.to, group.name),
			from:   group.from,
			to:     group.to,
			modes:  newModes(),
			routes: false, waypoints: false, options: true,
		})
	}
	for _, cluster := range clusters {

		count := map[ModeType][]*Section{}
		for mode := range cluster.modes {
			count[mode] = []*Section{}
		}

		for _, key := range d.Keys {
			if HAS_SINGLE && key != SINGLE {
				continue
			}
			section := d.Sections[key]
			if section.Key.Number < cluster.from || section.Key.Number > cluster.to {
				continue
			}
			for mode, contents := range cluster.modes {
				// TODO: skip if mode != section type (e.g. packrafting != hiking)
				if section.Key.Suffix == "H" && mode == RAFT || section.Key.Suffix == "P" && mode == HIKE {
					// TODO: check this!
					continue
				}

				count[mode] = append(count[mode], section)

				for _, routeKey := range section.RouteKeys {
					if routeKey.Required != REGULAR {
						continue
					}
					route := section.Routes[routeKey]
					routeMode := route.Modes[mode]
					if routeMode == nil {
						continue
					}
					network := routeMode.Network

					var rte gpx.Route
					var direction string
					if route.Key.Direction == "N" {
						direction = " northbound"
					} else if route.Key.Direction == "S" {
						direction = " southbound"
					}
					rte.Name = fmt.Sprintf("GPT%s %s%s", section.Key.Code(), section.Name, direction)
					rte.Desc = HEADING_SYMBOL + " " + rte.Name + "\n\n"

					var lines []geo.Line
					for _, segment := range routeMode.Segments {
						lines = append(lines, segment.Line)
					}

					var id int
					for i, straight := range network.Straights {
						if i > 0 {
							rte.Desc += "---\n"
						}
						for _, flush := range straight.Flushes {
							id++
							rte.Desc += flush.Description(id, false) + "\n"
						}
					}

					rte.Points = gpx.LinePoints(geo.MergeLines(lines))
					rte.Desc += section.Scraped[mode]
					contents["routes"].Routes = append(contents["routes"].Routes, rte)
				}

				for _, routeKey := range section.RouteKeys {

					if routeKey.Required != OPTIONAL {
						continue
					}

					route := section.Routes[routeKey]
					routeMode := route.Modes[mode]
					if routeMode == nil {
						continue
					}

					network := routeMode.Network

					var trk gpx.Track
					if route.Key.Alternatives {
						var direction string
						if route.Key.Direction == "N" {
							direction = " northbound"
						} else if route.Key.Direction == "S" {
							direction = " southbound"
						}
						trk.Name = fmt.Sprintf("GPT%s%s hiking alternatives %d", route.Section.Key.Code(), direction, route.Key.AlternativesIndex)
					} else if route.Key.Option == 0 {
						trk.Name = fmt.Sprintf("GPT%s variant %s%s", route.Section.Key.Code(), route.Key.Variant, route.Key.Network)
						if route.Name != "" {
							trk.Name += fmt.Sprintf(" (%s)", route.Name)
						}
					} else {
						trk.Name = fmt.Sprintf("GPT%s option %d%s%s", route.Section.Key.Code(), route.Key.Option, route.Key.Variant, route.Key.Network)
						if route.Option != "" && route.Key.Variant == "" && (route.Key.Network == "" || route.Key.Network == "a") {
							trk.Name += fmt.Sprintf(" (%s)", route.Option)
						}
					}
					trk.Desc = HEADING_SYMBOL + " " + trk.Name + "\n\n"

					var id int
					for i, straight := range network.Straights {
						if i > 0 {
							trk.Desc += "---\n"
						}
						for _, flush := range straight.Flushes {
							id++
							trk.Desc += flush.Description(id, false) + "\n"
						}
					}

					for _, segment := range routeMode.Segments {
						trk.Segments = append(trk.Segments, gpx.TrackSegment{Points: gpx.LineTrackPoints(segment.Line)})
					}
					contents["options"].Tracks = append(contents["options"].Tracks, trk)

				}

				for _, w := range section.Waypoints {
					contents["waypoints"].Waypoints = append(contents["waypoints"].Waypoints, gpx.Waypoint{
						Point: gpx.PosPoint(w.Pos),
						Name:  w.Name,
						Desc:  "GPT" + key.Code(),
					})
				}
			}
		}
		for mode := range cluster.modes {
			sections := count[mode]

			for _, section := range sections {

				var routes []*Route
				for _, routeKey := range section.RouteKeys {
					if routeKey.Required == OPTIONAL {
						continue
					}
					routes = append(routes, section.Routes[routeKey])
				}
				for _, route := range routes {
					if route.Modes[RAFT] != nil && route.Modes[HIKE] != nil && !route.Modes[RAFT].Segments[0].Line.Start().IsClose(route.Modes[HIKE].Segments[0].Line.Start(), DELTA) {
						// needs separate points for hiking and packrafting
						if mode == HIKE {
							wp1 := gpx.Waypoint{
								Point: gpx.PosPoint(route.Modes[HIKE].Segments[0].Line.Start()),
								Name:  fmt.Sprintf("GPT%s%s %s", section.Key.Code(), route.Key.Direction, section.Name),
							}
							cluster.modes[mode]["routes"].Waypoints = append(cluster.modes[mode]["routes"].Waypoints, wp1)
						} else {
							wp2 := gpx.Waypoint{
								Point: gpx.PosPoint(route.Modes[RAFT].Segments[0].Line.Start()),
								Name:  fmt.Sprintf("GPT%s%s %s", section.Key.Code(), route.Key.Direction, section.Name),
							}
							cluster.modes[mode]["routes"].Waypoints = append(cluster.modes[mode]["routes"].Waypoints, wp2)
						}

					} else {
						wp := gpx.Waypoint{
							Point: gpx.PosPoint(route.All[0].Line.Start()),
							Name:  fmt.Sprintf("GPT%s%s %s", section.Key.Code(), route.Key.Direction, section.Name),
						}
						cluster.modes[mode]["routes"].Waypoints = append(cluster.modes[mode]["routes"].Waypoints, wp)
					}
				}
			}

		}
	}

	for _, cluster := range clusters {
		for mode, modeMap := range cluster.modes {
			for contents, root := range modeMap {
				var modeString, contentsString string
				var output, options bool
				switch mode {
				case HIKE:
					modeString = "hiking"
				default:
					modeString = "packrafting"
				}
				switch contents {
				case "routes":
					output = cluster.routes
					contentsString = "routes"
				case "options":
					options = true
					output = cluster.options
					contentsString = "options"
				case "waypoints":
					output = cluster.waypoints
					contentsString = "waypoints"
				}
				if output {
					var fname string
					if cluster.name == "" {
						fname = fmt.Sprintf("%s (%s).gpx", strings.Title(contentsString), modeString)
					} else {
						fname = fmt.Sprintf("%s - %s %s.gpx", cluster.name, modeString, contentsString)
					}
					dir := "GPX Files (For Gaia GPS app)"
					var fpath string
					if options {
						if mode == HIKE {
							fpath = filepath.Join(dpath, dir, "Options (hiking)", fname)
						} else {
							fpath = filepath.Join(dpath, dir, "Options (packrafting)", fname)
						}
					} else {
						fpath = filepath.Join(dpath, dir, fname)
					}
					if err := root.Save(fpath); err != nil {
						return fmt.Errorf("writing gpx")
					}
				}
			}
		}
	}
	wp := func(waypoints []Waypoint, name string, prefix string) error {
		root := &gpx.Root{}
		for _, w := range waypoints {
			root.Waypoints = append(root.Waypoints, gpx.Waypoint{
				Point: gpx.PosPoint(w.Pos),
				Name:  prefix + w.Name,
			})
		}
		return root.Save(filepath.Join(dpath, "GPX Files (For Gaia GPS app)", name))
	}
	if err := wp(d.Resupplies, "Waypoints (resupplies).gpx", "Resupply: "); err != nil {
		return fmt.Errorf("writing resupplies gpx: %w", err)
	}
	if err := wp(d.Important, "Waypoints (important).gpx", "Important: "); err != nil {
		return fmt.Errorf("writing important gpx: %w", err)
	}
	if err := wp(d.Geographic, "Waypoints (geographic).gpx", ""); err != nil {
		return fmt.Errorf("writing important gpx: %w", err)
	}

	// AREAS
	{
		areaResolution := 0.7 // medium
		//areaResolution := 3.0
		areas := map[geo.Pos]bool{}
		round := func(number float64) float64 {
			return math.Floor(number/areaResolution) * areaResolution
		}
		markSingle := func(pos geo.Pos) {
			roundedLat, roundedLon := round(pos.Lat), round(pos.Lon)
			areas[geo.Pos{Lat: roundedLat, Lon: roundedLon}] = true
		}
		mark := func(pos geo.Pos) {
			markSingle(geo.Pos{Lat: pos.Lat, Lon: pos.Lon})
			markSingle(geo.Pos{Lat: pos.Lat, Lon: pos.Lon + areaResolution/2})
			markSingle(geo.Pos{Lat: pos.Lat, Lon: pos.Lon - areaResolution/2})
			markSingle(geo.Pos{Lat: pos.Lat + areaResolution/2, Lon: pos.Lon})
			markSingle(geo.Pos{Lat: pos.Lat + areaResolution/2, Lon: pos.Lon + areaResolution/2})
			markSingle(geo.Pos{Lat: pos.Lat + areaResolution/2, Lon: pos.Lon - areaResolution/2})
			markSingle(geo.Pos{Lat: pos.Lat - areaResolution/2, Lon: pos.Lon})
			markSingle(geo.Pos{Lat: pos.Lat - areaResolution/2, Lon: pos.Lon + areaResolution/2})
			markSingle(geo.Pos{Lat: pos.Lat - areaResolution/2, Lon: pos.Lon - areaResolution/2})
		}

		var areasPlacemarks []*kml.Placemark
		for _, key := range d.Keys {
			if HAS_SINGLE && key != SINGLE {
				continue
			}

			section := d.Sections[key]
			for _, route := range section.Routes {
				for _, segment := range route.All {
					for _, pos := range segment.Line {
						mark(pos)
					}
				}
			}
		}
		var areasSlice []geo.Pos
		for pos := range areas {
			areasSlice = append(areasSlice, pos)
		}
		sort.Slice(areasSlice, func(i, j int) bool {
			if areasSlice[i].Lat != areasSlice[j].Lat {
				return areasSlice[i].Lat > areasSlice[j].Lat
			} else {
				return areasSlice[i].Lon < areasSlice[j].Lon
			}
		})

		for i, pos := range areasSlice {
			areasPlacemarks = append(areasPlacemarks, &kml.Placemark{
				Visibility: 1,
				Open:       0,
				Name:       fmt.Sprintf("Area %03d/%03d", i+1, len(areasSlice)),
				Polygon: &kml.Polygon{
					OuterBoundaryIs: &kml.OuterBoundaryIs{
						LinearRing: &kml.LinearRing{
							Coordinates: kml.AreaCoordinates(pos.Lat, pos.Lat+areaResolution, pos.Lon, pos.Lon+areaResolution),
						},
					},
				},
			})
		}
		areasKml := kml.Root{
			Xmlns: "http://www.opengis.net/kml/2.2",
			Document: kml.Document{
				Name: "Areas.kmz",
				Folders: []*kml.Folder{
					{
						Name:       "Areas",
						Placemarks: areasPlacemarks,
					},
				},
			},
		}
		if err := areasKml.Save(filepath.Join(dpath, "GPX Files (For Gaia GPS app)", "Areas.kmz")); err != nil {
			return fmt.Errorf("writing areas kml: %w", err)
		}
	}

	//if false {
	//
	//	spfr := &kml.Folder{
	//		Name:       "Regular tracks",
	//		Visibility: 1,
	//		Open:       0,
	//		Placemarks: nil,
	//	}
	//	spfo := &kml.Folder{
	//		Name:       "Optional tracks",
	//		Visibility: 1,
	//		Open:       0,
	//		Placemarks: nil,
	//	}
	//	sp := kml.Root{
	//		Xmlns: "http://www.opengis.net/kml/2.2",
	//		Document: kml.Document{
	//			Name: "Route start points.kmz",
	//			Folders: []*kml.Folder{
	//				spfr,
	//				spfo,
	//			},
	//		},
	//	}
	//	processRoute := func(pr, hr *Route) error {
	//
	//		var r *Route
	//		if pr != nil {
	//			r = pr
	//		} else {
	//			r = hr
	//		}
	//		rf := &kml.Folder{Name: r.String()}
	//
	//		separateFolders := pr != nil && hr != nil && !pr.HasIdenticalNetworks(hr)
	//		combineRoutes := pr != nil && hr != nil && pr.HasIdenticalNetworks(hr)
	//
	//		addContents := func(f *kml.Folder, r *Route, suffix string) {
	//			for _, n := range r.Networks {
	//
	//				name := n.String()
	//				if suffix != "" {
	//					name += " " + suffix
	//				}
	//				f.Placemarks = append(f.Placemarks, &kml.Placemark{
	//					Name:       name,
	//					Visibility: 1,
	//					Open:       0,
	//					Point:      kml.PosPoint(n.Entry.Line.Start()),
	//				})
	//
	//				net := &kml.Placemark{
	//					Name:          n.String(),
	//					Visibility:    1,
	//					Open:          0,
	//					MultiGeometry: &kml.MultiGeometry{},
	//					Style: &kml.Style{LineStyle: &kml.LineStyle{
	//						Color: "ffffffff",
	//						Width: 10,
	//					}},
	//				}
	//				for _, segment := range n.Segments {
	//					net.MultiGeometry.LineStrings = append(net.MultiGeometry.LineStrings, &kml.LineString{
	//						Tessellate:  true,
	//						Coordinates: kml.LineCoordinates(segment.Line),
	//					})
	//				}
	//				f.Placemarks = append(f.Placemarks, net)
	//			}
	//		}
	//
	//		if separateFolders {
	//			pf := &kml.Folder{Name: "packrafting"}
	//			hf := &kml.Folder{Name: "hiking"}
	//			addContents(pf, pr, "packrafting")
	//			addContents(hf, hr, "hiking")
	//			rf.Folders = append(rf.Folders, pf)
	//			rf.Folders = append(rf.Folders, hf)
	//		} else {
	//			if combineRoutes {
	//				// both routes have identical networks, so only need to do one of them. either will do.
	//				addContents(rf, pr, "")
	//			} else if pr != nil {
	//				addContents(rf, pr, "")
	//			} else if hr != nil {
	//				addContents(rf, hr, "")
	//			} else {
	//				panic("shouldn't be here")
	//			}
	//		}
	//
	//		if r.Regular {
	//			spfr.Folders = append(spfr.Folders, rf)
	//		} else {
	//			spfo.Folders = append(spfo.Folders, rf)
	//		}
	//
	//		return nil
	//	}
	//	if err := d.ForRoutePairs(processRoute); err != nil {
	//		return err
	//	}
	//	return sp.Save(filepath.Join(dpath, "start-points.kmz"))
	//}
	return nil
}

var wpts = map[string]int{}

const Nomenclature = `EXP: Exploration Route

RR: Regular Route 
RH: Regular Hiking Route 
RP: Regular Packrafting Route
OH: Optional Hiking Route 
OP: Optional Packrafting Route 

LD: Land Routes (BB, CC, MR, PR, TL)
BB: Bush Bashing 
CC: Cross Country 
MR: Minor Road 
PR: Primary or Paved Road 
TL: Horse or Hiking Trail 

WR: Water Packrafting Routes (FJ, LK, RI)
FJ: Fjord Packrafting
LK: Lake Packrafting
RI: River Packrafting

V: Verified Route
A: Approximate Route
I: Investigation Route
1: One-Way Route
2: Two-Way Route
`

func debug(fpath string, name string) {
	if !DEBUG {
		return
	}
	f, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	root, err := kml.Load(fpath)
	if err != nil {
		panic(err)
	}
	var disp func(prefix string, folder *kml.Folder)
	disp = func(prefix string, folder *kml.Folder) {
		fmt.Fprintf(f, "%s %s (%d folders, %d placemarks)\n", prefix, folder.Name, len(folder.Folders), len(folder.Placemarks))
		for _, f := range folder.Folders {
			disp(prefix+"-", f)
		}
	}
	fmt.Fprintf(f, "%s (%d folders)\n", root.Document.Name, len(root.Document.Folders))
	for _, folder := range root.Document.Folders {
		disp("-", folder)
	}
}
