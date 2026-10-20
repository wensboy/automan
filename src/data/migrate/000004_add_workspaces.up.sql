-- 工作区: user 1:n workspace 1:n proxy(见 docs/design/v21.md §2)
CREATE TABLE IF NOT EXISTS workspaces (
    id         TEXT    PRIMARY KEY,
    name       TEXT    NOT NULL,
    user_id    TEXT    NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    deleted_at INTEGER NOT NULL DEFAULT 0
);
-- 同一用户内工作区不可重名(不同用户可重名)
CREATE UNIQUE INDEX IF NOT EXISTS idx_workspaces_user_name
    ON workspaces (user_id, name) WHERE deleted_at = 0;

-- 代理归属工作区(历史行默认空串, 不自动归属)
ALTER TABLE proxies ADD COLUMN workspace_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_proxies_workspace ON proxies (workspace_id, deleted_at);

-- 名字唯一性从"全库"改为"同一工作区内"(不同工作区可同名)
DROP INDEX IF EXISTS idx_proxies_name_active;
CREATE UNIQUE INDEX IF NOT EXISTS idx_proxies_workspace_name
    ON proxies (workspace_id, name) WHERE deleted_at = 0;
