package main

import (
	"database/sql"
	"strings"
	"slices"
	"strconv"
	"unicode/utf8"
	"log"
	"fmt"
)

const (
	// NOTE: if these values are modified the size of the buffer of the Ed_State table
	// should be adjusted as well. Ideally it should be equal or bigger than
	// 2*EdLineCountLimit*EdLineSizeLimit (the 2 is to accomodate the newlines)
	EdLineCountLimit = 6
	EdLineSizeLimit = 101
)

type EdMode int

const (
	EdCommandMode EdMode = iota
	EdInsertMode
)

type EdState struct {
	Buffer []string
	Cursor int
	Mode EdMode
}

func SaveEdStateByUserId(db *sql.DB, userId string, ed EdState) error {
	_, err := db.Exec("INSERT INTO Ed_State (user_id, buffer, cur, mode) VALUES ($1, $2, $3, $4) ON CONFLICT (user_id) DO UPDATE SET buffer = EXCLUDED.buffer, cur = EXCLUDED.cur, mode = EXCLUDED.mode;", userId, strings.Join(ed.Buffer, "\n"), ed.Cursor, ed.Mode)
	if err != nil {
		return err
	}
	return nil
}

func LoadEdStateByUserId(db *sql.DB, userId string) (EdState, error) {
	row := db.QueryRow("SELECT buffer, cur, mode FROM Ed_State WHERE user_id = $1", userId);
	var buffer string
	var cursor int
	var mode int
	err := row.Scan(&buffer, &cursor, &mode);
	if err == sql.ErrNoRows {
		return EdState{}, nil
	}
	if err != nil {
		return EdState{}, err
	}
	ed := EdState{
		Cursor: cursor,
		Mode: EdMode(mode),
	}
	if len(buffer) > 0 {
		ed.Buffer = strings.Split(buffer, "\n");
	}
	return ed, nil
}

func (ed *EdState) Print(env CommandEnvironment, line string) {
	env.SendMessage(env.AtAuthor()+" "+line);
}

func (ed *EdState) LineAt(index int) (string, bool) {
	if 0 <= index && index < len(ed.Buffer) {
		return ed.Buffer[index], true
	} else {
		return "", false
	}
}

func (ed *EdState) Huh(env CommandEnvironment) {
	env.SendMessage(env.AtAuthor()+" ?")
}

func (ed *EdState) ExecCommand(env CommandEnvironment, command string) {
	switch ed.Mode {
	case EdCommandMode:
		switch command {
		case "":
			newCursor := ed.Cursor + 1
			if line, ok := ed.LineAt(newCursor); ok {
				ed.Cursor = newCursor
				ed.Print(env, line)
			} else {
				ed.Huh(env)
			}
		case "a":
			ed.Mode = EdInsertMode
		case "d":
			if _, ok := ed.LineAt(ed.Cursor); ok {
				ed.Buffer = slices.Delete(ed.Buffer, ed.Cursor, ed.Cursor+1)
				if ed.Cursor >= len(ed.Buffer) && ed.Cursor > 0 { // Cursor overflew after deleting last line
					ed.Cursor -= 1
				}
			} else {
				ed.Huh(env)
			}
		case ",p":
			if len(ed.Buffer) > 0 {
				for _, line := range(ed.Buffer) {
					ed.Print(env, line)
				}
			} else {
				ed.Huh(env)
			}
		case "p":
			if line, ok := ed.LineAt(ed.Cursor); ok {
				ed.Print(env, line)
			} else {
				ed.Huh(env)
			}
		default:
			i, err := strconv.Atoi(command)
			if err != nil {
				ed.Huh(env)
				return
			}
			newCursor := i - 1 // 1-based indexing
			if line, ok := ed.LineAt(newCursor); ok {
				ed.Cursor = newCursor
				ed.Print(env, line)
			} else {
				ed.Huh(env)
			}
		}
	case EdInsertMode:
		if command == "." {
			ed.Mode = EdCommandMode
		} else {
			// NOTE: Keep in mind that to check the EdLineCountLimit
			// we use `>=`, but to check EdLineSizelimit we use
			// `>`. This is due to EdLineCountLimit being about
			// checking the size of the buffer BEFORE inserting any
			// new lines. While EdLineSizeLimit is about checking the
			// size of the line we are about to insert.
			if len(ed.Buffer) >= EdLineCountLimit {
				env.SendMessage(fmt.Sprintf("%s Your message exceeded line count limit (You may have %d lines maximum)", env.AtAuthor(), EdLineCountLimit))
				return
			}
			if utf8.RuneCountInString(command) > EdLineSizeLimit {
				env.SendMessage(fmt.Sprintf("%s Your message exceeded line size limit (Your lines may have %d characters maximum)", env.AtAuthor(), EdLineSizeLimit))
				return
			}
			if _, ok := ed.LineAt(ed.Cursor); ok {
				ed.Cursor += 1
				ed.Buffer = slices.Insert(ed.Buffer, ed.Cursor, command)
			} else if len(ed.Buffer) == 0 {
				ed.Cursor = 0
				ed.Buffer = append(ed.Buffer, command);
			} else {
				ed.Huh(env)
			}
		}
	default:
		log.Printf("Invalid mode of Ed State: %#v\n", ed)
		env.SendMessage(fmt.Sprintf("%s something went wrong with the state of your Ed. I've tried to correct it. Try again and ask %s to check the logs if the problem persists.", env.AtAuthor(), env.AtAdmin()));
		ed.Mode = EdCommandMode
	}
}
