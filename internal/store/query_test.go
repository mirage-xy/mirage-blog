package store

import (
	"context"
	"errors"
	"testing"
)

func insertSection(t *testing.T, s *Store, slug, title string, inNav bool, sortOrder int) {
	t.Helper()
	inNavInt := 0
	if inNav {
		inNavInt = 1
	}
	_, err := s.writeDB.Exec(
		`INSERT INTO sections
		 (slug, title, icon, summary, color, intro_md, cover, visibility, in_nav, count_label, sort_order, created_at, updated_at)
		 VALUES (?, ?, '📓', '', '#C26D7C', '', NULL, 'public', ?, '', ?, 1700000000, 1700000000)`,
		slug, title, inNavInt, sortOrder)
	if err != nil {
		t.Fatalf("insert section %s: %v", slug, err)
	}
}

// insertPost 插一篇文章；publishedAt 传 0 表示草稿（NULL）。
func insertPost(t *testing.T, s *Store, slug, section, status, visibility string, publishedAt int64) {
	t.Helper()
	var pub any
	if publishedAt != 0 {
		pub = publishedAt
	}
	_, err := s.writeDB.Exec(
		`INSERT INTO posts
		 (slug, title, summary, body_md, section, author_id, tags, cover, status, visibility,
		  reading_minutes, published_at, content_updated_at, created_at, updated_at, sort_order)
		 VALUES (?, ?, '', '正文', ?, NULL, '[]', NULL, ?, ?, 1, ?, NULL, 1700000000, 1700000000, 0)`,
		slug, slug, section, status, visibility, pub)
	if err != nil {
		t.Fatalf("insert post %s: %v", slug, err)
	}
}

func mustMigrate(t *testing.T, s *Store) {
	t.Helper()
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
}

func TestListSectionsOrdersBySortOrder(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	mustMigrate(t, s)

	// 故意打乱插入顺序，验证是按 sort_order 排的，不是按插入顺序。
	insertSection(t, s, "guestbook", "留言", true, 20)
	insertSection(t, s, "notes", "手记", true, 10)
	insertSection(t, s, "about", "关于", false, 30)

	sections, err := s.ListSections(ctx)
	if err != nil {
		t.Fatalf("ListSections: %v", err)
	}
	if len(sections) != 3 {
		t.Fatalf("拿到 %d 个板块，想要 3 个", len(sections))
	}

	want := []string{"notes", "guestbook", "about"}
	for i, slug := range want {
		if sections[i].Slug != slug {
			t.Errorf("第 %d 个是 %q，想要 %q", i, sections[i].Slug, slug)
		}
	}
	if sections[0].Title != "手记" || !sections[0].InNav {
		t.Errorf("字段扫描不对：%+v", sections[0])
	}
	if sections[2].InNav {
		t.Error("about 的 in_nav 应该是 false")
	}
}

func TestSectionBySlug(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	mustMigrate(t, s)
	insertSection(t, s, "notes", "手记", true, 10)

	sec, err := s.SectionBySlug(ctx, "notes")
	if err != nil {
		t.Fatalf("SectionBySlug: %v", err)
	}
	if sec.Title != "手记" || sec.Color != "#C26D7C" {
		t.Errorf("字段不对：%+v", sec)
	}

	if _, err := s.SectionBySlug(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("查不存在的板块应该返回 ErrNotFound，拿到 %v", err)
	}
}

func TestListPostsOrderFiltersAndPagination(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	mustMigrate(t, s)

	insertPost(t, s, "old", "notes", "published", "public", 1700000100)
	insertPost(t, s, "new", "notes", "published", "public", 1700000300)
	insertPost(t, s, "mid", "notes", "published", "public", 1700000200)
	insertPost(t, s, "secret", "notes", "published", "private", 1700000400)
	insertPost(t, s, "draft", "notes", "draft", "public", 0) // 草稿没有发布时间
	insertPost(t, s, "about-me", "about", "published", "public", 1700000050)

	// 1) 不带过滤：全部 6 篇，按发布时间倒序，草稿（NULL）落在最后。
	all, err := s.ListPosts(ctx, ListPostsOptions{})
	if err != nil {
		t.Fatalf("ListPosts: %v", err)
	}
	if len(all) != 6 {
		t.Fatalf("不过滤拿到 %d 篇，想要 6 篇", len(all))
	}
	wantOrder := []string{"secret", "new", "mid", "old", "about-me", "draft"}
	for i, slug := range wantOrder {
		if all[i].Slug != slug {
			t.Errorf("第 %d 篇是 %q，想要 %q", i, all[i].Slug, slug)
		}
	}
	if all[len(all)-1].PublishedAt != nil {
		t.Error("草稿的 published_at 应该是 nil")
	}

	// 2) 按板块 + 状态过滤
	notes, err := s.ListPosts(ctx, ListPostsOptions{Section: "notes", Status: "published"})
	if err != nil {
		t.Fatalf("ListPosts(notes): %v", err)
	}
	if len(notes) != 4 {
		t.Errorf("notes 下已发布 %d 篇，想要 4 篇（含私密）", len(notes))
	}

	// 3) 再加可见性过滤
	public, err := s.ListPosts(ctx, ListPostsOptions{Section: "notes", Status: "published", Visibility: "public"})
	if err != nil {
		t.Fatalf("ListPosts(public): %v", err)
	}
	if len(public) != 3 {
		t.Errorf("notes 下已发布且公开 %d 篇，想要 3 篇", len(public))
	}

	// 4) 分页：每页 2 条，翻到第 3 页应该只剩 2 条（共 6 条）
	page3, err := s.ListPosts(ctx, ListPostsOptions{Page: 3, PerPage: 2})
	if err != nil {
		t.Fatalf("ListPosts(page 3): %v", err)
	}
	if len(page3) != 2 {
		t.Errorf("第 3 页拿到 %d 篇，想要 2 篇", len(page3))
	}

	// 5) Count 要跟着同一套过滤条件走
	total, err := s.CountPosts(ctx, ListPostsOptions{Section: "notes", Status: "published", Visibility: "public"})
	if err != nil {
		t.Fatalf("CountPosts: %v", err)
	}
	if total != 3 {
		t.Errorf("CountPosts = %d，想要 3", total)
	}
	if total, err = s.CountPosts(ctx, ListPostsOptions{}); err != nil {
		t.Fatalf("CountPosts(all): %v", err)
	} else if total != 6 {
		t.Errorf("CountPosts(all) = %d，想要 6", total)
	}
}

func TestPostBySlug(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	mustMigrate(t, s)
	insertPost(t, s, "hello", "notes", "published", "public", 1700000100)

	p, err := s.PostBySlug(ctx, "hello")
	if err != nil {
		t.Fatalf("PostBySlug: %v", err)
	}
	if p.Slug != "hello" || p.Status != "published" || p.Tags != "[]" {
		t.Errorf("字段不对：%+v", p)
	}
	if p.PublishedAt == nil || *p.PublishedAt != 1700000100 {
		t.Errorf("published_at 不对：%v", p.PublishedAt)
	}

	if _, err := s.PostBySlug(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("查不到的文章应该返回 ErrNotFound，拿到 %v", err)
	}
}

func TestListPostsOptionsNormalize(t *testing.T) {
	got := ListPostsOptions{Page: -3, PerPage: 0}.normalize()
	if got.Page != 1 {
		t.Errorf("Page 应该是 1，拿到 %d", got.Page)
	}
	if got.PerPage != DefaultPerPage {
		t.Errorf("PerPage 应该是 %d，拿到 %d", DefaultPerPage, got.PerPage)
	}

	got = ListPostsOptions{PerPage: MaxPerPage + 500}.normalize()
	if got.PerPage != MaxPerPage {
		t.Errorf("PerPage 应该被截到 %d，拿到 %d", MaxPerPage, got.PerPage)
	}
}
