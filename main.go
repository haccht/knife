package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

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
	cmd   string
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

func (s *spec) bounds(tokenCount int) (int, int, int, bool) {
	if tokenCount == 0 {
		return 0, 0, 0, false
	}

	l := resolveBound(s.lidx, s.lopen, 1, tokenCount)
	r := resolveBound(s.ridx, s.ropen, tokenCount, tokenCount)
	step := 1
	if l > r {
		step = -1
	}
	return l, r, step, true
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
		selector, command, hasCommand := strings.Cut(arg, "|")
		columnSpec, pattern, hasExtract := strings.Cut(selector, "@")

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
		if hasCommand {
			if command == "" {
				return nil, fmt.Errorf("missing command in selector: %s", arg)
			}
			parsed.cmd = command
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

func hasCommandSpec(specs []*spec) bool {
	for _, s := range specs {
		if s.cmd != "" {
			return true
		}
	}
	return false
}

func process(r io.Reader, out io.Writer, opts options, specs []*spec) error {
	if hasCommandSpec(specs) {
		return processWithCommands(r, out, opts, specs)
	}

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
			l, r, step, ok := s.bounds(len(tokens))
			if !ok {
				continue
			}
			for i := l; ; i += step {
				if first {
					first = false
				} else {
					_ = w.WriteByte(' ')
				}
				_, _ = w.Write(s.selectField(tokens[i]))
				if i == r {
					break
				}
			}
		}
		_ = w.WriteByte('\n')
	}

	return nil
}

type outputField struct {
	value  []byte
	runner *commandRunner
}

type commandSpec struct {
	spec   *spec
	runner *commandRunner
}

func processWithCommands(r io.Reader, out io.Writer, opts options, specs []*spec) error {
	bufSize := normalizeOptions(opts)
	t, err := newTokenizer(r, opts.Separators, bufSize)
	if err != nil {
		return err
	}

	commandSpecs, runners, err := prepareCommandSpecs(specs, bufSize)
	if err != nil {
		return err
	}

	rows := newQueue[[]outputField](1024)
	producerDone := make(chan struct{})
	go func() {
		defer close(producerDone)
		defer func() {
			for _, runner := range runners {
				close(runner.input)
			}
		}()
		for {
			tokens, err := t.split()
			if err == io.EOF {
				break
			}
			if err != nil {
				rows.close(fmt.Errorf("unable to read: %s", err))
				return
			}

			row := make([]outputField, 0, len(specs))
			for _, cs := range commandSpecs {
				l, r, step, ok := cs.spec.bounds(len(tokens))
				if !ok {
					continue
				}
				for i := l; ; i += step {
					field := bytes.Clone(cs.spec.selectField(tokens[i]))
					outField := outputField{value: field}
					if cs.runner != nil {
						cs.runner.input <- field
						outField.runner = cs.runner
					}
					row = append(row, outField)
					if i == r {
						break
					}
				}
			}
			rows.push(row)
		}
		rows.close(nil)
	}()

	w := bufio.NewWriterSize(out, bufSize)
	for {
		row, ok, err := rows.pop()
		if err != nil {
			<-producerDone
			_ = waitCommandRunners(runners)
			return err
		}
		if !ok {
			break
		}

		for i, field := range row {
			if i > 0 {
				_ = w.WriteByte(' ')
			}
			value := field.value
			if field.runner != nil {
				result, ok, err := field.runner.output.pop()
				if err != nil {
					<-producerDone
					_ = waitCommandRunners(runners)
					return err
				}
				if !ok {
					<-producerDone
					_ = waitCommandRunners(runners)
					return fmt.Errorf("command %q returned fewer lines than selected fields", field.runner.command)
				}
				value = result
			}
			_, _ = w.Write(value)
		}
		_ = w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		<-producerDone
		_ = waitCommandRunners(runners)
		return err
	}

	<-producerDone
	if err := waitCommandRunners(runners); err != nil {
		return err
	}
	for _, runner := range runners {
		extra, ok, err := runner.output.pop()
		if err != nil {
			return err
		}
		if ok {
			return fmt.Errorf("command %q returned extra output line: %q", runner.command, extra)
		}
	}

	return nil
}

func prepareCommandSpecs(specs []*spec, bufSize int) ([]commandSpec, []*commandRunner, error) {
	commandSpecs := make([]commandSpec, 0, len(specs))
	runners := make([]*commandRunner, 0)
	byCommand := make(map[string]*commandRunner)

	for _, s := range specs {
		cs := commandSpec{spec: s}
		if s.cmd != "" {
			runner := byCommand[s.cmd]
			if runner == nil {
				var err error
				runner, err = newCommandRunner(s.cmd, bufSize)
				if err != nil {
					for _, started := range runners {
						close(started.input)
						_ = <-started.done
					}
					return nil, nil, err
				}
				byCommand[s.cmd] = runner
				runners = append(runners, runner)
			}
			cs.runner = runner
		}
		commandSpecs = append(commandSpecs, cs)
	}

	return commandSpecs, runners, nil
}

type commandRunner struct {
	command string
	input   chan []byte
	output  *queue[[]byte]
	done    chan error
}

func newCommandRunner(command string, bufSize int) (*commandRunner, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	runner := &commandRunner{
		command: command,
		input:   make(chan []byte, 1024),
		output:  newQueue[[]byte](1024),
		done:    make(chan error, 1),
	}

	writeErr := make(chan error, 1)
	go func() {
		bw := bufio.NewWriterSize(stdin, bufSize)
		for input := range runner.input {
			if _, err := bw.Write(input); err != nil {
				_ = stdin.Close()
				writeErr <- err
				return
			}
			if err := bw.WriteByte('\n'); err != nil {
				_ = stdin.Close()
				writeErr <- err
				return
			}
		}
		if err := bw.Flush(); err != nil {
			_ = stdin.Close()
			writeErr <- err
			return
		}
		writeErr <- stdin.Close()
	}()

	readErr := make(chan error, 1)
	go func() {
		br := bufio.NewReader(stdout)
		for {
			line, err := br.ReadBytes('\n')
			if len(line) > 0 {
				line = bytes.TrimSuffix(line, []byte{'\n'})
				line = bytes.TrimSuffix(line, []byte{'\r'})
				runner.output.push(line)
			}
			if err == io.EOF {
				runner.output.close(nil)
				readErr <- nil
				return
			}
			if err != nil {
				readErr <- err
				runner.output.close(err)
				return
			}
		}
	}()

	go func() {
		var err error
		if writeErr := <-writeErr; writeErr != nil {
			err = writeErr
		}
		if waitErr := cmd.Wait(); waitErr != nil && err == nil {
			err = fmt.Errorf("command %q failed: %w", command, waitErr)
		}
		if readErr := <-readErr; readErr != nil && err == nil {
			err = readErr
		}
		runner.done <- err
	}()

	return runner, nil
}

func waitCommandRunners(runners []*commandRunner) error {
	for _, runner := range runners {
		if err := <-runner.done; err != nil {
			return err
		}
	}
	return nil
}

type queue[T any] struct {
	mu     sync.Mutex
	cond   *sync.Cond
	values []T
	head   int
	closed bool
	err    error
}

func newQueue[T any](capacity int) *queue[T] {
	q := &queue[T]{values: make([]T, 0, capacity)}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *queue[T]) push(v T) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.values = append(q.values, v)
	q.cond.Signal()
}

func (q *queue[T]) close(err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.closed = true
	q.err = err
	q.cond.Broadcast()
}

func (q *queue[T]) pop() (T, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.head >= len(q.values) && !q.closed {
		q.cond.Wait()
	}
	if q.head >= len(q.values) {
		var zero T
		return zero, false, q.err
	}
	v := q.values[q.head]
	var zero T
	q.values[q.head] = zero
	q.head++
	if q.head > 1024 && q.head*2 >= len(q.values) {
		q.values = append([]T(nil), q.values[q.head:]...)
		q.head = 0
	}
	return v, true, nil
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
