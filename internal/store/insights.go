package store

import (
	"context"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

// RunInsights aggregates run analytics over the last `days` days, optionally
// scoped to a project: per-day counts by outcome, plus success rate, average
// duration (started→finished) and average dispatch wait (created→started).
func (s *Store) RunInsights(ctx context.Context, projectID string, days int) (*model.Insights, error) {
	if days <= 0 || days > 90 {
		days = 14
	}
	out := &model.Insights{Days: days}

	scope := ""
	args := []any{days}
	if projectID != "" {
		scope = " AND project_id=$2"
		args = append(args, projectID)
	}

	// Per-day buckets.
	rows, err := s.pool.Query(ctx, `
		SELECT to_char(created_at, 'YYYY-MM-DD') AS d, status, count(*)
		FROM runs
		WHERE created_at > now() - make_interval(days => $1)`+scope+`
		GROUP BY d, status`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byDay := map[string]*model.DayBucket{}
	for rows.Next() {
		var d, status string
		var n int
		if err := rows.Scan(&d, &status, &n); err != nil {
			return nil, err
		}
		b := byDay[d]
		if b == nil {
			b = &model.DayBucket{Date: d}
			byDay[d] = b
		}
		switch status {
		case model.StatusSuccess:
			b.Success += n
		case model.StatusFailed:
			b.Failed += n
		default:
			b.Other += n
		}
		b.Total += n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Fill every day in the window (including empty ones), oldest → newest.
	today := time.Now()
	for i := days - 1; i >= 0; i-- {
		key := today.AddDate(0, 0, -i).Format("2006-01-02")
		if b := byDay[key]; b != nil {
			out.PerDay = append(out.PerDay, *b)
		} else {
			out.PerDay = append(out.PerDay, model.DayBucket{Date: key})
		}
	}

	// Totals + averages over the same window.
	var success, finished int
	err = s.pool.QueryRow(ctx, `
		SELECT
		  count(*),
		  count(*) FILTER (WHERE status='success'),
		  count(*) FILTER (WHERE status IN ('success','failed')),
		  coalesce(avg(extract(epoch FROM finished_at - started_at)) FILTER (WHERE finished_at IS NOT NULL AND started_at IS NOT NULL), 0),
		  coalesce(avg(extract(epoch FROM started_at - created_at)) FILTER (WHERE started_at IS NOT NULL), 0)
		FROM runs
		WHERE created_at > now() - make_interval(days => $1)`+scope, args...).
		Scan(&out.Total, &success, &finished, &out.AvgDurationSec, &out.AvgWaitSec)
	if err != nil {
		return nil, err
	}
	if finished > 0 {
		out.SuccessRate = float64(success) / float64(finished) * 100
	}

	// Per-template breakdown (busiest first), same window.
	tscope := ""
	if projectID != "" {
		tscope = " AND r.project_id=$2"
	}
	trows, err := s.pool.Query(ctx, `
		SELECT r.template_id, t.name, count(*),
		  count(*) FILTER (WHERE r.status='success'),
		  count(*) FILTER (WHERE r.status IN ('success','failed')),
		  coalesce(avg(extract(epoch FROM r.finished_at - r.started_at)) FILTER (WHERE r.finished_at IS NOT NULL AND r.started_at IS NOT NULL), 0)
		FROM runs r JOIN templates t ON t.id = r.template_id
		WHERE r.created_at > now() - make_interval(days => $1) AND r.template_id IS NOT NULL`+tscope+`
		GROUP BY r.template_id, t.name
		ORDER BY count(*) DESC LIMIT 25`, args...)
	if err != nil {
		return nil, err
	}
	defer trows.Close()
	for trows.Next() {
		var ti model.TemplateInsight
		var fin int
		var succ int
		if err := trows.Scan(&ti.TemplateID, &ti.TemplateName, &ti.Runs, &succ, &fin, &ti.AvgDurationSec); err != nil {
			return nil, err
		}
		if fin > 0 {
			ti.SuccessRate = float64(succ) / float64(fin) * 100
		}
		out.ByTemplate = append(out.ByTemplate, ti)
	}
	if err := trows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
