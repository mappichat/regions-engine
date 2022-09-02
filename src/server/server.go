package server

import (
	"fmt"
	"log"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"github.com/mappichat/regions-engine/src/project_types"
)

var validate = validator.New()

func RunServer(
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
