package main

import (
	"regexp"
	"database/sql"
	"time"
	"log"
	"fmt"
	"errors"
	"net/http"
	"net/url"
	"io/ioutil"
	"github.com/tsoding/gatekeeper/internal"
	"runtime/debug"
	"math/rand"
)

var (
	CommandPrefix = "\\$"
	CommandRegexp = regexp.MustCompile("^ *"+CommandPrefix+" *([a-zA-Z0-9\\-_]+)( +(.*))?$")
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

type Command struct {
	Name string
	Args string
}

func parseCommand(source string) (Command, bool) {
	matches := CommandRegexp.FindStringSubmatch(source)
	if len(matches) == 0 {
		return Command{}, false
	}
	return Command{
		Name: matches[1],
		Args: matches[3],
	}, true
}

type CommandEnvironment interface {
	AtAdmin() string
	AtAuthor() string
	IsAuthorAdmin() bool
	AsDiscord() *DiscordEnvironment
	SendMessage(message string)
}

type CyrillifyEnvironment struct {
	InnerEnv CommandEnvironment
}

func (env *CyrillifyEnvironment) AsDiscord() *DiscordEnvironment {
	return env.InnerEnv.AsDiscord()
}

func (env *CyrillifyEnvironment) AtAdmin() string {
	return env.InnerEnv.AtAdmin()
}

func (env *CyrillifyEnvironment) AtAuthor() string {
	return env.InnerEnv.AtAuthor()
}

func (env *CyrillifyEnvironment) IsAuthorAdmin() bool {
	return env.InnerEnv.IsAuthorAdmin()
}

func (env CyrillifyEnvironment) SendMessage(message string) {
	env.InnerEnv.SendMessage(Cyrillify(message))
}

func Cyrillify(message string) string {
	result := []rune{}
	for _, x := range []rune(message) {
		if y, ok := CyrilMap[x]; ok {
			result = append(result, y)
		} else {
			result = append(result, x)
		}
	}
	return string(result)
}

var CyrilMap = map[rune]rune {
	'a': 'д',
	'e': 'ё',
	'b': 'б',
	'h': 'н',
	'k': 'к',
	'm': 'м',
	'n': 'и',
	'o': 'ф',
	'r': 'г',
	't': 'т',
	'u': 'ц',
	'x': 'ж',
	'w': 'ш',
	'A': 'Д',
	'G': 'Б',
	'E': 'Ё',
	'N': 'И',
	'O': 'Ф',
	'R': 'Я',
	'U': 'Ц',
	'W': 'Ш',
	'X': 'Ж',
	'Y': 'У',
}

func EvalCommand(db *sql.DB, command Command, env CommandEnvironment) {
	switch command.Name {
	case "ping":
		env.SendMessage(env.AtAuthor()+" pong")
	// TODO: uncarrot discord message by its id
	case "carrot":
		if db == nil {
			// TODO: add some sort of cooldown for the @admin pings
			env.SendMessage(env.AtAuthor()+" Something went wrong with the database. Commands that require it won't work. Please ask "+env.AtAdmin()+" to check the logs")
			return
		}

		message, err := internal.CarrotsonGenerate(db, command.Args, 128)
		if err != nil {
			env.SendMessage(env.AtAuthor()+" Something went wrong. Please ask "+env.AtAdmin()+" to check the logs")
			return
		}

		env.SendMessage(env.AtAuthor()+" "+maskDiscordPings(message))
	case "profile":
		if !env.IsAuthorAdmin() {
			env.SendMessage(env.AtAuthor()+" only for "+env.AtAdmin());
			return
		}

		innerCommand, ok := parseCommand(command.Args)
		if !ok {
			env.SendMessage(env.AtAuthor()+" failed to parse inner command")
			return
		}
		start := time.Now()
		EvalCommand(db, innerCommand, env);
		elapsed := time.Since(start)
		env.SendMessage(env.AtAuthor()+" `"+command.Args+"` took "+elapsed.String()+" to executed")
	case "cyril":
		innerCommand, ok := parseCommand(command.Args)
		if !ok {
			env.SendMessage(Cyrillify(command.Args))
		} else {
			EvalCommand(db, innerCommand, &CyrillifyEnvironment{
				InnerEnv: env,
			})
		}
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

		env.SendMessage(env.AtAuthor()+" "+response)
	case "version":
		env.SendMessage(env.AtAuthor()+" "+Commit)
	case "untrust":
		env.SendMessage(env.AtAuthor()+" what is done is done ( -_-)")
	case "count":
		if db == nil {
			env.SendMessage(env.AtAuthor()+" Something went wrong with the database. Commands that require it won't work. Please ask "+env.AtAdmin()+" to check the logs")
			return
		}

		discordEnv := env.AsDiscord()
		if discordEnv == nil {
			env.SendMessage(env.AtAuthor()+" This command only works in Discord, sorry")
			return
		}

		if !isMemberTrusted(discordEnv.m.Member) {
			env.SendMessage(env.AtAuthor()+" Only trusted users can trust others")
			return
		}
		count, err := TrustedTimesOfUser(db, discordEnv.m.Author);
		if err != nil {
			log.Println("Could not get amount of trusted times:", err)
			env.SendMessage(env.AtAuthor()+" Something went wrong. Please ask "+env.AtAdmin()+" to check the logs")
			return
		}
		env.SendMessage(fmt.Sprintf("%s Used %d out of %d trusts", env.AtAuthor(), count, MaxTrustedTimes))
	case "trust":
		if db == nil {
			env.SendMessage(env.AtAuthor()+" Something went wrong with the database. Commands that require it won't work. Please ask "+env.AtAdmin()+" to check the logs")
			return
		}

		discordEnv := env.AsDiscord()
		if discordEnv == nil {
			env.SendMessage(env.AtAuthor()+" This command only works in Discord, sorry")
			return
		}

		if !isMemberTrusted(discordEnv.m.Member) {
			env.SendMessage(env.AtAuthor()+" Only trusted users can trust others")
			return
		}

		if len(discordEnv.m.Mentions) == 0 {
			env.SendMessage(env.AtAuthor()+" Please ping the user you want to trust")
			return
		}

		if len(discordEnv.m.Mentions) > 1 {
			env.SendMessage(env.AtAuthor()+" You can't trust several people simultaneously")
			return
		}

		mention := discordEnv.m.Mentions[0]

		count, err := TrustedTimesOfUser(db, discordEnv.m.Author);
		if err != nil {
			log.Println("Could not get amount of trusted times:", err)
			env.SendMessage(env.AtAuthor()+" Something went wrong. Please ask "+env.AtAdmin()+" to check the logs")
			return
		}
		if count >= MaxTrustedTimes {
			if !env.IsAuthorAdmin() {
				env.SendMessage(fmt.Sprintf("%s You ran out of trusts. Used %d out of %d", env.AtAuthor(), count, MaxTrustedTimes))
				return
			} else {
				env.SendMessage(fmt.Sprintf("%s You ran out of trusts. Used %d out of %d. But since you are the %s I'll make an exception for you.", env.AtAuthor(), count, MaxTrustedTimes, env.AtAdmin()))
			}
		}

		if mention.ID == discordEnv.m.Author.ID {
			env.SendMessage(env.AtAuthor()+" On this server you can't trust yourself!")
			return
		}

		mentionMember, err := discordEnv.dg.GuildMember(discordEnv.m.GuildID, mention.ID)
		if err != nil {
			log.Printf("Could not get roles of user %s: %s\n", mention.ID, err)
			env.SendMessage(env.AtAuthor()+" Something went wrong. Please ask "+env.AtAdmin()+" to check the logs")
			return
		}

		if isMemberTrusted(mentionMember) {
			env.SendMessage(env.AtAuthor()+" That member is already trusted")
			return
		}

		// TODO: do all of that in a transation that is rollbacked when GuildMemberRoleAdd fails
		// TODO: add record to trusted users table
		_, err = db.Exec("INSERT INTO TrustLog (trusterId, trusteeId) VALUES ($1, $2);", discordEnv.m.Author.ID, mention.ID)
		if err != nil {
			log.Printf("Could not save a TrustLog entry: %s\n", err);
			env.SendMessage(env.AtAuthor()+" Something went wrong. Please ask "+env.AtAdmin()+" to check the logs")
			return
		}

		err = discordEnv.dg.GuildMemberRoleAdd(discordEnv.m.GuildID, mention.ID, TrustedRoleId)
		if err != nil {
			log.Printf("Could not assign role %s to user %s: %s\n", TrustedRoleId, mention.ID, err)
			env.SendMessage(env.AtAuthor()+" Something went wrong. Please ask "+env.AtAdmin()+" to check the logs")
			return
		}

		env.SendMessage(fmt.Sprintf("%s Trusted %s. Used %d out of %d trusts.", env.AtAuthor(), AtUser(mention), count+1, MaxTrustedTimes))
	case "mine":
		if env.AsDiscord() == nil {
			env.SendMessage(env.AtAuthor()+" This command only works in Discord, sorry")
			return
		}

		// TODO: make the field size customizable via the command parameters
		var seed string
		if len(command.Args) > 0 {
			seed = command.Args
		} else {
			seed = randomMinesweeperSeed()
		}

		r := rand.New(seedAsSource(seed))
		env.SendMessage(renderMinesweeperFieldForDiscord(randomMinesweeperField(r), seed));
	case "mineopen":
		if env.AsDiscord() == nil {
			env.SendMessage(env.AtAuthor()+" This command only works in Discord, sorry")
			return
		}

		if len(command.Args) == 0 {
			env.SendMessage(env.AtAuthor()+" please provide the seed")
			return
		}

		seed := command.Args
		r := rand.New(seedAsSource(seed))
		env.SendMessage(renderOpenMinesweeperFieldForDiscord(randomMinesweeperField(r), seed))
	case "code":
		env.SendMessage(fmt.Sprintf("%s `%s`", env.AtAuthor(), command.Args))
	case "ignore":
	default:
		env.SendMessage(fmt.Sprintf("%s command `%s` does not exist", env.AtAuthor(), command.Name))
	}
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
