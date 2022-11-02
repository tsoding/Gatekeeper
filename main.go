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

func isTrusted(member *discordgo.Member) bool {
	for _, roleId := range member.Roles {
		if roleId == TrustedRoleId {
			return true
		}
	}
	return false
}

func main() {
	discordToken, found := os.LookupEnv("DISCORD_TOKEN")
	if !found {
		log.Fatalf("Could not find DISCORD_TOKEN variable")
	}

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatalf("Could not start a new Discord session: %s\n", err)
	}

	postgresqlConnection, found := os.LookupEnv("POSTGRESQL_CONNECTION")
	if !found {
		log.Fatalf("Could not find POSTGRESQL_CONNECTION")
	}

	db, err := sql.Open("postgres", postgresqlConnection)
	if err != nil {
		log.Fatalf("Could not open PostgreSQL connection\n", err)
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.Bot {
			return
		}

		if strings.HasPrefix(m.Content, CommandPrefix+"trust ") {
			if isTrusted(m.Member) {
				for _, mention := range m.Mentions {
					mentionMember, err := dg.GuildMember(m.GuildID, mention.ID)
					if err != nil {
						log.Printf("Could not get roles of user %s: %s\n", mention.ID, err)
						continue
					}

					if !isTrusted(mentionMember) {
						// TODO: do all of that in a transation that is rollbacked when GuildMemberRoleAdd fails
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

						s.ChannelMessageSend(m.ChannelID, "Trusted "+mention.Username)
					}
				}
			} else {
				log.Printf("User %s is not trusted to trust others\n", m.Member.Nick);
			}
		} else {
			log.Printf("%s is not a trust command", m.Content)
		}
	})

	dg.Identify.Intents = discordgo.IntentsGuildMessages

	err = dg.Open()
	if err != nil {
		log.Fatalf("Could open Discord connection %s\n", err)
	}

	// Wait here until CTRL-C or other term signal is received.
	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}
