package main

import (
	"fmt"
	"strings"

	"github.com/dave/gpt/geo"
)

type Data struct {
	Keys       []SectionKey
	Sections   map[SectionKey]*Section
	Resupplies []Waypoint
	Geographic []Waypoint
	Important  []Waypoint
}

//func (d *Data) ForRoutePairs(f func(packrafting, hiking *Route) error) error {
//	for _, key := range d.Keys {
//		section := d.Sections[key]
//		regularKeys := map[RegularKey]bool{}
//		optionalKeys := map[OptionalKey]bool{}
//		if section.Packrafting != nil {
//			for regularKey := range section.Packrafting.Regular {
//				regularKeys[regularKey] = true
//			}
//			for optionalKey := range section.Packrafting.Options {
//				optionalKeys[optionalKey] = true
//			}
//		}
//		if section.Hiking != nil {
//			for regularKey := range section.Hiking.Regular {
//				regularKeys[regularKey] = true
//			}
//			for optionalKey := range section.Hiking.Options {
//				optionalKeys[optionalKey] = true
//			}
//		}
//		for regularKey := range regularKeys {
//			var p, h *Route
//			if section.Packrafting != nil {
//				p = section.Packrafting.Regular[regularKey]
//			}
//			if section.Hiking != nil {
//				h = section.Hiking.Regular[regularKey]
//			}
//			if err := f(p, h); err != nil {
//				return err
//			}
//		}
//		for optionalKey := range optionalKeys {
//			var p, h *Route
//			if section.Packrafting != nil {
//				p = section.Packrafting.Options[optionalKey]
//			}
//			if section.Hiking != nil {
//				h = section.Hiking.Options[optionalKey]
//			}
//			if err := f(p, h); err != nil {
//				return err
//			}
//		}
//	}
//	return nil
//}
//
//func (d *Data) ForRoutes(f func(r *Route) error) error {
//
//	bundle := func(b *Bundle) error {
//		for key, route := range b.Regular {
//			if err := f(route); err != nil {
//				return fmt.Errorf("%s route: %w", key.Debug(), err)
//			}
//		}
//		for key, route := range b.Options {
//			if err := f(route); err != nil {
//				return fmt.Errorf("%s route: %w", key.Debug(), err)
//			}
//		}
//		return nil
//	}
//
//	for _, key := range d.Keys {
//		if HAS_SINGLE && key != SINGLE {
//			continue
//		}
//		section := d.Sections[key]
//		if section.Hiking != nil {
//			if err := bundle(section.Hiking); err != nil {
//				return fmt.Errorf("GPT%v hiking: %w", section.Key.Code(), err)
//			}
//		}
//		if section.Packrafting != nil {
//			if err := bundle(section.Packrafting); err != nil {
//				return fmt.Errorf("GPT%v packrafting: %w", section.Key.Code(), err)
//			}
//		}
//	}
//	return nil
//}

func (d *Data) Normalise() error {

	logln("building networks")
	for _, sectionKey := range d.Keys {
		section := d.Sections[sectionKey]
		for _, routeKey := range section.RouteKeys {
			route := section.Routes[routeKey]

			if err := route.BuildNetworks(); err != nil {
				return fmt.Errorf("building network: %w", err)
			}
		}
	}

	logln("normalising networks")
	for _, sectionKey := range d.Keys {
		section := d.Sections[sectionKey]
		for _, routeKey := range section.RouteKeys {
			route := section.Routes[routeKey]

			for _, mode := range MODES {
				if route.Modes[mode] == nil {
					continue
				}
				if err := route.Modes[mode].Network.Normalise(); err != nil {
					return fmt.Errorf("normalising network: %w", err)
				}
			}

		}
	}

	//ioutil.WriteFile("./debug.txt", []byte(debugString), 0666)

	return nil
}

type Waypoint struct {
	geo.Pos
	Name   string
	Folder string
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
