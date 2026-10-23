package node

import (
	stdctx "context"
	"strings"
	"testing"

	domainbridge "github.com/wensboy/automan/backend/domain/bridge"
)

// 没有 command 时必须显式报错(不做语言/路径兜底)。
func TestStartRequiresCommand(t *testing.T) {
	runner := New()
	err := runner.Start(stdctx.Background(), domainbridge.RunnerSpec{
		Daemon: "node",
		Params: []byte(`{"args":["${AUTOMAN_ASSET}/index.js"]}`),
	})
	if err == nil || !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("应报 command is required, got %v", err)
	}
}

// options 通过变参注入(构造参数与运行态分离)。
func TestNewAppliesOptions(t *testing.T) {
	runner := New(WithLogger(nil))
	if runner.logger != nil {
		t.Fatal("WithLogger(nil) 应保持 nil")
	}
}
