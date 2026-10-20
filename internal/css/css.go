// Package css implements a small parser for the CSS subset used by ostyle
// theme files: :root rules with custom properties, @word/@ppt at-rules
// containing declarations and nested element rules, strings, and comments.
package css

import (
	"fmt"
	"strings"
)

type TokenType int

const (
	TokIdent TokenType = iota
	TokString
	TokLBrace
	TokRBrace
	TokColon
	TokSemicolon
	TokLParen
	TokRParen
	TokEOF
)

type Token struct {
	Type TokenType
	Text string
	Line int
}

// Decl is a single `prop: value;` declaration.
type Decl struct {
	Prop   string
	Value  string
	Tokens []Token
	Line   int
}

// Rule is a `selector { decls }` rule such as `:root` or `h1`.
type Rule struct {
	Selector string
	Decls    []Decl
	Line     int
}

// AtRule is an `@word { ... }` or `@ppt { ... }` block. It may contain bare
// declarations (e.g. background) and/or nested rules (e.g. :root, h1).
type AtRule struct {
	Name  string
	Decls []Decl
	Rules []Rule
	Line  int
}

type Stylesheet struct {
	Rules   []Rule
	AtRules []AtRule
}

func isWordChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == '-' || c == '_' || c == '.' || c == '#' || c == '%' || c == '+' || c == '/' || c == '@'
}

func Lex(src string) ([]Token, error) {
	var toks []Token
	line := 1
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, fmt.Errorf("line %d: unterminated comment", line)
			}
			line += strings.Count(src[i+2:i+2+end], "\n")
			i += 2 + end + 2
		case isWordChar(c):
			j := i
			for j < len(src) && isWordChar(src[j]) {
				j++
			}
			toks = append(toks, Token{TokIdent, src[i:j], line})
			i = j
		case c == '"':
			j := i + 1
			var b strings.Builder
			for j < len(src) && src[j] != '"' {
				if src[j] == '\\' && j+1 < len(src) {
					j++
				}
				if src[j] == '\n' {
					line++
				}
				b.WriteByte(src[j])
				j++
			}
			if j >= len(src) {
				return nil, fmt.Errorf("line %d: unterminated string", line)
			}
			toks = append(toks, Token{TokString, b.String(), line})
			i = j + 1
		case c == '{':
			toks = append(toks, Token{TokLBrace, "{", line})
			i++
		case c == '}':
			toks = append(toks, Token{TokRBrace, "}", line})
			i++
		case c == ':':
			toks = append(toks, Token{TokColon, ":", line})
			i++
		case c == ';':
			toks = append(toks, Token{TokSemicolon, ";", line})
			i++
		case c == '(':
			toks = append(toks, Token{TokLParen, "(", line})
			i++
		case c == ')':
			toks = append(toks, Token{TokRParen, ")", line})
			i++
		default:
			return nil, fmt.Errorf("line %d: unexpected character %q", line, string(c))
		}
	}
	return toks, nil
}

type parser struct {
	toks []Token
	pos  int
}

func (p *parser) peek() *Token {
	if p.pos >= len(p.toks) {
		return nil
	}
	return &p.toks[p.pos]
}

func (p *parser) next() Token {
	if p.pos >= len(p.toks) {
		line := 0
		if len(p.toks) > 0 {
			line = p.toks[len(p.toks)-1].Line
		}
		return Token{Type: TokEOF, Line: line}
	}
	t := p.toks[p.pos]
	p.pos++
	return t
}

func (p *parser) expect(tt TokenType, what string) (Token, error) {
	t := p.next()
	if t.Type != tt {
		return t, fmt.Errorf("line %d: expected %s", t.Line, what)
	}
	return t, nil
}

func Parse(src string) (*Stylesheet, error) {
	toks, err := Lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	ss := &Stylesheet{}
	for p.peek() != nil {
		t := p.peek()
		switch {
		case t.Type == TokIdent && strings.HasPrefix(t.Text, "@"):
			ar, err := p.atRule()
			if err != nil {
				return nil, err
			}
			ss.AtRules = append(ss.AtRules, ar)
		case t.Type == TokIdent || t.Type == TokColon:
			r, err := p.rule()
			if err != nil {
				return nil, err
			}
			ss.Rules = append(ss.Rules, r)
		default:
			return nil, fmt.Errorf("line %d: expected selector or @rule", t.Line)
		}
	}
	return ss, nil
}

func (p *parser) atRule() (AtRule, error) {
	nameTok := p.next()
	name := strings.ToLower(strings.TrimPrefix(nameTok.Text, "@"))
	if _, err := p.expect(TokLBrace, "'{' after @"+name); err != nil {
		return AtRule{}, err
	}
	ar := AtRule{Name: name, Line: nameTok.Line}
	for {
		t := p.peek()
		if t == nil {
			return ar, fmt.Errorf("line %d: missing '}' to close @%s", nameTok.Line, name)
		}
		if t.Type == TokRBrace {
			p.next()
			break
		}
		if t.Type == TokColon || (t.Type == TokIdent && p.pos+1 < len(p.toks) && p.toks[p.pos+1].Type == TokLBrace) {
			r, err := p.rule()
			if err != nil {
				return ar, err
			}
			ar.Rules = append(ar.Rules, r)
		} else if t.Type == TokIdent {
			d, err := p.decl()
			if err != nil {
				return ar, err
			}
			ar.Decls = append(ar.Decls, d)
		} else {
			return ar, fmt.Errorf("line %d: expected declaration or selector inside @%s", t.Line, name)
		}
	}
	return ar, nil
}

func (p *parser) rule() (Rule, error) {
	sel, line, err := p.selector()
	if err != nil {
		return Rule{}, err
	}
	if _, err := p.expect(TokLBrace, "'{' after selector "+sel); err != nil {
		return Rule{}, err
	}
	r := Rule{Selector: sel, Line: line}
	for {
		t := p.peek()
		if t == nil {
			return r, fmt.Errorf("line %d: missing '}' after %s", line, sel)
		}
		if t.Type == TokRBrace {
			p.next()
			break
		}
		if t.Type != TokIdent {
			return r, fmt.Errorf("line %d: expected declaration in %s", t.Line, sel)
		}
		d, err := p.decl()
		if err != nil {
			return r, err
		}
		r.Decls = append(r.Decls, d)
	}
	return r, nil
}

func (p *parser) selector() (string, int, error) {
	t := p.next()
	if t.Type == TokColon {
		id, err := p.expect(TokIdent, "pseudo-class name after ':'")
		if err != nil {
			return "", t.Line, err
		}
		return ":" + strings.ToLower(id.Text), t.Line, nil
	}
	if t.Type != TokIdent {
		return "", t.Line, fmt.Errorf("line %d: expected selector", t.Line)
	}
	return strings.ToLower(t.Text), t.Line, nil
}

func (p *parser) decl() (Decl, error) {
	propTok := p.next()
	if _, err := p.expect(TokColon, "':' after property "+propTok.Text); err != nil {
		return Decl{}, err
	}
	d := Decl{Prop: strings.ToLower(propTok.Text), Line: propTok.Line}
	for {
		t := p.peek()
		if t == nil {
			break
		}
		if t.Type == TokSemicolon {
			p.next()
			break
		}
		if t.Type == TokRBrace {
			break
		}
		d.Tokens = append(d.Tokens, p.next())
	}
	d.Value = JoinValue(d.Tokens)
	return d, nil
}

// JoinValue renders token values back to text: strings re-quoted, parens tight.
func JoinValue(toks []Token) string {
	var parts []string
	for _, t := range toks {
		switch t.Type {
		case TokString:
			parts = append(parts, `"`+t.Text+`"`)
		default:
			parts = append(parts, t.Text)
		}
	}
	s := strings.Join(parts, " ")
	s = strings.ReplaceAll(s, "( ", "(")
	s = strings.ReplaceAll(s, " )", ")")
	s = strings.ReplaceAll(s, " (", "(")
	return strings.TrimSpace(s)
}

// StringValue returns the value of a single-string declaration (font names).
func StringValue(d Decl) string {
	if len(d.Tokens) == 1 && d.Tokens[0].Type == TokString {
		return d.Tokens[0].Text
	}
	return strings.Trim(d.Value, `"`)
}
