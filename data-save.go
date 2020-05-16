package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dave/gpt/geo"
	"github.com/dave/gpt/gpx"
	"github.com/dave/gpt/kml"
)

type SegmentData struct {
	Segment                     *Segment
	HasPackrafting, HasHiking   bool
	PackraftingFrom, HikingFrom float64
}

type Mode int

const HIKE Mode = 1
const RAFT Mode = 2

var MODES = []Mode{HIKE, RAFT}

func (d SegmentData) PlacemarkName(networkId, networksLength int, n *Network) string {
	var b strings.Builder
	s := d.Segment

	if s.Experimental {
		b.WriteString("EXP")
		b.WriteString("-")
	}
	b.WriteString(s.Code)
	b.WriteString("-")
	b.WriteString(strings.Join(s.Terrains, "&"))
	if s.Verification != "" || s.Directional != "" {
		b.WriteString("-")
		b.WriteString(s.Verification)
		b.WriteString(s.Directional)
	}

	b.WriteString(" {")
	b.WriteString(n.Route.Section.Key.Code())
	b.WriteString(n.Route.RegularKey.Direction)
	if !n.Route.Regular {
		b.WriteString("-")
		if n.Route.OptionalKey.Option > 0 {
			b.WriteString(fmt.Sprintf("%02d", n.Route.OptionalKey.Option))
		}
		b.WriteString(n.Route.OptionalKey.Variant)
		if networksLength > 1 {
			b.WriteString(string('a' + networkId))
		}
	}
	b.WriteString("}")

	b.WriteString(" [")
	if d.HasHiking && d.HasPackrafting {
		if fmt.Sprintf("%.1f", d.PackraftingFrom) == fmt.Sprintf("%.1f", d.HikingFrom) {
			b.WriteString(fmt.Sprintf("%.1f", d.PackraftingFrom))
		} else {
			b.WriteString(fmt.Sprintf("%.1f", d.PackraftingFrom))
			b.WriteString("/")
			b.WriteString(fmt.Sprintf("%.1f", d.HikingFrom))
		}
	} else if d.HasPackrafting {
		b.WriteString(fmt.Sprintf("%.1f", d.PackraftingFrom))
	} else if d.HasHiking {
		b.WriteString(fmt.Sprintf("%.1f", d.HikingFrom))
	}
	b.WriteString("+")
	b.WriteString(fmt.Sprintf("%.1f", s.Length))
	b.WriteString("]")

	if s.Name != "" {
		b.WriteString(" (")
		b.WriteString(s.Name)
		b.WriteString(")")
	}
	return b.String()
}

func mergeSegments(n1, n2 *Network) ([]*SegmentData, error) {

	var np, nh *Network
	if n1.Route.Packrafting {
		np = n1
		nh = n2
	} else {
		nh = n1
		np = n2
	}
	sp, sh := np.Segments, nh.Segments

	//var out []*SegmentData
	all := map[string]bool{}
	allP := map[string]*Segment{}
	allH := map[string]*Segment{}
	for _, segment := range sp {
		allP[segment.Raw] = segment
		if !all[segment.Raw] {
			all[segment.Raw] = true
			//out = append(out, &SegmentData{Segment: segment})
		}
	}
	for _, segment := range sh {
		allH[segment.Raw] = segment
		if !all[segment.Raw] {
			all[segment.Raw] = true
			//out = append(out, &SegmentData{Segment: segment})
		}
	}

	// check reversal status of all segments
	for raw := range all {
		s1 := allP[raw]
		s2 := allH[raw]
		if s1 != nil && s2 != nil {
			if s1.Reversed != s2.Reversed {
				switch s1.Raw {
				case "EXP-OH-TL-I@21-03-#012", "EXP-OH-MR-V@21-03-#001", "EXP-OH-BB-A@28H-02B-#005":
				// TODO - what to do here?
				default:
					//debugfln("segment %s reversal state not the same between hiking and packrafting", s1.Raw)
					return nil, fmt.Errorf("segment %s reversal state not the same between hiking and packrafting", s1.Raw)
				}
			}
		}
	}

	search := func(segments []*Segment, from int, raw string) (found bool, index int) {
		for i := from; i < len(segments); i++ {
			if segments[i].Raw == raw {
				return true, i
			}
		}
		return false, 0
	}
	var ordered []*Segment
	segments := map[Mode][]*Segment{HIKE: sh, RAFT: sp}
	index := map[Mode]int{HIKE: 0, RAFT: 0}
	mode := HIKE
	for index[HIKE] < len(segments[HIKE]) || index[RAFT] < len(segments[RAFT]) {
		var opposite Mode
		if mode == HIKE {
			opposite = RAFT
		} else {
			opposite = HIKE
		}

		if index[mode] >= len(segments[mode]) {
			// if we've reached the end of mode, drain opposite
			for i := index[opposite]; i < len(segments[opposite]); i++ {
				ordered = append(ordered, segments[opposite][i])
				index[opposite]++
			}
			break
		}

		current := segments[mode][index[mode]]
		found, foundIndex := search(segments[opposite], index[opposite], current.Raw)
		if !found {
			ordered = append(ordered, current)
			index[mode]++
		} else {
			for i := index[opposite]; i <= foundIndex; i++ {
				ordered = append(ordered, segments[opposite][i])
				index[opposite]++
			}
			index[mode]++
			//index[opposite]++
		}
		//mode = opposite
	}

	var out []*SegmentData
	for _, segment := range ordered {
		data := &SegmentData{
			Segment: segment,
		}
		if allP[segment.Raw] != nil {
			data.HasPackrafting = true
			data.PackraftingFrom = allP[segment.Raw].From
		}
		if allH[segment.Raw] != nil {
			data.HasHiking = true
			data.HikingFrom = allH[segment.Raw].From
		}
		out = append(out, data)
	}

	//fmt.Println("***")
	//fmt.Println("P")
	//for i, s := range sp {
	//	fmt.Printf("%d) %s\n", i, s.Raw)
	//}
	//fmt.Println("")
	//fmt.Println("H")
	//for i, s := range sh {
	//	fmt.Printf("%d) %s\n", i, s.Raw)
	//}
	//fmt.Println("")
	//fmt.Println("OUT")
	//for i, data := range out {
	//	fmt.Printf("%d) %s\n", i, data.Segment.Raw)
	//}

	allRaw := map[string]bool{}
	for _, segment := range ordered {
		if allRaw[segment.Raw] {

			fmt.Println("")
			fmt.Println("PR")
			for i, s := range sp {
				fmt.Printf("%d) %s\n", i, s.Raw)
			}
			fmt.Println("")
			fmt.Println("H")
			for i, s := range sh {
				fmt.Printf("%d) %s\n", i, s.Raw)
			}
			fmt.Println("")
			fmt.Println("")
			panic(segment.Raw)
		}
		allRaw[segment.Raw] = true
	}

	//orderMap := map[string]map[bool]int{}
	//for i, segment := range sp {
	//	if orderMap[segment.Raw] == nil {
	//		orderMap[segment.Raw] = map[bool]int{true: -1, false: -1}
	//	}
	//	orderMap[segment.Raw][true] = i
	//}
	//for i, segment := range sh {
	//	if orderMap[segment.Raw] == nil {
	//		orderMap[segment.Raw] = map[bool]int{true: -1, false: -1}
	//	}
	//	orderMap[segment.Raw][false] = i
	//}
	//
	//var err error
	//sort.Slice(out, func(i, j int) bool {
	//	ip := orderMap[out[i].Segment.Raw][true]
	//	jp := orderMap[out[j].Segment.Raw][true]
	//
	//	ih := orderMap[out[i].Segment.Raw][false]
	//	jh := orderMap[out[j].Segment.Raw][false]
	//
	//	if ip > -1 && jp > -1 && ih > -1 && jh > -1 {
	//		lessP := ip < jp
	//		lessH := ih < jh
	//		if lessP == lessH {
	//			return lessP
	//		} else {
	//			err = fmt.Errorf("incompatible segment order in %q and %q", n1.Debug(), n2.Debug())
	//			return false
	//		}
	//	} else if ih > -1 && jh > -1 {
	//		return ih < jh
	//	} else if ip > -1 && jp > -1 {
	//		return ip < jp
	//	} else if ip > -1 && jh > -1 {
	//		// find the nearest item nearest jh that exists in jp both lists
	//		j1 := j
	//		jp := -1
	//		for jp == -1 && j1 > 0 {
	//			j1--
	//			jp = orderMap[out[j1].Segment.Raw][true]
	//		}
	//		if jp == -1 {
	//			jp = 0
	//		}
	//		return ip < jp
	//	} else if ih > -1 && jp > -1 {
	//		// find the nearest item nearest jh that exists in jp both lists
	//		j1 := j
	//		jh := -1
	//		for jh == -1 && j1 > 0 {
	//			j1--
	//			jh = orderMap[out[j1].Segment.Raw][false]
	//		}
	//		if jh == -1 {
	//			jh = 0
	//		}
	//		return ih < jh
	//	} else {
	//		debugf("ip: %d, ih: %d, jp: %d, jh: %d\n", ip, ih, jp, jh)
	//		return false
	//	}
	//})
	//if err != nil {
	//	return nil, err
	//}
	return out, nil
}

func findNetworksWithMatchingSegments(n *Network, search []*Network) map[int]int {
	count := map[int]int{}
	for _, segment := range n.Segments {
		for j, network := range search {
			for _, s := range network.Segments {
				if segment.Raw == s.Raw {
					count[j]++
				}
			}
		}
	}
	return count
}

//var counter, counterRoutes int

func (d *Data) SaveMaster(dpath string) error {
	logln("Saving kml master")
	regular := map[SectionKey]map[RegularKey]*kml.Folder{}
	optional := map[SectionKey]map[int]map[OptionalKey][]*kml.Folder{}
	var renames []struct{ from, to string }

	writeNetwork := func(networkId, networksLength int, n, n1 *Network) error {

		var segments []*SegmentData
		if n1 == nil {
			for _, segment := range n.Segments {
				sd := &SegmentData{Segment: segment}
				if n.Route.Packrafting {
					sd.HasPackrafting = true
					sd.PackraftingFrom = segment.From
				} else {
					sd.HasHiking = true
					sd.HikingFrom = segment.From
				}
				segments = append(segments, sd)
			}

		} else {
			s, err := mergeSegments(n, n1)
			if err != nil {
				return err
			}
			segments = s
		}
		var f *kml.Folder
		if n.Route.Regular {
			if regular[n.Route.Section.Key] == nil {
				regular[n.Route.Section.Key] = map[RegularKey]*kml.Folder{}
			}
			if regular[n.Route.Section.Key][n.Route.RegularKey] == nil {
				switch n.Route.RegularKey.Direction {
				case "S":
					regular[n.Route.Section.Key][n.Route.RegularKey] = &kml.Folder{Name: "Southbound"}
				case "N":
					regular[n.Route.Section.Key][n.Route.RegularKey] = &kml.Folder{Name: "Northbound"}
				default:
					regular[n.Route.Section.Key][n.Route.RegularKey] = &kml.Folder{Name: n.Route.Section.FolderName()}
				}
			}
			f = regular[n.Route.Section.Key][n.Route.RegularKey]
		} else {
			if optional[n.Route.Section.Key] == nil {
				optional[n.Route.Section.Key] = map[int]map[OptionalKey][]*kml.Folder{}
			}
			if optional[n.Route.Section.Key][n.Route.OptionalKey.Option] == nil {
				optional[n.Route.Section.Key][n.Route.OptionalKey.Option] = map[OptionalKey][]*kml.Folder{}
			}
			f = &kml.Folder{Name: n.FolderName(networkId, networksLength, true)}
			optional[n.Route.Section.Key][n.Route.OptionalKey.Option][n.Route.OptionalKey] = append(optional[n.Route.Section.Key][n.Route.OptionalKey.Option][n.Route.OptionalKey], f)
		}

		for _, segment := range segments {
			f.Placemarks = append(f.Placemarks, &kml.Placemark{
				Name:     segment.PlacemarkName(networkId, networksLength, n),
				Legacy:   segment.Segment.Raw,
				Open:     1,
				StyleUrl: fmt.Sprintf("#%s", segment.Segment.Style()),
				LineString: &kml.LineString{
					Tessellate:  true,
					Coordinates: kml.LineCoordinates(segment.Segment.Line),
				},
			})
			if segment.Segment.Raw != segment.PlacemarkName(networkId, networksLength, n) {
				renames = append(renames, struct{ from, to string }{from: segment.Segment.Raw, to: segment.PlacemarkName(networkId, networksLength, n)})
			}
		}

		return nil
	}
	process := func(packrafting, hiking *Route) error {
		//if packrafting.Debug() != "GPT28H packrafting - option 1 (Rio Turbio)" {
		//	return nil
		//}
		if packrafting != nil && !packrafting.Regular && packrafting.OptionalKey.Alternatives {
			// for hiking alternatives these segments are already included in the regular routes
			return nil
		}
		var r, r1 *Route
		if packrafting != nil {
			r = packrafting
			r1 = hiking
		} else if hiking != nil {
			r = hiking
			r1 = packrafting
		} else {
			return fmt.Errorf("packrafting and hiking routes both nil")
		}
		if r1 != nil {
			doneInR1 := map[*Network]bool{}
			var unique []struct{ n, n1 *Network }
			for _, n := range r.Networks {
				matches := findNetworksWithMatchingSegments(n, r1.Networks)
				if len(matches) > 1 {
					var errstring string
					errstring += fmt.Sprintf("%s has segments from more than one other network (probably hiking split by packrafting without a proper joining segment):\n", n.Debug())
					for i, count := range matches {
						errstring += fmt.Sprintf("%s: %d segments\n", r1.Networks[i].Debug(), count)
					}
					return errors.New(errstring)
				}
				var n1 *Network
				if len(matches) == 1 {
					for i := range matches {
						n1 = r1.Networks[i]
					}
				}
				if n1 != nil {
					doneInR1[n1] = true
				}
				unique = append(unique, struct{ n, n1 *Network }{n: n, n1: n1})
			}
			for _, n1 := range r1.Networks {
				if doneInR1[n1] {
					continue
				}
				unique = append(unique, struct{ n, n1 *Network }{n: n1, n1: nil})
			}
			for i, u := range unique {
				if err := writeNetwork(i, len(unique), u.n, u.n1); err != nil {
					return err
				}
			}
			//if len(unique) > 1 && !r.Regular {
			//	counter += len(unique)
			//	counterRoutes++
			//	debugfln("%s: %d networks", r.Debug(), len(unique))
			//}
		} else {
			for i, n := range r.Networks {
				if err := writeNetwork(i, len(r.Networks), n, nil); err != nil {
					return err
				}
			}

			//if len(r.Networks) > 1 && !r.Regular {
			//	counter += len(r.Networks)
			//	counterRoutes++
			//	debugfln("%s: %d networks", r.Debug(), len(r.Networks))
			//}
		}

		return nil
	}
	if err := d.ForRoutePairs(process); err != nil {
		return fmt.Errorf("generating master: %w", err)
	}

	rf := &kml.Folder{Name: "Regular Tracks"}
	for key, routes := range regular {
		if len(routes) == 1 {
			if routes[RegularKey{""}] == nil {
				panic("route not found")
			}
			rf.Folders = append(rf.Folders, routes[RegularKey{""}])
		} else if len(routes) == 2 {
			if routes[RegularKey{"N"}] == nil || routes[RegularKey{"S"}] == nil {
				panic("routes not found")
			}
			rf.Folders = append(rf.Folders, &kml.Folder{
				Name:    d.Sections[key].FolderName(),
				Folders: []*kml.Folder{routes[RegularKey{"N"}], routes[RegularKey{"S"}]},
			})
		} else {
			panic("wrong number of routes")
		}
	}
	sort.Slice(rf.Folders, func(i, j int) bool {
		return rf.Folders[i].Name < rf.Folders[j].Name
	})

	of := &kml.Folder{Name: "Optional Tracks"}
	for key, options := range optional {
		sec := &kml.Folder{Name: d.Sections[key].FolderName()}
		of.Folders = append(of.Folders, sec)
		for optionNumber, routes := range options {
			var name string
			if optionNumber == 0 {
				name = "Variants"
			} else {
				name = fmt.Sprintf("Option %d", optionNumber)
			}
			op := &kml.Folder{Name: name}
			sec.Folders = append(sec.Folders, op)
			for _, networks := range routes {
				for _, folder := range networks {
					op.Folders = append(op.Folders, folder)
				}
			}
			sort.Slice(op.Folders, func(i, j int) bool {
				return op.Folders[i].Name < op.Folders[j].Name
			})
		}
		sort.Slice(sec.Folders, func(i, j int) bool {
			extractInt := func(s string) int {
				var i int
				if s != "Variants" {
					i, _ = strconv.Atoi(strings.TrimPrefix(s, "Option "))
				}
				return i
			}
			return extractInt(sec.Folders[i].Name) < extractInt(sec.Folders[j].Name)
		})
	}
	sort.Slice(of.Folders, func(i, j int) bool {
		return of.Folders[i].Name < of.Folders[j].Name
	})

	doc := kml.Document{
		Name:    "GPT Master.kmz",
		Folders: []*kml.Folder{{Name: "Tracks", Folders: []*kml.Folder{rf, of}}},
	}
	addSegmentStyles(&doc)

	root := kml.Root{
		Xmlns:    "http://www.opengis.net/kml/2.2",
		Document: doc,
	}
	if err := root.Save(filepath.Join(dpath, "GPT Master.kmz")); err != nil {
		return fmt.Errorf("saving master: %w", err)
	}

	var sb strings.Builder
	for _, rename := range renames {
		sb.WriteString(fmt.Sprintf("%q, %q\n", rename.from, rename.to))
	}
	if err := ioutil.WriteFile(filepath.Join(dpath, "renames.txt"), []byte(sb.String()), 0666); err != nil {
		return fmt.Errorf("writing renames file: %w", err)
	}

	//debugfln("%d unique networks, %d routes", counter, counterRoutes)
	return nil
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

	collect := func(waypoints []Waypoint) []*kml.Placemark {
		var placemarks []*kml.Placemark
		for _, w := range waypoints {
			placemarks = append(placemarks, &kml.Placemark{
				Name:  w.Name,
				Point: kml.PosPoint(w.Pos),
			})
		}
		return placemarks
	}

	importantFolder := &kml.Folder{
		Name:       "Important Information",
		Open:       1,
		Placemarks: collect(d.Important),
	}
	resupplyFolder := &kml.Folder{
		Name:       "Resupply Locations",
		Open:       1,
		Placemarks: collect(d.Resupplies),
	}
	optionalStartFolder := &kml.Folder{
		Name: "Optional Routes",
		Open: 1,
	}
	regularStartFolder := &kml.Folder{
		Name: "Regular Routes",
		Open: 1,
	}

	for _, terminator := range d.Terminators {
		var folder *kml.Folder
		if terminator.Option == "" {
			folder = regularStartFolder
		} else {
			folder = optionalStartFolder
		}
		folder.Placemarks = append(folder.Placemarks, &kml.Placemark{
			Name:  terminator.String(),
			Point: kml.PosPoint(terminator.Pos),
		})
	}

	waypointsFolder := &kml.Folder{
		Name: "Waypoints by Section",
		Open: 1,
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
			Name: section.String(),
			Open: 1,
		}
		for _, w := range section.Waypoints {
			sectionFolder.Placemarks = append(sectionFolder.Placemarks, &kml.Placemark{
				Name:  w.Name,
				Open:  1,
				Point: kml.PosPoint(w.Pos),
			})
		}
		waypointsFolder.Folders = append(waypointsFolder.Folders, sectionFolder)
	}

	all := kml.Root{
		Xmlns: "http://www.opengis.net/kml/2.2",
		Document: kml.Document{
			Name: "All Points.kmz",
			Folders: []*kml.Folder{
				{
					Name: "Points",
					Open: 1,
					Folders: []*kml.Folder{
						{
							Name:    "Section Start and End Points",
							Open:    1,
							Folders: []*kml.Folder{regularStartFolder, optionalStartFolder},
						},
						resupplyFolder,
						importantFolder,
						waypointsFolder,
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

	optStart := kml.Root{
		Xmlns: "http://www.opengis.net/kml/2.2",
		Document: kml.Document{
			Name:    "Optional Start and End Points.kmz",
			Folders: []*kml.Folder{optionalStartFolder},
		},
	}
	if err := optStart.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "Optional Start and End Points.kmz")); err != nil {
		return fmt.Errorf("saving Optional Start and End Points.kmz: %w", err)
	}

	regStart := kml.Root{
		Xmlns: "http://www.opengis.net/kml/2.2",
		Document: kml.Document{
			Name:    "Regular Start and End Points.kmz",
			Folders: []*kml.Folder{regularStartFolder},
		},
	}
	if err := regStart.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Waypoints", "Regular Start and End Points.kmz")); err != nil {
		return fmt.Errorf("saving Regular Start and End Points.kmz: %w", err)
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
}

func (d *Data) SaveKmlTracks(dpath string, stamp string) error {
	logln("Saving kml tracks")
	//debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Tracks/Regular Tracks.kmz", "input-regular.txt")
	//debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Tracks/Optional Tracks.kmz", "input-optional.txt")
	//debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Tracks/All Tracks.kmz", "input-all.txt")
	do := func(name string, filter func(Track) bool) (*kml.Folder, error) {
		f := &kml.Folder{Name: name, Open: 1}
		for _, key := range d.Keys {
			if HAS_SINGLE && key != SINGLE {
				continue
			}
			section := d.Sections[key]
			sectionFolder := &kml.Folder{
				Name: section.String(),
				Open: 1,
			}
			for _, track := range section.Tracks {
				if name == "Regular Tracks" && track.Optional {
					continue
				} else if name == "Optional Tracks" && !track.Optional {
					continue
				}
				trackFolder := &kml.Folder{
					Name: track.String(),
					Open: 1,
				}
				for _, segment := range track.Segments {
					trackFolder.Placemarks = append(trackFolder.Placemarks, &kml.Placemark{
						Name:     segment.String(),
						Open:     1,
						StyleUrl: fmt.Sprintf("#%s", segment.Style()),
						LineString: &kml.LineString{
							Tessellate:  true,
							Coordinates: kml.LineCoordinates(segment.Line),
						},
					})
				}
				if len(trackFolder.Placemarks) > 0 {
					sectionFolder.Folders = append(sectionFolder.Folders, trackFolder)
				}
			}
			if len(sectionFolder.Folders) > 0 {
				f.Folders = append(f.Folders, sectionFolder)
			}
		}
		return f, nil
	}
	regularFolder, err := do("Regular Tracks", func(track Track) bool { return !track.Optional })
	if err != nil {
		return fmt.Errorf("building regular tracks kml: %w", err)
	}
	optionalFolder, err := do("Optional Tracks", func(track Track) bool { return track.Optional })
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
					Open:    1,
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
	logln("Saving gpx files")
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
			match: func(s *Segment) bool { return s.Track.Optional },
		},
		{
			path:  []string{"Combined Tracks", fmt.Sprintf("Regular Tracks (%s).gpx", stamp)},
			match: func(s *Segment) bool { return !s.Track.Optional },
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

	for _, section := range d.Sections {
		for _, track := range section.Tracks {
			for _, segment := range track.Segments {
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
				Name:     segment.String(),
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

	wpRegTerminators := gpx.Root{}
	wpOptTerminators := gpx.Root{}
	for _, t := range d.Terminators {
		if t.Option == "" {
			wpRegTerminators.Waypoints = append(wpRegTerminators.Waypoints, gpx.Waypoint{
				Point: gpx.PosPoint(t.Pos),
				Name:  t.String(),
			})
		} else {
			wpOptTerminators.Waypoints = append(wpOptTerminators.Waypoints, gpx.Waypoint{
				Point: gpx.PosPoint(t.Pos),
				Name:  t.String(),
			})
		}
	}
	if err := wpRegTerminators.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("Regular Start and End Points (%s).gpx", stamp))); err != nil {
		return fmt.Errorf("saving waypoints: %w", err)
	}
	if err := wpOptTerminators.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("Optional Start and End Points (%s).gpx", stamp))); err != nil {
		return fmt.Errorf("saving waypoints: %w", err)
	}

	if err := ioutil.WriteFile(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Nomenclature.txt"), []byte(Nomenclature), 0666); err != nil {
		return fmt.Errorf("saving Nomenclature.txt: %w", err)
	}

	return nil
}

func (d *Data) SaveGaia(dpath string) error {
	logln("Saving gaia files")
	type clusterStruct struct {
		name     string
		from, to int
		modes    map[string]map[string]*gpx.Root
	}
	newContents := func() map[string]*gpx.Root {
		return map[string]*gpx.Root{"routes": {}, "options": {}, "routes-markers": {}, "options-markers": {}, "waypoints": {}}
	}
	newModes := func() map[string]map[string]*gpx.Root {
		return map[string]map[string]*gpx.Root{"hiking": newContents(), "packrafting": newContents()}
	}
	clusters := []clusterStruct{
		{name: "north", from: 1, to: 16, modes: newModes()},
		{name: "south", from: 17, to: 39, modes: newModes()},
		{name: "extensions", from: 40, to: 99, modes: newModes()},
		{name: "all", from: 1, to: 99, modes: newModes()},
	}

	for _, cluster := range clusters {

		count := map[string][]*Section{}
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
				var bundle *Bundle
				if mode == "hiking" {
					bundle = section.Hiking
				} else if mode == "packrafting" {
					bundle = section.Packrafting
				}
				if bundle == nil {
					continue
				}
				count[mode] = append(count[mode], section)

				for key, route := range bundle.Regular {
					if len(route.Networks) != 1 {
						return fmt.Errorf("regular route %s has %d networks. regular routes should only have 1 network", route.Debug(), len(route.Networks))
					}
					network := route.Networks[0]

					var rte gpx.Route
					var direction string
					if key.Direction == "N" {
						direction = " northbound"
					} else if key.Direction == "S" {
						direction = " southbound"
					}
					rte.Name = fmt.Sprintf("GPT%v %s%s", section.Key.Code(), section.Name, direction)
					rte.Desc = HEADING_SYMBOL + " " + rte.Name + "\n\n"

					var lines []geo.Line
					for _, segment := range network.Segments {
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
							wp := gpx.Waypoint{
								Point: gpx.PosPoint(flush.Segments[0].Line.Start()),
								Name:  flush.Description(id, true),
								Desc:  rte.Name,
							}
							contents["routes-markers"].Waypoints = append(contents["routes-markers"].Waypoints, wp)
						}
					}

					rte.Points = gpx.LinePoints(geo.MergeLines(lines))
					if mode == "hiking" {
						rte.Desc += "\n" + section.ScrapedHiking
					} else {
						rte.Desc += "\n" + section.ScrapedPackrafting
					}
					contents["routes"].Routes = append(contents["routes"].Routes, rte)
				}

				for _, route := range bundle.Options {
					for i, network := range route.Networks {
						var networkString string
						if len(route.Networks) > 1 {
							networkString = fmt.Sprintf(" (%d/%d)", i+1, len(route.Networks))
						}
						var trk gpx.Track
						if route.OptionalKey.Alternatives {
							var direction string
							if route.OptionalKey.Direction == "N" {
								direction = " northbound"
							} else if route.OptionalKey.Direction == "S" {
								direction = " southbound"
							}
							trk.Name = fmt.Sprintf("GPT%v%s hiking alternatives%s", route.Section.Key.Code(), direction, networkString)
						} else if route.OptionalKey.Option == 0 {
							trk.Name = fmt.Sprintf("GPT%v variant %v%s", route.Section.Key.Code(), route.OptionalKey.Code(), networkString)
						} else {
							trk.Name = fmt.Sprintf("GPT%v option %v%s", route.Section.Key.Code(), route.OptionalKey.Code(), networkString)
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
								wp := gpx.Waypoint{
									Point: gpx.PosPoint(flush.Segments[0].Line.Start()),
									Name:  flush.Description(id, true),
									Desc:  trk.Name,
								}
								contents["options-markers"].Waypoints = append(contents["options-markers"].Waypoints, wp)
							}
						}

						for _, segment := range network.Segments {
							trk.Segments = append(trk.Segments, gpx.TrackSegment{Points: gpx.LineTrackPoints(segment.Line)})
						}
						contents["options"].Tracks = append(contents["options"].Tracks, trk)
					}
				}

				// reverse the order of all tracks and routes (better in Gaia like this)
				//for i, j := 0, len(contents["options"].Tracks)-1; i < j; i, j = i+1, j-1 {
				//	contents["options"].Tracks[i], contents["options"].Tracks[j] = contents["options"].Tracks[j], contents["options"].Tracks[i]
				//}
				//for i, j := 0, len(contents["routes"].Routes)-1; i < j; i, j = i+1, j-1 {
				//	contents["routes"].Routes[i], contents["routes"].Routes[j] = contents["routes"].Routes[j], contents["routes"].Routes[i]
				//}

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
		Outer:
			for _, terminator := range d.Terminators {
				for _, section := range sections {
					for _, key := range terminator.Sections {
						if key == section.Key {
							var desc string
							for i, key := range terminator.Sections {
								if i > 0 {
									desc += ", "
								}
								desc += fmt.Sprintf("GPT%s", key.Code())
							}
							if terminator.Option != "" {
								desc += fmt.Sprintf(" option %s", terminator.Option)
							}
							wp := gpx.Waypoint{
								Point: gpx.PosPoint(terminator.Pos),
								Name:  terminator.Name,
								Desc:  desc,
							}
							if terminator.Option == "" {
								cluster.modes[mode]["routes"].Waypoints = append(cluster.modes[mode]["routes"].Waypoints, wp)
							} else {
								cluster.modes[mode]["options"].Waypoints = append(cluster.modes[mode]["options"].Waypoints, wp)
							}
							continue Outer
						}
					}
				}
			}
		}
	}

	for _, cluster := range clusters {
		for mode, modeMap := range cluster.modes {
			for contents, root := range modeMap {

				name := fmt.Sprintf("%s-%s-%s.gpx", cluster.name, mode, contents)
				if err := root.Save(filepath.Join(dpath, "GPX Files (For Gaia GPS app)", name)); err != nil {
					return fmt.Errorf("writing gpx")
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
	if err := wp(d.Resupplies, "waypoints-resupplies.gpx", "Resupply: "); err != nil {
		return fmt.Errorf("writing resupplies gpx: %w", err)
	}
	if err := wp(d.Important, "waypoints-important.gpx", "Important: "); err != nil {
		return fmt.Errorf("writing important gpx: %w", err)
	}

	if false {

		spfr := &kml.Folder{
			Name:       "Regular tracks",
			Visibility: 1,
			Open:       0,
			Placemarks: nil,
		}
		spfo := &kml.Folder{
			Name:       "Optional tracks",
			Visibility: 1,
			Open:       0,
			Placemarks: nil,
		}
		sp := kml.Root{
			Xmlns: "http://www.opengis.net/kml/2.2",
			Document: kml.Document{
				Name: "Route start points.kmz",
				Folders: []*kml.Folder{
					spfr,
					spfo,
				},
			},
		}
		processRoute := func(pr, hr *Route) error {

			var r *Route
			if pr != nil {
				r = pr
			} else {
				r = hr
			}
			rf := &kml.Folder{Name: r.String()}

			separateFolders := pr != nil && hr != nil && !pr.HasIdenticalNetworks(hr)
			combineRoutes := pr != nil && hr != nil && pr.HasIdenticalNetworks(hr)

			addContents := func(f *kml.Folder, r *Route, suffix string) {
				for _, n := range r.Networks {

					name := n.String()
					if suffix != "" {
						name += " " + suffix
					}
					f.Placemarks = append(f.Placemarks, &kml.Placemark{
						Name:       name,
						Visibility: 0,
						Open:       0,
						Point:      kml.PosPoint(n.Entry.Line.Start()),
					})

					net := &kml.Placemark{
						Name:          n.String(),
						Visibility:    0,
						Open:          0,
						MultiGeometry: &kml.MultiGeometry{},
						Style: &kml.Style{LineStyle: &kml.LineStyle{
							Color: "ffffffff",
							Width: 10,
						}},
					}
					for _, segment := range n.Segments {
						net.MultiGeometry.LineStrings = append(net.MultiGeometry.LineStrings, &kml.LineString{
							Tessellate:  true,
							Coordinates: kml.LineCoordinates(segment.Line),
						})
					}
					f.Placemarks = append(f.Placemarks, net)
				}
			}

			if separateFolders {
				pf := &kml.Folder{Name: "packrafting"}
				hf := &kml.Folder{Name: "hiking"}
				addContents(pf, pr, "packrafting")
				addContents(hf, hr, "hiking")
				rf.Folders = append(rf.Folders, pf)
				rf.Folders = append(rf.Folders, hf)
			} else {
				if combineRoutes {
					// both routes have identical networks, so only need to do one of them. either will do.
					addContents(rf, pr, "")
				} else if pr != nil {
					addContents(rf, pr, "")
				} else if hr != nil {
					addContents(rf, hr, "")
				} else {
					panic("shouldn't be here")
				}
			}

			if r.Regular {
				spfr.Folders = append(spfr.Folders, rf)
			} else {
				spfo.Folders = append(spfo.Folders, rf)
			}

			return nil
		}
		if err := d.ForRoutePairs(processRoute); err != nil {
			return err
		}
		return sp.Save(filepath.Join(dpath, "start-points.kmz"))
	}
	return nil
}

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
