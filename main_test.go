package main

import (
	"bytes"
	"errors"
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

func TestRun(t *testing.T) {
	// Restrict to one prompt so the expected output is deterministic.
	orig := prompts
	prompts = []string{orig[1]}
	t.Cleanup(func() { prompts = orig })

	tests := []struct {
		name     string
		input    string
		wantAcc  string
		wantWord string
	}{
		{"perfect", "\n" + orig[1] + "\n", "Accuracy     : 100.0%", "Words Correct: 11 / 11"},
		{"early exit", "\nChannels are used to communicate\n", "Accuracy     : 45.5%", "Words Correct: 5 / 11"},
		{"no trailing newline", "\n" + orig[1], "Accuracy     : 100.0%", "Words Correct: 11 / 11"},
		{"empty stdin", "", "Accuracy     : 0.0%", "Words Correct: 0 / 11"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := run(strings.NewReader(tt.input), &out); err != nil {
				t.Fatalf("run() error = %v", err)
			}
			got := out.String()
			for _, want := range []string{
				orig[1],
				"Press [ENTER] when you are ready to begin...",
				"🚀 GO! Start typing...",
				"TEST RESULTS",
				tt.wantAcc,
				tt.wantWord,
			} {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\nfull output:\n%s", want, got)
				}
			}
			if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
				t.Errorf("output contains NaN/Inf:\n%s", got)
			}
		})
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestRunReadError(t *testing.T) {
	if err := run(errReader{}, &bytes.Buffer{}); err == nil {
		t.Fatal("run() expected error for failing reader, got nil")
	}
}
