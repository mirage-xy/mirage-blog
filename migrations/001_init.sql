-- 001_init.sql · M2 数据层：四张核心表
--
-- 依《决策记录》D20 与《项目说明文档》§4.1。表结构一字未改。
-- 迁移只增不改：这个文件一旦发布就不要再动，改结构请加 002_xxx.sql。
-- schema_migrations 不在这里建——它由迁移器自己 CREATE TABLE IF NOT EXISTS（D20 推论）。

CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    display_name  TEXT    NOT NULL,
    email         TEXT,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL DEFAULT 'owner',
    status        TEXT    NOT NULL DEFAULT 'active',
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    last_login_at INTEGER
);

CREATE TABLE sessions (
    id         TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    user_agent TEXT,
    ip         TEXT
);

CREATE INDEX idx_sessions_user    ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

CREATE TABLE sections (
    slug        TEXT PRIMARY KEY,
    title       TEXT    NOT NULL,
    icon        TEXT    NOT NULL DEFAULT '📁',
    summary     TEXT    NOT NULL DEFAULT '',
    color       TEXT    NOT NULL DEFAULT '#C26D7C',
    intro_md    TEXT    NOT NULL DEFAULT '',
    cover       TEXT,
    visibility  TEXT    NOT NULL DEFAULT 'public',
    in_nav      INTEGER NOT NULL DEFAULT 1,
    count_label TEXT    NOT NULL DEFAULT '',
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

-- posts.section 不建外键：板块 slug 会改名，加了外键改名要连带改一堆文章（D20 推论）。
CREATE TABLE posts (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    slug               TEXT    NOT NULL UNIQUE,
    title              TEXT    NOT NULL,
    summary            TEXT    NOT NULL DEFAULT '',
    body_md            TEXT    NOT NULL,
    section            TEXT    NOT NULL DEFAULT 'notes',
    author_id          INTEGER REFERENCES users(id) ON DELETE SET NULL,
    tags               TEXT    NOT NULL DEFAULT '[]',
    cover              TEXT,
    status             TEXT    NOT NULL DEFAULT 'draft',
    visibility         TEXT    NOT NULL DEFAULT 'public',
    reading_minutes    INTEGER NOT NULL DEFAULT 1,
    published_at       INTEGER,
    content_updated_at INTEGER,
    created_at         INTEGER NOT NULL,
    updated_at         INTEGER NOT NULL,
    sort_order         INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_posts_status  ON posts(status, published_at DESC);
CREATE INDEX idx_posts_section ON posts(section, status, published_at DESC);