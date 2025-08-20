package travel

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/utils/json"
)

type DistanceTool struct {
	data [][]string
	path string
}

var _ tool.Tool = &DistanceTool{}

// NewDistanceTool creates a new Distance calculation tool.
func NewDistanceTool(opts ...Option) (*DistanceTool, error) {
	options := &Options{
		DatabasePath: "../database",
	}
	for _, opt := range opts {
		opt(options)
	}

	distancePath := options.DistancePath
	if distancePath == "" {
		distancePath = filepath.Join(options.DatabasePath, "googleDistanceMatrix/distance.csv")
	}

	tool := &DistanceTool{
		path: distancePath,
	}

	if err := tool.loadData(); err != nil {
		return nil, fmt.Errorf("failed to load distance data: %w", err)
	}

	return tool, nil
}

func (t *DistanceTool) loadData() error {
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
func (t *DistanceTool) Name() string {
	return "GoogleDistanceMatrix"
}

// Description returns the description of the tool.
func (t *DistanceTool) Description() string {
	bytes, _ := json.Marshal(t.Schema())
	return `Estimate the distance, time and cost between two cities.
Calculate travel information between origin and destination cities using different transportation modes.
Input must be json schema: ` + string(bytes) + `
Example Input: {"origin": "Paris", "destination": "Lyon", "mode": "self-driving"}`
}

func (t *DistanceTool) Schema() *tool.PropertiesSchema {
	return &tool.PropertiesSchema{
		Type: tool.TypeJson,
		Properties: map[string]tool.PropertySchema{
			"origin": {
				Type:        tool.TypeString,
				Description: "The departure city of your journey",
			},
			"destination": {
				Type:        tool.TypeString,
				Description: "The destination city of your journey",
			},
			"mode": {
				Type:        tool.TypeString,
				Description: "The method of transportation: 'self-driving' or 'taxi'",
				Enum:        []string{"self-driving", "taxi"},
			},
		},
		Required: []string{"origin", "destination", "mode"},
	}
}

func (t *DistanceTool) Strict() bool {
	return true
}

// Call calculates distance between cities.
func (t *DistanceTool) Call(ctx context.Context, input string) (string, error) {
	var params map[string]interface{}
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		return "json unmarshal error, please try again", nil
	}

	origin, ok := params["origin"].(string)
	if !ok {
		return "origin parameter is required", nil
	}
	destination, ok := params["destination"].(string)
	if !ok {
		return "destination parameter is required", nil
	}
	mode, ok := params["mode"].(string)
	if !ok {
		mode = "self-driving" // default mode
	}

	return t.calculateDistance(origin, destination, mode)
}

func (t *DistanceTool) calculateDistance(origin, destination, mode string) (string, error) {
	origin = extractBeforeParenthesis(origin)
	destination = extractBeforeParenthesis(destination)

	if len(t.data) == 0 {
		return "No distance data available", nil
	}

	// Find header indices
	headerMap := make(map[string]int)
	for i, col := range t.data[0] {
		headerMap[col] = i
	}

	// Search for matching distance data
	for i := 1; i < len(t.data); i++ {
		row := t.data[i]
		if len(row) <= headerMap["destination"] {
			continue
		}

		if row[headerMap["origin"]] == origin && row[headerMap["destination"]] == destination {
			duration := row[headerMap["duration"]]
			distance := row[headerMap["distance"]]

			// Check for invalid data
			if duration == "" || distance == "" || duration == "NaN" || distance == "NaN" {
				return "No valid information.", nil
			}

			// Check for day duration (invalid)
			if strings.Contains(duration, "day") {
				return "No valid information.", nil
			}

			// Calculate cost based on mode
			var cost int
			if strings.Contains(mode, "driving") || mode == "self-driving" {
				// Extract numeric value from distance
				distanceStr := strings.ReplaceAll(distance, "km", "")
				distanceStr = strings.ReplaceAll(distanceStr, ",", "")
				if distVal, err := strconv.ParseFloat(strings.TrimSpace(distanceStr), 64); err == nil {
					cost = int(distVal * 0.05)
				}
			} else if mode == "taxi" {
				// Extract numeric value from distance
				distanceStr := strings.ReplaceAll(distance, "km", "")
				distanceStr = strings.ReplaceAll(distanceStr, ",", "")
				if distVal, err := strconv.ParseFloat(strings.TrimSpace(distanceStr), 64); err == nil {
					cost = int(math.Round(distVal))
				}
			}

			return fmt.Sprintf("%s, from %s to %s, duration: %s, distance: %s, cost: %d",
				mode, origin, destination, duration, distance, cost), nil
		}
	}

	return fmt.Sprintf("%s, from %s to %s, no valid information.", mode, origin, destination), nil
}
