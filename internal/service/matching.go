package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/yura888840/go-cv-matching-profiling-service/internal/matching"
)

var ErrNotFound = errors.New("vacancy not found")

type Repository interface {
	GetVacancy(ctx context.Context, id int64) (matching.Vacancy, error)
	ListCVs(ctx context.Context, limit int) ([]matching.CV, error)
}

type Request struct {
	VacancyID int64  `json:"vacancy_id" binding:"required,gt=0"`
	Top       int    `json:"top"`
	Workers   int    `json:"workers"`
	Mode      string `json:"mode"`
	CVLimit   int    `json:"cv_limit"`
}
type Result struct {
	VacancyID int64            `json:"vacancy_id"`
	Evaluated int              `json:"evaluated"`
	Matches   []matching.Match `json:"matches"`
}
type MatchingService struct {
	repo    Repository
	maxCVs  int
	workers int
}

func NewMatchingService(repo Repository, maxCVs int, workers int) *MatchingService {
	return &MatchingService{repo: repo, maxCVs: maxCVs, workers: workers}
}
func (s *MatchingService) Match(ctx context.Context, req Request) (Result, error) {
	if req.VacancyID <= 0 {
		return Result{}, fmt.Errorf("invalid vacancy id")
	}
	if req.Top == 0 {
		req.Top = 100
	}
	if req.Top < 0 || req.Top > 1000 {
		return Result{}, fmt.Errorf("top must be 1..1000")
	}
	if req.Workers == 0 {
		req.Workers = s.workers
	}
	if req.Workers < 1 || req.Workers > 64 {
		return Result{}, fmt.Errorf("workers must be 1..64")
	}
	if req.CVLimit == 0 {
		req.CVLimit = s.maxCVs
	}
	if req.CVLimit < 1 || req.CVLimit > s.maxCVs {
		return Result{}, fmt.Errorf("cv_limit must be 1..%d", s.maxCVs)
	}
	v, err := s.repo.GetVacancy(ctx, req.VacancyID)
	if err != nil {
		return Result{}, err
	}
	cvs, err := s.repo.ListCVs(ctx, req.CVLimit)
	if err != nil {
		return Result{}, err
	}
	var matches []matching.Match
	switch req.Mode {
	case "", "worker_pool":
		matches, err = matching.WorkerPool(ctx, v, cvs, req.Top, req.Workers)
	case "sequential":
		matches, err = matching.Sequential(ctx, v, cvs, req.Top)
	default:
		return Result{}, fmt.Errorf("unsupported mode")
	}
	if err != nil {
		return Result{}, err
	}
	if matches == nil {
		matches = []matching.Match{}
	}
	return Result{VacancyID: req.VacancyID, Evaluated: len(cvs), Matches: matches}, nil
}
