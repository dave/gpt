package main

import "fmt"

type Bundle struct {
	Regular map[RegularKey]*Route  // The regular route for this section.
	Options map[OptionalKey]*Route // Options and variants for this section. For the hiking bundle any options with packrafting terrain type are excluded.
}

// Post build tasks - normalise, tweak elevations for water, calculate stats etc.
func (b *Bundle) Post() error {

	/*
		if err := b.Regular.Normalise(); err != nil {
			return fmt.Errorf("normalising regular route: %w", err)
		}
		if err := b.Water(); err != nil {
			return fmt.Errorf("tweaking water elevations: %w", err)
		}
		if err := b.Calculate(); err != nil {
			return fmt.Errorf("calculating stats: %w", err)
		}
	*/
	return nil
}

// Water elevations should be flat or always run downhill
func (b *Bundle) Water() error {
	//	do := func(segments []*Segment, adjoining bool) error {

	//	}
	/*
		The elevations are often incorrect, especially in deep valleys or near cliffs. This can be corrected for water
		sections, because we know some things:

		Step 1: treat adjoining segments with the same terrain type as one section

		Lakes and ferries: apply the lowest point of the segment to the entire segment (but never less 0 m)

		Fjords: elevation = 0

		Rivers: Verify the river flow direction be looking at the start and end elevations (take average of first and
		last 1 km) and flow direction is downhill. Then step through all GPX points in the route ensuring none have a
		higher elevation than the previous points.
	*/
	return nil
}

/*
func (b *Bundle) Calculate() error {
	for _, route := range b.Regular {
		if err := route.CalculateSegmentStats(); err != nil {
			return fmt.Errorf("calculating regular route segment lengths: %w", err)
		}
	}
	for _, route := range b.Options {
		if err := route.CalculateSegmentStats(); err != nil {
			return fmt.Errorf("calculating optional route segment lengths: %w", err)
		}
	}
	return nil
}*/

type RegularKey struct {
	Direction string // North = "N", South = "S", All = ""
}

type OptionalKey struct {
	Option       int    // Option number. If true => Alternatives == false.
	Variant      string // Variant code. If true => Alternatives == false.
	Alternatives bool   // Hiking alternatives for packrafting routes. If true => Option == 0 && Variant == "".
	Direction    string // Only for the hiking alternatives, may have a direction from the track - e.g. N = North, S = South, "" = All
}

func (k OptionalKey) Code() string {
	if k.Option > 0 {
		return fmt.Sprintf("%02d%s", k.Option, k.Variant)
	}
	return k.Variant
}
