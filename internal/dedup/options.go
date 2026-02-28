package dedup

import "time"

// Options configures the dedup window.
type Options struct {
	Size int
	TTL  time.Duration
}

func (o Options) normalized() Options {
	if o.Size < 1 {
		o.Size = 1
	}

	return o
}
