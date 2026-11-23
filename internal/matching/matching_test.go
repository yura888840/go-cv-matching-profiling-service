package matching

import (
	"context"
	"reflect"
	"testing"
)

func TestEquivalent(t *testing.T) {
	ctx := context.Background()
	cvs := Generate(10003, 42)
	vs := GenerateVacancies(7, 43)
	for _, v := range vs {
		a, e := Sequential(ctx, v, cvs, 100)
		if e != nil {
			t.Fatal(e)
		}
		for _, w := range []int{1, 2, 4, 16} {
			b, e := WorkerPool(ctx, v, cvs, 100, w)
			if e != nil || !reflect.DeepEqual(a, b) {
				t.Fatalf("worker mismatch vacancy=%d workers=%d err=%v", v.ID, w, e)
			}
		}
	}
	all, e := ByVacancy(ctx, vs, cvs, 100, 4)
	if e != nil {
		t.Fatal(e)
	}
	for i, v := range vs {
		a, _ := Sequential(ctx, v, cvs, 100)
		if !reflect.DeepEqual(a, all[i]) {
			t.Fatalf("vacancy mismatch %d", i)
		}
	}
}
func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cvs := Generate(100, 1)
	v := GenerateVacancies(1, 2)[0]
	if _, err := Sequential(ctx, v, cvs, 10); err == nil {
		t.Fatal("expected cancellation")
	}
	if _, err := WorkerPool(ctx, v, cvs, 10, 4); err == nil {
		t.Fatal("expected cancellation")
	}
	if _, err := ByVacancy(ctx, []Vacancy{v}, cvs, 10, 4); err == nil {
		t.Fatal("expected cancellation")
	}
}
func BenchmarkSequential(b *testing.B) {
	cvs := Generate(100000, 42)
	v := GenerateVacancies(1, 43)[0]
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Sequential(ctx, v, cvs, 100)
	}
}
func BenchmarkWorkerPool(b *testing.B) {
	cvs := Generate(100000, 42)
	v := GenerateVacancies(1, 43)[0]
	ctx := context.Background()
	for _, w := range []int{2, 4, 8, 16} {
		b.Run("workers="+itoa(w), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = WorkerPool(ctx, v, cvs, 100, w)
			}
		})
	}
}
func BenchmarkByVacancy(b *testing.B) {
	cvs := Generate(100000, 42)
	vs := GenerateVacancies(8, 43)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ByVacancy(ctx, vs, cvs, 100, 8)
	}
}
func itoa(x int) string {
	if x == 0 {
		return "0"
	}
	s := ""
	for x > 0 {
		s = string(rune('0'+x%10)) + s
		x /= 10
	}
	return s
}
