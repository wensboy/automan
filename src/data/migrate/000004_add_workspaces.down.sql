DROP INDEX IF EXISTS idx_proxies_workspace_name;
CREATE UNIQUE INDEX IF NOT EXISTS idx_proxies_name_active ON proxies (name) WHERE deleted_at = 0;
DROP INDEX IF EXISTS idx_proxies_workspace;
ALTER TABLE proxies DROP COLUMN workspace_id;
DROP INDEX IF EXISTS idx_workspaces_user_name;
DROP TABLE IF EXISTS workspaces;
