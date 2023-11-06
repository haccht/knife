package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

type fieldReader struct {
	br *bufio.Reader
	rp sync.Pool
	fp sync.Pool
}

func NewFieldReader(r io.Reader) *fieldReader {
	br := bufio.NewReaderSize(os.Stdin, 65535)
	return &fieldReader{
		br: br,
		rp: sync.Pool{
			New: func() interface{} {
				s := make([]rune, 0, 64)
				return &s
			}},
		fp: sync.Pool{
			New: func() interface{} {
				s := make([]string, 0, 64)
				return &s
			}},
	}
}

func (fr *fieldReader) readOne() (string, bool, error) {
	ptr := fr.rp.Get().(*[]rune)
	defer fr.rp.Put(ptr)
	runes := (*ptr)[:0]

L:
	for {
		r, _, err := fr.br.ReadRune()
		if err != nil {
			return "", false, err
		}

		switch r {
		case ' ', '\r', '\v', '\f', '\t':
			break L
		case '\n':
			fr.br.UnreadRune()
			break L
		default:
			runes = append(runes, r)
		}
	}

	for {
		r, _, err := fr.br.ReadRune()
		if err != nil {
			return "", false, err
		}

		switch r {
		case ' ', '\r', '\v', '\f', '\t':
		case '\n':
			return string(runes), true, nil
		default:
			fr.br.UnreadRune()
			return string(runes), false, nil
		}
	}
}

func (fr *fieldReader) read() ([]string, error) {
	ptr := fr.fp.Get().(*[]string)
	defer fr.fp.Put(ptr)
	fields := (*ptr)[:0]

	for {
		f, eol, err := fr.readOne()
		if err != nil {
			return nil, err
		}

		if f != "" {
			fields = append(fields, f)
		}

		if eol {
			return fields, nil
		}
	}
}

type picker interface {
	Pick([]string) []string
}

type indexPicker struct {
	i int
}

func (p *indexPicker) Pick(f []string) []string {
	i := p.i
	if p.i < 0 {
		i = len(f) + p.i + 1
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
	cl := p.l
	if p.lopen {
		cl = 1
	} else {
		if cl < 0 {
			cl += len(f) + 1
		} else if cl < 1 {
			cl = 1
		}
	}

	cr := p.r
	if p.ropen {
		cr = len(f)
	} else {
		if cr < 0 {
			cr += len(f) + 1
		} else if len(f) < cr {
			cr = len(f)
		}
	}

	return pick(f, cl, cr)
}

func pick(f []string, l, r int) []string {
	if r < 1 || len(f) < l {
		return nil
	}
	return f[l-1 : r]
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

		if !(p.lopen || p.ropen) && p.l > p.r {
			p.l, p.r = p.r, p.l
		}

		return p, nil
	default:
		return nil, fmt.Errorf("failed to parse")
	}
}

func run() error {
	flag.Parse()

	pickers := make([]picker, flag.NArg())
	for i, arg := range flag.Args() {
		picker, err := newPicker(arg)
		if err != nil {
			return fmt.Errorf("invalid syntax: '%s'", arg)
		}

		pickers[i] = picker
	}

	li := make([]string, 0, 16)
	fr := NewFieldReader(os.Stdin)

	for {
		fields, err := fr.read()
		if err != nil && err != io.EOF {
			return fmt.Errorf("unable to read: %s", err)
		}

		li = li[:0]
		for _, picker := range pickers {
			li = append(li, picker.Pick(fields)...)
		}
		fmt.Println(strings.Join(li, " "))

		if err == io.EOF {
			return nil
		}
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
