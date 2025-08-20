package travel

import (
	"context"
	"strings"
	"testing"

	"github.com/antgroup/aievo/tool"
)

func TestFlightTool(t *testing.T) {
	tool, err := NewFlightTool(WithDatabasePath("../database"))
	if err != nil {
		t.Skipf("Skipping test due to database files not available: %v", err)
		return
	}

	ctx := context.Background()
	input := `{"departure_city": "New York", "destination_city": "London", "date": "2022-10-01"}`

	result, err := tool.Call(ctx, input)
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	t.Logf("FlightTool Result: %s", result)
}

func TestAccommodationTool(t *testing.T) {
	tool, err := NewAccommodationTool(WithDatabasePath("../database"))
	if err != nil {
		t.Skipf("Skipping test due to database files not available: %v", err)
		return
	}

	ctx := context.Background()
	input := `{"city": "Rome"}`

	result, err := tool.Call(ctx, input)
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	t.Logf("AccommodationTool Result: %s", result)
}

func TestRestaurantTool(t *testing.T) {
	tool, err := NewRestaurantTool(WithDatabasePath("../database"))
	if err != nil {
		t.Skipf("Skipping test due to database files not available: %v", err)
		return
	}

	ctx := context.Background()
	input := `{"city": "Tokyo"}`

	result, err := tool.Call(ctx, input)
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	t.Logf("RestaurantTool Result: %s", result)
}

func TestAttractionTool(t *testing.T) {
	tool, err := NewAttractionTool(WithDatabasePath("../database"))
	if err != nil {
		t.Skipf("Skipping test due to database files not available: %v", err)
		return
	}

	ctx := context.Background()
	input := `{"city": "London"}`

	result, err := tool.Call(ctx, input)
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	t.Logf("AttractionTool Result: %s", result)
}

func TestDistanceTool(t *testing.T) {
	tool, err := NewDistanceTool(WithDatabasePath("../database"))
	if err != nil {
		t.Skipf("Skipping test due to database files not available: %v", err)
		return
	}

	ctx := context.Background()
	input := `{"origin": "Paris", "destination": "Lyon", "mode": "self-driving"}`

	result, err := tool.Call(ctx, input)
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	t.Logf("DistanceTool Result: %s", result)
}

func TestCityTool(t *testing.T) {
	tool, err := NewCityTool(WithDatabasePath("../database"))
	if err != nil {
		t.Skipf("Skipping test due to database files not available: %v", err)
		return
	}

	ctx := context.Background()
	input := `{"state": "California"}`

	result, err := tool.Call(ctx, input)
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	t.Logf("CityTool Result: %s", result)
}

func TestCostEnquiryTool(t *testing.T) {
	tool, err := NewCostEnquiryTool(WithDatabasePath("../database"))
	if err != nil {
		t.Skipf("Skipping test due to database files not available: %v", err)
		return
	}

	ctx := context.Background()

	// Test with a complete plan
	input := `{
		"people_number": 2,
		"day": 1,
		"current_city": "from New York to Los Angeles",
		"transportation": "Flight Number: F3633413, from New York to Los Angeles, Departure Time: 08:00, Arrival Time: 11:30",
		"breakfast": "Joe's Coffee, New York",
		"attraction": "Central Park, New York",
		"lunch": "Tony's Italian, Los Angeles",
		"dinner": "Sunset Grill, Los Angeles",
		"accommodation": "Downtown Hotel, Los Angeles"
	}`

	result, err := tool.Call(ctx, input)
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	t.Logf("CostEnquiryTool Result: %s", result)

	// Test with minimal plan (only people_number)
	minimalInput := `{"people_number": 1}`
	result, err = tool.Call(ctx, minimalInput)
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	t.Logf("CostEnquiryTool Minimal Result: %s", result)

	// Test with invalid people_number
	invalidInput := `{"people_number": 0}`
	result, err = tool.Call(ctx, invalidInput)
	if err != nil {
		t.Errorf("Call() error = %v", err)
	}
	if !strings.Contains(result, "must be greater than 0") {
		t.Errorf("Expected error message for invalid people_number, got: %s", result)
	}
}

func TestToolInterfaces(t *testing.T) {
	tests := []struct {
		name string
		tool interface {
			Name() string
			Description() string
			Schema() *tool.PropertiesSchema
			Strict() bool
		}
	}{
		{"FlightTool", mustCreateFlightTool()},
		{"AccommodationTool", mustCreateAccommodationTool()},
		{"RestaurantTool", mustCreateRestaurantTool()},
		{"AttractionTool", mustCreateAttractionTool()},
		{"DistanceTool", mustCreateDistanceTool()},
		{"CityTool", mustCreateCityTool()},
		{"CostEnquiryTool", mustCreateCostEnquiryTool()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tool == nil {
				t.Skip("Tool cannot be created (likely missing database files)")
				return
			}

			// Test Name
			name := tt.tool.Name()
			if name == "" {
				t.Error("Name() should not return empty string")
			}

			// Test Description
			desc := tt.tool.Description()
			if desc == "" {
				t.Error("Description() should not return empty string")
			}

			// Test Schema
			schema := tt.tool.Schema()
			if schema == nil {
				t.Error("Schema() should not return nil")
			}

			// Test Strict
			if !tt.tool.Strict() {
				t.Error("Strict() should return true")
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ExtractBeforeParenthesis",
			input:    "New York (NY)",
			expected: "New York",
		},
		{
			name:     "ExtractBeforeParenthesis_NoParenthesis",
			input:    "London",
			expected: "London",
		},
		{
			name:     "ExtractBeforeParenthesis_Empty",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBeforeParenthesis(tt.input)
			if result != tt.expected {
				t.Errorf("extractBeforeParenthesis() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetValidNameCity(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedName string
		expectedCity string
	}{
		{
			name:         "ValidFormat",
			input:        "Hotel ABC, New York",
			expectedName: "Hotel ABC",
			expectedCity: "New York",
		},
		{
			name:         "ValidFormatWithState",
			input:        "Restaurant XYZ, Los Angeles (CA)",
			expectedName: "Restaurant XYZ",
			expectedCity: "Los Angeles",
		},
		{
			name:         "InvalidFormat",
			input:        "InvalidInput",
			expectedName: "-",
			expectedCity: "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, city := getValidNameCity(tt.input)
			if name != tt.expectedName || city != tt.expectedCity {
				t.Errorf("getValidNameCity() = (%v, %v), want (%v, %v)",
					name, city, tt.expectedName, tt.expectedCity)
			}
		})
	}
}

func TestExtractFromTo(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedFrom string
		expectedTo   string
	}{
		{
			name:         "ValidFormat",
			input:        "from New York to Los Angeles",
			expectedFrom: "New York",
			expectedTo:   "Los Angeles",
		},
		{
			name:         "ValidFormatWithComma",
			input:        "from Boston to Chicago, departure at 8am",
			expectedFrom: "Boston",
			expectedTo:   "Chicago",
		},
		{
			name:         "NoMatch",
			input:        "just some random text",
			expectedFrom: "",
			expectedTo:   "",
		},
		{
			name:         "EmptyString",
			input:        "",
			expectedFrom: "",
			expectedTo:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to := extractFromTo(tt.input)
			if from != tt.expectedFrom || to != tt.expectedTo {
				t.Errorf("extractFromTo() = (%v, %v), want (%v, %v)",
					from, to, tt.expectedFrom, tt.expectedTo)
			}
		})
	}
}

// Helper functions for testing - these skip if the tools can't be created
func mustCreateFlightTool() *FlightTool {
	tool, err := NewFlightTool()
	if err != nil {
		return nil // Return nil if can't create, test will skip
	}
	return tool
}

func mustCreateAccommodationTool() *AccommodationTool {
	tool, err := NewAccommodationTool()
	if err != nil {
		return nil
	}
	return tool
}

func mustCreateRestaurantTool() *RestaurantTool {
	tool, err := NewRestaurantTool()
	if err != nil {
		return nil
	}
	return tool
}

func mustCreateAttractionTool() *AttractionTool {
	tool, err := NewAttractionTool()
	if err != nil {
		return nil
	}
	return tool
}

func mustCreateDistanceTool() *DistanceTool {
	tool, err := NewDistanceTool()
	if err != nil {
		return nil
	}
	return tool
}

func mustCreateCityTool() *CityTool {
	tool, err := NewCityTool()
	if err != nil {
		return nil
	}
	return tool
}

func mustCreateCostEnquiryTool() *CostEnquiryTool {
	tool, err := NewCostEnquiryTool()
	if err != nil {
		return nil
	}
	return tool
}
