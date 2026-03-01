package config

import (
	"fmt"
	"strings"
)
type iniSections map[string]map[string]string
func parseINI(content string) (iniSections, error) {
	sections := make(iniSections)
	current := ""

	for lineNum, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)

		// Skip empty lines and full-line comments
		if line == "" || line[0] == '#' || line[0] == ';' {
			continue
		}

		// Section header: [section_name]
		if line[0] == '[' {
			end := strings.IndexByte(line, ']')
			if end < 0 {
				return nil, fmt.Errorf("line %d: missing closing ']'", lineNum+1)
			}
			current = strings.TrimSpace(line[1:end])
			if sections[current] == nil {
				sections[current] = make(map[string]string)
			}
			continue
		}

		// Key = value (strings.Cut splits on the first '=' found)
		key, val, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("line %d: expected 'key = value'", lineNum+1)
		}

		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		// Strip inline comments
		if comment, _, hasComment := strings.Cut(val, "#"); hasComment {
			val = strings.TrimSpace(comment)
		}

		if current == "" {
			return nil, fmt.Errorf("line %d: key '%s' outside of section", lineNum+1, key)
		}

		sections[current][key] = val
	}

	return sections, nil
}
