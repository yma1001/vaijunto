package cliui

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Prompter lê linhas do menu (LF, CRLF ou CR sozinho) e escreve o prompt em out.
type Prompter struct {
	sc  *bufio.Scanner
	out io.Writer
	EOF bool
}

// NewPrompter liga stdin/stdout (ou buffers nos testes) ao leitor de menu.
func NewPrompter(in io.Reader, out io.Writer) *Prompter {
	sc := bufio.NewScanner(in)
	sc.Split(scanPromptLine)
	return &Prompter{sc: sc, out: out}
}

// Read imprime o prompt e devolve a linha normalizada. EOF marca p.EOF para o menu sair.
func (p *Prompter) Read(prompt string) string {
	fmt.Fprint(p.out, prompt)
	if !p.sc.Scan() {
		p.EOF = true
		return ""
	}
	return NormalizeLine(p.sc.Text())
}

// NormalizeLine remove NUL (stdin UTF-16 no Windows) e espaços, inclusive \r.
func NormalizeLine(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	return strings.TrimSpace(s)
}

// scanPromptLine aceita LF, CRLF e CR sozinho (Windows/Linux no mesmo menu).
func scanPromptLine(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i := 0; i < len(data); i++ {
		switch data[i] {
		case '\n':
			return i + 1, dropCR(data[:i]), nil
		case '\r':
			if i+1 < len(data) {
				if data[i+1] == '\n' {
					return i + 2, data[:i], nil
				}
				return i + 1, data[:i], nil
			}
			if atEOF {
				return i + 1, data[:i], nil
			}
			// Espera um LF que ainda pode chegar (Windows CRLF partido em dois reads).
			return 0, nil, nil
		}
	}
	if atEOF {
		return len(data), dropCR(data), nil
	}
	return 0, nil, nil
}

// dropCR tira o CR final de uma linha LF (caso Windows CRLF já partido).
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[:len(data)-1]
	}
	return data
}
