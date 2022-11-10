package main

import (
	"os"
	"fmt"
	"time"
	"math/rand"
	"github.com/tsoding/gatekeeper/internal"
	"database/sql"
	"flag"
)

func carrotsonTraverseTree(db *sql.DB, message []rune, limit int, walk func([]rune) error) (err error) {
	err = walk(message)
	if err != nil {
		return
	}
	if len(message) < limit {
		var branches []internal.Branch
		branches, err = internal.QueryBranchesFromContext(db, internal.ContextOfMessage(message))
		if err != nil {
			return
		}
		for _, branch := range branches {
			err = carrotsonTraverseTree(db, append(message, branch.Follows), limit, walk)
			if err != nil {
				return
			}
		}
	}
	return
}

type Subcmd struct {
	Run func(args []string) int
}

var Subcmds = map[string]Subcmd{
	"carrotree": Subcmd{
		Run: func(args []string) int {
			subFlag := flag.NewFlagSet("carrotree", flag.ExitOnError)
			prefix := subFlag.String("p", "", "Prefix")
			limit := subFlag.Int("l", 1024, "Limit")

			subFlag.Parse(args)

			db := internal.StartPostgreSQL()
			if db == nil {
				return 1
			}
			defer db.Close()

			err := carrotsonTraverseTree(db, []rune(*prefix), *limit, func(message []rune) error {
				fmt.Println("CARROTSON:", string(message))
				return nil
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}

			return 0
		},
	},
	"carrot": Subcmd{
		Run: func(args []string) int {
			subFlag := flag.NewFlagSet("carrot", flag.ExitOnError)
			prefix := subFlag.String("p", "", "Prefix")
			limit := subFlag.Int("l", 1024, "Limit")

			subFlag.Parse(args)

			db := internal.StartPostgreSQL()
			if db == nil {
				return 1
			}
			defer db.Close()

			message, err := internal.CarrotsonGenerate(db, *prefix, *limit)
			if err != nil {
				fmt.Fprintln(os.Stderr, "ERROR: could not generate Carrotson message:", err)
				return 1;
			}

			fmt.Println("CARROTSON:", message)
			return 0
		},
	},
}

func topUsage(program string) {
	fmt.Fprintf(os.Stderr, "Usage: %s <SUBCOMMAND> [OPTIONS]\n", program);
	fmt.Fprintf(os.Stderr, "SUBCOMMANDS:\n");
	for name, _ := range(Subcmds) {
		fmt.Fprintf(os.Stderr, "    %s\n", name)
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	if len(os.Args) <= 0 {
		panic("Empty command line arguments")
	}
	program := "gaslighter"

	if len(os.Args) < 2 {
		topUsage(program)
		fmt.Fprintf(os.Stderr, "ERROR: no subcommand is provided\n");
		os.Exit(1)
	}
	subcmdName := os.Args[1]

	if subcmd, ok := Subcmds[subcmdName]; ok {
		os.Exit(subcmd.Run(os.Args[2:]))
	} else {
		topUsage(program)
		fmt.Fprintf(os.Stderr, "ERROR: unknown subcommand `%s`\n", subcmdName);
		os.Exit(1)
	}
}
