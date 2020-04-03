package gpx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
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
	Lat float64 `xml:"lat,attr"`
	Lon float64 `xml:"lon,attr"`
	Ele float64 `xml:"ele,omitempty"`
}

func PosPoint(p geo.Pos) Point {
	return Point{
		Lat: p.Lat,
		Lon: p.Lon,
		Ele: p.Ele,
	}
}

func (p Point) Pos() geo.Pos {
	return geo.Pos{
		Lat: p.Lat,
		Lon: p.Lon,
		Ele: p.Ele,
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
