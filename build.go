package main

import (
	"fmt"
)

func buildRoutes(data *Data) error {
	// Build routes
	for _, key := range data.Keys {
		section := data.Sections[key]
		buildBundle := func(bundle *Bundle, packrafting bool) {
			// RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
			code := "RH"
			if packrafting {
				code = "RP"
			}
			for _, track := range section.Tracks {
				regular := bundle.Regular[RegularKey{Direction: track.Direction}]
				if track.Code == "RR" || track.Code == code {
					for _, segment := range track.Segments {
						regular.Segments = append(regular.Segments, segment.DuplicateForTrack())
					}
				}
				if packrafting && track.Code == "RH" && len(track.Segments) > 0 {
					key := OptionalKey{Alternatives: true, Direction: track.Direction}
					if bundle.Options[key] == nil {
						bundle.Options[key] = &Route{
							Regular:     false,
							Section:     section,
							Packrafting: true,
							OptionalKey: key,
						}
					}
					for _, segment := range track.Segments {
						bundle.Options[key].Segments = append(bundle.Options[key].Segments, segment.DuplicateForTrack())
					}
				}
			}
			// order the segments by From km
			// TODO: why?
			//for _, route := range bundle.Regular {
			//	sort.Slice(route.Segments, func(i, j int) bool { return route.Segments[i].From < route.Segments[j].From })
			//}

			// optional routes
			optionalRoutes := map[OptionalKey]*Route{}
			for _, track := range section.Tracks {
				if !track.Optional {
					continue
				}
				for _, segment := range track.Segments {
					key := OptionalKey{Option: track.Option, Variant: segment.Variant}
					if optionalRoutes[key] == nil {
						optionalRoutes[key] = &Route{
							Regular:     false,
							Section:     section,
							Packrafting: packrafting,
							OptionalKey: key,
							Name:        track.Name,
						}
					}
					optionalRoutes[key].Segments = append(optionalRoutes[key].Segments, segment.DuplicateForTrack())
				}
			}
			for key, route := range optionalRoutes {
				include := true
				if !packrafting {
					// for hiking bundle, only include optional routes if they have no segments with packrafting terrain
					for _, segment := range route.Segments {
						if HasPackrafting(segment.Terrains) {
							include = false
							break
						}
					}
				}
				if include {
					bundle.Options[key] = route
				}
			}
		}

		newBundle := func(packrafting bool) *Bundle {
			routes := map[RegularKey]*Route{}
			for _, track := range section.Tracks {
				if track.Optional {
					continue
				}
				key := RegularKey{Direction: track.Direction}
				if routes[key] == nil {
					routes[key] = &Route{
						Section:     section,
						Regular:     true,
						RegularKey:  key,
						Hiking:      !packrafting,
						Packrafting: packrafting,
					}
				}
			}
			return &Bundle{
				Regular: routes,
				Options: map[OptionalKey]*Route{},
			}
		}

		// packrafting-only sections don't have a hiking bundle
		if section.Key.Suffix != "P" {
			section.Hiking = newBundle(false)
			buildBundle(section.Hiking, false)
		}

		// all sections have a packrafting bundle
		section.Packrafting = newBundle(true)
		buildBundle(section.Packrafting, true)
	}

	for _, key := range data.Keys {
		section := data.Sections[key]
		fmt.Println("Processing", section.String())
		if section.Hiking != nil {
			if err := section.Hiking.Post(); err != nil {
				return fmt.Errorf("post build for GPT%v hiking bundle: %w", section.Key.Code(), err)
			}
		}
		if section.Packrafting != nil {
			if err := section.Packrafting.Post(); err != nil {
				return fmt.Errorf("post build for GPT%v packrafting bundle: %w", section.Key.Code(), err)
			}
		}
	}

	//ioutil.WriteFile("./debug.txt", []byte(debugString), 0666)

	var count int
	for _, key := range data.Keys {
		section := data.Sections[key]
		if section.Hiking != nil {
			for _, route := range section.Hiking.Regular {
				count += len(route.Networks)
			}
			for _, route := range section.Hiking.Options {
				count += len(route.Networks)
			}
		}
		if section.Packrafting != nil {
			for _, route := range section.Packrafting.Regular {
				count += len(route.Networks)
			}
			for _, route := range section.Packrafting.Options {
				count += len(route.Networks)
			}
		}
	}

	return nil
}
