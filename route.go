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
	if false {
		if r.Debug() != "GPT07 hiking - option 2B (El Troncoso)" {
			return nil
		}
	}
	all := map[*Point]bool{}
	var ends []*Point
	for _, segment := range r.Segments {

		start := &Point{Segment: segment, Start: true, Index: 0, Pos: segment.Line[0]}
		ends = append(ends, start)
		all[start] = true
		segment.StartPoint = start

		finish := &Point{Segment: segment, End: true, Index: len(segment.Line) - 1, Pos: segment.Line[len(segment.Line)-1]}
		ends = append(ends, finish)
		all[finish] = true
		segment.EndPoint = finish

	}
	nearby := map[*Point][]*Point{}

	for _, end := range ends {
		all[end] = true
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
			all[mid] = true
			segment.MidPoints = append(segment.MidPoints, mid)
		}
	}
	var nodes []*Node
	done := map[*Point]bool{}
	for len(all) > len(done) {
		node := &Node{}
		var find func(*Point)
		find = func(p *Point) {
			if done[p] {
				return
			}
			done[p] = true
			node.Points = append(node.Points, p)
			for _, point := range nearby[p] {
				find(point)
			}
		}
		for point := range all {
			if !done[point] {
				find(point)
				break
			}
		}

		// if a node joins two forced separate segments, split them
		// TODO: WTF? This is shit.
		separate := [][]map[string]bool{
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
		}

		// if node has more than 1 point from the same segment, split and assign the other points based on distance
		needToSeparate := func(node *Node) (bool, [][]*Point) {

			var group0, group1 []*Point
			for _, sep := range separate {
				for _, point := range node.Points {
					if sep[0][point.Segment.Raw] || sep[0][point.Debug()] {
						group0 = append(group0, point)
					}
					if sep[1][point.Segment.Raw] || sep[1][point.Debug()] {
						group1 = append(group1, point)
					}
				}
				if len(group0) > 0 && len(group1) > 0 {
					return true, [][]*Point{group0, group1}
				}
			}

			segments := map[*Segment][]*Point{}
			for _, point := range node.Points {
				segments[point.Segment] = append(segments[point.Segment], point)
			}
			for _, points := range segments {
				if len(points) > 1 {
					var groups [][]*Point
					for _, p := range points {
						groups = append(groups, []*Point{p})
					}
					return true, groups
				}
			}
			return false, nil
		}
		var splitIfHasMultiple func(node *Node)
		splitIfHasMultiple = func(node *Node) {
			if found, groups := needToSeparate(node); found {
				var newNodes []*Node
				for _, group := range groups {
					newNodes = append(newNodes, &Node{Points: group})
				}
			NodePoints:
				for _, point := range node.Points {
					for _, group := range groups {
						for _, p := range group {
							if p == point {
								continue NodePoints
							}
						}
					}
					var closestNode *Node
					var closestDist float64
					for _, newNode := range newNodes {
						dist := newNode.Points[0].Pos.Distance(point.Pos)
						if closestNode == nil || dist < closestDist {
							closestNode = newNode
							closestDist = dist
						}
					}
					if closestNode == nil {
						panic("coulnd't find closest node")
					}
					closestNode.Points = append(closestNode.Points, point)
				}
				for _, newNode := range newNodes {
					splitIfHasMultiple(newNode)
				}
			} else {
				nodes = append(nodes, node)
			}
		}
		splitIfHasMultiple(node)
	}

	for _, node := range nodes {
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
