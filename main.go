package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/potatoqualitee/aitools/tools/pdf2img/converter"
	"github.com/spf13/pflag"
)

var (
	version = "dev"
)

// Exit codes
const (
	ExitSuccess           = 0
	ExitInvalidArgs       = 1
	ExitInputNotFound     = 2
	ExitInvalidPDF        = 3
	ExitOutputDirError    = 4
	ExitRenderFailed      = 5
	ExitWriteFailed       = 6
	ExitInitFailed        = 7
)

func main() {
	// Define flags
	output := pflag.StringP("output", "o", "", "Output directory (default: same as input file)")
	format := pflag.StringP("format", "f", "png", "Output format: png or jpeg")
	quality := pflag.IntP("quality", "q", 85, "JPEG quality (1-100)")
	dpi := pflag.IntP("dpi", "d", 150, "Render resolution in DPI")
	pages := pflag.StringP("pages", "p", "all", "Pages to convert: all, 1, 1-5, 1,3,5")
	prefix := pflag.String("prefix", "", "Output filename prefix (default: input filename)")
	jsonOutput := pflag.Bool("json", false, "Output results as JSON")
	showVersion := pflag.Bool("version", false, "Print version and exit")

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: pdf2img <input.pdf> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Converts PDF pages to images (PNG or JPEG).\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  <input.pdf>    Path to input PDF file\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		pflag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  pdf2img document.pdf\n")
		fmt.Fprintf(os.Stderr, "  pdf2img document.pdf -f jpeg -q 90 -d 300\n")
		fmt.Fprintf(os.Stderr, "  pdf2img document.pdf -p 1-5 -o ./images\n")
		fmt.Fprintf(os.Stderr, "  pdf2img document.pdf --pages \"1,3,5-7\" --prefix output\n")
	}

	pflag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("pdf2img version %s\n", version)
		os.Exit(ExitSuccess)
	}

	// Check for input file argument
	args := pflag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: input PDF file is required")
		pflag.Usage()
		os.Exit(ExitInvalidArgs)
	}

	inputFile := args[0]

	// Validate input file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: input file not found: %s\n", inputFile)
		os.Exit(ExitInputNotFound)
	}

	// Validate format
	*format = strings.ToLower(*format)
	if err := converter.ValidateFormat(*format); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(ExitInvalidArgs)
	}

	// Validate quality
	if *quality < 1 || *quality > 100 {
		fmt.Fprintln(os.Stderr, "Error: quality must be between 1 and 100")
		os.Exit(ExitInvalidArgs)
	}

	// Validate DPI
	if *dpi < 72 || *dpi > 600 {
		fmt.Fprintln(os.Stderr, "Error: DPI must be between 72 and 600")
		os.Exit(ExitInvalidArgs)
	}

	// Validate page range format
	if err := converter.ValidatePageRange(*pages); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(ExitInvalidArgs)
	}

	// Resolve input file to absolute path
	absInputFile, err := filepath.Abs(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to resolve input path: %v\n", err)
		os.Exit(ExitInvalidArgs)
	}

	// Resolve output directory
	outputDir := *output
	if outputDir != "" {
		absOutputDir, err := filepath.Abs(outputDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to resolve output path: %v\n", err)
			os.Exit(ExitOutputDirError)
		}
		outputDir = absOutputDir
	}

	// Create converter
	conv, err := converter.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to initialize converter: %v\n", err)
		os.Exit(ExitInitFailed)
	}
	defer conv.Close()

	// Build config
	cfg := converter.Config{
		InputFile: absInputFile,
		OutputDir: outputDir,
		Format:    *format,
		Quality:   *quality,
		DPI:       *dpi,
		Pages:     *pages,
		Prefix:    *prefix,
	}

	// Perform conversion
	result, err := conv.Convert(cfg)

	// Output results
	if *jsonOutput {
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonBytes))
	} else {
		if result.Success {
			fmt.Printf("Converted %d page(s) from %s\n", result.PageCount, filepath.Base(inputFile))
			for _, f := range result.OutputFiles {
				fmt.Printf("  %s\n", f)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", result.Error)
		}
	}

	// Determine exit code
	if err != nil {
		if strings.Contains(err.Error(), "failed to open PDF") {
			os.Exit(ExitInvalidPDF)
		}
		if strings.Contains(err.Error(), "output directory") {
			os.Exit(ExitOutputDirError)
		}
		if strings.Contains(err.Error(), "render") {
			os.Exit(ExitRenderFailed)
		}
		if strings.Contains(err.Error(), "save") || strings.Contains(err.Error(), "encode") {
			os.Exit(ExitWriteFailed)
		}
		os.Exit(ExitInvalidArgs)
	}

	os.Exit(ExitSuccess)
}
