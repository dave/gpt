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

func (d *Data) Scrape() error {

	cachedir := path.Join(os.Getenv("HOME"), ".gpt-cache")
	_ = os.MkdirAll(cachedir, 0777)

	urls := map[SectionKey]string{}
	for _, name := range pageUrls {
		key, err := NewSectionKey(strings.Split(name, "_")[0])
		if err != nil {
			return fmt.Errorf("parsing section key from %s: %w", name, err)
		}
		urls[key] = fmt.Sprintf("http://www.wikiexplora.com/%s", name)
	}
	for _, key := range d.Keys {
		if HAS_SINGLE && key != SINGLE {
			continue
		}
		section := d.Sections[key]
		if err := section.Scrape(cachedir, urls[key]); err != nil {
			return fmt.Errorf("scraping GPT%s: %w", section.Key.Code(), err)
		}
	}
	return nil
}

func (s *Section) Scrape(cachedir, url string) error {
	//if s.Key.Code() != "06" {
	//	return nil
	//}
	var description string
	var reader io.Reader
	cachefpath := filepath.Join(cachedir, fmt.Sprintf("GPT%s.txt", s.Key.Code()))
	f, err := os.Open(cachefpath)
	if err == nil {
		if LOG {
			fmt.Printf("Web scrape data for GPT%s found in cache file %q\n", s.Key.Code(), cachefpath)
		}
		reader = f
		defer f.Close()
	} else {
		if !os.IsNotExist(err) {
			return fmt.Errorf("opening file: %w", err)
		} else {
			if LOG {
				fmt.Printf("Scraping %q for description\n", url)
			}
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
		switch strings.TrimSpace(selection.Text()) {
		case "Recent Alerts and Suggestions", "Season Section Log", "Elevation Profile", "Satellite Image Map", "Summary Table", "Alerts and Logs of Past Seasons", "Older information for review", "Image Gallery":
			var ignoredCount int
			switch selection.Parent().Nodes[0].Data {
			case "h2":
				//ignoring an h2? skip all nodes until we find an h2
				next := selection.Parent().Next()
				for next != nil && len(next.Nodes) > 0 && next.Nodes[0].Data != "h2" {
					ignored[next.Nodes[0]] = true
					next = next.Next()
					ignoredCount++
				}
			case "h3", "h4":
				next := selection.Parent().Next()
				for next != nil && len(next.Nodes) > 0 && next.Nodes[0].Data != "h2" && next.Nodes[0].Data != "h3" && next.Nodes[0].Data != "h4" {
					ignored[next.Nodes[0]] = true
					next = next.Next()
					ignoredCount++
				}
			}
			if ignoredCount > 0 {
				description += "⦿ " + strings.TrimSpace(selection.Text()) + "\n\n"
				description += "☞ Section removed - see web page.\n\n"
			}
			return
		}

		var section []*goquery.Selection
		next := selection.Parent().Next()
		for next != nil && len(next.Nodes) > 0 && next.Nodes[0].Data != "h2" && next.Nodes[0].Data != "h3" && next.Nodes[0].Data != "h4" {
			switch {
			case strings.TrimSpace(next.Text()) == "To be issued.":
			case strings.TrimSpace(next.Text()) == "Not applicable.":
				// nothing
			default:
				section = append(section, next)
			}
			next = next.Next()
		}

		if ignoreSection {
			return
		} else if len(section) > 0 {
			//fmt.Printf("GPT%s %s:", s.Key.Code(), selection.Text())
			description += "⦿ " + strings.TrimSpace(selection.Text()) + "\n\n"
			for _, part := range section {
				//fmt.Print(" ", part.Nodes[0].Data)
				if part.Nodes[0].Data == "table" {
					description += "☞ Table removed - see web page.\n\n"
				} else if len(strings.TrimSpace(part.Text())) > 0 {
					description += strings.TrimSpace(part.Text()) + "\n\n"
				}
			}
			//fmt.Println()
		}
		//fmt.Printf("%d, text: %s, %s\n", i, s.Text(), s.Parent().Next().Nodes[0].Data)
	})

	if len(description) == 0 {
		s.Scraped += fmt.Sprintf("⦿ Full information\n\nUp to date information can be found at:\n\n%s\n\n", url)
	} else {
		s.Scraped += fmt.Sprintf("⦿ Full information\n\nThe following information may be incomplete and out of date. Be sure to check the up to date source:\n\n%s\n\n", url)
	}
	s.Scraped += description

	//fmt.Println(string(b))
	return nil
}

var pageUrls = []string{
	"GPT01_-_Cerro_Purgatorio",
	"GPT02_-_Mina_El_Teniente",
	"GPT03_-_R%C3%ADos_Claros",
	"GPT04_-_Alto_Huemul",
	"GPT05_-_R%C3%ADo_Colorado",
	"GPT06_-_Volc%C3%A1n_Descabezado",
	"GPT07_-_Laguna_Dial",
	"GPT08_-_Volc%C3%A1n_Chillan",
	"GPT09_-_Volc%C3%A1n_Antuco",
	"GPT10_-_Laguna_El_Barco",
	"GPT11_-_Cerro_Moncol",
	"GPT12_-_R%C3%ADo_Rahue",
	"GPT13_-_Laguna_Icalma",
	"GPT14_-_Volc%C3%A1n_Sollipulli",
	"GPT15_-_Curarrehue",
	"GPT16_-_Volc%C3%A1n_Quetrupillan",
	"GPT17H_-_Liqui%C3%B1e",
	"GPT17P_-_Neltume",
	"GPT18_-_Lago_Pirihueico",
	"GPT19_-_Volc%C3%A1n_Puyehue",
	"GPT20_-_Volc%C3%A1n_Antillanca",
	"GPT21_-_Lago_Todos_Los_Santos",
	"GPT22_-_Cocham%C3%B3",
	"GPT23_-_PN_Lago_Puelo",
	"GPT24H_-_PN_Los_Alerces_Tierra",
	"GPT24P_-_PN_Los_Alerces_Agua",
	"GPT25H_-_Aldea_Escolar",
	"GPT25P_-_Lago_Amutui_Quimei",
	"GPT26_-_Carrenleuf%C3%BA",
	"GPT27H_-_Lago_Palena",
	"GPT27P_-_Alto_R%C3%ADo_Palena",
	"GPT28H_-_La_Tapera",
	"GPT28P_-_Bajo_R%C3%ADo_Palena",
	"GPT29H_-_Rio_Cisnes",
	"GPT29P_-_Valle_Picacho",
	"GPT30H_-_Coyhaique",
	"GPT30P_-_Canal_Puyuhuapi",
	"GPT31H_-_Valle_Simpson",
	"GPT31P_-_Lagos_de_Ays%C3%A9n",
	"GPT32_-_Cerro_Castillo",
	"GPT33H_-_Puerto_Iba%C3%B1ez",
	"GPT33P_-_R%C3%ADo_Iba%C3%B1ez",
	"GPT34H_-_Lago_General_Carrera",
	"GPT34P_-_Lago_General_Carrera",
	"GPT35_-_RN_Lago_Jeinimeni",
	"GPT36H_-_Ruta_De_Los_Pioneros",
	"GPT36P_-_R%C3%ADo_Baker",
	"GPT37H_-_Lago_O%27Higgins",
	"GPT37P_-_Pen%C3%ADnsula_La_Florida",
	"GPT38_-_Glaciar_Chico",
	"GPT39_-_Monte_Fitz_Roy",
	"GPT40_-_Glaciar_Viedma",
	"GPT70_-_Alto_R%C3%ADo_Futaleuf%C3%BA",
	"GPT71_-_Espol%C3%B3n",
	"GPT72_-_Bajo_R%C3%ADo_Futaleuf%C3%BA",
	"GPT73P_-_Lago_Yelcho",
	"GPT74P_-_R%C3%ADo_Yelcho",
	"GPT75P_-_R%C3%ADo_Fr%C3%ADo",
	"GPT76_-_PN_Pumalin_Sur",
	"GPT77_-_PN_Pumalin_Norte",
	"GPT78_-_Estuario_de_Reloncav%C3%AD",
	"GPT80P_-_Valle_Exploradores",
	"GPT81P_-_Traves%C3%ADa_Leones-Soler",
	"GPT82P_-_Traves%C3%ADa_Soler-Nef",
	"GPT83P_-_Traves%C3%ADa_Nef-Colonia",
	"GPT90P_-_Volc%C3%A1n_Hudson",
	"GPT91P_-_Istmo_de_Ofqui",
	"GPT92P_-_Glacier_Steffens",
}
