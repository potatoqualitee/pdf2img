package converter

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

// Config holds the conversion configuration
type Config struct {
	InputFile string
	OutputDir string
	Format    string // "png" or "jpeg"
	Quality   int    // JPEG quality (1-100)
	DPI       int    // Render DPI
	Pages     string // Page range: "all", "1", "1-5", "1,3,5"
	Prefix    string // Output filename prefix
}

// Result holds the conversion result for a single file
type Result struct {
	InputFile   string
	OutputFiles []string
	PageCount   int
	Success     bool
	Error       string
}

// Converter handles PDF to image conversion
type Converter struct {
	pool     pdfium.Pool
	instance pdfium.Pdfium
}

// New creates a new Converter instance
func New() (*Converter, error) {
	// Initialize the WebAssembly pool
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PDFium: %w", err)
	}

	// Get an instance from the pool
	instance, err := pool.GetInstance(time.Second * 30)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to get PDFium instance: %w", err)
	}

	return &Converter{
		pool:     pool,
		instance: instance,
	}, nil
}

// Close releases resources
func (c *Converter) Close() {
	if c.instance != nil {
		c.instance.Close()
	}
	if c.pool != nil {
		c.pool.Close()
	}
}

// Convert performs the PDF to image conversion
func (c *Converter) Convert(cfg Config) (*Result, error) {
	result := &Result{
		InputFile:   cfg.InputFile,
		OutputFiles: []string{},
		Success:     false,
	}

	// Read PDF file
	pdfData, err := os.ReadFile(cfg.InputFile)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read PDF file: %v", err)
		return result, fmt.Errorf("failed to read PDF file: %w", err)
	}

	// Open the document
	doc, err := c.instance.OpenDocument(&requests.OpenDocument{
		File: &pdfData,
	})
	if err != nil {
		result.Error = fmt.Sprintf("failed to open PDF: %v", err)
		return result, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer c.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{
		Document: doc.Document,
	})

	// Get page count
	pageCountResp, err := c.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	if err != nil {
		result.Error = fmt.Sprintf("failed to get page count: %v", err)
		return result, fmt.Errorf("failed to get page count: %w", err)
	}
	totalPages := pageCountResp.PageCount

	// Parse page range
	pages, err := parsePageRange(cfg.Pages, totalPages)
	if err != nil {
		result.Error = fmt.Sprintf("invalid page range: %v", err)
		return result, fmt.Errorf("invalid page range: %w", err)
	}

	// Create output directory if needed
	if cfg.OutputDir != "" {
		if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
			result.Error = fmt.Sprintf("failed to create output directory: %v", err)
			return result, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Determine output directory
	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = filepath.Dir(cfg.InputFile)
	}

	// Determine prefix
	prefix := cfg.Prefix
	if prefix == "" {
		base := filepath.Base(cfg.InputFile)
		prefix = strings.TrimSuffix(base, filepath.Ext(base))
	}

	// Convert each page
	for _, pageNum := range pages {
		pageIndex := pageNum - 1 // 0-indexed

		// Render page
		renderResp, err := c.instance.RenderPageInDPI(&requests.RenderPageInDPI{
			DPI: cfg.DPI,
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: doc.Document,
					Index:    pageIndex,
				},
			},
		})
		if err != nil {
			result.Error = fmt.Sprintf("failed to render page %d: %v", pageNum, err)
			return result, fmt.Errorf("failed to render page %d: %w", pageNum, err)
		}

		// Generate output filename
		ext := cfg.Format
		if ext == "jpeg" {
			ext = "jpg"
		}
		outputFile := filepath.Join(outputDir, fmt.Sprintf("%s_page_%03d.%s", prefix, pageNum, ext))

		// Save image
		if err := saveImage(renderResp.Result.Image, outputFile, cfg.Format, cfg.Quality); err != nil {
			result.Error = fmt.Sprintf("failed to save page %d: %v", pageNum, err)
			return result, fmt.Errorf("failed to save page %d: %w", pageNum, err)
		}

		result.OutputFiles = append(result.OutputFiles, outputFile)
	}

	result.PageCount = len(pages)
	result.Success = true
	return result, nil
}

// parsePageRange parses a page range string and returns a slice of page numbers
func parsePageRange(rangeStr string, totalPages int) ([]int, error) {
	rangeStr = strings.TrimSpace(strings.ToLower(rangeStr))

	if rangeStr == "" || rangeStr == "all" {
		pages := make([]int, totalPages)
		for i := 0; i < totalPages; i++ {
			pages[i] = i + 1
		}
		return pages, nil
	}

	pageSet := make(map[int]bool)
	parts := strings.Split(rangeStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check for range (e.g., "1-5")
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range format: %s", part)
			}

			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid page number: %s", rangeParts[0])
			}

			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid page number: %s", rangeParts[1])
			}

			if start < 1 || end < 1 || start > totalPages || end > totalPages {
				return nil, fmt.Errorf("page numbers out of range (1-%d): %s", totalPages, part)
			}

			if start > end {
				start, end = end, start
			}

			for i := start; i <= end; i++ {
				pageSet[i] = true
			}
		} else {
			// Single page number
			pageNum, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid page number: %s", part)
			}

			if pageNum < 1 || pageNum > totalPages {
				return nil, fmt.Errorf("page number out of range (1-%d): %d", totalPages, pageNum)
			}

			pageSet[pageNum] = true
		}
	}

	// Convert set to sorted slice
	pages := make([]int, 0, len(pageSet))
	for page := range pageSet {
		pages = append(pages, page)
	}
	sort.Ints(pages)

	if len(pages) == 0 {
		return nil, fmt.Errorf("no valid pages specified")
	}

	return pages, nil
}

// saveImage saves an image to a file in the specified format
func saveImage(img image.Image, path string, format string, quality int) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		opts := &jpeg.Options{Quality: quality}
		if err := jpeg.Encode(file, img, opts); err != nil {
			return fmt.Errorf("failed to encode JPEG: %w", err)
		}
	case "png":
		if err := png.Encode(file, img); err != nil {
			return fmt.Errorf("failed to encode PNG: %w", err)
		}
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return nil
}

// ValidateFormat checks if the format is valid
func ValidateFormat(format string) error {
	format = strings.ToLower(format)
	if format != "png" && format != "jpeg" && format != "jpg" {
		return fmt.Errorf("invalid format: %s (must be png or jpeg)", format)
	}
	return nil
}

// ValidatePageRange validates a page range string format (without knowing total pages)
func ValidatePageRange(rangeStr string) error {
	rangeStr = strings.TrimSpace(strings.ToLower(rangeStr))
	if rangeStr == "" || rangeStr == "all" {
		return nil
	}

	// Match pattern: number, number-number, or comma-separated combinations
	pattern := `^(\d+(-\d+)?)(,\s*\d+(-\d+)?)*$`
	matched, err := regexp.MatchString(pattern, rangeStr)
	if err != nil {
		return err
	}
	if !matched {
		return fmt.Errorf("invalid page range format: %s", rangeStr)
	}
	return nil
}
