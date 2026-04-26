package main

import (
	"bytes"
	"strings"
	"testing"
)

func execute(t *testing.T, input string, opts options, args ...string) (string, error) {
	t.Helper()

	specs, err := genSpecs(args)
	if err != nil {
		return "", err
	}

	var out bytes.Buffer
	err = process(strings.NewReader(input), &out, opts, specs)
	return out.String(), err
}

func TestProcessPlainSelector(t *testing.T) {
	out, err := execute(t, "a b c\n", options{}, "2")
	if err != nil {
		t.Fatalf("process returned error: %v", err)
	}
	if out != "b\n" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestProcessRegexSelectorMixed(t *testing.T) {
	out, err := execute(t, "alpha id=123 z9\nbeta id=77 none\n", options{}, "1", "2@[0-9]+", "3@[0-9]+")
	if err != nil {
		t.Fatalf("process returned error: %v", err)
	}
	if out != "alpha 123 9\nbeta 77 none\n" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestProcessRegexSelectorRange(t *testing.T) {
	out, err := execute(t, "a1 b22 c333\n", options{}, "1:3@[0-9]+")
	if err != nil {
		t.Fatalf("process returned error: %v", err)
	}
	if out != "1 22 333\n" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestProcessRegexSelectorReverseRange(t *testing.T) {
	out, err := execute(t, "aa11 bb22 cc33\n", options{}, "3:1@[a-z]+")
	if err != nil {
		t.Fatalf("process returned error: %v", err)
	}
	if out != "cc bb aa\n" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestProcessRegexSelectorOpenNegativeRange(t *testing.T) {
	out, err := execute(t, "a1 b22 c333\n", options{}, "-2:@[0-9]+")
	if err != nil {
		t.Fatalf("process returned error: %v", err)
	}
	if out != "22 333\n" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestProcessRegexSelectorFallsBackToOriginalField(t *testing.T) {
	out, err := execute(t, "abc def\n", options{}, "1:2@[0-9]+")
	if err != nil {
		t.Fatalf("process returned error: %v", err)
	}
	if out != "abc def\n" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestProcessCommandSelector(t *testing.T) {
	out, err := execute(t, "u a b c id=123 z\nv d e f x99 y\n", options{}, "1", "4", "5|grep -oE [0-9]+", "6")
	if err != nil {
		t.Fatalf("process returned error: %v", err)
	}
	if out != "u c 123 z\nv f 99 y\n" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestProcessRegexCommandSelector(t *testing.T) {
	out, err := execute(t, "alice id=123\nbob id=77\n", options{}, "1", "2@[0-9]+|sed 's/^/#/'")
	if err != nil {
		t.Fatalf("process returned error: %v", err)
	}
	if out != "alice #123\nbob #77\n" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestProcessCommandSelectorAllowsBufferedOutput(t *testing.T) {
	out, err := execute(t, "a 1\nb 2\nc 3\n", options{}, "1", "2|awk '{ xs[NR] = $1 } END { for (i = 1; i <= NR; i++) print xs[i] * 10 }'")
	if err != nil {
		t.Fatalf("process returned error: %v", err)
	}
	if out != "a 10\nb 20\nc 30\n" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestGenSpecsRejectsEmptyCommand(t *testing.T) {
	if _, err := genSpecs([]string{"1|"}); err == nil {
		t.Fatal("expected missing command error")
	}
}

func TestProcessExplicitSeparatorKeepsEmptyFields(t *testing.T) {
	out, err := execute(t, "a,,,c\n", options{Separators: []string{","}}, "1:4")
	if err != nil {
		t.Fatalf("process returned error: %v", err)
	}
	if out != "a   c\n" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestProcessExplicitMultipleSeparators(t *testing.T) {
	out, err := execute(t, "a,-,c\n", options{Separators: []string{",", "-"}}, "1:5")
	if err != nil {
		t.Fatalf("process returned error: %v", err)
	}
	if out != "a   c\n" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestGenSpecsInvalidRegex(t *testing.T) {
	if _, err := genSpecs([]string{"1@("}); err == nil {
		t.Fatal("expected invalid regexp error")
	}
}

func TestProcessRejectsMultiByteSeparator(t *testing.T) {
	_, err := execute(t, "a::b\n", options{Separators: []string{"::"}}, "1:2")
	if err == nil {
		t.Fatal("expected invalid separator error")
	}
}
