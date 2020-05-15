package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

const WARNING_SYMBOL = "☞"
const HEADING_SYMBOL = "●" //"◉" //"✪" //"●" //"★" //"⦿"
const WAYPOINT_SYMBOL = "☉"
const ROUTE_SYMBOL = "⬲" //"⛢"

func (d *Data) Scrape() error {

	cachedir := path.Join(os.Getenv("HOME"), ".gpt-cache")
	_ = os.MkdirAll(cachedir, 0777)

	for _, key := range d.Keys {
		if HAS_SINGLE && key != SINGLE {
			continue
		}
		section := d.Sections[key]
		if err := section.Scrape(cachedir); err != nil {
			return fmt.Errorf("scraping GPT%s: %w", section.Key.Code(), err)
		}
	}
	return nil
}

func (s *Section) Scrape(cachedir string) error {
	var description string
	var summaryPackrafting, summaryHiking string
	write := func(s string) {
		description += s
	}
	writeS := func(s string) {
		summaryPackrafting += s
		summaryHiking += s
	}
	writeSP := func(s string) {
		summaryPackrafting += s
	}
	writeSH := func(s string) {
		summaryHiking += s
	}
	var reader io.Reader
	url := fmt.Sprintf("http://www.wikiexplora.com/GPT%s", s.Key.Code())
	cachefpath := filepath.Join(cachedir, fmt.Sprintf("GPT%s.html", s.Key.Code()))
	f, err := os.Open(cachefpath)
	if err == nil {
		logf("Web scrape data for GPT%s found in cache file %q\n", s.Key.Code(), cachefpath)
		reader = f
		defer f.Close()
	} else {
		if !os.IsNotExist(err) {
			return fmt.Errorf("opening file: %w", err)
		} else {
			logf("Scraping %q for description\n", url)
			// file not found
			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("getting %q: %w", url, err)
			}
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("reading %q: %w", url, err)
			}
			if err := ioutil.WriteFile(cachefpath, b, 0666); err != nil {
				return fmt.Errorf("writing %s: %w", cachefpath, err)
			}
			reader = bytes.NewBuffer(b)
		}
	}

	dom, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return fmt.Errorf("reading %s: %w", url, err)
	}
	ignored := map[*html.Node]bool{}
	dom.Find(".mw-headline").Each(func(i int, selection *goquery.Selection) {

		c := selection
		for c != nil && len(c.Nodes) > 0 {
			if ignored[c.Nodes[0]] {
				return
			}
			c = c.Parent()
		}

		var ignoreSection bool
		var summary []*html.Node
		title := strings.TrimSpace(getText(selection))
		switch title {
		case "Elevation Profile", "Satellite Image Map", "Summary Table", "Alerts and Logs of Past Seasons", "Older information for review", "Image Gallery":
			var ignoredCount int
			switch selection.Parent().Nodes[0].Data {
			case "h2":
				//ignoring an h2? skip all nodes until we find an h2
				next := selection.Parent().Next()
				for next != nil && len(next.Nodes) > 0 && next.Nodes[0].Data != "h2" {
					if title == "Summary Table" {
						// summary
						summary = append(summary, next.Nodes[0])
					}
					ignored[next.Nodes[0]] = true
					next = next.Next()
					ignoredCount++
				}
			case "h3", "h4", "h5":
				next := selection.Parent().Next()
				for next != nil && len(next.Nodes) > 0 && next.Nodes[0].Data != "h2" && next.Nodes[0].Data != "h3" && next.Nodes[0].Data != "h4" && next.Nodes[0].Data != "h5" {
					ignored[next.Nodes[0]] = true
					next = next.Next()
					ignoredCount++
				}
			}

			if len(summary) == 0 {
				if ignoredCount > 0 {
					write(HEADING_SYMBOL + " " + strings.TrimSpace(getText(selection)) + "\n\n")
					write(WARNING_SYMBOL + " Section removed - see web page.\n\n")
				}
			} else {
				writeS(HEADING_SYMBOL + " " + strings.TrimSpace(getText(selection)) + "\n\n")
				summary := goquery.Selection{Nodes: summary}
				trs := summary.Find("tr")
				trs.Each(func(i int, tr *goquery.Selection) {
					tds := tr.Find("td")
					firstColumn := strings.TrimSpace(tds.First().Text())
					switch firstColumn {
					case "Group", "Region", "Start", "Finish", "Status", "Traversable", "Packraft", "Connects to", "Options", "Comment", "Character", "Challenges":
						// one column
						writeS(firstColumn + ": " + strings.TrimSpace(getText(tds.First().Next())))
						writeS("\n")
					case "Attraction", "Difficulty", "Direction":
						// two columns
						writeSH(firstColumn + ": " + strings.TrimSpace(getText(tds.First().Next())))
						writeSP(firstColumn + ": " + strings.TrimSpace(getText(tds.First().Next().Next())))
						writeS("\n")
					}
				})
				writeS("\n")

			}
			return
			//case "Optional Routes":
			//	next := selection.Parent().Next()
			//	for next != nil && len(next.Nodes) > 0 && next.Nodes[0].Data != "h2" {
			//		fmt.Printf("GPT%s: %s %s\n", s.Key.Code(), next.Nodes[0].Data, getText(next))
			//		fmt.Println()
			//		next = next.Next()
			//	}
		}

		var section []*goquery.Selection
		next := selection.Parent().Next()
		for next != nil && len(next.Nodes) > 0 && next.Nodes[0].Data != "h2" && next.Nodes[0].Data != "h3" && next.Nodes[0].Data != "h4" && next.Nodes[0].Data != "h5" {
			switch {
			case strings.TrimSpace(getText(next)) == "To be issued.":
			case strings.TrimSpace(getText(next)) == "Not applicable.":
				// nothing
			default:
				section = append(section, next)
			}
			next = next.Next()
		}

		if ignoreSection {
			return
		} else if len(section) > 0 {
			write(HEADING_SYMBOL + " " + strings.TrimSpace(getText(selection)) + "\n\n")
			for _, part := range section {
				if part.Nodes[0].Data == "table" {
					write(WARNING_SYMBOL + " Table removed - see web page.\n\n")
				} else if str := strings.TrimSpace(getText(part)); len(str) > 0 {
					write(str + "\n\n")
				}
			}
		}
	})

	if s.Hiking != nil {
		s.Hiking.Scraped += fmt.Sprintf(HEADING_SYMBOL+" Full information\n\nThe following information may be incomplete and out of date. Be sure to check the up to date source:\n\n%s\n\n", url)
		s.Hiking.Scraped += summaryHiking
		s.Hiking.Scraped += description
	}

	if s.Packrafting != nil {
		s.Packrafting.Scraped += fmt.Sprintf(HEADING_SYMBOL+" Full information\n\nThe following information may be incomplete and out of date. Be sure to check the up to date source:\n\n%s\n\n", url)
		s.Packrafting.Scraped += summaryPackrafting
		s.Packrafting.Scraped += description
	}

	return nil
}

func getTextRaw(n *html.Node) string {
	if n == nil {
		return ""
	}
	var buf bytes.Buffer
	// Slightly optimized vs calling Each: no single selection object created
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			// Keep newlines and spaces, like jQuery
			buf.WriteString(n.Data)
		}
		if n.FirstChild != nil {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
	}
	f(n)

	return buf.String()
}

func getText(s *goquery.Selection) string {
	var buf bytes.Buffer

	// Slightly optimized vs calling Each: no single selection object created
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			// Keep newlines and spaces, like jQuery
			buf.WriteString(n.Data)
		}
		if n.FirstChild != nil {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
		if n.Data == "a" {
			text := strings.TrimSpace(getTextRaw(n))
			// find href
			var href string
			for _, attribute := range n.Attr {
				if attribute.Key == "href" {
					href = strings.TrimSpace(attribute.Val)
					break
				}
			}
			if strings.HasPrefix(href, "/") {
				href = "http://www.wikiexplora.com" + href
			}
			if href != "" && !strings.HasPrefix(href, "#") && href != text {
				buf.WriteString(fmt.Sprintf(" [%s]", href))
			}
		}
	}
	for _, n := range s.Nodes {
		f(n)
	}
	str := buf.String()
	str = strings.ReplaceAll(str, "\uF0B1", WAYPOINT_SYMBOL)
	str = strings.ReplaceAll(str, "\uF08F", ROUTE_SYMBOL)
	return str
}
