package internal

import (
	"database/sql"
	"strings"
	"unicode"
	"math"
)

const (
	ThresholdCount = 10
)

func GrokTokenizeMessage(message string) []string {
	return strings.FieldsFunc(message, func(x rune) bool {
		return unicode.IsSpace(x) || unicode.IsPunct(x)
	})
}

func grokGetTotalCounts(db *sql.DB) (yesCount int64, noCount int64, err error) {
	var yes bool
	var count int64
	rows, err := db.Query("select yes, sum(count) from grok group by yes");
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&yes, &count)
		if err != nil {
			panic(err)
		}
		if yes {
			yesCount = count
		} else {
			noCount = count
		}
	}
	return
}

func grokGetWordsCounts(db *sql.DB, word string) (yesCount int64, noCount int64, err error) {
	var yes bool
	var count int64
	rows, err := db.Query("select yes, count from grok where word = $1", word)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&yes, &count)
		if err != nil {
			panic(err)
		}
		if yes {
			yesCount = count
		} else {
			noCount = count
		}
	}
	return
}

func GrokQuery(db *sql.DB, prompt string) (float64, float64, error) {
	totalYesCount, totalNoCount, err := grokGetTotalCounts(db)
	if err != nil {
		return 0, 0, err
	}
	totalTotalCount := totalYesCount + totalNoCount;

	dp     := 0.0
	yesDp  := 0.0
	noDp   := 0.0
	yesP   := math.Log(float64(totalYesCount)/float64(totalTotalCount))
	noP    := math.Log(float64(totalNoCount)/float64(totalTotalCount))
	for _, token := range GrokTokenizeMessage(prompt) {
		word := strings.ToUpper(token)
		yesCount, noCount, err := grokGetWordsCounts(db, word)
		totalCount := yesCount + noCount
		if err != nil {
			return 0, 0, err
		}
		if totalCount < ThresholdCount {
			continue
		}
		if yesCount != 0 {
			yesDp += math.Log(float64(yesCount)/float64(totalYesCount))
		}
		if noCount != 0 {
			noDp += math.Log(float64(noCount)/float64(totalNoCount))
		}
		if totalCount != 0 {
			dp += math.Log(float64(totalCount)/float64(totalTotalCount))
		}
	}
	return yesDp + yesP - dp, noDp + noP - dp, nil
}
