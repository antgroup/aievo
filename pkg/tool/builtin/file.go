// Package builtin: FileRead / FileWrite / FileEdit / FileMultiEdit / Glob / Grep / Bash.
//
// Concurrency-safety contract:
//
//	read-y      → safe (run in parallel)
//	write-y     → unsafe (serial)
//	shell exec  → unsafe
//
// Inspired by Claude Code's File* tool family (book ch. 4).
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	aierrs "github.com/samson-samson/aievo-next/pkg/errors"
	"github.com/samson-samson/aievo-next/pkg/tool"
)

// ===== FileRead =====

type FileRead struct{}

func (FileRead) Name() string             { return "FileRead" }
func (FileRead) Description() string      { return `Read the contents of a file. Input: {"path":"..."} (optional: "offset":N,"limit":N to read a line range).` }
func (FileRead) IsConcurrencySafe() bool  { return true }
func (FileRead) Schema() []byte {
	return []byte(`{"type":"object","properties":{"path":{"type":"string"},"offset":{"type":"integer"},"limit":{"type":"integer"}},"required":["path"]}`)
}
func (FileRead) Call(ctx context.Context, input string) (string, error) {
	var in struct {
		Path   string
		Offset int
		Limit  int
	}
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", aierrs.NewFatal(fmt.Errorf("FileRead: %w", err))
	}
	data, err := os.ReadFile(in.Path)
	if err != nil {
		return "", aierrs.NewToolError("FileRead", err, false)
	}
	if in.Offset == 0 && in.Limit == 0 {
		return string(data), nil
	}
	lines := strings.Split(string(data), "\n")
	if in.Offset < 0 {
		in.Offset = 0
	}
	if in.Offset >= len(lines) {
		return "", nil
	}
	end := len(lines)
	if in.Limit > 0 && in.Offset+in.Limit < end {
		end = in.Offset + in.Limit
	}
	return strings.Join(lines[in.Offset:end], "\n"), nil
}

// ===== FileWrite =====

type FileWrite struct{}

func (FileWrite) Name() string             { return "FileWrite" }
func (FileWrite) Description() string      { return `Write contents to a file (creating dirs as needed). Input: {"path":"...","content":"..."}.` }
func (FileWrite) IsConcurrencySafe() bool  { return false }
func (FileWrite) Schema() []byte {
	return []byte(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`)
}
func (FileWrite) Call(ctx context.Context, input string) (string, error) {
	var in struct{ Path, Content string }
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", aierrs.NewFatal(fmt.Errorf("FileWrite: %w", err))
	}
	if err := os.MkdirAll(filepath.Dir(in.Path), 0o755); err != nil {
		return "", aierrs.NewToolError("FileWrite", err, false)
	}
	if err := os.WriteFile(in.Path, []byte(in.Content), 0o644); err != nil {
		return "", aierrs.NewToolError("FileWrite", err, false)
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(in.Content), in.Path), nil
}

// ===== FileEdit =====
// Replace old_string with new_string in a file. Fails if old_string is absent
// or appears more than once (use FileMultiEdit for multiple occurrences).

type FileEdit struct{}

func (FileEdit) Name() string             { return "FileEdit" }
func (FileEdit) Description() string      { return `Replace one exact occurrence of old_string with new_string. Input: {"path":"...","old_string":"...","new_string":"..."}.` }
func (FileEdit) IsConcurrencySafe() bool  { return false }
func (FileEdit) Schema() []byte {
	return []byte(`{"type":"object","properties":{"path":{"type":"string"},"old_string":{"type":"string"},"new_string":{"type":"string"}},"required":["path","old_string","new_string"]}`)
}
func (FileEdit) Call(ctx context.Context, input string) (string, error) {
	var in struct {
		Path       string
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
	}
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", aierrs.NewFatal(fmt.Errorf("FileEdit: %w", err))
	}
	data, err := os.ReadFile(in.Path)
	if err != nil {
		return "", aierrs.NewToolError("FileEdit", err, false)
	}
	body := string(data)
	count := strings.Count(body, in.OldString)
	if count == 0 {
		return "", aierrs.NewFatal(fmt.Errorf("FileEdit: old_string not found in %s", in.Path))
	}
	if count > 1 {
		return "", aierrs.NewFatal(fmt.Errorf("FileEdit: old_string matches %d times; tighten the snippet or use FileMultiEdit", count))
	}
	out := strings.Replace(body, in.OldString, in.NewString, 1)
	if err := os.WriteFile(in.Path, []byte(out), 0o644); err != nil {
		return "", aierrs.NewToolError("FileEdit", err, false)
	}
	return fmt.Sprintf("edited %s (delta %+d bytes)", in.Path, len(out)-len(body)), nil
}

// ===== FileMultiEdit =====
// Apply a sequence of edits to one file, atomically.

type FileMultiEdit struct{}

type editOp struct {
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
	ReplaceAll bool  `json:"replace_all,omitempty"`
}

func (FileMultiEdit) Name() string             { return "FileMultiEdit" }
func (FileMultiEdit) Description() string      { return `Apply multiple edits to one file atomically. Input: {"path":"...","edits":[{"old_string":"...","new_string":"...","replace_all":false}]}.` }
func (FileMultiEdit) IsConcurrencySafe() bool  { return false }
func (FileMultiEdit) Schema() []byte {
	return []byte(`{"type":"object","properties":{"path":{"type":"string"},"edits":{"type":"array"}},"required":["path","edits"]}`)
}
func (FileMultiEdit) Call(ctx context.Context, input string) (string, error) {
	var in struct {
		Path  string
		Edits []editOp
	}
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", aierrs.NewFatal(fmt.Errorf("FileMultiEdit: %w", err))
	}
	data, err := os.ReadFile(in.Path)
	if err != nil {
		return "", aierrs.NewToolError("FileMultiEdit", err, false)
	}
	body := string(data)
	for i, op := range in.Edits {
		if !strings.Contains(body, op.OldString) {
			return "", aierrs.NewFatal(fmt.Errorf("FileMultiEdit: edit %d old_string not found", i))
		}
		if op.ReplaceAll {
			body = strings.ReplaceAll(body, op.OldString, op.NewString)
		} else {
			body = strings.Replace(body, op.OldString, op.NewString, 1)
		}
	}
	if err := os.WriteFile(in.Path, []byte(body), 0o644); err != nil {
		return "", aierrs.NewToolError("FileMultiEdit", err, false)
	}
	return fmt.Sprintf("applied %d edits to %s", len(in.Edits), in.Path), nil
}

// ===== Glob =====

type Glob struct{}

func (Glob) Name() string             { return "Glob" }
func (Glob) Description() string      { return `Find files matching a glob pattern, sorted by modification time. Input: {"pattern":"**/*.go","root":"."}.` }
func (Glob) IsConcurrencySafe() bool  { return true }
func (Glob) Schema() []byte {
	return []byte(`{"type":"object","properties":{"pattern":{"type":"string"},"root":{"type":"string"}},"required":["pattern"]}`)
}
func (Glob) Call(ctx context.Context, input string) (string, error) {
	var in struct{ Pattern, Root string }
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", aierrs.NewFatal(fmt.Errorf("Glob: %w", err))
	}
	if in.Root == "" {
		in.Root = "."
	}
	type hit struct {
		path string
		mod  int64
	}
	var hits []hit
	err := filepath.WalkDir(in.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if matched, _ := doublestarMatch(in.Pattern, p); matched {
			info, _ := d.Info()
			hits = append(hits, hit{p, info.ModTime().Unix()})
		}
		return nil
	})
	if err != nil {
		return "", aierrs.NewToolError("Glob", err, false)
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].mod > hits[j].mod })
	var out strings.Builder
	for _, h := range hits {
		out.WriteString(h.path)
		out.WriteByte('\n')
	}
	return out.String(), nil
}

// doublestarMatch supports ** (any path segment) and standard * / ? globs.
// Translated to regexp for simplicity (small enough for example use).
func doublestarMatch(pattern, path string) (bool, error) {
	// Quick path: stdlib supports * / ? but not **.
	if !strings.Contains(pattern, "**") {
		return filepath.Match(pattern, filepath.Base(path))
	}
	rx := globToRegex(pattern)
	re, err := regexp.Compile(rx)
	if err != nil {
		return false, err
	}
	return re.MatchString(filepath.ToSlash(path)), nil
}

func globToRegex(p string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(p); i++ {
		switch p[i] {
		case '*':
			if i+1 < len(p) && p[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '|', '[', ']', '{', '}', '^', '$':
			b.WriteByte('\\')
			b.WriteByte(p[i])
		default:
			b.WriteByte(p[i])
		}
	}
	b.WriteString("$")
	return b.String()
}

// ===== Grep =====

type Grep struct{}

func (Grep) Name() string             { return "Grep" }
func (Grep) Description() string      { return `Search files matching a glob for a regex. Input: {"pattern":"regex","glob":"**/*.go","root":"."}. Output: file:line:match.` }
func (Grep) IsConcurrencySafe() bool  { return true }
func (Grep) Schema() []byte {
	return []byte(`{"type":"object","properties":{"pattern":{"type":"string"},"glob":{"type":"string"},"root":{"type":"string"}},"required":["pattern"]}`)
}
func (Grep) Call(ctx context.Context, input string) (string, error) {
	var in struct{ Pattern, Glob, Root string }
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", aierrs.NewFatal(fmt.Errorf("Grep: %w", err))
	}
	if in.Root == "" {
		in.Root = "."
	}
	if in.Glob == "" {
		in.Glob = "**/*"
	}
	re, err := regexp.Compile(in.Pattern)
	if err != nil {
		return "", aierrs.NewFatal(fmt.Errorf("Grep: bad regex: %w", err))
	}
	var out strings.Builder
	walkErr := filepath.WalkDir(in.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || ctx.Err() != nil {
			return ctx.Err()
		}
		if ok, _ := doublestarMatch(in.Glob, p); !ok {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for lineno, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				fmt.Fprintf(&out, "%s:%d:%s\n", p, lineno+1, line)
			}
		}
		return nil
	})
	if walkErr != nil {
		return "", aierrs.NewToolError("Grep", walkErr, false)
	}
	return out.String(), nil
}

// ===== Bash =====

type Bash struct {
	Workdir string
}

func (Bash) Name() string             { return "Bash" }
func (Bash) Description() string      { return `Execute a shell command. Input: {"cmd":"..."}.` }
func (Bash) IsConcurrencySafe() bool  { return false }
func (Bash) Schema() []byte {
	return []byte(`{"type":"object","properties":{"cmd":{"type":"string"}},"required":["cmd"]}`)
}
func (b Bash) Call(ctx context.Context, input string) (string, error) {
	var in struct{ Cmd string }
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return "", aierrs.NewFatal(fmt.Errorf("Bash: %w", err))
	}
	if strings.TrimSpace(in.Cmd) == "" {
		return "", aierrs.NewFatal(fmt.Errorf("Bash: empty command"))
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", in.Cmd)
	if b.Workdir != "" {
		cmd.Dir = b.Workdir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), aierrs.NewToolError("Bash", err, false)
	}
	return string(out), nil
}

var (
	_ tool.Tool = FileRead{}
	_ tool.Tool = FileWrite{}
	_ tool.Tool = FileEdit{}
	_ tool.Tool = FileMultiEdit{}
	_ tool.Tool = Glob{}
	_ tool.Tool = Grep{}
	_ tool.Tool = Bash{}
)
