package main

import (
	"os"
	"os/signal"
	"syscall"
	"log"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"database/sql"
	_ "github.com/lib/pq"
	"net/http"
	"context"
	"time"
	"regexp"
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

func AtID(id string) string {
	return "<@"+id+">"
}

func AtUser(user *discordgo.User) string {
	return AtID(user.ID)
}

var (
	AdminID = "180406039500292096"
	MaxTrustedTimes = 1
)

func TrustedTimesOfUser(db *sql.DB, user *discordgo.User) (int, error) {
	rows, err := db.Query("SELECT count(*) FROM TrustLog WHERE trusterId = $1", user.ID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	if rows.Next() {
		var count int
		if err := rows.Scan(&count); err != nil {
			return 0, err
		}
		return count, nil
	}

	return 0, fmt.Errorf("TrustedTimesOfUser: expected at least one row with result")
}

func handleDiscordMessage(db *sql.DB, dg *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	command, ok := parseCommand(m.Content)
	if !ok {
		return
	}

	switch command.Name {
	case "count":
		if !isMemberTrusted(m.Member) {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" Only trusted users can trust others")
			return
		}
		count, err := TrustedTimesOfUser(db, m.Author);
		if err != nil {
			log.Println("Could not get amount of trusted times:", err)
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" Something went wrong. Please ask "+AtID(AdminID)+" to check the logs")
			return
		}
		dg.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s Used %d out of %d trusts", AtUser(m.Author), count, MaxTrustedTimes))
	case "untrust":
		dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" what is done is done ( -_-)")
	case "trust":
		if !isMemberTrusted(m.Member) {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" Only trusted users can trust others")
			return
		}

		if len(m.Mentions) == 0 {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" Please ping the user you want to trust")
			return
		}

		if len(m.Mentions) > 1 {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" You can't trust several people simultaneously")
			return
		}

		mention := m.Mentions[0]

		count, err := TrustedTimesOfUser(db, m.Author);
		if err != nil {
			log.Println("Could not get amount of trusted times:", err)
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" Something went wrong. Please ask "+AtID(AdminID)+" to check the logs")
			return
		}
		if count >= MaxTrustedTimes {
			if m.Author.ID != AdminID {
				dg.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s You ran out of trusts. Used %d out of %d", AtUser(m.Author), count, MaxTrustedTimes))
				return
			} else {
				dg.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s You ran out of trusts. Used %d out of %d. But since you are the %s I'll make an exception for you.", AtUser(m.Author), count, MaxTrustedTimes, AtID(AdminID)))
			}
		}

		if mention.ID == m.Author.ID {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" On this server you can't trust yourself!")
			return
		}

		mentionMember, err := dg.GuildMember(m.GuildID, mention.ID)
		if err != nil {
			log.Printf("Could not get roles of user %s: %s\n", mention.ID, err)
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" Something went wrong. Please ask "+AtID(AdminID)+" to check the logs")
			return
		}

		if isMemberTrusted(mentionMember) {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" That member is already trusted")
			return
		}

		// TODO: do all of that in a transation that is rollbacked when GuildMemberRoleAdd fails
		// TODO: add record to trusted users table
		_, err = db.Exec("INSERT INTO TrustLog (trusterId, trusteeId) VALUES ($1, $2);", m.Author.ID, mention.ID)
		if err != nil {
			log.Printf("Could not save a TrustLog entry: %s\n", err);
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" Something went wrong. Please ask "+AtID(AdminID)+" to check the logs")
			return
		}

		err = dg.GuildMemberRoleAdd(m.GuildID, mention.ID, TrustedRoleId)
		if err != nil {
			log.Printf("Could not assign role %s to user %s: %s\n", TrustedRoleId, mention.ID, err)
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" Something went wrong. Please ask "+AtID(AdminID)+" to check the logs")
			return
		}

		dg.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s Trusted %s. Used %d out of %d trusts.", AtUser(m.Author), AtUser(mention), count+1, MaxTrustedTimes))
	case "ping":
		dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" pong")
	}
}

func main() {
	discordToken := LookupEnvOrDie("GATEKEEPER_DISCORD_TOKEN")
	// TODO: use PostgreSQL url here
	// postgresql://[user[:password]@][netloc][:port][/dbname][?param1=value1&...]
	pgsqlConnection := LookupEnvOrDie("GATEKEEPER_PGSQL_CONNECTION")

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatalln("Could not start a new Discord session:", err)
	}
	log.Println("Connected to Discord")

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
	log.Println("Connected to PostgreSQL")

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate){
		handleDiscordMessage(db, s, m)
	})

	dg.Identify.Intents = discordgo.IntentsGuildMessages

	err = dg.Open()
	if err != nil {
		log.Fatalln("Could not open Discord connection:", err)
	}

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
	if err := db.Close(); err != nil {
		log.Println("Could not close PostgreSQL connection:", err)
	}
	if err := dg.Close(); err != nil {
		log.Println("Could not close Discord connection:", err)
	}
}
