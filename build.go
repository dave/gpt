package main

import (
	"fmt"
	"sort"
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
				if track.Code == "RR" || track.Code == code {
					for _, segment := range track.Segments {
						bundle.Regular.Segments = append(bundle.Regular.Segments, segment)
					}
				}
				if packrafting && track.Code == "RH" && len(track.Segments) > 0 {
					key := OptionalKey{Alternatives: true}
					if bundle.Options[key] == nil {
						bundle.Options[key] = &Route{
							Section:     section,
							Hiking:      false,
							Packrafting: true,
							Key:         key,
						}
					}
					for _, segment := range track.Segments {
						bundle.Options[key].Segments = append(bundle.Options[key].Segments, segment)
					}
				}
			}
			// order the segments by From km
			sort.Slice(bundle.Regular.Segments, func(i, j int) bool { return bundle.Regular.Segments[i].From < bundle.Regular.Segments[j].From })

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
							Section: section,
							Key:     key,
							Name:    track.Name,
						}
					}
					optionalRoutes[key].Segments = append(optionalRoutes[key].Segments, segment)
				}
			}
			for key, route := range optionalRoutes {
				include := true
				if !packrafting {
					// for hiking bundle, only include optional routes if they have no segments with packrafting terrain
					for _, segment := range route.Segments {
						if IsPackrafting(segment.Terrain) {
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

		// packrafting-only sections don't have a hiking bundle
		if section.Key.Suffix != "P" {
			section.Hiking = &Bundle{
				Regular: &Route{Section: section, Hiking: true},
				Options: map[OptionalKey]*Route{},
			}
			buildBundle(section.Hiking, false)
		}

		// all sections have a packrafting bundle
		section.Packrafting = &Bundle{
			Regular: &Route{Section: section, Packrafting: true},
			Options: map[OptionalKey]*Route{},
		}
		buildBundle(section.Packrafting, true)
	}

	for _, key := range data.Keys {
		section := data.Sections[key]
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
	return nil
}
