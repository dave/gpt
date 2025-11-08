package routedata

import (
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dave/gpt/geo"
	"github.com/dave/gpt/globals"
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

	for _, required := range globals.REQUIRED_TYPES {
		rootFolder := &kml.Folder{}
		switch required {
		case globals.REGULAR:
			rootFolder.Name = "Regular Tracks"
		case globals.OPTIONAL:
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
				case globals.REGULAR:
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
				case globals.OPTIONAL:
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
			if route.Key.Required == globals.REGULAR {
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
			if route.Modes[globals.RAFT] != nil && route.Modes[globals.HIKE] != nil && !route.Modes[globals.RAFT].Segments[0].Line.Start().IsClose(route.Modes[globals.HIKE].Segments[0].Line.Start(), globals.DELTA) {
				// needs separate points for hiking and packrafting
				folder.Placemarks = append(folder.Placemarks, &kml.Placemark{
					Visibility: 1,
					Open:       0,
					StyleUrl:   fmt.Sprintf("#go"),
					Name:       fmt.Sprintf(nameFormat, " hiking") + " start",
					Point:      kml.PosPoint(route.Modes[globals.HIKE].Segments[0].Line.Start()),
				})
				folder.Placemarks = append(folder.Placemarks, &kml.Placemark{
					Visibility: 1,
					Open:       0,
					StyleUrl:   fmt.Sprintf("#go"),
					Name:       fmt.Sprintf(nameFormat, " packrafting") + " start",
					Point:      kml.PosPoint(route.Modes[globals.RAFT].Segments[0].Line.Start()),
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
			if route.Modes[globals.RAFT] != nil && route.Modes[globals.HIKE] != nil && !route.Modes[globals.RAFT].Segments[len(route.Modes[globals.RAFT].Segments)-1].Line.End().IsClose(route.Modes[globals.HIKE].Segments[len(route.Modes[globals.HIKE].Segments)-1].Line.End(), globals.DELTA) {
				// needs separate points for hiking and packrafting
				folder.Placemarks = append(folder.Placemarks, &kml.Placemark{
					Visibility: 1,
					Open:       0,
					StyleUrl:   fmt.Sprintf("#grn-square"),
					Name:       fmt.Sprintf(nameFormat, " hiking") + " end",
					Point:      kml.PosPoint(route.Modes[globals.HIKE].Segments[len(route.Modes[globals.HIKE].Segments)-1].Line.End()),
				})
				folder.Placemarks = append(folder.Placemarks, &kml.Placemark{
					Visibility: 1,
					Open:       0,
					StyleUrl:   fmt.Sprintf("#grn-square"),
					Name:       fmt.Sprintf(nameFormat, " packrafting") + " end",
					Point:      kml.PosPoint(route.Modes[globals.RAFT].Segments[len(route.Modes[globals.RAFT].Segments)-1].Line.End()),
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
		if globals.HAS_SINGLE && key != globals.SINGLE {
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
			if globals.HAS_SINGLE && key != globals.SINGLE {
				continue
			}
			section := d.Sections[key]
			sectionFolder := &kml.Folder{
				Name: section.FolderName(),
			}
			for _, route := range section.Routes {
				if name == "Regular Tracks" && route.Key.Required == globals.OPTIONAL {
					continue
				} else if name == "Optional Tracks" && route.Key.Required == globals.REGULAR {
					continue
				}
				var trackFolder *kml.Folder
				if route.Key.Required == globals.REGULAR {
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
	regularFolder, err := do("Regular Tracks", func(r *Route) bool { return r.Key.Required == globals.REGULAR })
	if err != nil {
		return fmt.Errorf("building regular tracks kml: %w", err)
	}
	optionalFolder, err := do("Optional Tracks", func(r *Route) bool { return r.Key.Required == globals.OPTIONAL })
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
			match: func(s *Segment) bool { return s.Route.Key.Required == globals.OPTIONAL },
		},
		{
			path:  []string{"Combined Tracks", fmt.Sprintf("Regular Tracks (%s).gpx", stamp)},
			match: func(s *Segment) bool { return s.Route.Key.Required == globals.REGULAR },
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
		if globals.HAS_SINGLE && key != globals.SINGLE {
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
			if routeKey.Required == globals.OPTIONAL {
				continue
			}
			routes = append(routes, section.Routes[routeKey])
		}
		for _, route := range routes {
			if route.Modes[globals.RAFT] != nil && route.Modes[globals.HIKE] != nil && !route.Modes[globals.RAFT].Segments[0].Line.Start().IsClose(route.Modes[globals.HIKE].Segments[0].Line.Start(), globals.DELTA) {
				// needs separate points for hiking and packrafting
				wp1 := gpx.Waypoint{
					Point: gpx.PosPoint(route.Modes[globals.HIKE].Segments[0].Line.Start()),
					Name:  fmt.Sprintf("GPT%s%s (%s) hiking", section.Key.Code(), route.Key.Direction, section.Name),
				}
				startPoints.Waypoints = append(startPoints.Waypoints, wp1)
				wp2 := gpx.Waypoint{
					Point: gpx.PosPoint(route.Modes[globals.RAFT].Segments[0].Line.Start()),
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

func shouldEmitSection(mode globals.ModeType, section *Section) (bool, error) {
	// Work out if any of the regular routes have hiking / packrafting specific modes.
	var hasHikeMode, hasRaftMode bool
	for routeKey, route := range section.Routes {
		if routeKey.Required == globals.OPTIONAL {
			continue
		}
		if route.Modes[globals.HIKE] != nil {
			hasHikeMode = true
		}
		if route.Modes[globals.RAFT] != nil {
			hasRaftMode = true
		}
	}

	if section.Key.Suffix == "P" && mode == globals.HIKE {
		if hasHikeMode {
			// This packrafting section has a hiking mode. This should never happen, so return an error.
			return false, fmt.Errorf("regular route in section GPT%s has hiking mode", section.Key.Code())
		}
		// Exclude packrafting section from the hiking output.
		return false, nil
	}
	if section.Key.Suffix == "H" && mode == globals.RAFT {
		if hasRaftMode {
			// The hiking section has a packrafting mode, so we include it in the output. This happens for several
			// sections e.g. GPT36H.
			return true, nil
		}
		// This hiking section has no packrafting mode, so exclude from the output.
		return false, nil
	}
	return true, nil
}

type bySectionFiles struct {
	regular *gpx.Paged
	options *gpx.Paged
}

func (d *Data) SaveGaia(dpath string) error {
	logln("saving gaia files")

	bySection := map[globals.ModeType]map[globals.SectionKey]*bySectionFiles{}
	bySection[globals.RAFT] = map[globals.SectionKey]*bySectionFiles{}
	bySection[globals.HIKE] = map[globals.SectionKey]*bySectionFiles{}
	// initialise bySection map with gpx file for each section
	for _, mode := range globals.MODES {
		for _, key := range d.Keys {
			if globals.HAS_SINGLE && key != globals.SINGLE {
				continue
			}
			section := d.Sections[key]
			ok, err := shouldEmitSection(mode, section)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			bySection[mode][key] = &bySectionFiles{
				regular: &gpx.Paged{
					Max: 1000,
				},
				options: &gpx.Paged{
					Max: 1000,
				},
			}
		}
	}

	// routes
	{
		for _, mode := range globals.MODES {
			root := &gpx.Paged{
				Max: 1000,
			}
			for _, key := range d.Keys {
				if globals.HAS_SINGLE && key != globals.SINGLE {
					continue
				}
				section := d.Sections[key]

				ok, err := shouldEmitSection(mode, section)
				if err != nil {
					return err
				}
				if !ok {
					continue
				}
				bucket := &gpx.Bucket{
					Order: section.Routes[section.RouteKeys[0]].All[0].Line.Start().Lat,
				}
				bucketBySection := &gpx.Bucket{
					Order: section.Routes[section.RouteKeys[0]].All[0].Line.Start().Lat,
				}
				for _, routeKey := range section.RouteKeys {
					if routeKey.Required != globals.REGULAR {
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
					rte.Desc = H1_SYMBOL + " " + rte.Name + "\n\n"

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
					bucket.Routes = append(bucket.Routes, rte)
					bucketBySection.Routes = append(bucketBySection.Routes, rte)

					// start waypoint
					if route.Modes[globals.RAFT] != nil && route.Modes[globals.HIKE] != nil && !route.Modes[globals.RAFT].Segments[0].Line.Start().IsClose(route.Modes[globals.HIKE].Segments[0].Line.Start(), globals.DELTA) {
						// start of packrafting version is different to start of hiking version.
						if mode == globals.HIKE {
							wp1 := gpx.Waypoint{
								Point: gpx.PosPoint(route.Modes[globals.HIKE].Segments[0].Line.Start()),
								Name:  fmt.Sprintf("GPT%s%s %s", section.Key.Code(), route.Key.Direction, section.Name),
							}
							bucket.Waypoints = append(bucket.Waypoints, wp1)
							bucketBySection.Waypoints = append(bucketBySection.Waypoints, wp1)
						} else {
							wp2 := gpx.Waypoint{
								Point: gpx.PosPoint(route.Modes[globals.RAFT].Segments[0].Line.Start()),
								Name:  fmt.Sprintf("GPT%s%s %s", section.Key.Code(), route.Key.Direction, section.Name),
							}
							bucket.Waypoints = append(bucket.Waypoints, wp2)
							bucketBySection.Waypoints = append(bucketBySection.Waypoints, wp2)
						}
					} else {
						wp := gpx.Waypoint{
							Point: gpx.PosPoint(route.All[0].Line.Start()),
							Name:  fmt.Sprintf("GPT%s%s %s", section.Key.Code(), route.Key.Direction, section.Name),
						}
						bucket.Waypoints = append(bucket.Waypoints, wp)
						bucketBySection.Waypoints = append(bucketBySection.Waypoints, wp)
					}
				}
				root.Buckets = append(root.Buckets, bucket)
				bySection[mode][key].regular.Buckets = append(bySection[mode][key].regular.Buckets, bucketBySection)
			}
			var modeString string
			switch mode {
			case globals.HIKE:
				modeString = "Hiking"
			case globals.RAFT:
				modeString = "Packrafting"
			}
			if err := root.Save(filepath.Join(dpath, "GPX Files (For Gaia GPS app)", "Combined", fmt.Sprintf("%s routes.gpx", modeString))); err != nil {
				return fmt.Errorf("writing routes (%s) gpx: %w", modeString, err)
			}
		}
	}

	// options
	{
		for _, mode := range globals.MODES {
			root := &gpx.Paged{
				Max: 1000,
			}
			for _, key := range d.Keys {
				if globals.HAS_SINGLE && key != globals.SINGLE {
					continue
				}
				section := d.Sections[key]

				ok, err := shouldEmitSection(mode, section)
				if err != nil {
					return err
				}
				if !ok {
					continue
				}
				bucket := &gpx.Bucket{
					Order: section.Routes[section.RouteKeys[0]].All[0].Line.Start().Lat,
				}
				bucketBySection := &gpx.Bucket{
					Order: section.Routes[section.RouteKeys[0]].All[0].Line.Start().Lat,
				}
				for _, routeKey := range section.RouteKeys {
					if routeKey.Required != globals.OPTIONAL {
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
					trk.Desc = H1_SYMBOL + " " + trk.Name + "\n\n"

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
					bucket.Tracks = append(bucket.Tracks, trk)
					bucketBySection.Tracks = append(bucketBySection.Tracks, trk)
				}
				root.Buckets = append(root.Buckets, bucket)
				bySection[mode][key].options.Buckets = append(bySection[mode][key].options.Buckets, bucketBySection)
			}
			var modeString string
			switch mode {
			case globals.HIKE:
				modeString = "Hiking"
			case globals.RAFT:
				modeString = "Packrafting"
			}
			if err := root.Save(filepath.Join(dpath, "GPX Files (For Gaia GPS app)", "Combined", fmt.Sprintf("%s options.gpx", modeString))); err != nil {
				return fmt.Errorf("writing options (%s) gpx: %w", modeString, err)
			}
		}
	}

	// route by section
	for mode, bySectionMap := range bySection {
		for key, files := range bySectionMap {
			var modeString string
			switch mode {
			case globals.HIKE:
				modeString = "hiking"
			case globals.RAFT:
				modeString = "packrafting"
			}
			section := d.Sections[key]
			if err := files.regular.Save(filepath.Join(dpath, "GPX Files (For Gaia GPS app)", "By section", fmt.Sprintf("GPT%s %s (%s route).gpx", key.Code(), section.Name, modeString))); err != nil {
				return fmt.Errorf("writing routes by section (%s) gpx: %w", key.Code(), err)
			}
			if err := files.options.Save(filepath.Join(dpath, "GPX Files (For Gaia GPS app)", "By section", fmt.Sprintf("GPT%s %s (%s options).gpx", key.Code(), section.Name, modeString))); err != nil {
				return fmt.Errorf("writing routes by section (%s) gpx: %w", key.Code(), err)
			}
		}
	}

	// waypoints
	{
		root := &gpx.Paged{
			Max: 1000,
		}
		var waypointsByKey = map[globals.SectionKey]*gpx.Paged{}
		for _, key := range d.Keys {
			if globals.HAS_SINGLE && key != globals.SINGLE {
				continue
			}
			waypointsByKey[key] = &gpx.Paged{
				Max: 1000,
			}
			for _, w := range d.Sections[key].Waypoints {
				bucket := &gpx.Bucket{
					Order: w.Pos.Lat,
				}
				bucketByKey := &gpx.Bucket{
					Order: w.Pos.Lat,
				}
				wpt := gpx.Waypoint{
					Point: gpx.PosPoint(w.Pos),
					Name:  w.Name,
					Desc:  "GPT" + key.Code(),
				}
				bucket.Waypoints = append(bucket.Waypoints, wpt)
				bucketByKey.Waypoints = append(bucketByKey.Waypoints, wpt)
				root.Buckets = append(root.Buckets, bucket)
				waypointsByKey[key].Buckets = append(waypointsByKey[key].Buckets, bucketByKey)
			}
		}
		if err := root.Save(filepath.Join(dpath, "GPX Files (For Gaia GPS app)", "Combined", "Waypoints (routes).gpx")); err != nil {
			return fmt.Errorf("writing waypoints (routes) gpx: %w", err)
		}
		for key, paged := range waypointsByKey {
			section := d.Sections[key]
			if err := paged.Save(filepath.Join(dpath, "GPX Files (For Gaia GPS app)", "By section", fmt.Sprintf("GPT%s %s (waypoints).gpx", key.Code(), section.Name))); err != nil {
				return fmt.Errorf("writing waypoints by section (%s) gpx: %w", key.Code(), err)
			}
		}
	}

	wp := func(waypoints []Waypoint, name string, prefix string) error {
		root := &gpx.Paged{
			Max: 1000,
		}
		for _, w := range waypoints {
			bucket := &gpx.Bucket{
				Order: w.Pos.Lat,
			}
			bucket.Waypoints = append(bucket.Waypoints, gpx.Waypoint{
				Point: gpx.PosPoint(w.Pos),
				Name:  prefix + w.Name,
			})
			root.Buckets = append(root.Buckets, bucket)
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

	// areas
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
			if globals.HAS_SINGLE && key != globals.SINGLE {
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
	if !globals.DEBUG {
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
