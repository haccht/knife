package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
)

type fieldReader struct {
	br *bufio.Reader
	bp sync.Pool
	fp sync.Pool
}

func newFieldReader(r io.Reader) *fieldReader {
	return &fieldReader{
		br: bufio.NewReaderSize(r, 65535),
		bp: sync.Pool{
			New: func() interface{} {
				s := make([]byte, 0, 64)
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
	ptr := fr.bp.Get().(*[]byte)
	defer fr.bp.Put(ptr)
	bytes := (*ptr)[:0]

L:
	// read one field
	for {
		b, err := fr.br.ReadByte()
		if err != nil {
			return "", false, err
		}

		switch b {
		case 9, 11, 12, 13, 32: //'\t', '\v', '\f', '\r'
			break L
		case 10: //'\n'
			fr.br.UnreadByte()
			break L
		default:
			bytes = append(bytes, b)
		}
	}

	// read trailing spaces
	for {
		b, err := fr.br.ReadByte()
		if err != nil {
			return "", false, err
		}

		switch b {
		case 9, 11, 12, 13, 32: //'\t', '\v', '\f', '\r'
		case 10: //'\n'
			return string(bytes), true, nil
		default:
			fr.br.UnreadByte()
			return string(bytes), false, nil
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
	pickers := make([]picker, len(os.Args)-1)
	for i, arg := range os.Args[1:] {
		picker, err := newPicker(arg)
		if err != nil {
			return fmt.Errorf("invalid syntax: '%s'", arg)
		}
		pickers[i] = picker
	}

	li := make([]string, 0, 64)
	fr := newFieldReader(os.Stdin)

	for {
		fields, err := fr.read()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("unable to read: %s", err)
		}

		li = li[:0]
		for _, picker := range pickers {
			li = append(li, picker.Pick(fields)...)
		}
		fmt.Println(strings.Join(li, " "))
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stdout, err.Error())
		os.Exit(1)
	}
}
