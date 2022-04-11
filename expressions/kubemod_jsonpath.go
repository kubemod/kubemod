/*
Licensed under the BSD 3-Clause License (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://opensource.org/licenses/BSD-3-Clause

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package expressions

import (
	"context"
	"fmt"
	"reflect"
	"text/scanner"
	"time"

	"github.com/PaesslerAG/gval"
	"github.com/kubemod/kubemod/jsonpath"
)

// NewJSONPathLanguage constructs the gval language used for the JSONPath match query.
func NewKubeModJSONPathLanguage() *gval.Language {
	// Initialize the JSONPath gval language.
	language := gval.NewLanguage(
		gval.Arithmetic(),
		gval.Bitmask(),
		gval.Text(),
		// Custom KubeMod boolean logic which **does not** treat Undefined and non-zero values as true.
		kubeModPropositionalLogic,
		gval.JSON(),
		gval.InfixOperator("in", inArray),

		gval.InfixShortCircuit("??", func(a interface{}) (interface{}, bool) {
			return a, a != false && a != nil
		}),

		gval.InfixOperator("??", func(a, b interface{}) (interface{}, error) {
			if a == false || a == nil {
				return b, nil
			}
			return a, nil
		}),

		gval.PostfixOperator("?", parseIf),

		gval.Function("date", dateGValFunction),

		jsonpath.PlaceholderExtension(),

		// Extend the language with custom functions"
		gval.Function("length", lengthGValFunction),
		gval.Function("isDefined", isDefinedGValFunction),
		gval.Function("isUndefined", isUndefinedGValFunction),
		gval.Function("isEmpty", isEmptyGValFunction),
		gval.Function("isNotEmpty", isNotEmptyGValFunction),
	)

	return &language
}

func dateGValFunction(arguments ...interface{}) (interface{}, error) {
	if len(arguments) != 1 {
		return nil, fmt.Errorf("date() expects exactly one string argument")
	}
	s, ok := arguments[0].(string)
	if !ok {
		return nil, fmt.Errorf("date() expects exactly one string argument")
	}
	for _, format := range [...]string{
		time.ANSIC,
		time.UnixDate,
		time.RubyDate,
		time.Kitchen,
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02",                         // RFC 3339
		"2006-01-02 15:04",                   // RFC 3339 with minutes
		"2006-01-02 15:04:05",                // RFC 3339 with seconds
		"2006-01-02 15:04:05-07:00",          // RFC 3339 with seconds and timezone
		"2006-01-02T15Z0700",                 // ISO8601 with hour
		"2006-01-02T15:04Z0700",              // ISO8601 with minutes
		"2006-01-02T15:04:05Z0700",           // ISO8601 with seconds
		"2006-01-02T15:04:05.999999999Z0700", // ISO8601 with nanoseconds
	} {
		ret, err := time.ParseInLocation(format, s, time.Local)
		if err == nil {
			return ret, nil
		}
	}
	return nil, fmt.Errorf("date() could not parse %s", s)
}

// gval function to add support for length().
// The function works with slices, maps and strings.
func lengthGValFunction(arguments ...interface{}) (interface{}, error) {
	if len(arguments) != 1 {
		return nil, fmt.Errorf("length() expects exactly one array, string or object argument")
	}

	return valLength(arguments[0])
}

func valLength(val interface{}) (interface{}, error) {
	switch v := val.(type) {
	case nil:
		return 0, nil
	case jsonpath.UndefinedType:
		return 0, nil
	case []interface{}:
		return len(v), nil
	case string:
		return len(v), nil
	case map[string]interface{}:
		return len(v), nil
	}

	return nil, fmt.Errorf("expected exactly one array, string or object argument")
}

// gval function to add support for isDefined().
func isDefinedGValFunction(arguments ...interface{}) (interface{}, error) {
	if len(arguments) != 1 {
		return nil, fmt.Errorf("isDefined() expects exactly one argument")
	}

	_, ok := arguments[0].(jsonpath.UndefinedType)

	return !ok, nil
}

// gval function to add support for isUndefined().
func isUndefinedGValFunction(arguments ...interface{}) (interface{}, error) {
	if len(arguments) != 1 {
		return nil, fmt.Errorf("isUndefined() expects exactly one argument")
	}

	_, ok := arguments[0].(jsonpath.UndefinedType)

	return ok, nil
}

// gval function to add support for isEmpty().
func isEmptyGValFunction(arguments ...interface{}) (interface{}, error) {
	if len(arguments) != 1 {
		return nil, fmt.Errorf("isEmpty() expects exactly one argument")
	}

	l, err := valLength(arguments[0])

	if err != nil {
		return nil, err
	}

	return l == 0, nil
}

// gval function to add support for isNotEmpty().
func isNotEmptyGValFunction(arguments ...interface{}) (interface{}, error) {
	if len(arguments) != 1 {
		return nil, fmt.Errorf("isNotEmpty() expects exactly one argument")
	}

	l, err := valLength(arguments[0])

	if err != nil {
		return nil, err
	}

	return l != 0, nil
}

func infixBoolOperator(name string, f func(a, b bool) (interface{}, error)) gval.Language {
	return gval.InfixOperator(name, func(a, b interface{}) (interface{}, error) {
		ab, aok := a.(bool)
		bb, bok := b.(bool)

		if aok && bok {
			return f(ab, bb)
		}

		if !aok {
			return nil, fmt.Errorf("unexpected operand type %T; expected bool", a)
		} else {
			return nil, fmt.Errorf("unexpected operand type %T; expected bool", b)
		}
	})
}

var kubeModPropositionalLogic = gval.NewLanguage(
	gval.PrefixOperator("!", func(c context.Context, v interface{}) (interface{}, error) {
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("unexpected operand type %T; expected bool", v)
		}
		return !b, nil
	}),

	gval.InfixShortCircuit("&&", func(a interface{}) (interface{}, bool) { return false, a == false }),
	infixBoolOperator("&&", func(a, b bool) (interface{}, error) { return a && b, nil }),
	gval.InfixShortCircuit("||", func(a interface{}) (interface{}, bool) { return true, a == true }),
	infixBoolOperator("||", func(a, b bool) (interface{}, error) { return a || b, nil }),

	infixBoolOperator("==", func(a, b bool) (interface{}, error) { return a == b, nil }),
	infixBoolOperator("!=", func(a, b bool) (interface{}, error) { return a != b, nil }),

	gval.Base(),
)

func inArray(a, b interface{}) (interface{}, error) {
	col, ok := b.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected type []interface{} for in operator but got %T", b)
	}
	for _, value := range col {
		if reflect.DeepEqual(a, value) {
			return true, nil
		}
	}
	return false, nil
}

func parseIf(c context.Context, p *gval.Parser, e gval.Evaluable) (gval.Evaluable, error) {
	a, err := p.ParseExpression(c)
	if err != nil {
		return nil, err
	}
	b := p.Const(nil)
	switch p.Scan() {
	case ':':
		b, err = p.ParseExpression(c)
		if err != nil {
			return nil, err
		}
	case scanner.EOF:
	default:
		return nil, p.Expected("<> ? <> : <>", ':', scanner.EOF)
	}
	return func(c context.Context, v interface{}) (interface{}, error) {
		x, err := e(c, v)
		if err != nil {
			return nil, err
		}
		if x == false || x == nil {
			return b(c, v)
		}
		return a(c, v)
	}, nil
}
