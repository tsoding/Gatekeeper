package main

import (
	"os"
	"log"
	"net"
	_"fmt"
	_"time"
	"encoding/json"
	"strings"
)

type MpvSong struct {
	title string
	artist string
}

type Object = map[string]interface{}
type Array = []interface{}

func startMpvControlThread(conn net.Conn, mpvIpcAddress string, msgs chan *MpvSong) {
	defer conn.Close()

	var root interface{}
	decoder := json.NewDecoder(conn)

	for {
		err := decoder.Decode(&root)
		if err != nil {
			log.Printf("MPV: Could not read from %s: %s\n", mpvIpcAddress, err);
			return;
		}
		cursor := root.(Object);
		if cursor["event"] != nil {
			if cursor["event"] == "file-loaded" {
				_, err := conn.Write([]byte("{ \"command\": [\"get_property\", \"metadata\"] }\n"));
				if err != nil {
					log.Printf("MPV: could not send command: %s\n", err);
					return
				}
			}
		} else if cursor["data"] != nil {
			cursor := cursor["data"].(Object);
			title := "UNKNOWN"
			artist := "UNKNOWN"
			for k, v := range cursor {
				switch strings.ToLower(k) {
				case "title": title = v.(string);
				case "artist": artist = v.(string);
				}
			}
			msgs <- &MpvSong{
				title: title,
				artist: artist,
			}
			log.Printf("MPV: %#v", cursor);
		} else {
			log.Printf("MPV: unknown message: %v\n", root);
		}
	}
}

func startMpvControl() (chan *MpvSong, bool) {
	msgs := make(chan *MpvSong);

	mpvIpcAddress := os.Getenv("GATEKEEPER_MPV_IPC_ADDRESS");
	if mpvIpcAddress == "" {
		log.Println("MPV: no GATEKEEPER_MPV_IPC_ADDRESS is provided.");
		return msgs, false
	}

	l, err := net.Listen("tcp", mpvIpcAddress);
	if err != nil {
		log.Printf("MPV: could not listen to %s: %s", mpvIpcAddress, err);
		return msgs, false
	}

	go func() {
		for {
			conn, err := l.Accept();
			if err != nil {
				log.Printf("MPV: failed to listen to %s: %s", mpvIpcAddress, err);
				return;
			}

			log.Printf("MPV: %s connected", conn.RemoteAddr());
			startMpvControlThread(conn, mpvIpcAddress, msgs);
		}
	}();

	return msgs, true
}
