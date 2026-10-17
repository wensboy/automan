-- 用户与会话: **以 docs/design/v18.md §3 的最新定义为准**
CREATE TABLE IF NOT EXISTS users (
    id         TEXT    PRIMARY KEY,              -- 应用生成的 ULID(与 proxies 同法)
    username   TEXT    NOT NULL,                 -- 登录名(未删除时唯一)
    password   TEXT    NOT NULL,                 -- bcrypt 哈希(永不存明文)
    nickname   TEXT    NOT NULL DEFAULT '',      -- 昵称(可空)
    created_at INTEGER NOT NULL,                 -- unix 毫秒
    updated_at INTEGER NOT NULL,                 -- unix 毫秒
    deleted_at INTEGER NOT NULL DEFAULT 0        -- unix 毫秒; 0 表示未删除(逻辑删除)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_active ON users (username) WHERE deleted_at = 0;

CREATE TABLE IF NOT EXISTS login_logs (
    id         TEXT    PRIMARY KEY,
    user_id    TEXT    NOT NULL,                 -- 关联 users.id(不加外键约束, 便于保留历史)
    username   TEXT    NOT NULL,                 -- 冗余一份: 用户不存在时也能审计
    result     TEXT    NOT NULL,                 -- success|failed
    reason     TEXT    NOT NULL DEFAULT '',      -- 失败原因(no such user / bad password / expired ...)
    token_hash TEXT    NOT NULL DEFAULT '',      -- 成功时记录本次会话 token 的 SHA-256(供启动恢复校验)
    created_at INTEGER NOT NULL                  -- unix 毫秒(记录创建时间 + 登录时间)
);
-- 注: 文档 §3 原写 `(user_id, at DESC)`, 但列已更名 created_at; 这里按实际列名建索引(否则建索引会失败)
CREATE INDEX IF NOT EXISTS idx_login_logs_user_at ON login_logs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_logs_at ON login_logs (created_at DESC);
