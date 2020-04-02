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
				if packrafting && track.Code == "RH" {
					for _, segment := range track.Segments {
						bundle.Alternatives = append(bundle.Alternatives, segment)
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
					key := OptionalKey{track.Option, segment.Variant}
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
			if err := section.Hiking.Regular.Normalise(); err != nil {
				return fmt.Errorf("normalising GPT%v regular hiking route: %w", section.Key.Code(), err)
			}
		}
		if section.Packrafting != nil {
			if err := section.Packrafting.Regular.Normalise(); err != nil {
				return fmt.Errorf("normalising GPT%v regular packrafting route: %w", section.Key.Code(), err)
			}
		}
		// Optional routes can't be normalised (not continuous enough)

	}
	return nil
}
