package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/iancoleman/strcase"
	"github.com/mappichat/regions-engine/src/project_types"
	"github.com/mitchellh/mapstructure"
	h3 "github.com/uber/h3-go/v3"
)

func DecodeSnakeCase(input interface{}) (map[string]interface{}, error) {
	output := map[string]interface{}{}
	if err := mapstructure.Decode(input, &output); err != nil {
		return nil, err
	}
	newOut := map[string]interface{}{}
	for k, v := range output {
		newOut[strcase.ToSnake(k)] = v
	}
	return newOut, nil
}

func WriteAsJsonFile(v interface{}, filePath string) error {
	base := path.Base(filePath)
	dirPath := filePath[:len(filePath)-len(base)]
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return err
	}

	log.Print("marshalling json")
	bytes, err := json.Marshal(v)
	if err != nil {
		return err
	}

	log.Print("writing json")
	if err = os.WriteFile(filePath, bytes, 0644); err != nil {
		return err
	}

	return nil
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func ReadJsonFile(filePath string, dest interface{}) error {
	var bytes = []byte{}
	var err error
	if FileExists(filePath) {
		bytes, err = os.ReadFile(path.Join(filePath))
		if err != nil {
			return err
		}
	} else {
		url, err := url.ParseRequestURI(filePath)
		if err != nil {
			return err
		}
		resp, err := http.Get(url.String())
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected http GET status: %s", resp.Status)
		}
		bytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
	}

	if err = json.Unmarshal(bytes, &dest); err != nil {
		return err
	}
	return nil
}

func H3Slice(resolution int) []string {
	size, ok := project_types.ResolutionSizes[resolution]
	if !ok {
		panic(errors.New("invalid resolution"))
	}

	slice := make([]string, size)
	i := 0
	for h := range H3Tiles(resolution) {
		slice[i] = h
		i++
	}
	return slice
}

func H3Tiles(resolution int) chan string {
	size, ok := project_types.ResolutionSizes[resolution]
	if !ok {
		panic(errors.New("invalid resolution"))
	}

	ch := make(chan string)

	start := h3.ToString(h3.FromGeo(h3.GeoCoord{
		Latitude:  0,
		Longitude: 0,
	}, int(resolution)))

	seen := make(map[string]bool, size)
	seen[start] = true
	stack := project_types.NewStaticStack[string](size)
	if err := stack.Push(start); err != nil {
		panic(err)
	}

	go func() {
		for stack.Length > 0 {
			h, err := stack.Pop()
			if err != nil {
				continue
			}
			ch <- *h
			for _, tile := range h3.KRing(h3.FromString(*h), 1) {
				tileString := h3.ToString(tile)

				if _, in := seen[tileString]; !in {
					err = stack.Push(tileString)
					if err != nil {
						panic(err)
					}
					seen[tileString] = true
				}
			}
		}
		close(ch)
	}()
	return ch
}

func H3BorderTiles(tiles []string) []string {
	border := []string{}
	seen := map[string]bool{}
	for _, tile := range tiles {
		seen[tile] = true
	}
	for _, tile := range tiles {
		for _, h := range h3.KRing(h3.FromString(tile), 1) {
			curr := h3.ToString(h)
			if _, ok := seen[curr]; !ok {
				border = append(border, curr)
				continue
			}
		}
	}
	return border
}

func EmptyPopMap(resolution int) project_types.PopMap {
	size, ok := project_types.ResolutionSizes[resolution]
	if !ok {
		panic(errors.New("invalid resolution"))
	}

	i := 0
	popMap := make(project_types.PopMap, size)

	for h := range H3Tiles(resolution) {
		popMap[h] = 0

		i++
		if i%1000000 == 0 {
			log.Print(i)
		}
	}
	return popMap
}

func Distance(lat1 float64, lng1 float64, lat2 float64, lng2 float64, unit ...string) float64 {
	radlat1 := float64(math.Pi * lat1 / 180)
	radlat2 := float64(math.Pi * lat2 / 180)

	theta := float64(lng1 - lng2)
	radtheta := float64(math.Pi * theta / 180)

	dist := math.Sin(radlat1)*math.Sin(radlat2) + math.Cos(radlat1)*math.Cos(radlat2)*math.Cos(radtheta)
	if dist > 1 {
		dist = 1
	}

	dist = math.Acos(dist)
	dist = dist * 180 / math.Pi
	dist = dist * 60 * 1.1515

	if len(unit) > 0 {
		if unit[0] == "K" {
			dist = dist * 1.609344
		} else if unit[0] == "N" {
			dist = dist * 0.8684
		}
	}

	return dist
}
