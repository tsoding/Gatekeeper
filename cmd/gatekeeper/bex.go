// Stolen from: https://gitlab.com/tsoding/bex/
package main

import (
	"fmt"
	"errors"
	"unicode"
	"strings"
	"strconv"
)

type ExprType int

const (
	ExprVoid ExprType = iota
	ExprInt
	ExprStr
	ExprFuncall
)

type Expr struct {
	Type ExprType
	AsInt int
	AsStr string
	AsFuncall Funcall
}

func NewExprStr(str string) Expr {
	return Expr{
		Type: ExprStr,
		AsStr: str,
	}
}

func NewExprInt(num int) Expr {
	return Expr{
		Type: ExprInt,
		AsInt: num,
	}
}

func ExprTypeName(typ ExprType) string {
	switch typ {
	case ExprVoid:
		return "Void"
	case ExprInt:
		return "Int"
	case ExprStr:
		return "Str"
	case ExprFuncall:
		return "Funcall"
	default:
		panic("unreachable")
	}
}

func (expr *Expr) Dump(level int) {
	for i := 0; i < level; i += 1 {
		fmt.Printf("  ");
	}

	switch expr.Type {
	case ExprVoid:
		fmt.Printf("Void\n");
	case ExprInt:
		fmt.Printf("Int: %d\n", expr.AsInt);
	case ExprStr:
		// TODO: Expr.Dump() does not escape strings
		fmt.Printf("Str: \"%s\"\n", expr.AsStr);
	case ExprFuncall:
		fmt.Printf("Funcall: %s\n", expr.AsFuncall.Name)
		for _, arg := range expr.AsFuncall.Args {
			arg.Dump(level + 1)
		}
	}
	panic("unreachable")
}

func (expr *Expr) String() string {
	switch expr.Type {
	case ExprVoid: return ""
	case ExprInt: return fmt.Sprintf("%d", expr.AsInt)
	// TODO: Expr.String() does not escape string
	case ExprStr: return fmt.Sprintf("\"%s\"", expr.AsStr)
	case ExprFuncall: return expr.AsFuncall.String()
	}
	panic("unreachable")
}

type Funcall struct {
	Name string
	Args []Expr
}

func (funcall *Funcall) String() string {
	var result strings.Builder
	fmt.Fprintf(&result, "%s", funcall.Name)
	if len(funcall.Args) > 0 {
		fmt.Fprintf(&result, "(")
		for i, arg := range funcall.Args {
			if i > 0 {
				fmt.Fprintf(&result, ", ")
			}
			fmt.Fprintf(&result, "%s", arg.String())
		}
		fmt.Fprintf(&result, ")")
	}
	return result.String()
}

func spanRunes(runes []rune, predicate func(rune) bool) ([]rune, []rune) {
	for i := range runes {
		if !predicate(runes[i]) {
			return runes[:i], runes[i:]
		}
	}
	return runes, []rune{}
}

func trimRunes(runes []rune) []rune {
	_, s := spanRunes(runes, unicode.IsSpace)
	return s
}

var EndOfSource = errors.New("EndOfSource")

func parseFuncallArgs(sourceRunes []rune) ([]rune, []Expr, error) {
	args := []Expr{}

	sourceRunes = trimRunes(sourceRunes)
	if !(len(sourceRunes) > 0 && sourceRunes[0] == '(') {
		return sourceRunes, args, errors.New("Expected (")
	}
	sourceRunes = sourceRunes[1:]

	sourceRunes = trimRunes(sourceRunes)
	if len(sourceRunes) <= 0 {
		return sourceRunes, args, errors.New("Expected )")
	}

	if sourceRunes[0] == ')' {
		sourceRunes = sourceRunes[1:]
		return sourceRunes, args, nil
	}

	for {
		restRunes, arg, err := parseExpr(sourceRunes)
		args = append(args, arg)
		if err != nil {
			return restRunes, args, err
		}
		sourceRunes = restRunes

		sourceRunes = trimRunes(sourceRunes)
		if len(sourceRunes) <= 0 {
			return sourceRunes, args, errors.New("Expected )")
		}

		if sourceRunes[0] == ')' {
			sourceRunes = sourceRunes[1:]
			return sourceRunes, args, nil
		}

		if sourceRunes[0] != ',' {
			return sourceRunes, args, errors.New("Expected ,")
		}
		sourceRunes = sourceRunes[1:]
		sourceRunes = trimRunes(sourceRunes)
	}

	panic("parseFuncallArgs: unreachable")
}

func parseExpr(sourceRunes []rune) ([]rune, Expr, error) {
	sourceRunes = trimRunes(sourceRunes)
	expr := Expr{}
	if len(sourceRunes) > 0 {
		if sourceRunes[0] == '"' {
			expr.Type = ExprStr
			sourceRunes = sourceRunes[1:]
			literalRunes := []rune{}
			i := 0
			span: for i < len(sourceRunes) {
				switch sourceRunes[i] {
				case '"':
					break span
				case '\\':
					i += 1
					if i >= len(sourceRunes) {
						return sourceRunes[i:], expr, errors.New("Unfinished escape sequence")
					}
					// TODO: support all common escape sequences
					switch sourceRunes[i] {
					case 'n':
						literalRunes = append(literalRunes, '\n')
						i += 1
					case '\\':
						literalRunes = append(literalRunes, '\\')
						i += 1
					case '"':
						literalRunes = append(literalRunes, '"')
						i += 1
					default:
						return sourceRunes[i:], expr, errors.New(fmt.Sprintf("Unknown escape sequence starting with `%c`", sourceRunes[i]))
					}
				default:
					literalRunes = append(literalRunes, sourceRunes[i])
					i += 1
				}
			}
			if i >= len(sourceRunes) {
				return sourceRunes[i:], expr, errors.New("Expected \"")
			}
			i += 1;
			sourceRunes = sourceRunes[i:]
			expr.AsStr = string(literalRunes)
			return sourceRunes, expr, nil
		} else if unicode.IsDigit(sourceRunes[0]) {
			expr.Type = ExprInt
			digits, restRunes := spanRunes(sourceRunes, func(x rune) bool { return unicode.IsDigit(x) })
			sourceRunes = restRunes
			val, err := strconv.ParseInt(string(digits), 10, 32)
			if err != nil {
				return sourceRunes, Expr{}, err
			}
			expr.AsInt = int(val)
			return sourceRunes, expr, nil
		} else if unicode.IsLetter(sourceRunes[0]) {
			name, restRunes := spanRunes(sourceRunes, func(x rune) bool {
				return unicode.IsLetter(x) || unicode.IsDigit(x) || x == '_'
			})
			sourceRunes = restRunes

			expr.Type = ExprFuncall
			expr.AsFuncall.Name = string(name)

			sourceRunes = trimRunes(sourceRunes)
			if len(sourceRunes) > 0 && sourceRunes[0] == '(' {
				restRunes, funcallArgs, err := parseFuncallArgs(sourceRunes)
				sourceRunes = restRunes
				expr.AsFuncall.Args = funcallArgs
				return restRunes, expr, err
			}

			return sourceRunes, expr, nil
		} else {
			return sourceRunes, Expr{}, errors.New(fmt.Sprintf("Unexpected character %q", sourceRunes[0]))
		}
	}

	return sourceRunes, Expr{}, EndOfSource
}

func ParseAllExprs(source string) ([]Expr, error) {
	sourceRunes := []rune(source)
	exprs := []Expr{}
	for {
		restRunes, expr, err := parseExpr(sourceRunes)
		if err != nil {
			if err == EndOfSource {
				err = nil
			}
			return exprs, err
		}
		sourceRunes = restRunes
		exprs = append(exprs, expr)
	}
}

type EvalScope struct {
	Funcs map[string]Func
}

type EvalContext struct {
	Scopes []EvalScope
	EvalPoints int
}

func (context *EvalContext) LookUpFunc(name string) (Func, bool) {
	scopes := context.Scopes
	for len(scopes) > 0 {
		n := len(scopes)
		fun, ok := scopes[n-1].Funcs[name]
		if ok {
			return fun, true
		}
		scopes = scopes[:n-1]
	}
	return nil, false
}

func (context *EvalContext) PushScope(scope EvalScope) {
	context.Scopes = append(context.Scopes, scope)
}

func (context *EvalContext) PopScope() {
	n := len(context.Scopes)
	context.Scopes = context.Scopes[0:n-1]
}

type Func = func(context *EvalContext, args []Expr) (Expr, error)

func (context *EvalContext) EvalExpr(expr Expr) (Expr, error) {
	if context.EvalPoints <= 0 {
		return Expr{}, errors.New(fmt.Sprintf("This expression is too complicated for you"));
	}
	context.EvalPoints -= 1;

	switch expr.Type {
	case ExprVoid, ExprInt, ExprStr:
		return expr, nil
	case ExprFuncall:
		fun, ok := context.LookUpFunc(expr.AsFuncall.Name)
		if !ok {
			return Expr{}, errors.New(fmt.Sprintf("Unknown function `%s`", expr.AsFuncall.Name))
		}
		return fun(context, expr.AsFuncall.Args)
	}
	panic("unreachable")
}

func (context *EvalContext) EvalExprs(exprs []Expr) (result Expr, err error) {
	for _, expr := range exprs {
		result, err = context.EvalExpr(expr)
		if err != nil {
			return
		}
	}
	return
}
