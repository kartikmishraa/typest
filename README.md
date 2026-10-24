# typest

A small CLI typing test written in Go. It shows a random technical prompt, times how long you take to type it, and reports your typing speed and accuracy.

## Requirements

- Go 1.22 or newer

## Usage

```sh
go run main.go
```

1. Read the prompt, then press **Enter** to start.
2. Type the prompt as fast as you can and press **Enter** again.
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

## How the metrics are calculated

| Metric | Formula |
|--------|---------|
| Gross WPM | `(characters typed / 5) / minutes elapsed`. A "word" is 5 characters. If no time has passed, this is 0. |
| Accuracy | `correct words / target words * 100`. Words are compared by position, and punctuation counts as part of the word. |

Whitespace at the start and end of your input, including the newline from pressing Enter, is removed before scoring. If you press Enter before you finish, the words you didn't type count as incorrect.

## Running tests

```sh
go test ./...
```
