package main

import (
	"regexp"
	"database/sql"
	"time"
	"log"
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
