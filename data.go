package main

import (
	"fmt"
	"sort"
	"strconv"
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

// Section is a section folder
type Section struct {
	Raw         string // raw name of the section folder
	Key         SectionKey
	Name        string   // name of the section
	Tracks      []*Track // raw tracks from the kml data
	Hiking      *Bundle  // If this section has a regular route that does not include packrafting, it's here.
	Packrafting *Bundle  // This is the regular route for this section with packrafting trails chosen when possible.
	Waypoints   []Waypoint
}

func (s Section) String() string {
	return fmt.Sprintf("GPT%s-%s", s.Key.Code(), s.Name)
}

type Bundle struct {
	Regular map[RegularKey]*Route  // The regular route for this section.
	Options map[OptionalKey]*Route // Options and variants for this section. For the hiking bundle any options with packrafting terrain type are excluded.
}

// Post build tasks - normalise, tweak elevations for water, calculate stats etc.
func (b *Bundle) Post() error {

	for _, route := range b.Regular {
		if err := route.BuildNetworks(); err != nil {
			return fmt.Errorf("building networks for regular route: %w", err)
		}
		for _, network := range route.Networks {
			network.Normalise()
		}
	}
	for key, route := range b.Options {
		if err := route.BuildNetworks(); err != nil {
			return fmt.Errorf("building networks for optional route %s: %w", key.Code(), err)
		}
		for _, network := range route.Networks {
			network.Normalise()
		}
	}

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

// CalculateSegmentStats calculates and corrects the Length data (and From data for regular routes) for each segment.
/*
func (r *Route) CalculateSegmentStats() {
	if !r.Regular {
		return
	}
	var l float64
	for _, segment := range r.Segments {
		segment.From = l
		l += segment.Length
	}
}
*/

type SectionKey struct {
	Number int
	Suffix string
}

func (k SectionKey) Code() string {
	return fmt.Sprintf("%02d%s", k.Number, k.Suffix)
}

func NewSectionKey(code string) (SectionKey, error) {
	var key SectionKey
	code = strings.TrimSpace(code)
	code = strings.TrimPrefix(code, "GPT")

	// TODO: remove this (typeo "GPP36H/36P-G (Lago Ciervo)")
	code = strings.TrimPrefix(code, "GPP")

	if strings.HasSuffix(code, "P") {
		key.Suffix = "P"
		code = strings.TrimSuffix(code, "P")
	}
	if strings.HasSuffix(code, "H") {
		key.Suffix = "H"
		code = strings.TrimSuffix(code, "H")
	}
	number, err := strconv.Atoi(code)
	if err != nil {
		return SectionKey{}, err
	}
	key.Number = number
	return key, nil
}

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

const DELTA = 0.075 // see https://docs.google.com/spreadsheets/d/1q610i2TkfUTHWvtqVAJ0V8zFtzPMQKBXEm7jiPyuDCQ/edit

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
		// if node has more than 1 point from the same segment, split and assign the other points based on distance
		hasMultiple := func(node *Node) (bool, *Segment) {
			segments := map[*Segment]bool{}
			for _, point := range node.Points {
				if segments[point.Segment] {
					return true, point.Segment
				}
				segments[point.Segment] = true
			}
			return false, nil
		}
		var splitIfHasMultiple func(node *Node)
		splitIfHasMultiple = func(node *Node) {
			if found, segment := hasMultiple(node); found {
				var newNodes []*Node
				for _, point := range node.Points {
					if point.Segment == segment {
						newNodes = append(newNodes, &Node{Points: []*Point{point}})
					}
				}
				for _, point := range node.Points {
					if point.Segment == segment {
						continue
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

	if false {
		fmt.Println()
		fmt.Println("***", r.Debug())
		for i, node := range nodes {
			fmt.Printf("#%d\n", i)
			for _, point := range node.Points {
				fmt.Printf("* %s\n", point.Debug())
			}
			fmt.Println()
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

	if false {
		if len(r.Networks) > 1 {
			fmt.Printf("%s %d segments %d networks\n", r.Debug(), len(r.Segments), len(r.Networks))
		}

		for i, network := range r.Networks {
			fmt.Printf("#%d\n", i)
			for _, segment := range network.Segments {
				fmt.Printf("* %s\n", segment.String())
			}
			fmt.Println()
		}
	}

	return nil
}

// Network is a collection of Segments which adjoin at nodes. There is a node at the start and end of
// each segment, and also at the mid point of each segment where another segment's end point adjoins.
type Network struct {
	Route    *Route
	Nodes    []*Node
	Segments []*Segment
	Extras   []*Segment // Regular segments sometimes have spurs which can't be ordered in a continuous gpx route. We add them as an optional track.
	Entry    *Segment

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
				if n.LongEdgePaths[point] == nil || n.LongEdgePaths[point].Length > path.Length {
					n.LongEdgePaths[point] = path.Copy()
				}
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

func (n *Network) Reorder() {

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
	var extraSegments []*Segment

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
		panic("couldn't find initial longest path!")
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
					extraSegments = append(extraSegments, edge.Segment)
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
						extraSegments = append(extraSegments, segment)
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
	if len(extraSegments) > 0 {
		debugString += fmt.Sprintln("EXTRAS:")
		for i, segment := range extraSegments {
			if i > 0 {
				debugString += fmt.Sprint(" ")
			}
			debugString += fmt.Sprint(segment.String())
		}
	}
	debugString += fmt.Sprintln()
	n.Segments = make([]*Segment, len(orderedSegments))
	for i := range orderedSegments {
		n.Segments[i] = orderedSegments[i]
	}
	if len(extraSegments) > 0 {
		n.Extras = extraSegments
	}
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

func (n *Network) Normalise() {

	//if n.Debug() != "GPT02 hiking - variant H #1" {
	//	return
	//}

	if false {
		if n.Debug() != "GPT07 hiking - option 2B (El Troncoso) #0" {
			return
		}
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

	n.Reorder()
}

func (n *Network) FindEntrySegment() {
	var lower func(s1, s2 *Segment) bool
	if n.Route.Regular || n.Route.OptionalKey.Alternatives {
		// regular route or hiking alternatives: find lowest From
		lower = func(s1, s2 *Segment) bool { return s1.From < s2.From }
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
	panic("should check of edge has node before getting opposite")
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
		return fmt.Sprintf("%s start", p.Segment.String())
	} else if p.End {
		return fmt.Sprintf("%s end", p.Segment.String())
	} else {
		return fmt.Sprintf("%s #%d", p.Segment.String(), p.Index)
	}
}

// Track is a track folder in a section folder
type Track struct {
	*Section
	Raw          string // raw name of the track folder
	Optional     bool   // is this section in the "Optional Tracks" folder?
	Experimental bool   // track folder has "EXP-" prefix
	Code         string // track type code - RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
	Direction    string // direction type - N: north, S: south, "": any
	Year         int    // year in brackets in the track folder
	Variants     bool   // track folder is named "Variants"
	Option       int    // option number if the track folder is named "Option X"
	Name         string // track name for optional tracks
	Segments     []*Segment
}

func (t Track) String() string {
	var b strings.Builder
	if t.Optional {
		if t.Variants {
			b.WriteString("Variants")
		} else {
			b.WriteString("Option ")
			b.WriteString(fmt.Sprint(t.Option))
			b.WriteString(" ")
			b.WriteString(t.Name)
		}
	} else {
		if t.Experimental {
			b.WriteString("EXP")
			b.WriteString("-")
		}
		b.WriteString(t.Code)
		b.WriteString(t.Direction)
	}
	if t.Year > 0 {
		b.WriteString(" (")
		b.WriteString(fmt.Sprintf("%04d", t.Year))
		b.WriteString(")")
	}
	return b.String()
}

// Segment is a placemark / linestring in a track folder
type Segment struct {
	*Track
	Raw          string   // raw name of the placemark
	Experimental bool     // segment name has "EXP-" prefix
	Code         string   // track code from the segment name - RR: Regular Route, RH: Regular Hiking Route, RP: Regular Packrafting Route, OH: Optional Hiking Route, OP: Optional Packrafting Route
	Terrains     []string // terrain codes from segment name - BB: Bush Bashing, CC: Cross Country, MR: Minor Road, PR: Primary or Paved Road, TL: Horse or Hiking Trail, FJ: Fjord Packrafting, LK: Lake Packrafting, RI: River Packrafting, FY: Ferry
	Verification string   // verification status - V: Verified Route, A: Approximate Route, I: Investigation Route
	Directional  string   // directional status - 1: One-Way Route, 2: Two-Way Route
	Variant      string   // variant from segment name
	Count        int      // counter for optional track
	From         float64  // from km for regular track
	Length       float64  // length km for regular track
	Name         string   // named feature
	Line         geo.Line
	StartPoint   *Point
	EndPoint     *Point
	MidPoints    []*Point
}

func (s *Segment) DuplicateForTrack() *Segment {
	// Segments can't be shared between packrafting and hiking routes because we may need to reverse the segment in one
	// route not in the other. So when we add a segment to a route, we duplicate it.
	out := &Segment{
		Track:        s.Track,
		Raw:          s.Raw,
		Experimental: s.Experimental,
		Code:         s.Code,
		Verification: s.Verification,
		Directional:  s.Directional,
		Variant:      s.Variant,
		Count:        s.Count,
		From:         s.From,
		Length:       s.Length,
		Name:         s.Name,

		// not assigned yet so no need to copy
		StartPoint: nil,
		EndPoint:   nil,
		MidPoints:  nil,

		// below
		Terrains: nil,
		Line:     nil,
	}
	line := make(geo.Line, len(s.Line))
	for i, pos := range s.Line {
		line[i] = pos
	}
	out.Line = line

	terrains := make([]string, len(s.Terrains))
	for i, t := range s.Terrains {
		terrains[i] = t
	}
	out.Terrains = terrains
	return out
}

func (s *Segment) Reverse() {
	debugString += fmt.Sprintf("Reversing %s\n", s.String())
	s.Line.Reverse()
	s.StartPoint, s.EndPoint = s.EndPoint, s.StartPoint

	s.StartPoint.Start = true
	s.StartPoint.End = false
	s.StartPoint.Index = 0

	s.EndPoint.Start = false
	s.EndPoint.End = true
	s.EndPoint.Index = len(s.Line) - 1

	for _, point := range s.MidPoints {
		point.Index = len(s.Line) - 1 - point.Index
	}
}

func (s *Segment) Points(reorder bool) []*Point {
	if reorder {
		// Make sure mid points are ordered correctly
		sort.Slice(s.MidPoints, func(i, j int) bool { return s.MidPoints[i].Index < s.MidPoints[j].Index })
	}
	return append([]*Point{s.StartPoint, s.EndPoint}, s.MidPoints...)
}

func (s Segment) String() string {
	var b strings.Builder
	if s.Experimental {
		b.WriteString("EXP")
		b.WriteString("-")
	}
	b.WriteString(s.Code)
	b.WriteString("-")
	b.WriteString(strings.Join(s.Terrains, "&"))
	if s.Verification != "" || s.Directional != "" {
		b.WriteString("-")
		b.WriteString(s.Verification)
		b.WriteString(s.Directional)
	}
	b.WriteString("@")
	b.WriteString(s.Section.Key.Code())
	if s.Optional {
		b.WriteString("-")
		if s.Option > 0 {
			b.WriteString(fmt.Sprintf("%02d", s.Option))
		}
		b.WriteString(s.Variant)
		b.WriteString("-")
		b.WriteString(fmt.Sprintf("#%03d", s.Count))
	} else {
		b.WriteString("-")
		b.WriteString(fmt.Sprintf("%.1f+%.1f", s.From, s.Length))
	}
	if s.Name != "" {
		b.WriteString(" (")
		b.WriteString(s.Name)
		b.WriteString(")")
	}
	return b.String()
}

func (s1 Segment) Similar(s2 *Segment) bool {
	return compareTerrain(s1.Terrains, s2.Terrains) &&
		s1.Verification == s2.Verification &&
		s1.Directional == s2.Directional &&
		s1.Experimental == s2.Experimental
}

// Compares two unordered slices of terrain types.
func compareTerrain(a1, a2 []string) bool {
	if len(a1) != len(a2) {
		return false
	}
	for _, t1 := range a1 {
		var found bool
		for _, t2 := range a2 {
			if t1 == t2 {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func Directional(code string) string {
	switch code {
	case "1":
		return "One-Way"
	case "2":
		return "Two-Way"
	}
	return ""
}

func Verification(code string) string {
	switch code {
	case "V":
		return "Verified"
	case "A":
		return "Approximate"
	case "I":
		return "Investigation"
	}
	return ""
}

func HasFerry(terrains []string) bool {
	for _, terrain := range terrains {
		switch terrain {
		case "FY":
			return true
		}
	}
	return false
}

func HasPackrafting(terrains []string) bool {
	for _, terrain := range terrains {
		switch terrain {
		case "FJ", "LK", "RI":
			return true
		}
	}
	return false
}

func Terrain(code string) string {
	switch code {
	case "BB":
		return "Bush Bashing"
	case "CC":
		return "Cross Country"
	case "MR":
		return "Minor Road"
	case "PR":
		return "Paved Road"
	case "TL":
		return "Trail"
	case "FJ":
		return "Fjord"
	case "LK":
		return "Lake"
	case "RI":
		return "River"
	case "FY":
		return "Ferry"
	}
	return ""
}

// Index in the track folder
func (s *Segment) Index() int {
	for i, segment := range s.Track.Segments {
		if s == segment {
			return i
		}
	}
	panic("can't find segment in track")
}
