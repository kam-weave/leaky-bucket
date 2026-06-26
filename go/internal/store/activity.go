package store

import (
	"context"
	"fmt"
	"time"
)

type ActivityLog struct {
	ID           int64     `json:"id"`
	UserID       string    `json:"user_id"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *Store) LogActivity(ctx context.Context, userID, action, resourceType, resourceID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO activity_log (user_id, action, resource_type, resource_id, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, userID, action, resourceType, resourceID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	return nil
}

type ActivityReport struct {
	UserID       string         `json:"user_id"`
	TotalActions int            `json:"total_actions"`
	ByAction     map[string]int `json:"by_action"`
	ByResource   map[string]int `json:"by_resource"`
}

func (s *Store) GenerateActivityReport(ctx context.Context, since time.Time) ([]ActivityReport, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT user_id, action, resource_type, COUNT(*) as cnt
		FROM activity_log
		WHERE created_at >= ?
		GROUP BY user_id, action, resource_type
		ORDER BY user_id
	`, since)
	if err != nil {
		return nil, fmt.Errorf("query activity: %w", err)
	}
	defer rows.Close()

	reportMap := make(map[string]*ActivityReport)
	for rows.Next() {
		var userID, action, resourceType string
		var count int
		if err := rows.Scan(&userID, &action, &resourceType, &count); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}

		report, ok := reportMap[userID]
		if !ok {
			report = &ActivityReport{
				UserID:     userID,
				ByAction:   make(map[string]int),
				ByResource: make(map[string]int),
			}
			reportMap[userID] = report
		}
		report.TotalActions += count
		report.ByAction[action] += count
		report.ByResource[resourceType] += count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	reports := make([]ActivityReport, 0, len(reportMap))
	for _, r := range reportMap {
		reports = append(reports, *r)
	}
	return reports, nil
}
