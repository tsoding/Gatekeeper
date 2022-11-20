package main

import (
	"strings"
	"log"
	"fmt"
	"os"
	"crypto/tls"
	"time"
	"database/sql"
	"github.com/tsoding/gatekeeper/internal"
)

const (
	BotAdminTwitchHandle = "Tsoding"
)

// https://dev.twitch.tv/docs/irc#connecting-to-the-twitch-irc-server
const (
	TwitchIrcAddress = "irc.chat.twitch.tv:6697"
	TwitchIrcChannel = "#tsoding"
)

type TwitchConnState int
const (
	TwitchConnect TwitchConnState = iota
	TwitchLogin
	TwitchJoin
	TwitchChat
)

type TwitchEnvironment struct {
	AuthorHandle string
	Conn *tls.Conn
	Channel string
}

func (env *TwitchEnvironment) AsDiscord() *DiscordEnvironment {
	return nil
}

func (env *TwitchEnvironment) AtAdmin() string {
	return "@"+BotAdminTwitchHandle
}

func (env *TwitchEnvironment) AtAuthor() string {
	if len(env.AuthorHandle) > 0 {
		return "@"+env.AuthorHandle
	}
	return ""
}

func (env *TwitchEnvironment) IsAuthorAdmin() bool {
	return strings.ToUpper(env.AuthorHandle) == strings.ToUpper(BotAdminTwitchHandle)
}

func (env *TwitchEnvironment) SendMessage(message string) {
	message = ". "+FilterTrailingForbidden(message);
	msg := IrcMsg{Name: IrcCmdPrivmsg, Args: []string{env.Channel, message}}
	err := msg.Send(env.Conn)
	if err != nil {
		log.Println("Error sending Twitch message \"%s\" for channel %s: %s", message, env.Channel, err)
	}
}

type TwitchConn struct {
	State TwitchConnState
	Reconnected int
	Nick string
	Pass string
	Conn *tls.Conn
	Quit chan int
	Incoming chan IrcMsg
	IncomingQuit chan int
}

func (conn *TwitchConn) Close() {
	// TODO: How to make this block until it's handled?
	conn.Quit <- 69
}

func twitchIncomingLoop(twitchConn *TwitchConn) {
	reply := make([]byte, 2048)
	for {
		n, err := twitchConn.Conn.Read(reply)
		if err != nil {
			log.Println("Could not read the reply:", err)
			break
		}

		lines := strings.Split(string(reply[0:n]), "\n")
		for _, line := range lines {
			line = strings.TrimSuffix(line, "\r")
			if len(line) == 0 {
				continue
			}
			msg, ok := ParseIrcMsg(line)
			if !ok {
				// TODO: we should probably restart the connection if parsing commands failed too many times
				log.Printf("Failed to parse command: |%s| %d\n", line, len(line))
				continue
			}
			twitchConn.Incoming <- msg
		}
	}

	twitchConn.IncomingQuit <- 69
}

// `granum` stands for `Grammatical Number`: https://en.wikipedia.org/wiki/Grammatical_number
func granum(amount int, singular string, plural string) string {
	if amount == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", amount, plural)
}

func startTwitch(db *sql.DB) (*TwitchConn, bool) {
	twitchConn := TwitchConn{
		Quit: make(chan int),
		Incoming: make(chan IrcMsg),
		IncomingQuit: make(chan int),
	}

	twitchConn.Nick = os.Getenv("GATEKEEPER_TWITCH_IRC_NICK");
	if twitchConn.Nick == "" {
		log.Println("No GATEKEEPER_TWITCH_IRC_NICK envar is provided.")
		return nil, false
	}

	twitchConn.Pass = os.Getenv("GATEKEEPER_TWITCH_IRC_PASS");
	if twitchConn.Pass == "" {
		log.Println("No GATEKEEPER_TWITCH_IRC_PASS envar is provided.")
		return nil, false
	}

	go func() {
		for {
			switch twitchConn.State {
			case TwitchConnect:
				// TODO: Can't ^C when the bot keeps reconnecting
				// Can be reproduced by turning off the Internet
				if twitchConn.Conn != nil {
					twitchConn.Conn.Close()
					twitchConn.Conn = nil
				}

				if twitchConn.Reconnected > 0 {
					seconds := 1 << (twitchConn.Reconnected - 1)
					log.Printf("Waiting %s before reconnecting Twitch IRC server\n", granum(seconds, "second", "seconds"))
					time.Sleep(time.Duration(seconds) * time.Second)
				}
				twitchConn.Reconnected += 1;

				conn, err := tls.Dial("tcp", TwitchIrcAddress, nil)
				if err != nil {
					log.Println("Failed to connect to Twitch IRC server:", err)
					continue
				}
				twitchConn.Conn = conn
				twitchConn.State = TwitchLogin
			case TwitchLogin:
				err := IrcMsg{Name: IrcCmdPass, Args: []string{"oauth:"+twitchConn.Pass}}.Send(twitchConn.Conn)
				if err != nil {
					log.Println(err)
				}
				err = IrcMsg{Name: IrcCmdNick, Args: []string{twitchConn.Nick}}.Send(twitchConn.Conn)
				if err != nil {
					log.Println(err)
				}
				twitchConn.State = TwitchJoin
				// TODO: check for authentication failures
				// Reconnection is pointless. Abandon Twitch service at all.
				go twitchIncomingLoop(&twitchConn)
			case TwitchJoin:
				select {
				case <-twitchConn.IncomingQuit:
					twitchConn.State = TwitchConnect
					continue
				case msg := <-twitchConn.Incoming:
					switch msg.Name {
					case IrcCmd001:
						// > 001 is a welcome event, so we join channels there
						// Source: https://github.com/go-irc/irc#example
						err := IrcMsg{Name: IrcCmdJoin, Args: []string{TwitchIrcChannel}}.Send(twitchConn.Conn)
						if err != nil {
							log.Println(err)
						}
						twitchConn.State = TwitchChat
						continue
					}
				}
			case TwitchChat:
				select {
				case <-twitchConn.Quit:
					log.Println("Twitch: closing connection...")
					twitchConn.Conn.Close()
					return
				case <-twitchConn.IncomingQuit:
					twitchConn.State = TwitchConnect
					continue
				case msg := <-twitchConn.Incoming:
					switch msg.Name {
					// TODO: Handle RECONNECT command
					// https://dev.twitch.tv/docs/irc/commands#reconnect
					case IrcCmdPing:
						// Reset the Reconnected counter only after
						// some time. I think the Twitch's Keep-Alive
						// PING is a good time to reset it.
						twitchConn.Reconnected = 0
						err := IrcMsg{Name: IrcCmdPong, Args: msg.Args}.Send(twitchConn.Conn)
						if err != nil {
							log.Println(err)
							continue
						}
					case IrcCmdPrivmsg:
						// TODO: this should be probably verified at parsing
						// Each IrcCmdName should have an associated arity with it that is verified at parse/serialize.
						if len(msg.Args) != 2 {
							log.Printf("Twitch: unexpected amount of args of PRIVMSG. Expected 2, but got %d\n", len(msg.Args))
							continue
						}

						command, ok := parseCommand(msg.Args[1])
						if !ok {
							if db != nil {
								internal.FeedMessageToCarrotson(db, msg.Args[1])
							}
							continue
						}

						EvalCommand(db, command, &TwitchEnvironment{
							AuthorHandle: msg.Nick(),
							Conn: twitchConn.Conn,
							Channel: TwitchIrcChannel,
						})
					}
				}
			default: panic("unreachable")
			}
		}
	}()

	return &twitchConn, true
}
