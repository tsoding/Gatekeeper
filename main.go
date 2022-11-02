package main

import (
	"os"
	"os/signal"
	"syscall"
	"log"
	"strings"
	"github.com/bwmarrin/discordgo"
	"database/sql"
	_ "github.com/lib/pq"
)

var CommandPrefix = "$"
var TrustedRoleId = "543864981171470346"

func isMemberTrusted(member *discordgo.Member) bool {
	for _, roleId := range member.Roles {
		if roleId == TrustedRoleId {
			return true
		}
	}
	return false
}

func lookupEnvOrDie(name string) string {
	env, found := os.LookupEnv(name)
	if !found {
		log.Fatalln("Could not find ", name, " variable")
	}
	return env
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
	} else {
		log.Printf("%s is not a trust command", m.Content)
	}
}

func main() {
	discordToken := lookupEnvOrDie("TREE1984_DISCORD_TOKEN")
	// TODO: use PostgreSQL url here
	// postgresql://[user[:password]@][netloc][:port][/dbname][?param1=value1&...]
	pgsqlConnection := lookupEnvOrDie("TREE1984_PGSQL_CONNECTION")

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

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session and the Postgres connection.
	db.Close()
	dg.Close()
}
