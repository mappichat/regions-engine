package fileio

import (
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/mappichat/regions-engine/src/project_types"
	"github.com/mappichat/regions-engine/src/utils"
	h3 "github.com/uber/h3-go/v3"
)

func LoadOptions(filePath string) (project_types.EngineOptions, error) {
	options := project_types.EngineOptions{}
	if err := utils.ReadJsonFile(filePath, &options); err != nil {
		return nil, err
	}
	return options, nil
}

func GenerateTestPopMap(resolution int) map[string]int {
	size, ok := project_types.ResolutionSizes[resolution]
	if !ok {
		panic(errors.New("invalid resolution"))
	}

	i := 0
	popMap := make(map[string]int, size)

	for h := range utils.H3Tiles(resolution) {
		if rand.Intn(10) == 0 {
			popMap[h] = int(math.Round(rand.ExpFloat64() * 100))
		} else {
			popMap[h] = 0
		}

		i++
		if i%1000000 == 0 {
			log.Print(i)
		}
	}
	return popMap
}

func polygonToH3Polygon(polygon [][][]float64) (h3.GeoPolygon, error) {
	if len(polygon) == 0 {
		return h3.GeoPolygon{}, errors.New("polygon empty")
	}
	geoFence := []h3.GeoCoord{}
	for _, coord := range polygon[0] {
		geoFence = append(geoFence, h3.GeoCoord{
			Latitude:  coord[0],
			Longitude: coord[1],
		})
	}
	holes := [][]h3.GeoCoord{}
	for _, shape := range polygon[1:] {
		hole := []h3.GeoCoord{}
		for _, coord := range shape {
			hole = append(hole, h3.GeoCoord{
				Latitude:  coord[0],
				Longitude: coord[1],
			})
		}
		holes = append(holes, hole)
	}
	return h3.GeoPolygon{Geofence: geoFence, Holes: holes}, nil
}

// filePath should point to a geojson file
func ReadCountriesFile(filePath string) (project_types.CountryPolygons, error) {
	geojson := project_types.GeoJson{}
	if err := utils.ReadJsonFile(filePath, &geojson); err != nil {
		return nil, err
	}

	countries := project_types.CountryPolygons{}
	for _, feature := range geojson.Features {
		newCountry := []h3.GeoPolygon{}
		geoType := strings.ToLower(feature.Geometry.Type)
		if geoType == "polygon" {
			polygon := [][][]float64{}
			for _, v1 := range feature.Geometry.Coordinates {
				a1 := [][]float64{}
				for _, v2 := range v1 {
					a2 := []float64{}
					for _, v3 := range v2 {
						a3 := v3.(float64)
						a2 = append(a2, a3)
					}
					// need to flip coordinates because geojson does lng,lat instead of lat,lng
					lng := a2[0]
					a2[0] = a2[1]
					a2[1] = lng
					a1 = append(a1, a2)
				}
				polygon = append(polygon, a1)
			}
			geoPolygon, err := polygonToH3Polygon(polygon)
			if err != nil {
				return nil, err
			}
			newCountry = append(newCountry, geoPolygon)
		} else if geoType == "multipolygon" {
			multipolygon := [][][][]float64{}
			for _, v1 := range feature.Geometry.Coordinates {
				a1 := [][][]float64{}
				for _, v2 := range v1 {
					a2 := [][]float64{}
					for _, v3 := range v2 {
						// need to flip coordinates because geojson does lng,lat instead of lat,lng
						s := reflect.ValueOf(v3)
						a3 := []float64{s.Index(1).Interface().(float64), s.Index(0).Interface().(float64)}
						a2 = append(a2, a3)
					}
					a1 = append(a1, a2)
				}
				multipolygon = append(multipolygon, a1)
			}
			for _, polygon := range multipolygon {
				geoPolygon, err := polygonToH3Polygon(polygon)
				if err != nil {
					return nil, err
				}
				newCountry = append(newCountry, geoPolygon)
			}
		} else {
			return nil, fmt.Errorf("unsupported geometry type %s", geoType)
		}
		countries[feature.Properties.Name] = newCountry
	}
	return countries, nil
}

func LoadPopMapJson(filePath string, resolution int) (project_types.PopMap, error) {
	popmap := project_types.PopMap{}
	if err := utils.ReadJsonFile(filePath, &popmap); err != nil {
		return nil, err
	}

	i := 0
	for h := range utils.H3Tiles(resolution) {
		if _, ok := popmap[h]; !ok {
			popmap[h] = 0
		}

		i++
		if i%1000000 == 0 {
			log.Print(i)
		}
	}
	return popmap, nil
}

func PopMapStats(popmap project_types.PopMap) (float64, float64) {
	size := len(popmap)
	mean := 0.0
	for _, pop := range popmap {
		mean += pop
	}
	mean = float64(mean) / float64(size)
	stddev := 0.0
	for _, pop := range popmap {
		diff := pop - mean
		stddev += (diff * diff)
	}
	stddev = math.Sqrt(stddev / float64(size))
	return mean, stddev
}

func ReadLevel(filePath string) (map[string]project_types.Region, error) {
	level := project_types.Level{}
	if err := utils.ReadJsonFile(filePath, &level); err != nil {
		return nil, err
	}
	return level, nil
}

func ReadLevels(dirPath string) ([]map[string]project_types.Region, []map[string]string) {
	matches, err := filepath.Glob(path.Join(dirPath, "level*.json"))
	if err != nil {
		panic(err)
	}
	number := len(matches)
	log.Printf("%d levels found\n", number)
	levels := make([]map[string]project_types.Region, number)
	parents := make([]map[string]string, number+1)
	wg := sync.WaitGroup{}
	for i := 0; i < number; i++ {
		wg.Add(2)
		go func(i int) {
			err := utils.ReadJsonFile(path.Join(dirPath, fmt.Sprintf("level%d.json", i)), &levels[i])
			if err != nil {
				panic(err)
			}
			wg.Done()
		}(i)
		go func(i int) {
			err := utils.ReadJsonFile(path.Join(dirPath, fmt.Sprintf("parents%d.json", i)), &parents[i])
			if err != nil {
				panic(err)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	return levels, parents
}

func WriteCountryMaps(countryPolygons project_types.CountryPolygons, countryToH3 project_types.CountryToH3, h3ToCountry project_types.H3ToCountry, dirName string) error {
	wg := sync.WaitGroup{}
	wg.Add(3)
	errs := [3]error{}
	go func() {
		errs[0] = utils.WriteAsJsonFile(countryPolygons, path.Join(dirName, "countryPolygons.json"))
		wg.Done()
	}()
	go func() {
		errs[1] = utils.WriteAsJsonFile(countryToH3, path.Join(dirName, "countryToH3.json"))
		wg.Done()
	}()
	go func() {
		errs[2] = utils.WriteAsJsonFile(h3ToCountry, path.Join(dirName, "h3ToCountry.json"))
		wg.Done()
	}()
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func ReadH3ToCountry(filePath string) (project_types.H3ToCountry, error) {
	h3ToCountry := project_types.H3ToCountry{}
	if err := utils.ReadJsonFile(filePath, &h3ToCountry); err != nil {
		return nil, err
	}
	return h3ToCountry, nil
}

func ReadCountryMaps(dirName string) (project_types.CountryPolygons, project_types.CountryToH3, project_types.H3ToCountry, error) {
	wg := sync.WaitGroup{}
	wg.Add(3)
	errs := [3]error{}
	countryPolygons := project_types.CountryPolygons{}
	go func() {
		errs[0] = utils.ReadJsonFile(dirName+"/countryPolygons.json", &countryPolygons)
		wg.Done()
	}()
	countryToH3 := project_types.CountryToH3{}
	go func() {
		errs[1] = utils.ReadJsonFile(dirName+"/countryToH3.json", &countryToH3)
		wg.Done()
	}()
	h3ToCountry := project_types.H3ToCountry{}
	go func() {
		errs[2] = utils.ReadJsonFile(dirName+"/h3ToCountry.json", &h3ToCountry)
		wg.Done()
	}()
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return countryPolygons, countryToH3, h3ToCountry, nil
}
