// 本文件是 store 引擎适配层: 把服务内相对路径转换为 store 指令并收敛返回类型。
package environments

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"

	domainenvironments "github.com/wensboy/automan/backend/domain/environments"
	domainstore "github.com/wensboy/automan/backend/domain/store"
)

// --- store 适配 ---

func (s *Service) do(cmd domainstore.Cmd, args any) (any, error) {
	if s.store == nil {
		return nil, errNoStore
	}
	return s.store.Do(context.Background(), domainstore.Task{Cmd: cmd, Args: args})
}

func (s *Service) fullPath(rel string) string {
	if rel == "" {
		return s.dir
	}
	return path.Join(s.dir, rel)
}

func (s *Service) readFile(rel string) ([]byte, error) {
	value, err := s.do(domainstore.CmdRead, domainstore.ReadArgs{Path: s.fullPath(rel)})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, domainenvironments.ErrNotFound
		}
		return nil, err
	}
	payload, ok := value.([]byte)
	if !ok {
		return nil, fmt.Errorf("environments: unexpected read result")
	}
	return payload, nil
}

func (s *Service) writeFile(rel string, payload []byte) error {
	_, err := s.do(domainstore.CmdWrite, domainstore.WriteArgs{Path: s.fullPath(rel), Data: payload})
	return err
}

func (s *Service) mkdir(rel string) error {
	_, err := s.do(domainstore.CmdMkdir, domainstore.MkdirArgs{Path: s.fullPath(rel)})
	return err
}

func (s *Service) remove(rel string) error {
	_, err := s.do(domainstore.CmdRemove, domainstore.RemoveArgs{Path: s.fullPath(rel)})
	return err
}

func (s *Service) stat(rel string) (domainstore.FileInfo, error) {
	value, err := s.do(domainstore.CmdStat, domainstore.StatArgs{Path: s.fullPath(rel)})
	if err != nil {
		return domainstore.FileInfo{}, err
	}
	info, ok := value.(domainstore.FileInfo)
	if !ok {
		return domainstore.FileInfo{}, fmt.Errorf("environments: unexpected stat result")
	}
	return info, nil
}

func (s *Service) list(rel string) ([]string, error) {
	value, err := s.do(domainstore.CmdList, domainstore.ListArgs{Dir: s.fullPath(rel)})
	if err != nil {
		return nil, err
	}
	names, ok := value.([]string)
	if !ok {
		return nil, fmt.Errorf("environments: unexpected list result")
	}
	return names, nil
}
