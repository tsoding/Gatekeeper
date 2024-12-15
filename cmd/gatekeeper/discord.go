package main

import (
	"github.com/bwmarrin/discordgo"
	"github.com/tsoding/gatekeeper/internal"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"log"
)

var DiscordPingRegexp = regexp.MustCompile("<@[0-9]+>")

const (
	RolesChannelId = "777109841416159264"

	PingEmojiId = "777109447600111656"
	PingedRoleId = "777108731766505472"

	AocEmojiId = "🌲"
	AocRoleId = "783548342390620201"

	IntrovertEmojiId = "👀"
	IntrovertRoleId = "791706084654055517"
)

func maskDiscordPings(message string) string {
	return DiscordPingRegexp.ReplaceAllString(message, "@[DISCORD PING REDACTED]")
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

func (env *DiscordEnvironment) AuthorUserId() string {
	return "discord#"+env.m.Author.ID
}

func (env *DiscordEnvironment) AtAuthor() string {
	return AtUser(env.m.Author)
}

func (env *DiscordEnvironment) IsAuthorAdmin() bool {
	return env.m.Author.ID == AdminID
}

func (env *DiscordEnvironment) SendMessage(message string) {
	_, err := env.dg.ChannelMessageSend(env.m.ChannelID, message)
	if err != nil {
		log.Println("Error during sending discord message", err)
	}
}

func logDiscordMessage(db *sql.DB, m *discordgo.MessageCreate) {
	_, err := db.Exec("INSERT INTO Discord_Log (message_id, user_id, user_name, text) VALUES ($1, $2, $3, $4)", m.ID, m.Author.ID, m.Author.Username, m.Content);
	if err != nil {
		log.Println("ERROR: logDiscordMessage: could not insert element", m.Author.ID, m.Author.Username, m.Content, ":", err);
		return
	}
}

func handleDiscordMessage(db *sql.DB, dg *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	logDiscordMessage(db, m);

	command, ok := parseCommand(m.Content)
	if !ok {
		if db != nil {
			internal.FeedMessageToCarrotson(db, m.Content)
		}
		return
	}

	env := &DiscordEnvironment{
		dg: dg,
		m: m,
	}
	EvalCommand(db, command, env);
}

var (
	// TODO: unhardcode these parameters (config, database, or something else)
	AdminID = "180406039500292096"
	MaxTrustedTimes = 1
	TrustedRoleId = "543864981171470346"
)

func roleOfEmoji(emoji *discordgo.Emoji) (string, bool) {
	emojiId := emoji.ID
	if emojiId == "" {
		emojiId = emoji.Name
	}
	switch emojiId {
	case PingEmojiId:      return PingedRoleId, true
	case AocEmojiId:       return AocRoleId, true
	case IntrovertEmojiId: return IntrovertRoleId, true
	default:               return "", false
	}
}

func startDiscord(db *sql.DB) (*discordgo.Session, error) {
	discordToken, found := os.LookupEnv("GATEKEEPER_DISCORD_TOKEN")
	if !found {
		return nil, fmt.Errorf("Could not find GATEKEEPER_DISCORD_TOKEN variable")
	}

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		return nil, err
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentGuildMessageReactions

	dg.AddHandler(func (s *discordgo.Session, m *discordgo.MessageReactionAdd) {
		if m.MessageReaction.ChannelID == RolesChannelId {
			roleId, found := roleOfEmoji(&m.MessageReaction.Emoji)
			if found {
				log.Println("Adding role", roleId, "to user", m.MessageReaction.UserID);
				err := s.GuildMemberRoleAdd(m.MessageReaction.GuildID, m.MessageReaction.UserID, roleId)
				if err != nil {
					log.Println("Error adding role:", err)
				}
			}
		}
	})
	dg.AddHandler(func (s *discordgo.Session, m *discordgo.MessageReactionRemove) {
		if m.MessageReaction.ChannelID == RolesChannelId {
			roleId, found := roleOfEmoji(&m.MessageReaction.Emoji)
			if found {
				log.Println("Removing role", roleId, "from user", m.MessageReaction.UserID);
				err := s.GuildMemberRoleRemove(m.MessageReaction.GuildID, m.MessageReaction.UserID, roleId)
				if err != nil {
					log.Println("Error removing role:", err)
				}
			}
		}
	})
	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate){
		handleDiscordMessage(db, s, m)
	})

	err = dg.Open()
	if err != nil {
		return nil, err
	}

	return dg, nil
}
