// Package model 是领域模型：只有结构体，不 import 任何项目内的包。
package model

// Section 是一个板块。
type Section struct {
	Slug       string
	Title      string
	Icon       string
	Summary    string
	Color      string
	IntroMD    string
	Cover      *string
	Visibility string // public | private
	InNav      bool   // 是否出现在顶栏导航
	CountLabel string // 卡片右下角的计数文案，手填，留空不显示
	SortOrder  int
	CreatedAt  int64
	UpdatedAt  int64
}

// Post 是一篇文章。时间一律 Unix 秒，可空的字段用指针。
type Post struct {
	ID               int64
	Slug             string
	Title            string
	Summary          string
	BodyMD           string
	Section          string
	AuthorID         *int64
	Tags             string // 原始 JSON 数组文本，如 ["Go","Nginx"]
	Cover            *string
	Status           string // draft | published
	Visibility       string // public | private
	ReadingMinutes   int
	PublishedAt      *int64 // 草稿为空
	ContentUpdatedAt *int64
	CreatedAt        int64
	UpdatedAt        int64
	SortOrder        int
}
