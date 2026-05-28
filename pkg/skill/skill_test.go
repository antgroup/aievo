package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDir_FrontmatterParsed(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "hello-world", `---
description: greet the user warmly
trigger: when asked to say hi
---

# Hello World

When user asks to greet, respond enthusiastically.
`)
	skills, err := LoadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills", len(skills))
	}
	s := skills[0]
	if s.Name != "hello-world" {
		t.Errorf("name = %q", s.Name)
	}
	if s.Description != "greet the user warmly" {
		t.Errorf("desc = %q", s.Description)
	}
	if s.Trigger != "when asked to say hi" {
		t.Errorf("trigger = %q", s.Trigger)
	}
	if !strings.Contains(s.Body, "respond enthusiastically") {
		t.Errorf("body missing expected text: %q", s.Body)
	}
}

func TestLoadDir_NoFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "minimal", "# Minimal Skill\n\nDo something.\n")
	skills, _ := LoadDir(root)
	if len(skills) != 1 {
		t.Fatal("expected 1 skill")
	}
	if skills[0].Description != "Minimal Skill" {
		t.Errorf("description fallback wrong: %q", skills[0].Description)
	}
}

func TestLoadDir_MissingDir(t *testing.T) {
	skills, err := LoadDir("/nonexistent/path/never")
	if err != nil {
		t.Fatal(err)
	}
	if skills != nil {
		t.Fatal("expected nil slice for missing dir")
	}
}

func TestFormatForPrompt(t *testing.T) {
	out := FormatForPrompt([]Skill{
		{Name: "a", Description: "first", Body: "body A"},
		{Name: "b", Description: "second", Body: "body B"},
	})
	if !strings.Contains(out, "## a") || !strings.Contains(out, "## b") {
		t.Fatalf("missing headings: %s", out)
	}
}

func TestLoadAll_ProjectOverridesUser(t *testing.T) {
	rootProj := t.TempDir()
	rootUser := t.TempDir()
	writeSkill(t, rootProj, "same", "---\ndescription: project\n---\n")
	writeSkill(t, rootUser, "same", "---\ndescription: user\n---\n")
	skills, err := LoadAll([]string{rootProj, rootUser})
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 (dedup), got %d", len(skills))
	}
	if skills[0].Description != "project" {
		t.Fatalf("project should win: %q", skills[0].Description)
	}
}
