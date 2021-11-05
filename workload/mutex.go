package workload

import (
	"fmt"
	"sync"
)

type Mutex struct {
	Ops int `yaml:"ops"`
}

func (h *Mutex) Setup() error {
	if h.Ops == 0 {
		h.Ops = 100000 // takes about ~1.5ms per Run()
	}
	return nil
}

func (h *Mutex) Run() error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(2)
	var count int
	go func() {
		defer wg.Done()
		for i := 0; i < h.Ops/2; i++ {
			mu.Lock()
			count++
			mu.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < h.Ops/2; i++ {
			mu.Lock()
			count++
			mu.Unlock()
		}
	}()
	wg.Wait()
	if count != h.Ops {
		return fmt.Errorf("bad count=%d want=%d", count, h.Ops)
	}
	return nil
}
