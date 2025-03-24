package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/fatih/color"
)

// ValidationError represents a single XML validation issue
type ValidationError struct {
	LineNumber int
	Column     int
	Line       string
	ErrorType  string
	Message    string
	Content    string // For highlighting purposes
}

// Global validation options
type ValidationOptions struct {
	MaxErrors int
	Debug     bool
	Color     bool // Whether to use colored output
}

// Define color functions 
var (
	successColor   = color.New(color.FgGreen).SprintFunc()
	errorColor     = color.New(color.FgRed).SprintFunc()
	highlightColor = color.New(color.FgYellow).SprintFunc() 
	headerColor    = color.New(color.FgCyan).SprintFunc()
	infoColor      = color.New(color.FgBlue).SprintFunc()
)

func main() {
	// Parse command-line flags
	opts := ValidationOptions{}
	flag.IntVar(&opts.MaxErrors, "max-errors", 5, "Maximum number of errors to report")
	flag.BoolVar(&opts.Debug, "debug", false, "Enable debug output")
	flag.BoolVar(&opts.Color, "color", true, "Enable colored output")
	flag.Parse()

	// Apply color setting
	if !opts.Color {
		// Disable all colors if the color flag is false
		color.NoColor = true
	}

	// Check for required file argument
	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: xml_validator [--max-errors=N] [--debug] [--color] <xml-file-or-URL>")
		os.Exit(1)
	}

	filepath := args[0]
	fmt.Printf("Validating XML: %s\n", filepath)
	fmt.Printf("Will report up to %d errors\n", opts.MaxErrors)

	// Read the file content (local or remote)
	content, err := readFileContent(filepath)
	if err != nil {
		fmt.Printf("❌ Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Run the validation
	allErrors := validateXML(content, opts)
	
	// Display results
	if len(allErrors) == 0 {
		fmt.Println(successColor("✅ XML is well-formed!"))
		os.Exit(0)
	}

	// Report errors
	fmt.Printf("%s Found %d XML issues (showing up to %d):\n", errorColor("❌"), len(allErrors), opts.MaxErrors)
	fmt.Println(headerColor("----------------------------------------"))
	
	maxToShow := opts.MaxErrors
	if maxToShow > len(allErrors) {
		maxToShow = len(allErrors)
	}
	
	for i := 0; i < maxToShow; i++ {
		displayError(content, allErrors[i], i+1)
	}
	
	if len(allErrors) > opts.MaxErrors {
		fmt.Printf("\n%s Found more errors than displayed (%d total). Run with --max-errors=%d to see all.\n", 
			infoColor("Note:"), len(allErrors), len(allErrors))
	}
	
	// Print correction tips
	printCorrectionTips()
	os.Exit(1)
}

// readFileContent reads content from a local file or remote URL
func readFileContent(filepath string) ([]byte, error) {
	if strings.HasPrefix(filepath, "http://") || strings.HasPrefix(filepath, "https://") {
		fmt.Println(infoColor("Downloading from URL..."))
		resp, err := http.Get(filepath)
		if err != nil {
			return nil, fmt.Errorf("failed to download file: %v", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("HTTP error: %s", resp.Status)
		}
		
		return io.ReadAll(resp.Body)
	} else {
		fmt.Println(infoColor("Reading local file..."))
		return os.ReadFile(filepath)
	}
}

// validateXML performs all validation checks on the XML content
func validateXML(content []byte, opts ValidationOptions) []ValidationError {
	var allErrors []ValidationError
	
	// 1. First use Go's XML parser for basic well-formedness
	basicErrors := validateBasicXML(content)
	allErrors = append(allErrors, basicErrors...)
	if len(allErrors) >= opts.MaxErrors && opts.MaxErrors > 0 {
		return allErrors[:opts.MaxErrors]
	}
	
	// If there are no basic XML errors, run additional checks
	if len(basicErrors) == 0 {
		fmt.Println(successColor("Basic XML validation passed. Performing additional checks..."))
		
		// 2. Check CDATA sections
		fmt.Println(infoColor("Checking CDATA sections..."))
		cdataErrors := validateCDATASections(content, opts)
		allErrors = append(allErrors, cdataErrors...)
		if len(allErrors) >= opts.MaxErrors && opts.MaxErrors > 0 {
			return allErrors[:opts.MaxErrors]
		}
		
		// 3. Check for control characters
		fmt.Println(infoColor("Checking for control characters..."))
		controlErrors := validateControlCharacters(content, opts)
		allErrors = append(allErrors, controlErrors...)
		if len(allErrors) >= opts.MaxErrors && opts.MaxErrors > 0 {
			return allErrors[:opts.MaxErrors]
		}
		
		// 4. Check hex color codes
		fmt.Println(infoColor("Checking hex color codes..."))
		hexErrors := validateHexColors(content, opts)
		allErrors = append(allErrors, hexErrors...)
		if len(allErrors) >= opts.MaxErrors && opts.MaxErrors > 0 {
			return allErrors[:opts.MaxErrors]
		}
		
		// 5. Check SVG syntax
		fmt.Println(infoColor("Checking SVG syntax..."))
		svgErrors := validateSVG(content, opts)
		allErrors = append(allErrors, svgErrors...)
	}
	
	// Limit errors if needed
	if opts.MaxErrors > 0 && len(allErrors) > opts.MaxErrors {
		return allErrors[:opts.MaxErrors]
	}
	
	return allErrors
}

// validateBasicXML uses Go's XML parser to check well-formedness
func validateBasicXML(content []byte) []ValidationError {
	var errors []ValidationError
	
	decoder := xml.NewDecoder(bytes.NewReader(content))
	
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Try to extract error location
			syntaxErr, ok := err.(*xml.SyntaxError)
			if ok {
				line, col, lineContent := findErrorPosition(content, int(syntaxErr.Line))
				errors = append(errors, ValidationError{
					LineNumber: line,
					Column:     col,
					Line:       lineContent,
					ErrorType:  "Basic XML Syntax Error",
					Message:    err.Error(),
				})
			} else {
				// Generic error without position info
				errors = append(errors, ValidationError{
					LineNumber: 0,
					ErrorType:  "XML Error",
					Message:    err.Error(),
				})
			}
			break // Stop at first error
		}
		
		// We could inspect tokens here for additional validation
		if token == nil {
			break
		}
	}
	
	return errors
}

// validateCDATASections checks for various CDATA section issues
func validateCDATASections(content []byte, opts ValidationOptions) []ValidationError {
	var errors []ValidationError
	lines := bytes.Split(content, []byte("\n"))
	
	// Define regex patterns for various CDATA issues
	reCDATAWithSpecialChar := regexp.MustCompile(`<!\[CDATA\[[^a-zA-Z0-9 ]`)
	reCDATAWithExclamation := regexp.MustCompile(`<!\[CDATA\[!`)
	reUnclosedCDATA := regexp.MustCompile(`<!\[CDATA\[(?:(?!\]\]>).)*$`)
	reNestedCDATA := regexp.MustCompile(`<!\[CDATA\[.*<!\[CDATA\[`)
	reMultiClosingCDATA := regexp.MustCompile(`<!\[CDATA\[.*\]\]>.*\]\]>`)
	reEmptyCDATA := regexp.MustCompile(`<!\[CDATA\[\]\]>`)
	
	for i, line := range lines {
		lineStr := string(line)
		
		// 1. Check for special characters after CDATA opening
		if matches := reCDATAWithSpecialChar.FindStringIndex(lineStr); matches != nil {
			badChar := lineStr[matches[0]+9] // Character after <![CDATA[
			errors = append(errors, ValidationError{
				LineNumber: i + 1,
				Column:     matches[0] + 9,
				Line:       lineStr,
				ErrorType:  "Special character after CDATA opening",
				Message:    fmt.Sprintf("Special character '%c' found immediately after CDATA opening", badChar),
				Content:    "<![CDATA[" + string(badChar),
			})
		}
		
		// 2. Check specifically for exclamation marks (common in WP exports)
		if matches := reCDATAWithExclamation.FindStringIndex(lineStr); matches != nil {
			errors = append(errors, ValidationError{
				LineNumber: i + 1,
				Column:     matches[0] + 9,
				Line:       lineStr,
				ErrorType:  "Exclamation mark after CDATA opening",
				Message:    "Exclamation mark found immediately after CDATA opening",
				Content:    "<![CDATA[!",
			})
		}
		
		// 3. Check for unclosed CDATA sections
		if matches := reUnclosedCDATA.FindStringIndex(lineStr); matches != nil {
			errors = append(errors, ValidationError{
				LineNumber: i + 1,
				Column:     matches[0],
				Line:       lineStr,
				ErrorType:  "Unclosed CDATA section",
				Message:    "CDATA section is not properly closed with ]]>",
				Content:    lineStr[matches[0]:],
			})
		}
		
		// 4. Check for nested CDATA sections
		if matches := reNestedCDATA.FindStringIndex(lineStr); matches != nil {
			errors = append(errors, ValidationError{
				LineNumber: i + 1,
				Column:     matches[0],
				Line:       lineStr,
				ErrorType:  "Nested CDATA sections",
				Message:    "CDATA sections cannot be nested",
				Content:    lineStr[matches[0]:matches[1]],
			})
		}
		
		// 5. Check for multiple CDATA closing sequences
		if matches := reMultiClosingCDATA.FindStringIndex(lineStr); matches != nil {
			errors = append(errors, ValidationError{
				LineNumber: i + 1,
				Column:     matches[0],
				Line:       lineStr,
				ErrorType:  "Multiple CDATA closing sequences",
				Message:    "Found multiple ']]>' sequences in a single CDATA block",
				Content:    lineStr[matches[0]:matches[1]],
			})
		}
		
		// 6. Check for empty CDATA sections
		if matches := reEmptyCDATA.FindStringIndex(lineStr); matches != nil {
			errors = append(errors, ValidationError{
				LineNumber: i + 1,
				Column:     matches[0],
				Line:       lineStr,
				ErrorType:  "Empty CDATA section",
				Message:    "CDATA section is empty",
				Content:    "<![CDATA[]]>",
			})
		}
		
		// Stop if we've reached max errors
		if opts.MaxErrors > 0 && len(errors) >= opts.MaxErrors {
			break
		}
	}
	
	return errors
}

// validateControlCharacters checks for control characters in XML
func validateControlCharacters(content []byte, opts ValidationOptions) []ValidationError {
	var errors []ValidationError
	lines := bytes.Split(content, []byte("\n"))
	
	for i, line := range lines {
		lineStr := string(line)
		
		// Look for control characters (except tab, CR, LF)
		for j, r := range lineStr {
			if r < 32 && r != '\t' && r != '\r' && r != '\n' {
				// Found a control character
				errors = append(errors, ValidationError{
					LineNumber: i + 1,
					Column:     j + 1,
					Line:       lineStr,
					ErrorType:  "Control character",
					Message:    fmt.Sprintf("Control character (hex 0x%02X) found", r),
					Content:    string(r),
				})
				
				// Stop checking this line if we found a control character
				break
			}
		}
		
		// Stop if we've reached max errors
		if opts.MaxErrors > 0 && len(errors) >= opts.MaxErrors {
			break
		}
	}
	
	return errors
}

// validateHexColors checks for malformed hex color codes
func validateHexColors(content []byte, opts ValidationOptions) []ValidationError {
	var errors []ValidationError
	lines := bytes.Split(content, []byte("\n"))
	
	// Valid hex colors: #RGB, #RRGGBB, #RRGGBBAA
	// Invalid: #R, #RG, #RGBG, #RRGGB, anything with more than 8 chars
	reInvalidHex := regexp.MustCompile(`#[0-9a-fA-F]{1,2}([^0-9a-fA-F]|$)|#[0-9a-fA-F]{4,5}([^0-9a-fA-F]|$)|#[0-9a-fA-F]{7,}`)
	
	for i, line := range lines {
		lineStr := string(line)
		
		// Find all invalid hex colors on this line
		matches := reInvalidHex.FindAllStringSubmatchIndex(lineStr, -1)
		for _, match := range matches {
			// Extract the hex code - careful to get just the hex part
			hexStart := match[0]
			hexEnd := match[1]
			if match[2] != -1 { // If there's a character after the hex, don't include it
				hexEnd = match[2]
			}
			hexCode := lineStr[hexStart:hexEnd]
			
			errors = append(errors, ValidationError{
				LineNumber: i + 1,
				Column:     hexStart + 1,
				Line:       lineStr,
				ErrorType:  "Invalid hex color",
				Message:    fmt.Sprintf("Invalid hex color code: %s (should be #RGB, #RRGGBB, or #RRGGBBAA)", hexCode),
				Content:    hexCode,
			})
		}
		
		// Stop if we've reached max errors
		if opts.MaxErrors > 0 && len(errors) >= opts.MaxErrors {
			break
		}
	}
	
	return errors
}

// validateSVG checks for SVG syntax issues in XML
func validateSVG(content []byte, opts ValidationOptions) []ValidationError {
	var errors []ValidationError
	lines := bytes.Split(content, []byte("\n"))
	
	// Pattern for SVG elements that should be self-closing
	// This is simplified - real SVG validation would need more sophisticated parsing
	reSVGSelfClosing := regexp.MustCompile(`<(path|rect|circle|ellipse|line|polyline|polygon|image|use)[^>]*[^/]>`)
	reSVGUnquotedAttr := regexp.MustCompile(`<svg[^>]*(width|height|viewBox)=([^"'][^ >]*)`)
	
	for i, line := range lines {
		lineStr := string(line)
		
		// Check for SVG elements that should be self-closing
		matches := reSVGSelfClosing.FindAllStringSubmatchIndex(lineStr, -1)
		for _, match := range matches {
			// Make sure this isn't followed by a closing tag on the same line
			tagName := lineStr[match[2]:match[3]]
			if !regexp.MustCompile(`</`+tagName+`>`).MatchString(lineStr[match[1]:]) {
				errors = append(errors, ValidationError{
					LineNumber: i + 1,
					Column:     match[0] + 1,
					Line:       lineStr,
					ErrorType:  "SVG self-closing tag issue",
					Message:    fmt.Sprintf("SVG <%s> tag should be self-closing with />", tagName),
					Content:    lineStr[match[0]:match[1]],
				})
			}
		}
		
		// Check for unquoted SVG attributes
		matches = reSVGUnquotedAttr.FindAllStringSubmatchIndex(lineStr, -1)
		for _, match := range matches {
			attrName := lineStr[match[2]:match[3]]
			attrValue := lineStr[match[4]:match[5]]
			errors = append(errors, ValidationError{
				LineNumber: i + 1,
				Column:     match[2] + 1,
				Line:       lineStr,
				ErrorType:  "SVG unquoted attribute",
				Message:    fmt.Sprintf("SVG attribute %s=%s should use quotes: %s=\"%s\"", attrName, attrValue, attrName, attrValue),
				Content:    attrName + "=" + attrValue,
			})
		}
		
		// Stop if we've reached max errors
		if opts.MaxErrors > 0 && len(errors) >= opts.MaxErrors {
			break
		}
	}
	
	return errors
}

// findErrorPosition converts a byte offset to line/column
func findErrorPosition(content []byte, offset int) (line, col int, lineContent string) {
	// Default values
	line = 1
	col = 1
	
	// Handle invalid offset
	if offset < 0 || offset >= len(content) {
		return line, col, ""
	}
	
	// Count lines and columns up to the offset
	for i := 0; i < offset; i++ {
		if content[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	
	// Extract the line content
	scanner := bufio.NewScanner(bytes.NewReader(content))
	currentLine := 1
	for scanner.Scan() {
		if currentLine == line {
			lineContent = scanner.Text()
			break
		}
		currentLine++
	}
	
	return line, col, lineContent
}

// displayError formats and prints a single validation error
func displayError(content []byte, err ValidationError, index int) {
	fmt.Printf("\n%s #%d:\n", headerColor("Issue"), index)
	fmt.Printf("%s %d, %s %d: %s\n", 
		infoColor("Line"), err.LineNumber, 
		infoColor("Column"), err.Column, 
		errorColor(err.ErrorType))
	fmt.Printf("%s %s\n", infoColor("Message:"), highlightColor(err.Message))
	
	// Show context (lines before and after the error)
	fmt.Printf("\n%s\n", infoColor("Context:"))
	fmt.Println(headerColor("----------------------------------------"))
	
	scanner := bufio.NewScanner(bytes.NewReader(content))
	lineNum := 1
	contextStart := err.LineNumber - 2
	if contextStart < 1 {
		contextStart = 1
	}
	contextEnd := err.LineNumber + 2
	
	for scanner.Scan() {
		if lineNum >= contextStart && lineNum <= contextEnd {
			line := scanner.Text()
			
			// Use different color for the line with the error
			if lineNum == err.LineNumber {
				fmt.Printf("%s: %s\n", infoColor(fmt.Sprintf("%4d", lineNum)), highlightColor(line))
			} else {
				fmt.Printf("%s: %s\n", infoColor(fmt.Sprintf("%4d", lineNum)), line)
			}
			
			// If this is the error line, add a pointer
			if lineNum == err.LineNumber && err.Column > 0 {
				pointer := strings.Repeat(" ", err.Column+5) + errorColor("^")
				if len(err.Content) > 1 {
					// For multi-character errors, extend the pointer
					pointer += errorColor(strings.Repeat("~", len(err.Content)-1))
				}
				fmt.Println(pointer)
			}
		}
		lineNum++
		if lineNum > contextEnd {
			break
		}
	}
	
	fmt.Println(headerColor("----------------------------------------"))
}

// printCorrectionTips prints common correction suggestions
func printCorrectionTips() {
	fmt.Printf("\n%s\n", headerColor("Common XML issues detected by this validator:"))
	fmt.Printf("  - %s\n", highlightColor("Special characters immediately after <![CDATA[ marker"))
	fmt.Printf("  - %s\n", highlightColor("Unescaped ']]>' sequences within CDATA content"))
	fmt.Printf("  - %s\n", highlightColor("Unclosed CDATA sections (missing ]]>)"))
	fmt.Printf("  - %s\n", highlightColor("Nested CDATA sections (not allowed in XML)"))
	fmt.Printf("  - %s\n", highlightColor("Control characters (non-printable ASCII 0-31) in CDATA sections"))
	fmt.Printf("  - %s\n", highlightColor("Malformed hex color codes (should be #RGB, #RRGGBB, or #RRGGBBAA)"))
	fmt.Printf("  - %s\n", highlightColor("Improperly closed SVG elements"))
	fmt.Printf("  - %s\n", highlightColor("SVG attributes without proper quoting"))
	
	fmt.Printf("\n%s\n", headerColor("Correction tips:"))
	fmt.Printf("  - %s: <![CDATA[content]]> with no special characters after opening marker\n", successColor("CDATA sections"))
	fmt.Printf("  - %s: Use standard formats like #RGB, #RRGGBB, #RRGGBBAA\n", successColor("Hex colors"))
	fmt.Printf("  - %s: Self-closing tags must end with />\n", successColor("SVG elements"))
	fmt.Printf("  - %s: Always use quotes for attribute values: width=\"100\"\n", successColor("SVG attributes"))
	fmt.Printf("  - %s: Remove them with:\n    %s\n", 
		successColor("Control characters"), 
		infoColor("go run xml_fixer.go yourfile.xml"))
	
	fmt.Printf("\n%s\n", highlightColor("For WordPress import files, CDATA errors are particularly important to fix."))
} 