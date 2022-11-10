package main

import (
	"hash/fnv"
	"strings"
	"fmt"
	"math/rand"
)

const (
	FieldRows = 9
	FieldCols = 9
	MinesCount = 13
	MaxMinesAttempts = FieldRows * FieldCols
	SeedSyllMaxLen = 5
	MineEmoji = "💥"
)

var (
	EmptyCellEmojis = [9]string{"🟦", "1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣"}
	SeedCons = []string{"b", "c", "d", "f", "g", "j", "k", "l", "m", "n", "p", "q", "s", "t", "v", "x", "z", "h", "r", "w", "y"}
	SeedVow = []string{"a", "e", "i", "o", "u"}
)

type MinesweeperField struct {
	cells [FieldRows][FieldCols]bool
}

func (field MinesweeperField) countNbors(row0 int, col0 int) (count int) {
	for drow := -1; drow <= 1; drow += 1 {
		for dcol := -1; dcol <= 1; dcol += 1 {
			if drow != 0 || dcol != 0 {
				row := drow + row0;
				col := dcol + col0;
				if 0 <= row && row < FieldRows && 0 <= col && col < FieldCols {
					if (field.cells[row][col]) {
						count += 1
					}
				}
			}
		}
	}
	return
}

func findFirstCell(field MinesweeperField) (row int, col int, found bool) {
	for row = 0; row < FieldRows; row += 1 {
		for col = 0; col < FieldRows; col += 1 {
			if !field.cells[row][col] && field.countNbors(row, col) == 0 {
				found = true
				return
			}
		}
	}
	return
}

func emojiOfCell(field MinesweeperField, row int, col int) string {
	if field.cells[row][col] {
		return MineEmoji
	}

	return EmptyCellEmojis[field.countNbors(row, col)]
}

func renderMinesweeperFieldForDiscord(field MinesweeperField, seed string) string {
	firstRow, firstCol, foundFirst := findFirstCell(field)
	var sb strings.Builder
	fmt.Fprintf(&sb, "👉 %s\n", seed)
	for row := 0; row < FieldRows; row += 1 {
		for col := 0; col < FieldCols; col += 1 {
			if foundFirst && row == firstRow && col == firstCol {
				fmt.Fprintf(&sb, "%s", emojiOfCell(field, row, col))
			} else {
				fmt.Fprintf(&sb, "||%s||", emojiOfCell(field, row, col))
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderOpenMinesweeperFieldForDiscord(field MinesweeperField, seed string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "open 👉 %s\n", seed)
	sb.WriteString("||")
	for row := 0; row < FieldRows; row += 1 {
		for col := 0; col < FieldCols; col += 1 {
			fmt.Fprintf(&sb, "%s", emojiOfCell(field, row, col))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("||")
	return sb.String()
}

func randomMinesweeperField(r *rand.Rand) (field MinesweeperField) {
	for i := 0; i < MinesCount; i += 1 {
		row := r.Intn(FieldRows)
		col := r.Intn(FieldCols)
		for j := 0; j < MaxMinesAttempts && field.cells[row][col]; j += 1 {
			row = r.Intn(FieldRows)
			col = r.Intn(FieldCols)
		}
		field.cells[row][col] = true
	}
	return
}

func randomMinesweeperSeed() string {
	var sb strings.Builder
	for i := 0; i < SeedSyllMaxLen; i += 1 {
		sb.WriteString(SeedCons[rand.Intn(len(SeedCons))])
		sb.WriteString(SeedVow[rand.Intn(len(SeedVow))])
	}
	return sb.String()
}

func seedAsSource(seed string) rand.Source {
	h := fnv.New64a()
	h.Write([]byte(seed))
	return rand.NewSource(int64(h.Sum64()))
}
