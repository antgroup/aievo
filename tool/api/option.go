package api

// An Option is an option for Bidi processing.
type Option func(*Tool)

func WithSuffix(suffix string) Option {
	return func(o *Tool) {
		o.suffix = suffix
	}
}
