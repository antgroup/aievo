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

type AccommodationTool struct {
	data [][]string
	path string
}

var _ tool.Tool = &AccommodationTool{}

// NewAccommodationTool creates a new Accommodation search tool.
func NewAccommodationTool(opts ...Option) (*AccommodationTool, error) {
	options := &Options{
		DatabasePath: "../database",
	}
	for _, opt := range opts {
		opt(options)
	}

	accommodationPath := options.AccommodationsPath
	if accommodationPath == "" {
		accommodationPath = filepath.Join(options.DatabasePath, "accommodations/clean_accommodations_2022.csv")
	}

	tool := &AccommodationTool{
		path: accommodationPath,
	}

	if err := tool.loadData(); err != nil {
		return nil, fmt.Errorf("failed to load accommodation data: %w", err)
	}

	return tool, nil
}

func (t *AccommodationTool) loadData() error {
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

	t.data = records
	return nil
}

// Name returns the name of the tool.
func (t *AccommodationTool) Name() string {
	return "AccommodationSearch"
}

// Description returns the description of the tool.
func (t *AccommodationTool) Description() string {
	bytes, _ := json.Marshal(t.Schema())
	return `Discover accommodations in your desired city.
Search for hotel rooms and accommodations in cities.
Input must be json schema: ` + string(bytes) + `
Example Input: {"city": "Rome"}`
}

func (t *AccommodationTool) Schema() *tool.PropertiesSchema {
	return &tool.PropertiesSchema{
		Type: tool.TypeJson,
		Properties: map[string]tool.PropertySchema{
			"city": {
				Type:        tool.TypeString,
				Description: "The name of the city where you're seeking accommodation",
			},
		},
		Required: []string{"city"},
	}
}

func (t *AccommodationTool) Strict() bool {
	return true
}

// Call searches for accommodations.
func (t *AccommodationTool) Call(ctx context.Context, input string) (string, error) {
	var params map[string]interface{}
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		return "json unmarshal error, please try again", nil
	}

	city, ok := params["city"].(string)
	if !ok {
		return "city parameter is required", nil
	}

	return t.searchAccommodations(city)
}

func (t *AccommodationTool) searchAccommodations(city string) (string, error) {
	city = extractBeforeParenthesis(city)

	if len(t.data) == 0 {
		return "No accommodation data available", nil
	}

	// Find header indices
	headerMap := make(map[string]int)
	for i, col := range t.data[0] {
		headerMap[col] = i
	}

	var results [][]string
	header := []string{"NAME", "price", "room type", "house_rules", "minimum nights", "maximum occupancy", "review rate number", "city"}

	// Search for matching accommodations
	for i := 1; i < len(t.data); i++ {
		row := t.data[i]
		if len(row) <= headerMap["city"] {
			continue
		}

		if row[headerMap["city"]] == city {
			results = append(results, row)
		}
	}

	if len(results) == 0 {
		return "There is no accommodation in this city.", nil
	}

	// Format results
	output := "Found accommodations:\n"
	output += strings.Join(header, " | ") + "\n"
	for _, result := range results {
		selectedFields := []string{
			result[headerMap["NAME"]],
			result[headerMap["price"]],
			result[headerMap["room type"]],
			result[headerMap["house_rules"]],
			result[headerMap["minimum nights"]],
			result[headerMap["maximum occupancy"]],
			result[headerMap["review rate number"]],
			result[headerMap["city"]],
		}
		output += strings.Join(selectedFields, " | ") + "\n"
	}

	return output, nil
}
