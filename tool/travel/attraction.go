package travel

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/utils/json"
)

type AttractionTool struct {
	data [][]string
	path string
}

var _ tool.Tool = &AttractionTool{}

// NewAttractionTool creates a new Attraction search tool.
func NewAttractionTool(opts ...Option) (*AttractionTool, error) {
	options := &Options{
		DatabasePath: "../database",
	}
	for _, opt := range opts {
		opt(options)
	}

	attractionPath := options.AttractionsPath
	if attractionPath == "" {
		attractionPath = filepath.Join(options.DatabasePath, "attractions/attractions.csv")
	}

	tool := &AttractionTool{
		path: attractionPath,
	}

	if err := tool.loadData(); err != nil {
		return nil, fmt.Errorf("failed to load attraction data: %w", err)
	}

	return tool, nil
}

func (t *AttractionTool) loadData() error {
	file, err := os.Open(t.path)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	// Filter out rows with empty/null values (equivalent to pandas dropna())
	filteredRecords := [][]string{}
	if len(records) > 0 {
		filteredRecords = append(filteredRecords, records[0]) // Keep header

		for i := 1; i < len(records); i++ {
			row := records[i]
			hasEmptyField := false

			// Check if any field is empty or contains only whitespace
			for _, field := range row {
				if strings.TrimSpace(field) == "" {
					hasEmptyField = true
					break
				}
			}

			// Only keep rows without empty fields
			if !hasEmptyField {
				filteredRecords = append(filteredRecords, row)
			}
		}
	}

	t.data = filteredRecords
	return nil
}

// Name returns the name of the tool.
func (t *AttractionTool) Name() string {
	return "AttractionSearch"
}

// Description returns the description of the tool.
func (t *AttractionTool) Description() string {
	bytes, _ := json.Marshal(t.Schema())
	return `Find attractions in a city of your choice.
Search for tourist attractions, landmarks, and points of interest in cities.
Input must be json schema: ` + string(bytes) + `
Example Input: {"city": "London"}`
}

func (t *AttractionTool) Schema() *tool.PropertiesSchema {
	return &tool.PropertiesSchema{
		Type: tool.TypeJson,
		Properties: map[string]tool.PropertySchema{
			"city": {
				Type:        tool.TypeString,
				Description: "The name of the city where you're seeking attractions",
			},
		},
		Required: []string{"city"},
	}
}

func (t *AttractionTool) Strict() bool {
	return true
}

// Call searches for attractions.
func (t *AttractionTool) Call(ctx context.Context, input string) (string, error) {
	var params map[string]interface{}
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		return "json unmarshal error, please try again", nil
	}

	city, ok := params["city"].(string)
	if !ok {
		return "city parameter is required", nil
	}

	return t.searchAttractions(city)
}

func (t *AttractionTool) searchAttractions(city string) (string, error) {
	city = extractBeforeParenthesis(city)

	if len(t.data) == 0 {
		return "No attraction data available", nil
	}

	// Find header indices
	headerMap := make(map[string]int)
	for i, col := range t.data[0] {
		headerMap[col] = i
	}

	var results [][]string
	header := []string{"Name", "Latitude", "Longitude", "Address", "Phone", "Website", "City"}

	// Search for matching attractions
	for i := 1; i < len(t.data); i++ {
		row := t.data[i]
		if len(row) <= headerMap["City"] {
			continue
		}

		if row[headerMap["City"]] == city {
			results = append(results, row)
		}
	}

	if len(results) == 0 {
		return "There is no attraction in this city.", nil
	}

	// Format results
	output := "Found attractions:\n"
	output += strings.Join(header, " | ") + "\n"
	for _, result := range results {
		selectedFields := []string{
			result[headerMap["Name"]],
			result[headerMap["Latitude"]],
			result[headerMap["Longitude"]],
			result[headerMap["Address"]],
			result[headerMap["Phone"]],
			result[headerMap["Website"]],
			result[headerMap["City"]],
		}
		output += strings.Join(selectedFields, " | ") + "\n"
	}

	return output, nil
}
