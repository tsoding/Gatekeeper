package main

import (
	"os"
	"os/signal"
	"syscall"
	"log"
	"time"
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

	// MPV //////////////////////////////
	_, ok = startMpvControl(tw);
	if !ok {
		log.Println("Could not start the MPV Control");
	}

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}
