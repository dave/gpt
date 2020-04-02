package kml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/dave/gpt/geo"
)

func Load(fpath string) (Root, error) {
	b, err := ioutil.ReadFile(fpath)
	if err != nil {
		return Root{}, fmt.Errorf("reading kml %q: %w", fpath, err)
	}
	return Decode(bytes.NewBuffer(b))
}

func Decode(reader io.Reader) (Root, error) {
	var r Root
	if err := xml.NewDecoder(reader).Decode(&r); err != nil {
		return Root{}, fmt.Errorf("decoding kml: %w", err)
	}
	return r, nil
}

type Root struct {
	Xmlns    string   `xml:"xmlns,attr"`
	Document Document `xml:"Document"`
}

func (r Root) Save(fpath string) error {
	wrapper := struct {
		Root
		XMLName struct{} `xml:"kml"`
	}{Root: r}
	bw, err := xml.MarshalIndent(wrapper, "", "\t")
	if err != nil {
		return fmt.Errorf("marshing kml: %w", err)
	}
	if err := ioutil.WriteFile(fpath, []byte(xml.Header+string(bw)), 0666); err != nil {
		return fmt.Errorf("writing kml file %q: %w", fpath, err)
	}
	return nil
}

type Document struct {
	Name        string    `xml:"name"`
	Description string    `xml:"description"`
	Visibility  int       `xml:"visibility"`
	Open        int       `xml:"open"`
	Styles      []*Style  `xml:"Style"`
	Folders     []*Folder `xml:"Folder"`
}

type Style struct {
	Id        string    `xml:"id,attr,omitempty"`
	LineStyle LineStyle `xml:"LineStyle"`
}

type LineStyle struct {
	Color string  `xml:"color"`
	Width float64 `xml:"width,omitempty"`
}

type Folder struct {
	Name        string       `xml:"name"`
	Description string       `xml:"description"`
	Visibility  int          `xml:"visibility"`
	Open        int          `xml:"open"`
	Placemarks  []*Placemark `xml:"Placemark"`
	Folders     []*Folder    `xml:"Folder"`
}

type Placemark struct {
	Name          string         `xml:"name"`
	Description   string         `xml:"description"`
	Visibility    int            `xml:"visibility"`
	Open          int            `xml:"open"`
	StyleUrl      string         `xml:"styleUrl,omitempty"`
	Point         *Point         `xml:"Point,omitempty"`
	LineString    *LineString    `xml:"LineString,omitempty"`
	MultiGeometry *MultiGeometry `xml:"MultiGeometry,omitempty"`
	Style         *Style         `xml:"Style"`
}

type Point struct {
	Coordinates string `xml:"coordinates"`
}

func (p Point) Pos() geo.Pos {
	var pos geo.Pos
	parts := strings.Split(strings.TrimSpace(p.Coordinates), ",")
	pos.Lon, _ = strconv.ParseFloat(parts[0], 64)
	pos.Lat, _ = strconv.ParseFloat(parts[1], 64)
	pos.Ele, _ = strconv.ParseFloat(parts[2], 64)
	return pos
}

type LineString struct {
	Extrude      bool   `xml:"extrude"`
	Tessellate   bool   `xml:"tessellate"`
	AltitudeMode string `xml:"altitudeMode"`
	Coordinates  string `xml:"coordinates"`
}

type MultiGeometry struct {
	LineString *LineString `xml:"LineString,omitempty"`
}

func (l LineString) Line() geo.Line {
	points := strings.Split(strings.TrimSpace(l.Coordinates), " ")
	line := make(geo.Line, len(points))
	for i, csv := range points {
		var p geo.Pos
		parts := strings.Split(csv, ",")
		p.Lon, _ = strconv.ParseFloat(parts[0], 64)
		p.Lat, _ = strconv.ParseFloat(parts[1], 64)
		p.Ele, _ = strconv.ParseFloat(parts[2], 64)
		line[i] = p
	}
	return line
}

func LineCoordinates(line geo.Line) string {
	var sb strings.Builder
	for i, pos := range line {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(PosCoordinates(pos))
	}
	return sb.String()
}

func PosCoordinates(pos geo.Pos) string {
	return fmt.Sprintf("%v,%v,%v", pos.Lon, pos.Lat, pos.Ele)
}

/*

var Colors = []struct{ Name, Color string }{
	{"red", "961400FF"},
	{"green", "9678FF00"},
	{"blue", "96FF7800"},
	{"cyan", "96F0FF14"},
	{"orange", "961478FF"},
	{"dark_green", "96008C14"},
	{"purple", "96FF7878"},
	{"pink", "96A078F0"},
	{"brown", "96143C96"},
	{"dark_blue", "96F01414"},
}

func GpxToKml(g gpx) kml {

	var styles []*Style
	for _, c := range kmlColors {
		styles = append(styles, &Style{
			Id: c.Name,
			LineStyle: LineStyle{
				Color: c.Color,
				Width: 4,
			},
		})
	}

	var folders []*Folder
	if len(g.Waypoints) > 0 {
		waypointFolder := &Folder{
			Name:        "Waypoints",
			Description: "",
			Visibility:  1,
			Open:        0,
		}
		for _, w := range g.Waypoints {
			waypointFolder.Placemarks = append(waypointFolder.Placemarks, &Placemark{
				Name:        w.Name,
				Description: w.Desc,
				Visibility:  1,
				Open:        0,

				Point: &Point{
					Coordinates: PointToCoodinates(w.Point),
				},
			})
		}
		folders = append(folders, waypointFolder)
	}
	if len(g.Routes) > 0 {
		routesFolder := &Folder{
			Name:        "Routes",
			Description: "",
			Visibility:  1,
			Open:        0,
		}
		//for i, r := range g.Routes {
		for _, r := range g.Routes {
			routesFolder.Placemarks = append(routesFolder.Placemarks, &Placemark{
				Name:        r.Name,
				Description: r.Desc,
				Visibility:  0,
				Open:        0,
				//StyleUrl:    fmt.Sprintf("#%s", kmlColors[i%len(kmlColors)].Name),
				//StyleUrl: "#blue",
				LineString: &LineString{
					Extrude:      true,
					Tessellate:   true,
					AltitudeMode: "clampToGround",
					Coordinates:  PointsToCoodinates(r.Points),
				},
				Style: &Style{
					LineStyle: LineStyle{
						Color: "#9678FF00",
						//Width: 2,
					},
				},
			})
		}
		folders = append(folders, routesFolder)
	}

	k := kml{
		Xmlns: "http://www.opengis.net/kml/2.2",
		Document: Document{
			Name:        "Great Himalaya Trail",
			Description: "",
			Visibility:  1,
			Open:        1,
			Styles:      styles,
			Folders:     folders,
		},
	}
	return k
}
*/

/*
<?xml version="1.0" encoding="UTF-8"?>
<kml>
	<Document>
		<name>Great Himalaya Trail</name>
        <description>...</description>
        <visibility>1</visibility>
        <open>1</open>
        <Style id="route_red">
            <LineStyle>
            <color>961400FF</color>
            <width>4</width>
            </LineStyle>
        </Style>
        ...

        <Folder>
            <name>Waypoints</name>
            <description>...</description>
            <visibility>1</visibility>
            <open>0</open>

            <Placemark>
                <name>...</name>
                <visibility>1</visibility>
                <open>0</open>
                <description>...</description>
                <Point>
                    <coordinates>
                        lat,lon,ele
                    </coordinates>
                </Point>
            </Placemark>
            ...
		</Folder>

		<Folder>
            <name>Routes</name>
            <description>...</description>
            <visibility>1</visibility>
            <open>0</open>

			<Placemark>
                <visibility>0</visibility>
                <open>0</open>
                <styleUrl>#route_red</styleUrl>
                <name>...</name>
                <description>...</description>
                <LineString>
                    <extrude>true</extrude>
                    <tessellate>true</tessellate>
                    <altitudeMode>clampToGround</altitudeMode>
                    <coordinates>
                        lat,lon,ele lat,lon,ele lat,lon,ele
                    </coordinates>
                </LineString>
            </Placemark>
            ...

        </Folder>
	</Document>
</kml>
*/
