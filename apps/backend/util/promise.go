package util

type PromiseFunction[T any, K any] func(resolve chan<- K, reject chan<- error, arg T)

type PromiseResult[K any] struct {
	Result *K
	Err    error
}

type Promise[T any, K any] struct {
	reserves []PromiseFunction[T, K]
	args     []T
}

func NewPromise[T any, K any]() *Promise[T, K] {
	return &Promise[T, K]{
		reserves: make([]PromiseFunction[T, K], 0),
		args:     make([]T, 0),
	}
}

func (p *Promise[T, K]) Add(f PromiseFunction[T, K], arg T) {
	p.reserves = append(p.reserves, f)
	p.args = append(p.args, arg)
}

func (p *Promise[T, K]) All() []PromiseResult[K] {
	results := make([]PromiseResult[K], len(p.reserves))
	resolveChans := make([]chan K, len(p.reserves))
	rejectChans := make([]chan error, len(p.reserves))

	for i, f := range p.reserves {
		resolveChan := make(chan K)
		rejectChan := make(chan error)
		resolveChans[i] = resolveChan
		rejectChans[i] = rejectChan
		go f(resolveChan, rejectChan, p.args[i])
	}

	for i := 0; i < len(p.reserves); i++ {
		select {
		case result := <-resolveChans[i]:
			results[i] = PromiseResult[K]{Result: &result, Err: nil}
		case err := <-rejectChans[i]:
			results[i] = PromiseResult[K]{Result: nil, Err: err}
		}
	}

	return results
}
