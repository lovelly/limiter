package limiter

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

type WorkerPool struct {
	workerChan     chan *Worker
	workerCount    int32
	maxWorkerCount int32
	close          int32
	expiry         int
	one  sync.Once
}

//限制同时并发workerNum 个
func NewWorkPool(workerNum int) *WorkerPool {
	p := &WorkerPool{
		maxWorkerCount: int32(workerNum),
		workerChan:     make(chan *Worker, 300),
		expiry:         120,
	}
	return p
}

//通常当pool无工作的是后， pool会保持300个协程序等外工作， 除非调用Close才关闭
// 如果你不能容忍这300个协程, 调用NewHupWorkPool，将把这300个也关闭，不建议使用这个函数
func NewHupWorkPool(workerNum int) *WorkerPool {
	p := NewWorkPool(workerNum)
	p.hub()
	return p
}

// close 保证正回收所有工作协程
func (p *WorkerPool) Close() {
	atomic.StoreInt32(&p.close, 1)
	p.one.Do(func() {
		close(p.workerChan)
	})
	for len(p.workerChan) > 0 {
		w := <-p.workerChan
		w.Close()
	}
}

// Wait 关闭所有协程并且在工作的协程都工作完了再返回
func (p *WorkerPool) Wait() {
	p.Close()
	for p.Running() != 0 {
		time.Sleep(10 * time.Millisecond)
	}
}

// 在调用close后在调用 PushJob, 将会返回失败
func (p *WorkerPool) PushJob(f JobFunc) error {
	if f == nil {
		return errors.New("job Cannot be null")
	}

	if p.maxWorkerCount > atomic.LoadInt32(&p.workerCount) && len(p.workerChan) < 1 {
		NewWorker(p.workEvent)
	}

	if atomic.LoadInt32(&p.close) == 1 { //不严格的判断
		return errors.New("pool already closed")
	}

	w := <-p.workerChan
	if w == nil {
		return errors.New("pool is closed")
	}
	w.jobQueue <- f
	return nil
}

// 在调用close后在调用 PushJob, 将会返回失败
func (p *WorkerPool) PushJobTimeOut(f JobFunc) error {
	if f == nil {
		return errors.New("job Cannot be null")
	}

	if p.maxWorkerCount > atomic.LoadInt32(&p.workerCount) && len(p.workerChan) < 1 {
		NewWorker(p.workEvent)
	}

	if atomic.LoadInt32(&p.close) == 1 { //不严格的判断
		return errors.New("pool already closed")
	}

	t := time.NewTimer(15 * time.Second)
	select {
	case w := <-p.workerChan:
		t.Stop()
		if w == nil {
			return errors.New("pool is closed")
		}
		w.jobQueue <- f
	case <-t.C:
		return errors.New("push job time out")
	}
	return nil
}

func (p *WorkerPool) Running() int32 {
	return atomic.LoadInt32(&p.workerCount)
}

func (p *WorkerPool) workEvent(w *Worker, e int) bool {
	switch e {
	case e_free:
		return p.putWork(w)
	case e_quit:
		atomic.AddInt32(&p.workerCount, -1)
		return true
	case e_start:
		atomic.AddInt32(&p.workerCount, 1)
		return true
	}
	return false
}

func (p *WorkerPool) hub() {
	go func() {
		expiry := time.Duration(p.expiry) * time.Second
		tick := time.NewTicker(expiry)
		defer tick.Stop()
		for range tick.C {
			if p.close == 1 {
				tick.Stop()
				return
			}
			now := time.Now()
			for len(p.workerChan) > 0 {
				w := <-p.workerChan
				if now.Sub(w.ttl) < expiry {
					p.putWork(w)
					break
				}
				w.Close()
			}
		}
	}()
}

func (p *WorkerPool) putWork(w *Worker) (ret bool) {
	if atomic.LoadInt32(&p.close) == 1 {
		return false
	}
	defer func() {
		if err := recover(); err != nil {
			ret = false
		}
	}()
	w.ttl = time.Now()
	select {
	case p.workerChan <- w:
		ret = true
		return
	default:
		return false
	}
}
