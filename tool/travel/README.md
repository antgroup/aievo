# Travel Tools

This package provides seven independent travel planning tools converted from Python to Go, following the same interface pattern as other tools in the aievo project.

## Available Tools

### 1. FlightSearch
Search for flights between departure and destination cities on specific dates.

**Usage:**
```go
tool, err := NewFlightTool(WithDatabasePath("../database"))
result, err := tool.Call(ctx, `{"departure_city": "New York", "destination_city": "London", "date": "2022-10-01"}`)
```

### 2. AccommodationSearch
Find accommodations in cities.

**Usage:**
```go
tool, err := NewAccommodationTool(WithDatabasePath("../database"))
result, err := tool.Call(ctx, `{"city": "Rome"}`)
```

### 3. RestaurantSearch
Discover restaurants in cities with cuisine and rating information.

**Usage:**
```go
tool, err := NewRestaurantTool(WithDatabasePath("../database"))
result, err := tool.Call(ctx, `{"city": "Tokyo"}`)
```

### 4. AttractionSearch
Find tourist attractions, landmarks, and points of interest in cities.

**Usage:**
```go
tool, err := NewAttractionTool(WithDatabasePath("../database"))
result, err := tool.Call(ctx, `{"city": "London"}`)
```

### 5. GoogleDistanceMatrix
Calculate distance, duration and cost between two cities using different transportation modes.

**Usage:**
```go
tool, err := NewDistanceTool(WithDatabasePath("../database"))
result, err := tool.Call(ctx, `{"origin": "Paris", "destination": "Lyon", "mode": "self-driving"}`)
```

### 6. CitySearch
Find cities within a specific state.

**Usage:**
```go
tool, err := NewCityTool(WithDatabasePath("../database"))
result, err := tool.Call(ctx, `{"state": "California"}`)
```

### 7. CostEnquiry
Calculate the cost of a detailed sub plan for one day, including transportation, meals, and accommodation.

**Usage:**
```go
tool, err := NewCostEnquiryTool(WithDatabasePath("../database"))
result, err := tool.Call(ctx, `{
  "people_number": 7,
  "day": 1,
  "current_city": "from Ithaca to Charlotte",
  "transportation": "Flight Number: F3633413, from Ithaca to Charlotte, Departure Time: 05:38, Arrival Time: 07:46",
  "breakfast": "Nagaland's Kitchen, Charlotte",
  "attraction": "The Charlotte Museum of History, Charlotte",
  "lunch": "Cafe Maple Street, Charlotte",
  "dinner": "Bombay Vada Pav, Charlotte",
  "accommodation": "Affordable Spacious Refurbished Room in Bushwick!, Charlotte"
}`)
```

## Configuration Options

Each tool can be configured with various options:

- `WithDatabasePath(path)`: Set the base database directory path
- `WithFlightsPath(path)`: Set the flights CSV file path  
- `WithAccommodationsPath(path)`: Set the accommodations CSV file path
- `WithRestaurantsPath(path)`: Set the restaurants CSV file path
- `WithAttractionsPath(path)`: Set the attractions CSV file path
- `WithDistancePath(path)`: Set the distance matrix CSV file path
- `WithCitiesPath(path)`: Set the cities text file path

## Database Files

The tools expect the following database files:
- `flights/clean_Flights_2022.csv` - Flight information
- `accommodations/clean_accommodations_2022.csv` - Accommodation information  
- `restaurants/clean_restaurant_2022.csv` - Restaurant information
- `attractions/attractions.csv` - Attraction information
- `googleDistanceMatrix/distance.csv` - Distance matrix data
- `background/citySet_with_states.txt` - Cities and states mapping

## Implementation Notes

- All tools implement the `tool.Tool` interface
- Input is expected in JSON format with specific schema for each tool
- Helper functions are provided in `utils.go` for common operations
- Error handling follows Go conventions with descriptive error messages
- CSV parsing is handled efficiently with the standard library
- The tools are thread-safe for concurrent usage

## Testing

Run tests with:
```bash
go test ./tool/travel/... -v
```

Note: Tests require the actual database files to be present. If files are missing, tests will be skipped.
