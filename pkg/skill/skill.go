// Package skill loads agent "skills" — directories containing a SKILL.md
// markdown file with YAML frontmatter. Inspired by Claude Code's Skill
// system (book ch. 12).
//
// A skill is plug-in capability described in natural language. The loader
// reads matching skills from one or more roots (e.g. ~/.aievo/skills/ and
// the project's .aievo/skills/) and returns them as []Skill values that
// callers inject into the system prompt.
package skill

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Skill is one loaded SKILL.md.
type Skill struct {
	Name        string // directory name
	Description string // from frontmatter `description:`
	Trigger     string // from frontmatter `trigger:` (optional)
	Body        string // markdown body (no frontmatter)
	Path        string // absolute path to SKILL.md
}

// LoadDir reads every <root>/<name>/SKILL.md and returns the skills found.
// Skips entries that don't have a SKILL.md (so a root with mixed contents
// is fine).
func LoadDir(root string) ([]Skill, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(root, e.Name(), "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		s, err := LoadFile(path)
		if err != nil {
			return nil, fmt.Errorf("skill %s: %w", e.Name(), err)
		}
		s.Name = e.Name()
		out = append(out, s)
	}
	return out, nil
}

// LoadFile parses one SKILL.md, splitting YAML frontmatter from body.
func LoadFile(path string) (Skill, error) {
	f, err := os.Open(path)
	if err != nil {
		return Skill{}, err
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	s := Skill{Path: path}
	var (
		inFront bool
		bodyBuf strings.Builder
		gotFM   bool
	)
	lineNo := 0
	for scan.Scan() {
		lineNo++
		line := scan.Text()
		if lineNo == 1 && line == "---" {
			inFront = true
			gotFM = true
			continue
		}
		if inFront {
			if line == "---" {
				inFront = false
				continue
			}
			k, v, ok := splitKV(line)
			if !ok {
				continue
			}
			switch strings.ToLower(k) {
			case "description":
				s.Description = v
			case "trigger":
				s.Trigger = v
			case "name":
				s.Name = v
			}
			continue
		}
		bodyBuf.WriteString(line)
		bodyBuf.WriteByte('\n')
	}
	if err := scan.Err(); err != nil {
		return Skill{}, err
	}
	if !gotFM {
		// No frontmatter — treat the whole file as body, with the
		// first non-empty line as the description.
		bodyBuf.Reset()
		f2, _ := os.Open(path)
		defer f2.Close()
		all, _ := os.ReadFile(path)
		s.Body = string(all)
		for _, line := range strings.Split(s.Body, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				s.Description = strings.TrimLeft(line, "# ")
				break
			}
		}
		return s, nil
	}
	s.Body = strings.TrimSpace(bodyBuf.String())
	return s, nil
}

func splitKV(line string) (string, string, bool) {
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return "", "", false
	}
	k := strings.TrimSpace(line[:i])
	v := strings.TrimSpace(line[i+1:])
	v = strings.Trim(v, `"'`)
	return k, v, k != ""
}

// FormatForPrompt renders skills into a single system-prompt section. The
// model sees skill metadata + body; tool selection still happens through
// the regular tool list (skills don't auto-register tools).
func FormatForPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# Available Skills\n\n")
	for _, s := range skills {
		fmt.Fprintf(&b, "## %s\n", s.Name)
		if s.Description != "" {
			fmt.Fprintf(&b, "*%s*\n\n", s.Description)
		}
		if s.Trigger != "" {
			fmt.Fprintf(&b, "Trigger: %s\n\n", s.Trigger)
		}
		b.WriteString(s.Body)
		b.WriteString("\n\n")
	}
	return b.String()
}

// DefaultRoots returns the conventional search roots, in priority order.
// Project-local skills override user-global skills with the same name.
func DefaultRoots() []string {
	home, _ := os.UserHomeDir()
	roots := []string{
		filepath.Join(".aievo", "skills"),
	}
	if home != "" {
		roots = append(roots, filepath.Join(home, ".aievo", "skills"))
	}
	return roots
}

// LoadAll loads skills from every root, project taking priority over
// user-global (later roots are shadowed by earlier ones on name conflict).
func LoadAll(roots []string) ([]Skill, error) {
	seen := map[string]struct{}{}
	var out []Skill
	for _, r := range roots {
		ss, err := LoadDir(r)
		if err != nil {
			return nil, err
		}
		for _, s := range ss {
			if _, dup := seen[s.Name]; dup {
				continue
			}
			seen[s.Name] = struct{}{}
			out = append(out, s)
		}
	}
	return out, nil
}
