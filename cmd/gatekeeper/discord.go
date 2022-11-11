package main

import (
	"github.com/bwmarrin/discordgo"
	"github.com/tsoding/gatekeeper/internal"
	"database/sql"
	"fmt"
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

type DiscordEnvironment struct {
	dg *discordgo.Session
	m *discordgo.MessageCreate
}

func (env *DiscordEnvironment) AsDiscord() *DiscordEnvironment {
	return env
}

func (env *DiscordEnvironment) AtAdmin() string {
	return AtID(AdminID)
}

func (env *DiscordEnvironment) AtAuthor() string {
	return AtUser(env.m.Author)
}

func (env *DiscordEnvironment) IsAuthorAdmin() bool {
	return env.m.Author.ID == AdminID
}

func (env *DiscordEnvironment) SendMessage(message string) {
	// TODO: Check for results of dg.ChannelMessageSend
	env.dg.ChannelMessageSend(env.m.ChannelID, message)
}

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

	EvalCommand(db, command, &DiscordEnvironment{
		dg: dg,
		m: m,
	})
}

var (
	// TODO: unhardcode these parameters (config, database, or something else)
	AdminID = "180406039500292096"
	MaxTrustedTimes = 1
	TrustedRoleId = "543864981171470346"
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
