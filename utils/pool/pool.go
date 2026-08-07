package pool

import (
	"github.com/panjf2000/ants/v2"
	"sync"
)

type Pool struct {
	pool *ants.Pool
	wg   sync.WaitGroup
}

func NewPoolWithSize(size int) *Pool {
	pool, _ := ants.NewPool(size)
	return &Pool{
		pool: pool,
	}
}

func (p *Pool) RunGo(taskFun func()) {
	p.wg.Add(1)
	_ = p.pool.Submit(func() {
		taskFun()
		defer p.wg.Done()
	})
}

func (p *Pool) Wait() {
	p.wg.Wait()
}

func (p *Pool) Release() {
	p.pool.Release()
}
