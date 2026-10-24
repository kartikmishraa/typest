// Command typest is a CLI typing test that displays a random technical prompt,
// times how long the user takes to type it, and reports gross WPM and accuracy.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/term"
)

// charsPerWord is the typing-industry definition of a single "word".
const charsPerWord = 5

// startHint is shown beneath the prompt to explain how the test begins and ends.
const startHint = "⌨️  Start typing when ready — the timer starts on your first keystroke. (Press [ENTER] when finished)"

// Raw-mode control characters handled by readTypedLine.
const (
	keyCtrlC     = 0x03
	keyCtrlD     = 0x04
	keyCtrlH     = 0x08 // Backspace on some terminals (and Windows).
	keyEscape    = 0x1b
	keyBackspace = 0x7f // Backspace on most Unix terminals (DEL).
)

// eraseLastChar moves the cursor back one column, blanks it, and moves back again.
const eraseLastChar = "\b \b"

// errInterrupted is returned when the user cancels the test with Ctrl+C.
var errInterrupted = errors.New("test cancelled")

// prompts holds the technical sentences a user may be asked to type.
var prompts = []string{
	"Go is an open source programming language designed to make it easy to build simple, reliable, efficient software.",
	"Channels are used to communicate between goroutines, allowing safe data sharing.",
	"Interfaces in Go are satisfied implicitly, so a type never needs to declare which interfaces it implements.",
	"A defer statement pushes a function call onto a list that runs after the surrounding function returns.",
	"Slices are built on top of arrays and provide a more powerful, flexible view into a sequence of elements.",
	"The context package carries deadlines, cancellation signals, and request-scoped values across API boundaries.",
}

// pickPrompt returns a random entry from the given list of prompts.
// It returns an empty string if the list is empty.
func pickPrompt(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[rand.Intn(len(list))]
}

// calculateWPM returns the gross words per minute for charsTyped characters
// typed over duration d. A zero or negative duration yields 0 to avoid
// dividing by zero.
func calculateWPM(charsTyped int, d time.Duration) float64 {
	minutes := d.Minutes()
	if minutes <= 0 {
		return 0
	}
	return (float64(charsTyped) / charsPerWord) / minutes
}

// calculateAccuracy compares target and typed word-by-word at matching positions.
// It returns the number of correct words, the total number of target words,
// and the accuracy as a percentage of the target words.
func calculateAccuracy(target, typed string) (correct, total int, percent float64) {
	targetWords := strings.Fields(target)
	typedWords := strings.Fields(typed)
	total = len(targetWords)

	// Bound by both slices so an early exit or extra words never index out of range.
	for i := 0; i < len(targetWords) && i < len(typedWords); i++ {
		if targetWords[i] == typedWords[i] {
			correct++
		}
	}

	if total == 0 {
		return correct, total, 0
	}
	return correct, total, float64(correct) / float64(total) * 100
}

// printResults writes the test results dashboard to w.
func printResults(w io.Writer, d time.Duration, wpm float64, correct, total int, accuracy float64) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "----------------------------------------")
	fmt.Fprintln(w, "             TEST RESULTS               ")
	fmt.Fprintln(w, "----------------------------------------")
	fmt.Fprintf(w, "Time Elapsed : %.2f seconds\n", d.Seconds())
	fmt.Fprintf(w, "Gross WPM    : %.0f words/minute\n", wpm)
	fmt.Fprintf(w, "Accuracy     : %.1f%%\n", accuracy)
	fmt.Fprintf(w, "Words Correct: %d / %d\n", correct, total)
	fmt.Fprintln(w, "========================================")
}

// skipEscapeSequence discards the remainder of a terminal escape sequence
// (e.g. arrow keys) after ESC has been read. Only bytes already buffered are
// consumed, so a lone ESC key press never blocks waiting for more input.
func skipEscapeSequence(r *bufio.Reader) {
	if r.Buffered() == 0 {
		return
	}
	b, err := r.ReadByte()
	if err != nil {
		return
	}
	switch b {
	case '[': // CSI: parameter bytes followed by a final byte in 0x40–0x7E.
		for r.Buffered() > 0 {
			c, err := r.ReadByte()
			if err != nil || (c >= 0x40 && c <= 0x7e) {
				return
			}
		}
	case 'O': // SS3: exactly one more byte (e.g. F1–F4 on some terminals).
		if r.Buffered() > 0 {
			_, _ = r.ReadByte()
		}
	}
}

// readTypedLine reads the user's attempt one key at a time. The timer starts on
// the first visible character; Enter and spaces before that are ignored. Enter,
// Ctrl+D or EOF ends the attempt. Backspace deletes the last typed character
// (the timer keeps running); escape sequences and other control keys are
// ignored. Accepted characters are echoed to echo (which may be nil), and now
// supplies the clock so timing can be tested deterministically.
func readTypedLine(r *bufio.Reader, echo io.Writer, now func() time.Time) (string, time.Duration, error) {
	var (
		typed   []rune
		start   time.Time
		started bool
	)
	write := func(s string) {
		if echo != nil {
			_, _ = io.WriteString(echo, s)
		}
	}
	finish := func() (string, time.Duration, error) {
		write("\r\n")
		if !started {
			return "", 0, nil
		}
		return string(typed), now().Sub(start), nil
	}

	for {
		ch, _, err := r.ReadRune()
		if errors.Is(err, io.EOF) {
			return finish()
		}
		if err != nil {
			return "", 0, fmt.Errorf("reading typed input: %w", err)
		}

		switch {
		case ch == keyCtrlC:
			write("\r\n")
			return "", 0, errInterrupted
		case ch == keyCtrlD:
			return finish()
		case ch == '\r' || ch == '\n':
			if started {
				return finish()
			}
		case ch == keyEscape:
			skipEscapeSequence(r)
		case ch == keyBackspace || ch == keyCtrlH:
			if len(typed) > 0 {
				typed = typed[:len(typed)-1]
				write(eraseLastChar)
			}
		case ch == ' ':
			if started {
				typed = append(typed, ch)
				write(" ")
			}
		case ch == utf8.RuneError || !unicode.IsPrint(ch):
			// Tab, other control keys and invalid bytes are ignored.
		default:
			if !started {
				start = now()
				started = true
			}
			typed = append(typed, ch)
			write(string(ch))
		}
	}
}

// rawModeFunc switches the input terminal into raw mode and returns a function
// that restores its previous state.
type rawModeFunc func() (restore func(), err error)

// run executes a single typing test, reading from in and writing to out.
// When enterRaw is non-nil the terminal is put into raw mode so the timer can
// start on the first keystroke, and typed characters are echoed to out.
func run(in io.Reader, out io.Writer, enterRaw rawModeFunc) error {
	targetText := pickPrompt(prompts)
	reader := bufio.NewReader(in)

	fmt.Fprintln(out, targetText)
	fmt.Fprintln(out)
	fmt.Fprintln(out, startHint)

	var echo io.Writer
	restore := func() {}
	if enterRaw != nil {
		r, err := enterRaw()
		if err != nil {
			return fmt.Errorf("enabling raw terminal mode: %w", err)
		}
		restore = sync.OnceFunc(r)
		defer restore()
		echo = out
	}

	input, duration, err := readTypedLine(reader, echo, time.Now)
	restore()
	if err != nil {
		return err
	}

	// Trim surrounding whitespace so trailing spaces don't affect scoring.
	typed := strings.TrimSpace(input)

	wpm := calculateWPM(utf8.RuneCountInString(typed), duration)
	correct, total, accuracy := calculateAccuracy(targetText, typed)
	printResults(out, duration, wpm, correct, total, accuracy)
	return nil
}

// terminalRawMode returns a rawModeFunc for the terminal on file descriptor fd.
func terminalRawMode(fd int) rawModeFunc {
	return func() (func(), error) {
		state, err := term.MakeRaw(fd)
		if err != nil {
			return nil, err
		}
		return func() { _ = term.Restore(fd, state) }, nil
	}
}

func main() {
	var enterRaw rawModeFunc
	if fd := int(os.Stdin.Fd()); term.IsTerminal(fd) {
		enterRaw = terminalRawMode(fd)
	}

	if err := run(os.Stdin, os.Stdout, enterRaw); err != nil {
		if errors.Is(err, errInterrupted) {
			fmt.Fprintln(os.Stderr, "Test cancelled.")
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
