package about

import "testing"

// application.json 优先, 构建注入兜底。
func TestPickPrecedence(t *testing.T) {
	if got := pick("0.1.0", "v1.2.3"); got != "0.1.0" {
		t.Fatalf("配置非空应优先, got %q", got)
	}
	if got := pick("", "v1.2.3"); got != "v1.2.3" {
		t.Fatalf("配置为空应回落注入值, got %q", got)
	}
	if got := pick("", ""); got != "" {
		t.Fatalf("都为空应返回空, got %q", got)
	}
}
