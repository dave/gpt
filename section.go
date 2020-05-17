package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Section is a section folder
type Section struct {
	Raw       string // raw name of the section folder
	Key       SectionKey
	Name      string // name of the section
	RouteKeys []RouteKey
	Routes    map[RouteKey]*Route
	Waypoints []Waypoint
	Scraped   map[ModeType]string
}

func (s Section) FolderName() string {
	return fmt.Sprintf("GPT%s (%s)", s.Key.Code(), s.Name)
}

func (s Section) String() string {
	return fmt.Sprintf("GPT%s-%s", s.Key.Code(), s.Name)
}

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
