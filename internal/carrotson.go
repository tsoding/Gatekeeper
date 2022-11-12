package internal

import (
	"database/sql"
	"log"
	"errors"
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
	Follows rune
	Frequency int64
}

var (
	EmptyFollowsError = errors.New("Empty follows of a Carrotson branch")
)

func QueryRandomBranchFromContext(db *sql.DB, context []rune) (*Branch, error) {
	row := db.QueryRow("SELECT follows, frequency FROM Carrotson_Branches WHERE context = $1 AND frequency > 0 ORDER BY random() LIMIT 1", string(context))
	var follows string
	var frequency int64
	err := row.Scan(&follows, &frequency)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(follows) == 0 {
		return nil, EmptyFollowsError
	}
	return &Branch{
		Follows: []rune(follows)[0],
		Frequency: frequency,
	}, nil
}

func QueryBranchesFromContext(db *sql.DB, context []rune) ([]Branch, error) {
	rows, err := db.Query("SELECT follows, frequency FROM Carrotson_Branches WHERE context = $1 AND frequency > 0", string(context))
	if err != nil {
		return nil, err
	}
	branches := []Branch{}
	for rows.Next() {
		branch := Branch{}
		var follows string
		err = rows.Scan(&follows, &branch.Frequency)
		if err != nil {
			return nil, err
		}
		if len(follows) == 0 {
			return nil, EmptyFollowsError
		}
		branch.Follows = []rune(follows)[0]
		branches = append(branches, branch)
	}
	return branches, nil
}

func ContextOfMessage(message []rune) []rune {
	i := len(message) - ContextSize
	if i < 0 {
		i = 0
	}
	return message[i:len(message)]
}

func CarrotsonGenerate(db *sql.DB, prefix string, limit int) (string, error) {
	var err error = nil
	message := []rune(prefix)
	branch, err := QueryRandomBranchFromContext(db, ContextOfMessage(message))
	for err == nil && branch != nil && len(message) < limit {
		message = append(message, branch.Follows)
		branch, err = QueryRandomBranchFromContext(db, ContextOfMessage(message))
	}
	return string(message), err
}

func FeedMessageToCarrotson(db *sql.DB, message string) {
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
