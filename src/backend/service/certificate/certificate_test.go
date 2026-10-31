package certificate

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	domaincertificate "github.com/wensboy/automan/backend/domain/certificate"
	domainnotice "github.com/wensboy/automan/backend/domain/notice"
	domainstore "github.com/wensboy/automan/backend/domain/store"
)

// fixedTime 是测试内注入的当前时间, 保证 createdAt/updatedAt 断言稳定。
var fixedTime = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

// fakeStore 按 store 引擎的语义在临时目录上实现所需指令(测试用)。
type fakeStore struct {
	root string
	mu   sync.Mutex
}

func (f *fakeStore) full(rel string) string { return filepath.Join(f.root, filepath.FromSlash(rel)) }

func (f *fakeStore) Submit(task domainstore.Task) <-chan domainstore.Result {
	channel := make(chan domainstore.Result, 1)
	value, err := f.Do(context.Background(), task)
	channel <- domainstore.Result{Value: value, Err: err}
	close(channel)
	return channel
}

func (f *fakeStore) Do(_ context.Context, task domainstore.Task) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch task.Cmd {
	case domainstore.CmdRead:
		return os.ReadFile(f.full(task.Args.(domainstore.ReadArgs).Path))
	case domainstore.CmdWrite:
		args := task.Args.(domainstore.WriteArgs)
		if err := os.MkdirAll(filepath.Dir(f.full(args.Path)), 0o755); err != nil {
			return nil, err
		}
		return nil, os.WriteFile(f.full(args.Path), args.Data, 0o644)
	case domainstore.CmdMkdir:
		return nil, os.MkdirAll(f.full(task.Args.(domainstore.MkdirArgs).Path), 0o755)
	case domainstore.CmdRemove:
		return nil, os.Remove(f.full(task.Args.(domainstore.RemoveArgs).Path))
	default:
		return nil, errors.New("fakeStore: unsupported command")
	}
}

// newService 构造服务: 记录文件写入 fake store, 证书目录用临时目录, 模板取仓库内真实模板。
func newService(t *testing.T) (*Service, string) {
	t.Helper()
	template, err := os.ReadFile("../../../data/default/certificate.tpl.json")
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	return newServiceWith(t, t.TempDir(), t.TempDir(), template)
}

func newServiceWith(t *testing.T, storeRoot, certDir string, template []byte) (*Service, string) {
	t.Helper()
	svc := New().
		WithStore(&fakeStore{root: storeRoot}).
		WithData(fstest.MapFS{"data/default/certificate.tpl.json": &fstest.MapFile{Data: template}})
	svc.certDir = certDir
	svc.now = func() time.Time { return fixedTime }
	if err := svc.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Cleanup(svc.Stop)
	return svc, certDir
}

// pairPEM 生成一对匹配的自签证书与私钥(PKCS8)。
func pairPEM(t *testing.T) ([]byte, []byte, time.Time) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	notAfter := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "example.com"},
		NotBefore:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:     notAfter,
		DNSNames:     []string{"example.com"},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), notAfter
}

// certPEM 只取证书 PEM 与 notAfter(不需要配对私钥的场景)。
func certPEM(t *testing.T) ([]byte, time.Time) {
	t.Helper()
	certificate, _, notAfter := pairPEM(t)
	return certificate, notAfter
}

// keyPEM 生成一把 PKCS8 私钥(与 openssl genpkey 的产物同类型)。
func keyPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func writeFile(t *testing.T, dir, name string, payload []byte) string {
	t.Helper()
	target := filepath.Join(dir, name)
	if err := os.WriteFile(target, payload, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return target
}

func TestSyncBuildsRecords(t *testing.T) {
	svc, dir := newService(t)
	payload, key, notAfter := pairPEM(t)
	writeFile(t, dir, "example.com.crt", payload)
	writeFile(t, dir, "example.com.key", key)
	writeFile(t, dir, "notes.txt", []byte("not a certificate"))
	writeFile(t, dir, ".hidden.crt", payload)

	snapshot, err := svc.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(snapshot.Records) != 1 {
		t.Fatalf("记录数应为 1(私钥不单独成条), got %+v", snapshot.Records)
	}
	if snapshot.SyncedAt != fixedTime.Format(time.RFC3339) {
		t.Fatalf("syncedAt = %q", snapshot.SyncedAt)
	}
	certificate := snapshot.Records[0]
	if certificate.ID != 1 || certificate.Name != "example.com" || certificate.Type != "crt" {
		t.Fatalf("证书记录异常: %+v", certificate)
	}
	if certificate.KeyPath != "example.com.key" {
		t.Fatalf("keyPath = %q", certificate.KeyPath)
	}
	if certificate.Valid != notAfter.Format(time.RFC3339) {
		t.Fatalf("valid = %q, want %q", certificate.Valid, notAfter.Format(time.RFC3339))
	}

	document, err := svc.Read(certificate.ID)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var content map[string]any
	if err := json.Unmarshal([]byte(document.Content), &content); err != nil {
		t.Fatalf("渲染内容不是 JSON: %v", err)
	}
	if content["name"] != "example.com" || content["certPath"] != "example.com.crt" {
		t.Fatalf("渲染内容异常: %+v", content)
	}
}

func TestSyncIsIdempotent(t *testing.T) {
	svc, dir := newService(t)
	payload, _ := certPEM(t)
	writeFile(t, dir, "a.crt", payload)

	first, err := svc.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	second, err := svc.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("无变化时快照应一致:\n%+v\n%+v", first, second)
	}
}

func TestSyncIgnoresOrphanKey(t *testing.T) {
	svc, dir := newService(t)
	writeFile(t, dir, "orphan.key", keyPEM(t))

	snapshot, err := svc.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(snapshot.Records) != 0 {
		t.Fatalf("没有同名证书的私钥不应成条: %+v", snapshot.Records)
	}
}

func TestSyncIgnoresMismatchedKey(t *testing.T) {
	svc, dir := newService(t)
	certificate, _, _ := pairPEM(t)
	_, otherKey, _ := pairPEM(t)
	writeFile(t, dir, "example.com.crt", certificate)
	writeFile(t, dir, "example.com.key", otherKey) // 同名但不是这把证书的私钥

	snapshot, err := svc.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(snapshot.Records) != 1 {
		t.Fatalf("应只有 1 条证书记录: %+v", snapshot.Records)
	}
	if snapshot.Records[0].KeyPath != "" {
		t.Fatalf("不匹配的私钥不应被配对: %+v", snapshot.Records[0])
	}
}

func TestImportKeyPairsWithCertificate(t *testing.T) {
	svc, dir := newService(t)
	certificate, key, _ := pairPEM(t)
	if _, err := svc.Import(writeFile(t, t.TempDir(), "pair.crt", certificate)); err != nil {
		t.Fatalf("import certificate: %v", err)
	}

	document, err := svc.Import(writeFile(t, t.TempDir(), "pair.key", key))
	if err != nil {
		t.Fatalf("import key: %v", err)
	}
	if document.Record.KeyPath != "pair.key" {
		t.Fatalf("私钥应挂到同名证书上: %+v", document.Record)
	}

	// 解不出私钥(纯文本) -> 拒绝
	broken := writeFile(t, t.TempDir(), "broken.key", []byte("private"))
	if _, err := svc.Import(broken); !errors.Is(err, domaincertificate.ErrInvalidCert) {
		t.Fatalf("无法解密的私钥应拒绝, got %v", err)
	}
	// 没有同名证书 -> 拒绝, 且不留文件
	orphan := writeFile(t, t.TempDir(), "orphan.key", keyPEM(t))
	if _, err := svc.Import(orphan); !errors.Is(err, domaincertificate.ErrInvalidCert) {
		t.Fatalf("无同名证书的私钥应拒绝, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "orphan.key")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("拒绝导入不应在证书目录留下文件: %v", err)
	}
	// 同名但私钥与证书不匹配 -> 拒绝
	otherCertificate, _, _ := pairPEM(t)
	_, otherKey, _ := pairPEM(t)
	if _, err := svc.Import(writeFile(t, t.TempDir(), "mismatch.crt", otherCertificate)); err != nil {
		t.Fatalf("import certificate: %v", err)
	}
	mismatched := writeFile(t, t.TempDir(), "mismatch.key", otherKey)
	if _, err := svc.Import(mismatched); !errors.Is(err, domaincertificate.ErrInvalidCert) {
		t.Fatalf("与证书不匹配的私钥应拒绝, got %v", err)
	}
}

func TestImportCopiesSiblingKey(t *testing.T) {
	svc, dir := newService(t)

	// 源目录里同名私钥与证书配对: 一并复制并配对
	certificate, key, _ := pairPEM(t)
	sourceDir := t.TempDir()
	writeFile(t, sourceDir, "pair.crt", certificate)
	writeFile(t, sourceDir, "pair.key", key)
	document, err := svc.Import(filepath.Join(sourceDir, "pair.crt"))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if document.Record.KeyPath != "pair.key" {
		t.Fatalf("同名私钥应一并配对: %+v", document.Record)
	}
	if _, err := os.Stat(filepath.Join(dir, "pair.key")); err != nil {
		t.Fatalf("同名私钥未复制到证书目录: %v", err)
	}

	// 源目录里同名私钥与证书不配对: 不复制、不配对
	otherCertificate, _, _ := pairPEM(t)
	_, foreignKey, _ := pairPEM(t)
	mixedDir := t.TempDir()
	writeFile(t, mixedDir, "mixed.crt", otherCertificate)
	writeFile(t, mixedDir, "mixed.key", foreignKey)
	document, err = svc.Import(filepath.Join(mixedDir, "mixed.crt"))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if document.Record.KeyPath != "" {
		t.Fatalf("不配对的私钥不应被采用: %+v", document.Record)
	}
	if _, err := os.Stat(filepath.Join(dir, "mixed.key")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("不配对的私钥不应被复制: %v", err)
	}

	// 证书目录里已有同名私钥且内容不同: 不覆盖, 也不配对
	conflictCertificate, _, _ := pairPEM(t)
	writeFile(t, dir, "conflict.key", foreignKey)
	conflictDir := t.TempDir()
	writeFile(t, conflictDir, "conflict.crt", conflictCertificate)
	document, err = svc.Import(filepath.Join(conflictDir, "conflict.crt"))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if document.Record.KeyPath != "" {
		t.Fatalf("已存在的不同私钥不应被配对: %+v", document.Record)
	}
	kept, err := os.ReadFile(filepath.Join(dir, "conflict.key"))
	if err != nil || string(kept) != string(foreignKey) {
		t.Fatalf("已存在的私钥不应被覆盖: %v", err)
	}
}

func TestSyncRemovesDeletedFiles(t *testing.T) {
	svc, dir := newService(t)
	payload, _ := certPEM(t)
	source := writeFile(t, dir, "a.crt", payload)
	writeFile(t, dir, "b.crt", payload)

	if _, err := svc.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if err := os.Remove(source); err != nil {
		t.Fatalf("remove: %v", err)
	}
	snapshot, err := svc.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(snapshot.Records) != 1 || snapshot.Records[0].CertPath != "b.crt" {
		t.Fatalf("记录未随文件删除: %+v", snapshot.Records)
	}
}

func TestSyncRenumbersIDs(t *testing.T) {
	storeRoot := t.TempDir()
	certDir := t.TempDir()
	template, err := os.ReadFile("../../../data/default/certificate.tpl.json")
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	svc, _ := newServiceWith(t, storeRoot, certDir, template)

	payload, _, _ := pairPEM(t)
	for _, name := range []string{"a.crt", "b.crt", "c.crt"} {
		writeFile(t, certDir, name, payload)
	}
	first, err := svc.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(first.Records) != 3 || first.Records[2].ID != 3 {
		t.Fatalf("首次同步应得到 1..3: %+v", first.Records)
	}

	// 删掉中间文件: 同步后 id 压缩为 1..2, 顺序保持
	if err := os.Remove(filepath.Join(certDir, "b.crt")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	second, err := svc.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(second.Records) != 2 {
		t.Fatalf("记录数应为 2: %+v", second.Records)
	}
	if second.Records[0].CertPath != "a.crt" || second.Records[0].ID != 1 {
		t.Fatalf("压缩后第一条应为 a.crt/1: %+v", second.Records[0])
	}
	if second.Records[1].CertPath != "c.crt" || second.Records[1].ID != 2 {
		t.Fatalf("压缩后第二条应为 c.crt/2: %+v", second.Records[1])
	}

	// 新增文件追加到尾部, nextId 同步为 n+1
	writeFile(t, certDir, "d.crt", payload)
	third, err := svc.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(third.Records) != 3 || third.Records[2].CertPath != "d.crt" || third.Records[2].ID != 3 {
		t.Fatalf("新文件应追加为 3: %+v", third.Records)
	}
	recordPayload, err := os.ReadFile(filepath.Join(storeRoot, "certs", "certificates.json"))
	if err != nil {
		t.Fatalf("read record file: %v", err)
	}
	var file struct {
		Meta struct {
			NextID int `json:"nextId"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(recordPayload, &file); err != nil {
		t.Fatalf("decode record file: %v", err)
	}
	if file.Meta.NextID != 4 {
		t.Fatalf("meta.nextId 应同步为 4, got %d", file.Meta.NextID)
	}
}

func TestSyncRejectsOverLimit(t *testing.T) {
	svc, dir := newService(t)
	svc.limit = 1
	payload, _ := certPEM(t)
	writeFile(t, dir, "a.crt", payload)
	writeFile(t, dir, "b.crt", payload)

	if _, err := svc.Sync(); !errors.Is(err, domaincertificate.ErrLimit) {
		t.Fatalf("超限应报 ErrLimit, got %v", err)
	}
	if snapshot := svc.List(); len(snapshot.Records) != 0 {
		t.Fatalf("超限时不应写入记录: %+v", snapshot.Records)
	}
}

func TestImportCopiesAndRegisters(t *testing.T) {
	svc, dir := newService(t)
	payload, notAfter := certPEM(t)
	source := writeFile(t, t.TempDir(), "imported.crt", payload)

	document, err := svc.Import(source)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if document.Record.Name != "imported" || document.Record.Type != "crt" {
		t.Fatalf("记录异常: %+v", document.Record)
	}
	if document.Record.Valid != notAfter.Format(time.RFC3339) {
		t.Fatalf("valid = %q", document.Record.Valid)
	}
	if _, err := os.Stat(filepath.Join(dir, "imported.crt")); err != nil {
		t.Fatalf("证书未复制到证书目录: %v", err)
	}

	if _, err := svc.Import(source); !errors.Is(err, domaincertificate.ErrExists) {
		t.Fatalf("重名应报 ErrExists, got %v", err)
	}
	unsupported := writeFile(t, t.TempDir(), "notes.txt", []byte("x"))
	if _, err := svc.Import(unsupported); !errors.Is(err, domaincertificate.ErrInvalidCert) {
		t.Fatalf("非法后缀应报 ErrInvalidCert, got %v", err)
	}
	if _, err := svc.Import("relative.crt"); !errors.Is(err, domaincertificate.ErrInvalidPath) {
		t.Fatalf("相对路径应报 ErrInvalidPath, got %v", err)
	}
}

func TestUpdateOnlyAllowsMetadata(t *testing.T) {
	svc, dir := newService(t)
	payload, _ := certPEM(t)
	writeFile(t, dir, "a.crt", payload)
	snapshot, err := svc.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	id := snapshot.Records[0].ID
	document, err := svc.Read(id)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	updated, err := svc.Update(id, withContent(t, document.Content, map[string]any{"name": "renamed"}))
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Record.Name != "renamed" {
		t.Fatalf("name 未更新: %+v", updated.Record)
	}
	if updated.Record.UpdatedAt != fixedTime.Format(time.RFC3339) {
		t.Fatalf("updatedAt = %q", updated.Record.UpdatedAt)
	}

	// tags 必须真正体现在渲染原文里(否则前端编辑看不到效果)
	tagged, err := svc.Update(id, withContent(t, updated.Content, map[string]any{"tags": []string{"prod", "edge"}}))
	if err != nil {
		t.Fatalf("update tags: %v", err)
	}
	var rendered map[string]any
	if err := json.Unmarshal([]byte(tagged.Content), &rendered); err != nil {
		t.Fatalf("decode content: %v", err)
	}
	tags, ok := rendered["tags"].([]any)
	if !ok || len(tags) != 2 || tags[0] != "prod" || tags[1] != "edge" {
		t.Fatalf("tags 未体现在原文里: %s", tagged.Content)
	}
	if rendered["name"] != "renamed" {
		t.Fatalf("原文 name 应为 updated 后的值: %s", tagged.Content)
	}

	if _, err := svc.Update(id, withContent(t, tagged.Content, map[string]any{"certPath": "other.crt"})); !errors.Is(err, domaincertificate.ErrInvalidFile) {
		t.Fatalf("改只读键应报 ErrInvalidFile, got %v", err)
	}
	if _, err := svc.Update(id, withContent(t, tagged.Content, map[string]any{"unknown": 1})); !errors.Is(err, domaincertificate.ErrInvalidFile) {
		t.Fatalf("未知键应报 ErrInvalidFile, got %v", err)
	}
	if _, err := svc.Update(id, "{"); !errors.Is(err, domaincertificate.ErrInvalidFile) {
		t.Fatalf("非法 JSON 应报 ErrInvalidFile, got %v", err)
	}
}

func TestUpdateKeyPathIsValidated(t *testing.T) {
	svc, dir := newService(t)
	certificate, key, _ := pairPEM(t)
	writeFile(t, dir, "a.crt", certificate)
	writeFile(t, dir, "custom.key", key) // 与本证书匹配, 但不是同名
	_, otherKey, _ := pairPEM(t)
	writeFile(t, dir, "other.key", otherKey) // 同名规则之外的另一把私钥(不匹配)
	snapshot, err := svc.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	id := snapshot.Records[0].ID
	document, err := svc.Read(id)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if document.Record.KeyPath != "" {
		t.Fatalf("没有同名私钥时 keyPath 应为空: %+v", document.Record)
	}

	// 路径不存在 / 内容解不出私钥 -> 拒绝
	if _, err := svc.Update(id, withContent(t, document.Content, map[string]any{"keyPath": "missing.key"})); !errors.Is(err, domaincertificate.ErrInvalidFile) {
		t.Fatalf("不存在的私钥应拒绝, got %v", err)
	}
	if _, err := svc.Update(id, withContent(t, document.Content, map[string]any{"keyPath": "a.crt"})); !errors.Is(err, domaincertificate.ErrInvalidFile) {
		t.Fatalf("非私钥内容应拒绝, got %v", err)
	}

	// 能解密但与证书不匹配 -> 拒绝(不允许把别的证书的私钥挂过来)
	if _, err := svc.Update(id, withContent(t, document.Content, map[string]any{"keyPath": "other.key"})); !errors.Is(err, domaincertificate.ErrInvalidFile) {
		t.Fatalf("与证书不匹配的私钥应拒绝, got %v", err)
	}
	externalOther := writeFile(t, t.TempDir(), "external-other.key", otherKey)
	if _, err := svc.Update(id, withContent(t, document.Content, map[string]any{"keyPath": externalOther})); !errors.Is(err, domaincertificate.ErrInvalidFile) {
		t.Fatalf("不匹配的外部私钥应拒绝, got %v", err)
	}

	// 证书目录内已有匹配私钥: 解密 + 配对校验后直接沿用
	updated, err := svc.Update(id, withContent(t, document.Content, map[string]any{"keyPath": "custom.key"}))
	if err != nil {
		t.Fatalf("update keyPath: %v", err)
	}
	if updated.Record.KeyPath != "custom.key" {
		t.Fatalf("keyPath 未更新: %+v", updated.Record)
	}
	if _, err := svc.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if record := svc.List().Records[0]; record.KeyPath != "custom.key" {
		t.Fatalf("同步不应覆盖手工指定的 keyPath: %+v", record)
	}

	// 任意路径: 解密 + 配对通过后复制到证书目录, 命名为与证书同名的 .key
	external := writeFile(t, t.TempDir(), "elsewhere.key", key)
	updated, err = svc.Update(id, withContent(t, updated.Content, map[string]any{"keyPath": external}))
	if err != nil {
		t.Fatalf("update keyPath from external path: %v", err)
	}
	if updated.Record.KeyPath != "a.key" {
		t.Fatalf("外部私钥应复制为同名 .key: %+v", updated.Record)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.key")); err != nil {
		t.Fatalf("私钥未复制到证书目录: %v", err)
	}

	// 私钥文件消失: 同步清空 keyPath
	for _, name := range []string{"custom.key", "a.key"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatalf("remove %s: %v", name, err)
		}
	}
	if _, err := svc.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if record := svc.List().Records[0]; record.KeyPath != "" {
		t.Fatalf("私钥消失后 keyPath 应清空: %+v", record)
	}

	// 空串: 解除配对
	if _, err := svc.Update(id, withContent(t, updated.Content, map[string]any{"keyPath": ""})); err != nil {
		t.Fatalf("清空 keyPath: %v", err)
	}
	if record := svc.List().Records[0]; record.KeyPath != "" {
		t.Fatalf("keyPath 应被清空: %+v", record)
	}
}

// fakeNotice 记录服务层投递的通知(业务结果由后端发起, 前端不 toast)。
type fakeNotice struct {
	mu      sync.Mutex
	entries []domainnotice.Entry
}

func (f *fakeNotice) Push(entry domainnotice.Entry) (domainnotice.Entry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, entry)
	return entry, nil
}

func (f *fakeNotice) levels() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	levels := make([]string, 0, len(f.entries))
	for _, entry := range f.entries {
		levels = append(levels, entry.Level)
	}
	return levels
}

func TestActionsNotifyFromService(t *testing.T) {
	svc, _ := newService(t)
	notices := &fakeNotice{}
	svc.WithNotice(notices)

	payload, _, _ := pairPEM(t)
	if _, err := svc.Import(writeFile(t, t.TempDir(), "a.crt", payload)); err != nil {
		t.Fatalf("import: %v", err)
	}
	if _, err := svc.Import("relative.crt"); !errors.Is(err, domaincertificate.ErrInvalidPath) {
		t.Fatalf("相对路径应报 ErrInvalidPath, got %v", err)
	}

	levels := notices.levels()
	if len(levels) != 2 || levels[0] != string(domainnotice.LevelSuccess) || levels[1] != string(domainnotice.LevelError) {
		t.Fatalf("成功/失败都应由服务层投递: %+v", levels)
	}
}

func TestExportCopiesToTarget(t *testing.T) {
	svc, dir := newService(t)
	payload, _ := certPEM(t)
	writeFile(t, dir, "a.crt", payload)
	snapshot, err := svc.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	id := snapshot.Records[0].ID

	target := filepath.Join(t.TempDir(), "exported.crt")
	if err := svc.Export(id, target); err != nil {
		t.Fatalf("export: %v", err)
	}
	exported, err := os.ReadFile(target)
	if err != nil || string(exported) != string(payload) {
		t.Fatalf("导出内容不一致: %v", err)
	}
	if err := svc.Export(id, target); !errors.Is(err, domaincertificate.ErrExists) {
		t.Fatalf("目标已存在应报 ErrExists, got %v", err)
	}
	if err := svc.Export(id, "relative.crt"); !errors.Is(err, domaincertificate.ErrInvalidPath) {
		t.Fatalf("相对路径应报 ErrInvalidPath, got %v", err)
	}
}

func TestDeleteBoundary(t *testing.T) {
	svc, dir := newService(t)
	payload, _ := certPEM(t)
	target := writeFile(t, dir, "a.crt", payload)
	snapshot, err := svc.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	id := snapshot.Records[0].ID

	// 只删映射: 文件保留, 下一次同步会重新加回(语义如此)
	if err := svc.Delete(id, false); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("只删映射不应删除文件: %v", err)
	}
	again, err := svc.Sync()
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(again.Records) != 1 {
		t.Fatalf("文件仍在时应重新登记: %+v", again.Records)
	}

	// 连文件一起删
	if err := svc.Delete(again.Records[0].ID, true); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("应删除证书文件: %v", err)
	}
	if len(svc.List().Records) != 0 {
		t.Fatalf("记录应为空: %+v", svc.List().Records)
	}
}

func TestCorruptRecordFileIsRebuilt(t *testing.T) {
	storeRoot := t.TempDir()
	certDir := t.TempDir()
	template, err := os.ReadFile("../../../data/default/certificate.tpl.json")
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	payload, _ := certPEM(t)
	writeFile(t, certDir, "a.crt", payload)

	first, _ := newServiceWith(t, storeRoot, certDir, template)
	if _, err := first.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	first.Stop()

	recordPath := filepath.Join(storeRoot, "certs", "certificates.json")
	if err := os.WriteFile(recordPath, []byte("{broken"), 0o644); err != nil {
		t.Fatalf("corrupt: %v", err)
	}

	second, _ := newServiceWith(t, storeRoot, certDir, template)
	snapshot := second.List()
	if len(snapshot.Records) != 1 || snapshot.Records[0].CertPath != "a.crt" {
		t.Fatalf("损坏后应重建记录: %+v", snapshot.Records)
	}
	if _, err := os.Stat(recordPath + ".bak"); err != nil {
		t.Fatalf("应保留备份文件: %v", err)
	}
}

// withContent 把渲染原文改成 map 后覆盖给定键, 返回新的 JSON 文本。
func withContent(t *testing.T, content string, patch map[string]any) string {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(content), &document); err != nil {
		t.Fatalf("decode content: %v", err)
	}
	for key, value := range patch {
		document[key] = value
	}
	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode content: %v", err)
	}
	return string(payload)
}
