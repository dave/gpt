package routedata

import (
	"fmt"
	"strings"

	"github.com/dave/gpt/globals"
)

//for _, sectionKey := range d.Keys {
//section := d.Sections[sectionKey]
//for _, routeKey := range section.RouteKeys {
//route := section.Routes[routeKey]
//}
//}

// Route is a continuous path composed of several adjoining segments
type Route struct {
	Section *Section
	Key     RouteKey
	Name    string // track name for optional tracks
	Option  string // name from option folder
	All     []*Segment
	Modes   map[globals.ModeType]*RouteModeData
}

type RouteModeData struct {
	Segments []*Segment
	Network  *Network
}

type RouteKey struct {
	Required          globals.RequiredType
	Direction         string // North = "N", South = "S", All = "" (regular and optional hiking alternatives)
	Option            int    // Option number. If true => Alternatives == false.
	Variant           string // Variant code (one or two upper case letters). If true => Alternatives == false.
	Alternatives      bool   // Hiking alternatives for packrafting routes. If true => Option == 0 && Variant == "".
	Network           string // Network code (one lower case letter) - MIGRATE THIS OUT
	AlternativesIndex int    // when hiking alternatives have several non-consecutive stretches, a separate route is created for each.
}

func (r *Route) FolderName() string {
	var name string
	if r.Key.Option > 0 {
		name = fmt.Sprintf("%02d%s%s", r.Key.Option, r.Key.Variant, r.Key.Network)
	} else {
		name = fmt.Sprintf("%s%s", r.Key.Variant, r.Key.Network)
	}

	// only show name if variant
	if r.Key.Option == 0 && r.Key.Variant != "" && r.Name != "" {
		name += fmt.Sprintf(" (%s)", r.Name)
	}

	return name
}

func (k RouteKey) Debug() string {
	if k.Required == globals.REGULAR {
		switch k.Direction {
		case "S":
			return "southbound"
		case "N":
			return "northbound"
		default:
			return "regular"
		}
	} else {
		if k.Alternatives {
			var direction string
			if k.Direction == "S" {
				direction = "southbound "
			} else if k.Direction == "N" {
				direction = "northbound "
			}
			return fmt.Sprintf("%shiking alternatives %d", direction, k.AlternativesIndex)
		}
		if k.Option > 0 {
			return fmt.Sprintf("option %d%s", k.Option, k.Variant)
		}
		return fmt.Sprintf("variant %s", k.Variant)
	}
}

//func (r *Route) HasIdenticalNetworks(r1 *Route) bool {
//	if len(r.Networks) != len(r1.Networks) {
//		return false
//	}
//	for i := range r.Networks {
//		if r.Networks[i].Signature() != r1.Networks[i].Signature() {
//			return false
//		}
//	}
//	return true
//}

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
	if r.Key.Required == globals.OPTIONAL {
		var alternatives string
		if r.Key.Alternatives {
			alternatives = "HA"
		}
		var option string
		if r.Key.Option > 0 {
			option = fmt.Sprintf("%02d", r.Key.Option)
		}
		key = fmt.Sprintf("-%s%s%s%s%s", alternatives, r.Key.Direction, option, r.Key.Variant, r.Key.Network)
	} else {
		if r.Key.Direction != "" {
			key = r.Key.Direction
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
	if r.Key.Direction == "N" {
		dir = " northbound"
	} else if r.Key.Direction == "S" {
		dir = " southbound"
	} else {
		dir = " regular"
	}

	var name string
	if r.Name != "" {
		name = fmt.Sprintf(" (%s)", r.Name)
	}

	if r.Key.Required == globals.REGULAR {
		return fmt.Sprintf("GPT%s%s%s", r.Section.Key.Code(), dir, name)
	}
	if r.Key.Alternatives {
		return strings.TrimSpace(fmt.Sprintf("GPT%s%s - hiking alternatives %d", r.Section.Key.Code(), dir, r.Key.AlternativesIndex))
	}
	if r.Key.Option == 0 {
		return strings.TrimSpace(fmt.Sprintf("GPT%s%s - variant %s%s", r.Section.Key.Code(), dir, r.Key.Variant, name))
	}
	return strings.TrimSpace(fmt.Sprintf("GPT%s%s - option %d%s%s", r.Section.Key.Code(), dir, r.Key.Option, r.Key.Variant, name))

}

func (r *Route) BuildNetworks() error {

	for _, mode := range globals.MODES {

		if r.Modes[mode] == nil {
			continue
		}
		rMode := r.Modes[mode]

		all := map[*Point]bool{}
		var allOrdered []*Point
		var ends []*Point
		for _, segment := range rMode.Segments {

			start := &Point{Segment: segment, Start: true, Index: 0, Pos: segment.Line[0]}
			ends = append(ends, start)
			if !all[start] {
				all[start] = true
				allOrdered = append(allOrdered, start)
			}
			segment.Modes[mode].StartPoint = start

			finish := &Point{Segment: segment, End: true, Index: len(segment.Line) - 1, Pos: segment.Line[len(segment.Line)-1]}
			ends = append(ends, finish)
			if !all[finish] {
				all[finish] = true
				allOrdered = append(allOrdered, finish)
			}
			segment.Modes[mode].EndPoint = finish

		}
		nearby := map[*Point][]*Point{}

		if r.Key.Required == globals.REGULAR {
			// forming network for regluar routes is trivial

			for i, segment := range rMode.Segments {
				segmentMode := segment.Modes[mode]
				if i == 0 {
					node := &Node{
						Network: rMode.Network,
						Points:  []*Point{segmentMode.StartPoint},
					}
					segmentMode.StartPoint.Node = node
					rMode.Network.Nodes = append(rMode.Network.Nodes, node)
				}
				if i > 0 {
					prev := rMode.Segments[i-1]
					prevMode := prev.Modes[mode]

					// ensure segments all join in regular routes
					if !prevMode.EndPoint.Pos.IsClose(segmentMode.StartPoint.Pos, globals.DELTA) {
						return fmt.Errorf("%q and %q are %dm apart", prev.Raw, segment.Raw, prevMode.EndPoint.Pos.Distance(segmentMode.StartPoint.Pos)*1000)
					}

					node := &Node{
						Network: rMode.Network,
						Points:  []*Point{prevMode.EndPoint, segmentMode.StartPoint},
					}
					prevMode.EndPoint.Node = node
					segmentMode.StartPoint.Node = node
					rMode.Network.Nodes = append(rMode.Network.Nodes, node)
				}
				if i == len(rMode.Segments)-1 {
					node := &Node{
						Network: rMode.Network,
						Points:  []*Point{segmentMode.EndPoint},
					}
					segmentMode.EndPoint.Node = node
					rMode.Network.Nodes = append(rMode.Network.Nodes, node)
				}
			}

		} else {
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
					if !end.Pos.IsClose(neighbour.Pos, globals.DELTA) {
						continue
					}
					nearby[end] = append(nearby[end], neighbour)
					nearby[neighbour] = append(nearby[neighbour], end)
				}
			Outer:
				for _, segment := range rMode.Segments {
					segmentMode := segment.Modes[mode]
					if end.Segment == segment {
						continue
					}
					for _, point := range nearby[end] {
						if point.Segment == segment {
							continue Outer
						}
					}
					found, index := segment.Line.IsClose(end.Pos, globals.DELTA)
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
					segmentMode.MidPoints = append(segmentMode.MidPoints, mid)
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

				needToSeparate := func(node *Node) (bool, [][]*Point) {
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
			// find a segment that hasn't been used
			var segment *Segment
			for _, s := range rMode.Segments {
				if !doneSegments[s] {
					segment = s
					break
				}
			}
			networkNodes := map[*Node]bool{} // log of the nodes we've added to this network, so we don't add them twice
			var find func(*Segment)
			find = func(segment *Segment) {
				if doneSegments[segment] {
					return
				}
				doneSegments[segment] = true
				for _, node := range nodes {
					if !node.ContainsSegment(segment) {
						continue
					}
					if !networkNodes[node] {
						node.Network = rMode.Network
						rMode.Network.Nodes = append(rMode.Network.Nodes, node)
						networkNodes[node] = true
					}
					for _, point := range node.Points {
						find(point.Segment)
					}
				}
			}
			find(segment)
			if len(rMode.Segments) != len(doneSegments) {
				return fmt.Errorf("route %q in %q contains more than one network", r.Debug(), r.Section.Raw)
			}
		}
	}
	return nil
}
