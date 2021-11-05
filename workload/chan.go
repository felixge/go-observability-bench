package workload

import (
	"sync"
)

type Chan struct {
	Messages int `yaml:"messages"`
}

func (h *Chan) Setup() error {
	if h.Messages == 0 {
		h.Messages = 10000 // should take ~4ms per Run()
	}
	return nil
}

func (h *Chan) Run() error {
	var wg sync.WaitGroup
	ch := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < h.Messages; i++ {
			ch <- struct{}{}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < h.Messages; i++ {
			<-ch
		}
	}()
	wg.Wait()
	return nil
}
