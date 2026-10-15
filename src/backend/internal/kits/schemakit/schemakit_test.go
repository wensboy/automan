package schemakit

import (
	"testing"
)

// 解析真实 schema 的关键片段: 嵌套分组展开为点号键; enum/array 带出允许值与元素形态.
func TestParseNestedProperties(t *testing.T) {
	raw := []byte(`{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"$schema": { "type": "string" },
			"appearence": {
				"type": "object",
				"properties": {
					"mode": { "enum": ["auto", "light", "dark"] },
					"compact": { "type": "boolean" }
				}
			},
			"proxy": {
				"type": "object",
				"properties": {
					"timeout": { "type": "number", "minimum": 1, "maximum": 120 },
					"bypass": { "type": "array", "items": { "type": "string" } },
					"userAgent": { "type": "string" }
				}
			}
		}
	}`)

	fields, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, ok := fields["$schema"]; ok {
		t.Fatalf("$schema 不应进入键集合")
	}
	if field, ok := fields["appearence.mode"]; !ok || field.Kind != KindEnum || len(field.Enum) != 3 {
		t.Fatalf("enum 键解析异常: %+v", fields["appearence.mode"])
	}
	if field, ok := fields["appearence.compact"]; !ok || field.Kind != KindBoolean {
		t.Fatalf("boolean 键解析异常: %+v", fields["appearence.compact"])
	}
	if field, ok := fields["proxy.timeout"]; !ok || field.Kind != KindNumber {
		t.Fatalf("number 键解析异常: %+v", fields["proxy.timeout"])
	}
	if field, ok := fields["proxy.bypass"]; !ok || field.Kind != KindArray || field.Items != KindString {
		t.Fatalf("array 键解析异常: %+v", fields["proxy.bypass"])
	}
	if _, ok := fields["proxy"]; ok {
		t.Fatalf("分组本身不应作为键")
	}
}

func TestParseRejectsBrokenSchema(t *testing.T) {
	if _, err := Parse([]byte(`{"type":"object"}`)); err == nil {
		t.Fatalf("没有 properties 应报错")
	}
	if _, err := Parse([]byte(`not-json`)); err == nil {
		t.Fatalf("非法 JSON 应报错")
	}
}
