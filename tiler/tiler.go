package tiler

import (
	"fmt"
	"image/png"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/dave/gpt/globals"
	"github.com/dave/gpt/routedata"
	"github.com/fogleman/gg"
)

const (
	TileSize = 256
)

func Output(dpath string, data *routedata.Data) {
	fpath := dpath + "/tile.png"
	saveTileToFile(8, 76, 160, fpath, data)
	//http.HandleFunc("/tiles/", tileHandler)
	//http.ListenAndServe(":8080", nil)
}

func saveTileToFile(z, x, y int, filename string, data *routedata.Data) {
	img := renderTile(z, x, y, data)
	file, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	png.Encode(file, img.Image())
}

func tileHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid tile path", http.StatusBadRequest)
		return
	}

	z, err := strconv.Atoi(parts[2])
	if err != nil {
		http.Error(w, "Invalid zoom level", http.StatusBadRequest)
		return
	}

	x, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid x coordinate", http.StatusBadRequest)
		return
	}

	y, err := strconv.Atoi(parts[4])
	if err != nil {
		http.Error(w, "Invalid y coordinate", http.StatusBadRequest)
		return
	}

	img := renderTile(z, x, y, nil)
	w.Header().Set("Content-Type", "image/png")
	png.Encode(w, img.Image())
}

func renderTile(z, x, y int, data *routedata.Data) *gg.Context {
	dc := gg.NewContext(TileSize, TileSize)
	dc.SetRGBA(0, 0, 0, 0)
	dc.Clear()
	dc.SetRGB(0, 0, 0) // Set text color to black

	dc.SetLineWidth(1.0)
	dc.SetLineCap(gg.LineCapRound)
	dc.SetLineJoin(gg.LineJoinRound)

	// Draw debug text in the center of the tile
	debugText := fmt.Sprintf("x: %d, y: %d, z: %d", x, y, z)
	dc.DrawStringAnchored(debugText, TileSize/2, TileSize/2, 0.5, 0.5)

	for sectionKey, section := range data.Sections {

		_ = sectionKey
		for routeKey, route := range section.Routes {
			if routeKey.Required == globals.OPTIONAL {
				continue
			}
			var found bool
			for _, segment := range route.All {
				for i := 0; i < len(segment.Line)-1; i++ {
					foundThis, xThis, yThis := isLatLonInTile(segment.Line[i].Lat, segment.Line[i].Lon, z, x, y)
					foundNext, xNext, yNext := isLatLonInTile(segment.Line[i+1].Lat, segment.Line[i+1].Lon, z, x, y)
					if foundThis && foundNext {
						found = true
						dc.MoveTo(xThis, yThis)
						dc.LineTo(xNext, yNext)
					}
				}
			}
			if found {
				firstSegment := route.All[0]
				firstPoint := firstSegment.Line[0]
				startFound, startX, startY := isLatLonInTile(firstPoint.Lat, firstPoint.Lon, z, x, y)
				if startFound {
					dc.DrawStringAnchored("GPT"+section.Key.Code()+" (start)", startX, startY, 0, 0)
				}

				lastSegment := route.All[len(route.All)-1]
				lastPoint := lastSegment.Line[len(lastSegment.Line)-1]
				endFound, endX, endY := isLatLonInTile(lastPoint.Lat, lastPoint.Lon, z, x, y)
				if endFound {
					dc.DrawStringAnchored("GPT"+section.Key.Code()+" (end)", endX, endY, 0, 0)
				}
			}
		}
	}

	dc.Stroke()
	return dc
}

// latLonToTileXY converts latitude and longitude to tile x/y coordinates at a given zoom level.
func latLonToTileXY(lat, lon float64, zoom int) (int, int) {
	tileX := int((lon + 180.0) / 360.0 * math.Exp2(float64(zoom)))
	tileY := int((1.0 - math.Log(math.Tan(lat*math.Pi/180.0)+1.0/math.Cos(lat*math.Pi/180.0))/math.Pi) / 2.0 * math.Exp2(float64(zoom)))
	return tileX, tileY
}

// latLonToPixelXY converts latitude and longitude to pixel x/y coordinates within a tile.
func latLonToPixelXY(lat, lon float64, zoom int) (float64, float64) {
	sinLat := math.Sin(lat * math.Pi / 180.0)
	pixelX := ((lon + 180.0) / 360.0) * 256.0 * math.Exp2(float64(zoom))
	pixelY := (0.5 - math.Log((1.0+sinLat)/(1.0-sinLat))/(4.0*math.Pi)) * 256.0 * math.Exp2(float64(zoom))
	return pixelX, pixelY
}

// isLatLonInTile checks if a given lat/lon is inside a specific tile and returns the x/y position within the tile.
func isLatLonInTile(lat, lon float64, z, x, y int) (bool, float64, float64) {
	// Calculate the tile boundaries
	lonMin := float64(x)/math.Exp2(float64(z))*360.0 - 180.0
	lonMax := float64(x+1)/math.Exp2(float64(z))*360.0 - 180.0
	latMin := math.Atan(math.Sinh(math.Pi*(1-2*float64(y+1)/math.Exp2(float64(z))))) * 180.0 / math.Pi
	latMax := math.Atan(math.Sinh(math.Pi*(1-2*float64(y)/math.Exp2(float64(z))))) * 180.0 / math.Pi

	// Calculate the pixel x/y position within the tile
	pixelX, pixelY := latLonToPixelXY(lat, lon, z)
	tilePixelX := pixelX - float64(x)*256.0
	tilePixelY := pixelY - float64(y)*256.0

	// Check if the given lat/lon is within the tile boundaries
	return lat >= latMin && lat <= latMax && lon >= lonMin && lon <= lonMax, tilePixelX, tilePixelY
}
