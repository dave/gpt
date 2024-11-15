package gpx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dave/gpt/geo"
)

func Load(fpath string) (Root, error) {
	b, err := ioutil.ReadFile(fpath)
	if err != nil {
		return Root{}, fmt.Errorf("reading gpx %q: %w", fpath, err)
	}
	return Decode(bytes.NewBuffer(b))
}

func Decode(reader io.Reader) (Root, error) {
	var r Root
	if err := xml.NewDecoder(reader).Decode(&r); err != nil {
		return Root{}, fmt.Errorf("decoding gpx: %w", err)
	}
	return r, nil
}

// Paged is a helper for paginated GPX files. When saving, it will split the output over several files. Buckets of
// items are kept together. Buckets are ordered.
type Paged struct {
	Version float64
	Max     int
	Buckets []*Bucket
}

func (p *Paged) Save(fpath string) error {
	fpath = strings.TrimSuffix(fpath, ".gpx")
	sort.Slice(p.Buckets, func(i, j int) bool { return p.Buckets[i].Order > p.Buckets[j].Order })
	var roots []*Root
	current := &Root{Version: p.Version}
	currentItemCount := 0
	for _, bucket := range p.Buckets {
		if currentItemCount > 0 && currentItemCount+bucket.Items() > p.Max {
			roots = append(roots, current)
			current = &Root{Version: p.Version}
			currentItemCount = 0
		}
		current.Waypoints = append(current.Waypoints, bucket.Waypoints...)
		current.Routes = append(current.Routes, bucket.Routes...)
		current.Tracks = append(current.Tracks, bucket.Tracks...)
		currentItemCount += bucket.Items()
	}
	roots = append(roots, current)
	for i, root := range roots {
		var fpathi string
		if len(roots) > 1 {
			fpathi = fmt.Sprintf("%s [%d of %d].gpx", fpath, i+1, len(roots))
		} else {
			fpathi = fmt.Sprintf("%s.gpx", fpath)
		}
		if err := root.Save(fpathi); err != nil {
			return err
		}
	}
	return nil
}

type Bucket struct {
	Order     float64
	Waypoints []Waypoint
	Tracks    []Track
	Routes    []Route
}

func (b *Bucket) Items() int {
	total := len(b.Waypoints) + len(b.Routes)
	for _, track := range b.Tracks {
		total += len(track.Segments)
	}
	return total
}

type Root struct {
	Version   float64    `xml:"version,attr"`
	Waypoints []Waypoint `xml:"wpt"`
	Tracks    []Track    `xml:"trk"`
	Routes    []Route    `xml:"rte"`
}

func (r Root) Save(fpath string) error {
	dpath, _ := filepath.Split(fpath)
	_ = os.MkdirAll(dpath, 0777)
	wrapper := struct {
		Root
		XMLName struct{} `xml:"gpx"`
	}{Root: r}
	bw, err := xml.MarshalIndent(wrapper, "", "\t")
	//bw, err := xml.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshing gpx: %w", err)
	}
	if err := ioutil.WriteFile(fpath, []byte(xml.Header+string(bw)), 0666); err != nil {
		return fmt.Errorf("writing gpx file %q: %w", fpath, err)
	}
	return nil
}

type Waypoint struct {
	Point
	Name string `xml:"name"`
	Sym  string `xml:"sym,omitempty"`
	Desc string `xml:"desc,omitempty"`
}

type Route struct {
	Name   string  `xml:"name"`
	Desc   string  `xml:"desc"`
	Points []Point `xml:"rtept"`
}

type Point struct {
	Lat geo.FloatFive `xml:"lat,attr"`
	Lon geo.FloatFive `xml:"lon,attr"`
	Ele geo.FloatZero `xml:"ele,omitempty"`
}

func PosPoint(p geo.Pos) Point {
	return Point{
		Lat: geo.FloatFive(p.Lat),
		Lon: geo.FloatFive(p.Lon),
		Ele: geo.FloatZero(p.Ele),
	}
}

func (p Point) Pos() geo.Pos {
	return geo.Pos{
		Lat: float64(p.Lat),
		Lon: float64(p.Lon),
		Ele: float64(p.Ele),
	}
}

type TrackPoint struct {
	Point
	Time *time.Time `xml:"time,omitempty"`
}

type Track struct {
	Name     string         `xml:"name"`
	Desc     string         `xml:"desc"`
	Segments []TrackSegment `xml:"trkseg"`
}

type TrackSegment struct {
	Points []TrackPoint `xml:"trkpt"`
}

func (t TrackSegment) Line() geo.Line {
	l := make(geo.Line, len(t.Points))
	for i, point := range t.Points {
		l[i] = point.Pos()
	}
	return l
}

func LinePoints(l geo.Line) []Point {
	points := make([]Point, len(l))
	for i, pos := range l {
		points[i] = PosPoint(pos)
	}
	return points
}

func LineTrackPoints(l geo.Line) []TrackPoint {
	points := make([]TrackPoint, len(l))
	for i, pos := range l {
		points[i] = TrackPoint{Point: PosPoint(pos)}
	}
	return points
}
