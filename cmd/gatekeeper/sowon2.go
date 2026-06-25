package main

import (
	"os"
	"log"
	"database/sql"
	"net/http"
)

type Song struct {
	title string
	artist string
	link string
}

func LogSong(db *sql.DB, song Song) {
	_, err := db.Exec("INSERT INTO Song_Log (artist, title, link) VALUES ($1, $2, $3)", song.artist, song.title, song.link);
	if err != nil {
		log.Println("ERROR: LogSong: could not insert element %#v: %s", song, err);
		return
	}
}

func LastSongPlayed(db *sql.DB) *Song {
	row := db.QueryRow("select artist, title, link from Song_Log order by startedAt desc limit 1")
	var artist string
	var title string
	var link string
	err := row.Scan(&artist, &title, &link)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		log.Printf("SOWON2: Could not query last played song: %s", err)
		return nil
	}
	return &Song{
		artist: artist,
		title: title,
		link: link,
	}
}

type Object = map[string]interface{}
type Array = []interface{}

func startSowon2Control() (chan Song, bool) {
	msgs := make(chan Song);

	sowon2HttpAddress := os.Getenv("GATEKEEPER_SOWON2_HTTP_ADDRESS");
	if sowon2HttpAddress == "" {
		sowon2HttpAddress = os.Getenv("GATEKEEPER_MPV_IPC_ADDRESS");
		if sowon2HttpAddress == "" {
			log.Println("SOWON2: No GATEKEEPER_SOWON2_HTTP_ADDRESS is provided.")
			return msgs, false
		}
		// We used to support MPV IPC control mechanism
		// https://github.com/mpv-player/mpv/blob/2339eb72767517fc5a113283939f59076946fbc1/DOCS/man/ipc.rst
		// But at some point we migrated to a custom HTTP based one. Since the format of the address is the
		// same we accept whatever you have in GATEKEEPER_MPV_IPC_ADDRESS for the sake of backward compatibility.
		log.Println("SOWON2: No GATEKEEPER_SOWON2_HTTP_ADDRESS is provided, but we found GATEKEEPER_MPV_IPC_ADDRESS. Using it instead. Please rename your GATEKEEPER_MPV_IPC_ADDRESS to GATEKEEPER_SOWON2_HTTP_ADDRESS when you get a chance!")
	}

	http.HandleFunc("/song", func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			log.Printf("ERROR: could not parse POST form: %v\n", err)
			return
		}

		values := r.PostForm;
		artist := values.Get("artist");
		title  := values.Get("title");
		link   := values.Get("link");
		msgs <- Song{
			title: title,
			artist: artist,
			link: link,
		}
	})

	log.Printf("Listenting to http://%v\n", sowon2HttpAddress);

	go func() {
		err := http.ListenAndServe(sowon2HttpAddress, nil);
		if err != nil {
			log.Printf("ERROR: could not start up the server: %v\n", err)
			return
		}
	}();

	return msgs, true
}
