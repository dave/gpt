package geo

import (
	"math"
)

type Line []Pos

func (l Line) Length() float64 {
	var total float64
	for i, pos := range l {
		if i == 0 {
			continue
		}
		total += l[i-1].Distance(pos)
	}
	return total
}

func (l Line) Reverse() {
	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}
}

// Start is the first Pos in the line
func (l Line) Start() Pos {
	return l[0]
}

// End is the last Pos in the line
func (l Line) End() Pos {
	return l[len(l)-1]
}

func MergeLines(lines []Line) Line {
	var totalLen int
	for _, s := range lines {
		totalLen += len(s)
	}
	tmp := make(Line, totalLen)
	var i int
	for _, s := range lines {
		i += copy(tmp[i:], s)
	}
	return tmp
}

type Pos struct {
	Lat, Lon, Ele float64
}

// distance in km to another location (only considering lat and lon)
func (p1 Pos) Distance(p2 Pos) float64 {
	const PI float64 = 3.141592653589793

	radlat1 := float64(PI * p1.Lat / 180)
	radlat2 := float64(PI * p2.Lat / 180)

	theta := float64(p1.Lon - p2.Lon)
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
