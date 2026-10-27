DROP INDEX IF EXISTS idx_proxy_trace_workspace;
ALTER TABLE proxy_trace DROP COLUMN workspace_id;
