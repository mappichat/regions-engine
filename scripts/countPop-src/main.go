package main

import (
	"log"
	"os"

	"github.com/mappichat/regions-engine/src/utils"
)

func main() {
	path := os.Args[1]

	popmap := map[string]int{}
	if err := utils.ReadJsonFile(path, &popmap); err != nil {
		log.Fatal(err.Error())
	}

	pop := 0
	for _, count := range popmap {
		pop += count
	}

	log.Printf("total population in map: %d\n", pop)
}
