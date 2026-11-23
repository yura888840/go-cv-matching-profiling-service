package service

import (
	"context"
	"github.com/yura888840/go-cv-matching-profiling-service/internal/matching"
	"testing"
)

type fakeRepo struct{}

func (fakeRepo) GetVacancy(context.Context, int64) (matching.Vacancy, error) {
	return matching.Vacancy{ID: 1, Required: 3, MinYears: 2, Region: 1, MaxSalary: 100000}, nil
}
func (fakeRepo) ListCVs(context.Context, int) ([]matching.CV, error) {
	return []matching.CV{{ID: 1, Skills: 3, Years: 5, Region: 1, Salary: 70000}, {ID: 2, Skills: 1, Years: 1, Region: 1, Salary: 50000}}, nil
}
func TestMatch(t *testing.T) {
	s := NewMatchingService(fakeRepo{}, 100, 4)
	for _, mode := range []string{"sequential", "worker_pool"} {
		r, e := s.Match(context.Background(), Request{VacancyID: 1, Mode: mode})
		if e != nil || len(r.Matches) != 1 || r.Matches[0].CVID != 1 {
			t.Fatalf("%s: %+v %v", mode, r, e)
		}
	}
}

func TestDefaultWorkersFromService(t *testing.T) {
	s := NewMatchingService(fakeRepo{}, 100, 1)
	r, err := s.Match(context.Background(), Request{VacancyID: 1, Mode: "worker_pool"})
	if err != nil || len(r.Matches) != 1 {
		t.Fatalf("unexpected result: %+v, %v", r, err)
	}
}
