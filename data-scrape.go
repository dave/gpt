package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func (d *Data) Scrape() error {
	urls := map[SectionKey]string{}
	for _, name := range pageUrls {
		key, err := NewSectionKey(strings.Split(name, "_")[0])
		if err != nil {
			return fmt.Errorf("parsing section key from %s: %w", name, err)
		}
		urls[key] = fmt.Sprintf("http://www.wikiexplora.com/%s", name)
	}
	for _, key := range d.Keys {
		section := d.Sections[key]

		if LOG {
			fmt.Printf("Scraping %q for description\n", urls[key])
		}

		resp, err := http.Get(urls[key])
		if err != nil {
			return fmt.Errorf("getting %q: %w", urls[key], err)
		}

		dom, err := goquery.NewDocumentFromReader(resp.Body)
		if err != nil {
			return fmt.Errorf("reading %s: %w", urls[key], err)
		}
		dom.Find(".mw-headline").Each(func(i int, s *goquery.Selection) {

			next := s.Parent().Next()
			if next != nil && len(next.Nodes) > 0 && next.Nodes[0].Data == "p" {
				switch {
				case strings.TrimSpace(s.Text()) == "Elevation Profile":
				case strings.TrimSpace(next.Text()) == "To be issued.":
					// nothing
				default:
					section.Scraped += s.Text() + "\n"
					section.Scraped += next.Text() + "\n"
				}

			}
			//fmt.Printf("%d, text: %s, %s\n", i, s.Text(), s.Parent().Next().Nodes[0].Data)
		})

		//fmt.Println(string(b))
	}
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
