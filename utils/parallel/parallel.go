package parallel

import (
	"sync"
)

func Parallel(fn func(int) any, times, concurrency int) []any {
	var wg sync.WaitGroup
	var results = make([]any, times)
	c := make(chan struct{}, concurrency)
	for i := 0; i < times; i++ {
		wg.Add(1)
		c <- struct{}{}
		go func(index int) {
			defer wg.Done()
			results[index] = fn(index)
			<-c
		}(i)
	}

	wg.Wait()
	close(c)
	return results
}
