package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"github.com/mappichat/regions-engine/src/database"
	"github.com/mappichat/regions-engine/src/engine"
	"github.com/mappichat/regions-engine/src/fileio"
	"github.com/mappichat/regions-engine/src/project_types"
	"github.com/mappichat/regions-engine/src/utils"
)

var validate = validator.New()

func runServer(
	levels []map[string]project_types.Region,
	parents []map[string]string,
	h3ToCountry project_types.H3ToCountry,
	countryToH3 project_types.CountryToH3,
	countryPolygons project_types.CountryPolygons,
	port int,
) {
	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Healthy")
	})

	app.Post("/regions", func(c *fiber.Ctx) error {
		log.Printf("/regions: %s\n", time.Now())
		payload := struct {
			Tiles  []string `json:"tiles" validate:"required"`
			Levels []int    `json:"levels" validate:"required"`
		}{}

		if err := c.BodyParser(&payload); err != nil {
			return err
		}
		if err := validate.Struct(payload); err != nil {
			return err
		}

		regions := map[int]map[string][]string{}

		for _, level := range payload.Levels {
			regions[level] = map[string][]string{}
			for _, tile := range payload.Tiles {
				parent := parents[level][tile]
				regions[level][parent] = levels[level][parent].Tiles
			}
		}

		return c.JSON(regions)
	})

	app.Post("/ring", func(c *fiber.Ctx) error {
		log.Printf("/ring: %s\n", time.Now())
		payload := struct {
			Tile   string `json:"tile" validate:"required"`
			Level  int    `json:"level"`
			Radius int    `json:"radius"`
		}{}

		if err := c.BodyParser(&payload); err != nil {
			return err
		}
		if err := validate.Struct(payload); err != nil {
			return err
		}

		regions := map[string][]string{}

		center := parents[payload.Level][payload.Tile]
		r := 0
		i := 0
		regionQueue := []string{center}
		seen := map[string]bool{center: true}
		for r <= payload.Radius {
			nextEnd := len(regionQueue)
			for i < nextEnd {
				current := regionQueue[i]
				regions[current] = levels[payload.Level][current].Tiles
				for neighbor := range levels[payload.Level][current].Neighbors {
					if _, ok := seen[neighbor]; !ok {
						regionQueue = append(regionQueue, neighbor)
						seen[neighbor] = true
					}
				}
				i++
			}
			r++
		}

		log.Print(len(regions))

		return c.JSON(regions)
	})

	app.Post("/country", func(c *fiber.Ctx) error {
		log.Printf("/country: %s\n", time.Now())
		payload := struct {
			Tile string `json:"tile" validate:"required"`
		}{}

		if err := c.BodyParser(&payload); err != nil {
			return err
		}
		if err := validate.Struct(payload); err != nil {
			return err
		}
		log.Print(payload.Tile)
		country := h3ToCountry[payload.Tile]
		log.Print(country)

		return c.JSON(countryToH3[h3ToCountry[payload.Tile]])
	})

	log.Fatal(app.Listen(fmt.Sprintf(":%d", port)))
}

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
		cmd.IntVar(&resolution, "r", 5, "h3 resolution used to generate regions")
		cmd.StringVar(&popMapPath, "p", "", "path to popmap file (json)")
		cmd.StringVar(&configPath, "c", "", "path to engine config file (json)")
		cmd.StringVar(&outDir, "o", "", "data output directory")
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
		err = engine.GenerateAndWriteLevels(popMap, countryToH3, outDir, resolution, options)
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

		runServer(levels, parents, h3ToCountry, countryToH3, countryPolygons, port)
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
