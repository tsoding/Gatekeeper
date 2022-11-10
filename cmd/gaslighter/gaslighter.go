package main

import (
	"os"
	"log"
	"time"
	"math/rand"
	"github.com/tsoding/gatekeeper/internal"
	"strconv"
)

func bump(x *int) int {
	prev := *x
	*x += 1
	return prev
}

func main() {
	rand.Seed(time.Now().UnixNano())

	argsCur := 0
	if argsCur >= len(os.Args) {
		panic("Empty command line arguments")
	}
	program := os.Args[bump(&argsCur)]

	if argsCur >= len(os.Args) {
		log.Printf("Usage: %s <SUBCOMMAND> [OPTIONS]\n", program);
		log.Printf("ERROR: no subcommand is provided\n");
		os.Exit(1)
	}
	subcommand := os.Args[bump(&argsCur)]

	switch subcommand {
	case "carrot":
		db := internal.StartPostgreSQL()
		if db == nil {
			os.Exit(1)
		}
		defer db.Close()

		prefix := ""
		limit := 1024
		weighted := false

		for argsCur < len(os.Args) {
			flag := os.Args[bump(&argsCur)]
			switch flag {
			case "-w":
				weighted = true
			case "-l":
				if argsCur < len(os.Args) {
					log.Printf("ERROR: no value is provided for %s\n", flag)
					return
				}
				value := os.Args[bump(&argsCur)]
				var err error
				limit, err = strconv.Atoi(value)
				if err != nil {
					log.Println("ERROR: could not parse %s as a value:", err)
					os.Exit(1)
				}
			default:
				prefix = flag
			}
		}

		message, err := internal.CarrotsonGenerate(db, prefix, limit, weighted)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}

		log.Println("CARROTSON:", message)
	default:
		log.Printf("Usage: %s <SUBCOMMAND> [OPTIONS]\n", program);
		log.Printf("ERROR: unknown subcommand `%s`\n", subcommand);
		os.Exit(1)
	}
}
