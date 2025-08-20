package travel

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/antgroup/aievo/tool"
	"github.com/antgroup/aievo/utils/json"
)

type CostEnquiryTool struct {
	flightsData        [][]string
	accommodationsData [][]string
	restaurantsData    [][]string
	distanceData       [][]string
	options            *Options
}

var _ tool.Tool = &CostEnquiryTool{}

// PlanData represents the structure of a day plan for cost calculation
type PlanData struct {
	PeopleNumber   int    `json:"people_number"`
	Day            int    `json:"day"`
	CurrentCity    string `json:"current_city"`
	Transportation string `json:"transportation"`
	Breakfast      string `json:"breakfast"`
	Attraction     string `json:"attraction"`
	Lunch          string `json:"lunch"`
	Dinner         string `json:"dinner"`
	Accommodation  string `json:"accommodation"`
}

// NewCostEnquiryTool creates a new Cost Enquiry tool.
func NewCostEnquiryTool(opts ...Option) (*CostEnquiryTool, error) {
	options := &Options{
		DatabasePath: "../database",
	}
	for _, opt := range opts {
		opt(options)
	}

	// Set default paths if not provided
	if options.FlightsPath == "" {
		options.FlightsPath = filepath.Join(options.DatabasePath, "flights/clean_Flights_2022.csv")
	}
	if options.AccommodationsPath == "" {
		options.AccommodationsPath = filepath.Join(options.DatabasePath, "accommodations/clean_accommodations_2022.csv")
	}
	if options.RestaurantsPath == "" {
		options.RestaurantsPath = filepath.Join(options.DatabasePath, "restaurants/clean_restaurant_2022.csv")
	}
	if options.DistancePath == "" {
		options.DistancePath = filepath.Join(options.DatabasePath, "googleDistanceMatrix/distance.csv")
	}

	tool := &CostEnquiryTool{
		options: options,
	}

	if err := tool.loadData(); err != nil {
		return nil, fmt.Errorf("failed to load cost enquiry data: %w", err)
	}

	return tool, nil
}

func (t *CostEnquiryTool) loadData() error {
	var err error

	// Load flights data
	t.flightsData, err = t.loadCSV(t.options.FlightsPath)
	if err != nil {
		return fmt.Errorf("failed to load flights data: %w", err)
	}

	// Load accommodations data
	t.accommodationsData, err = t.loadCSV(t.options.AccommodationsPath)
	if err != nil {
		return fmt.Errorf("failed to load accommodations data: %w", err)
	}

	// Load restaurants data
	t.restaurantsData, err = t.loadCSV(t.options.RestaurantsPath)
	if err != nil {
		return fmt.Errorf("failed to load restaurants data: %w", err)
	}

	// Load distance data
	t.distanceData, err = t.loadCSV(t.options.DistancePath)
	if err != nil {
		return fmt.Errorf("failed to load distance data: %w", err)
	}

	return nil
}

func (t *CostEnquiryTool) loadCSV(filename string) ([][]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	return records, nil
}

// Name returns the name of the tool.
func (t *CostEnquiryTool) Name() string {
	return "CostEnquiry"
}

// Description returns the description of the tool.
func (t *CostEnquiryTool) Description() string {
	bytes, _ := json.Marshal(t.Schema())
	return `Calculate the cost of a detailed sub plan for one day.
This function calculates the cost of a complete one-day travel plan including transportation, meals, and accommodation.
Input must be json schema: ` + string(bytes) + `
Example Input: {"people_number": 7, "day": 1, "current_city": "from Ithaca to Charlotte", "transportation": "Flight Number: F3633413, from Ithaca to Charlotte, Departure Time: 05:38, Arrival Time: 07:46", "breakfast": "Nagaland's Kitchen, Charlotte", "attraction": "The Charlotte Museum of History, Charlotte", "lunch": "Cafe Maple Street, Charlotte", "dinner": "Bombay Vada Pav, Charlotte", "accommodation": "Affordable Spacious Refurbished Room in Bushwick!, Charlotte"}`
}

func (t *CostEnquiryTool) Schema() *tool.PropertiesSchema {
	return &tool.PropertiesSchema{
		Type: tool.TypeJson,
		Properties: map[string]tool.PropertySchema{
			"people_number": {
				Type:        tool.TypeInt,
				Description: "The number of people in the travel plan",
			},
			"day": {
				Type:        tool.TypeInt,
				Description: "The day number of the plan",
			},
			"current_city": {
				Type:        tool.TypeString,
				Description: "Current city or travel route (e.g., 'from City A to City B')",
			},
			"transportation": {
				Type:        tool.TypeString,
				Description: "Transportation details (flight, self-driving, taxi)",
			},
			"breakfast": {
				Type:        tool.TypeString,
				Description: "Breakfast restaurant information (format: 'Restaurant Name, City')",
			},
			"attraction": {
				Type:        tool.TypeString,
				Description: "Attraction information (format: 'Attraction Name, City')",
			},
			"lunch": {
				Type:        tool.TypeString,
				Description: "Lunch restaurant information (format: 'Restaurant Name, City')",
			},
			"dinner": {
				Type:        tool.TypeString,
				Description: "Dinner restaurant information (format: 'Restaurant Name, City')",
			},
			"accommodation": {
				Type:        tool.TypeString,
				Description: "Accommodation information (format: 'Hotel Name, City')",
			},
		},
		Required: []string{"people_number"},
	}
}

func (t *CostEnquiryTool) Strict() bool {
	return true
}

// Call calculates the cost of the travel plan.
func (t *CostEnquiryTool) Call(ctx context.Context, input string) (string, error) {
	var planData PlanData
	err := json.Unmarshal([]byte(input), &planData)
	if err != nil {
		return "json unmarshal error, please try again", nil
	}

	if planData.PeopleNumber <= 0 {
		return "people_number must be greater than 0", nil
	}

	return t.calculateCost(planData)
}

func (t *CostEnquiryTool) calculateCost(plan PlanData) (string, error) {
	totalCost := 0.0
	var errors []string

	// Calculate transportation cost
	if plan.Transportation != "" && plan.Transportation != "-" {
		cost, err := t.calculateTransportationCost(plan.Transportation, plan.CurrentCity, plan.PeopleNumber)
		if err != nil {
			errors = append(errors, err.Error())
		} else {
			totalCost += cost
		}
	}

	// Calculate breakfast cost
	if plan.Breakfast != "" && plan.Breakfast != "-" {
		cost, err := t.calculateRestaurantCost(plan.Breakfast, plan.PeopleNumber)
		if err != nil {
			errors = append(errors, "The breakfast information is not valid, please check.")
		} else {
			totalCost += cost
		}
	}

	// Calculate lunch cost
	if plan.Lunch != "" && plan.Lunch != "-" {
		cost, err := t.calculateRestaurantCost(plan.Lunch, plan.PeopleNumber)
		if err != nil {
			errors = append(errors, "The lunch information is not valid, please check.")
		} else {
			totalCost += cost
		}
	}

	// Calculate dinner cost
	if plan.Dinner != "" && plan.Dinner != "-" {
		cost, err := t.calculateRestaurantCost(plan.Dinner, plan.PeopleNumber)
		if err != nil {
			errors = append(errors, "The dinner information is not valid, please check.")
		} else {
			totalCost += cost
		}
	}

	// Calculate accommodation cost
	if plan.Accommodation != "" && plan.Accommodation != "-" {
		cost, err := t.calculateAccommodationCost(plan.Accommodation, plan.PeopleNumber)
		if err != nil {
			errors = append(errors, "The accommodation information is not valid, please check.")
		} else {
			totalCost += cost
		}
	}

	if len(errors) == 0 {
		return fmt.Sprintf("The cost of your plan is %.0f dollars.", totalCost), nil
	} else {
		message := "Sorry, the cost of your plan is not available because of the following reasons:"
		for i, errMsg := range errors {
			message += fmt.Sprintf(" %d. %s", i+1, errMsg)
		}
		return message, nil
	}
}

func (t *CostEnquiryTool) calculateTransportationCost(transportation, currentCity string, peopleNumber int) (float64, error) {
	orgCity, destCity := extractFromTo(transportation)
	if orgCity == "" || destCity == "" {
		orgCity, destCity = extractFromTo(currentCity)
	}

	if orgCity == "" || destCity == "" {
		return 0, fmt.Errorf("the transportation information is not valid, please check")
	}

	lowerTransport := strings.ToLower(transportation)

	if strings.Contains(lowerTransport, "flight number") {
		return t.calculateFlightCost(transportation, peopleNumber)
	} else if strings.Contains(lowerTransport, "self-driving") {
		return t.calculateSelfDrivingCost(orgCity, destCity, peopleNumber)
	} else if strings.Contains(lowerTransport, "taxi") {
		return t.calculateTaxiCost(orgCity, destCity, peopleNumber)
	}

	return 0, fmt.Errorf("unsupported transportation type")
}

func (t *CostEnquiryTool) calculateFlightCost(transportation string, peopleNumber int) (float64, error) {
	// Extract flight number from transportation string
	// Format: "Flight Number: F3633413, ..."
	parts := strings.Split(transportation, "Flight Number: ")
	if len(parts) < 2 {
		return 0, fmt.Errorf("flight number not found")
	}

	flightNumber := strings.Split(parts[1], ",")[0]
	flightNumber = strings.TrimSpace(flightNumber)

	// Find flight in data
	headerMap := make(map[string]int)
	if len(t.flightsData) > 0 {
		for i, col := range t.flightsData[0] {
			headerMap[col] = i
		}
	}

	for i := 1; i < len(t.flightsData); i++ {
		row := t.flightsData[i]
		if len(row) > headerMap["Flight Number"] && row[headerMap["Flight Number"]] == flightNumber {
			priceStr := row[headerMap["Price"]]
			price, err := strconv.ParseFloat(priceStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid price format")
			}
			return price * float64(peopleNumber), nil
		}
	}

	return 0, fmt.Errorf("flight information is not valid")
}

func (t *CostEnquiryTool) calculateSelfDrivingCost(orgCity, destCity string, peopleNumber int) (float64, error) {
	cost, err := t.getDistanceCost(orgCity, destCity, "driving")
	if err != nil {
		return 0, err
	}
	// Self-driving: max 5 people per car
	return cost * math.Ceil(float64(peopleNumber)/5.0), nil
}

func (t *CostEnquiryTool) calculateTaxiCost(orgCity, destCity string, peopleNumber int) (float64, error) {
	cost, err := t.getDistanceCost(orgCity, destCity, "taxi")
	if err != nil {
		return 0, err
	}
	// Taxi: max 4 people per taxi
	return cost * math.Ceil(float64(peopleNumber)/4.0), nil
}

func (t *CostEnquiryTool) getDistanceCost(orgCity, destCity, mode string) (float64, error) {
	orgCity = extractBeforeParenthesis(orgCity)
	destCity = extractBeforeParenthesis(destCity)

	headerMap := make(map[string]int)
	if len(t.distanceData) > 0 {
		for i, col := range t.distanceData[0] {
			headerMap[col] = i
		}
	}

	for i := 1; i < len(t.distanceData); i++ {
		row := t.distanceData[i]
		if len(row) <= headerMap["destination"] {
			continue
		}

		if row[headerMap["origin"]] == orgCity && row[headerMap["destination"]] == destCity {
			distance := row[headerMap["distance"]]

			// Extract numeric value from distance
			distanceStr := strings.ReplaceAll(distance, "km", "")
			distanceStr = strings.ReplaceAll(distanceStr, ",", "")
			distVal, err := strconv.ParseFloat(strings.TrimSpace(distanceStr), 64)
			if err != nil {
				return 0, fmt.Errorf("invalid distance format")
			}

			switch mode {
			case "driving":
				return distVal * 0.05, nil
			case "taxi":
				return math.Round(distVal), nil
			default:
				return 0, fmt.Errorf("unsupported mode: %s", mode)
			}
		}
	}

	return 0, fmt.Errorf("distance information not found")
}

func (t *CostEnquiryTool) calculateRestaurantCost(restaurant string, peopleNumber int) (float64, error) {
	name, city := getValidNameCity(restaurant)
	if name == "-" || city == "-" {
		return 0, fmt.Errorf("invalid restaurant format")
	}

	headerMap := make(map[string]int)
	if len(t.restaurantsData) > 0 {
		for i, col := range t.restaurantsData[0] {
			headerMap[col] = i
		}
	}

	for i := 1; i < len(t.restaurantsData); i++ {
		row := t.restaurantsData[i]
		if len(row) <= headerMap["City"] {
			continue
		}

		if row[headerMap["Name"]] == name && row[headerMap["City"]] == city {
			costStr := row[headerMap["Average Cost"]]
			cost, err := strconv.ParseFloat(costStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid cost format")
			}
			return cost * float64(peopleNumber), nil
		}
	}

	return 0, fmt.Errorf("restaurant not found")
}

func (t *CostEnquiryTool) calculateAccommodationCost(accommodation string, peopleNumber int) (float64, error) {
	name, city := getValidNameCity(accommodation)
	if name == "-" || city == "-" {
		return 0, fmt.Errorf("invalid accommodation format")
	}

	headerMap := make(map[string]int)
	if len(t.accommodationsData) > 0 {
		for i, col := range t.accommodationsData[0] {
			headerMap[col] = i
		}
	}

	for i := 1; i < len(t.accommodationsData); i++ {
		row := t.accommodationsData[i]
		if len(row) <= headerMap["city"] {
			continue
		}

		if row[headerMap["NAME"]] == name && row[headerMap["city"]] == city {
			priceStr := row[headerMap["price"]]
			price, err := strconv.ParseFloat(priceStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid price format")
			}

			maxOccupancyStr := row[headerMap["maximum occupancy"]]
			maxOccupancy, err := strconv.ParseFloat(maxOccupancyStr, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid occupancy format")
			}

			rooms := math.Ceil(float64(peopleNumber) / maxOccupancy)
			return price * rooms, nil
		}
	}

	return 0, fmt.Errorf("accommodation not found")
}

// extractFromTo extracts 'A' and 'B' from the format "from A to B"
func extractFromTo(text string) (string, string) {
	pattern := `from\s+(.+?)\s+to\s+([^,]+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(text)
	if len(matches) >= 3 {
		from := strings.TrimSpace(matches[1])
		to := strings.TrimSpace(matches[2])
		// Remove any trailing comma or whitespace from 'to'
		if idx := strings.IndexAny(to, ","); idx != -1 {
			to = strings.TrimSpace(to[:idx])
		}
		return from, to
	}
	return "", ""
}
