package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"mirage/internal/model"
)

// sectionColumns 明确列字段，不用 SELECT *：加字段时应该由人决定要不要读它。
const sectionColumns = `slug, title, icon, summary, color, intro_md,
	cover, visibility, in_nav, count_label, sort_order, created_at, updated_at`

// ListSections 返回全部板块，按 sort_order 升序、slug 兜底保证顺序稳定。
//
// 可见性过滤不在这里做——私密板块对访客是否可见是权限层的事（M6）。
func (s *Store) ListSections(ctx context.Context) ([]model.Section, error) {
	rows, err := s.readDB.QueryContext(ctx,
		"SELECT "+sectionColumns+" FROM sections ORDER BY sort_order, slug")
	if err != nil {
		return nil, fmt.Errorf("list sections: %w", err)
	}
	defer rows.Close()

	var sections []model.Section
	for rows.Next() {
		sec, err := scanSection(rows)
		if err != nil {
			return nil, err
		}
		sections = append(sections, sec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sections: %w", err)
	}
	return sections, nil
}

// SectionBySlug 按 slug 取一个板块，查不到返回 ErrNotFound。
func (s *Store) SectionBySlug(ctx context.Context, slug string) (model.Section, error) {
	row := s.readDB.QueryRowContext(ctx,
		"SELECT "+sectionColumns+" FROM sections WHERE slug = ?", slug)

	sec, err := scanSection(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Section{}, ErrNotFound
		}
		return model.Section{}, err
	}
	return sec, nil
}

// scanner 抹平 *sql.Row 和 *sql.Rows 的差别，两者都有 Scan。
type scanner interface {
	Scan(dest ...any) error
}

func scanSection(sc scanner) (model.Section, error) {
	var sec model.Section
	err := sc.Scan(
		&sec.Slug, &sec.Title, &sec.Icon, &sec.Summary, &sec.Color, &sec.IntroMD,
		&sec.Cover, &sec.Visibility, &sec.InNav, &sec.CountLabel, &sec.SortOrder,
		&sec.CreatedAt, &sec.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Section{}, err
		}
		return model.Section{}, fmt.Errorf("scan section: %w", err)
	}
	return sec, nil
}
