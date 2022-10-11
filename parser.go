package css

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"github.com/gorilla/css/scanner"
	"io"
	"io/ioutil"
	"strings"
)

type tokenType int

const (
	tokenFirstToken tokenType = iota - 1
	tokenBlockStart
	tokenBlockEnd
	tokenRuleName
	tokenValue
	tokenSelector
	tokenStyleSeparator
	tokenStatementEnd
)

// Rule is a string type that represents a CSS rule.
type Rule string

type tokenEntry struct {
	value string
	token *scanner.Token
}

type tokenizer struct {
	s *scanner.Scanner
}

// Type returns the rule type, which can be a class, id or a tag.
func (rule Rule) Type() string {
	if strings.HasPrefix(string(rule), ".") {
		return "class"
	}
	if strings.HasPrefix(string(rule), "#") {
		return "id"
	}
	return "tag"
}

func (e tokenEntry) typ() tokenType {
	return newTokenType(e.value)
}

func (t *tokenizer) next() (*tokenEntry, error) {
	token := t.s.Next()
	if token == nil || token.Type == scanner.TokenEOF {
		return &tokenEntry{}, errors.New("EOF")
	}

	//if token.Type == scanner.TokenS {
	//	return nil, nil
	//}

	return &tokenEntry{
		value: token.Value,
		token: token,
	}, nil
}

func (t tokenType) String() string {
	switch t {
	case tokenBlockStart:
		return "BLOCK_START"
	case tokenBlockEnd:
		return "BLOCK_END"
	case tokenStyleSeparator:
		return "STYLE_SEPARATOR"
	case tokenStatementEnd:
		return "STATEMENT_END"
	case tokenSelector:
		return "SELECTOR"
	}
	return "VALUE"
}

func newTokenType(typ string) tokenType {
	switch typ {
	case "{":
		return tokenBlockStart
	case "}":
		return tokenBlockEnd
	case ":":
		return tokenStyleSeparator
	case ";":
		return tokenStatementEnd
	case ".", "#":
		return tokenSelector
	}
	return tokenValue
}

func newTokenizer(r io.Reader) *tokenizer {
	data, _ := ioutil.ReadAll(r)
	s := scanner.New(string(data))

	return &tokenizer{
		s: s,
	}
}

func buildList(r io.Reader) *list.List {
	l := list.New()
	t := newTokenizer(r)
	for {
		token, err := t.next()
		if err != nil {
			break
		}
		if token != nil {
			l.PushBack(token)
		}
	}

	//el := l.Front()
	//for ; el != nil; el = el.Next() {
	//	log.Println(el.Value.(*tokenEntry).token.Type, el.Value.(*tokenEntry).token.Value)
	//}

	return l
}

// TODO: rules can be comma separated
func parse(l *list.List) (map[Rule]map[string]string, error) {

	var (
		// Information about the current block that is parsed.
		rule     []string
		style    string
		value    string
		selector string

		isBlock bool

		// Parsed styles.
		css    = make(map[Rule]map[string]string)
		styles = make(map[string]string)

		// Previous token for the state machine.
		prevToken = tokenType(tokenFirstToken)
	)

	for e := l.Front(); e != nil; e = l.Front() {
		token := e.Value.(*tokenEntry)
		typ := token.typ()
		l.Remove(e)

		if token.token.Type == scanner.TokenS {
			continue
		}

		switch typ {
		case tokenValue:
			//fmt.Printf("typ: %v, value: %q, prevToken: %v\n", token.typ(), token.value, prevToken)
			switch prevToken {
			case tokenFirstToken, tokenBlockEnd:
				rule = append(rule, token.value)
			case tokenSelector:
				rule = append(rule, selector+token.value)
			case tokenBlockStart, tokenStatementEnd:
				style = token.value
			case tokenStyleSeparator:
				value = token.value
			case tokenValue:
				rule = append(rule, token.value)
			default:
				return css, fmt.Errorf("line %d: invalid syntax", token.token.Line)
			}
		case tokenSelector:
			selector = token.value
		case tokenBlockStart:
			if prevToken != tokenValue {
				return css, fmt.Errorf("line %d: block is missing rule identifier", token.token.Line)
			}
			isBlock = true
		case tokenStatementEnd:
			//fmt.Printf("prevToken: %v, style: %v, value: %v\n", prevToken, style, value)
			if prevToken != tokenValue || style == "" || value == "" {
				return css, fmt.Errorf("line %d: expected style before semicolon", token.token.Line)
			}
			styles[style] = value
		case tokenBlockEnd:
			if !isBlock {
				return css, fmt.Errorf("line %d: rule block ends without a beginning", token.token.Line)
			}
			for i := range rule {
				oldRule, ok := css[Rule(rule[i])]
				if ok {
					// merge rules
					for style, value := range oldRule {
						if _, ok := styles[style]; !ok {
							styles[style] = value
						}
					}
				}
				css[Rule(rule[i])] = styles

			}
			styles = map[string]string{}
			style, value = "", ""
			isBlock = false
		}
		prevToken = token.typ()
	}
	return css, nil
}

// Unmarshal will take a byte slice, containing sylesheet rules and return
// a map of a rules map.
func Unmarshal(b []byte) (map[Rule]map[string]string, error) {
	return parse(buildList(bytes.NewReader(b)))
}

// CSSStyle returns an error-checked parsed style, or an error if the
// style is unknown. Most of the styles are not supported yet.
func CSSStyle(name string, styles map[string]string) (Style, error) {
	value := styles[name]
	styleFn, ok := StylesTable[name]
	if !ok {
		return Style{}, errors.New("unknown style")
	}
	return styleFn(value)
}
