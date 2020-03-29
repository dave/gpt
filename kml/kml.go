package kml

import (
	"math"
	"strconv"
	"strings"
)

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

/*
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

type Root struct {
	Xmlns    string   `xml:"xmlns,attr"`
	Document Document `xml:"Document"`
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

type LineString struct {
	Extrude      bool   `xml:"extrude"`
	Tessellate   bool   `xml:"tessellate"`
	AltitudeMode string `xml:"altitudeMode"`
	Coordinates  string `xml:"coordinates"`
}

type MultiGeometry struct {
	LineString *LineString `xml:"LineString,omitempty"`
}

type Location struct {
	Lat, Lon, Ele float64
}

func (l LineString) Reverse() string {
	c := l.Coordinates
	c = strings.ReplaceAll(c, "\t", "")
	c = strings.ReplaceAll(c, "\n", "")
	c = strings.TrimSuffix(c, " ")
	s := strings.Split(c, " ")
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return strings.Join(s, " ")
}

func (l LineString) Locations() []Location {
	var locations []Location
	c := l.Coordinates
	c = strings.ReplaceAll(c, "\t", "")
	c = strings.ReplaceAll(c, "\n", "")
	c = strings.TrimSuffix(c, " ")
	for _, csv := range strings.Split(c, " ") {
		parts := strings.Split(csv, ",")
		var l Location
		l.Lon, _ = strconv.ParseFloat(parts[0], 64)
		l.Lat, _ = strconv.ParseFloat(parts[1], 64)
		l.Ele, _ = strconv.ParseFloat(parts[2], 64)
		locations = append(locations, l)
	}
	return locations
}

// distance in km to another location (only considering lat and lon)
func (l Location) Distance(to Location) float64 {
	const PI float64 = 3.141592653589793

	radlat1 := float64(PI * l.Lat / 180)
	radlat2 := float64(PI * to.Lat / 180)

	theta := float64(l.Lon - to.Lon)
	radtheta := float64(PI * theta / 180)

	dist := math.Sin(radlat1)*math.Sin(radlat2) + math.Cos(radlat1)*math.Cos(radlat2)*math.Cos(radtheta)

	if dist > 1 {
		dist = 1
	}

	dist = math.Acos(dist)
	dist = dist * 180 / PI
	dist = dist * 60 * 1.1515

	dist = dist * 1.609344

	return dist
}
