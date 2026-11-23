package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/yura888840/go-cv-matching-profiling-service/internal/matching"
)

type metrics struct {
	Mode            string  `json:"mode"`
	CVs             int     `json:"cvs"`
	Workers         int     `json:"workers"`
	Iterations      int     `json:"iterations"`
	DurationMS      float64 `json:"duration_ms"`
	CVPerSecond     float64 `json:"cv_per_second"`
	TotalAllocBytes uint64  `json:"total_alloc_bytes"`
	Allocations     uint64  `json:"allocations"`
	HeapBeforeBytes uint64  `json:"heap_before_bytes"`
	HeapAfterBytes  uint64  `json:"heap_after_bytes"`
	PeakHeapBytes   uint64  `json:"peak_heap_bytes"`
	CPUProfile      string  `json:"cpu_profile,omitempty"`
	HeapProfile     string  `json:"heap_profile,omitempty"`
}

func main() {
	n := flag.Int("cvs", 100000, "number of synthetic CVs")
	mode := flag.String("mode", "worker_pool", "sequential or worker_pool")
	workers := flag.Int("workers", 4, "number of workers")
	iterations := flag.Int("iterations", 10, "matching iterations")
	cpu := flag.String("cpuprofile", "", "CPU profile output path")
	heap := flag.String("memprofile", "", "heap profile output path")
	flag.Parse()
	if *n < 1 || *iterations < 1 || *workers < 1 || (*mode != "sequential" && *mode != "worker_pool") {
		fmt.Fprintln(os.Stderr, "invalid flags")
		os.Exit(2)
	}
	cvs := matching.Generate(*n, 42)
	vacancy := matching.GenerateVacancies(1, 43)[0]
	// Exclude test data generation from CPU and allocation measurements.
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if *cpu != "" {
		f, err := os.Create(*cpu)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			panic(err)
		}
	}
	start := time.Now()
	var result []matching.Match
	for i := 0; i < *iterations; i++ {
		var err error
		if *mode == "sequential" {
			result, err = matching.Sequential(context.Background(), vacancy, cvs, 100)
		} else {
			result, err = matching.WorkerPool(context.Background(), vacancy, cvs, 100, *workers)
		}
		if err != nil {
			panic(err)
		}
	}
	duration := time.Since(start)
	if *cpu != "" {
		pprof.StopCPUProfile()
	}
	runtime.ReadMemStats(&after)
	// HeapSys/HeapAlloc after the run are not true peak values. For an exact peak,
	// sample externally (e.g. /proc or container metrics); report sampled end heap only.
	if *heap != "" {
		f, err := os.Create(*heap)
		if err != nil {
			panic(err)
		}
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			panic(err)
		}
		f.Close()
	}
	_ = result
	m := metrics{Mode: *mode, CVs: *n, Workers: *workers, Iterations: *iterations, DurationMS: float64(duration.Microseconds()) / 1000, CVPerSecond: float64(*n**iterations) / duration.Seconds(), TotalAllocBytes: after.TotalAlloc - before.TotalAlloc, Allocations: after.Mallocs - before.Mallocs, HeapBeforeBytes: before.HeapAlloc, HeapAfterBytes: after.HeapAlloc, PeakHeapBytes: 0, CPUProfile: *cpu, HeapProfile: *heap}
	if err := json.NewEncoder(os.Stdout).Encode(m); err != nil {
		panic(err)
	}
}
