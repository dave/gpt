package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/dave/gpt/geo"
	"github.com/dave/gpt/kml"
)

//func mergeSegments(n1, n2 *Network) ([]*SegmentData, error) {
//
//	var np, nh *Network
//	if n1.Route.Packrafting {
//		np = n1
//		nh = n2
//	} else {
//		nh = n1
//		np = n2
//	}
//	sp, sh := np.Segments, nh.Segments
//
//	var out []*SegmentData
//	all := map[string]bool{}
//	allP := map[string]*Segment{}
//	allH := map[string]*Segment{}
//	for _, segment := range sp {
//		allP[segment.Raw] = segment
//		if !all[segment.Raw] {
//			all[segment.Raw] = true
//			out = append(out, &SegmentData{Segment: segment})
//		}
//	}
//	for _, segment := range sh {
//		allH[segment.Raw] = segment
//		if !all[segment.Raw] {
//			all[segment.Raw] = true
//			out = append(out, &SegmentData{Segment: segment})
//		}
//	}
//	for _, data := range out {
//		if allP[data.Segment.Raw] != nil {
//			data.HasPackrafting = true
//			data.PackraftingFrom = allP[data.Segment.Raw].From
//		}
//		if allH[data.Segment.Raw] != nil {
//			data.HasHiking = true
//			data.HikingFrom = allH[data.Segment.Raw].From
//		}
//	}
//
//	// check reversal status of all segments
//	for raw := range all {
//		s1 := allP[raw]
//		s2 := allH[raw]
//		if s1 != nil && s2 != nil {
//			//if s1.Reversed != s2.Reversed {
//			//	switch s1.Raw {
//			//	case "EXP-OH-TL-I@21-03-#012", "EXP-OH-MR-V@21-03-#001", "EXP-OH-BB-A@28H-02B-#005":
//			//	// TODO - what to do here?
//			//	default:
//			//		//debugfln("segment %s reversal state not the same between hiking and packrafting", s1.Raw)
//			//		return nil, fmt.Errorf("segment %s reversal state not the same between hiking and packrafting", s1.Raw)
//			//	}
//			//}
//		}
//	}
//
//	orderMap := map[string]map[bool]int{}
//	for i, segment := range sp {
//		if orderMap[segment.Raw] == nil {
//			orderMap[segment.Raw] = map[bool]int{true: -1, false: -1}
//		}
//		orderMap[segment.Raw][true] = i
//	}
//	for i, segment := range sh {
//		if orderMap[segment.Raw] == nil {
//			orderMap[segment.Raw] = map[bool]int{true: -1, false: -1}
//		}
//		orderMap[segment.Raw][false] = i
//	}
//
//	var err error
//	sort.Slice(out, func(i, j int) bool {
//		i1 := orderMap[out[i].Segment.Raw][true]
//		j1 := orderMap[out[j].Segment.Raw][true]
//
//		i2 := orderMap[out[i].Segment.Raw][false]
//		j2 := orderMap[out[j].Segment.Raw][false]
//
//		if (i1 == -1 || j1 == -1) && (i2 == -1 || j2 == -1) {
//			return false
//		} else if i1 == -1 || j1 == -1 {
//			return i2 < j2
//		} else if i2 == -1 || j2 == -1 {
//			return i1 < j1
//		}
//
//		less1 := i1 < j1
//		less2 := i2 < j2
//		if less1 != less2 {
//			//debugfln("%q (#%d) and %q (#%d) in %q: %v", out[i], orderMap[out[i]][true], out[j], orderMap[out[j]][true], n1.Debug(), less1)
//			//debugfln("%q (#%d) and %q (#%d) in %q: %v", out[i], orderMap[out[i]][false], out[j], orderMap[out[j]][false], n2.Debug(), less2)
//			err = fmt.Errorf("incompatible segment order in %q and %q", n1.Debug(), n2.Debug())
//			return false
//		}
//		return less1
//	})
//	if err != nil {
//		return nil, err
//	}
//	return out, nil
//}
//
//func findNetworksWithMatchingSegments(n *Network, search []*Network) map[int]int {
//	count := map[int]int{}
//	for _, segment := range n.Segments {
//		for j, network := range search {
//			for _, s := range network.Segments {
//				if segment.Raw == s.Raw {
//					count[j]++
//				}
//			}
//		}
//	}
//	return count
//}

//var counter, counterRoutes int

func (d *Data) SaveMaster(dpath string, updateLegacy bool) error {
	logln("saving kml master")
	var renames []struct{ from, to string }

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
							routeHolderFolder = &kml.Folder{Name: fmt.Sprintf("Option %d", route.Key.Option)}
						}
						//routeFolder = &kml.Folder{Name: route.FolderName()}
						optionalRouteHolderFolders[route.Key.Option] = routeHolderFolder
						sectionFolder.Folders = append(sectionFolder.Folders, routeHolderFolder)
					}
					routeFolder = &kml.Folder{Name: route.FolderName()}
					routeHolderFolder.Folders = append(routeHolderFolder.Folders, routeFolder)
				default:
					panic("")
				}
				for _, segment := range route.All {
					legacy := segment.Legacy
					if updateLegacy {
						legacy = segment.PlacemarkName()
					}
					routeFolder.Placemarks = append(routeFolder.Placemarks, &kml.Placemark{
						Name:     segment.PlacemarkName(),
						Legacy:   legacy,
						Open:     1,
						StyleUrl: fmt.Sprintf("#%s", segment.Style()),
						LineString: &kml.LineString{
							Tessellate:  true,
							Coordinates: kml.LineCoordinates(segment.Line),
						},
					})
					if segment.Legacy != segment.PlacemarkName() {
						renames = append(renames, struct{ from, to string }{from: segment.Legacy, to: segment.PlacemarkName()})
					}
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

	startFolder, resupplyFolder, geographicFolder, importantFolder, waypointsFolder := d.getWaypointFolders()

	pointsFolder := &kml.Folder{
		Name: "Points",
		Folders: []*kml.Folder{
			importantFolder,
			waypointsFolder,
			startFolder,
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
		var sb strings.Builder
		for _, rename := range renames {
			sb.WriteString(fmt.Sprintf("%q, %q\n", rename.from, rename.to))
		}
		if err := ioutil.WriteFile(filepath.Join(dpath, "renames.txt"), []byte(sb.String()), 0666); err != nil {
			return fmt.Errorf("writing renames file: %w", err)
		}
	}
	return nil
}

func (d *Data) getWaypointFolders() (startFolder, resupplyFolder, geographicFolder, importantFolder, waypointsFolder *kml.Folder) {

	collect := func(waypoints []Waypoint, style string) []*kml.Placemark {
		var placemarks []*kml.Placemark
		for _, w := range waypoints {
			placemarks = append(placemarks, &kml.Placemark{
				StyleUrl: style,
				Name:     w.Name,
				Point:    kml.PosPoint(w.Pos),
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
	startFolder = &kml.Folder{
		Name: "Section Start Points",
	}

	for _, key := range d.Keys {
		section := d.Sections[key]
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
				startFolder.Placemarks = append(startFolder.Placemarks, &kml.Placemark{
					StyleUrl: fmt.Sprintf("#go"),
					Name:     fmt.Sprintf("GPT%s%s hiking (%s)", section.Key.Code(), route.Key.Direction, section.Name),
					Point:    kml.PosPoint(route.Modes[HIKE].Segments[0].Line.Start()),
				})
				startFolder.Placemarks = append(startFolder.Placemarks, &kml.Placemark{
					StyleUrl: fmt.Sprintf("#go"),
					Name:     fmt.Sprintf("GPT%s%s packrafting (%s)", section.Key.Code(), route.Key.Direction, section.Name),
					Point:    kml.PosPoint(route.Modes[RAFT].Segments[0].Line.Start()),
				})
			} else {
				startFolder.Placemarks = append(startFolder.Placemarks, &kml.Placemark{
					StyleUrl: fmt.Sprintf("#go"),
					Name:     fmt.Sprintf("GPT%s%s (%s)", section.Key.Code(), route.Key.Direction, section.Name),
					Point:    kml.PosPoint(route.All[0].Line.Start()),
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
		if len(section.Waypoints) == 0 {
			continue
		}
		sectionFolder := &kml.Folder{
			Name: section.FolderName(),
		}
		for _, w := range section.Waypoints {
			sectionFolder.Placemarks = append(sectionFolder.Placemarks, &kml.Placemark{
				StyleUrl: "#ylw-blank",
				Name:     w.Name,
				Point:    kml.PosPoint(w.Pos),
			})
		}
		waypointsFolder.Folders = append(waypointsFolder.Folders, sectionFolder)
	}

	return
}

func (d *Data) SaveKmlWaypoints(dpath string, stamp string) error {
	logln("Saving kml waypoints")
	/*
		debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Waypoints/All Points.kmz", "input-all.txt")
		debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Waypoints/Important Infromation.kmz", "input-important.txt")
		debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Waypoints/Optional Start and End Points.kmz", "input-optional-start.txt")
		debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Waypoints/Regular Start and End Points.kmz", "input-regular-start.txt")
		debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Waypoints/Resupply Locations.kmz", "input-resupply.txt")
		debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Waypoints/Waypoints.kmz", "input-waypoints.txt")
	*/

	startFolder, resupplyFolder, geographicFolder, importantFolder, waypointsFolder := d.getWaypointFolders()

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
						startFolder,
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
			Name:    "Section Start Points.kmz",
			Folders: []*kml.Folder{startFolder},
		},
	}
	if err := regStart.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "Section Start Points.kmz")); err != nil {
		return fmt.Errorf("saving Section Start Points.kmz: %w", err)
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
	decorations := []string{"blank", "stars", "circle"}
	for _, colour := range colours {
		for _, decoration := range decorations {
			d.Styles = append(d.Styles, addStyle(fmt.Sprintf("%s-%s", colour, decoration)))
		}
	}
	d.Styles = append(d.Styles, addStyle("go"))
}

//func (d *Data) SaveKmlTracks(dpath string, stamp string) error {
//	logln("Saving kml tracks")
//	//debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Tracks/Regular Tracks.kmz", "input-regular.txt")
//	//debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Tracks/Optional Tracks.kmz", "input-optional.txt")
//	//debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Tracks/All Tracks.kmz", "input-all.txt")
//	do := func(name string, filter func(Track) bool) (*kml.Folder, error) {
//		f := &kml.Folder{Name: name, Open: 1}
//		for _, key := range d.Keys {
//			if HAS_SINGLE && key != SINGLE {
//				continue
//			}
//			section := d.Sections[key]
//			sectionFolder := &kml.Folder{
//				Name: section.String(),
//				Open: 1,
//			}
//			for _, track := range section.Tracks {
//				if name == "Regular Tracks" && track.Optional {
//					continue
//				} else if name == "Optional Tracks" && !track.Optional {
//					continue
//				}
//				trackFolder := &kml.Folder{
//					Name: track.String(),
//					Open: 1,
//				}
//				for _, segment := range track.Segments {
//					trackFolder.Placemarks = append(trackFolder.Placemarks, &kml.Placemark{
//						Name:     segment.String(),
//						Open:     1,
//						StyleUrl: fmt.Sprintf("#%s", segment.Style()),
//						LineString: &kml.LineString{
//							Tessellate:  true,
//							Coordinates: kml.LineCoordinates(segment.Line),
//						},
//					})
//				}
//				if len(trackFolder.Placemarks) > 0 {
//					sectionFolder.Folders = append(sectionFolder.Folders, trackFolder)
//				}
//			}
//			if len(sectionFolder.Folders) > 0 {
//				f.Folders = append(f.Folders, sectionFolder)
//			}
//		}
//		return f, nil
//	}
//	regularFolder, err := do("Regular Tracks", func(track Track) bool { return !track.Optional })
//	if err != nil {
//		return fmt.Errorf("building regular tracks kml: %w", err)
//	}
//	optionalFolder, err := do("Optional Tracks", func(track Track) bool { return track.Optional })
//	if err != nil {
//		return fmt.Errorf("building optional tracks kml: %w", err)
//	}
//	all := kml.Root{
//		Xmlns: "http://www.opengis.net/kml/2.2",
//		Document: kml.Document{
//			Name: "All Tracks.kmz",
//			Open: 1,
//			Folders: []*kml.Folder{
//				{
//					Name:    "Tracks",
//					Open:    1,
//					Folders: []*kml.Folder{regularFolder, optionalFolder},
//				},
//			},
//		},
//	}
//	addSegmentStyles(&all.Document)
//	if err := all.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "All Tracks.kmz")); err != nil {
//		return fmt.Errorf("saving All Tracks.kmz: %w", err)
//	}
//	regular := kml.Root{
//		Xmlns: "http://www.opengis.net/kml/2.2",
//		Document: kml.Document{
//			Name:    "Regular Tracks.kmz",
//			Open:    1,
//			Folders: []*kml.Folder{regularFolder},
//		},
//	}
//	addSegmentStyles(&regular.Document)
//	if err := regular.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "Regular Tracks.kmz")); err != nil {
//		return fmt.Errorf("saving Regular Tracks.kmz: %w", err)
//	}
//	optional := kml.Root{
//		Xmlns: "http://www.opengis.net/kml/2.2",
//		Document: kml.Document{
//			Name:    "Optional Tracks.kmz",
//			Open:    1,
//			Folders: []*kml.Folder{optionalFolder},
//		},
//	}
//	addSegmentStyles(&optional.Document)
//	if err := optional.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "Optional Tracks.kmz")); err != nil {
//		return fmt.Errorf("saving Optional Tracks.kmz: %w", err)
//	}
//
//	//debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "All Tracks.kmz"), "output-all.txt")
//	//debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "Regular Tracks.kmz"), "output-regular.txt")
//	//debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "Optional Tracks.kmz"), "output-optional.txt")
//
//	return nil
//}

//func (d *Data) SaveGpx(dpath string, stamp string) error {
//	logln("Saving gpx files")
//	type matcher struct {
//		path     []string
//		match    func(*Segment) bool
//		segments []*Segment
//	}
//	matchers := []*matcher{
//		{
//			path:  []string{"Combined Tracks", fmt.Sprintf("All Optional and Regular Tracks (%s).gpx", stamp)},
//			match: func(s *Segment) bool { return true },
//		},
//		{
//			path:  []string{"Combined Tracks", fmt.Sprintf("Optional Tracks (%s).gpx", stamp)},
//			match: func(s *Segment) bool { return s.Track.Optional },
//		},
//		{
//			path:  []string{"Combined Tracks", fmt.Sprintf("Regular Tracks (%s).gpx", stamp)},
//			match: func(s *Segment) bool { return !s.Track.Optional },
//		},
//
//		{
//			path: []string{"Exploration Hiking Tracks", "Optional Tracks", "EXP-OH-LD-A.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "OH" && s.Verification == "A"
//			},
//		},
//		{
//			path: []string{"Exploration Hiking Tracks", "Optional Tracks", "EXP-OH-LD-I.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "OH" && s.Verification == "I"
//			},
//		},
//		{
//			path: []string{"Exploration Hiking Tracks", "Optional Tracks", "EXP-OH-LD-V.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "OH" && s.Verification == "V"
//			},
//		},
//
//		{
//			path: []string{"Exploration Hiking Tracks", "Regular Tracks", "EXP-RH-LD-A.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "RH" && s.Verification == "A"
//			},
//		},
//		{
//			path: []string{"Exploration Hiking Tracks", "Regular Tracks", "EXP-RH-LD-I.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "RH" && s.Verification == "I"
//			},
//		},
//		{
//			path: []string{"Exploration Hiking Tracks", "Regular Tracks", "EXP-RH-LD-V.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "RH" && s.Verification == "V"
//			},
//		},
//
//		{
//			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OH-LD-A.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "OH" && s.Verification == "A"
//			},
//		},
//		{
//			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OH-LD-I.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "OH" && s.Verification == "I"
//			},
//		},
//		{
//			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OH-LD-V.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "OH" && s.Verification == "V"
//			},
//		},
//		{
//			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-LD-A.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "OP" && s.Verification == "A"
//			},
//		},
//		{
//			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-LD-I.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "OP" && s.Verification == "I"
//			},
//		},
//		{
//			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-LD-V.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "OP" && s.Verification == "V"
//			},
//		},
//		{
//			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-WR-1.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "OP" && s.Directional == "1"
//			},
//		},
//		{
//			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-WR-2.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "OP" && s.Directional == "2"
//			},
//		},
//
//		{
//			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-RH-LD-A.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "RH" && s.Verification == "A"
//			},
//		},
//		{
//			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-RH-LD-I.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "RH" && s.Verification == "I"
//			},
//		},
//		{
//			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-RH-LD-V.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "RH" && s.Verification == "V"
//			},
//		},
//
//		{
//			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-LD-A.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "RP" && s.Verification == "A"
//			},
//		},
//		{
//			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-LD-I.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "RP" && s.Verification == "I"
//			},
//		},
//		{
//			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-LD-V.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "RP" && s.Verification == "V"
//			},
//		},
//		{
//			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-WR-1.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "RP" && s.Directional == "1"
//			},
//		},
//		{
//			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-WR-2.gpx"},
//			match: func(s *Segment) bool {
//				return s.Experimental && s.Code == "RP" && s.Directional == "2"
//			},
//		},
//
//		{
//			path: []string{"Hiking Tracks", "Optional Tracks", "OH-FY-1.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OH" && HasFerry(s.Terrains) && s.Directional == "1"
//			},
//		},
//		{
//			path: []string{"Hiking Tracks", "Optional Tracks", "OH-FY-2.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OH" && HasFerry(s.Terrains) && s.Directional == "2"
//			},
//		},
//		{
//			path: []string{"Hiking Tracks", "Optional Tracks", "OH-LD-A.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OH" && s.Verification == "A"
//			},
//		},
//		{
//			path: []string{"Hiking Tracks", "Optional Tracks", "OH-LD-I.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OH" && s.Verification == "I"
//			},
//		},
//		{
//			path: []string{"Hiking Tracks", "Optional Tracks", "OH-LD-V.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OH" && s.Verification == "V"
//			},
//		},
//
//		{
//			path: []string{"Hiking Tracks", "Regular Tracks", "RH-FY-1.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RH" && HasFerry(s.Terrains) && s.Directional == "1"
//			},
//		},
//		{
//			path: []string{"Hiking Tracks", "Regular Tracks", "RH-FY-2.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RH" && HasFerry(s.Terrains) && s.Directional == "2"
//			},
//		},
//		{
//			path: []string{"Hiking Tracks", "Regular Tracks", "RH-LD-A.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RH" && s.Verification == "A"
//			},
//		},
//		{
//			path: []string{"Hiking Tracks", "Regular Tracks", "RH-LD-I.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RH" && s.Verification == "I"
//			},
//		},
//		{
//			path: []string{"Hiking Tracks", "Regular Tracks", "RH-LD-V.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RH" && s.Verification == "V"
//			},
//		},
//
//		{
//			path: []string{"Hiking Tracks", "Regular Tracks", "RR-FY-1.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RR" && HasFerry(s.Terrains) && s.Directional == "1"
//			},
//		},
//		{
//			path: []string{"Hiking Tracks", "Regular Tracks", "RR-FY-2.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RR" && HasFerry(s.Terrains) && s.Directional == "2"
//			},
//		},
//		{
//			path: []string{"Hiking Tracks", "Regular Tracks", "RR-LD-A.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RR" && s.Verification == "A"
//			},
//		},
//		{
//			path: []string{"Hiking Tracks", "Regular Tracks", "RR-LD-I.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RR" && s.Verification == "I"
//			},
//		},
//		{
//			path: []string{"Hiking Tracks", "Regular Tracks", "RR-LD-V.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RR" && s.Verification == "V"
//			},
//		},
//
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-FY-1.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OH" && HasFerry(s.Terrains) && s.Directional == "1"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-FY-2.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OH" && HasFerry(s.Terrains) && s.Directional == "2"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-LD-A.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OH" && s.Verification == "A"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-LD-I.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OH" && s.Verification == "I"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-LD-V.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OH" && s.Verification == "V"
//			},
//		},
//
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-FY-1.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OP" && HasFerry(s.Terrains) && s.Directional == "1"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-FY-2.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OP" && HasFerry(s.Terrains) && s.Directional == "2"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-LD-A.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OP" && s.Verification == "A"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-LD-I.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OP" && s.Verification == "I"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-LD-V.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OP" && s.Verification == "V"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-WR-1.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OP" && s.Directional == "1"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-WR-2.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "OP" && s.Directional == "2"
//			},
//		},
//
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-FY-1.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RH" && HasFerry(s.Terrains) && s.Directional == "1"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-FY-2.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RH" && HasFerry(s.Terrains) && s.Directional == "2"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-LD-A.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RH" && s.Verification == "A"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-LD-I.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RH" && s.Verification == "I"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-LD-V.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RH" && s.Verification == "V"
//			},
//		},
//
//		{
//			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-FY-1.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RP" && HasFerry(s.Terrains) && s.Directional == "1"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-FY-2.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RP" && HasFerry(s.Terrains) && s.Directional == "2"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-LD-A.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RP" && s.Verification == "A"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-LD-I.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RP" && s.Verification == "I"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-LD-V.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RP" && s.Verification == "V"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-WR-1.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RP" && s.Directional == "1"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-WR-2.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RP" && s.Directional == "2"
//			},
//		},
//
//		{
//			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-FY-1.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RR" && HasFerry(s.Terrains) && s.Directional == "1"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-FY-2.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RR" && HasFerry(s.Terrains) && s.Directional == "2"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-LD-A.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RR" && s.Verification == "A"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-LD-I.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RR" && s.Verification == "I"
//			},
//		},
//		{
//			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-LD-V.gpx"},
//			match: func(s *Segment) bool {
//				return !s.Experimental && s.Code == "RR" && s.Verification == "V"
//			},
//		},
//	}
//
//	for _, section := range d.Sections {
//		for _, track := range section.Tracks {
//			for _, segment := range track.Segments {
//				for _, m := range matchers {
//					if m.match(segment) {
//						m.segments = append(m.segments, segment)
//					}
//				}
//			}
//		}
//	}
//
//	for _, m := range matchers {
//
//		if len(m.segments) == 0 {
//			continue
//		}
//
//		g := gpx.Root{}
//		for _, segment := range m.segments {
//			g.Tracks = append(g.Tracks, gpx.Track{
//				Name:     segment.String(),
//				Segments: []gpx.TrackSegment{{Points: gpx.LineTrackPoints(segment.Line)}},
//			})
//		}
//		fpath := filepath.Join(append([]string{dpath, "GPX Files (For Smartphones and Basecamp)"}, m.path...)...)
//		if err := g.Save(fpath); err != nil {
//			return fmt.Errorf("saving %q: %w", m.path[len(m.path)-1], err)
//		}
//	}
//
//	wpAll := gpx.Root{}
//	for _, key := range d.Keys {
//		if HAS_SINGLE && key != SINGLE {
//			continue
//		}
//		section := d.Sections[key]
//		for _, w := range section.Waypoints {
//			wpAll.Waypoints = append(wpAll.Waypoints, gpx.Waypoint{
//				Point: gpx.PosPoint(w.Pos),
//				Name:  w.Name,
//			})
//		}
//	}
//
//	if err := wpAll.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("All Other Waypoints (%s).gpx", stamp))); err != nil {
//		return fmt.Errorf("saving waypoints: %w", err)
//	}
//
//	wpImp := gpx.Root{}
//	for _, w := range d.Important {
//		wpImp.Waypoints = append(wpImp.Waypoints, gpx.Waypoint{
//			Point: gpx.PosPoint(w.Pos),
//			Name:  w.Name,
//		})
//	}
//	if err := wpImp.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("Important Infromation (%s).gpx", stamp))); err != nil {
//		return fmt.Errorf("saving waypoints: %w", err)
//	}
//
//	wpRes := gpx.Root{}
//	for _, w := range d.Resupplies {
//		wpRes.Waypoints = append(wpRes.Waypoints, gpx.Waypoint{
//			Point: gpx.PosPoint(w.Pos),
//			Name:  w.Name,
//		})
//	}
//	if err := wpRes.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("Resupply Locations (%s).gpx", stamp))); err != nil {
//		return fmt.Errorf("saving waypoints: %w", err)
//	}
//
//	wpRegTerminators := gpx.Root{}
//	wpOptTerminators := gpx.Root{}
//	for _, t := range d.Terminators {
//		if t.Option == "" {
//			wpRegTerminators.Waypoints = append(wpRegTerminators.Waypoints, gpx.Waypoint{
//				Point: gpx.PosPoint(t.Pos),
//				Name:  t.String(),
//			})
//		} else {
//			wpOptTerminators.Waypoints = append(wpOptTerminators.Waypoints, gpx.Waypoint{
//				Point: gpx.PosPoint(t.Pos),
//				Name:  t.String(),
//			})
//		}
//	}
//	if err := wpRegTerminators.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("Regular Start and End Points (%s).gpx", stamp))); err != nil {
//		return fmt.Errorf("saving waypoints: %w", err)
//	}
//	if err := wpOptTerminators.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("Optional Start and End Points (%s).gpx", stamp))); err != nil {
//		return fmt.Errorf("saving waypoints: %w", err)
//	}
//
//	if err := ioutil.WriteFile(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Nomenclature.txt"), []byte(Nomenclature), 0666); err != nil {
//		return fmt.Errorf("saving Nomenclature.txt: %w", err)
//	}
//
//	return nil
//}

//func (d *Data) SaveGaia(dpath string) error {
//	logln("Saving gaia files")
//	type clusterStruct struct {
//		name     string
//		from, to int
//		modes    map[string]map[string]*gpx.Root
//	}
//	newContents := func() map[string]*gpx.Root {
//		return map[string]*gpx.Root{"routes": {}, "options": {}, "routes-markers": {}, "options-markers": {}, "waypoints": {}}
//	}
//	newModes := func() map[string]map[string]*gpx.Root {
//		return map[string]map[string]*gpx.Root{"hiking": newContents(), "packrafting": newContents()}
//	}
//	clusters := []clusterStruct{
//		{name: "north", from: 1, to: 16, modes: newModes()},
//		{name: "south", from: 17, to: 39, modes: newModes()},
//		{name: "extensions", from: 40, to: 99, modes: newModes()},
//		{name: "all", from: 1, to: 99, modes: newModes()},
//	}
//
//	for _, cluster := range clusters {
//
//		count := map[string][]*Section{}
//		for mode := range cluster.modes {
//			count[mode] = []*Section{}
//		}
//
//		for _, key := range d.Keys {
//			if HAS_SINGLE && key != SINGLE {
//				continue
//			}
//			section := d.Sections[key]
//			if section.Key.Number < cluster.from || section.Key.Number > cluster.to {
//				continue
//			}
//			for mode, contents := range cluster.modes {
//				var bundle *Bundle
//				if mode == "hiking" {
//					bundle = section.Hiking
//				} else if mode == "packrafting" {
//					bundle = section.Packrafting
//				}
//				if bundle == nil {
//					continue
//				}
//				count[mode] = append(count[mode], section)
//
//				for key, route := range bundle.Regular {
//					if len(route.Networks) != 1 {
//						return fmt.Errorf("regular route %s has %d networks. regular routes should only have 1 network", route.Debug(), len(route.Networks))
//					}
//					network := route.Networks[0]
//
//					var rte gpx.Route
//					var direction string
//					if key.Direction == "N" {
//						direction = " northbound"
//					} else if key.Direction == "S" {
//						direction = " southbound"
//					}
//					rte.Name = fmt.Sprintf("GPT%v %s%s", section.Key.Code(), section.Name, direction)
//					rte.Desc = HEADING_SYMBOL + " " + rte.Name + "\n\n"
//
//					var lines []geo.Line
//					for _, segment := range network.Segments {
//						lines = append(lines, segment.Line)
//					}
//
//					var id int
//					for i, straight := range network.Straights {
//						if i > 0 {
//							rte.Desc += "---\n"
//						}
//						for _, flush := range straight.Flushes {
//							id++
//							rte.Desc += flush.Description(id, false) + "\n"
//							wp := gpx.Waypoint{
//								Point: gpx.PosPoint(flush.Segments[0].Line.Start()),
//								Name:  flush.Description(id, true),
//								Desc:  rte.Name,
//							}
//							contents["routes-markers"].Waypoints = append(contents["routes-markers"].Waypoints, wp)
//						}
//					}
//
//					rte.Points = gpx.LinePoints(geo.MergeLines(lines))
//					if mode == "hiking" {
//						rte.Desc += "\n" + section.Hiking.Scraped
//					} else {
//						rte.Desc += "\n" + section.Packrafting.Scraped
//					}
//					contents["routes"].Routes = append(contents["routes"].Routes, rte)
//				}
//
//				for _, route := range bundle.Options {
//					for i, network := range route.Networks {
//						var networkString string
//						if len(route.Networks) > 1 {
//							networkString = fmt.Sprintf(" (%d/%d)", i+1, len(route.Networks))
//						}
//						var trk gpx.Track
//						if route.OptionalKey.Alternatives {
//							var direction string
//							if route.OptionalKey.Direction == "N" {
//								direction = " northbound"
//							} else if route.OptionalKey.Direction == "S" {
//								direction = " southbound"
//							}
//							trk.Name = fmt.Sprintf("GPT%v%s hiking alternatives%s", route.Section.Key.Code(), direction, networkString)
//						} else if route.OptionalKey.Option == 0 {
//							trk.Name = fmt.Sprintf("GPT%v variant %v%s", route.Section.Key.Code(), route.OptionalKey.Code(), networkString)
//						} else {
//							trk.Name = fmt.Sprintf("GPT%v option %v%s", route.Section.Key.Code(), route.OptionalKey.Code(), networkString)
//						}
//						trk.Desc = HEADING_SYMBOL + " " + trk.Name + "\n\n"
//
//						var id int
//						for i, straight := range network.Straights {
//							if i > 0 {
//								trk.Desc += "---\n"
//							}
//							for _, flush := range straight.Flushes {
//								id++
//								trk.Desc += flush.Description(id, false) + "\n"
//								wp := gpx.Waypoint{
//									Point: gpx.PosPoint(flush.Segments[0].Line.Start()),
//									Name:  flush.Description(id, true),
//									Desc:  trk.Name,
//								}
//								contents["options-markers"].Waypoints = append(contents["options-markers"].Waypoints, wp)
//							}
//						}
//
//						for _, segment := range network.Segments {
//							trk.Segments = append(trk.Segments, gpx.TrackSegment{Points: gpx.LineTrackPoints(segment.Line)})
//						}
//						contents["options"].Tracks = append(contents["options"].Tracks, trk)
//					}
//				}
//
//				// reverse the order of all tracks and routes (better in Gaia like this)
//				//for i, j := 0, len(contents["options"].Tracks)-1; i < j; i, j = i+1, j-1 {
//				//	contents["options"].Tracks[i], contents["options"].Tracks[j] = contents["options"].Tracks[j], contents["options"].Tracks[i]
//				//}
//				//for i, j := 0, len(contents["routes"].Routes)-1; i < j; i, j = i+1, j-1 {
//				//	contents["routes"].Routes[i], contents["routes"].Routes[j] = contents["routes"].Routes[j], contents["routes"].Routes[i]
//				//}
//
//				for _, w := range section.Waypoints {
//					contents["waypoints"].Waypoints = append(contents["waypoints"].Waypoints, gpx.Waypoint{
//						Point: gpx.PosPoint(w.Pos),
//						Name:  w.Name,
//						Desc:  "GPT" + key.Code(),
//					})
//				}
//			}
//		}
//		for mode := range cluster.modes {
//			sections := count[mode]
//		Outer:
//			for _, terminator := range d.Terminators {
//				for _, section := range sections {
//					for _, key := range terminator.Sections {
//						if key == section.Key {
//							var desc string
//							for i, key := range terminator.Sections {
//								if i > 0 {
//									desc += ", "
//								}
//								desc += fmt.Sprintf("GPT%s", key.Code())
//							}
//							if terminator.Option != "" {
//								desc += fmt.Sprintf(" option %s", terminator.Option)
//							}
//							wp := gpx.Waypoint{
//								Point: gpx.PosPoint(terminator.Pos),
//								Name:  terminator.Name,
//								Desc:  desc,
//							}
//							if terminator.Option == "" {
//								cluster.modes[mode]["routes"].Waypoints = append(cluster.modes[mode]["routes"].Waypoints, wp)
//							} else {
//								cluster.modes[mode]["options"].Waypoints = append(cluster.modes[mode]["options"].Waypoints, wp)
//							}
//							continue Outer
//						}
//					}
//				}
//			}
//		}
//	}
//
//	for _, cluster := range clusters {
//		for mode, modeMap := range cluster.modes {
//			for contents, root := range modeMap {
//
//				name := fmt.Sprintf("%s-%s-%s.gpx", cluster.name, mode, contents)
//				if err := root.Save(filepath.Join(dpath, "GPX Files (For Gaia GPS app)", name)); err != nil {
//					return fmt.Errorf("writing gpx")
//				}
//			}
//		}
//	}
//	wp := func(waypoints []Waypoint, name string, prefix string) error {
//		root := &gpx.Root{}
//		for _, w := range waypoints {
//			root.Waypoints = append(root.Waypoints, gpx.Waypoint{
//				Point: gpx.PosPoint(w.Pos),
//				Name:  prefix + w.Name,
//			})
//		}
//		return root.Save(filepath.Join(dpath, "GPX Files (For Gaia GPS app)", name))
//	}
//	if err := wp(d.Resupplies, "waypoints-resupplies.gpx", "Resupply: "); err != nil {
//		return fmt.Errorf("writing resupplies gpx: %w", err)
//	}
//	if err := wp(d.Important, "waypoints-important.gpx", "Important: "); err != nil {
//		return fmt.Errorf("writing important gpx: %w", err)
//	}
//
//	if false {
//
//		spfr := &kml.Folder{
//			Name:       "Regular tracks",
//			Visibility: 1,
//			Open:       0,
//			Placemarks: nil,
//		}
//		spfo := &kml.Folder{
//			Name:       "Optional tracks",
//			Visibility: 1,
//			Open:       0,
//			Placemarks: nil,
//		}
//		sp := kml.Root{
//			Xmlns: "http://www.opengis.net/kml/2.2",
//			Document: kml.Document{
//				Name: "Route start points.kmz",
//				Folders: []*kml.Folder{
//					spfr,
//					spfo,
//				},
//			},
//		}
//		processRoute := func(pr, hr *Route) error {
//
//			var r *Route
//			if pr != nil {
//				r = pr
//			} else {
//				r = hr
//			}
//			rf := &kml.Folder{Name: r.String()}
//
//			separateFolders := pr != nil && hr != nil && !pr.HasIdenticalNetworks(hr)
//			combineRoutes := pr != nil && hr != nil && pr.HasIdenticalNetworks(hr)
//
//			addContents := func(f *kml.Folder, r *Route, suffix string) {
//				for _, n := range r.Networks {
//
//					name := n.String()
//					if suffix != "" {
//						name += " " + suffix
//					}
//					f.Placemarks = append(f.Placemarks, &kml.Placemark{
//						Name:       name,
//						Visibility: 0,
//						Open:       0,
//						Point:      kml.PosPoint(n.Entry.Line.Start()),
//					})
//
//					net := &kml.Placemark{
//						Name:          n.String(),
//						Visibility:    0,
//						Open:          0,
//						MultiGeometry: &kml.MultiGeometry{},
//						Style: &kml.Style{LineStyle: &kml.LineStyle{
//							Color: "ffffffff",
//							Width: 10,
//						}},
//					}
//					for _, segment := range n.Segments {
//						net.MultiGeometry.LineStrings = append(net.MultiGeometry.LineStrings, &kml.LineString{
//							Tessellate:  true,
//							Coordinates: kml.LineCoordinates(segment.Line),
//						})
//					}
//					f.Placemarks = append(f.Placemarks, net)
//				}
//			}
//
//			if separateFolders {
//				pf := &kml.Folder{Name: "packrafting"}
//				hf := &kml.Folder{Name: "hiking"}
//				addContents(pf, pr, "packrafting")
//				addContents(hf, hr, "hiking")
//				rf.Folders = append(rf.Folders, pf)
//				rf.Folders = append(rf.Folders, hf)
//			} else {
//				if combineRoutes {
//					// both routes have identical networks, so only need to do one of them. either will do.
//					addContents(rf, pr, "")
//				} else if pr != nil {
//					addContents(rf, pr, "")
//				} else if hr != nil {
//					addContents(rf, hr, "")
//				} else {
//					panic("shouldn't be here")
//				}
//			}
//
//			if r.Regular {
//				spfr.Folders = append(spfr.Folders, rf)
//			} else {
//				spfo.Folders = append(spfo.Folders, rf)
//			}
//
//			return nil
//		}
//		if err := d.ForRoutePairs(processRoute); err != nil {
//			return err
//		}
//		return sp.Save(filepath.Join(dpath, "start-points.kmz"))
//	}
//	return nil
//}

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
