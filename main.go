package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	flags "github.com/jessevdk/go-flags"
)

const defaultSeparators = " \t\v\f\r"
const defaultBufferSize = 1 << 20

type options struct {
	Separators []string `short:"F" long:"field-separator" description:"Single-byte field separator. Repeat to use multiple separators."`
	BufferSize int      `long:"buffer-size" description:"Buffer size in bytes for buffered I/O (default: 1MB)"`
}

type tokenizer struct {
	br         *bufio.Reader
	tokens     [][]byte
	separators [256]bool
	explicit   bool
}

func newTokenizer(r io.Reader, seps []string, bufSize int) (*tokenizer, error) {
	explicit := len(seps) > 0
	if !explicit {
		seps = []string{defaultSeparators}
	}

	var sepSet [256]bool
	for _, sep := range seps {
		if explicit && len(sep) != 1 {
			return nil, fmt.Errorf("field separator must be a single byte: %q", sep)
		}
		for i := 0; i < len(sep); i++ {
			sepSet[sep[i]] = true
		}
	}

	return &tokenizer{
		br:         bufio.NewReaderSize(r, bufSize),
		tokens:     make([][]byte, 0, 1024),
		separators: sepSet,
		explicit:   explicit,
	}, nil
}

func (t *tokenizer) split() ([][]byte, error) {
	line, err := t.br.ReadSlice('\n')
	if err != nil && err != io.EOF {
		return nil, err
	}

	trimmedLine := bytes.TrimRight(line, "\n")

	t.tokens = t.tokens[:0]
	if t.explicit {
		start := 0
		for i, c := range trimmedLine {
			if t.separators[c] {
				t.tokens = append(t.tokens, trimmedLine[start:i])
				start = i + 1
			}
		}
		t.tokens = append(t.tokens, trimmedLine[start:])
		return t.tokens, err
	}

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
	re    *regexp.Regexp
}

func resolveBound(idx int, open bool, openValue int, tokenCount int) int {
	if open {
		idx = openValue
	}
	if idx <= 0 {
		idx = tokenCount + idx + 1
	}
	if idx < 1 {
		return 0
	}
	if idx > tokenCount {
		return tokenCount - 1
	}
	return idx - 1
}

func (s *spec) each(tokens [][]byte, fn func([]byte)) {
	if len(tokens) == 0 {
		return
	}

	l := resolveBound(s.lidx, s.lopen, 1, len(tokens))
	r := resolveBound(s.ridx, s.ropen, len(tokens), len(tokens))
	step := 1
	if l > r {
		step = -1
	}
	for i := l; ; i += step {
		fn(tokens[i])
		if i == r {
			break
		}
	}
}

func (s *spec) selectField(token []byte) []byte {
	if s.re == nil {
		return token
	}

	match := s.re.Find(token)
	if match == nil {
		return token
	}
	return match
}

func parseIndex(part string) (int, error) {
	if part == "" {
		return 0, nil
	}
	return strconv.Atoi(part)
}

func parseColumnSpec(arg string) (*spec, error) {
	left, right, hasRange := strings.Cut(arg, ":")
	if !hasRange {
		idx, err := strconv.Atoi(left)
		if err != nil {
			return nil, err
		}
		return &spec{lidx: idx, ridx: idx}, nil
	}

	lidx, err := parseIndex(left)
	if err != nil {
		return nil, err
	}
	ridx, err := parseIndex(right)
	if err != nil {
		return nil, err
	}
	return &spec{lidx: lidx, ridx: ridx, lopen: left == "", ropen: right == ""}, nil
}

func genSpecs(args []string) ([]*spec, error) {
	specs := make([]*spec, 0, len(args))

	for _, arg := range args {
		columnSpec, pattern, hasExtract := strings.Cut(arg, "@")

		parsed, err := parseColumnSpec(columnSpec)
		if err != nil {
			return nil, err
		}
		if hasExtract {
			if pattern == "" {
				return nil, fmt.Errorf("missing regexp in selector: %s", arg)
			}

			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid regexp in selector %q: %w", arg, err)
			}
			parsed.re = re
		}

		specs = append(specs, parsed)
	}
	return specs, nil
}

func normalizeOptions(opts options) int {
	bufSize := opts.BufferSize
	if bufSize <= 0 {
		bufSize = defaultBufferSize
	}

	return bufSize
}

func process(r io.Reader, out io.Writer, opts options, specs []*spec) error {
	bufSize := normalizeOptions(opts)
	w := bufio.NewWriterSize(out, bufSize)
	defer w.Flush()
	t, err := newTokenizer(r, opts.Separators, bufSize)
	if err != nil {
		return err
	}

	for {
		tokens, err := t.split()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("unable to read: %s", err)
		}

		first := true
		for _, s := range specs {
			s.each(tokens, func(token []byte) {
				if first {
					first = false
				} else {
					_ = w.WriteByte(' ')
				}
				_, _ = w.Write(s.selectField(token))
			})
		}
		_ = w.WriteByte('\n')
	}

	return nil
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

	return process(os.Stdin, os.Stdout, opts, specs)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stdout, err.Error())
		os.Exit(1)
	}
}
