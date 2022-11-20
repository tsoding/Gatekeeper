package main

import (
	"strings"
	"io"
	"fmt"
	"regexp"
)

type IrcCmdName string
const (
	IrcCmdPass IrcCmdName	= "PASS"
	IrcCmdNick				= "NICK"
	IrcCmdJoin				= "JOIN"
	IrcCmdPrivmsg			= "PRIVMSG"
	IrcCmdPing				= "PING"
	IrcCmdPong				= "PONG"
	IrcCmd001				= "001"
)

type IrcMsg struct {
	Prefix string
	Name IrcCmdName
	Args []string
}

// TODO: IrcMsg serialization should probably check for max IRC message length
func (msg *IrcMsg) String() (result string, ok bool) {
	var sb strings.Builder

	if len(msg.Prefix) > 0 {
		if !VerifyPrefix(msg.Prefix) {
			return
		}

		sb.WriteString(":")
		sb.WriteString(msg.Prefix)
		sb.WriteString(" ")
	}

	if !VerifyCmdName(string(msg.Name)) {
		return
	}
	sb.WriteString(string(msg.Name))

	n := len(msg.Args)
	for i := 0 ; i < n-1 ; i++ {
		if !VerifyMiddle(msg.Args[i]) {
			return
		}
		sb.WriteString(" ")
		sb.WriteString(msg.Args[i])
	}
	if n > 0 {
		if !VerifyTrailing(msg.Args[n-1]) {
			return
		}
		sb.WriteString(" :")
		sb.WriteString(msg.Args[n-1])
	}
	result = sb.String()
	ok = true
	return
}

func VerifyPrefix(prefix string) bool {
	// I don't know the exact format of prefix (RFC 1459 redirects to
	// RFC 952 which I'm too lazy to read), but here I simply assume
	// it may not contain NUL, SPACE, CR and LF, because that makes my
	// life easier.
	//
	// If it turns out that they may contain those, it's easy to fix.
	return !strings.ContainsAny(prefix, "\x00 \r\n")
}

var CmdNameRegexp = regexp.MustCompile("^([0-9]{3}|[a-zA-Z]+)$")

func VerifyCmdName(name string) bool {
	return CmdNameRegexp.MatchString(name)
}

func VerifyMiddle(middle string) bool {
	// From RFC 1459
	// <middle>   ::= <Any *non-empty* sequence of octets not including SPACE or
	//                 NUL or CR or LF, the first of which may not be ':'>
	if len(middle) == 0 {
		return false
	}
	if strings.HasPrefix(middle, ":") {
		return false
	}
	if strings.ContainsAny(middle, " \x00\r\n") {
		return false
	}
	return true
}

var TrailingForbidden = "\x00\r\n";

func FilterTrailingForbidden(s string) string {
	result := []byte{}
	for _, x := range []byte(s) {
		if strings.IndexByte(TrailingForbidden, x) < 0 {
			result = append(result, x)
		}
	}
	return string(result)
}

func VerifyTrailing(trailing string) bool {
	// From RFC 1459
	// <trailing> ::= <Any, possibly *empty*, sequence of octets not including
	//                 NUL or CR or LF>
	return !strings.ContainsAny(trailing, TrailingForbidden)
}

func ParseIrcMsg(source string) (msg IrcMsg, ok bool) {
	if strings.HasPrefix(source, ":") {
		split := strings.SplitN(source, " ", 2)
		if len(split) < 2 {
			return
		}
		msg.Prefix = strings.TrimPrefix(split[0], ":")
		if !VerifyPrefix(msg.Prefix) {
			return
		}
		source = split[1]
	}

	split := strings.SplitN(source, " ", 2)
	if len(split) < 2 {
		return
	}
	if !VerifyCmdName(split[0]) {
		return
	}
	msg.Name = IrcCmdName(split[0])
	source = split[1]

Loop:
	for len(source) > 0 {
		if strings.HasPrefix(source, ":") {
			trailing := strings.TrimPrefix(source, ":")
			if !VerifyTrailing(trailing) {
				return
			}
			msg.Args = append(msg.Args, trailing)
			break
		} else {
			split = strings.SplitN(source, " ", 2)
			switch len(split) {
			case 1:
				middle := split[0]
				if !VerifyMiddle(middle) {
					return
				}
				msg.Args = append(msg.Args, middle)
				break Loop
			case 2:
				middle := split[0]
				if !VerifyMiddle(middle) {
					return
				}
				msg.Args = append(msg.Args, middle)
				source = split[1]
			default:
				return // must be unreachable
			}
		}
	}

	ok = true
	return
}

func (msg IrcMsg) Send(writer io.Writer) error {
	msgString, ok := msg.String()
	if !ok {
		return fmt.Errorf("Could not serialize IRC message %#v", msg)
	}
	msgBytes := []byte(msgString+"\r\n")
	n, err := writer.Write(msgBytes)
	if err != nil {
		return fmt.Errorf("Could not send command %s: %w", msg.Name, err)
	}
	if n != len(msgBytes) {
		return fmt.Errorf("Command %s was not fully sent", msg.Name)
	}
	return nil
}

func (msg IrcMsg) Nick() string {
	// From RFC 1459
	// <prefix>   ::= <servername> | <nick> [ '!' <user> ] [ '@' <host> ]
	nick := strings.SplitN(msg.Prefix, "!", 2)[0]
	nick = strings.SplitN(nick, "@", 2)[0]
	return nick
}
