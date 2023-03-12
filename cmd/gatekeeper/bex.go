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
	ExprVar
	ExprFuncall
)

type Expr struct {
	Type ExprType
	AsInt int
	AsStr string
	AsVar string
	AsFuncall *Funcall
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
	case ExprVar:
		fmt.Printf("Var: %s\n", expr.AsVar);
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
	case ExprVar: return fmt.Sprintf("%s", expr.AsVar)
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
	fmt.Fprintf(&result, "%s(", funcall.Name)
	for i, arg := range funcall.Args {
		if i > 0 {
			fmt.Fprintf(&result, ", ")
		}
		fmt.Fprintf(&result, "%s", arg.String())
	}
	fmt.Fprintf(&result, ")")
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

func parseExpr(sourceRunes []rune) ([]rune, Expr, error) {
	sourceRunes = trimRunes(sourceRunes)
	expr := Expr{}
	if len(sourceRunes) > 0 {
		if sourceRunes[0] == '"' {
			expr.Type = ExprStr
			sourceRunes = sourceRunes[1:]
			// TODO: parseExpr does not support string escaping
			literalRunes, restRune := spanRunes(sourceRunes, func(x rune) bool {return x != '"'})
			if len(restRune) <= 0 && restRune[0] != '"' {
				return restRune, expr, errors.New("Expected \"")
			}
			sourceRunes = restRune[1:]
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
			sourceRunes = trimRunes(restRunes)

			if len(sourceRunes) > 0 && sourceRunes[0] == '(' {
				expr.Type = ExprFuncall
				expr.AsFuncall = &Funcall{
					Name: string(name),
				}

				for {
					sourceRunes = sourceRunes[1:]
					restRunes, arg, err := parseExpr(sourceRunes)
					if err != nil {
						return restRunes, arg, err
					}
					sourceRunes = restRunes
					expr.AsFuncall.Args = append(expr.AsFuncall.Args, arg)

					sourceRunes = trimRunes(sourceRunes)
					if len(sourceRunes) <= 0 || sourceRunes[0] != ',' {
						break
					}
				}

				if len(sourceRunes) <= 0 || sourceRunes[0] != ')' {
					return sourceRunes, expr, errors.New("Expected )")
				}

				return sourceRunes[1:], expr, nil
			} else {
				expr.Type = ExprVar
				expr.AsVar = string(name)
				return sourceRunes, expr, nil
			}
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
	Vars map[string]Expr
	Funcs map[string]Func
}

type EvalContext struct {
	Scopes []EvalScope
}

func (context *EvalContext) LookUpVar(name string) (Expr, bool) {
	scopes := context.Scopes
	for len(scopes) > 0 {
		n := len(scopes)
		varr, ok := scopes[n-1].Vars[name]
		if ok {
			return varr, true
		}
		scopes = scopes[:n-1]
	}
	return Expr{}, false
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
	switch expr.Type {
	case ExprVoid, ExprInt, ExprStr:
		return expr, nil
	case ExprVar:
		val, ok := context.LookUpVar(expr.AsVar)
		if !ok {
			return Expr{}, errors.New(fmt.Sprintf("Unknown variable `%s`", expr.AsVar))
		}
		return context.EvalExpr(val)
	case ExprFuncall:
		fun, ok := context.LookUpFunc(expr.AsFuncall.Name)
		if !ok {
			return Expr{}, errors.New(fmt.Sprintf("Unknown function `%s`", expr.AsFuncall.Name))
		}
		return fun(context, expr.AsFuncall.Args)
	}
	panic("unreachable")
}
