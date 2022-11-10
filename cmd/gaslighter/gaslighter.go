package main

import (
	"os"
	"log"
	"time"
	"math/rand"
	"github.com/tsoding/gatekeeper/internal"
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

		prefix := "" // TODO: supply prefix via the args
		limit := 1024 // TODO: supply limit vai the args
		message, err := internal.CarrotsonGenerate(db, prefix, limit, false)
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
