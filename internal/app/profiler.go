package app

import (
	"context"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/hertz-contrib/pprof"
)

// StartPprofServer starts an optional pprof HTTP server on the address in
// PPROF_ADDR using Hertz's pprof middleware. It returns a no-op stop func when
// profiling is disabled, so binaries can unconditionally
// `defer app.StartPprofServer()()`. Set PPROF_MEMPROFILERATE=1 to record every
// allocation (exact counts at the cost of throughput); the default samples at
// 512KiB intervals and under-counts small allocations.
func StartPprofServer() func() {
	addr := os.Getenv("PPROF_ADDR")
	if addr == "" {
		return func() {}
	}

	if os.Getenv("PPROF_MEMPROFILERATE") == "1" {
		runtime.MemProfileRate = 1
	}

	hz := server.New(server.WithHostPorts(addr))
	pprof.Register(hz)

	go func() {
		log.Printf("pprof server listening on %s", addr)
		if err := hz.Run(); err != nil {
			log.Printf("pprof server error: %v", err)
		}
	}()

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hz.Shutdown(ctx)
	}
}
