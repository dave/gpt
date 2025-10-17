package routedata

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dave/gpt/globals"
)

// Section is a section folder
type Section struct {
	Raw       string // raw name of the section folder
	Key       globals.SectionKey
	Name      string // name of the section
	RouteKeys []RouteKey
	Routes    map[RouteKey]*Route
	Waypoints []Waypoint
	Scraped   map[globals.ModeType]string
}

func (s Section) FolderName() string {
	return fmt.Sprintf("GPT%s (%s)", s.Key.Code(), s.Name)
}

func NewSectionKey(code string) (globals.SectionKey, error) {
	var key globals.SectionKey
	code = strings.TrimSpace(code)
	code = strings.TrimPrefix(code, "GPT")

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
		return globals.SectionKey{}, err
	}
	key.Number = number
	return key, nil
}
