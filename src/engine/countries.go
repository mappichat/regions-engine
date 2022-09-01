package engine

import (
	"log"

	"github.com/mappichat/regions-engine/src/project_types"
	"github.com/mappichat/regions-engine/src/utils"
	"github.com/uber/h3-go/v3"
)

func GenerateCountryMaps(countryPolygons project_types.CountryPolygons, resolution int, coastFill int) (project_types.H3ToCountry, project_types.CountryToH3) {
	h3ToCountry := project_types.H3ToCountry{}
	countryToH3 := project_types.CountryToH3{}
	log.Print("assigning tiles to countries")
	for country, polygons := range countryPolygons {
		tiles := []string{}
		for _, polygon := range polygons {
			for _, h := range h3.Polyfill(polygon, resolution) {
				tile := h3.ToString(h)
				tiles = append(tiles, tile)
				h3ToCountry[tile] = country
			}
		}
		countryToH3[country] = tiles
		log.Print(country)
	}

	// give countries of size 0 some tiles
	log.Print("giving zero tile countries some tiles")
	for country, tiles := range countryToH3 {
		if len(tiles) == 0 {
			for _, polygon := range countryPolygons[country] {
				for _, coord := range polygon.Geofence {
					tile := h3.ToString(h3.FromGeo(coord, resolution))
					if _, ok := h3ToCountry[tile]; !ok {
						h3ToCountry[tile] = country
						countryToH3[country] = append(countryToH3[country], tile)
					}
				}
			}
			// log.Printf("%s size: %d\n", country, len(countryToH3[country]))
		}
	}

	// Assign coastline and unclaimed tiles to countries
	log.Print("assigning coast and unnassigned land near coast")
	for country, tiles := range countryToH3 {
		for _, tile := range utils.H3BorderTiles(tiles) {
			for _, h := range h3.KRing(h3.FromString(tile), coastFill) {
				neighbor := h3.ToString(h)
				if _, ok := h3ToCountry[neighbor]; !ok {
					h3ToCountry[neighbor] = country
					countryToH3[country] = append(countryToH3[country], neighbor)
				}
			}
		}
		// log.Printf("%s size: %d\n", country, len(tiles))
	}

	return h3ToCountry, countryToH3
}

func CountryCentroid(tiles []string) h3.GeoCoord {
	latsum := 0.0
	lonsum := 0.0
	for i := range tiles {
		h := h3.ToGeo(h3.FromString(tiles[i]))
		latsum += h.Latitude
		lonsum += h.Longitude
	}
	n := len(tiles)
	return h3.GeoCoord{Latitude: latsum / float64(n), Longitude: lonsum / float64(n)}
}
