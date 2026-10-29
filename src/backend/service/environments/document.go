// 本文件负责模板渲染与 env 文档解析(压缩、结构校验、类型转换)。
package environments

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"text/template"

	domainenvironments "github.com/wensboy/automan/backend/domain/environments"
)

// entryName 是模板注入的条目键, 与 data/default/env.tpl.json 的 meta.entry 一致。
const entryName = "environments"

// envTemplateData 是 env 模板的渲染上下文: 模板按 Go 标准库 text/template 语义写
// {{.Dir}} / {{.Filename}} / {{.Entry}}。
type envTemplateData struct {
	Dir      string
	Filename string
	Entry    string
}

// compactJSON 去除空白(存储/缓存用), 失败时返回原始内容。
func compactJSON(payload []byte) []byte {
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, payload); err != nil {
		return payload
	}
	return buffer.Bytes()
}

func (s *Service) renderTemplate(rel string, entries []domainenvironments.Entry) (string, error) {
	if s.data == nil {
		return "", fmt.Errorf("environments: template unavailable")
	}
	payload, err := fs.ReadFile(s.data, s.template)
	if err != nil {
		return "", fmt.Errorf("environments: read template: %w", err)
	}

	directory := path.Dir(rel)
	if directory == "." {
		directory = ""
	}

	tpl, err := template.New(path.Base(s.template)).Parse(string(payload))
	if err != nil {
		return "", fmt.Errorf("%w: %v", domainenvironments.ErrInvalidFile, err)
	}
	var buffer bytes.Buffer
	if err := tpl.Execute(&buffer, envTemplateData{
		Dir:      directory,
		Filename: path.Base(rel),
		Entry:    entryName,
	}); err != nil {
		return "", fmt.Errorf("%w: %v", domainenvironments.ErrUnknownPlace, err)
	}
	rendered := buffer.String()
	if len(entries) == 0 {
		return rendered, nil
	}

	var document map[string]any
	if err := json.Unmarshal([]byte(rendered), &document); err != nil {
		return "", fmt.Errorf("%w: %v", domainenvironments.ErrInvalidFile, err)
	}
	document[entryName] = entries
	formatted, err := json.MarshalIndent(document, "", "    ")
	if err != nil {
		return "", err
	}
	return string(formatted) + "\n", nil
}

func (s *Service) parseDocument(rel string, payload []byte) (domainenvironments.Document, error) {
	var root map[string]any
	if err := json.Unmarshal(payload, &root); err != nil {
		return domainenvironments.Document{}, fmt.Errorf("%w: %v", domainenvironments.ErrInvalidFile, err)
	}
	metaRaw, ok := root["meta"].(map[string]any)
	if !ok {
		return domainenvironments.Document{}, fmt.Errorf("%w: meta is missing", domainenvironments.ErrInvalidFile)
	}
	entry := strings.TrimSpace(fmt.Sprint(metaRaw["entry"]))
	if entry == "" || entry == "<nil>" {
		return domainenvironments.Document{}, fmt.Errorf("%w: meta.entry is missing", domainenvironments.ErrInvalidFile)
	}
	listRaw, ok := root[entry]
	if !ok {
		return domainenvironments.Document{}, fmt.Errorf("%w: entry %q not found", domainenvironments.ErrInvalidFile, entry)
	}
	list, ok := listRaw.([]any)
	if !ok {
		return domainenvironments.Document{}, fmt.Errorf("%w: entry %q must be an array", domainenvironments.ErrInvalidFile, entry)
	}
	if s.maxEntry > 0 && len(list) > s.maxEntry {
		return domainenvironments.Document{}, fmt.Errorf("%w: entry %q exceeds %d items", domainenvironments.ErrInvalidFile, entry, s.maxEntry)
	}

	values := map[string]string{}
	entries := make([]domainenvironments.Entry, 0, len(list))
	for index, item := range list {
		record, ok := item.(map[string]any)
		if !ok {
			return domainenvironments.Document{}, fmt.Errorf("%w: item #%d is not an object", domainenvironments.ErrInvalidFile, index+1)
		}
		key := strings.TrimSpace(fmt.Sprint(record["key"]))
		valueType := strings.TrimSpace(fmt.Sprint(record["type"]))
		if key == "" || key == "<nil>" {
			return domainenvironments.Document{}, fmt.Errorf("%w: item #%d key is required", domainenvironments.ErrInvalidFile, index+1)
		}
		validType := false
		for _, candidate := range domainenvironments.Types {
			if candidate == valueType {
				validType = true
				break
			}
		}
		if !validType {
			return domainenvironments.Document{}, fmt.Errorf("%w: item #%d unknown type %q", domainenvironments.ErrInvalidFile, index+1, valueType)
		}
		rendered, renderErr := renderValue(valueType, record["value"])
		if renderErr != nil {
			return domainenvironments.Document{}, fmt.Errorf("%w: item #%d (%s): %v", domainenvironments.ErrInvalidFile, index+1, key, renderErr)
		}
		values[key] = rendered
		entries = append(entries, domainenvironments.Entry{Type: valueType, Key: key, Value: rendered})
	}

	meta := map[string]string{}
	for name, value := range metaRaw {
		meta[name] = fmt.Sprint(value)
	}

	return domainenvironments.Document{
		Path:    rel,
		Name:    path.Base(rel),
		Content: string(payload),
		Entry:   entry,
		Entries: entries,
		Values:  values,
		Meta:    meta,
	}, nil
}

func renderValue(valueType string, raw any) (string, error) {
	switch valueType {
	case "string":
		return fmt.Sprint(raw), nil
	case "int":
		number, err := toFloat(raw)
		if err != nil || number != float64(int64(number)) {
			return "", fmt.Errorf("value is not an integer")
		}
		return strconv.FormatInt(int64(number), 10), nil
	case "float":
		number, err := toFloat(raw)
		if err != nil {
			return "", fmt.Errorf("value is not a number")
		}
		return strconv.FormatFloat(number, 'f', -1, 64), nil
	case "bool":
		if boolean, ok := raw.(bool); ok {
			return strconv.FormatBool(boolean), nil
		}
		if text, ok := raw.(string); ok {
			boolean, err := strconv.ParseBool(strings.TrimSpace(text))
			if err != nil {
				return "", fmt.Errorf("value is not a boolean")
			}
			return strconv.FormatBool(boolean), nil
		}
		return "", fmt.Errorf("value is not a boolean")
	case "array":
		value, err := toJSONValue(raw)
		if err != nil {
			return "", err
		}
		if _, ok := value.([]any); !ok {
			return "", fmt.Errorf("value is not an array")
		}
		payload, _ := json.Marshal(value)
		return string(payload), nil
	default:
		value, err := toJSONValue(raw)
		if err != nil {
			return "", err
		}
		if _, ok := value.(map[string]any); !ok {
			return "", fmt.Errorf("value is not an object")
		}
		payload, _ := json.Marshal(value)
		return string(payload), nil
	}
}

func toFloat(raw any) (float64, error) {
	switch value := raw.(type) {
	case float64:
		return value, nil
	case string:
		return strconv.ParseFloat(strings.TrimSpace(value), 64)
	default:
		return 0, fmt.Errorf("value is not numeric")
	}
}

func toJSONValue(raw any) (any, error) {
	if text, ok := raw.(string); ok {
		var decoded any
		if err := json.Unmarshal([]byte(text), &decoded); err != nil {
			return nil, fmt.Errorf("value is not valid JSON: %v", err)
		}
		return decoded, nil
	}
	return raw, nil
}
