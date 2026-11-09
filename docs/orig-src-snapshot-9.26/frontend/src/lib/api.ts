/**
 * 后端 API 封装层（迁移对照表：mock 数据 → bindings）。
 * 所有后端调用只允许出现在本文件（对应后端 internal/platform 隔离铁律）。
 */
import * as backend from "../../bindings/armory/internal/service/api";
import { Events } from "@wailsio/runtime";

/** 分类（与后端 model.Category JSON 对齐） */
export interface Category {
  id: string;
  name: string;
  icon?: string;
  order: number;
}

/** 工具视图（后端 service.ToolView JSON） */
export interface Tool {
  id: string;
  name: string;
  path: string;
  args?: string;
  type: "app" | "jar" | "py" | "sh" | "bin";
  categoryId: string;
  iconPath?: string;
  envVersion?: string; // java8 | java11 | java17，空=系统默认
  lastRunAt?: string;
  runCount?: number;
  hasIcon: boolean;
}

/** 运行时版本（后端 model.RuntimeVersion）：Java / Python 通用 */
export interface RuntimeVersion {
  name: string;
  path: string;
}

/** 绑定原始值 → Tool 归一化（补默认字段，兼容 model.Tool 与 ToolView 两种返回） */
const asTool = (t: any): Tool => ({ ...t, hasIcon: !!t.hasIcon });

export const api = {
  listCategories: (): Promise<Category[]> => backend.ListCategories().then((v) => v ?? []),
  listTools: (categoryId: string): Promise<Tool[]> =>
    backend.ListTools(categoryId).then((v) => (v ?? []).map(asTool)),
  addByPath: (path: string, categoryId: string, name = "", args = "", envVersion = ""): Promise<Tool> =>
    backend.AddToolByPath(path, categoryId, name, args, envVersion).then(asTool),
  update: (id: string, name: string, path: string, args: string, categoryId: string, envVersion = "") =>
    backend.UpdateTool(id, name, path, args, categoryId, envVersion),
  remove: (id: string) => backend.DeleteTool(id),
  addCategory: (name: string) => backend.AddCategory(name),
  renameCategory: (id: string, name: string) => backend.RenameCategory(id, name),
  deleteCategory: (id: string) => backend.DeleteCategory(id),
  launch: (id: string) => backend.Launch(id),
  getIcon: (id: string): Promise<string> => backend.GetIcon(id),
  /** 设置 */
  getSettings: () => backend.GetSettings(),
  setTheme: (mode: "auto" | "light" | "dark") => backend.SetTheme(mode),
  setReduceMotion: (v: boolean) => backend.SetReduceMotion(v),
  /** 开机自启（SMAppService 登录项；失败会 reject，如 dev 裸二进制运行） */
  setLaunchAtLogin: (v: boolean) => backend.SetLaunchAtLogin(v),
  /** 界面字体缩放：standard | large | xlarge（持久化） */
  setFontScale: (scale: "standard" | "large" | "xlarge") => backend.SetFontScale(scale),
  getAccentColor: (): Promise<string> => backend.GetAccentColor(),
  /** 原生玻璃层深浅同步（深色 tint / 清除），修复 CSS 主题与系统外观脱节 */
  setGlassTheme: (dark: boolean) => backend.SetGlassTheme(dark),
  /** 全局运行环境：Java / Python 分开管理，版本自定义（名称 + 路径） */
  saveJavaVersions: (vs: RuntimeVersion[]) => backend.SaveJavaVersions(vs),
  savePythonVersions: (vs: RuntimeVersion[]) => backend.SavePythonVersions(vs),
  /** 本机扫描（M4-1）：自动发现已装 JDK / Python */
  detectJavaVersions: (): Promise<RuntimeVersion[]> =>
    backend.DetectJavaVersions().then((v) => v ?? []),
  detectPythonVersions: (): Promise<RuntimeVersion[]> =>
    backend.DetectPythonVersions().then((v) => v ?? []),
  /** 最近使用（M4-2）：按最后启动时间倒序 */
  listRecent: (n = 12): Promise<Tool[]> =>
    backend.ListRecentTools(n).then((v) => (v ?? []).map(asTool)),
  /** 工具目录说明文档（M4-3），无则空串 */
  getReadme: (id: string): Promise<string> => backend.GetToolReadme(id),
  /** 备份 / 还原（M4-4）：返回目标路径，取消返回空串 */
  exportBackup: (): Promise<string> => backend.ExportBackup(),
  importBackup: (): Promise<string> => backend.ImportBackup(),

  /** Finder 文件拖入（Go 转发） */
  onFilesDropped: (cb: (paths: string[]) => void) =>
    Events.On("files-dropped", (e: any) => cb(e?.data?.files ?? [])),
  /** 服务层推送提示 */
  onToast: (cb: (msg: string) => void) =>
    Events.On("toast", (e: any) => cb(e?.data ?? "")),
  /** 面板真正出现（⌥Space 呼出）——入场动画只在该时机播放 */
  onPanelShown: (cb: () => void) => Events.On("panel-shown", () => cb()),

  /** 面板显隐（Launchpad 式：Go 侧窗口淡出动画） */
  hideWindow: () => backend.HidePanel(),
};

/** 类型 → 兜底 emoji（真实图标缺失时） */
export const TYPE_EMOJI: Record<Tool["type"], string> = {
  app: "🚀",
  jar: "☕",
  py: "🐍",
  sh: "⌨️",
  bin: "🔧",
};
export const TYPE_LABEL: Record<Tool["type"], string> = {
  app: "App",
  jar: "JAR",
  py: "Python",
  sh: "Shell",
  bin: "二进制",
};
