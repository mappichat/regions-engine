package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	utils "github.com/mappichat/regions-engine/src/utils"
	"github.com/uber/h3-go/v3"
)

func main() {
	start := time.Now()
	path := os.Args[1]
	res, _ := strconv.Atoi(os.Args[2])

	log.Print("reading json")
	oldmap := map[string]int{}
	if err := utils.ReadJsonFile(path, &oldmap); err != nil {
		log.Fatal(err.Error())
	}

	log.Printf("populating converted map using resolution %d\n", res)
	newMap := map[string]int{}
	for oldH, pop := range oldmap {
		h := h3.FromString(oldH)
		newH := h3.ToString(h3.ToParent(h, res))
		if _, ok := newMap[newH]; !ok {
			newMap[newH] = pop
		} else {
			newMap[newH] += pop
		}
	}

	log.Print("marshalling json")
	utils.WriteAsJsonFile(newMap, fmt.Sprintf("./popmap%d.json", res))

	log.Printf("total time: %s\n", time.Since(start))
}
