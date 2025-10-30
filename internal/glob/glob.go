package glob

import (
	"regexp"
	"strings"
)

// splitBraceContent splits brace content by commas, respecting nested braces
func splitBraceContent(content string) []string {
	var parts []string
	var current strings.Builder
	depth := 0

	for i := 0; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
			current.WriteByte(content[i])
		case '}':
			depth--
			current.WriteByte(content[i])
		case ',':
			if depth == 0 {
				// Top-level comma, split here
				parts = append(parts, current.String())
				current.Reset()
			} else {
				// Nested comma, keep it
				current.WriteByte(content[i])
			}
		default:
			current.WriteByte(content[i])
		}
	}

	// Add the last part
	if current.Len() > 0 || len(parts) > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// ToRegex converts a glob pattern to a regular expression string
func ToRegex(pattern string) string {
	var result strings.Builder
	result.WriteString("^")

	i := 0
	for i < len(pattern) {
		switch pattern[i] {
		case '*':
			// Check for ** (recursive match)
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				// ** matches zero or more directories
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					result.WriteString("(.*/)?")
					i += 3 // skip **/
				} else {
					result.WriteString(".*")
					i += 2 // skip **
				}
			} else {
				// * matches anything except path separator
				result.WriteString("[^/]*")
				i++
			}

		case '?':
			// ? matches any single character except path separator
			result.WriteString("[^/]")
			i++

		case '[':
			// Character class - find the closing ]
			j := i + 1
			// Check if we have a negation
			negated := false
			if j < len(pattern) && pattern[j] == '!' {
				negated = true
				j++
			}

			// Find closing bracket
			closingBracket := j
			for closingBracket < len(pattern) && pattern[closingBracket] != ']' {
				closingBracket++
			}

			if closingBracket < len(pattern) {
				// Valid character class
				if negated {
					result.WriteString("[^")
				} else {
					result.WriteString("[")
				}

				// Copy content, skipping path separators
				for j < closingBracket {
					if pattern[j] == '/' {
						j++
						continue
					}
					result.WriteByte(pattern[j])
					j++
				}

				result.WriteString("]")
				i = closingBracket + 1
			} else {
				// Unclosed bracket, treat as literal
				result.WriteString("\\[")
				i++
			}

		case '{':
			// Brace expansion {a,b,c} -> (a|b|c)
			j := i + 1
			braceContent := ""
			depth := 1

			for j < len(pattern) && depth > 0 {
				if pattern[j] == '{' {
					depth++
				} else if pattern[j] == '}' {
					depth--
				}
				if depth > 0 {
					braceContent += string(pattern[j])
				}
				j++
			}

			if depth == 0 {
				// Valid brace expression - split by commas, respecting nesting
				parts := splitBraceContent(braceContent)
				result.WriteString("(")
				for k, part := range parts {
					if k > 0 {
						result.WriteString("|")
					}
					// Recursively process each part
					innerRegex := ToRegex(part)
					// Remove ^ and $ from inner pattern
					innerRegex = strings.TrimPrefix(innerRegex, "^")
					innerRegex = strings.TrimSuffix(innerRegex, "$")
					result.WriteString(innerRegex)
				}
				result.WriteString(")")
				i = j
			} else {
				// Unclosed brace, treat as literal
				result.WriteString("\\{")
				i++
			}

		case '\\':
			// Escape next character
			if i+1 < len(pattern) {
				i++
				result.WriteString(regexp.QuoteMeta(string(pattern[i])))
				i++
			} else {
				result.WriteString("\\\\")
				i++
			}

		case '.', '+', '(', ')', '|', '^', '$':
			// Escape regex special characters
			result.WriteString("\\" + string(pattern[i]))
			i++

		default:
			// Regular character
			result.WriteByte(pattern[i])
			i++
		}
	}

	result.WriteString("$")
	return result.String()
}

// Match matches a string against a glob pattern
func Match(pattern, str string) (bool, error) {
	regexPattern := ToRegex(pattern)
	return regexp.MatchString(regexPattern, str)
}
