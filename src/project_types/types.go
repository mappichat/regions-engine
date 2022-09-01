package project_types

import (
	"errors"
	"sort"

	h3 "github.com/uber/h3-go/v3"
)

type Region struct {
	Index      string          `json:"index"`
	Population float64         `json:"population"`
	Tiles      []string        `json:"tiles"`
	Neighbors  map[string]bool `json:"neighbors"`
	Centroid   h3.GeoCoord     `json:"centroid"`
}

// assumes regions are presorted by h3 index
func NearestRegion(r *Region, h3SortedRegions []Region) *Region {
	index := sort.Search(len(h3SortedRegions), func(i int) bool {
		return h3.FromString(r.Index) > h3.FromString(h3SortedRegions[i].Index)
	})
	return &h3SortedRegions[index]
}

type PopMap map[string]float64

type LevelOptions struct {
	MaxRegionSize         int     `json:"maxRegionSize"`
	MaxPop                float64 `json:"maxPopulation"`
	DistanceExponent      float64 `json:"distanceExponent"`
	IslandDampeningPasses int     `json:"islandDampeningPasses"`
	SmallRegionMergeLimit int     `json:"smallRegionMergeLimit"`
}

type EngineOptions []LevelOptions

type LevelQueue struct {
	Length  int
	Regions []Region
}

func (q *LevelQueue) Len() int {
	return q.Length
}

func (q *LevelQueue) Less(i, j int) bool {
	return q.Regions[i].Population > q.Regions[j].Population
}

func (q *LevelQueue) Swap(i, j int) {
	buffer := q.Regions[i]
	q.Regions[i] = q.Regions[j]
	q.Regions[j] = buffer
}

func (q *LevelQueue) Push(x any) {
	q.Regions = append(q.Regions, x.(Region))
	q.Length++
}

func (q *LevelQueue) Pop() any {
	reg := q.Regions[q.Length-1]
	q.Regions = q.Regions[:q.Length-1]
	q.Length--
	return reg
}

var ResolutionSizes map[int]int = map[int]int{
	0:  122,
	1:  842,
	2:  5882,
	3:  41162,
	4:  288122,
	5:  2016842,
	6:  14117882,
	7:  98825162,
	8:  691776122,
	9:  4842432842,
	10: 33897029882,
	11: 237279209162,
	12: 1660954464122,
	13: 11626681248842,
	14: 81386768741882,
	15: 569707381193162,
}

type StaticStack[T any] struct {
	Length int
	Slice  []T
	Size   int
}

func (s *StaticStack[T]) Push(v T) error {
	if s.Length >= s.Size {
		return errors.New("stack overflow")
	}
	s.Slice[s.Length] = v
	s.Length++
	return nil
}

func (s *StaticStack[T]) Pop() (*T, error) {
	if s.Length <= 0 {
		return nil, errors.New("stack underflow")
	}
	s.Length--
	return &s.Slice[s.Length], nil
}

func NewStaticStack[T any](size int) *StaticStack[T] {
	return &StaticStack[T]{
		Length: 0,
		Slice:  make([]T, size),
		Size:   size,
	}
}

type GeoJson struct {
	Features []struct {
		Properties struct {
			Name string `json:"ADMIN"`
		} `json:"properties"`
		Geometry struct {
			Type        string            `json:"type"`
			Coordinates [][][]interface{} `json:"coordinates"`
		} `json:"geometry"`
	} `json:"features"`
}

type CountryPolygons map[string][]h3.GeoPolygon
type H3ToCountry map[string]string
type CountryToH3 map[string][]string

type Level map[string]Region

func LevelTotalTiles(level Level) int {
	sum := 0
	for _, region := range level {
		sum += len(region.Tiles)
	}
	return sum
}

func LevelTotalPop(level Level) float64 {
	sum := 0.0
	for _, region := range level {
		sum += region.Population
	}
	return sum
}
