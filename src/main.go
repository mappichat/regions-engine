package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mappichat/regions-engine/src/database"
	"github.com/mappichat/regions-engine/src/engine"
	"github.com/mappichat/regions-engine/src/fileio"
	"github.com/mappichat/regions-engine/src/project_types"
	"github.com/mappichat/regions-engine/src/server"
	"github.com/mappichat/regions-engine/src/utils"
)

func main() {
	startTime := time.Now()

	var err error
	// utils.ConfigureEnv()
	if len(os.Args) < 2 {
		log.Fatal("run using one of these subcommands: generate, serve, dbwrite")
	}

	var countryPolygons project_types.CountryPolygons
	var countryToH3 project_types.CountryToH3
	var h3ToCountry project_types.H3ToCountry

	switch os.Args[1] {
	case "generate":
		if len(os.Args) < 3 {
			log.Fatal("generate subcommand has one argument: [countries-geojson-path]")
		}
		countriesPath := os.Args[2]

		cmd := flag.NewFlagSet("generate", flag.ExitOnError)
		var resolution int
		var popMapPath string
		var configPath string
		var outDir string
		var memsafeStitching bool
		cmd.IntVar(&resolution, "r", 5, "h3 resolution used to generate regions")
		cmd.StringVar(&popMapPath, "p", "", "path to popmap file (json)")
		cmd.StringVar(&configPath, "c", "", "path to engine config file (json)")
		cmd.StringVar(&outDir, "o", "", "data output directory")
		cmd.BoolVar(&memsafeStitching, "m", false, "Stitch country level data together one level at a time instead of concurrently. This can prevent crashes from using too much memory at higher resolutions. (Typically >= 7)")
		cmd.Parse(os.Args[3:])

		if outDir == "" {
			outDir = fmt.Sprintf("./resolution%d-data/", resolution)
		}

		log.Print("loading countries geojson data")
		countryPolygons, err = fileio.ReadCountriesFile(countriesPath)
		if err != nil {
			log.Fatal(err)
		}
		log.Print("generating country maps")
		h3ToCountry, countryToH3 = engine.GenerateCountryMaps(countryPolygons, resolution, 1)

		log.Print("writing country maps to json")
		if err = fileio.WriteCountryMaps(countryPolygons, countryToH3, h3ToCountry, outDir); err != nil {
			log.Fatal(err)
		}

		log.Print("loading popmap")
		var popMap project_types.PopMap
		if popMapPath == "" {
			popMap = utils.EmptyPopMap(resolution)
		} else {
			popMap, err = fileio.LoadPopMapJson(popMapPath, resolution)
			if err != nil {
				log.Fatal(err)
			}
		}

		log.Print("calculating popmap stats")
		mean, std := fileio.PopMapStats(popMap)
		log.Printf("popmap mean: %f, standard deviation: %f\n", mean, std)

		var options project_types.EngineOptions
		if configPath != "" {
			options, err = fileio.LoadOptions(configPath)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			if _, ok := utils.DefaultOptions[resolution]; !ok {
				log.Fatal(errors.New("if resolution isn't [5-7] you must specify your own config file with -c"))
			} else {
				options = utils.DefaultOptions[resolution]
			}
		}

		log.Print("generating levels")
		err = engine.GenerateAndWriteLevels(popMap, countryToH3, outDir, resolution, memsafeStitching, options)
		if err != nil {
			log.Fatal(err)
		}

		log.Print(time.Since(startTime))
	case "serve":
		if len(os.Args) < 3 {
			log.Fatal("serve subcommand has one argument: [data-directory]")
		}
		dataDir := os.Args[2]

		cmd := flag.NewFlagSet("serve", flag.ExitOnError)
		var port int
		cmd.IntVar(&port, "p", 8080, "serving port")
		cmd.Parse(os.Args[3:])

		log.Print("reading country maps from json")
		countryPolygons, countryToH3, h3ToCountry, err = fileio.ReadCountryMaps(dataDir)
		if err != nil {
			log.Fatal(err)
		}
		log.Print("reading levels and parents from json files")
		levels, parents := fileio.ReadLevels(dataDir)

		log.Print(time.Since(startTime))

		server.RunServer(levels, parents, h3ToCountry, countryToH3, countryPolygons, port)
	case "dbwrite":
		if len(os.Args) < 4 {
			log.Fatal("dbwrite subcommand has two argument: [data-directory] [sql-connection-string]")
		}
		dataDir := os.Args[2]
		connectionString := os.Args[3]

		// cmd := flag.NewFlagSet("dbwrite", flag.ExitOnError)
		// var tableName string
		// cmd.StringVar(&tableName, "t", "regions", "serving port")
		// cmd.Parse(os.Args[3:])

		db, err := database.SqlInitialize(connectionString)
		if err != nil {
			log.Fatal(err)
		}

		log.Print("creating tables")
		if err := database.CreateTables(db); err != nil {
			log.Fatal(err)
		}

		log.Print("reading country maps from json")
		if _, _, h3ToCountry, err := fileio.ReadCountryMaps(dataDir); err == nil {
			log.Print("populating countries")
			if err := database.PopulateCountries(db, &h3ToCountry); err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}

		log.Print("reading levels and parents from json files")
		levels, _ := fileio.ReadLevels(dataDir)
		log.Print("populating tiles")
		if err := database.PopulateTiles(db, levels); err != nil {
			log.Fatal(err)
		}
		log.Print("populating neighbors")
		if err := database.PopulateNeighbors(db, levels); err != nil {
			log.Fatal(err)
		}

		log.Print(time.Since(startTime))
	default:
		log.Fatal("run using one of these subcommands: generate, serve, dbwrite")
	}
}
