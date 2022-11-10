package main

import (
	"encoding/json"
	"regexp"
	"net/http"
	"log"
	"fmt"
	"database/sql"
)

type Route struct {
	Regexp *regexp.Regexp
	Handler func(wp *WebApp, w http.ResponseWriter, r *http.Request, matches []string)
}

func handlerStatic(wp *WebApp, w http.ResponseWriter, r *http.Request, matches []string) {
	switch matches[1] {
	// TODO: unhardcode static files
	case "": fallthrough
	case "index.html":
		log.Println("serve index")
		http.ServeFile(w, r, "index.html")
	case "index.js":
		http.ServeFile(w, r, "index.js")
	default:
		w.WriteHeader(404)
		fmt.Fprintf(w, "Resource is not found\n")
	}
}

func handlerAllUser(wp *WebApp, w http.ResponseWriter, r *http.Request, matches []string) {
	rows, err := wp.DB.Query("SELECT id FROM TrustedUsers;")
	if err != nil {
		log.Println("Could not query children ids from database:", err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Server pooped its pants\n")
		return
	}

	usersIds := []string{}
	for rows.Next() {
		var userId string
		err = rows.Scan(&userId)
		if err != nil {
			log.Println("Could not collect user ids:", err)
			return
		}
		usersIds = append(usersIds, userId)
	}

	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"usersIds": usersIds,
	})
	if err != nil {
		log.Println("Could not encode respose:", err)
	}
}

func handlerChildrenOfUser(wp *WebApp, w http.ResponseWriter, r *http.Request, matches []string) {
	rows, err := wp.DB.Query("SELECT trusteeId FROM TrustLog WHERE trusterId = $1;", matches[1])
	if err != nil {
		log.Println("Could not query children ids from database:", err)
		w.WriteHeader(500)
		fmt.Fprintf(w, "Server pooped its pants\n")
		return
	}
	defer rows.Close()

	childrenIds := []string{}
	for rows.Next() {
		var childId string
		err = rows.Scan(&childId)
		if err != nil {
			log.Println("Could not collect children ids:", err)
			return
		}
		childrenIds = append(childrenIds, childId)
	}

	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"parentId": matches[1],
		"childrenIds": childrenIds,
	})
	if err != nil {
		log.Println("Could not encode respose:", err)
	}
}

type WebApp struct {
	Routes []Route
	DB *sql.DB
}

func (wp *WebApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range(wp.Routes) {
		matches := route.Regexp.FindStringSubmatch(r.URL.Path)
		if len(matches) > 0 {
			route.Handler(wp, w, r, matches)
			return
		}
	}

	w.WriteHeader(404)
	fmt.Fprintf(w, "Resource is not found\n")
}
