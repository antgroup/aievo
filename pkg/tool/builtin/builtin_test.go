package builtin

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileWriteReadEdit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	if _, err := (FileWrite{}).Call(context.Background(), `{"path":"`+path+`","content":"hello world"}`); err != nil {
		t.Fatal(err)
	}
	got, err := (FileRead{}).Call(context.Background(), `{"path":"`+path+`"}`)
	if err != nil || got != "hello world" {
		t.Fatalf("read got=%q err=%v", got, err)
	}
	if _, err := (FileEdit{}).Call(context.Background(),
		`{"path":"`+path+`","old_string":"world","new_string":"there"}`); err != nil {
		t.Fatal(err)
	}
	got, _ = (FileRead{}).Call(context.Background(), `{"path":"`+path+`"}`)
	if got != "hello there" {
		t.Fatalf("after edit got=%q", got)
	}
}

func TestFileEdit_RejectsAmbiguous(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	_, _ = (FileWrite{}).Call(context.Background(), `{"path":"`+path+`","content":"a a a"}`)
	_, err := (FileEdit{}).Call(context.Background(),
		`{"path":"`+path+`","old_string":"a","new_string":"X"}`)
	if err == nil {
		t.Fatal("expected ambiguous-match error")
	}
}

func TestFileMultiEdit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	_, _ = (FileWrite{}).Call(context.Background(), `{"path":"`+path+`","content":"foo bar baz"}`)
	_, err := (FileMultiEdit{}).Call(context.Background(),
		`{"path":"`+path+`","edits":[{"old_string":"foo","new_string":"FOO"},{"old_string":"baz","new_string":"BAZ"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := (FileRead{}).Call(context.Background(), `{"path":"`+path+`"}`)
	if got != "FOO bar BAZ" {
		t.Fatalf("got=%q", got)
	}
}

func TestGlob_MatchesDoublestar(t *testing.T) {
	dir := t.TempDir()
	_ = writeAll(dir, "sub/a.go", "sub/b.txt", "a.go")
	out, err := (Glob{}).Call(context.Background(), `{"pattern":"**/*.go","root":"`+dir+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "sub/a.go") {
		t.Fatalf("Glob missed files: %q", out)
	}
	if strings.Contains(out, "b.txt") {
		t.Fatalf("Glob matched non-.go: %q", out)
	}
}

func TestGrep_FindsLines(t *testing.T) {
	dir := t.TempDir()
	_ = writeAll(dir, "x.txt", "y.txt")
	out, err := (Grep{}).Call(context.Background(),
		`{"pattern":"hello","glob":"**/*.txt","root":"`+dir+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("Grep failed: %q", out)
	}
}

func TestBashEcho(t *testing.T) {
	out, err := (Bash{}).Call(context.Background(), `{"cmd":"echo aievo-next"}`)
	if err != nil || out != "aievo-next\n" {
		t.Fatalf("got=%q err=%v", out, err)
	}
}

func TestWebFetch_Stub(t *testing.T) {
	// Don't hit the real internet; verify URL validation path.
	_, err := (WebFetch{}).Call(context.Background(), `{"url":"::not-a-url::"}`)
	if err == nil {
		t.Fatal("expected url parse error")
	}
}

func TestTaskStore(t *testing.T) {
	store := NewTaskStore()
	tools := store.Tools()
	create, update, list := tools[0], tools[1], tools[2]
	out, err := create.Call(context.Background(), `{"subject":"do X"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"pending"`) {
		t.Fatalf("create out: %q", out)
	}
	if _, err := update.Call(context.Background(), `{"id":"1","status":"completed"}`); err != nil {
		t.Fatal(err)
	}
	lst, _ := list.Call(context.Background(), `{}`)
	if !strings.Contains(lst, `"completed"`) {
		t.Fatalf("list: %q", lst)
	}
}

func TestPlanState(t *testing.T) {
	st := NewPlanState()
	tools := st.Tools()
	enter, exit := tools[0], tools[1]
	if st.Active() {
		t.Fatal("should start inactive")
	}
	_, _ = enter.Call(context.Background(), `{}`)
	if !st.Active() {
		t.Fatal("enter should activate")
	}
	_, _ = exit.Call(context.Background(), `{"plan":"my plan"}`)
	if st.Active() {
		t.Fatal("exit should deactivate")
	}
	if st.Plan() != "my plan" {
		t.Fatalf("plan = %q", st.Plan())
	}
}

func TestConcurrencySafeFlags(t *testing.T) {
	cases := []struct {
		name string
		safe bool
	}{
		{"FileRead", FileRead{}.IsConcurrencySafe()},
		{"FileWrite", FileWrite{}.IsConcurrencySafe()},
		{"FileEdit", FileEdit{}.IsConcurrencySafe()},
		{"FileMultiEdit", FileMultiEdit{}.IsConcurrencySafe()},
		{"Glob", Glob{}.IsConcurrencySafe()},
		{"Grep", Grep{}.IsConcurrencySafe()},
		{"Bash", Bash{}.IsConcurrencySafe()},
		{"WebFetch", WebFetch{}.IsConcurrencySafe()},
		{"WebSearch", WebSearch{}.IsConcurrencySafe()},
	}
	want := map[string]bool{
		"FileRead": true, "FileWrite": false, "FileEdit": false, "FileMultiEdit": false,
		"Glob": true, "Grep": true, "Bash": false, "WebFetch": true, "WebSearch": true,
	}
	for _, c := range cases {
		if c.safe != want[c.name] {
			t.Errorf("%s safe=%v want=%v", c.name, c.safe, want[c.name])
		}
	}
}

func writeAll(dir string, names ...string) error {
	for _, n := range names {
		_, err := (FileWrite{}).Call(context.Background(),
			`{"path":"`+dir+`/`+n+`","content":"hello world"}`)
		if err != nil {
			return err
		}
	}
	return nil
}
