package bootstrap

import (
	"testing"
	"testing/fstest"

	"github.com/wensboy/automan/backend/internal/kits/schemakit"
)

// 默认值层: 空分组不产出键, $schema 不参与, 数组原样保留.
func TestLoadDefaultsFlattensDefaultsFile(t *testing.T) {
	data := fstest.MapFS{
		defaultsSrc: &fstest.MapFile{Data: []byte(`{
			"$schema": "./settings.user.schema.json",
			"appearence": { "mode": "auto" },
			"theme": {},
			"general": { "language": "English" },
			"proxy": { "bypass": ["localhost", "127.0.0.1"], "timeout": 30, "autoStart": false }
		}`)},
	}

	defaults, err := loadDefaults(data)
	if err != nil {
		t.Fatalf("loadDefaults: %v", err)
	}
	want := map[string]any{
		"appearence.mode":  "auto",
		"general.language": "English",
		"proxy.timeout":    float64(30),
		"proxy.autoStart":  false,
	}
	for key, value := range want {
		got, ok := defaults[key]
		if !ok {
			t.Fatalf("缺键 %q (got %v)", key, defaults)
		}
		if got != value {
			t.Fatalf("键 %q want=%v got=%v", key, value, got)
		}
	}
	if _, ok := defaults["$schema"]; ok {
		t.Fatalf("$schema 不应进入默认值表")
	}
	if _, ok := defaults["theme"]; ok {
		t.Fatalf("空分组不应产出键")
	}
	if items, ok := defaults["proxy.bypass"].([]any); !ok || len(items) != 2 {
		t.Fatalf("数组默认值应原样保留, got %v", defaults["proxy.bypass"])
	}
}

func TestLoadDefaultsMissingFile(t *testing.T) {
	if _, err := loadDefaults(fstest.MapFS{}); err == nil {
		t.Fatalf("默认值文件缺失应报错")
	}
}

// 键表来自 schema: 分组展开为点号键, enum/array 形态正确.
func TestLoadSchemaFields(t *testing.T) {
	data := fstest.MapFS{
		schemaSrc: &fstest.MapFile{Data: []byte(`{
			"type": "object",
			"properties": {
				"$schema": { "type": "string" },
				"appearence": { "type": "object", "properties": {
					"mode": { "enum": ["auto", "light", "dark"] },
					"compact": { "type": "boolean" }
				}},
				"proxy": { "type": "object", "properties": {
					"bypass": { "type": "array", "items": { "type": "string" } }
				}}
			}
		}`)},
	}

	fields, err := loadSchemaFields(data)
	if err != nil {
		t.Fatalf("loadSchemaFields: %v", err)
	}
	if field, ok := fields["appearence.mode"]; !ok || field.Kind != schemakit.KindEnum || len(field.Enum) != 3 {
		t.Fatalf("enum 键异常: %+v", fields["appearence.mode"])
	}
	if field, ok := fields["proxy.bypass"]; !ok || field.Kind != schemakit.KindArray || field.Items != schemakit.KindString {
		t.Fatalf("array 键异常: %+v", fields["proxy.bypass"])
	}
	if _, ok := fields["$schema"]; ok {
		t.Fatalf("$schema 不应进入键表")
	}
}

func TestLoadSchemaFieldsRejectsBrokenSchema(t *testing.T) {
	data := fstest.MapFS{schemaSrc: &fstest.MapFile{Data: []byte(`{}`)}}
	if _, err := loadSchemaFields(data); err == nil {
		t.Fatalf("缺少 properties 的 schema 应报错")
	}
}
