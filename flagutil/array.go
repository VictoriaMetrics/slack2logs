package flagutil

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

type ArrayString []string

// NewArrayString returns new ArrayString with the given name and description.
func NewArrayString(name, description string) *ArrayString {
	description += "\nSupports an `array` of values separated by comma or specified via multiple flags."
	var a ArrayString
	flag.Var(&a, name, description)
	return &a
}

// String implements flag.Value interface
func (a *ArrayString) String() string {
	aEscaped := make([]string, len(*a))
	for i, v := range *a {
		if strings.ContainsAny(v, `,'"{[(`+"\n") {
			v = fmt.Sprintf("%q", v)
		}
		aEscaped[i] = v
	}
	return strings.Join(aEscaped, ",")
}

// Set implements flag.Value interface
func (a *ArrayString) Set(value string) error {
	values := parseArrayValues(value)
	*a = append(*a, values...)
	return nil
}

func parseArrayValues(s string) []string {
	if len(s) == 0 {
		return nil
	}
	var values []string
	for {
		v, tail := getNextArrayValue(s)
		values = append(values, v)
		if len(tail) == 0 {
			return values
		}
		s = tail
		if s[0] == ',' {
			s = s[1:]
		}
	}
}

func getNextArrayValue(s string) (string, string) {
	v, tail := getNextArrayValueMaybeQuoted(s)
	if strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) {
		vUnquoted, err := strconv.Unquote(v)
		if err == nil {
			return vUnquoted, tail
		}
		v = v[1 : len(v)-1]
		v = strings.ReplaceAll(v, `\"`, `"`)
		v = strings.ReplaceAll(v, `\\`, `\`)
		return v, tail
	}
	if strings.HasPrefix(v, `'`) && strings.HasSuffix(v, `'`) {
		v = v[1 : len(v)-1]
		v = strings.ReplaceAll(v, `\'`, "'")
		v = strings.ReplaceAll(v, `\\`, `\`)
		return v, tail
	}
	return v, tail
}

var closeQuotes = map[byte]byte{
	'"':  '"',
	'\'': '\'',
	'[':  ']',
	'{':  '}',
	'(':  ')',
}

func getNextArrayValueMaybeQuoted(s string) (string, string) {
	idx := 0
	for {
		n := strings.IndexAny(s[idx:], `,"'[{(`)
		if n < 0 {
			// The last item
			return s, ""
		}
		idx += n
		ch := s[idx]
		if ch == ',' {
			// The next item
			return s[:idx], s[idx:]
		}
		idx++
		m := indexCloseQuote(s[idx:], closeQuotes[ch])
		idx += m
	}
}

func indexCloseQuote(s string, closeQuote byte) int {
	if closeQuote == '"' || closeQuote == '\'' {
		idx := 0
		for {
			n := strings.IndexByte(s[idx:], closeQuote)
			if n < 0 {
				return 0
			}
			idx += n
			if n := getTrailingBackslashesCount(s[:idx]); n%2 == 1 {
				// The quote is escaped with backslash. Skip it
				idx++
				continue
			}
			return idx + 1
		}
	}
	idx := 0
	for {
		n := strings.IndexAny(s[idx:], `"'[{()}]`)
		if n < 0 {
			return 0
		}
		idx += n
		ch := s[idx]
		if ch == closeQuote {
			return idx + 1
		}
		idx++
		m := indexCloseQuote(s[idx:], closeQuotes[ch])
		if m == 0 {
			return 0
		}
		idx += m
	}
}

func getTrailingBackslashesCount(s string) int {
	n := len(s)
	for n > 0 && s[n-1] == '\\' {
		n--
	}
	return len(s) - n
}

// GetOptionalArg returns optional arg under the given argIdx.
func (a *ArrayString) GetOptionalArg(argIdx int) string {
	x := *a
	if argIdx >= len(x) {
		if len(x) == 1 {
			return x[0]
		}
		return ""
	}
	return x[argIdx]
}
