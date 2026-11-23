package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yura888840/go-cv-matching-profiling-service/internal/matching"
	"github.com/yura888840/go-cv-matching-profiling-service/internal/service"
)

type Repository struct{ pool *pgxpool.Pool }

func New(ctx context.Context, url string) (*Repository, error) {
	p, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err = p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	return &Repository{p}, nil
}
func (r *Repository) Close() { r.pool.Close() }
func (r *Repository) GetVacancy(ctx context.Context, id int64) (matching.Vacancy, error) {
	var v matching.Vacancy
	var required int64
	err := r.pool.QueryRow(ctx, `SELECT id, required_skills, min_years, region, max_salary FROM vacancies WHERE id=$1`, id).Scan(&v.ID, &required, &v.MinYears, &v.Region, &v.MaxSalary)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, service.ErrNotFound
	}
	v.Required = uint64(required)
	return v, err
}
func (r *Repository) ListCVs(ctx context.Context, limit int) ([]matching.CV, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, skills, years, region, salary FROM cvs WHERE active = TRUE ORDER BY id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cvs := make([]matching.CV, 0, min(limit, 10000))
	for rows.Next() {
		var c matching.CV
		var skills int64
		if err := rows.Scan(&c.ID, &skills, &c.Years, &c.Region, &c.Salary); err != nil {
			return nil, err
		}
		c.Skills = uint64(skills)
		cvs = append(cvs, c)
	}
	return cvs, rows.Err()
}

func (r *Repository) Ping(ctx context.Context) error { return r.pool.Ping(ctx) }
