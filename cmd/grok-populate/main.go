package main

import (
	"fmt"
	"os"
	"time"
	"io"
	"log"
	"strings"
	"encoding/csv"
	"github.com/tsoding/gatekeeper/internal"
	"database/sql"
)

const (
	FieldMessageId int = iota
	FieldUserId
	FieldUserName
	FieldPostedAt
	FieldText
)

const (
	ThresholdSecs = 20
	TimeLayout = "2006-01-02 15:04:05"
)

func populateDatabaseFromLog(db *sql.DB, inputPath string) error {
	file, err := os.Open(inputPath);
	if err != nil {
		return err
	}
	defer file.Close()
	r := csv.NewReader(file)
	header := true
	var a, b []string
	state := 0
	count := 0
	for {
		records, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break;
			}
			return err
		}
		if header {
			header = false
			continue
		}
		switch state {
		case 0:
			a = records
			state += 1
		case 1:
			b = records
			state += 1
		case 2:
			a = b
			b = records
		default:
			panic("unreachable")
		}
		if state == 2 {
			msg := strings.ToUpper(b[FieldText])
			if msg == "YES" || msg == "NO" {
				ta, err := time.Parse(TimeLayout, a[FieldPostedAt])
				if err != nil {
					return err
				}

				tb, err := time.Parse(TimeLayout, b[FieldPostedAt])
				if err != nil {
					return err
				}

				if tb.Sub(ta).Seconds() < ThresholdSecs {
					count += 1
					log.Printf("[%v] [%v] %v\n", count, b[FieldText], a[FieldText]);
					tokens := internal.GrokTokenizeMessage(a[FieldText])
					yes := strings.ToUpper(b[FieldText]) == "YES"
					for _, token := range tokens {
						_, err := db.Exec(`INSERT INTO Grok (yes, word, count)
	VALUES ($1, $2, 1)
	ON CONFLICT (yes, word) DO UPDATE SET count = Grok.count + 1;`, yes, strings.ToUpper(token))
						if err != nil {
							return err
						}
					}
					state = 0
				}
			}
		}
	}
	return nil
}

func main() {
	args := os.Args
	programName, args := args[0], args[1:]

	if len(args) == 0 {
		fmt.Printf("Usage: %v <inputPath>\n", programName);
		fmt.Printf("ERROR: no input file is provided\n");
		os.Exit(1)
	}

	inputPath, args := args[0], args[1:]

	// PostgreSQL //////////////////////////////
	db := internal.StartPostgreSQL()
	if db == nil {
		log.Println("Starting without PostgreSQL. Commands that require it won't work.")
	} else {
		defer db.Close()
	}

	err := populateDatabaseFromLog(db, inputPath)
	if err != nil {
		panic(err)
	}
}
