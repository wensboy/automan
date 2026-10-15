package bootstrap

import (
	stdctx "context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
	domainstore "github.com/wensboy/automan/backend/domain/store"
	"github.com/wensboy/automan/backend/internal/flat"
	"github.com/wensboy/automan/backend/internal/kits/enginekit"
	"github.com/wensboy/automan/backend/internal/kits/schemakit"
	"github.com/wensboy/automan/backend/internal/kits/storekit"
)

// 与设置相关的三类数据文件:
//   - data/schema/settings.user.schema.json : 键与类型的**唯一权威**(决定哪些键可读写)
//   - data/defaults/settings.user.json      : 默认值层(候选 B: 与用户文件模板分离)
//   - data/schema/settings.user.json        : 写到用户目录的**初始用户文件**(只放最小配置)
//
// 用户目录侧两份(必须同目录: 用户文件里 "$schema": "./settings.user.schema.json" 依赖这一点):
//   - settings.json             : 缺失时创建, 之后不覆盖(保留用户改动)
//   - settings.user.schema.json : **每次启动覆盖**, 随应用更新自动同步最新 schema
const (
	userSettingsFile = "settings.json"
	userSchemaFile   = "settings.schema.json"
	templateSrc      = "data/schema/settings.user.json"
	schemaSrc        = "data/schema/settings.user.schema.json"
	defaultsSrc      = "data/default/settings.user.json"
)

// loadDefaults 读取默认值层并摊平成"点号键 -> 默认值".
func loadDefaults(data fs.FS) (map[string]any, error) {
	tree, err := readTree(data, defaultsSrc)
	if err != nil {
		return nil, err
	}
	defaults := map[string]any{}
	flat.Paths(defaults, "", tree)
	return defaults, nil
}

// loadSchemaFields 读取 schema, 得到"点号键 -> 字段形态"(键集合与类型的权威来源).
func loadSchemaFields(data fs.FS) (map[string]schemakit.Field, error) {
	raw, err := fs.ReadFile(data, schemaSrc)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: read %s: %w", schemaSrc, err)
	}
	fields, err := schemakit.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: parse %s: %w", schemaSrc, err)
	}
	return fields, nil
}

// readTree 读取一个 JSON 对象文件(去掉 $schema 这类编辑器提示键).
func readTree(data fs.FS, src string) (flat.Tree, error) {
	raw, err := fs.ReadFile(data, src)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: read %s: %w", src, err)
	}
	tree := flat.Tree{}
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, fmt.Errorf("bootstrap: parse %s: %w", src, err)
	}
	delete(tree, "$schema")
	return tree, nil
}

// syncUserFiles 同步用户目录: 用户文件缺失才创建; schema 每次启动覆盖.
func syncUserFiles(ctx domaincontext.Context, data fs.FS) error {
	files, ok := enginekit.Of[domainstore.Store](ctx, "store")
	if !ok {
		return errors.New("bootstrap: store engine unavailable")
	}
	// store 领域走标准库 context (与 settings 引擎内部一致)
	base := stdctx.Background()
	if err := storekit.Mkdir(base, files, "."); err != nil {
		return fmt.Errorf("bootstrap: ensure user dir: %w", err)
	}
	if err := copyIfMissing(base, files, data, templateSrc, userSettingsFile); err != nil {
		return err
	}
	return copyAlways(base, files, data, schemaSrc, userSchemaFile)
}

// copyIfMissing 仅在目标不存在时写入(保留用户已改动的内容).
func copyIfMissing(ctx stdctx.Context, files domainstore.Store, data fs.FS, src, dst string) error {
	if _, err := storekit.Read(ctx, files, dst); err == nil {
		return nil
	}
	return copyAlways(ctx, files, data, src, dst)
}

// copyAlways 每次启动覆盖写入(用于 schema: 应用更新后自动同步最新版本).
func copyAlways(ctx stdctx.Context, files domainstore.Store, data fs.FS, src, dst string) error {
	payload, err := fs.ReadFile(data, src)
	if err != nil {
		return fmt.Errorf("bootstrap: read %s: %w", src, err)
	}
	if err := storekit.Write(ctx, files, dst, payload); err != nil {
		return fmt.Errorf("bootstrap: write %s: %w", dst, err)
	}
	return nil
}
