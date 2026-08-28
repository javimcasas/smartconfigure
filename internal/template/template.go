package template

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var varPattern = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}`)

// Template holds the parsed command template: the ordered, non-blank lines
// to send over the SSH session, and the set of {VARIABLE} names referenced
// in them (in first-seen order, deduplicated).
type Template struct {
	Lines     []string
	Variables []string
}

// Load reads a template file. Blank lines are treated as formatting only
// (for human readability in the template file) and are never sent to the
// device. Every other line is sent verbatim, with {VARIABLE} placeholders
// substituted per device before sending.
func Load(path string) (*Template, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	seen := map[string]bool{}
	var vars []string

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
		for _, m := range varPattern.FindAllStringSubmatch(line, -1) {
			name := m[1]
			if !seen[name] {
				seen[name] = true
				vars = append(vars, name)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("template file %q has no command lines", path)
	}

	return &Template{Lines: lines, Variables: vars}, nil
}

// Render substitutes {VARIABLE} placeholders in a line using the given
// values map. A placeholder with no matching value is left untouched, so
// it's visible (and greppable) in the session log instead of silently
// turning into an empty string.
func Render(line string, values map[string]string) string {
	return varPattern.ReplaceAllStringFunc(line, func(match string) string {
		name := match[1 : len(match)-1]
		if v, ok := values[name]; ok {
			return v
		}
		return match
	})
}

// HasUnresolvedPlaceholder reports whether s still contains a {VARIABLE}
// placeholder after Render — i.e. Render had no value for it. This is the
// building block for validating a template+Excel pairing (via --dry-run)
// before ever connecting to a device.
func HasUnresolvedPlaceholder(s string) bool {
	return varPattern.MatchString(s)
}
