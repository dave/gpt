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

func (d *Data) ForRoutePairs(f func(packrafting, hiking *Route) error) error {
	for _, key := range d.Keys {
		section := d.Sections[key]
		regularKeys := map[RegularKey]bool{}
		optionalKeys := map[OptionalKey]bool{}
		if section.Packrafting != nil {
			for regularKey := range section.Packrafting.Regular {
				regularKeys[regularKey] = true
			}
			for optionalKey := range section.Packrafting.Options {
				optionalKeys[optionalKey] = true
			}
		}
		if section.Hiking != nil {
			for regularKey := range section.Hiking.Regular {
				regularKeys[regularKey] = true
			}
			for optionalKey := range section.Hiking.Options {
				optionalKeys[optionalKey] = true
			}
		}
		for regularKey := range regularKeys {
			var p, h *Route
			if section.Packrafting != nil {
				p = section.Packrafting.Regular[regularKey]
			}
			if section.Hiking != nil {
				h = section.Hiking.Regular[regularKey]
			}
			if err := f(p, h); err != nil {
				return err
			}
		}
		for optionalKey := range optionalKeys {
			var p, h *Route
			if section.Packrafting != nil {
				p = section.Packrafting.Options[optionalKey]
			}
			if section.Hiking != nil {
				h = section.Hiking.Options[optionalKey]
			}
			if err := f(p, h); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *Data) ForRoutes(f func(r *Route) error) error {

	bundle := func(b *Bundle) error {
		for key, route := range b.Regular {
			if err := f(route); err != nil {
				return fmt.Errorf("%s route: %w", key.Debug(), err)
			}
		}
		for key, route := range b.Options {
			if err := f(route); err != nil {
				return fmt.Errorf("%s route: %w", key.Debug(), err)
			}
		}
		return nil
	}

	for _, key := range d.Keys {
		if HAS_SINGLE && key != SINGLE {
			continue
		}
		section := d.Sections[key]
		if section.Hiking != nil {
			if err := bundle(section.Hiking); err != nil {
				return fmt.Errorf("GPT%v hiking: %w", section.Key.Code(), err)
			}
		}
		if section.Packrafting != nil {
			if err := bundle(section.Packrafting); err != nil {
				return fmt.Errorf("GPT%v packrafting: %w", section.Key.Code(), err)
			}
		}
	}
	return nil
}

func (d *Data) Normalise() error {

	processRoute := func(r *Route) error {
		logln("Normalising", r.Debug())
		if err := r.BuildNetwork(); err != nil {
			return fmt.Errorf("building network: %w", err)
		}
		//if err := r.Network.Normalise(); err != nil {
		//	return fmt.Errorf("normalising route: %w", err)
		//}
		//for _, network := range r.Networks {
		//	if err := network.Normalise(normalise); err != nil {
		//		return fmt.Errorf("normalising network: %w", err)
		//	}
		//}
		return nil
	}

	if err := d.ForRoutes(processRoute); err != nil {
		return fmt.Errorf("normalising: %w", err)
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
