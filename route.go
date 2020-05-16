package main

import (
	"fmt"
	"strings"
)

// Route is a continuous path composed of several adjoining segments (maybe from different tracks)
type Route struct {
	*Section
	Hiking, Packrafting bool        // is this part of the packrafting or hiking route? (for both regular and optional routes)
	Regular             bool        // is this the regular route? If Regular == true, Key == OptionalKey{}
	RegularKey          RegularKey  // key for regular routes.
	OptionalKey         OptionalKey // key for optional routes.
	Name                string      // track name for optional tracks
	Segments            []*Segment
	Networks            []*Network
}

func (r *Route) HasIdenticalNetworks(r1 *Route) bool {
	if len(r.Networks) != len(r1.Networks) {
		return false
	}
	for i := range r.Networks {
		if r.Networks[i].Signature() != r1.Networks[i].Signature() {
			return false
		}
	}
	return true
}

// String: a-GPTbbc-defggh-i/j
// a: P, H: packrafting / hiking specific route
// bb: 2 digit section number
// c?: P, H: packrafting / hiking specific section
// d: R, O: regular / optional
// e?: N, S: northbound / southbound route
// f?: A: hiking alternatives route
// gg?: option number (2 digits)
// h?: variant letter
// i?: network number
// j?: total number of networks
func (r *Route) String() string {
	var key string
	if !r.Regular {
		var alternatives string
		if r.OptionalKey.Alternatives {
			alternatives = "HA"
		}
		var option string
		if r.OptionalKey.Option > 0 {
			option = fmt.Sprintf("%02d", r.OptionalKey.Option)
		}
		key = fmt.Sprintf("-%s%s%s%s", alternatives, r.OptionalKey.Direction, option, r.OptionalKey.Variant)
	} else {
		if r.RegularKey.Direction != "" {
			key = r.RegularKey.Direction
		}
	}
	return fmt.Sprintf(
		"GPT%s%s",
		r.Section.Key.Code(),
		key,
	)
}

func (r *Route) Debug() string {

	var dir string
	if r.RegularKey.Direction == "N" {
		dir = " northbound"
	} else if r.RegularKey.Direction == "S" {
		dir = " southbound"
	}

	typ := "hiking"
	if r.Packrafting {
		typ = "packrafting"
	}

	var name string
	if r.Name != "" {
		name = fmt.Sprintf(" (%s)", r.Name)
	}

	if r.Regular {
		return fmt.Sprintf("GPT%s%s %s%s", r.Section.Key.Code(), dir, typ, name)
	}
	if r.OptionalKey.Alternatives {
		return strings.TrimSpace(fmt.Sprintf("GPT%s%s %s - hiking alternatives", r.Section.Key.Code(), dir, typ))
	}
	if r.OptionalKey.Option == 0 {
		return strings.TrimSpace(fmt.Sprintf("GPT%s%s %s - variant %s%s", r.Section.Key.Code(), dir, typ, r.OptionalKey.Variant, name))
	}
	return strings.TrimSpace(fmt.Sprintf("GPT%s%s %s - option %d%s%s", r.Section.Key.Code(), dir, typ, r.OptionalKey.Option, r.OptionalKey.Variant, name))

}

func (r *Route) BuildNetworks() error {

	//if !(r.String() == "GPT02-H" && r.Hiking) {
	//	return nil
	//}

	all := map[*Point]bool{}
	var allOrdered []*Point
	var ends []*Point
	for _, segment := range r.Segments {

		start := &Point{Segment: segment, Start: true, Index: 0, Pos: segment.Line[0]}
		ends = append(ends, start)
		if !all[start] {
			all[start] = true
			allOrdered = append(allOrdered, start)
		}
		segment.StartPoint = start

		finish := &Point{Segment: segment, End: true, Index: len(segment.Line) - 1, Pos: segment.Line[len(segment.Line)-1]}
		ends = append(ends, finish)
		if !all[finish] {
			all[finish] = true
			allOrdered = append(allOrdered, finish)
		}
		segment.EndPoint = finish

	}
	nearby := map[*Point][]*Point{}

	for _, end := range ends {
		if !all[end] {
			all[end] = true
			allOrdered = append(allOrdered, end)
		}
		for _, neighbour := range ends {
			if end == neighbour {
				continue
			}
			if end.Segment == neighbour.Segment {
				continue
			}
			if !end.Pos.IsClose(neighbour.Pos, DELTA) {
				continue
			}
			nearby[end] = append(nearby[end], neighbour)
			nearby[neighbour] = append(nearby[neighbour], end)
		}
	Outer:
		for _, segment := range r.Segments {
			if end.Segment == segment {
				continue
			}
			for _, point := range nearby[end] {
				if point.Segment == segment {
					continue Outer
				}
			}
			found, index := segment.Line.IsClose(end.Pos, DELTA)
			if !found {
				continue
			}
			mid := &Point{Segment: segment, Index: index, Pos: segment.Line[index]}
			nearby[end] = append(nearby[end], mid)
			nearby[mid] = append(nearby[mid], end)
			if !all[mid] {
				all[mid] = true
				allOrdered = append(allOrdered, mid)
			}
			segment.MidPoints = append(segment.MidPoints, mid)
		}
	}

	var nodes []*Node
	done := map[*Point]bool{}
	for len(all) > len(done) {
		node := &Node{}
		var addPointAndAllNearby func(*Point)
		addPointAndAllNearby = func(p *Point) {
			if done[p] {
				return
			}
			done[p] = true
			node.Points = append(node.Points, p)
			for _, point := range nearby[p] {
				if p == point || done[point] {
					continue
				}
				addPointAndAllNearby(point)
			}
		}
		for _, point := range allOrdered {
			// find any unused point and add it to the node
			if !done[point] {
				addPointAndAllNearby(point)
				break
			}
		}

		// if a node joins two forced separate segments, split them
		// TODO: WTF? This is shit.
		separationRules := [][]map[string]bool{
			{
				{"RR-PR-V@40-0.0+0.7": true, "RR-MR-V@40-0.7+0.1": true, "RR-TL-V@40-0.8+15.8": true},
				{"RR-PR-V@40-64.3+0.7": true, "RR-PR-V@40-62.3+2.0": true},
			},
			{
				{"RR-TL-V@38-21.8+1.6 end": true, "RR-TL-V@38-23.4+0.2 end": true},
				{"RH-TL-V@38-23.7+9.3 start": true, "RH-TL-V@38-23.6+0.2 end": true},
			},
			{
				{"RR-TL-V@39-40.1+6.4": true, "RR-TL-V@39-46.5+1.5": true, "RR-TL-V@39-48.0+1.8": true},
				{"RR-TL-V@39-50.3+1.5": true, "RR-TL-V@39-51.7+1.5": true, "RR-TL-V@39-53.2+5.7": true},
			},
			{
				{"RP-FJ-2@77-51.8+6.6 (Fiordo Cahuelmo) end": true, "RP-TL-V@77-58.4+0.2 start": true},
				{"RP-TL-V@77-58.5+0.2 start": true, "RP-FJ-2@77-58.7+7.0 (Fiordo Cahuelmo) start": true},
			},
			{
				{"EXP-OP-TL-V@92P-A-#001 end": true},
				{"EXP-OP-TL-V@92P-A-#002 end": true},
				{"EXP-OP-TL-V@92P-A-#003 end": true},
				{"EXP-OP-TL-V@92P-A-#003 #6": true},
			},
			{
				{"OP-MR-V@28H-01-#001": true, "OP-LK-2@28H-01-#002 (Lago Verde)": true, "OP-LK-2@28H-01-#003 (Lago Verde)": true, "OP-TL-V@28H-01-#004": true, "OH-TL-V@28H-01-#005": true, "OH-TL-I@28H-01-#006 start": true},
				{"OP-MR-V@28H-01-#009": true, "OP-LK-2@28H-01-#008 (Lago Verde)": true, "OP-RI-1@28H-01-#007 (Rio Turbio) #266": true, "OP-RI-1@28H-01-#007 (Rio Turbio) end": true},
			},
		}

		needToSeparate := func(node *Node) (bool, [][]*Point) {
			for _, rule := range separationRules {
				var separatedPointGroups [][]*Point
				for _, separationGroup := range rule {
					var separatedPoints []*Point
					for _, point := range node.Points {
						if separationGroup[point.Segment.Raw] || separationGroup[point.Debug()] {
							separatedPoints = append(separatedPoints, point)
						}
					}
					if len(separatedPoints) > 0 {
						separatedPointGroups = append(separatedPointGroups, separatedPoints)
					}
				}
				if len(separatedPointGroups) > 1 {
					return true, separatedPointGroups
				}
			}

			// if node has more than 1 point from the same segment, split and assign the other points based on distance
			segments := map[*Segment][]*Point{}
			var segmentsOrdered []*Segment
			for _, point := range node.Points {
				if segments[point.Segment] == nil {
					segmentsOrdered = append(segmentsOrdered, point.Segment)
				}
				segments[point.Segment] = append(segments[point.Segment], point)
			}
			for _, segment := range segmentsOrdered {
				points := segments[segment]
				if len(points) > 1 {
					var separatedPointGroups [][]*Point
					for _, p := range points {
						separatedPointGroups = append(separatedPointGroups, []*Point{p})
					}
					return true, separatedPointGroups
				}
			}
			return false, nil
		}
		var addOrSplit func(node *Node)
		addOrSplit = func(node *Node) {
			if found, groups := needToSeparate(node); !found {
				nodes = append(nodes, node)
			} else {
				var newNodes []*Node
				for _, group := range groups {
					newNodes = append(newNodes, &Node{Points: group})
				}
			NodePoints:
				for _, point := range node.Points {
					for _, group := range groups {
						for _, p := range group {
							if p == point {
								// only consider points that aren't included in the separated point groups
								continue NodePoints
							}
						}
					}
					var closestNode *Node
					var closestDist float64
					for _, newNode := range newNodes {
						for _, p := range newNode.Points {
							dist := p.Pos.Distance(point.Pos)
							if closestNode == nil || dist < closestDist {
								closestNode = newNode
								closestDist = dist
							}
						}
					}
					if closestNode == nil {
						panic("coulnd't find closest node")
					}
					closestNode.Points = append(closestNode.Points, point)
				}
				for _, newNode := range newNodes {
					addOrSplit(newNode)
				}
			}
		}
		addOrSplit(node)
	}

	const PRINT_NODES = false

	if PRINT_NODES {
		debugf("\n\n%s\n", r.String())
	}
	for i, node := range nodes {
		if PRINT_NODES {
			debugf("%d) %s\n", i, node.Debug())
		}
		for _, point := range node.Points {
			point.Node = node
		}
	}

	doneSegments := map[*Segment]bool{}
	for len(r.Segments) > len(doneSegments) {
		// find a segment that hasn't been used
		var segment *Segment
		for _, s := range r.Segments {
			if !doneSegments[s] {
				segment = s
				break
			}
		}
		network := &Network{Route: r}
		networkNodes := map[*Node]bool{} // log of the nodes we've added to this network, so we don't add them twice
		var find func(*Segment)
		find = func(segment *Segment) {
			if doneSegments[segment] {
				return
			}
			doneSegments[segment] = true
			network.Segments = append(network.Segments, segment)
			for _, node := range nodes {
				if !node.ContainsSegment(segment) {
					continue
				}
				if !networkNodes[node] {
					node.Network = network
					network.Nodes = append(network.Nodes, node)
					networkNodes[node] = true
				}
				for _, point := range node.Points {
					find(point.Segment)
				}
			}
		}
		find(segment)
		r.Networks = append(r.Networks, network)
	}

	return nil
}
