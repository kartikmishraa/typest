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
	"time"
	"unicode/utf8"
)

// charsPerWord is the typing-industry definition of a single "word".
const charsPerWord = 5

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

// readLine reads up to and including the next newline. Reaching EOF is not
// treated as an error so that piped input without a trailing newline still works.
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return line, nil
}

// run executes a single typing test, reading from in and writing to out.
func run(in io.Reader, out io.Writer) error {
	targetText := pickPrompt(prompts)
	reader := bufio.NewReader(in)

	fmt.Fprintln(out, targetText)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Press [ENTER] when you are ready to begin...")
	if _, err := readLine(reader); err != nil {
		return fmt.Errorf("waiting for start: %w", err)
	}

	fmt.Fprintln(out, "🚀 GO! Start typing... (Press [ENTER] when you are finished)")
	startTime := time.Now()
	input, err := readLine(reader)
	duration := time.Since(startTime)
	if err != nil {
		return fmt.Errorf("reading typed input: %w", err)
	}

	// Strip the trailing newline from Enter so the last word isn't marked as a typo.
	typed := strings.TrimSpace(input)

	wpm := calculateWPM(utf8.RuneCountInString(typed), duration)
	correct, total, accuracy := calculateAccuracy(targetText, typed)
	printResults(out, duration, wpm, correct, total, accuracy)
	return nil
}

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
