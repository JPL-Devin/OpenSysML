package main

import (
	"fmt"
	"github.com/Open-MBEE/Systemica/internal/core/lexer"
	"github.com/Open-MBEE/Systemica/internal/core/source"
)

func main() {
	src := "flow of Water"
	sf := source.New("test", []byte(src))
	l := lexer.New(sf)
	for {
		tok := l.Next()
		end := tok.Span.Offset + tok.Span.Len
		if end > len(src) {
			end = len(src)
		}
		fmt.Printf("Token: %v, KeywordID: '%s', Text: '%s'\n", tok.Kind, tok.KeywordID, src[tok.Span.Offset:end])
		if tok.Kind == lexer.EOF {
			break
		}
	}
}
