package travel

// Options represents the configuration options for the Travel tool.
type Options struct {
	// DatabasePath is the path to the database directory
	DatabasePath string
	// FlightsPath is the path to the flights CSV file
	FlightsPath string
	// AccommodationsPath is the path to the accommodations CSV file
	AccommodationsPath string
	// RestaurantsPath is the path to the restaurants CSV file
	RestaurantsPath string
	// AttractionsPath is the path to the attractions CSV file
	AttractionsPath string
	// DistancePath is the path to the distance matrix CSV file
	DistancePath string
	// CitiesPath is the path to the cities text file
	CitiesPath string
}

// Option is a function that configures Options.
type Option func(*Options)

// WithDatabasePath sets the database path.
func WithDatabasePath(path string) Option {
	return func(o *Options) {
		o.DatabasePath = path
	}
}

// WithFlightsPath sets the flights CSV file path.
func WithFlightsPath(path string) Option {
	return func(o *Options) {
		o.FlightsPath = path
	}
}

// WithAccommodationsPath sets the accommodations CSV file path.
func WithAccommodationsPath(path string) Option {
	return func(o *Options) {
		o.AccommodationsPath = path
	}
}

// WithRestaurantsPath sets the restaurants CSV file path.
func WithRestaurantsPath(path string) Option {
	return func(o *Options) {
		o.RestaurantsPath = path
	}
}

// WithAttractionsPath sets the attractions CSV file path.
func WithAttractionsPath(path string) Option {
	return func(o *Options) {
		o.AttractionsPath = path
	}
}

// WithDistancePath sets the distance matrix CSV file path.
func WithDistancePath(path string) Option {
	return func(o *Options) {
		o.DistancePath = path
	}
}

// WithCitiesPath sets the cities text file path.
func WithCitiesPath(path string) Option {
	return func(o *Options) {
		o.CitiesPath = path
	}
}
