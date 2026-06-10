package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/nikiv/ansible-ui/internal/model"
)

// ListProjectMembers returns a project's members (joined with the user record).
func (s *Store) ListProjectMembers(ctx context.Context, projectID string) ([]*model.ProjectMember, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT pm.project_id, pm.user_id, u.username, u.email, pm.role, pm.created_at
		FROM project_members pm JOIN users u ON u.id = pm.user_id
		WHERE pm.project_id=$1 ORDER BY pm.created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.ProjectMember
	for rows.Next() {
		var m model.ProjectMember
		if err := rows.Scan(&m.ProjectID, &m.UserID, &m.Username, &m.Email, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// SetProjectMember adds or updates a member's role (upsert).
func (s *Store) SetProjectMember(ctx context.Context, projectID, userID, role string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO project_members (project_id, user_id, role) VALUES ($1,$2,$3)
		ON CONFLICT (project_id, user_id) DO UPDATE SET role=excluded.role`,
		projectID, userID, role)
	return err
}

// RemoveProjectMember removes a member from a project.
func (s *Store) RemoveProjectMember(ctx context.Context, projectID, userID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM project_members WHERE project_id=$1 AND user_id=$2`, projectID, userID)
	return err
}

// GetProjectRole returns a user's role in a project, or "" if not a member.
func (s *Store) GetProjectRole(ctx context.Context, projectID, userID string) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx, `SELECT role FROM project_members WHERE project_id=$1 AND user_id=$2`, projectID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return role, err
}

// ProjectMemberCount reports how many members a project has (0 = unrestricted).
func (s *Store) ProjectMemberCount(ctx context.Context, projectID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM project_members WHERE project_id=$1`, projectID).Scan(&n)
	return n, err
}
