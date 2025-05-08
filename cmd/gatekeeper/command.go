package main

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/tsoding/gatekeeper/internal"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"runtime/debug"
	"time"
	"strings"
	"strconv"
	"math"
)

var (
	// TODO: make the CommandPrefix configurable from the database, so we can set it per instance
	CommandPrefix = "[\\$\\!]"
	CommandDef = "([a-zA-Z0-9\\-_]+)( +(.*))?"
	CommandRegexp = regexp.MustCompile("^ *("+CommandPrefix+") *"+CommandDef+"$")
	CommandNoPrefixRegexp = regexp.MustCompile("^ *"+CommandDef+"$")
	Commit        = func() string {
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
	Prefix string
	Name string
	Args string
}

func parseCommand(source string) (Command, bool) {
	matches := CommandRegexp.FindStringSubmatch(source)
	if len(matches) == 0 {
		return Command{}, false
	}
	return Command{
		Prefix: matches[1],
		Name:   matches[2],
		Args:   matches[4],
	}, true
}

type CommandEnvironment interface {
	AtAdmin() string
	AtAuthor() string
	AuthorUserId() string
	IsAuthorAdmin() bool
	AsDiscord() *DiscordEnvironment
	SendMessage(message string)
}

type CyrillifyEnvironment struct {
	InnerEnv CommandEnvironment
}

func (env *CyrillifyEnvironment) AuthorUserId() string {
	return env.InnerEnv.AuthorUserId()
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

var CyrilMap = map[rune]rune{
	'a': 'д',
	'e': 'ё',
	'b': 'б',
	'h': 'н',
	'k': 'к',
	'm': 'м',
	'n': 'п',
	'o': 'ф',
	'r': 'г',
	't': 'т',
	'u': 'ц',
	'x': 'ж',
	'w': 'ш',
	'A': 'Д',
	'G': 'Б',
	'E': 'Ё',
	'N': 'Й',
	'O': 'Ф',
	'R': 'Я',
	'U': 'Ц',
	'W': 'Ш',
	'X': 'Ж',
	'Y': 'У',
}


func fancyRune(chr rune) rune {
	if chr >= 'A' && chr <= 'Z' {
		return '𝓐' + chr - 'A'
	}

	if chr >= 'a' && chr <= 'z' {
		return '𝓪' + chr - 'a'
	}

	return chr
}

func fancyString(peasantString string) string {
	fancyRunes := make([]rune, len(peasantString))

	for i, peasantRune := range peasantString {
		fancyRunes[i] = fancyRune(peasantRune)
	}

	return string(fancyRunes)
}

var discordEmojiRegex = regexp.MustCompile(`(<:[a-zA-Z_0-9]+:[0-9]+>)`)

func fancyDiscordMessage(peasantMessage string) string {
	peasantEmojis := discordEmojiRegex.FindAllString(peasantMessage, -1)
	peasantNonEmojis := discordEmojiRegex.Split(peasantMessage, -1)

	i, j := 0, 0

	fancyResult := []string{}

	for i < len(peasantNonEmojis) {
		fancyResult = append(fancyResult, fancyString(peasantNonEmojis[i]))
		i++

		if j < len(peasantEmojis) {
			fancyResult = append(fancyResult, peasantEmojis[j])
			j++
		}
	}

	return strings.Join(fancyResult, "")
}

func EvalContextFromCommandEnvironment(env CommandEnvironment, command Command, count int64) EvalContext {
	return EvalContext{
		EvalPoints: 100,
		Scopes: []EvalScope{
			EvalScope{
				Funcs: map[string]Func{
					"count": func(context *EvalContext, args []Expr) (Expr, error) {
						return NewExprInt(int(count)), nil
					},
					"days_left_until": func(context *EvalContext, args []Expr) (Expr, error) {
						if len(args) != 1 {
							return Expr{}, fmt.Errorf("Expected 1 arguments")
						}
						result, err := context.EvalExpr(args[0])
						if err != nil {
							return Expr{}, err
						}
						if result.Type != ExprStr {
							return Expr{}, fmt.Errorf("%s is not a String. Expected a String in a format YYYY-MM-DD.", result.String())
						}
						date, err := time.Parse("2006-01-02", result.AsStr)
						if err != nil {
							return Expr{}, fmt.Errorf("`%s` is not a valid date. Expected format YYYY-MM-DD.", result.AsStr)
						}
						return NewExprInt(int(math.Ceil(date.Sub(time.Now()).Hours()/24))), nil
					},
					"twitch_or_discord": func(context *EvalContext, args []Expr) (result Expr, err error) {
						if len(args) != 2 {
							return Expr{}, fmt.Errorf("Expected 2 arguments")
						}

						if env.AsDiscord() == nil {
							result, err = context.EvalExpr(args[0])
						} else {
							result, err = context.EvalExpr(args[1])
						}
						return
					},
					"input": func(context *EvalContext, args []Expr) (Expr, error) {
						if len(args) > 0 {
							return Expr{}, fmt.Errorf("Too many arguments")
						}
						return NewExprStr(command.Args), nil
					},
					"replace": func(context *EvalContext, args[]Expr) (Expr, error) {
						arity := 3;
						if len(args) != arity {
							return Expr{}, fmt.Errorf("replace: Expected %d arguments but got %d", arity, len(args))
						}

						regExpr, err := context.EvalExpr(args[0])
						if err != nil {
							return Expr{}, err
						}
						if regExpr.Type != ExprStr {
							return Expr{}, fmt.Errorf("replace: Argument 1 is expected to be %s, but got %s", ExprTypeName(ExprStr), ExprTypeName(regExpr.Type))
						}

						srcExpr, err := context.EvalExpr(args[1])
						if err != nil {
							return Expr{}, err
						}
						if srcExpr.Type != ExprStr {
							return Expr{}, fmt.Errorf("replace: Argument 2 is expected to be %s, but got %s", ExprTypeName(ExprStr), ExprTypeName(srcExpr.Type))
						}

						replExpr, err := context.EvalExpr(args[2])
						if err != nil {
							return Expr{}, err
						}
						if replExpr.Type != ExprStr {
							return Expr{}, fmt.Errorf("replace: Argument 3 is expected to be %s, but got %s", ExprTypeName(ExprStr), ExprTypeName(replExpr.Type))
						}

						reg, err := regexp.Compile(regExpr.AsStr);
						if err != nil {
							return Expr{}, fmt.Errorf("replace: Could not compile regexp `%s`: %w", regExpr.AsStr, err)
						}

						return NewExprStr(string(reg.ReplaceAll([]byte(srcExpr.AsStr), []byte(replExpr.AsStr)))), nil
					},
					"year": func(context *EvalContext, args []Expr) (Expr, error) {
						if len(args) > 0 {
							return Expr{}, fmt.Errorf("Too many arguments");
						}
						return NewExprInt(time.Now().Year()), nil
					},
					"do": func(context *EvalContext, args []Expr) (result Expr, err error) {
						for _, arg := range args {
							result, err := context.EvalExpr(arg)
							if err != nil {
								return result, err
							}
						}
						return Expr{}, nil
					},
					"concat": func(context *EvalContext, args []Expr) (Expr, error) {
						sb := strings.Builder{}
						for _, arg := range args {
							result, err := context.EvalExpr(arg)
							if err != nil {
								return result, err
							}
							switch result.Type {
							case ExprVoid:
							case ExprInt: sb.WriteString(strconv.Itoa(result.AsInt))
							case ExprStr: sb.WriteString(result.AsStr)
							case ExprFuncall: return Expr{}, fmt.Errorf("`%s` is neither String nor Integer")
							}
						}
						return NewExprStr(sb.String()), nil
					},
					"add": func(context *EvalContext, args []Expr) (Expr, error) {
						sum := 0
						for _, arg := range args {
							result, err := context.EvalExpr(arg)
							if err != nil {
								return result, err
							}
							if result.Type != ExprInt {
								return Expr{}, fmt.Errorf("%s is not an integer", result.String())
							}
							sum += result.AsInt
						}
						return NewExprInt(sum), nil
					},
					"sub": func(context *EvalContext, args []Expr) (Expr, error) {
						if len(args) == 0 {
							return NewExprInt(0), nil
						}
						first, err := context.EvalExpr(args[0])
						if err != nil {
							return first, err
						}
						if first.Type != ExprInt {
							return Expr{}, fmt.Errorf("%s is not an integer", first.String())
						}
						if len(args) == 1 {
							return NewExprInt(-first.AsInt), nil
						}
						sum := first.AsInt
						for _, arg := range args[1:] {
							result, err := context.EvalExpr(arg)
							if err != nil {
								return result, err
							}
							if result.Type != ExprInt {
								return Expr{}, fmt.Errorf("%s is not an integer", result.String())
							}
							sum -= result.AsInt
						}
						return NewExprInt(sum), nil
					},
					"author": func(context *EvalContext, args []Expr) (Expr, error) {
						if len(args) > 0 {
							return Expr{}, fmt.Errorf("Too many arguments");
						}
						return NewExprStr(env.AtAuthor()), nil
					},
					"or": func(context *EvalContext, args []Expr) (Expr, error) {
						for _, arg := range args {
							result, err := context.EvalExpr(arg)
							if err != nil {
								return Expr{}, err
							}
							switch result.Type {
							case ExprInt:
								if result.AsInt != 0 {
									return result, nil
								}
							case ExprStr:
								if len(result.AsStr) != 0 {
									return result, nil
								}
							case ExprFuncall:
								return result, nil
							}
						}
						return Expr{}, nil
					},
					"uppercase": func(context *EvalContext, args []Expr) (Expr, error) {
						sb := strings.Builder{}
						for _, arg := range args {
							result, err := context.EvalExpr(arg)
							if err != nil {
								return Expr{}, err
							}

							switch result.Type {
							case ExprVoid:
							case ExprInt:
								sb.WriteString(strconv.Itoa(result.AsInt))
							case ExprStr:
								sb.WriteString(strings.ToUpper(result.AsStr));
							default:
								return Expr{}, fmt.Errorf("%s evaluated into %s which is neither Int, Str, nor Void. `uppercase` command cannot display that.", arg.String(), result.String());
							}
						}

						return NewExprStr(sb.String()), nil
					},
					"urlencode": func(context *EvalContext, args []Expr) (Expr, error) {
						sb := strings.Builder{}
						for _, arg := range args {
							result, err := context.EvalExpr(arg)
							if err != nil {
								return Expr{}, err
							}

							switch result.Type {
							case ExprVoid:
							case ExprInt:
								sb.WriteString(strconv.Itoa(result.AsInt))
							case ExprStr:
								sb.WriteString(result.AsStr);
							default:
								return Expr{}, fmt.Errorf("%s evaluated into %s which is neither Int, Str, nor Void. `urlencode` command cannot display that.", arg.String(), result.String());
							}
						}
						return NewExprStr(url.PathEscape(sb.String())), nil
					},
					"say": func(context *EvalContext, args []Expr) (Expr, error) {
						sb := strings.Builder{}
						for _, arg := range args {
							result, err := context.EvalExpr(arg)
							if err != nil {
								return Expr{}, err
							}

							switch result.Type {
							case ExprVoid:
							case ExprInt:
								sb.WriteString(strconv.Itoa(result.AsInt))
							case ExprStr:
								sb.WriteString(result.AsStr);
							default:
								return Expr{}, fmt.Errorf("%s evaluated into %s which is neither Int, Str, nor Void. `say` command cannot display that.", arg.String(), result.String());
							}
						}
						env.SendMessage(sb.String())
						return Expr{}, nil
					},
					"discord": func(context *EvalContext, args []Expr) (result Expr, err error) {
						if env.AsDiscord() == nil {
							env.SendMessage(env.AtAuthor() + " This command is only for discord, sorry")
							return
						}
						result, err = context.EvalExprs(args)
						return
					},
					"choice": func(context *EvalContext, args []Expr) (result Expr, err error) {
						if len(args) <= 0 {
							return Expr{}, fmt.Errorf("Can't choose among zero options")
						}
						return context.EvalExpr(args[rand.Intn(len(args))])
					},
					"let": func(context *EvalContext, args []Expr) (result Expr, err error) {
						if len(args) <= 0 {
							return Expr{}, nil
						}
						binds := args[:len(args)-1]
						body := args[len(args)-1]
						context.PushScope(EvalScope{
							Funcs: map[string]Func{},
						})
						defer context.PopScope()
						scope := &context.Scopes[len(context.Scopes)-1]
						for _, bind := range binds {
							if bind.Type != ExprFuncall {
								return Expr{}, fmt.Errorf("`%s` is not a Funcall. Bindings must be Funcalls. For example: let(x(34), y(35), say(add(x, y))).", bind.String())
							}
							value := Expr{}
							for _, arg := range bind.AsFuncall.Args {
								value, err = context.EvalExpr(arg)
								if err != nil {
									return Expr{}, err
								}
							}
							_, exists := context.LookUpFunc(bind.AsFuncall.Name)
							if exists {
								return Expr{}, fmt.Errorf("Redefinition of the let-binding `%s`", bind.AsFuncall.Name)
							}
							scope.Funcs[bind.AsFuncall.Name] = func(context *EvalContext, args []Expr) (Expr, error) {
								if len(args) > 0 {
									return Expr{}, fmt.Errorf("Let binding `%s` accepts 0 arguments, but you provided", bind.AsFuncall.Name, len(args))
								}
								return value, nil
							};
						}
						if body.Type == ExprFuncall && body.AsFuncall.Name != "do" {
							return Expr{}, fmt.Errorf("Wrap `%s` in `do(%s)`", body.String(), body.String())
						}
						return context.EvalExpr(body)
					},
					"fancy": func(context *EvalContext, args []Expr) (result Expr, err error) {
						sb := strings.Builder{}
						for _, arg := range args {
							result, err := context.EvalExpr(arg)
							if err != nil {
								return Expr{}, err
							}

							switch result.Type {
							case ExprVoid:
							case ExprInt:
								sb.WriteString(strconv.Itoa(result.AsInt))
							case ExprStr:
								if env.AsDiscord() == nil {
									sb.WriteString(fancyString(result.AsStr));
								} else {
									sb.WriteString(fancyDiscordMessage(result.AsStr));
								}
							default:
								return Expr{}, fmt.Errorf("%s evaluated into %s which is neither Int, Str, nor Void. `fancy` command cannot display that.", arg.String(), result.String());
							}
						}
						return NewExprStr(sb.String()), nil
					},
				},
			},
		},
	}
}

func EvalBuiltinCommand(db *sql.DB, command Command, env CommandEnvironment, context EvalContext) {
	switch command.Name {
	case "bottomspammers":
		discordEnv := env.AsDiscord()
		if discordEnv == nil {
			env.SendMessage(env.AtAuthor() + " This command only works in Discord, sorry")
			return
		}

		name := strings.TrimSpace(command.Args)

		if len(name) == 0 {
			rows, err := db.Query("select user_name, count(text) as count from discord_log group by user_name order by count asc limit 10");
			if err != nil {
				log.Printf("%s\n", err)
				env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
				return
			}
			defer rows.Close()

			sb := strings.Builder{}
			for index := 1; rows.Next(); index += 1 {
				var userName string
				var count int
				err := rows.Scan(&userName, &count)
				if err != nil {
					log.Printf("%s\n", err)
				} else {
					sb.WriteString(fmt.Sprintf("%d. %s (%d)\n", index, userName, count))
				}
			}
			env.SendMessage(env.AtAuthor() + " Bottom Spammers:\n"+sb.String())
		} else {
			rows, err := db.Query("select user_name, count(text) as count from discord_log where user_name = $1 group by user_name;", name);
			if err != nil {
				log.Printf("%s\n", err)
				env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
				return
			}
			defer rows.Close()

			sb := strings.Builder{}
			for rows.Next() {
				var userName string
				var count int
				err := rows.Scan(&userName, &count)
				if err != nil {
					log.Printf("%s\n", err)
				} else {
					sb.WriteString(fmt.Sprintf("%s (%d)\n", userName, count))
				}
			}
			env.SendMessage(env.AtAuthor() + " " + sb.String())
		}
	case "topspammers":
		discordEnv := env.AsDiscord()
		if discordEnv == nil {
			env.SendMessage(env.AtAuthor() + " This command only works in Discord, sorry")
			return
		}

		name := strings.TrimSpace(command.Args)

		if len(name) == 0 {
			rows, err := db.Query("select user_name, count(text) as count from discord_log group by user_name order by count desc limit 10");
			if err != nil {
				log.Printf("%s\n", err)
				env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
				return
			}
			defer rows.Close()

			sb := strings.Builder{}
			for index := 1; rows.Next(); index += 1 {
				var userName string
				var count int
				err := rows.Scan(&userName, &count)
				if err != nil {
					log.Printf("%s\n", err)
				} else {
					sb.WriteString(fmt.Sprintf("%d. %s (%d)\n", index, userName, count))
				}
			}
			env.SendMessage(env.AtAuthor() + " Top Spammers:\n"+sb.String())
		} else {
			rows, err := db.Query("select user_name, count(text) as count from discord_log where user_name = $1 group by user_name;", name);
			if err != nil {
				log.Printf("%s\n", err)
				env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
				return
			}
			defer rows.Close()

			sb := strings.Builder{}
			for rows.Next() {
				var userName string
				var count int
				err := rows.Scan(&userName, &count)
				if err != nil {
					log.Printf("%s\n", err)
				} else {
					sb.WriteString(fmt.Sprintf("%s (%d)\n", userName, count))
				}
			}
			env.SendMessage(env.AtAuthor() + " " + sb.String())
		}
	case "actualban":
		if !env.IsAuthorAdmin() {
			env.SendMessage(env.AtAuthor() + " only for " + env.AtAdmin())
			return
		}

		discordEnv := env.AsDiscord()
		if discordEnv == nil {
			env.SendMessage(env.AtAuthor() + " This command only works in Discord, sorry")
			return
		}

		prefix := strings.TrimSpace(command.Args)

		if len(prefix) == 0 {
			env.SendMessage(env.AtAuthor() + " Prefix cannot be empty")
			return
		}

		st, err := discordEnv.dg.GuildMembersSearch(discordEnv.m.GuildID, prefix, 1000);
		if err != nil {
			log.Printf("%s\n", err)
			env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
			return
		}

		if len(st) == 0 {
			break
		}

		for i := range st {
			err = discordEnv.dg.GuildBanCreate(discordEnv.m.GuildID, st[i].User.ID, 0)
			if err != nil {
				log.Printf("%s\n", err)
				env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
				return
			}

			env.SendMessage(env.AtAuthor() + " " + st[i].User.Username + " is banned")
		}

		env.SendMessage(env.AtAuthor() + " Done 🙂")
	case "song":
		song := LastSongPlayed(db)
		if song != nil {
			env.SendMessage(env.AtAuthor() + " " + fmt.Sprintf("🎶 🎵 Last Song: \"%s\" by %s 🎵 🎶", song.title, song.artist))
		} else {
			env.SendMessage(env.AtAuthor() + " No song has been played so far")
		}
	case "search":
		if !env.IsAuthorAdmin() {
			env.SendMessage(env.AtAuthor() + " only for " + env.AtAdmin())
			return
		}

		discordEnv := env.AsDiscord()
		if discordEnv == nil {
			env.SendMessage(env.AtAuthor() + " This command only works in Discord, sorry")
			return
		}

		prefix := command.Args
		st, err := discordEnv.dg.GuildMembersSearch(discordEnv.m.GuildID, prefix, 1000);
		if err != nil {
			log.Printf("%s\n", err)
			env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
			return
		}

		env.SendMessage(env.AtAuthor() + " There are "+strconv.Itoa(len(st))+" members that start with "+prefix);
		if 0 < len(st) && len(st) <= 100 {
			sb := strings.Builder{}
			for _, s := range st {
				sb.WriteString(s.User.Username)
				sb.WriteString(" ")
			}
			env.SendMessage("Their names are: "+sb.String());
		}
	case "edlimit":
		env.SendMessage(fmt.Sprintf("%s Line Count: %d, Line Size: %d", env.AtAuthor(), EdLineCountLimit, EdLineSizeLimit))
		return;
	case "ed":
		userId := env.AuthorUserId()
		ed, err := LoadEdStateByUserId(db, userId)
		if err != nil {
			log.Printf("Could not load Ed_State of user %s: %s\n", userId, err)
			env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
			return
		}
		ed.ExecCommand(env, command.Args);
		err = SaveEdStateByUserId(db, userId, ed)
		if err != nil {
			log.Printf("Could not save %#v of user %s: %s\n", ed, userId, err)
			env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
			return
		}
		return
	case "showcmd":
		regexp.MustCompile("^"+CommandDef+"$")
		matches := CommandNoPrefixRegexp.FindStringSubmatch(command.Args)
		if len(matches) == 0 {
			// TODO: give more info on the syntactic error to the user
			env.SendMessage(env.AtAuthor() + " syntax error")
			return
		}

		name := matches[1]
		row := db.QueryRow("SELECT bex FROM commands WHERE name = $1", name);
		var bex string
		err := row.Scan(&bex)
		if err == sql.ErrNoRows {
			env.SendMessage(fmt.Sprintf("%s command %s does not exist", env.AtAuthor(), name))
			return
		}
		if err != nil {
			env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
			log.Printf("Error while querying command %s: %s\n", command.Name, err);
			return
		}
		env.SendMessage(fmt.Sprintf("%s %s", env.AtAuthor(), bex))
	case "addcmd":
		fallthrough
	case "updcmd":
		if !env.IsAuthorAdmin() {
			env.SendMessage(env.AtAuthor() + " only for " + env.AtAdmin())
			return
		}

		regexp.MustCompile("^"+CommandDef+"$")
		matches := CommandNoPrefixRegexp.FindStringSubmatch(command.Args)
		if len(matches) == 0 {
			// TODO: give more info on the syntactic error to the user
			env.SendMessage(env.AtAuthor() + " syntax error")
			return
		}

		name := matches[1]
		bex := matches[3]

		_, err := db.Exec("INSERT INTO Commands (name, bex) VALUES ($1, $2) ON CONFLICT (name) DO UPDATE SET bex = EXCLUDED.bex;", name, bex);
		if err != nil {
			log.Printf("Could not update command %s: %s\n", name, err)
			env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
			return
		}
		// TODO: report "added" instead of "updated" when the command didn't exist but was newly created
		env.SendMessage(fmt.Sprintf("%s command %s is updated", env.AtAuthor(), name))
	case "delcmd":
		if !env.IsAuthorAdmin() {
			env.SendMessage(env.AtAuthor() + " only for " + env.AtAdmin())
			return
		}

		regexp.MustCompile("^"+CommandDef+"$")
		matches := CommandNoPrefixRegexp.FindStringSubmatch(command.Args)
		if len(matches) == 0 {
			// TODO: give more info on the syntactic error to the user
			env.SendMessage(env.AtAuthor() + " syntax error")
			return
		}

		name := matches[1]
		_, err := db.Exec("DELETE FROM commands WHERE name = $1", name);
		if err != nil {
			env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
			log.Printf("Error while querying command %s: %s\n", command.Name, err);
			return
		}
		// TODO: report "does not exist" when the deleted command didn't exist
		env.SendMessage(fmt.Sprintf("%s deleted %s", env.AtAuthor(), name))
		return
	case "eval":
		if !env.IsAuthorAdmin() {
			env.SendMessage(env.AtAuthor() + " only for " + env.AtAdmin())
			return
		}
		exprs, err := ParseAllExprs(command.Args)
		if err != nil {
			env.SendMessage(fmt.Sprintf("%s could not parse expression `%s`: %s", env.AtAuthor(), command.Args, err))
			return
		}
		if len(exprs) == 0 {
			env.SendMessage(fmt.Sprintf("%s no expressions were provided for evaluation", env.AtAuthor()))
			return
		}
		for _, expr := range exprs {
			_, err := context.EvalExpr(expr)
			if err != nil {
				env.SendMessage(fmt.Sprintf("%s could not evaluate expression `%s`: %s", env.AtAuthor(), command.Args, err))
				return
			}
		}
	// TODO: uncarrot discord message by its id
	case "carrot":
		if db == nil {
			// TODO: add some sort of cooldown for the @admin pings
			env.SendMessage(env.AtAuthor() + " Something went wrong with the database. Commands that require it won't work. Please ask " + env.AtAdmin() + " to check the logs")
			return
		}

		message, err := internal.CarrotsonGenerate(db, command.Args, 256)
		if err != nil {
			log.Printf("%s\n", err)
			env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
			return
		}

		env.SendMessage(env.AtAuthor() + " " + maskDiscordPings(message))
	case "profile":
		if !env.IsAuthorAdmin() {
			env.SendMessage(env.AtAuthor() + " only for " + env.AtAdmin())
			return
		}

		innerCommand, ok := parseCommand(command.Args)
		if !ok {
			env.SendMessage(env.AtAuthor() + " failed to parse inner command")
			return
		}
		start := time.Now()
		EvalCommand(db, innerCommand, env)
		elapsed := time.Since(start)
		env.SendMessage(env.AtAuthor() + " `" + command.Args + "` took " + elapsed.String() + " to executed")
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
				response = "Could not find `" + place + "`"
			} else if err == SomebodyTryingToHackWeather {
				response = "Are you trying to hack me or something? ._."
			} else if err != nil {
				response = "Something went wrong while querying the weather for `" + place + "`. " + env.AtAdmin() + " please check the logs."
				log.Println("Error while checking the weather for `"+place+"`:", err)
			}
		} else {
			response = "No place is provided for the weather command"
		}

		env.SendMessage(env.AtAuthor() + " " + response)
	case "version":
		env.SendMessage(env.AtAuthor() + " " + Commit)
	case "count":
		if db == nil {
			env.SendMessage(env.AtAuthor() + " Something went wrong with the database. Commands that require it won't work. Please ask " + env.AtAdmin() + " to check the logs")
			return
		}

		discordEnv := env.AsDiscord()
		if discordEnv == nil {
			env.SendMessage(env.AtAuthor() + " This command only works in Discord, sorry")
			return
		}

		if !isMemberTrusted(discordEnv.m.Member) {
			env.SendMessage(env.AtAuthor() + " Only trusted users can trust others")
			return
		}
		count, err := TrustedTimesOfUser(db, discordEnv.m.Author)
		if err != nil {
			log.Println("Could not get amount of trusted times:", err)
			env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
			return
		}
		if count > MaxTrustedTimes {
			env.SendMessage(fmt.Sprintf("%s Used %d out of %d trusts <:tsodinSus:940724160680845373>", env.AtAuthor(), count, MaxTrustedTimes))
		} else {
			env.SendMessage(fmt.Sprintf("%s Used %d out of %d trusts", env.AtAuthor(), count, MaxTrustedTimes))
		}
		/*
	case "trust":
		if db == nil {
			env.SendMessage(env.AtAuthor() + " Something went wrong with the database. Commands that require it won't work. Please ask " + env.AtAdmin() + " to check the logs")
			return
		}

		discordEnv := env.AsDiscord()
		if discordEnv == nil {
			env.SendMessage(env.AtAuthor() + " This command only works in Discord, sorry")
			return
		}

		if !isMemberTrusted(discordEnv.m.Member) {
			env.SendMessage(env.AtAuthor() + " Only trusted users can trust others")
			return
		}

		if len(discordEnv.m.Mentions) == 0 {
			env.SendMessage(env.AtAuthor() + " Please ping the user you want to trust")
			return
		}

		if len(discordEnv.m.Mentions) > 1 {
			env.SendMessage(env.AtAuthor() + " You can't trust several people simultaneously")
			return
		}

		mention := discordEnv.m.Mentions[0]

		count, err := TrustedTimesOfUser(db, discordEnv.m.Author)
		if err != nil {
			log.Println("Could not get amount of trusted times:", err)
			env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
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
			env.SendMessage(env.AtAuthor() + " On this server you can't trust yourself!")
			return
		}

		mentionMember, err := discordEnv.dg.GuildMember(discordEnv.m.GuildID, mention.ID)
		if err != nil {
			log.Printf("Could not get roles of user %s: %s\n", mention.ID, err)
			env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
			return
		}

		if isMemberTrusted(mentionMember) {
			env.SendMessage(env.AtAuthor() + " That member is already trusted")
			return
		}

		// TODO: do all of that in a transation that is rollbacked when GuildMemberRoleAdd fails
		// TODO: add record to trusted users table
		_, err = db.Exec("INSERT INTO TrustLog (trusterId, trusteeId) VALUES ($1, $2);", discordEnv.m.Author.ID, mention.ID)
		if err != nil {
			log.Printf("Could not save a TrustLog entry: %s\n", err)
			env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
			return
		}

		err = discordEnv.dg.GuildMemberRoleAdd(discordEnv.m.GuildID, mention.ID, TrustedRoleId)
		if err != nil {
			log.Printf("Could not assign role %s to user %s: %s\n", TrustedRoleId, mention.ID, err)
			env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
			return
		}

		env.SendMessage(fmt.Sprintf("%s Trusted %s. Used %d out of %d trusts.", env.AtAuthor(), AtUser(mention), count+1, MaxTrustedTimes))
		*/
	case "mine":
		if env.AsDiscord() == nil {
			env.SendMessage(env.AtAuthor() + " This command only works in Discord, sorry")
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
		env.SendMessage(renderMinesweeperFieldForDiscord(randomMinesweeperField(r), seed))
	case "mineopen":
		if env.AsDiscord() == nil {
			env.SendMessage(env.AtAuthor() + " This command only works in Discord, sorry")
			return
		}

		if len(command.Args) == 0 {
			env.SendMessage(env.AtAuthor() + " please provide the seed")
			return
		}

		seed := command.Args
		r := rand.New(seedAsSource(seed))
		env.SendMessage(renderOpenMinesweeperFieldForDiscord(randomMinesweeperField(r), seed))
	default:
		env.SendMessage(fmt.Sprintf("%s command `%s` does not exist", env.AtAuthor(), command.Name))
	}
}

func EvalCommand(db *sql.DB, command Command, env CommandEnvironment) {
	row := db.QueryRow("SELECT bex, count FROM commands WHERE name = $1", command.Name);
	var bex string
	var count int64
	err := row.Scan(&bex, &count)
	if err == sql.ErrNoRows {
		EvalBuiltinCommand(db, command, env, EvalContextFromCommandEnvironment(env, command, 0))
		return
	}
	if err != nil {
		env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
		log.Printf("Error while querying command %s: %s\n", command.Name, err);
		return
	}

	exprs, err := ParseAllExprs(bex)
	if err != nil {
		env.SendMessage(fmt.Sprintf("%s Error while parsing `%s` command: %s", env.AtAuthor(), command.Name, err));
		return
	}

	count += 1
	context := EvalContextFromCommandEnvironment(env, command, count)

	for _, expr := range exprs {
		_, err := context.EvalExpr(expr)
		if err != nil {
			env.SendMessage(fmt.Sprintf("%s Could not evaluate command's expression `%s`: %s", env.AtAuthor(), bex, err));
			return
		}
	}

	_, err = db.Exec("UPDATE commands SET count = $1 WHERE name = $2;", count, command.Name);
	if err != nil {
		env.SendMessage(env.AtAuthor() + " Something went wrong. Please ask " + env.AtAdmin() + " to check the logs")
		log.Printf("Error while querying command %s: %s\n", command.Name, err);
		return
	}
}

var (
	PlaceNotFound               = errors.New("PlaceNotFound")
	SomebodyTryingToHackWeather = errors.New("SomebodyTryingToHackWeather")
)

func checkWeatherOf(place string) (string, error) {
	res, err := http.Get("https://wttr.in/" + url.PathEscape(place) + "?format=4")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
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
