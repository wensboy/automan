// Certificate 绑定: 全局证书记录的同步/导入/编辑/导出/删除。
package web

import (
	domaincertificate "github.com/wensboy/automan/backend/domain/certificate"
	"github.com/wensboy/automan/backend/domain/context"
	apperr "github.com/wensboy/automan/backend/internal/err"
	"github.com/wensboy/automan/backend/internal/kits/srvkit"
	servicecertificate "github.com/wensboy/automan/backend/service/certificate"
)

// CertificateService 绑定到前端。
type CertificateService struct {
	ctx context.Context
}

// NewCertificateService 创建绑定。
func NewCertificateService(ctx context.Context) *CertificateService {
	return &CertificateService{ctx: ctx}
}

func (s *CertificateService) service() (*servicecertificate.Service, error) {
	svc, ok := srvkit.Of(s.ctx, servicecertificate.Key)
	if !ok {
		return nil, apperr.New(apperr.CodeNotFound)
	}
	return svc, nil
}

// List 返回证书目录对应的记录快照(ID 升序)。
func (s *CertificateService) List() (domaincertificate.Snapshot, error) {
	svc, err := s.service()
	if err != nil {
		return domaincertificate.Snapshot{}, err
	}
	return svc.List(), nil
}

// Read 返回单条记录的详情(记录 + 模板渲染原文)。
func (s *CertificateService) Read(id int) (domaincertificate.Document, error) {
	svc, err := s.service()
	if err != nil {
		return domaincertificate.Document{}, err
	}
	return svc.Read(id)
}

// Sync 按证书目录立即校正记录。
func (s *CertificateService) Sync() (domaincertificate.Snapshot, error) {
	svc, err := s.service()
	if err != nil {
		return domaincertificate.Snapshot{}, err
	}
	return svc.Sync()
}

// Import 把本地证书文件复制进证书目录并登记。
func (s *CertificateService) Import(path string) (domaincertificate.Document, error) {
	svc, err := s.service()
	if err != nil {
		return domaincertificate.Document{}, err
	}
	return svc.Import(path)
}

// Update 保存编辑后的元数据(只允许 name/tags/valid)。
func (s *CertificateService) Update(id int, content string) (domaincertificate.Document, error) {
	svc, err := s.service()
	if err != nil {
		return domaincertificate.Document{}, err
	}
	return svc.Update(id, content)
}

// Export 把指定证书复制到目标路径。
func (s *CertificateService) Export(id int, target string) error {
	svc, err := s.service()
	if err != nil {
		return err
	}
	return svc.Export(id, target)
}

// Delete 删除记录(removeFile=true 时连同证书文件)。
func (s *CertificateService) Delete(id int, removeFile bool) error {
	svc, err := s.service()
	if err != nil {
		return err
	}
	return svc.Delete(id, removeFile)
}
