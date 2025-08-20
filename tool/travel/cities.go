package travel

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/utils/json"
)

type CityTool struct {
	data map[string][]string
	path string
}

var _ tool.Tool = &CityTool{}

// NewCityTool creates a new City search tool.
func NewCityTool(opts ...Option) (*CityTool, error) {
	options := &Options{
		DatabasePath: "../database",
	}
	for _, opt := range opts {
		opt(options)
	}

	cityPath := options.CitiesPath
	if cityPath == "" {
		cityPath = filepath.Join(options.DatabasePath, "background/citySet_with_states.txt")
	}

	tool := &CityTool{
		path: cityPath,
		data: make(map[string][]string),
	}

	if err := tool.loadData(); err != nil {
		return nil, fmt.Errorf("failed to load city data: %w", err)
	}

	return tool, nil
}

func (t *CityTool) loadData() error {
	file, err := os.Open(t.path)
	if err != nil {
		return err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) == 2 {
			city, state := parts[0], parts[1]
			if _, exists := t.data[state]; !exists {
				t.data[state] = []string{}
			}
			t.data[state] = append(t.data[state], city)
		}
	}

	return nil
}

// Name returns the name of the tool.
func (t *CityTool) Name() string {
	return "CitySearch"
}

// Description returns the description of the tool.
func (t *CityTool) Description() string {
	bytes, _ := json.Marshal(t.Schema())
	return `Find cities in a state of your choice.
Search for cities within a specific state.
Input must be json schema: ` + string(bytes) + `
Example Input: {"state": "California"}`
}

func (t *CityTool) Schema() *tool.PropertiesSchema {
	return &tool.PropertiesSchema{
		Type: tool.TypeJson,
		Properties: map[string]tool.PropertySchema{
			"state": {
				Type:        tool.TypeString,
				Description: "The name of the state where you're seeking cities",
			},
		},
		Required: []string{"state"},
	}
}

func (t *CityTool) Strict() bool {
	return true
}

// Call searches for cities within a state.
func (t *CityTool) Call(ctx context.Context, input string) (string, error) {
	var params map[string]interface{}
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		return "json unmarshal error, please try again", nil
	}

	state, ok := params["state"].(string)
	if !ok {
		return "state parameter is required", nil
	}

	return t.searchCities(state)
}

func (t *CityTool) searchCities(state string) (string, error) {
	cities, exists := t.data[state]
	if !exists {
		return fmt.Sprintf("Invalid State: %s", state), nil
	}

	if len(cities) == 0 {
		return fmt.Sprintf("No cities found in state: %s", state), nil
	}

	output := fmt.Sprintf("Cities in %s:\n", state)
	for _, city := range cities {
		output += "- " + city + "\n"
	}

	return output, nil
}
