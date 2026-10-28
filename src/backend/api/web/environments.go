// Environments 绑定: 统一环境变量的树/内容/增删。
package web

import (
	"github.com/wensboy/automan/backend/domain/context"
	domainenvironments "github.com/wensboy/automan/backend/domain/environments"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	serviceenvironments "github.com/wensboy/automan/backend/service/environments"
)

// EnvironmentsService 绑定到前端。
type EnvironmentsService struct {
	ctx context.Context
}

// NewEnvironmentsService 创建绑定。
func NewEnvironmentsService(ctx context.Context) *EnvironmentsService {
	return &EnvironmentsService{ctx: ctx}
}

func (s *EnvironmentsService) service() (*serviceenvironments.Service, error) {
	svc, ok := srvkit.Of(s.ctx, serviceenvironments.Key)
	if !ok {
		return nil, apperr.New(apperr.CodeNotFound)
	}
	return svc, nil
}

// Tree 返回树形路径。
func (s *EnvironmentsService) Tree() ([]domainenvironments.Node, error) {
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	return svc.Tree()
}

// Read 返回单个挂载点内容(原文 + 解析值)。
func (s *EnvironmentsService) Read(path string) (domainenvironments.Document, error) {
	svc, err := s.service()
	if err != nil {
		return domainenvironments.Document{}, err
	}
	return svc.Read(path)
}

// CreateDir 创建分组。
func (s *EnvironmentsService) CreateDir(path string) (domainenvironments.Node, error) {
	svc, err := s.service()
	if err != nil {
		return domainenvironments.Node{}, err
	}
	return svc.CreateDir(path)
}

// CreateFile 通过模板创建挂载点。
func (s *EnvironmentsService) CreateFile(path string, entries []domainenvironments.Entry) (domainenvironments.Document, error) {
	svc, err := s.service()
	if err != nil {
		return domainenvironments.Document{}, err
	}
	return svc.CreateFile(path, entries)
}

// DeleteDir 删除分组(默认仅空目录)。
func (s *EnvironmentsService) DeleteDir(path string, recursive bool) error {
	svc, err := s.service()
	if err != nil {
		return err
	}
	return svc.DeleteDir(path, recursive)
}

// UpdateFile 保存编辑后的内容。
func (s *EnvironmentsService) UpdateFile(path string, content string) (domainenvironments.Document, error) {
	svc, err := s.service()
	if err != nil {
		return domainenvironments.Document{}, err
	}
	return svc.UpdateFile(path, content)
}

// Rename 重命名文件或目录。
func (s *EnvironmentsService) Rename(from string, to string) error {
	svc, err := s.service()
	if err != nil {
		return err
	}
	return svc.Rename(from, to)
}

// DeleteFile 删除挂载点。
func (s *EnvironmentsService) DeleteFile(path string) error {
	svc, err := s.service()
	if err != nil {
		return err
	}
	return svc.DeleteFile(path)
}

// Refresh 失效缓存并重新读取。
func (s *EnvironmentsService) Refresh(path string) (domainenvironments.Document, error) {
	svc, err := s.service()
	if err != nil {
		return domainenvironments.Document{}, err
	}
	return svc.Refresh(path)
}

// Values 返回扁平值(供前端/调试使用)。
func (s *EnvironmentsService) Values(paths []string) (map[string]string, error) {
	svc, err := s.service()
	if err != nil {
		return nil, err
	}
	return svc.Values(paths...)
}
