package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	outputFile    string
	rootDir       string
	extensionsStr string
	excludeStr    string
	recursive     bool
	showHelp      bool
)

func usage() {
	fmt.Fprintf(os.Stderr, `gptizer: Collects code from files into a single Markdown file.

Usage: %s -o <output.md> [options]

Options:
  -o <filename>   Output Markdown file name. (Default: gptizer_output.md)
  -d <directory>  Directory to search for files. (Default: current directory)
  -e <exts>       Comma-separated list of file extensions to include (e.g., ".go,.md,.txt").
                  (Default: ".go,.md")
  -ex <exts>      Comma-separated list of file extensions to exclude (e.g., ".g.dart").
  -r              Recursively search subdirectories. (Default: false)
  -h, --help      Show this help message.

Examples:
  # Collect .go and .md files from the current directory into output.md
  gptizer -o output.md

  # Collect .js and .css files from ./src recursively into project.md
  gptizer -o project.md -d ./src -e .js,.css -r

  # Collect only .py files from /path/to/code into collection.md
  gptizer -o collection.md -d /path/to/code -e .py

  # Collect .dart files but exclude generated ones
  gptizer -o output.md -e .dart -ex .g.dart
`, filepath.Base(os.Args[0]))
}

func main() {
	flag.StringVar(&outputFile, "o", "", "Output Markdown file name")
	flag.StringVar(&rootDir, "d", "", "Directory to search (default: current directory)")
	flag.StringVar(&extensionsStr, "e", ".go,.md", "Comma-separated file extensions to include (e.g., .go,.md)")
	flag.StringVar(&excludeStr, "ex", "", "Comma-separated file extensions to exclude (e.g., .g.dart)")
	flag.BoolVar(&recursive, "r", false, "Recursively search subdirectories")
	flag.BoolVar(&showHelp, "h", false, "Show help message")
	flag.BoolVar(&showHelp, "help", false, "Show help message") // Allow --help

	flag.Usage = usage

	flag.Parse()

	if showHelp || len(os.Args) == 1 { // Show help if -h/--help or no args
		flag.Usage()
		os.Exit(0)
	}

	if outputFile == "" {
		outputFile = "gptizer_output.md"
	}

	absOutput, absErr := filepath.Abs(outputFile)
	if absErr != nil {
		log.Fatalf("Error making output file path absolute: %v", absErr)
	}

	if rootDir == "" {
		var err error
		rootDir, err = os.Getwd()
		if err != nil {
			log.Fatalf("Error getting current working directory: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Info: No directory specified (-d), using current directory: %s\n", rootDir)
	} else {
		info, err := os.Stat(rootDir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				log.Fatalf("Error: Directory specified with -d does not exist: %s", rootDir)
			}
			log.Fatalf("Error accessing directory %s: %v", rootDir, err)
		}
		if !info.IsDir() {
			log.Fatalf("Error: Path specified with -d is not a directory: %s", rootDir)
		}
	}

	var err error
	rootDir, err = filepath.Abs(rootDir)
	if err != nil {
		log.Fatalf("Error making root directory path absolute: %v", err)
	}

	extensions := make(map[string]bool)
	rawExts := strings.Split(extensionsStr, ",")
	for _, ext := range rawExts {
		trimmedExt := strings.TrimSpace(ext)
		if trimmedExt == "" {
			continue
		}

		if !strings.HasPrefix(trimmedExt, ".") {
			trimmedExt = "." + trimmedExt
		}
		extensions[trimmedExt] = true
	}
	if len(extensions) == 0 {
		log.Fatal("Error: No valid extensions provided with -e.")
	}
	fmt.Fprintf(os.Stderr, "Info: Searching for extensions: %v\n", keys(extensions))
	fmt.Fprintf(os.Stderr, "Info: Recursive search: %v\n", recursive)

	exclude := make(map[string]bool)
	if excludeStr != "" {
		rawEx := strings.Split(excludeStr, ",")
		for _, ext := range rawEx {
			trimmedExt := strings.TrimSpace(ext)
			if trimmedExt == "" {
				continue
			}

			if !strings.HasPrefix(trimmedExt, ".") {
				trimmedExt = "." + trimmedExt
			}
			exclude[trimmedExt] = true
		}
		fmt.Fprintf(os.Stderr, "Info: Excluding extensions: %v\n", keys(exclude))
	}

	outFile, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Error creating output file %s: %v", outputFile, err)
	}
	defer outFile.Close()

	writer := bufio.NewWriter(outFile)
	defer writer.Flush()

	fileCount := 0
	var walkErr error
	linesPerExt := make(map[string]int)
	totalLines := 0
	var files []string

	walkFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {

			fmt.Fprintf(os.Stderr, "Warning: Error accessing %s: %v. Skipping.\n", path, err)

			if errors.Is(err, fs.ErrPermission) && d.IsDir() {
				return fs.SkipDir
			}

			return nil
		}

		if d.IsDir() {

			if !recursive && path != rootDir {

			}

			return nil
		}

		fileExt := filepath.Ext(path)
		if _, ok := extensions[fileExt]; ok {
			if _, ex := exclude[fileExt]; ex {
				return nil
			}

			absPath, _ := filepath.Abs(path)
			if absPath == absOutput {
				return nil
			}

			files = append(files, path)
		}
		return nil
	}

	err = filepath.WalkDir(rootDir, walkFunc)

	if err != nil && !errors.Is(err, fs.SkipDir) {
		log.Fatalf("Error walking directory %s: %v", rootDir, err)
	}
	if walkErr != nil {

		log.Fatalf("Error during file processing: %v", walkErr)
	}

	sort.Strings(files)

	for i, path := range files {
		relativePath, err := filepath.Rel(rootDir, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not get relative path for %s: %v. Using absolute.\n", path, err)
			relativePath = path
		}

		fmt.Fprintf(os.Stderr, "Processing %d/%d: %s\n", i+1, len(files), relativePath)

		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Error reading file %s: %v. Skipping.\n", path, err)
			continue
		}

		fileExt := filepath.Ext(path)
		lines := len(strings.Split(strings.TrimSuffix(string(content), "\n"), "\n"))
		linesPerExt[fileExt] += lines
		totalLines += lines

		_, err = fmt.Fprintf(writer, "## File: `%s`\n\n", relativePath)
		if err != nil {
			log.Fatalf("Error writing header for %s: %v", relativePath, err)
		}

		lang := strings.TrimPrefix(fileExt, ".")
		if lang == "md" {
			lang = "markdown"
		}

		_, err = fmt.Fprintf(writer, "```%s\n", lang)
		if err != nil {
			log.Fatalf("Error writing start fence for %s: %v", relativePath, err)
		}

		_, err = writer.Write(content)
		if err != nil {
			log.Fatalf("Error writing content for %s: %v", relativePath, err)
		}

		_, err = fmt.Fprintf(writer, "\n```\n\n")
		if err != nil {
			log.Fatalf("Error writing end fence for %s: %v", relativePath, err)
		}
		fileCount++
	}

	err = writer.Flush()
	if err != nil {
		log.Fatalf("Error flushing output buffer: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Success: Collected content from %d file(s) into %s\n", fileCount, outputFile)

	fmt.Fprintf(os.Stderr, "Statistics:\n")
	fmt.Fprintf(os.Stderr, "Total lines of code: %d\n", totalLines)
	fmt.Fprintf(os.Stderr, "Lines per extension:\n")
	for ext, lines := range linesPerExt {
		fmt.Fprintf(os.Stderr, "  %s: %d\n", ext, lines)
	}
}

func keys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
