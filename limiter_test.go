package limiter

import (
	"context"
	"fmt"
	"golang.org/x/time/rate"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"sync"
	"testing"
	"time"
)

var (
	n = 1000000
)

const (
	_   = 1 << (10 * iota)
	KiB // 1024
	MiB // 1048576
	GiB // 1073741824
	TiB // 1099511627776             (超过了int32的范围)
	PiB // 1125899906842624
	EiB // 1152921504606846976
	ZiB // 1180591620717411303424    (超过了int64的范围)
	YiB // 1208925819614629174706176
)

var (
	inc     int
	incLk   sync.Mutex
	limiter = rate.NewLimiter(100000, 100000)
)

func demoFunc() error {
	n := 10
	incLk.Lock()
	inc++
	inc -= n
	inc += n
	incLk.Unlock()
	r := RandInt(1, 10)
	time.Sleep(time.Duration(r) * time.Second)
	return nil
}

func TestWrokerStart(t *testing.T) {
	go http.ListenAndServe(":8087", nil)
	p := NewHupWorkPool(n / 2)
	start := time.Now().Unix()
	wg := sync.WaitGroup{}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			p.PushJob(func() {
				demoFunc()
			})
			wg.Done()
		}()
	}
	wg.Wait()
	fmt.Println("end wait ", time.Now().Unix()-start)
	p.Wait()
	fmt.Println("end pool ", time.Now().Unix()-start)
	fmt.Println("inc1: ", inc)
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	t.Logf("memory usage:%d MB", mem.TotalAlloc/MiB)
}

func TestNoPool(t *testing.T) {
	c := context.Background()
	limiter := NewRateLimit(n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		limiter.Wait(c)
		wg.Add(1)
		go func() {
			demoFunc()
			wg.Done()
		}()
	}
	wg.Wait()
	fmt.Println("inc2: ", inc)
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	t.Logf("memory usage:%d MB", mem.TotalAlloc/MiB)
}

func T1estLimiter(t *testing.T) {
	c, _ := context.WithCancel(context.TODO())
	for i := 0; i < 100000; i++ {
		limiter.Allow()
	}
	for i := 0; i < 200000; i++ {
		limiter.Wait(c)
	}
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	t.Logf("memory usage:%d MB", mem.TotalAlloc/MiB)
}

func RandInt(n1, n2 int) int {
	if n1 == n2 {
		return n1
	}

	min, max := int64(n1), int64(n2)
	if min > max {
		min, max = max, min
	}
	return int(rand.Int63n(max-min+1) + min)
}
