package text

type Options struct {
	ProcessMode string
	Content     string
	Keywords    []string
}

type Option func(*Options)

func WithProcessMode(processMode string) Option {
	return func(o *Options) {
		o.ProcessMode = processMode
	}
}

func WithContent(content string) Option {
	return func(o *Options) {
		o.Content = content
	}
}

func WithKeywords(keywords []string) Option {
	return func(o *Options) {
		o.Keywords = keywords
	}
}
