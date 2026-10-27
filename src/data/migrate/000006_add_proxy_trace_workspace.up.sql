-- 操作追踪需要 workspace 维度(见 v25 §3 修订)
ALTER TABLE proxy_trace ADD COLUMN workspace_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_proxy_trace_workspace ON proxy_trace (workspace_id, created_at DESC);
