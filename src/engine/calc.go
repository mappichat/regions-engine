package engine

import (
	"container/heap"
	"errors"
	"fmt"
	"log"
	"math"
	"path"
	"runtime"
	"sort"
	"sync"

	"github.com/mappichat/regions-engine/src/project_types"
	"github.com/mappichat/regions-engine/src/utils"
	h3 "github.com/uber/h3-go/v3"
)

func GenerateLevel0(pop_map project_types.PopMap, tiles []string) (project_types.Level, error) {
	level := make(map[string]project_types.Region, len(tiles))
	tileSet := make(map[string]bool, len(tiles))
	for j := range tiles {
		tileSet[tiles[j]] = true
	}

	i := 0
	for _, tile := range tiles {
		pop, ok := pop_map[tile]
		if !ok {
			return nil, errors.New(tile + " not found in pop map")
		}

		newRegion := project_types.Region{
			Index:      tile,
			Population: pop,
			Tiles:      []string{tile},
			Neighbors:  map[string]bool{},
			Centroid:   h3.ToGeo(h3.FromString(tile)),
		}
		for _, k := range h3.KRing(h3.FromString(tile), 1) {
			h3String := h3.ToString(k)
			if h3String != tile {
				newRegion.Neighbors[h3String] = true
			}
		}
		level[tile] = newRegion
		i++
		if i%1000000 == 0 {
			log.Print(i)
		}
	}

	// give regions with zero neighbors a neighbor
	sorted := []string{}
	for tile := range level {
		sorted = append(sorted, tile)
	}
	sort.Strings(sorted)
	for tile := range level {
		if len(level[tile].Neighbors) == 0 {
			index := sort.SearchStrings(sorted, tile)
			neighbor := sorted[(index+1)%len(sorted)]
			level[tile].Neighbors[neighbor] = true
			level[neighbor].Neighbors[tile] = true
		}
	}

	return level, nil
}

func calcCentroid(r *project_types.Region) h3.GeoCoord {
	latSum := 0.0
	lonSum := 0.0
	size := len(r.Tiles)
	for _, tile := range r.Tiles {
		latSum += h3.ToGeo(h3.FromString(tile)).Latitude
		lonSum += h3.ToGeo(h3.FromString(tile)).Longitude
	}
	return h3.GeoCoord{Latitude: latSum / float64(size), Longitude: lonSum / float64(size)}
}

func mergeRegions(level map[string]project_types.Region, parents map[string]string, into string, mergee string) {
	intoRegion := level[into]

	intoRegion.Population += level[mergee].Population

	for _, tile := range level[mergee].Tiles {
		parents[tile] = intoRegion.Index
	}
	intoRegion.Tiles = append(intoRegion.Tiles, level[mergee].Tiles...)

	for neighbor := range level[mergee].Neighbors {
		intoRegion.Neighbors[neighbor] = true
		delete(level[neighbor].Neighbors, mergee)
		level[neighbor].Neighbors[into] = true
	}
	delete(intoRegion.Neighbors, mergee)
	delete(intoRegion.Neighbors, into)

	intoRegion.Centroid = calcCentroid(&intoRegion)

	delete(level, mergee)
	level[into] = intoRegion
}

func GenerateLevel(prevLevel map[string]project_types.Region, options *project_types.LevelOptions) (map[string]project_types.Region, map[string]string) {
	// initializations
	queue := &project_types.LevelQueue{Length: 0, Regions: []project_types.Region{}}
	for _, region := range prevLevel {
		heap.Push(queue, region)
	}
	parents := map[string]string{}
	level := map[string]project_types.Region{}

	// main loop
	for queue.Length > 0 {
		next := heap.Pop(queue).(project_types.Region)
		if _, in := parents[next.Index]; in {
			continue
		}
		locQueue := &project_types.LevelQueue{Length: 0, Regions: []project_types.Region{}}
		locQueue.Push(next)
		region := project_types.Region{
			Index:      next.Index,
			Population: 0,
			Tiles:      []string{},
			Neighbors:  map[string]bool{},
			Centroid:   h3.GeoCoord{Latitude: 0, Longitude: 0},
		}

		// <- centroid this prevents readding on already seen tiles ->
		latSum := 0.0
		lonSum := 0.0
		// <- ->

		for locQueue.Length > 0 {
			currentRegion := heap.Pop(locQueue).(project_types.Region)

			// check against constraints
			if _, in := parents[currentRegion.Index]; in {
				continue
			}
			if len(region.Tiles) > 0 { // constraints only apply if region is non empty
				if prevLevel[currentRegion.Index].Population+region.Population > options.MaxPop {
					continue
				}
				if len(currentRegion.Tiles)+len(region.Tiles) > options.MaxRegionSize {
					continue
				}
			}

			// Add to parent region
			region.Tiles = append(region.Tiles, currentRegion.Tiles...)
			for _, tile := range currentRegion.Tiles {
				parents[tile] = region.Index
				geo := h3.ToGeo(h3.FromString(tile))
				latSum += geo.Latitude
				lonSum += geo.Longitude
			}
			region.Centroid = h3.GeoCoord{
				Latitude:  latSum / float64(len(region.Tiles)),
				Longitude: lonSum / float64(len(region.Tiles)),
			}
			region.Population += prevLevel[currentRegion.Index].Population

			for neighbor := range currentRegion.Neighbors {
				parent, in := parents[neighbor]
				if !in {
					// // check size to make sure it doesn't break constraint
					if len(prevLevel[neighbor].Tiles)+len(region.Tiles) > options.MaxRegionSize {
						continue
					}

					// check population to make sure it doesn't break constraint
					if prevLevel[neighbor].Population+region.Population > options.MaxPop {
						continue
					}

					// <- centroid mult ->
					latDiff := prevLevel[neighbor].Centroid.Latitude - region.Centroid.Latitude
					lonDiff := prevLevel[neighbor].Centroid.Longitude - region.Centroid.Longitude
					dist := math.Sqrt((latDiff * latDiff) + (lonDiff * lonDiff))
					// <- ->

					weightedPop := prevLevel[neighbor].Population
					if weightedPop == 0 {
						weightedPop = 1.0
					}
					weightedPop *= math.Pow(dist, options.DistanceExponent)

					heap.Push(locQueue, project_types.Region{
						Index:      neighbor,
						Population: weightedPop,
						Tiles:      prevLevel[neighbor].Tiles,
						Neighbors:  prevLevel[neighbor].Neighbors,
						Centroid:   prevLevel[neighbor].Centroid,
					})
				} else if parent != region.Index {
					region.Neighbors[parent] = true
					level[parent].Neighbors[region.Index] = true
				}
			}
		}
		level[region.Index] = region
	}

	// remove islands
	if len(level) == 1 { // entire level merged; return
		return level, parents
	}
	for j := 0; j < options.IslandDampeningPasses; j++ { // number of passes
		for k := range level {
			if len(level[k].Neighbors) == 1 {
				for n := range level[k].Neighbors { // will only run once
					mergeRegions(level, parents, n, k)
					break
				}
			}
		}
	}

	// merge small regions
	if len(level) == 1 { // entire level merged; return
		return level, parents
	}
	for k, region := range level {
		if len(region.Tiles) <= options.SmallRegionMergeLimit && len(region.Neighbors) > 0 {
			var smallestNeighbor project_types.Region
			size := 569707381193163 // No region can have this many tiles
			for n := range region.Neighbors {
				if len(level[n].Tiles) < size {
					smallestNeighbor = level[n]
					size = len(smallestNeighbor.Tiles)
				}
			}
			mergeRegions(level, parents, smallestNeighbor.Index, k)
		}
	}

	// give regions with zero neighbors a neighbor
	if len(level) == 1 { // entire level merged; return
		return level, parents
	}
	sorted := []string{}
	for tile := range level {
		sorted = append(sorted, tile)
	}
	sort.Strings(sorted)
	for tile := range level {
		if len(level[tile].Neighbors) == 0 {
			index := sort.SearchStrings(sorted, tile)
			neighbor := sorted[(index+1)%len(sorted)]
			level[tile].Neighbors[neighbor] = true
			level[neighbor].Neighbors[tile] = true
		}
	}

	return level, parents
}

func GenerateAndWriteLevels(popMap project_types.PopMap, countryToH3 project_types.CountryToH3, dirName string, resolution int, options []project_types.LevelOptions) error {
	log.Print("calculating country centroids")
	// get country neighbors
	countryCentroids := map[string]h3.GeoCoord{}
	for country, tiles := range countryToH3 {
		countryCentroids[country] = CountryCentroid(tiles)
	}

	// concurrency stuff
	processes := runtime.GOMAXPROCS(runtime.NumCPU())
	log.Printf("max processes running: %d\n", processes)
	wg := sync.WaitGroup{}
	guard := make(chan struct{}, processes)
	mutex := sync.Mutex{}
	errs := []error{}

	log.Print("generating country level0's")
	zeroLevels := map[string]project_types.Level{}
	for country := range countryToH3 {
		wg.Add(1)
		guard <- struct{}{}
		go func(country string) {
			next, err := GenerateLevel0(popMap, countryToH3[country])
			mutex.Lock()
			errs = append(errs, err)
			zeroLevels[country] = next
			mutex.Unlock()
			wg.Done()
			<-guard
		}(country)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	errs = []error{}

	log.Print("generating country levels")
	countryLevels := make([]map[string]project_types.Level, len(options))
	countryParents := make([]map[string]map[string]string, len(options))
	for i := 0; i < len(options); i++ {
		countryLevels[i] = map[string]project_types.Level{}
		countryParents[i] = map[string]map[string]string{}
		rangeObj := zeroLevels
		if i > 0 {
			rangeObj = countryLevels[i-1]
		}
		for country := range rangeObj {
			var prevLevel project_types.Level
			if i == 0 {
				prevLevel = zeroLevels[country]
			} else {
				prevLevel = countryLevels[i-1][country]
			}
			wg.Add(1)
			guard <- struct{}{}
			go func(country string, prevLevel project_types.Level) {
				nextLevel, nextParents := GenerateLevel(prevLevel, &options[i])

				mutex.Lock()
				countryLevels[i][country] = nextLevel
				countryParents[i][country] = nextParents
				mutex.Unlock()

				log.Print(country)
				wg.Done()
				<-guard
			}(country, prevLevel)
		}
		wg.Wait()

		// merge finished countries
		for country := range countryLevels[i] {
			if len(countryLevels[i][country]) == 1 {
				for _, region := range countryLevels[i][country] { // only run once
					// find nearest neighbor
					neighbor := ""
					minDist := math.MaxFloat64
					for curr, centroid := range countryCentroids {
						calcDist := utils.Distance(countryCentroids[country].Latitude, countryCentroids[country].Longitude, centroid.Latitude, centroid.Longitude)
						if calcDist < minDist && curr != country && len(countryLevels[i][curr]) != 0 {
							minDist = calcDist
							neighbor = curr
						}
					}
					countryLevels[i][neighbor][region.Index] = region
					for child := range countryParents[i][country] {
						countryParents[i][neighbor][child] = region.Index
					}
					for _, tile := range utils.H3BorderTiles(region.Tiles) {
						if newNeighbor, ok := countryParents[i][neighbor][tile]; ok {
							countryLevels[i][neighbor][region.Index].Neighbors[newNeighbor] = true
							countryLevels[i][neighbor][newNeighbor].Neighbors[region.Index] = true
						}
					}
					break
				}
				delete(countryLevels[i], country)
				delete(countryParents[i], country)
			}
		}
	}

	// log.Print("generating country levels")
	// countryLevels := map[string][]project_types.Level{}
	// countryParents := map[string][]map[string]string{}
	// for country := range zeroLevels {
	// 	wg.Add(1)
	// 	guard <- struct{}{}
	// 	go func(country string) {
	// 		prevLevel := zeroLevels[country]
	// 		for i := 0; i < len(options); i++ {
	// 			var nextLevel project_types.Level
	// 			var nextParents map[string]string
	// 			nextLevel, nextParents = GenerateLevel(prevLevel, &options[i])

	// 			mutex.Lock()
	// 			countryLevels[country] = append(countryLevels[country], nextLevel)
	// 			countryParents[country] = append(countryParents[country], nextParents)
	// 			mutex.Unlock()

	// 			prevLevel = nextLevel
	// 		}
	// 		log.Print(country)
	// 		wg.Done()
	// 		<-guard
	// 	}(country)
	// }
	// wg.Wait()

	log.Print("stitching global levels")
	count := 0
	for i := 0; i < len(options); i++ {
		wg.Add(1)
		guard <- struct{}{}
		go func(j int) {
			level := project_types.Level{}
			parents := map[string]string{}
			for country := range countryLevels[j] {
				for tileIndex, region := range countryLevels[j][country] {
					level[tileIndex] = region
				}
				for child, parent := range countryParents[j][country] {
					parents[child] = parent
				}
				count++
				if count%100 == 0 {
					log.Print(count)
				}
			}
			utils.WriteAsJsonFile(level, path.Join(dirName, fmt.Sprintf("level%d.json", j)))
			utils.WriteAsJsonFile(parents, path.Join(dirName, fmt.Sprintf("parents%d.json", j)))
			log.Print("total regions and size of parents:")
			log.Print(len(level), len(parents))
			log.Print("total tiles and population:")
			log.Print(project_types.LevelTotalTiles(level), project_types.LevelTotalPop(level))

			wg.Done()
			<-guard
		}(i)
	}
	wg.Wait()

	return nil
}
