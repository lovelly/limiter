package limiter

import (
	"log"
	"runtime/debug"
	"time"
)

const (
	e_start = 1
	e_free  = 2
	e_quit  = 3
)

type JobFunc func()
type eventfunc func(w *Worker, e int) bool

type Worker struct {
	jobQueue   chan JobFunc
	noticeFunc eventfunc
	ttl        time.Time
}

func NewWorker(ef eventfunc) *Worker {
	w := &Worker{
		jobQueue:   make(chan JobFunc),
		noticeFunc: ef,
	}
	if w.noticeFunc != nil {
		w.noticeFunc(w, e_start)
	}
	go w.start()
	return w
}

func (w *Worker) start() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("error: ", err, string(debug.Stack()))
			w.start()
			return
		}
		w.noticeFunc(w, e_quit)
	}()

	for {
		if w.noticeFunc != nil && !w.noticeFunc(w, e_free) {
			return
		}
		select {
		case job := <-w.jobQueue:
			if job == nil {
				w.jobQueue <- nil
				return
			}
			job()
		}
	}
}

func (w *Worker) PushJob(job JobFunc) {
	w.jobQueue <- job
}

func (w *Worker) Close() {
	w.jobQueue <- nil
	<-w.jobQueue
}
