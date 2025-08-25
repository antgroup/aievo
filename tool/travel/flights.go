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

type FlightTool struct {
	data [][]string
	path string
}

var _ tool.Tool = &FlightTool{}

// NewFlightTool creates a new Flight search tool.
func NewFlightTool(opts ...Option) (*FlightTool, error) {
	options := &Options{
		DatabasePath: "../database",
	}
	for _, opt := range opts {
		opt(options)
	}

	flightPath := options.FlightsPath
	if flightPath == "" {
		flightPath = filepath.Join(options.DatabasePath, "flights/clean_Flights_2022.csv")
	}

	tool := &FlightTool{
		path: flightPath,
	}

	if err := tool.loadData(); err != nil {
		return nil, fmt.Errorf("failed to load flight data: %w", err)
	}

	return tool, nil
}

func (t *FlightTool) loadData() error {
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
func (t *FlightTool) Name() string {
	return "FlightSearch"
}

// Description returns the description of the tool.
func (t *FlightTool) Description() string {
	bytes, _ := json.Marshal(t.Schema())
	return `A flight information retrieval tool.
Search for flights between departure and destination cities on specific dates.
Input must be json schema: ` + string(bytes) + `
Example Input: {"departure_city": "New York", "destination_city": "London", "date": "2022-10-01"}`
}

func (t *FlightTool) Schema() *tool.PropertiesSchema {
	return &tool.PropertiesSchema{
		Type: tool.TypeJson,
		Properties: map[string]tool.PropertySchema{
			"departure_city": {
				Type:        tool.TypeString,
				Description: "The city you'll be flying out from",
			},
			"destination_city": {
				Type:        tool.TypeString,
				Description: "The city you aim to reach",
			},
			"date": {
				Type:        tool.TypeString,
				Description: "The date of your travel in YYYY-MM-DD format",
			},
		},
		Required: []string{"departure_city", "destination_city", "date"},
	}
}

func (t *FlightTool) Strict() bool {
	return true
}

// Call searches for flights.
func (t *FlightTool) Call(ctx context.Context, input string) (string, error) {
	var params map[string]interface{}
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		return "json unmarshal error, please try again", nil
	}

	departure, ok := params["departure_city"].(string)
	if !ok {
		return "departure_city parameter is required", nil
	}
	destination, ok := params["destination_city"].(string)
	if !ok {
		return "destination_city parameter is required", nil
	}
	date, ok := params["date"].(string)
	if !ok {
		return "date parameter is required", nil
	}

	return t.searchFlights(departure, destination, date)
}

func (t *FlightTool) searchFlights(origin, destination, departureDate string) (string, error) {
	origin = extractBeforeParenthesis(origin)
	destination = extractBeforeParenthesis(destination)

	if len(t.data) == 0 {
		return "No flight data available", nil
	}

	// Find header indices
	headerMap := make(map[string]int)
	for i, col := range t.data[0] {
		headerMap[col] = i
	}

	var results [][]string
	header := []string{"Flight Number", "Price", "DepTime", "ArrTime", "ActualElapsedTime", "FlightDate", "OriginCityName", "DestCityName", "Distance"}

	// Search for matching flights
	for i := 1; i < len(t.data); i++ {
		row := t.data[i]
		if len(row) <= headerMap["DestCityName"] {
			continue
		}

		if row[headerMap["OriginCityName"]] == origin &&
			row[headerMap["DestCityName"]] == destination &&
			row[headerMap["FlightDate"]] == departureDate {
			results = append(results, row)
		}
	}

	if len(results) == 0 {
		return fmt.Sprintf("There is no flight from %s to %s on %s.", origin, destination, departureDate), nil
	}

	// Format results
	output := "Found flights:\n"
	output += strings.Join(header, " | ") + "\n"
	for _, result := range results {
		selectedFields := []string{
			result[headerMap["Flight Number"]],
			result[headerMap["Price"]],
			result[headerMap["DepTime"]],
			result[headerMap["ArrTime"]],
			result[headerMap["ActualElapsedTime"]],
			result[headerMap["FlightDate"]],
			result[headerMap["OriginCityName"]],
			result[headerMap["DestCityName"]],
			result[headerMap["Distance"]],
		}
		output += strings.Join(selectedFields, " | ") + "\n"
	}

	return output, nil
}
