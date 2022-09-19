package database

import (
	"fmt"
	"log"
	"math"
	"strings"
	"sync"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/mappichat/regions-engine/src/project_types"
)

const maxInsert int = 65535

func SqlInitialize(connectString string) (*sqlx.DB, error) {
	var err error
	Sqldb, err := sqlx.Connect("postgres", connectString)
	if err != nil {
		return Sqldb, err
	}
	if err = Sqldb.Ping(); err != nil {
		return Sqldb, err
	}
	return Sqldb, nil
}

func CreateTables(db *sqlx.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS countries (
		h3 text PRIMARY KEY,
		country text
	);`); err != nil {
		return err
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS tiles (
		h3 text,
		region text,
		level int,
		PRIMARY KEY (level, h3)
	);`); err != nil {
		return err
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS neighbors (
		region text,
		neighbor text,
		level int,
		PRIMARY KEY (level, region, neighbor)
	);`); err != nil {
		return err
	}

	return nil
}

func PopulateCountries(db *sqlx.DB, h3ToCountry *project_types.H3ToCountry) error {
	values := []map[string]interface{}{}
	for h3, country := range *h3ToCountry {
		values = append(values, map[string]interface{}{"h3": h3, "country": strings.ToLower(country)})
	}
	batchSize := maxInsert / 2
	for i := 0; i < len(values); i += batchSize {
		if _, err := db.NamedExec(
			`INSERT INTO countries (h3, country) VALUES (:h3, :country)`,
			values[i:int(math.Min(float64(len(values)), float64(i+batchSize)))],
		); err != nil {
			return err
		}
	}
	return nil
}

func PopulateTiles(db *sqlx.DB, levels []map[string]project_types.Region) error {
	wg := sync.WaitGroup{}
	for i := range levels {
		wg.Add(1)
		go func(index int) {
			PopulateTile(db, index, &levels[index])
			wg.Done()
		}(i)
	}
	wg.Wait()
	return nil
}

func PopulateTile(db *sqlx.DB, levelIndex int, level *map[string]project_types.Region) error {
	batchSize := maxInsert / 3

	values := []map[string]interface{}{}
	for h3 := range *level {
		for _, tile := range (*level)[h3].Tiles {
			values = append(values, map[string]interface{}{"h3": tile, "region": h3, "level": levelIndex})
		}
	}
	total := len(values)
	for i := 0; i < len(values); i += batchSize {
		fmt.Printf(" %f%%\r", 100*(float64(i)/float64(total)))
		if _, err := db.NamedExec(
			`INSERT INTO tiles (h3, region, level) VALUES (:h3, :region, :level)`,
			values[i:int(math.Min(float64(len(values)), float64(i+batchSize)))],
		); err != nil {
			log.Fatal(err)
		}
	}

	return nil
}

func PopulateNeighbors(db *sqlx.DB, levels []map[string]project_types.Region) error {
	wg := sync.WaitGroup{}
	for i := range levels {
		wg.Add(1)
		go func(index int) {
			PopulateNeighbor(db, index, &levels[index])
			wg.Done()
		}(i)
	}
	wg.Wait()
	return nil
}

func hashPair(h1 string, h2 string) string {
	if h1 < h2 {
		return h1 + h2
	} else {
		return h2 + h1
	}
}

func PopulateNeighbor(db *sqlx.DB, levelIndex int, level *map[string]project_types.Region) error {
	batchSize := maxInsert / 3
	values := []map[string]interface{}{}
	for h3 := range *level {
		for neighbor := range (*level)[h3].Neighbors {
			values = append(values, map[string]interface{}{"region": h3, "neighbor": neighbor, "level": levelIndex})
		}
	}
	total := len(values)
	for i := 0; i < len(values); i += batchSize {
		fmt.Printf(" %f%%\r", 100*(float64(i)/float64(total)))
		if _, err := db.NamedExec(
			`INSERT INTO neighbors (region, neighbor, level) VALUES (:region, :neighbor, :level)`,
			values[i:int(math.Min(float64(len(values)), float64(i+batchSize)))],
		); err != nil {
			log.Fatal(err)
		}
	}

	return nil
}
