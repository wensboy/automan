-- 代理归属用户(多对一): 查询一律收束到"当前用户"的代理。
-- 历史行没有归属信息 -> 默认空串(登录后不可见; 如需接管由后续迁移/一次性脚本处理)。
ALTER TABLE proxies ADD COLUMN user_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_proxies_user ON proxies (user_id, deleted_at);
