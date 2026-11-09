// 统一的后端调用封装（composable）。
//
// 设计目标：对外暴露的方法签名与 recovered/frontend-src/src/wails.js 完全一致，
// 组件代码无需感知底层是「官方 bindings」还是「window._wails.invoke 回退」。
//
// 调用优先级：
//   1) 官方 bindings（wails3 generate bindings 产物，位于 frontend/src/bindings/）。
//      该产物由后端 service 包编译后生成；若尚未生成，则动态 import 会失败并被忽略。
//   2) 回退：window._wails.invoke(methodID, args)，methodID 见 recovered/frontend-src/src/wails.js。
//
// 事件订阅：优先 @wailsio/runtime 的 Events.On；回退到 window.runtime?.Events?.On。
import { Events } from '@wailsio/runtime'

// ---------------------------------------------------------------------------
// 方法名 -> 回退 methodID（来自 recovered/frontend-src/src/wails.js 编译产物）
// ---------------------------------------------------------------------------
const METHOD_IDS = {
  listCategories: 3100259903,
  listTools: 3183816730,
  addByPath: 2651014998,
  update: 2712947188,
  remove: 1681267698,
  addCategory: 2804677478,
  renameCategory: 617782669,
  deleteCategory: 1941850880,
  launch: 1789475188,
  getIcon: 1552606880,
  getSettings: 1922004728,
  setTheme: 431449390,
  setReduceMotion: 368741529,
  setLaunchAtLogin: 935936796,
  setAlwaysOnTop: 3781127072,
  setFontScale: 2260736686,
  getAccentColor: 2159151094,
  setGlassTheme: 772031016,
  saveJavaVersions: 94791763,
  savePythonVersions: 3333401133,
  detectJavaVersions: 165915057,
  detectPythonVersions: 3762742563,
  listRecent: 1308117803,
  getReadme: 2907890021,
  exportBackup: 135633011,
  importBackup: 901854628,
  hideWindow: 3214670595,
}

// 前端名 -> Go 导出名（官方 bindings 产物中的 service.API 方法名，见 INTERFACE.md A.3）
const BINDING_NAMES = {
  listCategories: 'ListCategories',
  listTools: 'ListTools',
  addByPath: 'AddToolByPath',
  update: 'UpdateTool',
  remove: 'DeleteTool',
  addCategory: 'AddCategory',
  addSubCategory: 'AddSubCategory',
  reorderCategories: 'ReorderCategories',
  reorderTools: 'ReorderTools',
  renameTag: 'RenameTag',
  deleteTag: 'DeleteTag',
  renameCategory: 'RenameCategory',
  deleteCategory: 'DeleteCategory',
  launch: 'Launch',
  getIcon: 'GetIcon',
  getSettings: 'GetSettings',
  setTheme: 'SetTheme',
  setReduceMotion: 'SetReduceMotion',
  setLaunchAtLogin: 'SetLaunchAtLogin',
  setAlwaysOnTop: 'SetAlwaysOnTop',
  setFontScale: 'SetFontScale',
  getAccentColor: 'GetAccentColor',
  setGlassTheme: 'SetGlassTheme',
  saveJavaVersions: 'SaveJavaVersions',
  savePythonVersions: 'SavePythonVersions',
  detectJavaVersions: 'DetectJavaVersions',
  detectPythonVersions: 'DetectPythonVersions',
  listRecent: 'ListRecentTools',
  getReadme: 'GetToolReadme',
  exportBackup: 'ExportBackup',
  importBackup: 'ImportBackup',
  hideWindow: 'HidePanel',
}

// ---------------------------------------------------------------------------
// 底层调用：直接使用 @wailsio/runtime 的 Call.ByID(methodID, ...args)。
//
// 说明（重要）：Wails v3 beta.26 的正确调用方式是运行时模块导出的
//   Call.ByID(methodID, ...args)
// 它内部走 window._wails 的运行时通道并返回 Promise。
// 之前尝试过的 window._wails.invoke(methodID, args) 是【错误的签名】——
// 该版本 window._wails.invoke 只接受单个消息对象，不接受 (methodID, args)，
// 会导致所有后端调用静默失败（如点「设置」无反应）。此处改为直接、正确的方式。
//
// 我们显式维护前端方法名 -> methodID 映射（与 wails3 generate bindings 产物、
// 以及原版编译产物中的 $Call.ByID(...) 完全一致），确保稳定可离线调用。
// ---------------------------------------------------------------------------
import { Call } from '@wailsio/runtime'

const callById = (methodID, args) => Call.ByID(methodID, ...args)
// 新增方法走按名调用（Name 由后端 FQN 决定，无需预先知道 methodID）。
const callByName = (methodName, args) =>
  Call.ByName(`armory/internal/service.API.${methodName}`, ...args)

// 纯浏览器开发环境（无 Wails 运行时）时的安全兜底：返回 undefined，避免整页崩溃。
function hasRuntime() {
  return typeof window !== 'undefined' && !!window._wails
}

async function invoke(name, args) {
  if (!hasRuntime()) {
    // 非 Wails 环境（纯 vite dev 预览）：返回兜底值。
    return undefined
  }
  const methodID = METHOD_IDS[name]
  if (methodID == null) {
    // 未知方法：走按名调用（用于没有历史 methodID 的新方法，如 addSubCategory/reorderTools/renameTag 等）。
    const goName = BINDING_NAMES[name] || name
    return callByName(goName, args)
  }
  return callById(methodID, args)
}

// 列表项归一化：保证 hasIcon 为布尔（对应原 call(...).map(Tu)）
const normalizeTool = (e) => ({ ...e, hasIcon: !!e.hasIcon })

const api = {
  listCategories: () => invoke('listCategories', []).then((e) => e ?? []),
  listTools: (categoryId) => invoke('listTools', [categoryId]).then((e) => (e ?? []).map(normalizeTool)),
  // 说明：对外保持友好的 (path, name, args, categoryId, envVersion) 顺序；
  // 但后端 AddToolByPath 的真实参数顺序（与原版编译产物一致）是
  // (path, categoryID, name, args, envVersion)，故此处在线路上重排。
  addByPath: (path, name, args = '', jvmArgs = '', categoryId = '', envVersion = '') =>
    invoke('addByPath', [path, categoryId, name, args, jvmArgs, envVersion]).then(normalizeTool),
  update: (id, name, path, args, jvmArgs, tags, term, categoryId, envVersion = '', workingDir = '') =>
    invoke('update', [id, name, path, args, jvmArgs, tags ?? [], !!term, categoryId, envVersion, workingDir]),
  remove: (id) => invoke('remove', [id]),
  addCategory: (name) => invoke('addCategory', [name]),
  addSubCategory: (parentId, name) => invoke('addSubCategory', [parentId, name]),
  reorderCategories: (ids) => invoke('reorderCategories', [ids]),
  reorderTools: (ids) => invoke('reorderTools', [ids]),
  renameCategory: (id, name) => invoke('renameCategory', [id, name]),
  deleteCategory: (id) => invoke('deleteCategory', [id]),
  // 标签管理（跨工具批量）：标签条 chip 右键入口。
  renameTag: (oldName, newName) => invoke('renameTag', [oldName, newName]),
  deleteTag: (name) => invoke('deleteTag', [name]),
  launch: (id) => invoke('launch', [id]),
  getIcon: (id) => invoke('getIcon', [id]),
  getSettings: () => invoke('getSettings', []),
  setTheme: (v) => invoke('setTheme', [v]),
  setReduceMotion: (v) => invoke('setReduceMotion', [v]),
  setLaunchAtLogin: (v) => invoke('setLaunchAtLogin', [v]),
  setAlwaysOnTop: (v) => invoke('setAlwaysOnTop', [v]),
  setFontScale: (v) => invoke('setFontScale', [v]),
  getAccentColor: () => invoke('getAccentColor', []),
  setGlassTheme: (dark) => invoke('setGlassTheme', [dark]),
  saveJavaVersions: (v) => invoke('saveJavaVersions', [v]),
  savePythonVersions: (v) => invoke('savePythonVersions', [v]),
  detectJavaVersions: () => invoke('detectJavaVersions', []).then((e) => e ?? []),
  detectPythonVersions: () => invoke('detectPythonVersions', []).then((e) => e ?? []),
  listRecent: (n = 12) => invoke('listRecent', [n]).then((e) => (e ?? []).map(normalizeTool)),
  getReadme: (id) => invoke('getReadme', [id]),
  exportBackup: () => invoke('exportBackup', []),
  importBackup: () => invoke('importBackup', []),
  hideWindow: () => invoke('hideWindow', []),

  // ---- 事件订阅（后端/运行时 -> 前端回调），返回取消函数 ----
  // 说明：与原始版本一致，后端监听 Wails 原生 WindowFilesDropped 后以自定义
  // 事件 "files-dropped" 转发，payload 形如 { files: string[] }，在此取 e.data.files。
  // 同时兼容 Wails 原生事件 "common:WindowFilesDropped"（payload e.data.filenames）
  // 作为兜底，确保任一管线都能工作。
  onFilesDropped: (cb) => {
    const offCustom = subscribe('files-dropped', (e) => cb(e?.data?.files ?? []))
    const offNative = subscribe('common:WindowFilesDropped', (e) => {
      const files = e?.data?.files ?? e?.data?.filenames ?? []
      if (files.length) cb(files)
    })
    return () => {
      offCustom?.()
      offNative?.()
    }
  },
  onToast: (cb) => subscribe('toast', (e) => cb(e?.data ?? '')),
  onPanelShown: (cb) => subscribe('panel-shown', () => cb()),
}

// 订阅 Wails 事件：优先 @wailsio/runtime 的 Events.On，回退 window.runtime.Events.On。
function subscribe(name, cb) {
  try {
    const off = Events?.On?.(name, cb)
    if (off) return off
  } catch (_) {
    // 忽略，走回退
  }
  // @ts-ignore
  const runtime = typeof window !== 'undefined' ? window.runtime : undefined
  const off = runtime?.Events?.On?.(name, cb)
  return off || (() => {})
}

// 类型标签 / emoji（与原版一致，对应编译产物中的 Du / Eu）
export const typeLabels = {
  app: 'App',
  jar: 'JAR',
  py: 'Python',
  sh: 'Shell',
  bin: '二进制',
  url: 'URL',
}
export const typeEmojis = {
  app: '🚀',
  jar: '☕',
  py: '🐍',
  sh: '⌨️',
  bin: '🔧',
  url: '🔗',
}

export function useApi() {
  return api
}

export { api }
export default useApi
