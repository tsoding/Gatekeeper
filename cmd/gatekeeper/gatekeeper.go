package main

import (
	"os"
	"os/signal"
	"syscall"
	"log"
	"net/http"
	"context"
	"time"
	"regexp"
	"math/rand"
	"github.com/tsoding/gatekeeper/internal"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// PostgreSQL //////////////////////////////
	db := internal.StartPostgreSQL()
	if db == nil {
		log.Println("Starting without PostgreSQL. Commands that require it won't work.")
	} else {
		defer db.Close()
	}

	// Discord //////////////////////////////
	dg, err := startDiscord(db)
	if err != nil {
		log.Println("Could not open Discord connection:", err);
	} else {
		defer dg.Close();
	}

	// Twitch //////////////////////////////
	tw, ok := startTwitch(db);
	if !ok {
		log.Println("Could not open Twitch connection");
	} else {
		defer tw.Close()
	}

	// HTTP Server //////////////////////////////
	// TODO: web client for the REST API
	server := &http.Server{
		// TODO: customizable port (probably via envar)
		Addr: "localhost:6969",
		Handler: &WebApp{
			Routes: []Route{
				Route{
					Regexp: regexp.MustCompile("^/user/([0-9]+)/children$"),
					Handler: handlerChildrenOfUser,
				},
				Route{
					Regexp: regexp.MustCompile("^/user$"),
					Handler: handlerAllUser,
				},
				Route{
					Regexp: regexp.MustCompile("^/(.*)$"),
					Handler: handlerStatic,
				},
			},
			DB: db,
		},
	}
	server = nil

	go func() {
		if server != nil {
			log.Println("Starting Web server")
			err := server.ListenAndServe()
			log.Println("HTTP server stopped:", err)
		} else {
			log.Println("HTTP server is temporarily disabled")
		}
	}()

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session, the Postgres connection and the HTTP server.
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Println("Could not shutdown HTTP server:", err)
		}
	}
}
