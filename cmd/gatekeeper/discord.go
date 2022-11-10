package main

import (
	"github.com/bwmarrin/discordgo"
	"github.com/tsoding/gatekeeper/internal"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"runtime/debug"
	"os"
)

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

var CarrotsonWeighted = true

func handleDiscordMessage(db *sql.DB, dg *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	command, ok := parseCommand(m.Content)
	if !ok {
		if db != nil {
			internal.FeedMessageToCarrotson(db, m.Content)
		}
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
	case "unweighted":
		if m.Author.ID != AdminID {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" only for "+AtID(AdminID)+" sorry")
			return
		}

		CarrotsonWeighted = false

		dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" switched to unweighted branching")
	case "weighted":
		if m.Author.ID != AdminID {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" only for "+AtID(AdminID)+" sorry")
			return
		}

		CarrotsonWeighted = true

		dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" switched to weighted branching")
	case "isweighted":
		if CarrotsonWeighted {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" yes")
		} else {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" no")
		}
	case "carrot":
		if db == nil {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" Something went wrong with the database. Commands that require it won't work. Please ask "+AtID(AdminID)+" to check the logs")
			return
		}

		message, err := internal.CarrotsonGenerate(db, command.Args, 1024, CarrotsonWeighted)
		if err != nil {
			dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" Something went wrong. Please ask "+AtID(AdminID)+" to check the logs")
			return
		}
		dg.ChannelMessageSend(m.ChannelID, AtUser(m.Author)+" "+message)
	}
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

func startDiscord(db *sql.DB) (*discordgo.Session, error) {
	discordToken, found := os.LookupEnv("GATEKEEPER_DISCORD_TOKEN")
	if !found {
		return nil, fmt.Errorf("Could not find GATEKEEPER_DISCORD_TOKEN variable")
	}

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		return nil, err
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate){
		handleDiscordMessage(db, s, m)
	})

	err = dg.Open()
	if err != nil {
		return nil, err
	}

	return dg, nil
}
