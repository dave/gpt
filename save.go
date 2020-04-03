package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/dave/gpt/geo"
	"github.com/dave/gpt/gpx"
	"github.com/dave/gpt/kml"
)

func saveKmlWaypoints(data *Data, dpath string, stamp string) error {
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
		Placemarks: collect(data.Important),
	}
	resupplyFolder := &kml.Folder{
		Name:       "Resupply Locations",
		Open:       1,
		Placemarks: collect(data.Resupplies),
	}
	optionalStartFolder := &kml.Folder{
		Name: "Optional Routes",
		Open: 1,
	}
	regularStartFolder := &kml.Folder{
		Name: "Regular Routes",
		Open: 1,
	}

	for _, node := range data.Nodes {
		var folder *kml.Folder
		if node.Option == "" {
			folder = regularStartFolder
		} else {
			folder = optionalStartFolder
		}
		folder.Placemarks = append(folder.Placemarks, &kml.Placemark{
			Name:  node.Raw,
			Point: kml.PosPoint(node.Pos),
		})
	}

	waypointsFolder := &kml.Folder{
		Name: "Waypoints by Section",
		Open: 1,
	}
	for _, key := range data.Keys {
		section := data.Sections[key]
		if len(section.Waypoints) == 0 {
			continue
		}
		sectionFolder := &kml.Folder{
			Name: section.Raw,
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

func saveKmlTracks(data *Data, dpath string, stamp string) error {
	//debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Tracks/Regular Tracks.kmz", "input-regular.txt")
	//debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Tracks/Optional Tracks.kmz", "input-optional.txt")
	//debug("/Users/dave/src/gpt/input/Track Files/KMZ File (For Google Earth and Smartphones)/Tracks/All Tracks.kmz", "input-all.txt")
	do := func(name string, filter func(Track) bool) (*kml.Folder, error) {
		f := &kml.Folder{Name: name, Open: 1}
		for _, key := range data.Keys {
			section := data.Sections[key]
			sectionFolder := &kml.Folder{
				Name: section.Raw,
				Open: 1,
			}
			for _, track := range section.Tracks {
				if name == "Regular Tracks" && track.Optional {
					continue
				} else if name == "Optional Tracks" && !track.Optional {
					continue
				}
				trackFolder := &kml.Folder{
					Name: track.Raw,
					Open: 1,
				}
				for _, segment := range track.Segments {
					trackFolder.Placemarks = append(trackFolder.Placemarks, &kml.Placemark{
						Name: segment.Raw,
						Open: 1,
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
	if err := optional.Save(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "Optional Tracks.kmz")); err != nil {
		return fmt.Errorf("saving Optional Tracks.kmz: %w", err)
	}

	//debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "All Tracks.kmz"), "output-all.txt")
	//debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "Regular Tracks.kmz"), "output-regular.txt")
	//debug(filepath.Join(dpath, "KMZ File (For Google Earth and Smartphones)", "Tracks", "Optional Tracks.kmz"), "output-optional.txt")

	return nil
}

func debug(fpath string, name string) {
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

func saveGpx(data *Data, dpath string, stamp string) error {
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
				return s.Experimental && s.Code == "OH" && !IsPackrafting(s.Terrain) && s.Verification == "A"
			},
		},
		{
			path: []string{"Exploration Hiking Tracks", "Optional Tracks", "EXP-OH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OH" && !IsPackrafting(s.Terrain) && s.Verification == "I"
			},
		},
		{
			path: []string{"Exploration Hiking Tracks", "Optional Tracks", "EXP-OH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OH" && !IsPackrafting(s.Terrain) && s.Verification == "V"
			},
		},

		{
			path: []string{"Exploration Hiking Tracks", "Regular Tracks", "EXP-RH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RH" && !IsPackrafting(s.Terrain) && s.Verification == "A"
			},
		},
		{
			path: []string{"Exploration Hiking Tracks", "Regular Tracks", "EXP-RH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RH" && !IsPackrafting(s.Terrain) && s.Verification == "I"
			},
		},
		{
			path: []string{"Exploration Hiking Tracks", "Regular Tracks", "EXP-RH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RH" && !IsPackrafting(s.Terrain) && s.Verification == "V"
			},
		},

		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OH" && !IsPackrafting(s.Terrain) && s.Verification == "A"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OH" && !IsPackrafting(s.Terrain) && s.Verification == "I"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OH" && !IsPackrafting(s.Terrain) && s.Verification == "V"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-LD-A.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OP" && !IsPackrafting(s.Terrain) && s.Verification == "A"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-LD-I.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OP" && !IsPackrafting(s.Terrain) && s.Verification == "I"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-LD-V.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OP" && !IsPackrafting(s.Terrain) && s.Verification == "V"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-WR-1.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OP" && IsPackrafting(s.Terrain) && s.Directional == "1"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-OP-WR-2.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "OP" && IsPackrafting(s.Terrain) && s.Directional == "2"
			},
		},

		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-RH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RH" && !IsPackrafting(s.Terrain) && s.Verification == "A"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-RH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RH" && !IsPackrafting(s.Terrain) && s.Verification == "I"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Optional Tracks", "EXP-RH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RH" && !IsPackrafting(s.Terrain) && s.Verification == "V"
			},
		},

		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-LD-A.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RP" && !IsPackrafting(s.Terrain) && s.Verification == "A"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-LD-I.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RP" && !IsPackrafting(s.Terrain) && s.Verification == "I"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-LD-V.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RP" && !IsPackrafting(s.Terrain) && s.Verification == "V"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-WR-1.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RP" && IsPackrafting(s.Terrain) && s.Directional == "1"
			},
		},
		{
			path: []string{"Exploration Packrafting Tracks", "Regular Tracks", "EXP-RP-WR-2.gpx"},
			match: func(s *Segment) bool {
				return s.Experimental && s.Code == "RP" && IsPackrafting(s.Terrain) && s.Directional == "2"
			},
		},

		{
			path: []string{"Hiking Tracks", "Optional Tracks", "OH-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && s.Terrain == "FY" && s.Directional == "1"
			},
		},
		{
			path: []string{"Hiking Tracks", "Optional Tracks", "OH-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && s.Terrain == "FY" && s.Directional == "2"
			},
		},
		{
			path: []string{"Hiking Tracks", "Optional Tracks", "OH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && !IsPackrafting(s.Terrain) && s.Verification == "A"
			},
		},
		{
			path: []string{"Hiking Tracks", "Optional Tracks", "OH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && !IsPackrafting(s.Terrain) && s.Verification == "I"
			},
		},
		{
			path: []string{"Hiking Tracks", "Optional Tracks", "OH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && !IsPackrafting(s.Terrain) && s.Verification == "V"
			},
		},

		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RH-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && s.Terrain == "FY" && s.Directional == "1"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RH-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && s.Terrain == "FY" && s.Directional == "2"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && !IsPackrafting(s.Terrain) && s.Verification == "A"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && !IsPackrafting(s.Terrain) && s.Verification == "I"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && !IsPackrafting(s.Terrain) && s.Verification == "V"
			},
		},

		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RR-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && s.Terrain == "FY" && s.Directional == "1"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RR-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && s.Terrain == "FY" && s.Directional == "2"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RR-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && !IsPackrafting(s.Terrain) && s.Verification == "A"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RR-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && !IsPackrafting(s.Terrain) && s.Verification == "I"
			},
		},
		{
			path: []string{"Hiking Tracks", "Regular Tracks", "RR-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && !IsPackrafting(s.Terrain) && s.Verification == "V"
			},
		},

		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && s.Terrain == "FY" && s.Directional == "1"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && s.Terrain == "FY" && s.Directional == "2"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && !IsPackrafting(s.Terrain) && s.Verification == "A"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && !IsPackrafting(s.Terrain) && s.Verification == "I"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OH" && !IsPackrafting(s.Terrain) && s.Verification == "V"
			},
		},

		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OP" && s.Terrain == "FY" && s.Directional == "1"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OP" && s.Terrain == "FY" && s.Directional == "2"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OP" && !IsPackrafting(s.Terrain) && s.Verification == "A"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OP" && !IsPackrafting(s.Terrain) && s.Verification == "I"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OP" && !IsPackrafting(s.Terrain) && s.Verification == "V"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-WR-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OP" && IsPackrafting(s.Terrain) && s.Directional == "1"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "OP-WR-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "OP" && IsPackrafting(s.Terrain) && s.Directional == "2"
			},
		},

		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && s.Terrain == "FY" && s.Directional == "1"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && s.Terrain == "FY" && s.Directional == "2"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && !IsPackrafting(s.Terrain) && s.Verification == "A"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && !IsPackrafting(s.Terrain) && s.Verification == "I"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Optional Tracks", "RH-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RH" && !IsPackrafting(s.Terrain) && s.Verification == "V"
			},
		},

		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RP" && s.Terrain == "FY" && s.Directional == "1"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RP" && s.Terrain == "FY" && s.Directional == "2"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RP" && !IsPackrafting(s.Terrain) && s.Verification == "A"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RP" && !IsPackrafting(s.Terrain) && s.Verification == "I"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RP" && !IsPackrafting(s.Terrain) && s.Verification == "V"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-WR-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RP" && IsPackrafting(s.Terrain) && s.Directional == "1"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RP-WR-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RP" && IsPackrafting(s.Terrain) && s.Directional == "2"
			},
		},

		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-FY-1.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && s.Terrain == "FY" && s.Directional == "1"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-FY-2.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && s.Terrain == "FY" && s.Directional == "2"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-LD-A.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && !IsPackrafting(s.Terrain) && s.Verification == "A"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-LD-I.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && !IsPackrafting(s.Terrain) && s.Verification == "I"
			},
		},
		{
			path: []string{"Packrafting Tracks", "Regular Tracks", "RR-LD-V.gpx"},
			match: func(s *Segment) bool {
				return !s.Experimental && s.Code == "RR" && !IsPackrafting(s.Terrain) && s.Verification == "V"
			},
		},
	}

	for _, section := range data.Sections {
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

		/*
			var existing int
			test, err := gpx.Load(filepath.Join(append([]string{"/Users/dave/src/gpt/input/Track Files/GPX Files (For Smartphones and Basecamp)"}, m.path...)...))
			if err == nil {
				existing = len(test.Tracks)
			}
			if existing != len(m.segments) {
				//	if m.path[len(m.path)-1] == "RH-FY-2.gpx" {
				//		for _, track := range test.Tracks {
				//			fmt.Println(track.Name)
				//		}
				//	}
				//	for _, segment := range m.segments {
				//		fmt.Println("-", segment.Raw)
				//	}
				switch m.path[len(m.path)-1] {
				case "RR-FY-2.gpx", "RH-FY-2.gpx", "RR-LD-V.gpx", "RH-LD-V.gpx":
					// special case because tracks in 24P are RH but should be RR, so renamed
				default:
					return fmt.Errorf("%s test %d, built %d", filepath.Join(m.path...), existing, len(m.segments))
				}
			}
		*/

		if len(m.segments) == 0 {
			continue
		}

		g := gpx.Root{}
		for _, segment := range m.segments {
			g.Tracks = append(g.Tracks, gpx.Track{
				Name:     segment.Raw,
				Segments: []gpx.TrackSegment{{Points: gpx.LineTrackPoints(segment.Line)}},
			})
		}
		fpath := filepath.Join(append([]string{dpath, "GPX Files (For Smartphones and Basecamp)"}, m.path...)...)
		if err := g.Save(fpath); err != nil {
			return fmt.Errorf("saving %q: %w", m.path[len(m.path)-1], err)
		}
	}

	wpAll := gpx.Root{}
	for _, key := range data.Keys {
		section := data.Sections[key]
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
	for _, w := range data.Important {
		wpImp.Waypoints = append(wpImp.Waypoints, gpx.Waypoint{
			Point: gpx.PosPoint(w.Pos),
			Name:  w.Name,
		})
	}
	if err := wpImp.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("Important Infromation (%s).gpx", stamp))); err != nil {
		return fmt.Errorf("saving waypoints: %w", err)
	}

	wpRes := gpx.Root{}
	for _, w := range data.Resupplies {
		wpRes.Waypoints = append(wpRes.Waypoints, gpx.Waypoint{
			Point: gpx.PosPoint(w.Pos),
			Name:  w.Name,
		})
	}
	if err := wpRes.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("Resupply Locations (%s).gpx", stamp))); err != nil {
		return fmt.Errorf("saving waypoints: %w", err)
	}

	wpRegNode := gpx.Root{}
	wpOptNode := gpx.Root{}
	for _, node := range data.Nodes {
		if node.Option == "" {
			wpRegNode.Waypoints = append(wpRegNode.Waypoints, gpx.Waypoint{
				Point: gpx.PosPoint(node.Pos),
				Name:  node.Raw,
			})
		} else {
			wpOptNode.Waypoints = append(wpOptNode.Waypoints, gpx.Waypoint{
				Point: gpx.PosPoint(node.Pos),
				Name:  node.Raw,
			})
		}
	}
	if err := wpRegNode.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("Regular Start and End Points (%s).gpx", stamp))); err != nil {
		return fmt.Errorf("saving waypoints: %w", err)
	}
	if err := wpOptNode.Save(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Waypoints", fmt.Sprintf("Optional Start and End Points (%s).gpx", stamp))); err != nil {
		return fmt.Errorf("saving waypoints: %w", err)
	}

	if err := ioutil.WriteFile(filepath.Join(dpath, "GPX Files (For Smartphones and Basecamp)", "Nomenclature.txt"), []byte(Nomenclature), 0666); err != nil {
		return fmt.Errorf("saving Nomenclature.txt: %w", err)
	}

	return nil
}

func saveGaia(data *Data, dpath string) error {
	type clusterStruct struct {
		name     string
		from, to int
		modes    map[string]map[string]*gpx.Root
	}
	newContents := func() map[string]*gpx.Root {
		return map[string]*gpx.Root{"routes": {}, "options": {}, "markers": {}, "waypoints": {}}
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

		for _, key := range data.Keys {
			section := data.Sections[key]
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

				var rte gpx.Route
				rte.Name = fmt.Sprintf("GPT%v %s", section.Key.Code(), bundle.Regular.Section.Name)
				var lines []geo.Line

				// Multiple segments are often in series with exactly the same details. If so we combine them into a single
				// waypoint
				var groups [][]*Segment
				for _, segment := range bundle.Regular.Segments {
					lines = append(lines, segment.Line)

					if len(groups) > 0 && segment.Similar(groups[len(groups)-1][len(groups[len(groups)-1])-1]) {
						// segment is similar to the previous one -> add to current group
						groups[len(groups)-1] = append(groups[len(groups)-1], segment)
					} else {
						// segment is not similar -> add a new group
						groups = append(groups, []*Segment{segment})
					}
				}
				for _, group := range groups {

					wp := gpx.Waypoint{
						Point: gpx.PosPoint(group[0].Line.Start()),
					}

					var totalLength float64

					for _, segment := range group {

						rte.Desc += segment.Description() + "\n"
						wp.Desc += segment.Raw + "\n"

						totalLength += segment.Length
					}

					wp.Name = fmt.Sprintf("GPT%s %s", group[0].Section.Key.Code(), group[0].DescriptionLength(totalLength))

					contents["markers"].Waypoints = append(contents["markers"].Waypoints, wp)

				}

				if mode == "packrafting" && len(bundle.Alternatives) > 0 {
					// packrafting sections have the hiking alternatives where the regular hiking route went a different way
					var alternatives gpx.Track
					alternatives.Name = fmt.Sprintf("GPT%s hiking alternatives", section.Key.Code())
					for _, segment := range bundle.Alternatives {
						alternatives.Segments = append(alternatives.Segments, gpx.TrackSegment{Points: gpx.LineTrackPoints(segment.Line)})

						alternatives.Desc += segment.Description() + "\n"
					}
					contents["options"].Tracks = append(contents["options"].Tracks, alternatives)
				}
				rte.Points = gpx.LinePoints(geo.MergeLines(lines))
				contents["routes"].Routes = append(contents["routes"].Routes, rte)

				for _, route := range bundle.Options {
					var trk gpx.Track
					if route.Key.Option == 0 {
						trk.Name = fmt.Sprintf("GPT%v variant %v", route.Section.Key.Code(), route.Key.Code())
					} else {
						trk.Name = fmt.Sprintf("GPT%v option %v", route.Section.Key.Code(), route.Key.Code())
					}
					for _, segment := range route.Segments {
						trk.Segments = append(trk.Segments, gpx.TrackSegment{Points: gpx.LineTrackPoints(segment.Line)})
						trk.Desc += segment.Description() + "\n"
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
		Outer:
			for _, node := range data.Nodes {
				for _, section := range sections {
					for _, key := range node.Sections {
						if key == section.Key {
							var desc string
							for i, key := range node.Sections {
								if i > 0 {
									desc += ", "
								}
								desc += fmt.Sprintf("GPT%s", key.Code())
							}
							if node.Option != "" {
								desc += fmt.Sprintf(" option %s", node.Option)
							}
							wp := gpx.Waypoint{
								Point: gpx.PosPoint(node.Pos),
								Name:  node.Name,
								Desc:  desc,
							}
							if node.Option == "" {
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
	if err := wp(data.Resupplies, "waypoints-resupplies.gpx", "Resupply: "); err != nil {
		return fmt.Errorf("writing resupplies gpx: %w", err)
	}
	if err := wp(data.Important, "waypoints-important.gpx", "Important: "); err != nil {
		return fmt.Errorf("writing important gpx: %w", err)
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
