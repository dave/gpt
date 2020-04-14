package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dave/gpt/geo"
)

// Network is a collection of Segments which adjoin at nodes. There is a node at the start and end of
// each segment, and also at the mid point of each segment where another segment's end point adjoins.
type Network struct {
	Route     *Route
	Nodes     []*Node
	Segments  []*Segment
	Entry     *Segment
	Straights []*Straight

	// Short edges go from point to point, and can be shorter than a segment where the segment has mid points. len(ShortEdges) >= len(Segments).
	ShortEdges map[*Node][]*Edge
	// Long edges ignore mid points and go from start point to end point of each segment. len(LongEdges) == len(Segments).
	LongEdges map[*Node][]*Edge
	// For each point, this is the shortest path along short edges from the network entry point.
	ShortEdgePaths map[*Point]*Path
	// For each point, this is the shortest path along long edges from the entry point. For some networks the main network
	// entry point does not lead to all points along long edges. After the network is fully explored from the entry point,
	// we choose the nearest unused point as the new entry point and repeat until all edges are used. Long edge paths DO
	// NOT traverse segments in reverse.
	LongEdgePaths map[*Point]*Path
}

func (n *Network) Normalise(normalise bool) error {

	if !normalise {
		// if normalise has been disabled with a flag (for testing) we still need to run BuildStraights
		n.BuildStraights()
		return nil
	}

	n.FindEntrySegment()

	if len(n.Entry.StartPoint.Node.Points) > 1 && len(n.Entry.EndPoint.Node.Points) == 1 {
		n.Entry.Reverse()
	}

	n.BuildEdges()

	n.BuildShortEdgePaths()

	n.ReverseSegments()

	for _, segment := range n.Segments {
		segment.From = n.ShortEdgePaths[segment.StartPoint].Length
	}

	n.BuildLongEdgePaths()

	if err := n.Reorder(); err != nil {
		return fmt.Errorf("reordering: %w", err)
	}

	n.BuildStraights()

	n.LevelWater()

	return nil
}

func (n *Network) BuildStraights() {
	newFlush := func(segment *Segment) *Flush {
		return &Flush{
			From:         segment.From,
			Terrains:     segment.Terrains,
			Verification: segment.Verification,
			Directional:  segment.Directional,
			Experimental: segment.Experimental,
		}
	}
	newStraight := func(segment *Segment) *Straight {
		return &Straight{
			Flushes: []*Flush{newFlush(segment)},
		}
	}
	for i, segment := range n.Segments {
		if i > 0 {
			prev := n.Segments[i-1]
			if !prev.EndPoint.Node.Contains(segment.StartPoint) {
				// new straight
				n.Straights = append(n.Straights, newStraight(segment))
			} else if !prev.Similar(segment) {
				// new flush
				s := n.Straights[len(n.Straights)-1]
				s.Flushes = append(s.Flushes, newFlush(segment))
			}
		} else {
			n.Straights = append(n.Straights, newStraight(segment))
		}
		s := n.Straights[len(n.Straights)-1]
		f := s.Flushes[len(s.Flushes)-1]
		f.Length += segment.Length
		f.Segments = append(f.Segments, segment)
		s.Segments = append(s.Segments, segment)
	}
}

func (n *Network) LevelWater() {
	//FJ: Fjord Packrafting, LK: Lake Packrafting, RI: River Packrafting, FY: Ferry
	water := map[string]bool{"FJ": true, "LK": true, "RI": true, "FY": true}
	same := func(s1, s2 *Segment) bool {
		if len(s2.Terrains) != 1 {
			return false
		}
		if !water[s2.Terrains[0]] {
			return false
		}
		return s1.Terrains[0] == s2.Terrains[0]
	}
	match := func(s *Segment) bool {
		if len(s.Terrains) != 1 {
			return false
		}
		return water[s.Terrains[0]]
	}
	var stretches [][]*Segment
	for _, straight := range n.Straights {
		var stretch []*Segment
		for i, segment := range straight.Segments {
			if i > 0 {
				if !same(straight.Segments[i-1], segment) {
					if len(stretch) > 0 {
						stretches = append(stretches, stretch)
					}
					stretch = nil
				}
			}
			if match(segment) {
				stretch = append(stretch, segment)
			}
		}
		if len(stretch) > 0 {
			stretches = append(stretches, stretch)
		}
	}
	for _, stretch := range stretches {
		switch stretch[0].Terrains[0] {
		case "FJ":
			// all elevations should be zero
			for _, segment := range stretch {
				for i := range segment.Line {
					segment.Line[i] = geo.Pos{
						Lat: segment.Line[i].Lat,
						Lon: segment.Line[i].Lon,
						Ele: 0,
					}
				}
			}
		case "LK", "FY":
			// find the lowest non negative elevation
			var lowest float64
			var found bool
			for _, segment := range stretch {
				for _, pos := range segment.Line {
					if !found || pos.Ele < lowest {
						lowest = pos.Ele
						found = true
					}
				}
			}
			if lowest < 0 {
				lowest = 0
			}
			for _, segment := range stretch {
				for i := range segment.Line {
					segment.Line[i] = geo.Pos{
						Lat: segment.Line[i].Lat,
						Lon: segment.Line[i].Lon,
						Ele: lowest,
					}
				}
			}
		case "RI":
			var lastPos geo.Pos
			var foundPos bool
			var uphillCount, downhillCount int
			for _, segment := range stretch {
				for _, pos := range segment.Line {
					if foundPos {
						if lastPos.Ele < pos.Ele {
							uphillCount++
						}
						if lastPos.Ele > pos.Ele {
							downhillCount++
						}
					}
					lastPos = pos
					foundPos = true
				}
			}
			uphill := uphillCount > downhillCount
			var lastEle float64
			var foundEle bool
			for _, segment := range stretch {
				for i := range segment.Line {
					if foundEle {
						if uphill {
							// elevations can only rise
							if segment.Line[i].Ele < lastEle {
								segment.Line[i] = geo.Pos{
									Lat: segment.Line[i].Lat,
									Lon: segment.Line[i].Lon,
									Ele: lastEle,
								}
							}
						} else {
							// elevations can only fall
							// elevations can only rise
							if segment.Line[i].Ele > lastEle {
								segment.Line[i] = geo.Pos{
									Lat: segment.Line[i].Lat,
									Lon: segment.Line[i].Lon,
									Ele: lastEle,
								}
							}
						}
					}
					lastEle = segment.Line[i].Ele
					foundEle = true
				}
			}
		}
	}
}

func (n *Network) BuildEdges() {
	// build edges
	n.ShortEdges = map[*Node][]*Edge{}
	n.LongEdges = map[*Node][]*Edge{}
	for _, segment := range n.Segments {
		points := segment.Points(true)
		for i, point := range points {
			if i == 0 {
				continue
			}
			prev := points[i-1]

			// calculate length
			var length float64
			for i := prev.Index; i <= point.Index; i++ {
				if i == prev.Index {
					continue // skip first
				}
				length += segment.Line[i-1].Distance(segment.Line[i])
			}

			edge := &Edge{
				Segment: segment,
				Nodes:   [2]*Node{prev.Node, point.Node},
				Length:  length,
			}
			n.ShortEdges[prev.Node] = append(n.ShortEdges[prev.Node], edge)
			n.ShortEdges[point.Node] = append(n.ShortEdges[point.Node], edge)
		}
		edge := &Edge{
			Segment: segment,
			Nodes:   [2]*Node{segment.StartPoint.Node, segment.EndPoint.Node},
			Length:  segment.Line.Length(),
		}
		n.LongEdges[segment.StartPoint.Node] = append(n.LongEdges[segment.StartPoint.Node], edge)
		n.LongEdges[segment.EndPoint.Node] = append(n.LongEdges[segment.EndPoint.Node], edge)
	}
}

func (n *Network) BuildShortEdgePaths() {
	// for each segment, find the shortest path from the entry point to the start point and to the end point. If it's a
	// shorter path to the end point, the segment should be reversed.
	n.ShortEdgePaths = map[*Point]*Path{}
	var explore func(*Path, *Node)
	explore = func(path *Path, node *Node) {

		for _, point := range node.Points {
			if n.ShortEdgePaths[point] == nil || n.ShortEdgePaths[point].Length > path.Length {
				n.ShortEdgePaths[point] = path.Copy()
			}
		}

		for _, edge := range n.ShortEdges[node] {
			if path.Has(edge) {
				continue
			}
			// all points attached to this node
			explore(path.CopyAndAdd(edge), edge.Opposite(node))
		}
	}
	explore(&Path{From: n.Entry.StartPoint.Node}, n.Entry.StartPoint.Node)
}

func (n *Network) ReverseSegments() {
	for _, segment := range n.Segments {
		if n.ShortEdgePaths[segment.EndPoint].Length < n.ShortEdgePaths[segment.StartPoint].Length {
			segment.Reverse()
		}
	}
}

func (n *Network) BuildLongEdgePaths() {
	n.LongEdgePaths = map[*Point]*Path{}

	var orderedPoints []*Point // ordered by proximity to start point
	for point := range n.ShortEdgePaths {
		orderedPoints = append(orderedPoints, point)
	}
	sort.Slice(orderedPoints, func(i, j int) bool {
		return n.ShortEdgePaths[orderedPoints[i]].Length < n.ShortEdgePaths[orderedPoints[j]].Length
	})

	findStartPoint := func() *Point {
		for _, p := range orderedPoints {
			if n.LongEdgePaths[p] == nil {
				return p
			}
		}
		return nil
	}

	for len(n.ShortEdgePaths) > len(n.LongEdgePaths) {
		entry := findStartPoint()

		var explore func(*Path, *Node)
		explore = func(path *Path, node *Node) {

			for _, point := range node.Points {
				//if n.Route.Regular {
				// TODO: ???
				//	if n.LongEdgePaths[point] == nil || n.LongEdgePaths[point].Length < path.Length {
				//		n.LongEdgePaths[point] = path.Copy()
				//	}
				//} else {
				if n.LongEdgePaths[point] == nil || n.LongEdgePaths[point].Length > path.Length {
					n.LongEdgePaths[point] = path.Copy()
				}
				//}
			}

			for _, edge := range n.LongEdges[node] {
				if path.Has(edge) {
					continue
				}
				// only traverse this edge if we are at the start of it's segment
				if !node.Contains(edge.Segment.StartPoint) {
					continue
				}
				// all other points attached to this node
				explore(path.CopyAndAdd(edge), edge.Opposite(node))
			}
		}
		explore(&Path{From: entry.Node}, entry.Node)
	}
}

var debugString string

func (n *Network) Reorder() error {

	debugString += fmt.Sprintln("*** Network:", n.Debug())
	debugString += fmt.Sprintf("Entry: %s\n", n.Entry.String())
	debugString += fmt.Sprintln("Segments:")
	for _, segment := range n.Segments {
		debugString += fmt.Sprintln(segment.String())
	}
	debugString += fmt.Sprintln("Points:")
	for point := range n.ShortEdgePaths {
		debugString += fmt.Sprintln(point.Debug())
	}
	debugString += fmt.Sprintln("Nodes:")
	for i, node := range n.Nodes {
		debugString += fmt.Sprintf("%d)", i)
		for _, point := range node.Points {
			debugString += fmt.Sprintf(" [%s]", point.Debug())
		}
		debugString += fmt.Sprintln()
	}
	debugString += fmt.Sprintln("Paths:")
	for point, path := range n.LongEdgePaths {
		debugString += fmt.Sprintf("* Point %s: (%.3f) - ", point.Debug(), path.Length)
		for j, edge := range path.Edges {
			if j > 0 {
				debugString += fmt.Sprint(" -> ")
			}
			debugString += fmt.Sprint(edge.Segment.String())
		}
		if len(path.Edges) == 0 {
			debugString += fmt.Sprint("(no edges)")
		}
		debugString += fmt.Sprintln()
	}

	usedSegments := map[*Segment]bool{}
	var orderedSegments []*Segment

	// To start, find the longest path that starts at the entry point. We only consider the shortest path to get to
	// each point.
	var longestPath *Path
	for _, path := range n.LongEdgePaths {
		if path.From != n.Entry.StartPoint.Node {
			continue
		}
		if longestPath == nil || path.Length > longestPath.Length {
			longestPath = path
		}
	}
	if longestPath == nil {
		return fmt.Errorf("couldn't find initial longest path for %s", n.Debug())
	}
	for _, edge := range longestPath.Edges {
		usedSegments[edge.Segment] = true
		orderedSegments = append(orderedSegments, edge.Segment)
	}

	// Subsequently find the longest uninterrupted stretch of edges that haven't been used.
	// Repeat this until all segments have been used. If we fail to find any segments then they might
	// not be on a shortest path. In that case, just use the original order.
	for len(usedSegments) < len(n.Segments) {
		var longestStretch []*Edge
		var longestLength float64
		for _, path := range n.LongEdgePaths {
			// find the first segment that hasn't been used, and walk forward until you find another that has been used.
			var index int
			for index < len(path.Edges) {
				stretch, length := findNextUnusedStretch(usedSegments, path, index)
				if length > longestLength {
					longestStretch = stretch
					longestLength = length
				}
				index += len(stretch) + 1
			}
		}
		if len(longestStretch) > 0 {
			for _, edge := range longestStretch {
				usedSegments[edge.Segment] = true
				if n.Route.Regular {
					// any segments after the main long route in regular routes are added to the
					// optional routes.
					return fmt.Errorf("non-linear segments in %s", n.Debug())
				} else {
					orderedSegments = append(orderedSegments, edge.Segment)
				}
			}
		} else {
			for _, segment := range n.Segments {
				if !usedSegments[segment] {
					usedSegments[segment] = true
					if n.Route.Regular {
						// any segments after the main long route in regular routes are added to the
						// optional routes.
						return fmt.Errorf("non-linear segments in %s", n.Debug())
					} else {
						orderedSegments = append(orderedSegments, segment)
					}
				}
			}
		}
	}
	debugString += fmt.Sprintln("NEW ORDER:")
	for i, segment := range orderedSegments {
		if i > 0 {
			debugString += fmt.Sprint(" ")
		}
		debugString += fmt.Sprint(segment.String())
	}
	debugString += fmt.Sprintln()
	debugString += fmt.Sprintln()
	n.Segments = make([]*Segment, len(orderedSegments))
	for i := range orderedSegments {
		n.Segments[i] = orderedSegments[i]
	}
	return nil
}

func findNextUnusedStretch(used map[*Segment]bool, path *Path, fromIndex int) (edges []*Edge, length float64) {
	for i := fromIndex; i < len(path.Edges); i++ {
		if used[path.Edges[i].Segment] {
			return edges, length
		}
		edges = append(edges, path.Edges[i])
		length += path.Edges[i].Length
	}
	return edges, length
}

func (n *Network) FindEntrySegment() {
	var force string
	switch {
	//case n.Route.Regular && n.Route.Section.Key.Code() == "25H" && n.Route.Hiking:
	//	force = "RR-MR-V@25H-0.0+0.7"
	//case n.Route.Regular && n.Route.Section.Key.Code() == "25H" && n.Route.Packrafting:
	//	force = "RP-LK-2@25H-0.0+3.2 (Lago Kruger)"
	case n.Route.Regular && n.Route.Section.Key.Code() == "91P":
		force = "RP-RI-1@91P- (Rio Exploradores)"
	}
	if force != "" {
		for _, segment := range n.Segments {
			if segment.Raw == force {
				n.Entry = segment
				return
			}
		}
		panic(fmt.Sprintf("can't find forced entry segment %s", force))
	}

	var lower func(s1, s2 *Segment) bool
	if n.Route.Regular || n.Route.OptionalKey.Alternatives {
		// regular route or hiking alternatives: find lowest From
		lower = func(s1, s2 *Segment) bool {
			if s1.From == 0.0 && s2.From == 0.0 {
				// If we have multiple segments with zero from, the segment code may tell us which should be the entry
				// segment. RP => first for packrafting.
				var values map[string]int
				if n.Route.Packrafting {
					values = map[string]int{
						"RP": 1,
						"RR": 2,
						"RH": 3,
					}
				} else {
					values = map[string]int{
						"RH": 1,
						"RR": 2,
						"RP": 3,
					}
				}
				// return true if s1 is the entry segment, false for s2
				return values[s1.Code] < values[s2.Code]
			}
			return s1.From < s2.From
		}
	} else {
		// optional routes: find lowest Count
		lower = func(s1, s2 *Segment) bool { return s1.Count < s2.Count }
	}
	var lowest *Segment
	for _, segment := range n.Segments {
		if lowest == nil || lower(segment, lowest) {
			lowest = segment
		}
	}
	n.Entry = lowest
}

func (n *Network) Debug() string {
	return fmt.Sprintf("%s #%d", n.Route.Debug(), n.Index())
}

func (n *Network) Index() int {
	for i, network := range n.Route.Networks {
		if network == n {
			return i
		}
	}
	panic("network not found in route")
}

// Path is a path through a network
type Path struct {
	From   *Node
	Edges  []*Edge
	Length float64
}

func (p1 *Path) Is(p2 *Path) bool {
	if p1.From != p2.From {
		return false
	}
	if len(p1.Edges) != len(p2.Edges) {
		return false
	}
	for i := range p1.Edges {
		if p1.Edges[i] != p2.Edges[i] {
			return false
		}
	}
	return true
}

func (p *Path) Copy() *Path {
	edges := make([]*Edge, len(p.Edges))
	copy(edges, p.Edges)
	return &Path{From: p.From, Edges: edges, Length: p.Length}
}

func (p *Path) CopyAndAdd(e *Edge) *Path {
	edges := make([]*Edge, len(p.Edges)+1)
	copy(edges, p.Edges)
	edges[len(edges)-1] = e
	return &Path{From: p.From, Edges: edges, Length: p.Length + e.Length}
}

func (p *Path) Has(edge *Edge) bool {
	for _, e := range p.Edges {
		if e == edge {
			return true
		}
	}
	return false
}

// Edge connects exactly two Nodes. Distinct from Segments because Segments can have several nodes along their length.
type Edge struct {
	Segment *Segment
	Nodes   [2]*Node
	Length  float64
}

func (e Edge) Has(n *Node) bool {
	return e.Nodes[0] == n || e.Nodes[1] == n
}

func (e Edge) Opposite(n *Node) *Node {
	if e.Nodes[0] == n {
		return e.Nodes[1]
	} else if e.Nodes[1] == n {
		return e.Nodes[0]
	}
	panic("should check edge has node before getting opposite")
}

// Node is a collection of Points in the same approximate location.
type Node struct {
	Network *Network
	Points  []*Point
}

func (n *Node) Debug() string {
	var s string
	for i, point := range n.Points {
		if i > 0 {
			s += " "
		}
		s += fmt.Sprint(point.Debug())
	}
	return s
}

func (n *Node) ContainsSegment(s *Segment) bool {
	for _, point := range n.Points {
		if point.Segment == s {
			return true
		}
	}
	return false
}

func (n *Node) Contains(p *Point) bool {
	for _, point := range n.Points {
		if point == p {
			return true
		}
	}
	return false
}

// Point is the start, end or mid-point of a Segment.
type Point struct {
	Node       *Node
	Segment    *Segment
	Start, End bool // Is this the start or end of the segment?
	Index      int  // The index in segment.Line
	Pos        geo.Pos
}

func (p Point) Debug() string {
	if p.Start {
		return fmt.Sprintf("%s start", p.Segment.Raw)
	} else if p.End {
		return fmt.Sprintf("%s end", p.Segment.Raw)
	} else {
		return fmt.Sprintf("%s #%d", p.Segment.Raw, p.Index)
	}
}

type Straight struct {
	Flushes  []*Flush
	Segments []*Segment
}

type Flush struct {
	From, Length float64
	Terrains     []string
	Verification string
	Directional  string
	Experimental bool
	Segments     []*Segment
}

func (f Flush) Description(id int, waypoint bool) string {
	var sb strings.Builder
	if !waypoint {
		sb.WriteString(fmt.Sprintf("#%d at %.1f km: ", id, f.From))
	}
	for i, terrain := range f.Terrains {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(Terrain(terrain))
	}
	properties := f.Properties()
	if len(properties) > 0 {
		sb.WriteString(fmt.Sprintf(" (%s)", strings.Join(properties, ", ")))
	}
	sb.WriteString(fmt.Sprintf(" for %.1f km", f.Length))
	if waypoint {
		sb.WriteString(fmt.Sprintf(" #%d", id))
	}
	return sb.String()
}

func (f Flush) Properties() []string {
	var properties []string
	if f.Verification != "" {
		properties = append(properties, f.Verification)
	}
	if f.Directional != "" {
		properties = append(properties, f.Directional)
	}
	if f.Experimental {
		properties = append(properties, "EXP")
	}
	return properties
}
