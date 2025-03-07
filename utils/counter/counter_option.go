package counter

type Options struct {
	total int
	desc  string
}

type Option func(*Options)

func WithTotal(total int) Option {
	return func(o *Options) {
		o.total = total
	}
}

func WithDesc(desc string) Option {
	return func(o *Options) {
		o.desc = desc
	}
}
