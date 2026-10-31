import { Dialogs, Events } from "@wailsio/runtime"

import {
  Delete,
  Export,
  Import,
  List,
  Read,
  Sync,
  Update,
} from "../../bindings/github.com/wensboy/automan/backend/api/web/certificateservice"
import type {
  Document,
  Record,
  Snapshot,
} from "../../bindings/github.com/wensboy/automan/backend/domain/certificate/models"
import { staticValue } from "@/components/custom/share/provider/static-provider"

/**
 * 证书域 API 适配(见 docs/design/v27.md):
 *   · 列表/详情/同步/导入/编辑/导出/删除全部走后端 certificate 服务;
 *   · 证书目录是唯一事实来源, 后端在同步后广播 `certificate:changed`, 这里只做薄封装。
 */
export type { Document, Record, Snapshot }

/** 拉取全量记录快照(ID 升序)。 */
export async function loadCertificates(): Promise<Snapshot> {
  return await List()
}

/** 读取单条记录详情(记录 + 模板渲染原文)。 */
export async function readCertificate(id: number): Promise<Document> {
  return await Read(id)
}

/** 按证书目录立即校正记录。 */
export async function syncCertificates(): Promise<Snapshot> {
  return await Sync()
}

/** 保存编辑后的元数据(只允许 name/tags/valid)。 */
export async function saveCertificate(id: number, content: string): Promise<Document> {
  return await Update(id, content)
}

/** 删除记录(removeFile=true 时连同证书文件)。 */
export async function deleteCertificate(id: number, removeFile: boolean): Promise<void> {
  await Delete(id, removeFile)
}

/** 导出证书到 target(绝对路径, 可重命名)。 */
export async function exportCertificate(id: number, target: string): Promise<void> {
  await Export(id, target)
}

/** 选择本地证书文件(原生对话框); 取消返回空串。 */
export async function pickCertificateFile(): Promise<string> {
  return await Dialogs.OpenFile({
    Title: staticValue("contentArea.home.certificates.importTitle") ?? "",
    CanChooseFiles: true,
    AllowsMultipleSelection: false,
    Filters: [
      {
        DisplayName: staticValue("contentArea.home.certificates.filterName") ?? "",
        Pattern: "*.crt;*.pem;*.key",
      },
    ],
  })
}

/** 选择导出位置(原生对话框); 取消返回空串。 */
export async function pickExportPath(record: Record): Promise<string> {
  return await Dialogs.SaveFile({
    Title: staticValue("contentArea.home.certificates.exportTitle") ?? "",
    Filename: `${record.name}.${record.type}`,
    CanChooseFiles: true,
  })
}

/** 导入本地证书文件(复制进证书目录并登记)。 */
export async function importCertificate(path: string): Promise<Document> {
  return await Import(path)
}

/** 订阅记录变化(同步/导入/编辑/删除), 返回退订函数。 */
export function subscribeCertificates(listener: () => void): () => void {
  return Events.On("certificate:changed", () => listener())
}
