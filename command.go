package main

import (
	"regexp"
)

var (
	CommandPrefix = "$"
	CommandRegexp = regexp.MustCompile("^ *\\"+CommandPrefix+" *([a-zA-Z0-9\\-_]+)( +(.*))?$")
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
