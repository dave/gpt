package main

import (
	"fmt"
	"path/filepath"

	"github.com/dave/gpt/geo"
	"github.com/dave/gpt/gpx"
)

func saveRoutes(data *Data, dpath string) error {
	type clusterStruct struct {
		from, to int
		modes    map[string]map[string]*gpx.Root
	}
	newContents := func() map[string]*gpx.Root {
		return map[string]*gpx.Root{"regular": {}, "options": {}, "markers": {}, "waypoints": {}}
	}
	newModes := func() map[string]map[string]*gpx.Root {
		return map[string]map[string]*gpx.Root{"hiking": newContents(), "packrafting": newContents()}
	}
	clusters := []clusterStruct{
		{from: 1, to: 16, modes: newModes()},
		{from: 17, to: 40, modes: newModes()},
		{from: 41, to: 99, modes: newModes()},
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
				contents["regular"].Routes = append(contents["regular"].Routes, rte)

				//optionsSummaryTrack := gpx.Track{
				//	Name: fmt.Sprintf("GPT%s options", section.Key.Code()),
				//}
				for _, route := range bundle.Options {
					var trk gpx.Track
					if route.Key.Option == 0 {
						trk.Name = fmt.Sprintf("GPT%v variant %v", route.Section.Key.Code(), route.Key.Code())
					} else {
						trk.Name = fmt.Sprintf("GPT%v option %v", route.Section.Key.Code(), route.Key.Code())
					}
					for _, segment := range route.Segments {
						trk.Segments = append(trk.Segments, gpx.TrackSegment{Points: gpx.LineTrackPoints(segment.Line)})
						//optionsSummaryTrack.Segments = append(optionsSummaryTrack.Segments, gpx.TrackSegment{Points: gpx.LineTrackPoints(segment.Line)})
						trk.Desc += segment.Description() + "\n"
					}

					contents["options"].Tracks = append(contents["options"].Tracks, trk)
				}
				//regular.Tracks = append(regular.Tracks, optionsSummaryTrack)

				for key, waypoints := range data.Waypoints {
					if key != section.Key {
						continue
					}
					for _, w := range waypoints {
						contents["waypoints"].Waypoints = append(contents["waypoints"].Waypoints, gpx.Waypoint{
							Point: gpx.PosPoint(w.Pos),
							Name:  w.Name,
							Desc:  "GPT" + key.Code(),
						})
					}
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
								cluster.modes[mode]["regular"].Waypoints = append(cluster.modes[mode]["regular"].Waypoints, wp)
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
				name := fmt.Sprintf("GPT%02d-%02d-%s-%s.gpx", cluster.from, cluster.to, mode, contents)
				if err := root.Save(filepath.Join(dpath, name)); err != nil {
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
		return root.Save(filepath.Join(dpath, name))
	}
	if err := wp(data.Resupplies, "waypoints-resupplies.gpx", "Resupply: "); err != nil {
		return fmt.Errorf("writing resupplies gpx: %w", err)
	}
	if err := wp(data.Important, "waypoints-important.gpx", "Important: "); err != nil {
		return fmt.Errorf("writing important gpx: %w", err)
	}
	return nil
}
