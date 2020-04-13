package main

import (
	"fmt"
	"strings"

	"github.com/dave/gpt/geo"
)

type Data struct {
	Keys        []SectionKey
	Sections    map[SectionKey]*Section
	Terminators []Terminator // Waypoints marking the start/end of sections
	Resupplies  []Waypoint
	Important   []Waypoint
}

func (d *Data) BuildRoutes() error {
	// Build routes
	for _, key := range d.Keys {
		section := d.Sections[key]
		buildBundle := func(bundle *Bundle, packrafting bool) {
			hiking := !packrafting
			// RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
			code := "RH"
			if packrafting {
				code = "RP"
			}
			for _, track := range section.Tracks {
				if track.Optional {
					continue
				}
				if track.Code == "RR" || track.Code == code {
					key := RegularKey{Direction: track.Direction}
					for _, segment := range track.Segments {
						if bundle.Regular[key] == nil {
							bundle.Regular[key] = &Route{
								Section:     section,
								Regular:     true,
								RegularKey:  key,
								Hiking:      hiking,
								Packrafting: packrafting,
							}
						}
						bundle.Regular[key].Segments = append(bundle.Regular[key].Segments, segment.DuplicateForTrack())
					}
				}
				if packrafting && track.Code == "RH" {
					key := OptionalKey{Alternatives: true, Direction: track.Direction}
					for _, segment := range track.Segments {
						if bundle.Options[key] == nil {
							bundle.Options[key] = &Route{
								Section:     section,
								Regular:     false,
								OptionalKey: key,
								Hiking:      hiking,
								Packrafting: packrafting,
							}
						}
						bundle.Options[key].Segments = append(bundle.Options[key].Segments, segment.DuplicateForTrack())
					}
				}
			}

			// optional routes
			for _, track := range section.Tracks {
				if !track.Optional {
					continue
				}
				for _, segment := range track.Segments {
					if hiking && segment.Code == "OP" {
						// Exclude all "OP" segments from the hiking bundle
						continue
					}
					key := OptionalKey{Option: track.Option, Variant: segment.Variant}
					if bundle.Options[key] == nil {
						bundle.Options[key] = &Route{
							Section:     section,
							Regular:     false,
							OptionalKey: key,
							Hiking:      hiking,
							Packrafting: packrafting,
							Name:        track.Name,
						}
					}
					bundle.Options[key].Segments = append(bundle.Options[key].Segments, segment.DuplicateForTrack())
				}
			}
		}

		// packrafting-only sections don't have a hiking bundle
		if section.Key.Suffix != "P" {
			section.Hiking = &Bundle{Regular: map[RegularKey]*Route{}, Options: map[OptionalKey]*Route{}}
			buildBundle(section.Hiking, false)
		}

		// all sections have a packrafting bundle
		section.Packrafting = &Bundle{Regular: map[RegularKey]*Route{}, Options: map[OptionalKey]*Route{}}
		buildBundle(section.Packrafting, true)
	}

	return nil
}

func (d *Data) Normalise() error {

	processRoute := func(r *Route) error {
		if LOG {
			fmt.Println("Normalising", r.Debug())
		}
		if err := r.BuildNetworks(); err != nil {
			return fmt.Errorf("building networks: %w", err)
		}
		for _, network := range r.Networks {
			if err := network.Normalise(); err != nil {
				return fmt.Errorf("normalising network: %w", err)
			}
		}
		return nil
	}

	processBundle := func(b *Bundle) error {
		for key, route := range b.Regular {
			if err := processRoute(route); err != nil {
				return fmt.Errorf("normalising regular %+v route: %w", key, err)
			}
		}
		for key, route := range b.Options {
			if err := processRoute(route); err != nil {
				return fmt.Errorf("normalising optional %+v route: %w", key, err)
			}
		}
		return nil
	}

	for _, key := range d.Keys {
		section := d.Sections[key]
		if section.Hiking != nil {
			if err := processBundle(section.Hiking); err != nil {
				return fmt.Errorf("normalising GPT%v hiking bundle: %w", section.Key.Code(), err)
			}
		}
		if section.Packrafting != nil {
			if err := processBundle(section.Packrafting); err != nil {
				return fmt.Errorf("normalising GPT%v packrafting bundle: %w", section.Key.Code(), err)
			}
		}
	}

	//ioutil.WriteFile("./debug.txt", []byte(debugString), 0666)

	return nil
}

type Waypoint struct {
	geo.Pos
	Name string
}

// Terminator is the position of the start/end of a section
type Terminator struct {
	geo.Pos
	Option    string
	Raw, Name string
	Sections  []SectionKey // One waypoint can be at the start / end of multiple
}

func (n Terminator) String() string {
	var b strings.Builder
	b.WriteString("GPT")
	var codes []string
	for _, section := range n.Sections {
		codes = append(codes, section.Code())
	}
	b.WriteString(strings.Join(codes, "/"))
	if n.Option != "" {
		b.WriteString("-")
		b.WriteString(n.Option)
	}
	b.WriteString(" (")
	b.WriteString(n.Name)
	b.WriteString(")")
	return b.String()
}
