package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"math"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestPickPrompt(t *testing.T) {
	if got := pickPrompt(nil); got != "" {
		t.Errorf("pickPrompt(nil) = %q, want empty string", got)
	}
	for range 100 {
		if got := pickPrompt(prompts); !slices.Contains(prompts, got) {
			t.Fatalf("pickPrompt returned %q, which is not in prompts", got)
		}
	}
}

func TestCalculateWPM(t *testing.T) {
	tests := []struct {
		name  string
		chars int
		d     time.Duration
		want  float64
	}{
		{"one minute", 300, time.Minute, 60},
		{"thirty seconds", 100, 30 * time.Second, 40},
		{"zero duration", 50, 0, 0},
		{"negative duration", 50, -time.Second, 0},
		{"no chars", 0, time.Minute, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateWPM(tt.chars, tt.d)
			if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("calculateWPM(%d, %v) = %v, want %v", tt.chars, tt.d, got, tt.want)
			}
		})
	}
}

func TestCalculateAccuracy(t *testing.T) {
	goPrompt := prompts[0]
	chanPrompt := prompts[1]
	tests := []struct {
		name        string
		target      string
		typed       string
		wantCorrect int
		wantTotal   int
		wantPercent float64
	}{
		{"perfect run", goPrompt, goPrompt, 18, 18, 100},
		{"early exit", chanPrompt, "Channels are used to communicate", 5, 11, 5.0 / 11.0 * 100},
		{"typo", "a b c d", "a x c d", 3, 4, 75},
		{"extra words", "a b", "a b c d e", 2, 2, 100},
		{"empty input", "a b c", "", 0, 3, 0},
		{"empty target", "", "a b c", 0, 0, 0},
		{"extra whitespace", "a b c", "  a   b\tc \n", 3, 3, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			correct, total, pct := calculateAccuracy(tt.target, tt.typed)
			if correct != tt.wantCorrect || total != tt.wantTotal || math.Abs(pct-tt.wantPercent) > 1e-9 {
				t.Errorf("calculateAccuracy() = (%d, %d, %v), want (%d, %d, %v)",
					correct, total, pct, tt.wantCorrect, tt.wantTotal, tt.wantPercent)
			}
		})
	}
}

func TestPrintResults(t *testing.T) {
	var buf bytes.Buffer
	printResults(&buf, 7780*time.Millisecond, 49.4, 5, 11, 5.0/11.0*100)

	want := `
----------------------------------------
             TEST RESULTS               
----------------------------------------
Time Elapsed : 7.78 seconds
Gross WPM    : 49 words/minute
Accuracy     : 45.5%
Words Correct: 5 / 11
========================================
`
	if got := buf.String(); got != want {
		t.Errorf("printResults output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// fakeClock returns a clock that advances by step on every call, along with a
// pointer to the number of times it has been called.
func fakeClock(step time.Duration) (func() time.Time, *int) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	calls := 0
	return func() time.Time {
		calls++
		return base.Add(time.Duration(calls) * step)
	}, &calls
}

func TestReadTypedLine(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantText  string
		wantDur   time.Duration
		wantCalls int
		wantEcho  string
		wantErr   error
		wantRest  string // unread input left in the reader
	}{
		{"simple line", "go fast\n", "go fast", 10 * time.Second, 2, "go fast\r\n", nil, ""},
		{"carriage return ends", "go fast\rleft", "go fast", 10 * time.Second, 2, "go fast\r\n", nil, "left"},
		{"leading enter and spaces ignored", "\n\r  \n go\n", "go", 10 * time.Second, 2, "go\r\n", nil, ""},
		{"tab ignored", "a\tb\n", "ab", 10 * time.Second, 2, "ab\r\n", nil, ""},
		{"backspace deletes last char", "abx\x7fc\n", "abc", 10 * time.Second, 2, "abx\b \bc\r\n", nil, ""},
		{"ctrl+h deletes last char", "abx\x08c\n", "abc", 10 * time.Second, 2, "abx\b \bc\r\n", nil, ""},
		{"backspace deletes multiple", "abcd\x7f\x7f\x7fx\n", "ax", 10 * time.Second, 2, "abcd\b \b\b \b\b \bx\r\n", nil, ""},
		{"backspace deletes space", "a \x7fb\n", "ab", 10 * time.Second, 2, "a \b \bb\r\n", nil, ""},
		{"backspace deletes multibyte rune", "hé\x7fe\n", "he", 10 * time.Second, 2, "hé\b \be\r\n", nil, ""},
		{"backspace before typing ignored", "\x7f\x08ab\n", "ab", 10 * time.Second, 2, "ab\r\n", nil, ""},
		{"extra backspaces ignored", "a\x7f\x7f\x7fb\n", "b", 10 * time.Second, 2, "a\b \bb\r\n", nil, ""},
		{"timer keeps running after erasing all", "ab\x7f\x7f\n", "", 10 * time.Second, 2, "ab\b \b\b \b\r\n", nil, ""},
		{"arrow keys ignored", "a\x1b[Db\x1bOPc\n", "abc", 10 * time.Second, 2, "abc\r\n", nil, ""},
		{"extended CSI ignored", "a\x1b[1;5Cb\n", "ab", 10 * time.Second, 2, "ab\r\n", nil, ""},
		{"unicode", "héllo 世界\n", "héllo 世界", 10 * time.Second, 2, "héllo 世界\r\n", nil, ""},
		{"ctrl+d submits partial", "Chan\x04more", "Chan", 10 * time.Second, 2, "Chan\r\n", nil, "more"},
		{"eof submits partial", "Chan", "Chan", 10 * time.Second, 2, "Chan\r\n", nil, ""},
		{"empty eof", "", "", 0, 0, "\r\n", nil, ""},
		{"only enter then eof", "\n\n", "", 0, 0, "\r\n", nil, ""},
		{"ctrl+c cancels", "ab\x03cd\n", "", 0, 1, "ab\r\n", errInterrupted, "cd\n"},
		{"trailing space kept", "go \n", "go ", 10 * time.Second, 2, "go \r\n", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bufio.NewReader(strings.NewReader(tt.input))
			var echo bytes.Buffer
			clock, calls := fakeClock(10 * time.Second)

			text, d, err := readTypedLine(r, &echo, clock)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if text != tt.wantText {
				t.Errorf("text = %q, want %q", text, tt.wantText)
			}
			if d != tt.wantDur {
				t.Errorf("duration = %v, want %v", d, tt.wantDur)
			}
			if *calls != tt.wantCalls {
				t.Errorf("clock called %d times, want %d", *calls, tt.wantCalls)
			}
			if echo.String() != tt.wantEcho {
				t.Errorf("echo = %q, want %q", echo.String(), tt.wantEcho)
			}
			rest, _ := io.ReadAll(r)
			if string(rest) != tt.wantRest {
				t.Errorf("unread input = %q, want %q", rest, tt.wantRest)
			}
		})
	}
}

func TestReadTypedLineNilEcho(t *testing.T) {
	clock, _ := fakeClock(time.Second)
	text, _, err := readTypedLine(bufio.NewReader(strings.NewReader("ok\n")), nil, clock)
	if err != nil || text != "ok" {
		t.Fatalf("readTypedLine() = %q, %v; want %q, nil", text, err, "ok")
	}
}

func TestReadTypedLineLoneEscapeDoesNotBlock(t *testing.T) {
	// ESC is the last buffered byte; the reader must not wait for more input.
	clock, _ := fakeClock(time.Second)
	text, _, err := readTypedLine(bufio.NewReader(strings.NewReader("a\x1b")), nil, clock)
	if err != nil || text != "a" {
		t.Fatalf("readTypedLine() = %q, %v; want %q, nil", text, err, "a")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestReadTypedLineReadError(t *testing.T) {
	clock, _ := fakeClock(time.Second)
	if _, _, err := readTypedLine(bufio.NewReader(errReader{}), nil, clock); err == nil {
		t.Fatal("expected error for failing reader, got nil")
	}
}

// fakeTerminal records raw-mode transitions and what had been written to out
// at the moment the terminal was restored.
type fakeTerminal struct {
	out          *bytes.Buffer
	entered      int
	restored     int
	outAtRestore string
	enterErr     error
}

func (f *fakeTerminal) enterRaw() (func(), error) {
	if f.enterErr != nil {
		return nil, f.enterErr
	}
	f.entered++
	return func() {
		f.restored++
		f.outAtRestore = f.out.String()
	}, nil
}

func withSinglePrompt(t *testing.T) string {
	t.Helper()
	orig := prompts
	prompts = []string{orig[1]}
	t.Cleanup(func() { prompts = orig })
	return orig[1]
}

func TestRun(t *testing.T) {
	target := withSinglePrompt(t)

	tests := []struct {
		name     string
		input    string
		wantAcc  string
		wantWord string
	}{
		{"perfect", target + "\n", "Accuracy     : 100.0%", "Words Correct: 11 / 11"},
		{"early exit", "Channels are used to communicate\n", "Accuracy     : 45.5%", "Words Correct: 5 / 11"},
		{"leading enter ignored", "\n\n" + target + "\n", "Accuracy     : 100.0%", "Words Correct: 11 / 11"},
		{"no trailing newline", target, "Accuracy     : 100.0%", "Words Correct: 11 / 11"},
		{"empty stdin", "", "Accuracy     : 0.0%", "Words Correct: 0 / 11"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := run(strings.NewReader(tt.input), &out, nil); err != nil {
				t.Fatalf("run() error = %v", err)
			}
			got := out.String()
			for _, want := range []string{target, startHint, "TEST RESULTS", tt.wantAcc, tt.wantWord} {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\nfull output:\n%s", want, got)
				}
			}
			for _, unwanted := range []string{"Press [ENTER] when you are ready", "NaN", "Inf"} {
				if strings.Contains(got, unwanted) {
					t.Errorf("output unexpectedly contains %q:\n%s", unwanted, got)
				}
			}
		})
	}
}

func TestRunRawMode(t *testing.T) {
	target := withSinglePrompt(t)
	var out bytes.Buffer
	ft := &fakeTerminal{out: &out}

	if err := run(strings.NewReader("Chanm\x7fnels\n"), &out, ft.enterRaw); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if ft.entered != 1 || ft.restored != 1 {
		t.Errorf("raw mode entered %d / restored %d times, want 1 / 1", ft.entered, ft.restored)
	}
	if !strings.Contains(ft.outAtRestore, target+"\n\n"+startHint+"\nChanm\b \bnels\r\n") {
		t.Errorf("prompt, hint and echoed input not written before restore:\n%q", ft.outAtRestore)
	}
	if strings.Contains(ft.outAtRestore, "TEST RESULTS") {
		t.Error("results were printed before the terminal was restored")
	}
	// The corrected word "Channels" should count as correct.
	if !strings.Contains(out.String(), "Words Correct: 1 / 11") {
		t.Errorf("unexpected results:\n%s", out.String())
	}
}

func TestRunRawModeCancelled(t *testing.T) {
	withSinglePrompt(t)
	var out bytes.Buffer
	ft := &fakeTerminal{out: &out}

	err := run(strings.NewReader("Chan\x03"), &out, ft.enterRaw)
	if !errors.Is(err, errInterrupted) {
		t.Fatalf("run() error = %v, want errInterrupted", err)
	}
	if ft.restored != 1 {
		t.Errorf("terminal restored %d times, want 1", ft.restored)
	}
	if strings.Contains(out.String(), "TEST RESULTS") {
		t.Error("results printed for a cancelled test")
	}
}

func TestRunRawModeError(t *testing.T) {
	withSinglePrompt(t)
	ft := &fakeTerminal{out: &bytes.Buffer{}, enterErr: errors.New("not a tty")}
	if err := run(strings.NewReader("x\n"), &bytes.Buffer{}, ft.enterRaw); err == nil {
		t.Fatal("run() expected error when raw mode fails, got nil")
	}
}

func TestRunReadError(t *testing.T) {
	if err := run(errReader{}, &bytes.Buffer{}, nil); err == nil {
		t.Fatal("run() expected error for failing reader, got nil")
	}
}
