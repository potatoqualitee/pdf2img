# pdf2img

A cross-platform PDF to image converter using PDFium (via WebAssembly).

## Features

- Converts PDF pages to PNG or JPEG images
- Cross-platform: Windows, Linux, macOS (Intel and ARM)
- No external dependencies (PDFium is embedded via WebAssembly)
- Configurable DPI, quality, and page selection
- JSON output mode for programmatic use

## Installation

```bash
go install github.com/potatoqualitee/aitools/tools/pdf2img@latest
```

Or build from source:

```bash
cd tools/pdf2img
go build -o pdf2img .
```

## Usage

```
pdf2img <input.pdf> [options]

Arguments:
  <input.pdf>    Path to input PDF file

Options:
  -o, --output <dir>    Output directory (default: same as input file)
  -f, --format <fmt>    Output format: png or jpeg (default: png)
  -q, --quality <n>     JPEG quality 1-100 (default: 85)
  -d, --dpi <n>         Render resolution 72-600 (default: 150)
  -p, --pages <range>   Pages: all, 1, 1-5, 1,3,5 (default: all)
      --prefix <name>   Output filename prefix (default: input name)
      --json            Output results as JSON
      --version         Print version and exit
```

## Examples

```bash
# Convert all pages to PNG
pdf2img document.pdf

# Convert to JPEG with high quality
pdf2img document.pdf -f jpeg -q 95

# Convert first 5 pages at 300 DPI
pdf2img document.pdf -p 1-5 -d 300 -o ./images

# Convert specific pages
pdf2img document.pdf -p "1,3,5-7" --prefix output

# JSON output for scripting
pdf2img document.pdf --json
```

## Output

Images are saved as `{prefix}_page_{number}.{format}`:

```
document_page_001.png
document_page_002.png
document_page_003.png
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Invalid arguments |
| 2 | Input file not found |
| 3 | Invalid PDF file |
| 4 | Output directory error |
| 5 | Render failed |
| 6 | Write failed |
| 7 | Initialization failed |

## Dependencies

- [go-pdfium](https://github.com/klippa-app/go-pdfium) - PDFium bindings with WebAssembly support
- [pflag](https://github.com/spf13/pflag) - POSIX/GNU-style flag parsing

## License

Apache 2.0 (same as go-pdfium)
