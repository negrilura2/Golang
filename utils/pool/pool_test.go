package pool

import (
	"testing"
	"time"
)

func TestPoolWaitNoDeadlock(t *testing.T) {
	p := NewPoolWithSize(2)
	defer p.Release()

	done := make(chan struct{})
	go func() {
		p.RunGo(func() {
			panic("模拟任务崩溃")
		})
		p.RunGo(func() {
			time.Sleep(100 * time.Millisecond)
		})
		p.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("死锁： Wait 2 秒没返回")
	}
}
