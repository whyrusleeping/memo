package memo

import (
	"context"
	"sync"
)

func NewMemoizer(work func(context.Context, string) (interface{}, error)) *Memoizer {
	return &Memoizer{
		memo: make(map[string]*memoWaiter),
		work: work,
	}
}

func (m *Memoizer) SetConcurrencyLimit(n int) {
	m.limiter = make(chan struct{}, n)
}

type Memoizer struct {
	lk      sync.Mutex
	memo    map[string]*memoWaiter
	work    func(context.Context, string) (interface{}, error)
	limiter chan struct{}
}

type memoWaiter struct {
	wait   chan struct{}
	result interface{}
	err    error
}

func (m *Memoizer) Do(ctx context.Context, key string) (interface{}, error) {

	m.lk.Lock()
	w, ok := m.memo[key]
	if ok {
		m.lk.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-w.wait:
		}
		return w.result, w.err
	}

	if m.limiter != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case m.limiter <- struct{}{}:
		}

		defer func() {
			<-m.limiter
		}()
	}

	w = &memoWaiter{
		wait: make(chan struct{}),
	}
	m.memo[key] = w
	m.lk.Unlock()

	res, err := m.work(ctx, key)
	w.result = res
	w.err = err
	close(w.wait)

	return res, err
}
