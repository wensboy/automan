package store

// 内置指令.
const (
	CmdRead   Cmd = "fs.read"
	CmdWrite  Cmd = "fs.write"
	CmdAppend Cmd = "fs.append"
	CmdRemove Cmd = "fs.remove"
	CmdMkdir  Cmd = "fs.mkdir"
	CmdStat   Cmd = "fs.stat"
	CmdList   Cmd = "fs.list"
)
