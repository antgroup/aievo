package travel

import "errors"

// Common errors
var (
	ErrNoData        = errors.New("no data found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrFileNotFound  = errors.New("database file not found")
	ErrInvalidFormat = errors.New("invalid data format")
)

// FlightInfo represents flight information.
type FlightInfo struct {
	FlightNumber      string `json:"flight_number"`
	Price             string `json:"price"`
	DepTime           string `json:"dep_time"`
	ArrTime           string `json:"arr_time"`
	ActualElapsedTime string `json:"actual_elapsed_time"`
	FlightDate        string `json:"flight_date"`
	OriginCityName    string `json:"origin_city_name"`
	DestCityName      string `json:"dest_city_name"`
	Distance          string `json:"distance"`
}

// AccommodationInfo represents accommodation information.
type AccommodationInfo struct {
	Name             string `json:"name"`
	Price            string `json:"price"`
	RoomType         string `json:"room_type"`
	HouseRules       string `json:"house_rules"`
	MinimumNights    string `json:"minimum_nights"`
	MaximumOccupancy string `json:"maximum_occupancy"`
	ReviewRateNumber string `json:"review_rate_number"`
	City             string `json:"city"`
}

// RestaurantInfo represents restaurant information.
type RestaurantInfo struct {
	Name            string `json:"name"`
	AverageCost     string `json:"average_cost"`
	Cuisines        string `json:"cuisines"`
	AggregateRating string `json:"aggregate_rating"`
	City            string `json:"city"`
}

// AttractionInfo represents attraction information.
type AttractionInfo struct {
	Name      string `json:"name"`
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
	Address   string `json:"address"`
	Phone     string `json:"phone"`
	Website   string `json:"website"`
	City      string `json:"city"`
}

// DistanceInfo represents distance calculation information.
type DistanceInfo struct {
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Duration    string `json:"duration"`
	Distance    string `json:"distance"`
	Cost        int    `json:"cost"`
	Mode        string `json:"mode"`
}

// CityInfo represents city information.
type CityInfo struct {
	State  string   `json:"state"`
	Cities []string `json:"cities"`
}
