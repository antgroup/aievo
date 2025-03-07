package counter

import (
	"fmt"
	"sync"
	"time"
)

type Counter struct {
	count     int
	total     int
	mutex     sync.Mutex
	desc      string
	startTime time.Time
}

func NewCounter(opts ...Option) *Counter {
	options := &Options{}

	for _, opt := range opts {
		opt(options)
	}

	return &Counter{
		count:     0,
		total:     options.total,
		desc:      options.desc,
		startTime: time.Now(),
	}
}

func (c *Counter) Add() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.count++
	elapsed := time.Since(c.startTime).Seconds()
	speed := float64(c.count) / elapsed
	remaining := float64(c.total-c.count) / speed
	fmt.Printf("%s: %d/%d, 速度: %.2f/s, 预计剩余时间: %.2f秒\n",
		c.desc, c.count, c.total, speed, remaining)
}
