package context

import "io/fs"

// Program 是 **CLI 启动所需的外部输入**: 原始参数与内嵌资源。
//
// 与 Context(根上下文接口: 生命周期 + 引擎表 + 服务表)分开:
//   - Program 只描述"从哪里来、带了什么";
//     · Context 描述"运行起来之后有什么能力"。
type Program struct {
	// Args 是完整的 os.Args(含程序名), 交给 cli 框架解析。
	Args []string
	// Assets 是前端构建产物(data 之外的静态资源)。
	Assets fs.FS
	// Data 是应用数据源(含 data/conf/command.json 等内嵌配置)。
	Data fs.FS
}
