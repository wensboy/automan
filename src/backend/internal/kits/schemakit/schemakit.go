// Package schemakit 读取用户配置的 JSON Schema, 得到"点号键 -> 字段形态"的键集合.
// 只做最小解析(properties/enum/type/items), 不引入校验库: 类型都固定在 schema 里,
// 服务层据此判断键是否存在与类型是否匹配(见 docs/design/v15.md 第 5 节"路线 1"、第 12 节 D)。
package schemakit

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Kind 是键的形态.
type Kind string

const (
	KindString  Kind = "string"
	KindNumber  Kind = "number"
	KindBoolean Kind = "boolean"
	KindArray   Kind = "array"
	KindEnum    Kind = "enum"
)

// Field 描述一个可配置键.
type Field struct {
	Kind Kind
	// Enum 是 KindEnum 时的允许值集合.
	Enum []any
	// Items 是 KindArray 时的元素形态.
	Items Kind
}

// Parse 解析 schema 文本, 返回"点号键 -> 字段".
// 只收集 properties 树中的叶子; 根上的 $schema 属于编辑器提示, 不参与。
func Parse(raw []byte) (map[string]Field, error) {
	root := map[string]any{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("schemakit: parse schema: %w", err)
	}
	fields := map[string]Field{}
	properties, ok := asMap(root["properties"])
	if !ok {
		return nil, errors.New("schemakit: schema has no properties")
	}
	walk(properties, "", fields)
	if len(fields) == 0 {
		return nil, errors.New("schemakit: schema declares no keys")
	}
	return fields, nil
}

func walk(properties map[string]any, prefix string, out map[string]Field) {
	for name, raw := range properties {
		if name == "$schema" {
			continue
		}
		key := name
		if prefix != "" {
			key = prefix + "." + name
		}
		node, ok := asMap(raw)
		if !ok {
			continue
		}
		if nested, ok := asMap(node["properties"]); ok {
			walk(nested, key, out)
			continue
		}
		if field, ok := parseField(node); ok {
			out[key] = field
		}
	}
}

func parseField(node map[string]any) (Field, bool) {
	if values, ok := node["enum"].([]any); ok {
		return Field{Kind: KindEnum, Enum: values}, true
	}
	kind, ok := node["type"].(string)
	if !ok {
		return Field{}, false
	}
	switch kind {
	case "string":
		return Field{Kind: KindString}, true
	case "number", "integer":
		return Field{Kind: KindNumber}, true
	case "boolean":
		return Field{Kind: KindBoolean}, true
	case "array":
		items, _ := asMap(node["items"])
		itemKind, _ := items["type"].(string)
		return Field{Kind: KindArray, Items: Kind(itemKind)}, true
	default:
		return Field{}, false
	}
}

func asMap(value any) (map[string]any, bool) {
	m, ok := value.(map[string]any)
	return m, ok
}
