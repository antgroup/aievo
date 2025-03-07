package retriever

type Options struct {
	ProviderUrl string
}

type Option func(*Options)

func WithProviderUrl(url string) Option {
	return func(o *Options) {
		o.ProviderUrl = url
	}
}
