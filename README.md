# typest

A small CLI typing test written in Go. It shows a random technical prompt, times how long you take to type it, and reports your typing speed and accuracy.

## Requirements

- Go 1.26 or newer (required by the `golang.org/x/term` dependency)

## Usage

```sh
go run main.go
```

1. Read the prompt. There's no need to press anything to begin. **The timer starts on your first keystroke.**
2. Type the prompt as fast as you can and press **Enter** when you're finished.
3. Your results appear:

```
----------------------------------------
             TEST RESULTS               
----------------------------------------
Time Elapsed : 24.20 seconds
Gross WPM    : 58 words/minute
Accuracy     : 100.0%
Words Correct: 18 / 18
========================================
```

## Controls

| Key | Behavior |
|-----|----------|
| Any visible character | Starts the timer (first one only) and is recorded. |
| Enter | Finishes the test. Ignored until you have typed something. |
| Space | Ignored until you have typed something, then recorded. |
| Backspace | Deletes the last character you typed. The timer keeps running. |
| Tab, arrow keys | Ignored. |
| Ctrl+D | Finishes the test with what you've typed so far. |
| Ctrl+C | Cancels the test without showing results (exit code 130). |

The terminal is put into raw mode during the test so that individual keystrokes can be detected, and it is restored before results are shown. If input is piped instead of coming from a terminal (e.g. `echo "Go is fun" | go run main.go`), the timer starts when the first visible character is read.

## How the metrics are calculated

| Metric | Formula |
|--------|---------|
| Gross WPM | `(characters typed / 5) / minutes elapsed`. A "word" is 5 characters. If no time has passed, this is 0. |
| Accuracy | `correct words / target words * 100`. Words are compared by position, and punctuation counts as part of the word. |

Whitespace at the start and end of your input is removed before scoring. Only your final text counts: characters you delete with Backspace aren't counted, but the time spent correcting them is. If you press Enter before you finish, the words you didn't type count as incorrect.

## Running tests

```sh
go test ./...
```
