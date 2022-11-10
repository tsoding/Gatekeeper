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
)

var (
	CommandPrefix = "\\$"
	CommandRegexp = regexp.MustCompile("^ *"+CommandPrefix+" *([a-zA-Z0-9\\-_]+)( +(.*))?$")
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
	SendMessage(message string)
}

type CyrillifyEnvironment struct {
	InnerEnv CommandEnvironment
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
	case "carrot":
		if db == nil {
			env.SendMessage(env.AtAuthor()+" Something went wrong with the database. Commands that require it won't work. Please ask "+env.AtAdmin()+" to check the logs")
			return
		}

		message, err := carrotsonGenerate(db, command.Args, 128, false)
		if err != nil {
			env.SendMessage(env.AtAuthor()+" Something went wrong. Please ask "+env.AtAdmin()+" to check the logs")
			return
		}
		env.SendMessage(env.AtAuthor()+" "+message)
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
		// TODO: disallow too many nested profiles
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
