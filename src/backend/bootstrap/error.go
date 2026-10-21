package bootstrap

import (
	"errors"

	domainconfig "github.com/wensboy/automan/backend/domain/config"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/configkit"
)

// registerErrors 把 data/conf/error.json 的定义注册到 apperr.
// 启动早期执行, 之后所有服务/绑定层报错都复用这套码.
func registerErrors(provider domainconfig.Provider) error {
	defs, ok := configkit.Get(provider, domainconfig.ErrorKey)
	if !ok {
		return errors.New("bootstrap: error definitions unavailable")
	}
	entries := make([]apperr.Entry, 0, len(defs.Builtin)+len(defs.Service))
	for _, item := range defs.Builtin {
		entries = append(entries, apperr.Entry{Code: item.Code, Msg: item.Msg})
	}
	for _, item := range defs.Service {
		entries = append(entries, apperr.Entry{Code: item.Code, Msg: item.Msg})
	}
	return apperr.Register(entries)
}
