package flat

import "testing"

// 模板替换: 命中则替换; resolve 不可用则整条丢弃。
func TestResolve(t *testing.T) {
	root := func() (string, bool) { return "/repo", true }
	if got, ok := Resolve("A=${AUTOMAN_ROOT}/x", "${AUTOMAN_ROOT}", root); !ok || got != "A=/repo/x" {
		t.Fatalf("替换异常: %q ok=%v", got, ok)
	}
	missing := func() (string, bool) { return "", false }
	if _, ok := Resolve("B=${AUTOMAN_ROOT}/x", "${AUTOMAN_ROOT}", missing); ok {
		t.Fatal("解析不到时应丢弃整条")
	}
	// 不含模板的串原样返回
	if got, ok := Resolve("C=1", "${AUTOMAN_ROOT}", missing); !ok || got != "C=1" {
		t.Fatalf("无模板不应丢弃: %q ok=%v", got, ok)
	}
}

// 批量: 丢弃空串与不可解析项。
func TestResolveAll(t *testing.T) {
	root := func() (string, bool) { return "/r", true }
	got := ResolveAll([]string{"", "A=1", "B=${AUTOMAN_ROOT}"}, "${AUTOMAN_ROOT}", root)
	if len(got) != 2 || got[0] != "A=1" || got[1] != "B=/r" {
		t.Fatalf("批量结果异常: %+v", got)
	}
}

// 键名/取值: 只拆第一个等号。
func TestKeyNamesAndValues(t *testing.T) {
	entries := []string{"A=1", "B=x=y", "FLAG"}
	if got := KeyNames(entries); got != "A,B" {
		t.Fatalf("键名异常: %q", got)
	}
	values := Values(entries)
	if len(values) != 3 || values[0] != "1" || values[1] != "x=y" || values[2] != "FLAG" {
		t.Fatalf("取值异常: %+v", values)
	}
}
