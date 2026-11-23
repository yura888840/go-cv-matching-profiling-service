package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/yura888840/go-cv-matching-profiling-service/internal/matching"
	"os"
	"runtime"
	"runtime/debug"
	"time"
)

func main() {
	n := flag.Int("cvs", 100000, "number of CVs")
	nv := flag.Int("vacancies", 8, "number of vacancies")
	workers := flag.Int("workers", runtime.GOMAXPROCS(0), "worker count")
	top := flag.Int("top", 100, "top candidates per vacancy")
	flag.Parse()
	if *n < 0 || *nv < 1 || *workers < 1 || *top < 0 {
		fmt.Fprintln(os.Stderr, "invalid flags")
		os.Exit(2)
	}
	cvs := matching.Generate(*n, 42)
	vs := matching.GenerateVacancies(*nv, 43)
	ctx := context.Background()
	fmt.Printf("CVs=%d vacancies=%d workers=%d top=%d GOMAXPROCS=%d\n", *n, *nv, *workers, *top, runtime.GOMAXPROCS(0))
	run := func(name string, fn func() error, ops int) {
		runtime.GC()
		debug.FreeOSMemory()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		err := fn()
		elapsed := time.Since(start)
		runtime.ReadMemStats(&after)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("%-24s duration=%-12s scores/sec=%-12.0f allocated=%d KB\n", name, elapsed, float64(ops)/elapsed.Seconds(), (after.TotalAlloc-before.TotalAlloc)/1024)
	}
	run("sequential-one", func() error { _, err := matching.Sequential(ctx, vs[0], cvs, *top); return err }, *n)
	run("worker-pool-one", func() error { _, err := matching.WorkerPool(ctx, vs[0], cvs, *top, *workers); return err }, *n)
	run("sequential-all", func() error {
		for _, v := range vs {
			if _, err := matching.Sequential(ctx, v, cvs, *top); err != nil {
				return err
			}
		}
		return nil
	}, *n**nv)
	run("by-vacancy-all", func() error { _, err := matching.ByVacancy(ctx, vs, cvs, *top, *workers); return err }, *n**nv)
}
