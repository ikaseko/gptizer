# gptizer

`gptizer` is a simple command-line utility written in Go that collects code and text from specified files within a directory (and optionally its subdirectories) into a single Markdown file.

This is particularly useful for preparing code snippets or project context to be easily copied and pasted into Large Language Models (LLMs) like ChatGPT, Claude, etc.

## Features

*   Collects files based on specified extensions (e.g., `.go`, `.py`, `.md`).
*   Searches the target directory recursively (`-r` flag) or non-recursively.
*   Outputs a structured Markdown file with clear headers indicating the source file path.
*   Uses fenced code blocks in Markdown, attempting to infer the language from the file extension.
*   Simple command-line interface with help (`-h`, `--help`).
*   Fast processing using Go's standard library.

## Installation / Building

You need Go installed (version 1.16 or later recommended for `io/fs`).

1.  Clone this repository or download the `gptizer.go` file.
2.  Navigate to the directory containing `gptizer.go`.
3.  Build the executable:
    ```bash
    go build gptizer.go
    ```
    This will create an executable named `gptizer` (or `gptizer.exe` on Windows) in the current directory. You can move this executable to a directory in your system's PATH for easier access.

## Usage

```bash
./gptizer -o <output.md> [options]