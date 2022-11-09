package main

import (
	"database/sql"
	"log"
	"fmt"
	"math/rand"
)

const ContextSize = 8

type Path struct {
	context []rune
	follows rune
}

func splitMessageIntoPaths(message []rune) (branches []Path) {
	for i := -ContextSize; i + ContextSize < len(message); i += 1 {
		j := i
		if j < 0 {
			j = 0
		}
		branches = append(branches, Path{
			context: message[j:i + ContextSize],
			follows: message[i + ContextSize],
		})
	}
	return
}

type Branch struct {
	follows rune
	frequency int64
}

func queryBranchesFromContext(db *sql.DB, context []rune) ([]Branch, error) {
	rows, err := db.Query("SELECT follows, frequency FROM Carrotson_Branches WHERE context = $1", string(context))
	if err != nil {
		return nil, err
	}
	branches := []Branch{}
	for rows.Next() {
		branch := Branch{}
		var follows string
		err = rows.Scan(&follows, &branch.frequency)
		if err != nil {
			return nil, err
		}
		if len(follows) == 0 {
			return nil, fmt.Errorf("Empty follows")
		}
		branch.follows = []rune(follows)[0]
		branches = append(branches, branch)
	}
	return branches, nil
}

type BranchStrategy int

func branchRandomly(branches []Branch, weighted bool) rune {
	if weighted {
		var sum int64 = 0
		for _, branch := range branches {
			sum += branch.frequency
		}

		index := rand.Int63n(sum)
		var psum int64 = 0
		for _, branch := range branches {
			psum += branch.frequency
			if index <= psum {
				return branch.follows
			}
		}

		panic("unreachable")
	} else {
		if len(branches) == 0 {
			panic("unreachable")
		}

		return branches[rand.Intn(len(branches))].follows
	}
}

func contextOfMessage(message []rune) []rune {
	i := len(message) - ContextSize
	if i < 0 {
		i = 0
	}
	return message[i:len(message)]
}

func carrotsonGenerate(db *sql.DB, prefix string, limit int, weighted bool) (string, error) {
	var err error = nil
	message := []rune(prefix)
	branches, err := queryBranchesFromContext(db, contextOfMessage(message))
	for err == nil && len(branches) > 0 && len(message) < limit {
		follows := branchRandomly(branches, weighted)
		message = append(message, follows)
		branches, err = queryBranchesFromContext(db, contextOfMessage(message))
	}
	return string(message), err
}

func feedMessageToCarrotson(db *sql.DB, message string) {
	tx, err := db.Begin()
	if err != nil {
		log.Println("ERROR: feedMessageToCarrotson: could not start transaction:", err)
		return
	}
	for _, path := range splitMessageIntoPaths([]rune(message)) {
		_, err := tx.Exec("INSERT INTO Carrotson_Branches (context, follows, frequency) VALUES ($1, $2, 1) ON CONFLICT (context, follows) DO UPDATE SET frequency = Carrotson_Branches.frequency + 1;", string(path.context), string([]rune{path.follows}))
		if err != nil {
			log.Println("ERROR: feedMessageToCarrotson: could not insert element", string(path.context), string([]rune{path.follows}), ":", err)
			err := tx.Rollback()
			if err != nil {
				log.Println("ERROR: feedMessageToCarrotson: could not rollback transaction after failure:", err)
			}
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		log.Println("ERROR: feedMessageToCarrotson: could not commit transaction:", err)
	}
}
