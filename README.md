# Go XML Validator

A comprehensive XML validation tool that checks for well-formedness and common issues often found in XML files, particularly WordPress export files.

**I created this using Cursor AI and Claude. This is very much an experiment for me.**

## Features

- Basic XML well-formedness validation
- CDATA section validation
  - Detect special characters immediately after CDATA opening
  - Find unclosed CDATA sections
  - Identify nested CDATA sections
  - Find multiple CDATA closing sequences
  - Detect empty CDATA sections
- Control character detection
- Hex color code validation
- SVG syntax validation
  - Self-closing tag issues
  - Unquoted attribute values

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/go-xml-validator.git
cd go-xml-validator

# Build the binary
go build -o xml-validator
```

## Usage

```bash
# Basic usage
./xml-validator path/to/file.xml

# Validate a remote XML file
./xml-validator https://example.com/file.xml

# Show more errors (default is 5)
./xml-validator --max-errors=10 path/to/file.xml

# Enable debug output
./xml-validator --debug path/to/file.xml
```

## Output

The tool provides detailed error reports with line numbers, context, and suggestions for fixing issues:

```
Validating XML: example.xml
Will report up to 5 errors
Basic XML validation passed. Performing additional checks...
Checking CDATA sections...
Checking for control characters...
Checking hex color codes...
Checking SVG syntax...
❌ Found 2 XML issues (showing up to 5):
----------------------------------------

Issue #1:
Line 42, Column 15: CDATA error
Message: Special character '!' found immediately after CDATA opening

Context:
----------------------------------------
  40: <content:encoded>
  41: <div class="content">
  42: <![CDATA[!-- This is a common error -->
                   ^
  43: <p>Some content here</p>
  44: ]]>
----------------------------------------

Issue #2:
Line 127, Column 25: Invalid hex color
Message: Invalid hex color code: #12 (should be #RGB, #RRGGBB, or #RRGGBBAA)

Context:
----------------------------------------
 125: <div class="styles">
 126: <style>
 127: .background-color { color: #12 }
                               ^~
 128: </style>
 129: </div>
----------------------------------------
```

## Why This Tool

Many XML validation tools only check for well-formedness, but miss common issues that can cause problems with XML processing, especially for WordPress imports. This tool is designed to catch these specific issues, making it easier to fix XML files before importing them.

## License

MIT 
