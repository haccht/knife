package main

import (
	"bufio"
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

type options struct {
	Separators string `short:"F" long:"field-separators" description:"Field separators (default: whitespaces)"`
}

type fieldReader struct {
	br         *bufio.Reader
	fields     []string
	bufBytes   []byte
	separators []byte
}

func newFieldReader(r io.Reader, s string) *fieldReader {
	separators := defaultSeparators
	if s != "" {
		separators = s
	}

	return &fieldReader{
		br:         bufio.NewReaderSize(r, 65536),
		fields:     make([]string, 0, 1024),
		bufBytes:   make([]byte, 0, 1024),
		separators: []byte(separators),
	}
}

func (fr *fieldReader) readOne() (string, bool, error) {
	fr.bufBytes = fr.bufBytes[:0]

L:
	// read one field
	for {
		b, err := fr.br.ReadByte()
		if err != nil {
			return "", false, err
		}

		switch {
		case slices.Contains(fr.separators, b):
			break L
		case b == returnode:
			fr.br.UnreadByte()
			break L
		default:
			fr.bufBytes = append(fr.bufBytes, b)
		}
	}

	// read trailing spaces
	for {
		b, err := fr.br.ReadByte()
		if err != nil {
			return "", false, err
		}

		switch {
		case slices.Contains(fr.separators, b):
		case b == returnode:
			return string(fr.bufBytes), true, nil
		default:
			fr.br.UnreadByte()
			return string(fr.bufBytes), false, nil
		}
	}
}

func (fr *fieldReader) read() ([]string, error) {
	fr.fields = fr.fields[:0]

	for {
		f, eol, err := fr.readOne()
		if err != nil {
			return nil, err
		}

		if f != "" {
			fr.fields = append(fr.fields, f)
		}

		if eol {
			return fr.fields, nil
		}
	}
}

type picker interface {
	Pick([]string) []string
}

func newPicker(indexStr string) (picker, error) {
	s := strings.SplitN(indexStr, ":", 2)

	switch len(s) {
	case 1:
		p := &indexPicker{}

		i, err := strconv.Atoi(s[0])
		if err != nil {
			return nil, err
		}
		p.i = i

		return p, nil
	case 2:
		p := &rangePicker{}

		if s[0] == "" {
			p.lopen = true
		} else {
			l, err := strconv.Atoi(s[0])
			if err != nil {
				return nil, err
			}
			p.l = l
		}

		if s[1] == "" {
			p.ropen = true
		} else {
			r, err := strconv.Atoi(s[1])
			if err != nil {
				return nil, err
			}
			p.r = r
		}

		return p, nil
	default:
		return nil, fmt.Errorf("failed to parse")
	}
}

type indexPicker struct {
	i int
}

func (p *indexPicker) Pick(f []string) []string {
	var i int

	if p.i == 0 {
		return pick(f, 1, len(f))
	} else if p.i < 0 {
		i = len(f) + p.i + 1
	} else {
		i = p.i
	}

	return pick(f, i, i)
}

type rangePicker struct {
	l     int
	r     int
	lopen bool
	ropen bool
}

func (p *rangePicker) Pick(f []string) []string {
	var l, r int

	if p.lopen {
		l = 1
	} else {
		if p.l < 0 {
			l = len(f) + p.l + 1
		} else {
			l = p.l
		}
	}

	if p.ropen {
		r = len(f)
	} else {
		if p.r < 0 {
			r = len(f) + p.r + 1
		} else {
			r = p.r
		}
	}

	if l > r && (p.l <= 0 || p.r >= 0) {
		s := pick(f, r, l)
		slices.Reverse(s)
		return s
	}
	return pick(f, l, r)
}

func pick(f []string, l, r int) []string {
	if r <= 0 || l > len(f) {
		return nil
	}

	if l <= 0 {
		l = 1
	}

	if r > len(f) {
		r = len(f)
	}

	return f[l-1 : r]
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

	pickers := make([]picker, len(args))
	for i, arg := range args {
		picker, err := newPicker(arg)
		if err != nil {
			return fmt.Errorf("invalid syntax: '%s'", arg)
		}
		pickers[i] = picker
	}

	fl := make([]string, 0, 64)
	fr := newFieldReader(os.Stdin, opts.Separators)

	for {
		fields, err := fr.read()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("unable to read: %s", err)
		}

		fl = fl[:0]
		for _, picker := range pickers {
			fl = append(fl, picker.Pick(fields)...)
		}
		fmt.Println(strings.Join(fl, " "))
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stdout, err.Error())
		os.Exit(1)
	}
}
