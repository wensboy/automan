// 本文件负责记录文件的编解码与落盘(格式见 docs/design/v27.md §2)。
package certificate

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	domaincertificate "github.com/wensboy/automan/backend/domain/certificate"
	domainlog "github.com/wensboy/automan/backend/domain/log"
	"github.com/wensboy/automan/backend/internal/kits/logkit"
)

// recordFile 是记录文件的磁盘形态。
type recordFile struct {
	Meta struct {
		Version  int    `json:"version"`
		SyncedAt string `json:"syncedAt"`
		NextID   int    `json:"nextId"`
	} `json:"meta"`
	Certificates []domaincertificate.Record `json:"certificates"`
}

// recordPath 返回记录文件相对 store 根的路径。
func (s *Service) recordPath() string {
	return path.Join(s.root, s.file)
}

// load 读入记录文件: 不存在视为空; 损坏时备份后重建(由随后的同步按证书目录补全)。
func (s *Service) load() error {
	payload, err := s.readRecordFile(s.recordPath())
	if errors.Is(err, domaincertificate.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	var file recordFile
	if err := json.Unmarshal(payload, &file); err != nil {
		return s.rebuild(payload, err)
	}
	records, err := normalizeRecords(file.Certificates)
	if err != nil {
		return s.rebuild(payload, err)
	}

	s.mu.Lock()
	s.records = records
	s.syncedAt = file.Meta.SyncedAt
	s.nextID = max(file.Meta.NextID, maxRecordID(records)+1, 1)
	s.mu.Unlock()
	return nil
}

// rebuild 备份损坏的记录文件并清空内存记录, 由随后的同步重建。
func (s *Service) rebuild(payload []byte, cause error) error {
	backup := s.recordPath() + ".bak"
	if err := s.writeRecordFile(backup, payload); err != nil {
		return err
	}
	s.mu.Lock()
	s.records = []domaincertificate.Record{}
	s.nextID = 1
	s.syncedAt = ""
	s.mu.Unlock()
	s.log(domainlog.LevelWarn, "certificate: record file rebuilt", logkit.F("err", cause), logkit.F("backup", backup))
	return nil
}

func (s *Service) persist(records []domaincertificate.Record, syncedAt string, nextID int) error {
	file := recordFile{Certificates: records}
	file.Meta.Version = 1
	file.Meta.SyncedAt = syncedAt
	file.Meta.NextID = nextID

	payload, err := json.MarshalIndent(file, "", "    ")
	if err != nil {
		return fmt.Errorf("certificate: encode records: %w", err)
	}
	return s.writeRecordFile(s.recordPath(), append(payload, '\n'))
}

// normalizeRecords 校验磁盘记录: id 唯一且为正、类型合法、certPath 为纯文件名。
func normalizeRecords(items []domaincertificate.Record) ([]domaincertificate.Record, error) {
	records := make([]domaincertificate.Record, 0, len(items))
	seen := make(map[int]bool, len(items))
	for _, item := range items {
		if item.ID <= 0 || seen[item.ID] {
			return nil, fmt.Errorf("%w: bad id %d", domaincertificate.ErrInvalidFile, item.ID)
		}
		seen[item.ID] = true
		if !validType(item.Type) {
			return nil, fmt.Errorf("%w: bad type %q", domaincertificate.ErrInvalidFile, item.Type)
		}
		if strings.TrimSpace(item.CertPath) == "" || strings.ContainsAny(item.CertPath, `/\`) {
			return nil, fmt.Errorf("%w: bad certPath %q", domaincertificate.ErrInvalidFile, item.CertPath)
		}
		if item.Tags == nil {
			item.Tags = []string{}
		}
		records = append(records, item)
	}
	sortRecords(records)
	return records, nil
}

func maxRecordID(records []domaincertificate.Record) int {
	largest := 0
	for _, record := range records {
		largest = max(largest, record.ID)
	}
	return largest
}
