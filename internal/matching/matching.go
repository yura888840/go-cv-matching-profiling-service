package matching

import (
	"container/heap"
	"context"
	"math/rand"
	"runtime"
	"sort"
	"sync"
)

type CV struct {
	ID     int
	Skills uint64
	Years  int
	Region int
	Salary int
}
type Vacancy struct {
	ID        int
	Required  uint64
	MinYears  int
	Region    int
	MaxSalary int
}
type Match struct {
	CVID  int
	Score int
}

// Score is deterministic, integer-only, and intentionally CPU-light.
// A zero score means that a hard requirement failed.
func Score(v Vacancy, c CV) int {
	if c.Years < v.MinYears || (v.Region != 0 && c.Region != v.Region) || (v.MaxSalary > 0 && c.Salary > v.MaxSalary) {
		return 0
	}
	required := v.Required
	if required == 0 {
		return 0
	}
	overlap := bitsCount(required & c.Skills)
	if overlap == 0 {
		return 0
	}
	total := bitsCount(required)
	return (80*overlap)/total + min(20, (c.Years-v.MinYears)*4)
}
func bitsCount(x uint64) int {
	n := 0
	for x != 0 {
		x &= x - 1
		n++
	}
	return n
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func Generate(n int, seed int64) []CV {
	r := rand.New(rand.NewSource(seed))
	out := make([]CV, n)
	for i := range out {
		out[i] = CV{ID: i + 1, Skills: r.Uint64(), Years: r.Intn(20), Region: 1 + r.Intn(5), Salary: 30000 + r.Intn(90000)}
	}
	return out
}
func GenerateVacancies(n int, seed int64) []Vacancy {
	r := rand.New(rand.NewSource(seed))
	out := make([]Vacancy, n)
	for i := range out {
		out[i] = Vacancy{ID: i + 1, Required: r.Uint64() & r.Uint64(), MinYears: r.Intn(10), Region: 1 + r.Intn(5), MaxSalary: 50000 + r.Intn(80000)}
		if out[i].Required == 0 {
			out[i].Required = 1
		}
	}
	return out
}

type minHeap []Match

func (h minHeap) Len() int { return len(h) }
func (h minHeap) Less(i, j int) bool {
	if h[i].Score != h[j].Score {
		return h[i].Score < h[j].Score
	}
	return h[i].CVID > h[j].CVID
}
func (h minHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x any)   { *h = append(*h, x.(Match)) }
func (h *minHeap) Pop() any     { old := *h; n := len(old); v := old[n-1]; *h = old[:n-1]; return v }
func better(a, b Match) bool    { return a.Score > b.Score || (a.Score == b.Score && a.CVID < b.CVID) }
func addTop(h *minHeap, m Match, k int) {
	if m.Score == 0 || k <= 0 {
		return
	}
	if h.Len() < k {
		heap.Push(h, m)
	} else if better(m, (*h)[0]) {
		(*h)[0] = m
		heap.Fix(h, 0)
	}
}
func finish(h minHeap) []Match {
	out := append([]Match(nil), h...)
	sort.Slice(out, func(i, j int) bool { return better(out[i], out[j]) })
	return out
}
func Sequential(ctx context.Context, v Vacancy, cvs []CV, k int) ([]Match, error) {
	h := make(minHeap, 0, max(0, k))
	for i, c := range cvs {
		if i%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		addTop(&h, Match{c.ID, Score(v, c)}, k)
	}
	return finish(h), nil
}

// WorkerPool splits a single vacancy across workers; each worker keeps local Top K.
func WorkerPool(ctx context.Context, v Vacancy, cvs []CV, k, workers int) ([]Match, error) {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if len(cvs) == 0 || k <= 0 {
		return []Match{}, ctx.Err()
	}
	workers = min(workers, len(cvs))
	partials := make([]minHeap, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			start := len(cvs) * w / workers
			end := len(cvs) * (w + 1) / workers
			h := make(minHeap, 0, k)
			for i := start; i < end; i++ {
				if (i-start)%1024 == 0 && ctx.Err() != nil {
					return
				}
				c := cvs[i]
				addTop(&h, Match{c.ID, Score(v, c)}, k)
			}
			partials[w] = h
		}(w)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h := make(minHeap, 0, k)
	for _, p := range partials {
		for _, m := range p {
			addTop(&h, m, k)
		}
	}
	return finish(h), nil
}

// ByVacancy uses a bounded worker pool across vacancies. Each vacancy is processed sequentially.
func ByVacancy(ctx context.Context, vs []Vacancy, cvs []CV, k, workers int) ([][]Match, error) {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	results := make([][]Match, len(vs))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if ctx.Err() != nil {
					continue
				}
				results[i], _ = Sequential(ctx, vs[i], cvs, k)
			}
		}()
	}
loop:
	for i := range vs {
		select {
		case <-ctx.Done():
			break loop
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
