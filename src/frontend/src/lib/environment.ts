/**
 * Environment 文件解析(见 docs/design/v25.md §21)。
 *
 * 结构对齐 data/default/env.tpl.json:
 *   { meta: { path, entry }, <entry>: [ { type, key, value } ] }
 *
 * 约定:
 *   · 所有 key 都是**顶层 key**(不做嵌套);
 *   · value 一律以 string 展示, 依据 type(int|float|string|array|object) 解析;
 *   · entry 取 meta.entry 指向的字段; 该字段缺失/不是数组/条目非法 => 整体判为错误,
 *     由 list 视图显示 empty error(便于定位问题文件)。
 */

import {
  CreateDir,
  CreateFile,
  DeleteDir,
  DeleteFile,
  Read,
  Refresh,
  Rename,
  Tree,
  UpdateFile,
} from "../../bindings/github.com/wensboy/automan/backend/api/web/environmentsservice"

export type EnvNode = {
  id: string
  name: string
  type: "dir" | "file"
  children?: EnvNode[]
}

export type EnvEntryRow = { type: string; key: string; value: string }

export type EnvDocument = {
  path: string
  name: string
  content: string
  entry: string
  entries: EnvEntryRow[]
}

function toNode(raw: { name?: unknown; path?: unknown; type?: unknown; children?: unknown }): EnvNode {
  const children = Array.isArray(raw.children) ? raw.children : []
  return {
    id: String(raw.path ?? ""),
    name: String(raw.name ?? ""),
    type: String(raw.type) === "dir" ? "dir" : "file",
    children: children.length > 0 ? children.map((child) => toNode(child as unknown as Record<string, unknown>)) : undefined,
  }
}

export async function loadEnvTree(): Promise<EnvNode[]> {
  const nodes = (await Tree()) ?? []
  return nodes.map((node) => toNode(node as unknown as Record<string, unknown>))
}

export async function readEnvFile(path: string): Promise<EnvDocument> {
  const doc = await Read(path)
  return {
    path: String(doc?.path ?? path),
    name: String(doc?.name ?? ""),
    content: String(doc?.content ?? ""),
    entry: String(doc?.entry ?? ""),
    entries: (doc?.entries ?? []).map((item) => ({
      type: String(item?.type ?? ""),
      key: String(item?.key ?? ""),
      value: String(item?.value ?? ""),
    })),
  }
}

export async function createEnvDir(path: string): Promise<void> {
  await CreateDir(path)
}

export async function createEnvFile(path: string): Promise<void> {
  await CreateFile(path, null)
}

export async function deleteEnvDir(path: string): Promise<void> {
  await DeleteDir(path, true)
}

export async function deleteEnvFile(path: string): Promise<void> {
  await DeleteFile(path)
}

export async function renameEnvPath(from: string, to: string): Promise<void> {
  await Rename(from, to)
}

export async function refreshEnvFile(path: string): Promise<EnvDocument> {
  const doc = await Refresh(path)
  return {
    path: String(doc?.path ?? path),
    name: String(doc?.name ?? ""),
    content: String(doc?.content ?? ""),
    entry: String(doc?.entry ?? ""),
    entries: (doc?.entries ?? []).map((item) => ({
      type: String(item?.type ?? ""),
      key: String(item?.key ?? ""),
      value: String(item?.value ?? ""),
    })),
  }
}

export async function saveEnvFile(path: string, content: string): Promise<EnvDocument> {
  const doc = await UpdateFile(path, content)
  return {
    path: String(doc?.path ?? path),
    name: String(doc?.name ?? ""),
    content: String(doc?.content ?? ""),
    entry: String(doc?.entry ?? ""),
    entries: (doc?.entries ?? []).map((item) => ({
      type: String(item?.type ?? ""),
      key: String(item?.key ?? ""),
      value: String(item?.value ?? ""),
    })),
  }
}

/** 展示用: 把(可能压缩的)JSON 文本格式化为 4 空格缩进; 解析失败原样返回。 */
export function prettyJSON(text: string): string {
  try {
    return JSON.stringify(JSON.parse(text), null, 4)
  } catch {
    return text
  }
}
