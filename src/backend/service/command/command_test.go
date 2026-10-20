package command

import (
	"os"
	"strings"
	"testing"

	domaincontext "github.com/wensboy/automan/backend/domain/context"
)

func realService(t *testing.T) *Service {
	t.Helper()
	return New(os.DirFS("../../.."), nil) // ../../.. = src/
}

// 清单: 只取 runtime, 顺序与 groups 一致, id = group.cmd。
func TestListReadsRuntimeGroups(t *testing.T) {
	groups, err := realService(t).List(nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(groups) != 4 {
		t.Fatalf("应返回 4 个分组, got %+v", groups)
	}
	first := groups[0]
	if first.Name != "setting" || len(first.Items) != 2 {
		t.Fatalf("首组异常: %+v", first)
	}
	if first.Items[0].ID != "setting.open" || first.Items[0].Label != "Open" {
		t.Fatalf("条目异常: %+v", first.Items[0])
	}
	if groups[2].Name != "proxy" || len(groups[2].Items) != 0 {
		t.Fatalf("空分组也应保留: %+v", groups[2])
	}
}

// 执行: 内置命令返回 action; 未实现/格式错误明确报错。
func TestRunRoutesAndValidates(t *testing.T) {
	svc := realService(t)

	result, err := svc.Run(nil, "setting.open")
	if err != nil {
		t.Fatalf("Run(setting.open): %v", err)
	}
	if !result.OK || result.Action != "openSettings" {
		t.Fatalf("结果异常: %+v", result)
	}

	if _, err := svc.Run(nil, "proxy.start"); err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("未实现命令应报错, got %v", err)
	}
	if _, err := svc.Run(nil, "no-dot"); err == nil {
		t.Fatalf("格式错误应报错")
	}
	if _, err := svc.Run(nil, "  "); err == nil {
		t.Fatalf("空 id 应报错")
	}
}

// 扩展点: Register 后即可路由; 重复注册被拒。
func TestRegisterExtension(t *testing.T) {
	svc := realService(t)
	err := svc.Register("proxy.start", func(_ domaincontext.Context, _ Request) (Result, error) {
		return Result{Action: "noop"}, nil
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := svc.Register("proxy.start", func(_ domaincontext.Context, _ Request) (Result, error) { return Result{}, nil }); err == nil {
		t.Fatal("重复注册应报错")
	}
}
