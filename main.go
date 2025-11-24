package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	flags "github.com/jessevdk/go-flags"
)

const returnode = byte('\n')
const defaultSeparators = " \t\v\f\r"
const defaultJoin = " "

type options struct {
	Separators string `short:"F" long:"field-separators" description:"Field separators (default: whitespaces)"`
	Join       string `short:"j" long:"join" description:"Separator string used when joining fields (default: space)"`
}

type tokenizer struct {
	br         *bufio.Reader
	buf        []byte
	tokens     [][]byte
	separators [256]bool
}

func newTokenizer(r io.Reader, seps string) *tokenizer {
	if seps == "" {
		seps = defaultSeparators
	}

	var sepSet [256]bool
	for i := 0; i < len(seps); i++ {
		sepSet[seps[i]] = true
	}

	return &tokenizer{
		br:         bufio.NewReaderSize(r, 1<<20),
		buf:        make([]byte, 0, 1024),
		tokens:     make([][]byte, 0, 1024),
		separators: sepSet,
	}
}

func (t *tokenizer) split() ([][]byte, error) {
	line, err := t.br.ReadSlice('\n')
	if err != nil && err != io.EOF {
		return nil, err
	}

	trimmedLine := bytes.TrimRight(line, "\n")

	t.tokens = t.tokens[:0]
	start := -1
	for i, c := range trimmedLine {
		if t.separators[c] {
			if start != -1 {
				t.tokens = append(t.tokens, trimmedLine[start:i])
				start = -1
			}
		} else {
			if start == -1 {
				start = i
			}
		}
	}
	if start != -1 {
		t.tokens = append(t.tokens, trimmedLine[start:])
	}

	return t.tokens, err
}

type spec struct {
	lidx  int
	ridx  int
	lopen bool
	ropen bool
}

func (s *spec) pick(tokens [][]byte) [][]byte {
	l := s.lidx
	if s.lopen {
		l = 1
	}
	if l <= 0 {
		l = len(tokens) + l + 1
	}

	r := s.ridx
	if s.ropen {
		r = len(tokens)
	}
	if r <= 0 {
		r = len(tokens) + r + 1
	}

	if r > len(tokens) {
		r = len(tokens)
	}

	var reverse bool
	if l > r {
		l, r = r, l
		reverse = true
	}

	f := tokens[l-1 : r]
	if reverse {
		slices.Reverse(f)
	}
	return f
}

func genSpecs(args []string) ([]*spec, error) {
	var s []*spec

	for _, arg := range args {
		parts := strings.SplitN(arg, ":", 2)

		switch len(parts) {
		case 1:
			idx, err := strconv.Atoi(parts[0])
			if err != nil {
				return nil, err
			}

			s = append(s, &spec{idx, idx, false, false})
		case 2:
			var lidx, ridx int
			var err error

			lopen := parts[0] == ""
			if !lopen {
				lidx, err = strconv.Atoi(parts[0])
				if err != nil {
					return nil, err
				}
			}

			ropen := parts[1] == ""
			if !ropen {
				ridx, err = strconv.Atoi(parts[1])
				if err != nil {
					return nil, err
				}
			}

			s = append(s, &spec{lidx, ridx, lopen, ropen})
		}
	}
	return s, nil
}

func run() error {
	var opts options
	args, err := flags.Parse(&opts)
	if err != nil {
		if fe, ok := err.(*flags.Error); ok && fe.Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}

	specs, err := genSpecs(args)
	if err != nil {
		return fmt.Errorf("invalid syntax: %s", err)
	}

	join := opts.Join
	if join == "" {
		join = defaultJoin
	}

	w := bufio.NewWriter(os.Stdout)
	t := newTokenizer(os.Stdin, opts.Separators)
	for {
		tokens, err := t.split()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("unable to read: %s", err)
		}

		top := true
		for _, s := range specs {
			for _, f := range s.pick(tokens) {
				if top {
					top = !top
				} else {
					w.WriteString(join)
				}
				w.Write(f)
			}
		}
		w.WriteByte('\n')
		w.Flush()
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stdout, err.Error())
		os.Exit(1)
	}
}
