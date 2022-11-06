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
	"github.com/tsoding/smig"
	"runtime/debug"
	"net/url"
	"io/ioutil"
	"errors"
	"math/rand"
)

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

var (
	PlaceNotFound = errors.New("PlaceNotFound")
	SomebodyTryingToHackWeather = errors.New("SomebodyTryingToHackWeather")
)

func checkWeatherOf(place string) (string, error) {
	res, err := http.Get("https://wttr.in/"+url.PathEscape(place)+"?format=4")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body);
	if err != nil {
		return "", err
	}

	if res.StatusCode == 404 {
		return "", PlaceNotFound
	} else if res.StatusCode == 400 {
		return "", SomebodyTryingToHackWeather
	} else if res.StatusCode > 400 {
		return "", fmt.Errorf("Unsuccesful response from wttr.in with code %d: %s", res.StatusCode, string(body))
	}

	return string(body), nil
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
	case "version":
		// TODO: Check for results of dg.ChannelMessageSend
		dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" "+Commit)
	case "count":
		if db == nil {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" Something went wrong with the database. Commands that require it won't work. Please ask "+AtID(AdminID)+" to check the logs")
			return
		}

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
		if db == nil {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" Something went wrong with the database. Commands that require it won't work. Please ask "+AtID(AdminID)+" to check the logs")
			return
		}

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
	case "weather":
		place := command.Args

		var response string
		var err error
		if len(place) > 0 {
			response, err = checkWeatherOf(place)
			if err == PlaceNotFound {
				response = "Could not find `"+place+"`"
			} else if err == SomebodyTryingToHackWeather {
				response = "Are you trying to hack me or something? ._."
			} else if err != nil {
				response = "Something went wrong while querying the weather for `"+place+"`. "+AtID(AdminID)+" please check the logs."
				log.Println("Error while checking the weather for `"+place+"`:", err)
			}
		} else {
			response = "No place is provided for "+CommandPrefix+"weather"
		}

		dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" "+response)
	case "ping":
		dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" pong")
	case "minesweeper": fallthrough
	case "mine":
		// TODO: make the field size customizable via the command parameters
		var seed string
		if len(command.Args) > 0 {
			seed = command.Args
		} else {
			seed = randomMinesweeperSeed()
		}

		r := rand.New(seedAsSource(seed))
		dg.ChannelMessageSend(m.ChannelID, renderMinesweeperFieldForDiscord(randomMinesweeperField(r), seed));
	case "mineopen":
		if len(command.Args) == 0 {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" please provide the seed")
			return
		}

		seed := command.Args
		r := rand.New(seedAsSource(seed))
		dg.ChannelMessageSend(m.ChannelID, renderOpenMinesweeperFieldForDiscord(randomMinesweeperField(r), seed))
	}
}

func migratePostgres(db *sql.DB) bool {
	log.Println("Checking if there are any migrations to apply")
	tx, err := db.Begin()
	if err != nil {
		log.Println("Error starting the migration transaction:", err)
		return false
	}

	err = smig.MigratePG(tx, "./sql/")
	if err != nil {
		log.Println("Error during the migration:", err)

		err = tx.Rollback()
		if err != nil {
			log.Println("Error rolling back the migration transaction:", err)
		}

		return false
	}

	err = tx.Commit()
	if err != nil {
		log.Println("Error during committing the transaction:", err)
		return false
	}

	log.Println("All the migrations are applied")
	return true
}

func startPostgreSQL() *sql.DB {
	pgsqlConnection, found := os.LookupEnv("GATEKEEPER_PGSQL_CONNECTION")
	if !found {
		log.Println("Could not find GATEKEEPER_PGSQL_CONNECTION variable")
		return nil
	}

	db, err := sql.Open("postgres", pgsqlConnection)
	if err != nil {
		log.Println("Could not open PostgreSQL connection:", err)
		return nil
	}

	ok := migratePostgres(db)
	if !ok {
		err := db.Close()
		if err != nil {
			log.Println("Error while closing PostgreSQL connection due to failed migration:", err)
		}
		return nil
	}

	return db
}

var (
	// TODO: unhardcode these parameters (config, database, or something else)
	AdminID = "180406039500292096"
	MaxTrustedTimes = 1
	TrustedRoleId = "543864981171470346"

	Commit = func() string {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					return setting.Value
				}
			}
		}
		return "<none>"
	}()
)

func main() {
	// PostgreSQL //////////////////////////////
	db := startPostgreSQL()

	if db == nil {
		log.Println("Starting without PostgreSQL. Commands that require it won't work.")
	}

	// Discord //////////////////////////////
	discordToken := LookupEnvOrDie("GATEKEEPER_DISCORD_TOKEN")

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatalln("Could not start a new Discord session:", err)
	}
	log.Println("Connected to Discord")

	dg.Identify.Intents = discordgo.IntentsGuildMessages

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate){
		handleDiscordMessage(db, s, m)
	})

	err = dg.Open()
	if err != nil {
		log.Fatalln("Could not open Discord connection:", err)
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
	if err := dg.Close(); err != nil {
		log.Println("Could not close Discord connection:", err)
	}
	if db != nil {
		if err := db.Close(); err != nil {
			log.Println("Could not close PostgreSQL connection:", err)
		}
	}
}
