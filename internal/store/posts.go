package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"mirage/internal/model"
)

// 分页默认值与上限，对齐《项目说明文档》§5.0。
const (
	DefaultPerPage = 20
	MaxPerPage     = 100
)

// ListPostsOptions 是文章列表的筛选条件。空串 / 0 表示「这一维不过滤」。
//
// 做成结构体是为了 M6：权限层到时候只是把 Status、Visibility 填上，
// 不用改签名，也不用改所有调用方。
type ListPostsOptions struct {
	Section    string // 板块 slug
	Status     string // draft | published
	Visibility string // public | private
	Page       int    // 从 1 起，小于 1 当 1
	PerPage    int    // 默认 20，上限 100
}

// normalize 把分页参数夹到合法区间。
func (o ListPostsOptions) normalize() ListPostsOptions {
	if o.Page < 1 {
		o.Page = 1
	}
	if o.PerPage <= 0 {
		o.PerPage = DefaultPerPage
	}
	if o.PerPage > MaxPerPage {
		o.PerPage = MaxPerPage
	}
	return o
}

// where 拼出 WHERE 子句和一个参数列表。没有条件时返回空串。
//
// 不用 1 = 1 表示「不过滤」——那会让 SQLite 走全表扫描（D20 推论）。
func (o ListPostsOptions) where() (string, []any) {
	var conds []string
	var args []any
	if o.Section != "" {
		conds = append(conds, "section = ?")
		args = append(args, o.Section)
	}
	if o.Status != "" {
		conds = append(conds, "status = ?")
		args = append(args, o.Status)
	}
	if o.Visibility != "" {
		conds = append(conds, "visibility = ?")
		args = append(args, o.Visibility)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// postColumns 明确列字段，不用 SELECT *。
const postColumns = `id, slug, title, summary, body_md, section, author_id, tags,
	cover, status, visibility, reading_minutes, published_at, content_updated_at,
	created_at, updated_at, sort_order`

// ListPosts 返回一页文章，顺序是 published_at DESC, id DESC——id 兜底，
// 保证同一秒发布的两篇顺序稳定。
//
// total 另调 CountPosts：两个查询分开，列表查询能保持简单，也便于各自走索引。
// 草稿的 published_at 是 NULL，SQLite 里 NULL 排最小，DESC 时落在最后。
func (s *Store) ListPosts(ctx context.Context, opts ListPostsOptions) ([]model.Post, error) {
	opts = opts.normalize()
	where, args := opts.where()
	query := "SELECT " + postColumns + " FROM posts" + where +
		" ORDER BY published_at DESC, id DESC LIMIT ? OFFSET ?"
	args = append(args, opts.PerPage, (opts.Page-1)*opts.PerPage)

	rows, err := s.readDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate posts: %w", err)
	}
	return posts, nil
}

// CountPosts 数一下同样的筛选条件下有多少篇，和 ListPosts 配合算 has_more。
func (s *Store) CountPosts(ctx context.Context, opts ListPostsOptions) (int, error) {
	where, args := opts.where()

	var total int
	if err := s.readDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM posts"+where, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count posts: %w", err)
	}
	return total, nil
}

// PostBySlug 按 slug 取一篇，查不到返回 ErrNotFound。
func (s *Store) PostBySlug(ctx context.Context, slug string) (model.Post, error) {
	row := s.readDB.QueryRowContext(ctx,
		"SELECT "+postColumns+" FROM posts WHERE slug = ?", slug)

	p, err := scanPost(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Post{}, ErrNotFound
		}
		return model.Post{}, err
	}
	return p, nil
}

func scanPost(sc scanner) (model.Post, error) {
	var p model.Post
	err := sc.Scan(
		&p.ID, &p.Slug, &p.Title, &p.Summary, &p.BodyMD, &p.Section, &p.AuthorID, &p.Tags,
		&p.Cover, &p.Status, &p.Visibility, &p.ReadingMinutes, &p.PublishedAt,
		&p.ContentUpdatedAt, &p.CreatedAt, &p.UpdatedAt, &p.SortOrder,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Post{}, err
		}
		return model.Post{}, fmt.Errorf("scan post: %w", err)
	}
	return p, nil
}
