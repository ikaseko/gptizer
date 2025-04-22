## File: `main.go`

```go
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
	"strings"
)

var (
	outputFile    string
	rootDir       string
	extensionsStr string
	recursive     bool
	showHelp      bool
)

func usage() {
	fmt.Fprintf(os.Stderr, `gptizer: Collects code from files into a single Markdown file.

Usage: %s -o <output.md> [options]

Options:
  -o <filename>   Required. Output Markdown file name.
  -d <directory>  Directory to search for files. (Default: current directory)
  -e <exts>       Comma-separated list of file extensions to include (e.g., ".go,.md,.txt").
                  (Default: ".go,.md")
  -r              Recursively search subdirectories. (Default: false)
  -h, --help      Show this help message.

Examples:
  # Collect .go and .md files from the current directory into output.md
  gptizer -o output.md

  # Collect .js and .css files from ./src recursively into project.md
  gptizer -o project.md -d ./src -e .js,.css -r

  # Collect only .py files from /path/to/code into collection.md
  gptizer -o collection.md -d /path/to/code -e .py
`, filepath.Base(os.Args[0]))
	flag.PrintDefaults() // Optional: Print default values if you prefer
}

func main() {
	// --- Argument Parsing ---
	flag.StringVar(&outputFile, "o", "", "Output Markdown file name (required)")
	flag.StringVar(&rootDir, "d", "", "Directory to search (default: current directory)")
	flag.StringVar(&extensionsStr, "e", ".go,.md", "Comma-separated file extensions to include (e.g., .go,.md)")
	flag.BoolVar(&recursive, "r", false, "Recursively search subdirectories")
	flag.BoolVar(&showHelp, "h", false, "Show help message")
	flag.BoolVar(&showHelp, "help", false, "Show help message") // Allow --help

	// Customize usage message
	flag.Usage = usage

	flag.Parse()

	// --- Input Validation ---
	if showHelp || len(os.Args) == 1 { // Show help if -h/--help or no args
		flag.Usage()
		os.Exit(0)
	}

	if outputFile == "" {
		log.Fatal("Error: Output file (-o) is required.")
	}

	// Determine root directory
	if rootDir == "" {
		var err error
		rootDir, err = os.Getwd()
		if err != nil {
			log.Fatalf("Error getting current working directory: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Info: No directory specified (-d), using current directory: %s\n", rootDir)
	} else {
		// Ensure the specified directory exists
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
	// Make rootDir absolute for consistent relative paths later
	var err error
	rootDir, err = filepath.Abs(rootDir)
	if err != nil {
		log.Fatalf("Error making root directory path absolute: %v", err)
	}

	// Parse extensions
	extensions := make(map[string]bool)
	rawExts := strings.Split(extensionsStr, ",")
	for _, ext := range rawExts {
		trimmedExt := strings.TrimSpace(ext)
		if trimmedExt == "" {
			continue
		}
		// Ensure extension starts with a dot
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

	// --- File Processing ---
	outFile, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Error creating output file %s: %v", outputFile, err)
	}
	defer outFile.Close()

	// Use buffered writer for potentially better performance
	writer := bufio.NewWriter(outFile)
	defer writer.Flush() // Ensure buffer is written before exiting

	fileCount := 0
	var walkErr error // To store the first error encountered during walk

	// Use filepath.WalkDir (more efficient than Walk for Go 1.16+)
	walkFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Report errors accessing files/dirs but continue walking if possible
			fmt.Fprintf(os.Stderr, "Warning: Error accessing %s: %v. Skipping.\n", path, err)
			// If the error prevents reading the directory content, return it to stop
			if errors.Is(err, fs.ErrPermission) && d.IsDir() {
				return fs.SkipDir // Skip this directory but continue elsewhere
			}
			// For other errors on files/dirs, just skip the entry
			return nil
		}

		// Skip directories themselves, only process files
		if d.IsDir() {
			// Skip subdirectories if not recursive and not the root directory
			if !recursive && path != rootDir {
				// fmt.Fprintf(os.Stderr, "Debug: Skipping directory (non-recursive): %s\n", path)
				return fs.SkipDir
			}
			// fmt.Fprintf(os.Stderr, "Debug: Entering directory: %s\n", path)
			return nil // Continue walking into directory
		}

		// Check extension
		fileExt := filepath.Ext(path)
		if _, ok := extensions[fileExt]; ok {
			// Get relative path for cleaner headers
			relativePath, err := filepath.Rel(rootDir, path)
			if err != nil {
				// Should generally not happen if path comes from WalkDir rooted at rootDir
				fmt.Fprintf(os.Stderr, "Warning: Could not get relative path for %s: %v. Using absolute.\n", path, err)
				relativePath = path
			}

			fmt.Fprintf(os.Stderr, "Processing: %s\n", relativePath) // Progress indicator

			// Read file content
			content, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Error reading file %s: %v. Skipping.\n", path, err)
				return nil // Skip this file, continue walking
			}

			// Write header and content to markdown
			// Use relative path in header
			_, err = fmt.Fprintf(writer, "## File: `%s`\n\n", relativePath)
			if err != nil {
				walkErr = fmt.Errorf("error writing header for %s: %w", relativePath, err)
				return walkErr // Stop walking on write error
			}

			// Determine code block language hint (optional but nice)
			lang := strings.TrimPrefix(fileExt, ".")
			if lang == "md" {
				lang = "markdown"
			} // common alias

			_, err = fmt.Fprintf(writer, "```%s\n", lang)
			if err != nil {
				walkErr = fmt.Errorf("error writing start fence for %s: %w", relativePath, err)
				return walkErr
			}

			_, err = writer.Write(content) // Write content using buffered writer
			if err != nil {
				walkErr = fmt.Errorf("error writing content for %s: %w", relativePath, err)
				return walkErr
			}

			_, err = fmt.Fprintf(writer, "\n```\n\n") // Add newline before closing fence
			if err != nil {
				walkErr = fmt.Errorf("error writing end fence for %s: %w", relativePath, err)
				return walkErr
			}
			fileCount++
		}
		return nil // Continue walking
	}

	err = filepath.WalkDir(rootDir, walkFunc)

	// Check for errors during the walk itself or errors stored in walkErr
	if err != nil && !errors.Is(err, fs.SkipDir) { // Don't treat SkipDir as a fatal error
		log.Fatalf("Error walking directory %s: %v", rootDir, err)
	}
	if walkErr != nil {
		// This catches errors from within the walkFunc, especially write errors
		log.Fatalf("Error during file processing: %v", walkErr)
	}

	// --- Finalization ---
	err = writer.Flush() // Final flush
	if err != nil {
		log.Fatalf("Error flushing output buffer: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Success: Collected content from %d file(s) into %s\n", fileCount, outputFile)
}

// Helper function to get keys from a map (for printing extensions)
func keys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

```

