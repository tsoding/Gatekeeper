package main

import (
	"os"
	"os/signal"
	"syscall"
	"log"
	"fmt"
	_ "html"
	"strings"
	"github.com/bwmarrin/discordgo"
	"database/sql"
	_ "github.com/lib/pq"
	"net/http"
	"encoding/json"
	"regexp"
	"context"
	"time"
)

var CommandPrefix = "$"
var TrustedRoleId = "543864981171470346"

func LookupEnvOrDie(name string) string {
	env, found := os.LookupEnv(name)
	if !found {
		log.Fatalln("Could not find ", name, " variable")
	}
	return env
}

func isMemberTrusted(member *discordgo.Member) bool {
	for _, roleId := range member.Roles {
		if roleId == TrustedRoleId {
			return true
		}
	}
	return false
}

func handleDiscordMessage(db *sql.DB, dg *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	if strings.HasPrefix(m.Content, CommandPrefix+"trust ") {
		if isMemberTrusted(m.Member) {
			for _, mention := range m.Mentions {
				mentionMember, err := dg.GuildMember(m.GuildID, mention.ID)
				if err != nil {
					log.Printf("Could not get roles of user %s: %s\n", mention.ID, err)
					continue
				}

				if !isMemberTrusted(mentionMember) {
					// TODO: do all of that in a transation that is rollbacked when GuildMemberRoleAdd fails
					// TODO: limit the amount of trusts a user can do
					_, err = db.Exec("INSERT INTO TrustLog (trusterId, trusteeId) VALUES ($1, $2);", m.Author.ID, mention.ID)
					if err != nil {
						log.Printf("Could not save a TrustLog entry\n");
						continue
					}

					err := dg.GuildMemberRoleAdd(m.GuildID, mention.ID, TrustedRoleId)
					if err != nil {
						log.Printf("Could not assign role %s to user %s: %s\n", TrustedRoleId, mention.ID, err)
						continue
					}

					dg.ChannelMessageSend(m.ChannelID, "Trusted "+mention.Username)
				}
			}
		} else {
			log.Printf("User %s is not trusted to trust others\n", m.Member.Nick);
		}
	} else if strings.HasPrefix(m.Content, CommandPrefix+"ping") {
		dg.ChannelMessageSend(m.ChannelID, "Pong")
	} else {
		log.Printf("%s is not a trust command", m.Content)
	}
}

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

func main() {
	discordToken := LookupEnvOrDie("TREE1984_DISCORD_TOKEN")
	// TODO: use PostgreSQL url here
	// postgresql://[user[:password]@][netloc][:port][/dbname][?param1=value1&...]
	pgsqlConnection := LookupEnvOrDie("TREE1984_PGSQL_CONNECTION")


	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatalln("Could not start a new Discord session:", err)
	}

	// TODO: REST API for controling/monitoring the bot
	// TODO: web client for the REST API
	// TODO: migrate database

	db, err := sql.Open("postgres", pgsqlConnection)
	if err != nil {
		log.Fatalln("Could not open PostgreSQL connection:", err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatalln("PostgreSQL connection didn't respond to ping:", err)
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate){
		handleDiscordMessage(db, s, m)
	})

	dg.Identify.Intents = discordgo.IntentsGuildMessages

	err = dg.Open()
	if err != nil {
		log.Fatalln("Could not open Discord connection:", err)
	}

	server := http.Server{
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

	go func() {
		err := server.ListenAndServe()
		log.Println("HTTP server stopped:", err)
	}()

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session, the Postgres connection and the HTTP server.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Println("Could not shutdown HTTP server:", err)
	}
	if err := db.Close(); err != nil {
		log.Println("Could not close PostgreSQL connection:", err)
	}
	if err := dg.Close(); err != nil {
		log.Println("Could not close Discord connection:", err)
	}
}
