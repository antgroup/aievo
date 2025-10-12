package bash

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/utils/json"
)

// platform returns the current operating system in a user-friendly format.
func platform() string {
	system := runtime.GOOS
	if system == "darwin" {
		return "MacOS"
	}
	return system
}

// Tool implements the tool.Tool interface for executing shell commands.
type Tool struct {
	AskHumanInput   bool
	PreExecCommands []string   // List of commands to execute before the main command
	mu              sync.Mutex // Protects AskHumanInput in concurrent scenarios
}

// New creates a new Tool instance with the provided options.
func New(opts ...Option) (*Tool, error) {
	options := NewDefaultOptions()

	for _, opt := range opts {
		opt(options)
	}

	return &Tool{
		AskHumanInput:   options.AskHumanInput,
		PreExecCommands: options.PreExecCommands,
	}, nil
}

// Name returns the name of the tool.
func (t *Tool) Name() string {
	return "terminal"
}

// Description returns a description of the tool, including input format and examples.
func (t *Tool) Description() string {
	bytes, _ := json.Marshal(t.Schema())
	return fmt.Sprintf("Run shell commands on this %s machine.", platform()) + `
Input Format:` + string(bytes) + `
Example Input: {\"command\": \"ls -l\"}
Example Input: "{\"command\": \"echo 'from typing import List\\nprint(\\\"test!\\\")' > test.py && python3 test.py\"}"`
}

// Example Input: {"command": "python -c 'print(\"hello world\")'}`
// Example Input: {\"command\": \"python -c 'print(\\\"All tests passed\\\")'\"}`
// Example Input: "{\"command\": \"echo 'from typing import List\\nprint(\\\"test!\\\")' > test.py && python3 test.py\"}"`

// Schema returns the JSON schema for the tool's input.
func (t *Tool) Schema() *tool.PropertiesSchema {
	return &tool.PropertiesSchema{
		Type: tool.TypeJson,
		Properties: map[string]tool.PropertySchema{
			"command": {
				Type:        tool.TypeString,
				Description: "The command to execute.",
			},
		},
		Required: []string{"command"},
	}
}

// Strict indicates whether the tool enforces strict input validation.
func (t *Tool) Strict() bool {
	return true
}

// Call executes the provided shell command and returns the output.
func (t *Tool) Call(_ context.Context, input string) (string, error) {
	var m map[string]interface{}

	// Parse the input JSON
	err := json.Unmarshal([]byte(input), &m)
	if err != nil {
		return fmt.Sprintf("invalid input format: %v", err), nil
	}

	// Validate the command field
	command, ok := m["command"].(string)
	if !ok || command == "" {
		return fmt.Sprintf("command is required and must be a non-empty string"), nil
	}

	// Ask for human confirmation if enabled
	if t.AskHumanInput {
		t.mu.Lock()
		defer t.mu.Unlock()

		fmt.Printf("Proceed with command execution? (y/n): ")
		var answer string
		_, err := fmt.Scanln(&answer)
		if err != nil {
			return fmt.Sprintf("failed to read user input: %v", err), nil
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" {
			return "command execution aborted by user", nil
		}
	}

	// Combine pre-execution commands and the main command into a single shell session
	var fullCommand string
	if len(t.PreExecCommands) > 0 {
		// Redirect pre-execution command output to /dev/null (or NUL on Windows)
		for _, preCmd := range t.PreExecCommands {
			if runtime.GOOS == "windows" {
				fullCommand += fmt.Sprintf("%s > NUL 2>&1 && ", preCmd)
			} else {
				fullCommand += fmt.Sprintf("%s > /dev/null 2>&1 && ", preCmd)
			}
		}
		fullCommand += command // Append the main command
	} else {
		fullCommand = command
	}

	fmt.Println(fullCommand)

	// Execute the combined command in a single shell session
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", fullCommand)
	} else {
		cmd = exec.Command("bash", "-c", fullCommand)
	}

	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()
	defer stdoutPipe.Close()
	defer stderrPipe.Close()

	// execute command
	err = cmd.Start()
	if err != nil {
		return fmt.Sprintf("failed to execute command: %v", err), nil
	}

	readPipe := func(pipe io.ReadCloser) (string, error) {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, pipe)
		if err != nil {
			return "", fmt.Errorf("failed to read from pipe: %v", err)
		}
		return buf.String(), nil
	}

	stdout, _ := readPipe(stdoutPipe)
	stderr, _ := readPipe(stderrPipe)

	return stdout + stderr, nil
}
