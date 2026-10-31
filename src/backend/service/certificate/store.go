// 本文件是 store 引擎适配层: 记录文件读写(证书目录不在此层, 见 dirfs.go)。
package certificate

import (
	"context"
	"errors"
	"fmt"
	"os"

	domaincertificate "github.com/wensboy/automan/backend/domain/certificate"
	domainstore "github.com/wensboy/automan/backend/domain/store"
)

func (s *Service) do(cmd domainstore.Cmd, args any) (any, error) {
	if s.store == nil {
		return nil, errNoStore
	}
	return s.store.Do(context.Background(), domainstore.Task{Cmd: cmd, Args: args})
}

// readRecordFile 读取相对 store 根的文件; 不存在收敛为 ErrNotFound。
func (s *Service) readRecordFile(rel string) ([]byte, error) {
	value, err := s.do(domainstore.CmdRead, domainstore.ReadArgs{Path: rel})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, domaincertificate.ErrNotFound
		}
		return nil, err
	}
	payload, ok := value.([]byte)
	if !ok {
		return nil, errors.New("certificate: unexpected read result")
	}
	return payload, nil
}

// writeRecordFile 原子写相对 store 根的文件(临时文件 + rename 由引擎保证)。
func (s *Service) writeRecordFile(rel string, payload []byte) error {
	if _, err := s.do(domainstore.CmdWrite, domainstore.WriteArgs{Path: rel, Data: payload}); err != nil {
		return fmt.Errorf("certificate: write records: %w", err)
	}
	return nil
}
