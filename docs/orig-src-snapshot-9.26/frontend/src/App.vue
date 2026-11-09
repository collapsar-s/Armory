<script setup lang="ts">
/**
 * Armory 主面板（M2）。
 * 迁移基准：ui-prototype/index.html；动画/主题常量已抽离至 lib/motion.ts、lib/theme.ts。
 * 数据源：后端 bindings（src/lib/api.ts 是唯一后端调用入口）。
 */
import { computed, nextTick, onMounted, reactive, ref, watch } from "vue";
import { api, TYPE_EMOJI, TYPE_LABEL, type Category, type Tool } from "./lib/api";
import { initMotion, setUserReducedMotion } from "./lib/motion";
import { applyFontScale, applyTheme, initTheme, type FontScale, type ThemeMode } from "./lib/theme";
import { mdToHtml } from "./lib/md";

/* ---------------- 状态 ---------------- */
const categories = ref<Category[]>([]);
const tools = ref<Tool[]>([]);
const icons = reactive<Record<string, string>>({});
const activeCat = ref("all");
const search = ref("");
const mode = ref<"grid" | "cmd">("grid");
const cmdIndex = ref(0);
const settingsOpen = ref(false);
const settingsClosing = ref(false);
const addOpen = ref(false);
const addClosing = ref(false);
const addForm = reactive({ name: "", path: "", args: "", categoryId: "" });
const addUseGlobal = ref(false);   // 「是否采用全局设置」开关
const addEnvVersion = ref("");     // 勾选后选定的全局版本名
/** 依据路径后缀推断类型（仅用于弹窗交互：决定显示 Java 还是 Python 版本选项） */
const addType = computed<"jar" | "py" | "bin">(() => {
  const p = addForm.path.trim().toLowerCase();
  if (p.endsWith(".jar")) return "jar";
  if (p.endsWith(".py")) return "py";
  return "bin";
});
const dragging = ref(false);
const appEl = ref<HTMLElement>();
const toastMsg = ref("");
const toastVisible = ref(false);

// 设置（M2-c）
const themeMode = ref<ThemeMode>("auto");
const reduceMotion = ref(false);
const launchOnLogin = ref(false);

// 弹层：'edit' 编辑工具 | 'add' 添加工具 | 'catadd' 新建分类 | 'catedit' 分类管理
/* 工具编辑：居中模态（与设置/添加/分类弹窗统一，M3 弹窗位置规范） */
const editOpen = ref(false);
const editClosing = ref(false);
const editId = ref(""); const editIcon = ref(""); // icon=工具类型（jar/py 版本下拉判断）
const popName = ref(""); const popPath = ref(""); const popArgs = ref("");
const editReadme = ref("");          // 工具说明文档原文（M4-3）
const renderedReadme = ref("");      // 渲染后的 HTML
const popEnv = ref(""); // jar/py 工具的运行时版本名，空=系统默认
const popCat = ref("");

/* 分类管理 / 新建分类：居中模态（与【设置】同体系，M3 视觉统一） */
const catOpen = ref(false);
const catClosing = ref(false);
const catMode = ref<"catadd" | "catedit">("catadd");
const catId = ref(""); const catOrigName = ref(""); const catNewName = ref("");

let toastTimer: ReturnType<typeof setTimeout> | undefined;

const VIRTUAL_ALL: Category = { id: "all", name: "全部", order: -1 };
const VIRTUAL_RECENT: Category = { id: "recent", name: "最近", order: -0.5 };
const recentTools = ref<Tool[]>([]);
const railCats = computed(() => [
  VIRTUAL_ALL,
  ...(recentTools.value.length ? [VIRTUAL_RECENT] : []), // 有启动记录才显示「最近」
  ...categories.value,
]);
const activeCatName = computed(() =>
  railCats.value.find((c) => c.id === activeCat.value)?.name ?? "");

/* ---------------- 数据加载 ---------------- */
async function loadCats() {
  categories.value = await api.listCategories();
}
async function loadTools() {
  if (mode.value !== "cmd" && activeCat.value === "recent") {
    tools.value = recentTools.value; // 最近使用（按启动时间倒序）
  } else {
    tools.value = await api.listTools(mode.value === "cmd" ? "all" : activeCat.value);
  }
  for (const t of tools.value) {
    if (t.hasIcon && !icons[t.id]) {
      api.getIcon(t.id).then((b64) => { if (b64) icons[t.id] = "data:image/png;base64," + b64; });
    }
  }
}
async function refresh() {
  recentTools.value = await api.listRecent(12);
  await Promise.all([loadCats(), loadTools()]);
}

/* ---------------- 入场动画（M2-2：呼出"浮起"） ---------------- */
function replayEnter() {
  const el = appEl.value;
  if (!el) return;
  el.classList.remove("entering");
  void el.offsetWidth; // 强制 reflow 以重启动画
  el.classList.add("entering");
}

onMounted(async () => {
  initMotion();
  themeMode.value = await initTheme();
  const s = await api.getSettings();
  reduceMotion.value = !!s.reduceMotion;
  launchOnLogin.value = !!s.launchOnLogin;
  setUserReducedMotion(reduceMotion.value);
  await refresh();
  replayEnter();

  window.addEventListener("keydown", onGlobalKey);
  // 入场动画时机：仅应用启动首次 + 每次 ⌥Space 真正呼出（panel-shown 事件）。
  // 不挂在 window focus 上——面板常驻后每次切回都会触发，弹跳只会造成干扰。
  api.onPanelShown(() => { replayEnter(); nextTick(() => searchEl.value?.focus()); });
  api.onFilesDropped(onFilesDropped);
  api.onToast(showToast);
  // 拖拽状态探测（drop 本体由 Wails 原生通道处理）
  document.addEventListener("dragenter", onDragEnter);
  document.addEventListener("dragleave", onDragLeave);
  document.addEventListener("dragover", (e) => e.preventDefault());
  document.addEventListener("drop", onDragEnd);
  document.addEventListener("dragend", onDragEnd);
});
let dragDepth = 0;
function onDragEnter() { dragDepth++; dragging.value = true; }
function onDragLeave() { dragDepth = Math.max(0, dragDepth - 1); if (!dragDepth) dragging.value = false; }
function onDragEnd() { dragDepth = 0; dragging.value = false; }

/* ---------------- 设置抽屉（M2-c） ---------------- */
/* ---------------- 设置弹窗（居中模态） ---------------- */
function openSettings() {
  settingsClosing.value = false;
  settingsOpen.value = true;
  loadEnvConfig(); // 每次打开拉取最新，避免展示陈旧值
}
function closeSettings() {
  if (!settingsOpen.value || settingsClosing.value) return;
  settingsClosing.value = true; // 播放退出动画后再卸载
  setTimeout(() => { settingsOpen.value = false; settingsClosing.value = false; }, 180);
}

/* ---------------- 添加工具弹窗（对标 SpearX 表单布局） ---------------- */
function openAddDialog() {
  addForm.name = ""; addForm.path = ""; addForm.args = "";
  addForm.categoryId = activeCat.value === "all" ? (categories.value[0]?.id ?? "") : activeCat.value;
  addUseGlobal.value = false; addEnvVersion.value = "";
  if (!javaVersions.value.length && !pythonVersions.value.length) loadEnvConfig(); // 版本下拉数据源
  addClosing.value = false;
  addOpen.value = true;
}
function closeAddDialog() {
  if (!addOpen.value || addClosing.value) return;
  addClosing.value = true;
  setTimeout(() => { addOpen.value = false; addClosing.value = false; }, 180);
}
async function submitAdd() {
  if (!addForm.path.trim()) { showToast("路径不能为空"); return; }
  // 勾选全局但没配置可选版本 → 提示先去设置配置
  if (addUseGlobal.value) {
    const list = addType.value === "jar" ? javaVersions.value : pythonVersions.value;
    if (!list.length) { showToast("请先在「设置 → 运行环境」中配置版本"); return; }
    if (!addEnvVersion.value) { showToast("请选择一个全局版本"); return; }
  }
  try {
    const env = addUseGlobal.value ? addEnvVersion.value : "";
    const t = await api.addByPath(addForm.path.trim(), addForm.categoryId, addForm.name.trim(), addForm.args.trim(), env);
    addOpen.value = false;
    await refresh();
    showToast(`已收录 ${t.name}`);
  } catch (err: any) { showToast(err?.message ?? String(err)); }
}

async function onThemePick(m: ThemeMode) {
  themeMode.value = m;
  applyTheme(m);
  await api.setTheme(m);
}

/* ---------------- 全局运行环境（Java / Python 分开管理，版本自定义） ---------------- */
interface RuntimeRow { name: string; path: string; }
const envJava = ref<RuntimeRow[]>([]);   // 编辑中的 Java 版本列表
const envPy = ref<RuntimeRow[]>([]);     // 编辑中的 Python 版本列表
const javaVersions = ref<RuntimeRow[]>([]);     // 已保存（工具编辑弹窗下拉数据源）
const pythonVersions = ref<RuntimeRow[]>([]);   // 已保存
const envSaving = ref<"" | "java" | "py">("");

async function loadEnvConfig() {
  const s = (await api.getSettings()) as any;
  envJava.value = (s.javaVersions ?? []).map((v: any) => ({ name: v.name ?? "", path: v.path ?? "" }));
  envPy.value = (s.pythonVersions ?? []).map((v: any) => ({ name: v.name ?? "", path: v.path ?? "" }));
  javaVersions.value = envJava.value.filter((v) => v.name);
  pythonVersions.value = envPy.value.filter((v) => v.name);
  const fs = (s as any).fontScale;
  fontScale.value = fs === "standard" || fs === "xlarge" ? fs : "large";
}
function addEnvRow(list: RuntimeRow[]) { list.push({ name: "", path: "" }); }
function delEnvRow(list: RuntimeRow[], i: number) { list.splice(i, 1); }

async function saveEnv(kind: "java" | "py") {
  const list = kind === "java" ? envJava : envPy;
  const missingName = list.value.some((v) => !v.name.trim() && v.path.trim());
  if (missingName) { showToast(kind === "java" ? "Java" : "Python" + "有配置行未填版本名"); return; }
  envSaving.value = kind;
  try {
    if (kind === "java") await api.saveJavaVersions(list.value.map((v) => ({ name: v.name.trim(), path: v.path.trim() })));
    else await api.savePythonVersions(list.value.map((v) => ({ name: v.name.trim(), path: v.path.trim() })));
    await loadEnvConfig();
    showToast(kind === "java" ? "Java 环境已保存，立即生效" : "Python 环境已保存，立即生效");
  } catch (err: any) { showToast(err?.message ?? String(err)); }
  finally { envSaving.value = ""; }
}
async function onReducePick(v: boolean) {
  reduceMotion.value = v;
  setUserReducedMotion(v);
  await api.setReduceMotion(v);
  showToast(v ? "已减弱动态效果" : "已恢复完整动画");
}

/* 开机自启（M3-1）：先乐观更新，注册失败回滚并提示具体原因 */
async function onLoginPick(v: boolean) {
  if (launchOnLogin.value === v) return;
  launchOnLogin.value = v;
  try {
    await api.setLaunchAtLogin(v);
    showToast(v ? "已开启开机自启（登录时自动启动）" : "已关闭开机自启");
  } catch (err: any) {
    launchOnLogin.value = !v; // 注册失败回滚
    showToast(err?.message ?? String(err));
  }
}

/* ---------------- 备份 / 还原（M4-4） ---------------- */
const importArmed = ref(false); // 两段式确认（防覆盖当前库）

async function doExport() {
  try {
    const p = await api.exportBackup();
    if (p) showToast(`已导出到 ${p}`);
  } catch (err: any) { showToast(err?.message ?? String(err)); }
}
async function doImport() {
  if (!importArmed.value) {  // 第一次点：进入确认态，3 秒不点自动恢复
    importArmed.value = true;
    setTimeout(() => { importArmed.value = false; }, 3000);
    return;
  }
  importArmed.value = false;
  try {
    const p = await api.importBackup();
    if (!p) return; // 用户取消
    closeSettings();
    // 还原可能改变了主题/字体等设置：全部重新应用
    themeMode.value = await initTheme();
    const s = (await api.getSettings()) as any;
    reduceMotion.value = !!s.reduceMotion; setUserReducedMotion(reduceMotion.value);
    applyFontScale(s.fontScale);
    launchOnLogin.value = !!s.launchOnLogin;
    await refresh();
    showToast("已从备份还原");
  } catch (err: any) { showToast(err?.message ?? String(err)); }
}

/* ---------------- 本机运行时扫描（M4-1） ---------------- */
function mergeEnvRows(list: RuntimeRow[], found: RuntimeVersion[]) {
  let added = 0;
  for (const f of found) {
    if (list.some((r) => r.path === f.path)) continue; // 按路径去重
    list.push({ name: f.name, path: f.path });
    added++;
  }
  return added;
}
async function scanJava() {
  try {
    const found = await api.detectJavaVersions();
    const n = mergeEnvRows(envJava, found);
    showToast(n ? `发现 ${n} 个 JDK，已填入列表（记得保存）` : "未在本机发现新的 JDK");
  } catch (err: any) { showToast(err?.message ?? String(err)); }
}
async function scanPython() {
  try {
    const found = await api.detectPythonVersions();
    const n = mergeEnvRows(envPy, found);
    showToast(n ? `发现 ${n} 个 Python，已填入列表（记得保存）` : "未在本机发现新的 Python");
  } catch (err: any) { showToast(err?.message ?? String(err)); }
}

/* ---------------- 界面字体缩放 ---------------- */
const fontScale = ref<FontScale>("large");
async function onFontScalePick(v: FontScale) {
  fontScale.value = v;
  applyFontScale(v);
  await api.setFontScale(v);
}

/* ---------------- 分类切换 ---------------- */
async function switchCat(id: string) {
  activeCat.value = id;
  if (mode.value === "cmd") { search.value = ""; mode.value = "grid"; }
  await loadTools();
}

/* ---------------- 搜索 / 命令面板 ---------------- */
const cmdResults = computed(() => {
  const q = search.value.trim().toLowerCase();
  if (!q) return tools.value;
  return tools.value.filter(
    (t) => t.name.toLowerCase().includes(q) || pathIncludes(t, q));
});
function pathIncludes(t: Tool, q: string) {
  return t.path.toLowerCase().includes(q) || (TYPE_LABEL[t.type] || "").toLowerCase().includes(q);
}
function hl(name: string): string {
  const q = search.value.trim().toLowerCase();
  if (!q) return name;
  const i = name.toLowerCase().indexOf(q);
  if (i < 0) return name;
  return `${name.slice(0, i)}<mark>${name.slice(i, i + q.length)}</mark>${name.slice(i + q.length)}`;
}
function onSearchInput() {
  if (search.value.trim() && mode.value === "grid") { mode.value = "cmd"; }
  if (!search.value.trim() && mode.value === "cmd") { mode.value = "grid"; loadTools(); }
  cmdIndex.value = 0;
  if (mode.value === "cmd") loadTools();
}
async function onSearchKey(e: KeyboardEvent) {
  if (mode.value !== "cmd") return;
  if (e.key === "ArrowDown") { e.preventDefault(); cmdIndex.value = Math.min(cmdIndex.value + 1, cmdResults.value.length - 1); scrollCmd(); }
  if (e.key === "ArrowUp") { e.preventDefault(); cmdIndex.value = Math.max(cmdIndex.value - 1, 0); scrollCmd(); }
  if (e.key === "Enter") {
    e.preventDefault();
    const t = cmdResults.value[cmdIndex.value];
    if (t) (e.metaKey ? openEdit(t) : doLaunch(t));
  }
}
function scrollCmd() {
  nextTick(() => document.querySelector(".cmdrow.active")?.scrollIntoView({ block: "nearest" }));
}

/* ---------------- 启动 ---------------- */
async function doLaunch(t: Tool) {
  try {
    await api.launch(t.id);
    showToast(`已启动 ${t.name}`);
    setTimeout(() => api.hideWindow(), 320); // 启动反馈播完再收面板（任务书 §2.5）
    await refresh(); // 静默刷新：更新最近使用排序与启动次数
  } catch (err: any) {
    showToast(`启动失败: ${err?.message ?? err}`);
  }
}

/* ---------------- 文件拖入收录 ---------------- */
async function onFilesDropped(paths: string[]) {
  if (!paths.length) return;
  let ok = 0;
  for (const p of paths) {
    try {
      await api.addByPath(p, activeCat.value === "all" ? "" : activeCat.value, "", "");
      ok++;
    } catch (err: any) { showToast(`收录失败: ${err?.message ?? err}`); }
  }
  if (ok) { await refresh(); showToast(`已收录 ${ok} 个工具`); }
}

/* ---------------- 工具编辑（居中模态） ---------------- */
function openEdit(t: Tool) {
  editId.value = t.id; editIcon.value = t.type;
  popName.value = t.name; popPath.value = t.path; popArgs.value = t.args ?? "";
  popCat.value = t.categoryId; popEnv.value = t.envVersion ?? "";
  editReadme.value = ""; renderedReadme.value = "";
  api.getReadme(t.id).then((md) => {  // 说明文档（M4-3）：加载后渲染
    editReadme.value = md;
    renderedReadme.value = md ? mdToHtml(md) : "";
  });
  if (!javaVersions.value.length && !pythonVersions.value.length) loadEnvConfig(); // 版本下拉数据源
  editClosing.value = false; editOpen.value = true;
}
function closeEditDialog() {
  if (!editOpen.value || editClosing.value) return;
  editClosing.value = true; // 播放退出动画后再卸载
  setTimeout(() => { editOpen.value = false; editClosing.value = false; }, 180);
}
function openCatAdd() {
  catMode.value = "catadd"; catNewName.value = ""; catOrigName.value = ""; catId.value = "";
  catClosing.value = false; catOpen.value = true;
}
function openCatEdit(c: Category) {
  catMode.value = "catedit"; catId.value = c.id; catOrigName.value = c.name; catNewName.value = c.name;
  catClosing.value = false; catOpen.value = true;
}
function closeCatDialog() {
  if (!catOpen.value || catClosing.value) return;
  catClosing.value = true; // 播放退出动画后再卸载
  setTimeout(() => { catOpen.value = false; catClosing.value = false; }, 180);
}
async function saveCat() {
  const name = catNewName.value.trim();
  if (!name) { showToast("分类名不能为空"); return; }
  try {
    if (catMode.value === "catadd") {
      await api.addCategory(name);
      showToast("分类已创建");
      activeCat.value = "all";
    } else if (name !== catOrigName.value) {
      await api.renameCategory(catId.value, name);
      showToast("已保存");
    }
    catOpen.value = false;
    await refresh();
  } catch (err: any) { showToast(err?.message ?? String(err)); }
}
async function deleteCat() {
  if (catMode.value !== "catedit" || !catId.value) return;
  try {
    // 选中项落到被删分类的前一个相邻项（首项被删则取后一个；无剩余则回"全部"），
    // 视图与药丸原地延续，不再跳回顶部
    const list = categories.value;
    const idx = list.findIndex((c) => c.id === catId.value);
    const nextId = idx > 0 ? list[idx - 1].id
      : (list.length > 1 ? list[1].id : "all");
    await api.deleteCategory(catId.value);
    catOpen.value = false; activeCat.value = nextId;
    await refresh();
    // 新选中项不在侧栏可视区时才最小滚动定位（可见则完全不动，无跳动）
    await nextTick();
    (railEl.value?.querySelector(".rail-item.active") as HTMLElement | null)
      ?.scrollIntoView({ block: "nearest", behavior: "smooth" });
    showToast("分类已删除");
  } catch (err: any) { showToast(err?.message ?? String(err)); }
}

async function savePop() {
  try {
    await api.update(editId.value, popName.value.trim(), popPath.value.trim(), popArgs.value.trim(), popCat.value, popEnv.value);
    showToast("已保存");
    editOpen.value = false;
    await refresh();
  } catch (err: any) { showToast(err?.message ?? String(err)); }
}
async function deleteCurrent() {
  if (editId.value) {
    await api.remove(editId.value);
    editOpen.value = false; await refresh(); showToast("已删除");
  }
}
/* 删除两段式确认（3 秒内再点一次才执行，防误删） */
const confirmTool = ref(false);
const confirmCat = ref(false);
function armConfirm(which: "tool" | "cat") {
  const flag = which === "tool" ? confirmTool : confirmCat;
  if (flag.value) {
    flag.value = false;
    if (which === "tool") deleteCurrent(); else deleteCat();
    return;
  }
  flag.value = true;
  setTimeout(() => { flag.value = false; }, 3000);
}

/* ---------------- 键盘 / Esc 层级 ---------------- */
function onGlobalKey(e: KeyboardEvent) {
  if (e.altKey && e.code === "Space") return; // 原生热键处理
  if (e.key === "Escape") {
    if (editOpen.value) { closeEditDialog(); return; }
    if (addOpen.value) { closeAddDialog(); return; }
    if (catOpen.value) { closeCatDialog(); return; }
    if (settingsOpen.value) { closeSettings(); return; }
    if (mode.value === "cmd") { search.value = ""; mode.value = "grid"; loadTools(); return; }
    api.hideWindow();
  }
}

/* ---------------- Toast ---------------- */
function showToast(msg: string) {
  toastMsg.value = msg;
  toastVisible.value = true;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => (toastVisible.value = false), 2200);
}

/* ---------------- 视图辅助 ---------------- */
const searchEl = ref<HTMLInputElement>();
/* centerAnchor 已废弃：所有管理型弹窗均改为居中模态 */
const catOf = (id: string) => categories.value.find((c) => c.id === id)?.name ?? "";

/* 卡片 specular 高光（M2-3，任务书 §2.5）：CSS 变量跟随鼠标 */
function cardHover(e: MouseEvent) {
  const el = e.currentTarget as HTMLElement;
  const r = el.getBoundingClientRect();
  el.style.setProperty("--mx", ((e.clientX - r.left) / r.width * 100) + "%");
  el.style.setProperty("--my", ((e.clientY - r.top) / r.height * 100) + "%");
}

/* 侧栏药丸 FLIP（M2-4）：top 直接锚定分类项 offsetTop，杜绝静态位置偏移 */
const railEl = ref<HTMLElement>();
const pillStyle = ref<Record<string, string>>({});
function movePill(el: HTMLElement | null) {
  if (!el) return;
  pillStyle.value = { top: el.offsetTop + "px", height: el.offsetHeight + "px" };
}
function onRailClick(c: Category, ev: MouseEvent) {
  switchCat(c.id);
  movePill(ev.currentTarget as HTMLElement);
}
function onRailContext(c: Category, ev: MouseEvent) {
  if (c.id === "all") return;
  ev.preventDefault();
  // 分类管理走居中模态（与【设置】同体系），不再跟随鼠标定位
  openCatEdit(c);
}
/* 药丸同步：分类列表或选中项变化（删除/新建/重命名/自动切换）时，
   重新锚定到当前 .rail-item.active，杜绝悬空残留 */
function syncPill() {
  const el = railEl.value?.querySelector(".rail-item.active") as HTMLElement | null;
  movePill(el);
}
watch([railCats, activeCat], () => nextTick(syncPill), { immediate: true });
</script>

<template>
  <div ref="appEl" class="app" data-file-drop-target :class="{ dragging }">
    <!-- 左侧栏（整列，对标 SpearX：红绿灯区 → 品牌 → 分类 → 新建） -->
    <div class="sidebar">
      <div class="brand">
        <span>Armory</span>
      </div>
      <nav ref="railEl" class="rail">
        <div class="rail-pill" :style="pillStyle"></div>
        <button v-for="c in railCats" :key="c.id" class="rail-item"
          :class="{ active: c.id === activeCat }"
          @click="onRailClick(c, $event)" @contextmenu="onRailContext(c, $event)">
          <svg v-if="c.id === 'all'" viewBox="0 0 24 24"><path d="M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z"/></svg>
          <!-- 最近使用：时钟图标 -->
          <svg v-else-if="c.id === 'recent'" viewBox="0 0 24 24"><path d="M12 2a10 10 0 100 20 10 10 0 000-20zm0 2a8 8 0 110 16 8 8 0 010-16zm-1 3v6l5 2.5.8-1.6-4.3-2.1V7H11z"/></svg>
          <!-- macOS 风格文件夹：后壳 + 前盖 + 高光，明暗主题均清晰 -->
          <svg v-else class="folder-ico" viewBox="0 0 24 24" aria-hidden="true">
            <path d="M2.5 6.2c0-.9.7-1.7 1.7-1.7h4.9c.5 0 .9.2 1.2.5l1.9 2h7.6c.9 0 1.7.8 1.7 1.7v.8H2.5V6.2z" fill="#3E8FD8"/>
            <rect x="2.5" y="8.2" width="19" height="11.3" rx="2" fill="#5FB2F0"/>
            <rect x="2.5" y="8.2" width="19" height="3" rx="1.5" fill="#ffffff" opacity=".28"/>
          </svg>
          <span>{{ c.name }}</span>
        </button>
      </nav>
      <button class="rail-item rail-add" title="新建分类"
        @click="openCatAdd()">
        <svg viewBox="0 0 24 24"><path d="M11 5h2v6h6v2h-6v6h-2v-6H5v-2h6z"/></svg>
        <span>新建</span>
      </button>
    </div>

    <!-- 右侧：内容列 -->
    <div class="main">
      <header class="topbar" style="--wails-draggable: drag" data-wails-draggable>
      <div class="searchbox">
        <svg viewBox="0 0 24 24"><path d="M10 2a8 8 0 105.3 14l4.9 4.9 1.4-1.4-4.9-4.9A8 8 0 0010 2zm0 2a6 6 0 110 12 6 6 0 010-12z"/></svg>
        <input ref="searchEl" v-model="search" type="text"
          placeholder="搜索工具，或直接键入启动…"
          autocomplete="off" spellcheck="false"
          @input="onSearchInput" @keydown="onSearchKey">
        <span class="kbd">⌥Space</span>
      </div>
      <button class="iconbtn" title="设置" @click="drawerOpen = !drawerOpen">
        <svg viewBox="0 0 24 24"><path d="M12 8a4 4 0 100 8 4 4 0 000-8zm8.94 3a8 8 0 00-.36-1.2l2-1.55-2-3.46-2.35.95a8 8 0 00-1.04-.6L16.8 2.5h-4l-.4 2.64c-.36.17-.7.37-1.03.6l-2.36-.95-2 3.46 2 1.55c-.16.39-.28.79-.37 1.2l-2.64.4v4l2.64.4c.09.41.21.81.37 1.2l-2 1.55 2 3.46 2.36-.95c.33.23.67.43 1.03.6l.4 2.64h4l.39-2.64c.36-.17.71-.37 1.04-.6l2.35.95 2-3.46-2-1.55c.16-.39.27-.79.36-1.2l2.64-.4v-4l-2.64-.4z"/></svg>
      </button>
      </header>

      <!-- 内容区 -->
      <main class="content">
        <!-- 网格态 -->
        <div v-if="mode === 'grid'" class="view enter-grid">
          <div class="grid">
            <div v-for="t in tools" :key="t.id" class="card" tabindex="0"
              @pointermove="cardHover"
              @click="doLaunch(t)"
              @contextmenu.prevent="openEdit(t, { x: $event.clientX, y: $event.clientY })"
              @keydown.enter.exact.prevent="doLaunch(t)"
              @keydown.meta.enter.prevent="openEdit(t, centerAnchor())">
              <div class="ticon" :class="t.type">
                <img v-if="icons[t.id]" :src="icons[t.id]" :alt="t.name">
                <span v-else>{{ TYPE_EMOJI[t.type] }}</span>
              </div>
              <div class="tname">{{ t.name }}</div>
              <div class="ttype">{{ TYPE_LABEL[t.type] }} · {{ catOf(t.categoryId) }}</div>
            </div>
            <div class="card add-card" tabindex="0" title="添加工具"
              @click="openAddDialog"
              @keydown.enter.prevent="openAddDialog">
              <div class="ticon add">＋</div>
              <div class="tname">添加</div>
              <div class="ttype">拖入或点击</div>
            </div>
          </div>
          <div v-if="!tools.length" class="empty-hint">
            把 .app / .jar / .py / .sh 拖进面板即可收录
          </div>
          <div v-if="dragging" class="dropzone-hint">松手即可收录到「{{ activeCatName }}」</div>
        </div>

        <!-- 命令面板态 -->
        <div v-else class="view enter-cmd">
          <div class="cmdlist">
            <div v-for="(t, i) in cmdResults" :key="t.id"
              class="cmdrow" :class="{ active: i === cmdIndex }"
              :style="{ animationDelay: Math.min(i, 8) * 40 + 'ms' }"
              @click="doLaunch(t)"
              @mouseenter="cmdIndex = i"
              @contextmenu.prevent="openEdit(t, { x: $event.clientX, y: $event.clientY })">
              <div class="ticon" :class="t.type">
                <img v-if="icons[t.id]" :src="icons[t.id]" :alt="t.name">
                <span v-else>{{ TYPE_EMOJI[t.type] }}</span>
              </div>
              <div class="cinfo">
                <div class="cname" v-html="hl(t.name)"></div>
                <div class="cpath">{{ catOf(t.categoryId) }} · {{ t.path }}<template v-if="t.args"> {{ t.args }}</template></div>
              </div>
              <span class="ckbd">⏎ 启动 · ⌘⏎ 编辑</span>
            </div>
            <div v-if="!cmdResults.length" class="cmd-empty">没有匹配「{{ search }}」的工具</div>
          </div>
        </div>
      </main>
    </div>
    <!-- /main -->

    <!-- 工具编辑（居中模态，与设置/添加/分类弹窗统一；头部显示工具真实图标） -->
    <template v-if="editOpen">
      <div class="settings-mask" :class="{ closing: editClosing }" @click="closeEditDialog"></div>
      <div class="settings-dialog edit-dialog" :class="{ closing: editClosing }" role="dialog" aria-label="编辑工具">
        <div class="settings-head">
          <div class="dlg-title">
            <span class="edit-ico">
              <img v-if="icons[editId]" :src="icons[editId]" :alt="popName">
              <span v-else>{{ TYPE_EMOJI[editIcon as keyof typeof TYPE_EMOJI] }}</span>
            </span>
            编辑工具
          </div>
          <button class="iconbtn" data-tip="关闭" title="关闭" @click="closeEditDialog">
            <svg viewBox="0 0 24 24"><path d="M6.4 5 5 6.4 10.6 12 5 17.6 6.4 19 12 13.4 17.6 19 19 17.6 13.4 12 19 6.4 17.6 5 12 10.6z"/></svg>
          </button>
        </div>
        <div class="settings-body edit-body">
          <div class="form-row">
            <label>工具名称</label>
            <input v-model="popName">
          </div>
          <div class="form-row">
            <label>路径</label>
            <input v-model="popPath">
          </div>
          <div class="form-row">
            <label>启动参数</label>
            <input v-model="popArgs" placeholder="选填，如 -sV -p 1-65535">
          </div>
          <div v-if="editIcon === 'jar'" class="form-row">
            <label>Java 版本</label>
            <select v-model="popEnv">
              <option value="">自动（系统默认）</option>
              <option v-for="v in javaVersions" :key="v.name" :value="v.name">{{ v.name }}</option>
            </select>
          </div>
          <div v-if="editIcon === 'py'" class="form-row">
            <label>Python 版本</label>
            <select v-model="popEnv">
              <option value="">自动（系统默认）</option>
              <option v-for="v in pythonVersions" :key="v.name" :value="v.name">{{ v.name }}</option>
            </select>
          </div>
          <div class="form-row">
            <label>所属分类</label>
            <select v-model="popCat">
              <option v-for="c in categories" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select>
          </div>
          <!-- 说明文档（M4-3）：工具目录有 README/说明文件时只读展示 -->
          <div v-if="editReadme" class="readme-box">
            <div class="readme-title">说明文档</div>
            <!-- eslint-disable-next-line vue/no-v-html —— 内容已经 mdToHtml 转义 -->
            <div class="readme-body" v-html="renderedReadme"></div>
          </div>
        </div>
        <div class="settings-foot dlg-actions">
          <button class="btn btn-danger" :class="{ arm: confirmTool }" @click="armConfirm('tool')">{{ confirmTool ? "确认删除？" : "删除" }}</button>
          <button class="btn btn-ghost" @click="closeEditDialog">取消</button>
          <button class="btn btn-primary" @click="savePop">保存</button>
        </div>
      </div>
    </template>

    <!-- 分类管理 / 新建分类（居中模态，与【设置】同体系） -->
    <template v-if="catOpen">
      <div class="settings-mask" :class="{ closing: catClosing }" @click="closeCatDialog"></div>
      <div class="settings-dialog cat-dialog" :class="{ closing: catClosing }" role="dialog" :aria-label="catMode === 'catadd' ? '新建分类' : '分类管理'">
        <div class="settings-head">
          <div class="dlg-title"><span class="dlg-ico">{{ catMode === 'catadd' ? '＋' : '✎' }}</span>{{ catMode === 'catadd' ? '新建分类' : '分类管理' }}</div>
          <button class="iconbtn" data-tip="关闭" title="关闭" @click="closeCatDialog">
            <svg viewBox="0 0 24 24"><path d="M6.4 5 5 6.4 10.6 12 5 17.6 6.4 19 12 13.4 17.6 19 19 17.6 13.4 12 19 6.4 17.6 5 12 10.6z"/></svg>
          </button>
        </div>
        <div class="settings-body cat-body">
          <div class="form-row">
            <label>{{ catMode === 'catadd' ? '分类名' : '重命名' }}</label>
            <input v-model="catNewName" :placeholder="catMode === 'catadd' ? '例如：内网渗透' : ''" @keydown.enter="saveCat">
          </div>
          <div v-if="catMode === 'catedit'" class="form-hint" style="padding: 8px 2px 2px 84px;">
            修改后保存立即生效；删除分类时，其中的工具会自动归入「其他分类」
          </div>
        </div>
        <div class="settings-foot dlg-actions">
          <button v-if="catMode === 'catedit'" class="btn btn-danger" @click="deleteCat">删除</button>
          <button class="btn btn-ghost" @click="closeCatDialog">取消</button>
          <button class="btn btn-primary" @click="saveCat">{{ catMode === 'catadd' ? '创建' : '保存' }}</button>
        </div>
      </div>
    </template>

    <!-- 设置弹窗（居中模态：遮罩点击/关闭按钮/Esc 均可关闭，内容区可滚动） -->
    <template v-if="settingsOpen">
      <div class="settings-mask" :class="{ closing: settingsClosing }" @click="closeSettings"></div>
      <div class="settings-dialog" :class="{ closing: settingsClosing }" role="dialog" aria-label="设置">
        <div class="settings-head">
          <div class="drawer-title">设置</div>
          <button class="iconbtn" data-tip="关闭" title="关闭" @click="closeSettings">
            <svg viewBox="0 0 24 24"><path d="M6.4 5 5 6.4 10.6 12 5 17.6 6.4 19 12 13.4 17.6 19 19 17.6 13.4 12 19 6.4 17.6 5 12 10.6z"/></svg>
          </button>
        </div>
        <div class="settings-body">
          <div class="group-title">通用</div>
          <div class="group">
            <div class="row">
              <div class="rinfo">
                <div class="rlabel">开机自启</div>
                <div class="rsub">登录 macOS 时自动启动（可在系统设置 → 登录项中管理）</div>
              </div>
              <div class="seg">
                <button :class="{ on: launchOnLogin }" @click="onLoginPick(true)">开</button>
                <button :class="{ on: !launchOnLogin }" @click="onLoginPick(false)">关</button>
              </div>
            </div>
          </div>

          <div class="group-title">外观</div>
          <div class="group">
            <div class="row">
              <div class="rinfo">
                <div class="rlabel">主题</div>
                <div class="rsub">跟随系统明暗，或手动指定</div>
              </div>
              <div class="seg">
                <button :class="{ on: themeMode === 'auto' }" @click="onThemePick('auto')">自动</button>
                <button :class="{ on: themeMode === 'light' }" @click="onThemePick('light')">浅色</button>
                <button :class="{ on: themeMode === 'dark' }" @click="onThemePick('dark')">深色</button>
              </div>
            </div>
            <div class="row">
              <div class="rinfo">
                <div class="rlabel">界面字体</div>
                <div class="rsub">全局缩放，立即生效</div>
              </div>
              <div class="seg">
                <button :class="{ on: fontScale === 'standard' }" @click="onFontScalePick('standard')">标准</button>
                <button :class="{ on: fontScale === 'large' }" @click="onFontScalePick('large')">大</button>
                <button :class="{ on: fontScale === 'xlarge' }" @click="onFontScalePick('xlarge')">特大</button>
              </div>
            </div>
            <div class="row">
              <div class="rinfo">
                <div class="rlabel">减弱动态效果</div>
                <div class="rsub">动画降级为快速淡入淡出</div>
              </div>
              <div class="seg">
                <button :class="{ on: reduceMotion }" @click="onReducePick(true)">开</button>
                <button :class="{ on: !reduceMotion }" @click="onReducePick(false)">关</button>
              </div>
            </div>
          </div>

          <div class="group-title">Java 环境</div>
          <div class="group">
            <div class="env-grid">
              <div class="env-row" v-for="(row, i) in envJava" :key="'j' + i">
                <input v-model="row.name" class="env-name" placeholder="版本名" spellcheck="false">
                <input v-model="row.path" placeholder="java 可执行文件路径" spellcheck="false" autocomplete="off">
                <button class="env-del" title="删除此版本" @click="delEnvRow(envJava, i)">
                  <svg viewBox="0 0 24 24"><path d="M9 3v1H4v2h16V4h-5V3H9zm-3.5 5 .8 12.1c.1 1 .9 1.9 2 1.9h7.4c1.1 0 1.9-.9 2-1.9L18.5 8h-13zM10 11h1.5v7H10v-7zm2.5 0H14v7h-1.5v-7z"/></svg>
                </button>
              </div>
              <button v-if="!envJava.length" class="env-empty" @click="addEnvRow(envJava)">尚未配置，点击添加 Java 版本</button>
              <div class="env-foot">
                <span class="env-foot-left">
                  <button class="env-add" @click="addEnvRow(envJava)">＋ 添加版本</button>
                  <button class="env-add" @click="scanJava">⟳ 扫描本机</button>
                </span>
                <button class="btn btn-primary env-save" :disabled="envSaving === 'java'" @click="saveEnv('java')">{{ envSaving === 'java' ? "保存中…" : "保存" }}</button>
              </div>
            </div>
          </div>
          <div class="group-hint">配置后，JAR 工具启动时按「编辑 → Java 版本」使用对应解释器；未指定则用系统默认 java。</div>

          <div class="group-title">Python 环境</div>
          <div class="group">
            <div class="env-grid">
              <div class="env-row" v-for="(row, i) in envPy" :key="'p' + i">
                <input v-model="row.name" class="env-name" placeholder="版本名" spellcheck="false">
                <input v-model="row.path" placeholder="python 可执行文件路径" spellcheck="false" autocomplete="off">
                <button class="env-del" title="删除此版本" @click="delEnvRow(envPy, i)">
                  <svg viewBox="0 0 24 24"><path d="M9 3v1H4v2h16V4h-5V3H9zm-3.5 5 .8 12.1c.1 1 .9 1.9 2 1.9h7.4c1.1 0 1.9-.9 2-1.9L18.5 8h-13zM10 11h1.5v7H10v-7zm2.5 0H14v7h-1.5v-7z"/></svg>
                </button>
              </div>
              <button v-if="!envPy.length" class="env-empty" @click="addEnvRow(envPy)">尚未配置，点击添加 Python 版本</button>
              <div class="env-foot">
                <span class="env-foot-left">
                  <button class="env-add" @click="addEnvRow(envPy)">＋ 添加版本</button>
                  <button class="env-add" @click="scanPython">⟳ 扫描本机</button>
                </span>
                <button class="btn btn-primary env-save" :disabled="envSaving === 'py'" @click="saveEnv('py')">{{ envSaving === 'py' ? "保存中…" : "保存" }}</button>
              </div>
            </div>
          </div>
          <div class="group-hint">Python 工具可在「编辑 → Python 版本」中选择这里配置的版本；未选择则用系统默认 python3。</div>

          <div class="group-title">快捷键</div>
          <div class="group">
            <div class="row">
              <div class="rinfo">
                <div class="rlabel">呼出 / 隐藏面板</div>
                <div class="rsub">全局热键，任何界面按下即生效</div>
              </div>
              <div class="kbd-chips"><span>⌥</span><span>Space</span></div>
            </div>
          </div>

          <div class="group-title">收录</div>
          <div class="group">
            <div class="row">
              <div class="rinfo">
                <div class="rlabel">拖拽收录</div>
                <div class="rsub">把 .app / .jar / .py / .sh 拖进面板</div>
              </div>
              <span class="state-dot on"></span>
            </div>
            <div class="row">
              <div class="rinfo">
                <div class="rlabel">真实图标</div>
                <div class="rsub">自动提取 .app 原版图标并缓存</div>
              </div>
              <span class="state-dot on"></span>
            </div>
          </div>

          <div class="group-title">数据</div>
          <div class="group">
            <div class="row">
              <div class="rinfo">
                <div class="rlabel">导出备份</div>
                <div class="rsub">工具、分类、设置与图标打包为 zip</div>
              </div>
              <button class="btn btn-ghost env-save" @click="doExport">导出</button>
            </div>
            <div class="row">
              <div class="rinfo">
                <div class="rlabel">导入还原</div>
                <div class="rsub">从备份包恢复，会覆盖当前全部数据</div>
              </div>
              <button class="btn btn-danger env-save" @click="doImport">{{ importArmed ? "确认还原？" : "导入" }}</button>
            </div>
          </div>
        </div>
        <div class="settings-foot">Armory v0.1.0 · 个人专用 · 数据存储于 ~/.armory</div>
      </div>
    </template>

    <!-- 添加工具弹窗（表单字段对齐数据结构：名称/路径/参数/分类） -->
    <template v-if="addOpen">
      <div class="settings-mask" :class="{ closing: addClosing }" @click="closeAddDialog"></div>
      <div class="settings-dialog" :class="{ closing: addClosing }" role="dialog" aria-label="添加工具">
        <div class="settings-head">
          <div class="dlg-title"><span class="dlg-ico">＋</span>添加工具</div>
          <button class="iconbtn" data-tip="关闭" title="关闭" @click="closeAddDialog">
            <svg viewBox="0 0 24 24"><path d="M6.4 5 5 6.4 10.6 12 5 17.6 6.4 19 12 13.4 17.6 19 19 17.6 13.4 12 19 6.4 17.6 5 12 10.6z"/></svg>
          </button>
        </div>
        <div class="settings-body">
          <div class="form-row">
            <label>工具名称</label>
            <input v-model="addForm.name" placeholder="选填，留空自动识别" @keydown.enter="submitAdd">
          </div>
          <div class="form-row">
            <label>路径</label>
            <input v-model="addForm.path" placeholder="必填，如 /Applications/X.app 或 ~/Tools/x.jar" @keydown.enter="submitAdd">
          </div>
          <div class="form-row">
            <label>启动参数</label>
            <input v-model="addForm.args" placeholder="选填，如 -sV -p 1-65535" @keydown.enter="submitAdd">
          </div>
          <div class="form-row">
            <label>所属分类</label>
            <select v-model="addForm.categoryId">
              <option v-for="c in categories" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select>
          </div>
          <div class="form-row" v-if="addType === 'jar' || addType === 'py'">
            <label>运行版本</label>
            <div class="switch-row">
              <label class="switch">
                <input type="checkbox" v-model="addUseGlobal">
                <span class="track"><span class="thumb"></span></span>
              </label>
              <div class="switch-info">
                <div class="switch-label">是否采用全局设置</div>
                <div class="switch-sub">{{ addUseGlobal ? "使用「设置 → " + (addType === 'jar' ? "Java 环境" : "Python 环境") + "」中配置的版本" : "使用系统默认解释器" }}</div>
              </div>
            </div>
          </div>
          <div class="form-row" v-if="addUseGlobal && addType !== 'bin'">
            <label>全局版本</label>
            <select v-model="addEnvVersion">
              <option v-if="addType === 'jar' && !javaVersions.length" value="" disabled>尚未在设置中配置 Java 版本</option>
              <option v-if="addType === 'py' && !pythonVersions.length" value="" disabled>尚未在设置中配置 Python 版本</option>
              <option v-for="v in (addType === 'jar' ? javaVersions : pythonVersions)" :key="v.name" :value="v.name">{{ v.name }}</option>
            </select>
          </div>
          <div class="form-hint">也支持把文件直接拖进面板收录，类型与图标会自动识别</div>
        </div>
        <div class="settings-foot dlg-actions">
          <button class="btn btn-ghost" @click="closeAddDialog">取消</button>
          <button class="btn btn-primary" @click="submitAdd">添加工具</button>
        </div>
      </div>
    </template>

    <!-- Toast -->
    <div class="toast" :class="{ show: toastVisible }"><span class="dot"></span>{{ toastMsg }}</div>
  </div>
</template>

<style>
/* ============================================================
   主题 Token（M2-7，任务书 §2.4）—— 唯一色板，禁止页面内自造颜色
   ============================================================ */
:root {
  --r-window: 24px; --r-card: 14px; --r-btn: 8px;
  /* 动画常量（lib/motion.ts 同源，CSS 侧用变量） */
  --spring: cubic-bezier(0.34, 1.4, 0.64, 1);
  --spring-soft: cubic-bezier(0.3, 1, 0.5, 1);
  --ease-in: cubic-bezier(0.5, 0, 0.75, 0);

  /* Light（默认） */
  --text-1: rgba(0, 0, 0, 0.88);
  --text-2: rgba(0, 0, 0, 0.55);
  --text-3: rgba(0, 0, 0, 0.35);
  --accent: #0a84ff; /* 运行时被系统强调色覆盖（theme.ts） */
  --glass-border: rgba(255, 255, 255, 0.55);
  --hairline: rgba(127, 127, 127, 0.15);
  --field-bg: rgba(127, 127, 127, 0.08);
  --card-bg: rgba(127, 127, 127, 0.10);
  --card-hover: rgba(127, 127, 127, 0.20);
  --pill: rgba(127, 127, 127, 0.18);
  --surface-1: rgba(245, 246, 248, 0.88);  /* 弹层 */
  --surface-2: rgba(245, 246, 248, 0.92);  /* 抽屉/Toast */
  --highlight: rgba(255, 255, 255, 0.5);   /* specular 高光强度 */
  --search-bg: rgba(255, 255, 255, 0.35);
  /* 面板主背景（近实心：玻璃只从侧栏透出，其余区域正常显示） */
  --panel-bg: rgba(243, 245, 248, 0.97);
}
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {
    --text-1: rgba(255, 255, 255, 0.92);
    --text-2: rgba(255, 255, 255, 0.60);
    --text-3: rgba(255, 255, 255, 0.35);
    --glass-border: rgba(255, 255, 255, 0.14);
    --hairline: rgba(255, 255, 255, 0.10);
    --field-bg: rgba(255, 255, 255, 0.07);
    --card-bg: rgba(255, 255, 255, 0.07);
    --card-hover: rgba(255, 255, 255, 0.16);
    --pill: rgba(255, 255, 255, 0.14);
    --surface-1: rgba(40, 42, 50, 0.88);
    --surface-2: rgba(40, 42, 50, 0.92);
    --highlight: rgba(255, 255, 255, 0.18);
    --search-bg: rgba(0, 0, 0, 0.22);
    --panel-bg: rgba(28, 30, 36, 0.97);
  }
}
:root[data-theme="dark"] {
  --text-1: rgba(255, 255, 255, 0.92);
  --text-2: rgba(255, 255, 255, 0.60);
  --text-3: rgba(255, 255, 255, 0.35);
  --glass-border: rgba(255, 255, 255, 0.14);
  --hairline: rgba(255, 255, 255, 0.10);
  --field-bg: rgba(255, 255, 255, 0.07);
  --card-bg: rgba(255, 255, 255, 0.07);
  --card-hover: rgba(255, 255, 255, 0.16);
  --pill: rgba(255, 255, 255, 0.14);
  --surface-1: rgba(40, 42, 50, 0.88);
  --surface-2: rgba(40, 42, 50, 0.92);
  --highlight: rgba(255, 255, 255, 0.18);
  --search-bg: rgba(0, 0, 0, 0.22);
  --panel-bg: rgba(28, 30, 36, 0.97);
}
/* 减弱动态效果（系统偏好或用户设置，lib/motion.ts 切 class） */
:root.reduce-motion *, :root.reduce-motion *::before, :root.reduce-motion *::after {
  animation-duration: 100ms !important;
  animation-iteration-count: 1 !important;
  transition-duration: 100ms !important;
}

* { margin: 0; padding: 0; box-sizing: border-box; }
/* 深色文字 bug 的根源修复：表单控件的 UA 默认色（ButtonText）跟随"系统外观"
   而非应用主题——系统浅色 + 应用深色时按钮文字变黑。统一强制走主题 token。 */
button, input, select, textarea { color: var(--text-1); font-family: inherit; }
html, body, #app {
  height: 100%;
  background: transparent !important;
  font-family: -apple-system, BlinkMacSystemFont, "SF Pro Text", "PingFang SC", sans-serif;
  color: var(--text-1);
  user-select: none;
  overflow: hidden;
}
input, select { font-family: inherit; color: inherit; }

/* 布局：左侧栏整列 + 右侧内容列（对标 SpearX） */
.app { height: 100%; display: flex; flex-direction: row; transform-origin: 50% 20%; }
.sidebar {
  width: 88px; flex-shrink: 0;
  display: flex; flex-direction: column;
  border-right: 1px solid var(--hairline);
}
.main { flex: 1; min-width: 0; display: flex; flex-direction: column; }

/* 品牌（红绿灯下方） */
.brand {
  display: flex; align-items: center; justify-content: center; gap: 6px;
  padding: 34px 4px 12px;
  font-size: 13px; font-weight: 600; opacity: 0.85; flex-shrink: 0;
}
.brand .logo { font-size: 16px; }

/* 侧栏分类 */
.rail {
  position: relative; /* 药丸的定位基准（缺失会横跨整窗宽度） */
  flex: 1; padding: 4px 8px;
  display: flex; flex-direction: column; gap: 2px; overflow-y: auto;
  scrollbar-width: none;
}
/* 呼出入场（Launchpad 对标）：窗口层 alpha 淡入（Go 侧）+ 内容层 1.05→1.0 回落，
   双层并行 ≈ Launchpad 图标回落感；260ms 弹性 */
.app.entering { animation: winIn 260ms var(--spring) both; }
@keyframes winIn {
  from { opacity: 0; transform: scale(1.05); }
  to   { opacity: 1; transform: scale(1); }
}

/* 顶栏（仅内容区横跨；红绿灯在侧栏上方，无需避让） */
.topbar { display: flex; align-items: center; gap: 10px; padding: 12px 16px 10px; flex-shrink: 0; }
.searchbox {
  flex: 1; display: flex; align-items: center; gap: 8px;
  background: var(--search-bg);
  border: 1px solid var(--glass-border);
  border-radius: 999px; padding: 8px 16px; min-width: 0; /* 药丸形（对标 SpearX） */
  transition: box-shadow 160ms var(--spring-soft), border-color 160ms;
}
.searchbox:focus-within { border-color: var(--accent); box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 12%, transparent); }
.searchbox svg { width: 15px; height: 15px; fill: var(--text-3); flex-shrink: 0; }
.searchbox input { flex: 1; border: none; outline: none; background: transparent; font-size: 13px; min-width: 0; }
.searchbox input::placeholder { color: var(--text-3); }
.kbd {
  font-size: 10px; opacity: 0.5;
  border: 1px solid var(--glass-border); border-radius: 5px;
  padding: 1px 5px; flex-shrink: 0;
}
.iconbtn {
  position: relative; /* 悬停提示锚点 */
  width: 32px; height: 32px; flex-shrink: 0;
  border: none; border-radius: var(--r-btn); background: transparent; cursor: pointer;
  display: grid; place-items: center; opacity: 0.7;
  transition: background 120ms ease-out, transform 80ms ease-out, opacity 120ms;
}
.iconbtn:hover { background: var(--card-hover); opacity: 1; }
.iconbtn:active { transform: scale(0.92); }
.iconbtn svg { width: 16px; height: 16px; fill: currentColor; }
/* 悬停功能提示（纯 CSS，对标系统 tooltip 观感） */
.iconbtn[data-tip]::after {
  content: attr(data-tip);
  position: absolute; top: calc(100% + 8px); right: 0;
  background: var(--surface-1);
  -webkit-backdrop-filter: blur(20px); backdrop-filter: blur(20px);
  border: 1px solid var(--glass-border);
  color: var(--text-1);
  font-size: 11px; font-weight: 500; line-height: 1;
  padding: 6px 10px; border-radius: 7px;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.18);
  white-space: nowrap; pointer-events: none;
  opacity: 0; transform: translateY(-3px);
  transition: opacity 140ms var(--spring-soft), transform 140ms var(--spring-soft);
  z-index: 50;
}
.iconbtn[data-tip]:hover::after { opacity: 1; transform: translateY(0); }
/* 弹窗内的按钮提示朝下弹（在顶部边缘） */
.settings-head .iconbtn[data-tip]::after { top: calc(100% + 6px); }

.rail::-webkit-scrollbar { display: none; }
.rail-item {
  position: relative; z-index: 1;
  border: none; background: transparent; cursor: pointer;
  display: flex; flex-direction: column; align-items: center; gap: 4px;
  padding: 9px 4px 8px; border-radius: var(--r-btn);
  color: inherit; opacity: 0.6; font-size: 11px; font-family: inherit;
  transition: opacity 150ms, background 150ms;
}
.rail-item svg { width: 18px; height: 18px; fill: currentColor; }
/* 说明文档只读区（编辑弹窗，M4-3） */
.readme-box {
  margin-top: 10px;
  border: 1px solid var(--glass-border);
  border-radius: var(--r-btn);
  background: var(--card-bg);
  max-height: 180px;
  overflow-y: auto;
  padding: 10px 12px;
}
.readme-title {
  font-size: 10.5px; font-weight: 600; opacity: 0.5;
  letter-spacing: 0.4px; margin-bottom: 6px;
}
.readme-body { font-size: 12px; line-height: 1.6; opacity: 0.85; }
.readme-body h3, .readme-body h4, .readme-body h5, .readme-body h6 {
  font-size: 12.5px; font-weight: 650; margin: 8px 0 4px;
}
.readme-body p { margin: 4px 0; word-break: break-word; }
.readme-body ul { padding-left: 18px; margin: 4px 0; }
.readme-body li { margin: 2px 0; }
.readme-body code {
  font-family: ui-monospace, "SF Mono", Menlo, monospace;
  font-size: 11px;
  background: var(--field-bg);
  border-radius: 4px; padding: 1px 5px;
}
.readme-body pre {
  background: var(--field-bg);
  border-radius: var(--r-btn);
  padding: 8px 10px; margin: 6px 0;
  overflow-x: auto;
}
.readme-body pre code { background: transparent; padding: 0; }
.folder-ico {
  width: 17px; height: 17px; fill: none; flex-shrink: 0;
  filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.18));
  transition: transform 160ms var(--spring);
}
.rail-item:hover .folder-ico { transform: scale(1.08); }
.rail-item.active .folder-ico { transform: scale(1.05); }
.rail-item.active, .rail-item:hover { opacity: 1; }
.rail-item:hover:not(.active) { background: var(--card-bg); }
/* 分类图标使用 .folder-ico（macOS 文件夹），catdot 已废弃 */
.rail-add { margin-top: auto; opacity: 0.4; }
.rail-add:hover { opacity: 0.9; }
.rail-pill {
  position: absolute; left: 8px; right: 8px; top: 0; height: 0;
  border-radius: var(--r-btn); background: var(--pill);
  box-shadow: inset 0 1px 0 var(--glass-border), 0 1px 4px rgba(0,0,0,0.08);
  z-index: 0;
  transition: top 240ms var(--spring), height 240ms var(--spring);
  pointer-events: none;
}

.content { flex: 1; min-width: 0; position: relative; overflow: hidden; }
.view { position: absolute; inset: 0; padding: 10px 14px 14px; overflow-y: auto; scrollbar-width: none; }
.view::-webkit-scrollbar { display: none; }
.view.enter-grid { animation: viewGridIn 240ms var(--spring) both; }
@keyframes viewGridIn { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: translateY(0); } }
.view.enter-cmd { animation: viewCmdIn 240ms var(--spring) both; }
@keyframes viewCmdIn { from { opacity: 0; transform: translateY(-8px); } to { opacity: 1; transform: translateY(0); } }

/* 网格与卡片 */
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(92px, 1fr)); gap: 10px; align-content: start; }
.card {
  position: relative;
  display: flex; flex-direction: column; align-items: center; gap: 7px;
  padding: 13px 6px 10px; border-radius: var(--r-card);
  background: var(--card-bg); border: 1px solid transparent;
  cursor: pointer; will-change: transform;
  transition: transform 120ms ease-out, background 120ms ease-out, border-color 120ms;
}
/* M2-3：Liquid Glass specular highlight，光斑跟随鼠标 */
.card::after {
  content: ""; position: absolute; inset: 0; border-radius: inherit;
  background: radial-gradient(90px 90px at var(--mx, 50%) var(--my, 0%), var(--highlight) 0%, transparent 65%);
  opacity: 0; transition: opacity 120ms ease-out; pointer-events: none;
}
.card:hover { transform: translateY(-2px); background: var(--card-hover); border-color: var(--glass-border); }
.card:hover::after { opacity: 1; }
.card:active { transform: scale(0.96); }
.card:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.ticon {
  width: 48px; height: 48px; border-radius: 12px;
  display: grid; place-items: center; font-size: 24px; line-height: 1;
  box-shadow: 0 3px 8px rgba(0,0,0,0.18), inset 0 1px 0 rgba(255,255,255,0.35);
  position: relative; z-index: 1; overflow: hidden;
}
.ticon img { width: 100%; height: 100%; object-fit: cover; }
.ticon.jar { background: linear-gradient(135deg, #f7b733, #e08b2d); }
.ticon.py { background: linear-gradient(135deg, #4b8bbe, #306998); }
.ticon.sh { background: linear-gradient(135deg, #3a3a3c, #1c1c1e); }
.ticon.bin { background: linear-gradient(135deg, #5e5ce6, #3634a3); }
.ticon.app { background: linear-gradient(135deg, #30d158, #1e9e42); }
.ticon.add { background: transparent; border: 1.5px dashed var(--text-3); box-shadow: none; font-size: 20px; opacity: 0.6; }
.add-card { background: transparent; border: 1px dashed var(--text-3); }
.add-card:hover { background: var(--card-bg); }
.tname { font-size: 11.5px; font-weight: 500; max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; position: relative; z-index: 1; }
.ttype { font-size: 9.5px; opacity: 0.45; position: relative; z-index: 1; }
.empty-hint { text-align: center; padding: 30px 0; font-size: 12px; opacity: 0.5; }
.dropzone-hint {
  margin-top: 12px; padding: 18px;
  border: 1.5px dashed var(--accent); border-radius: var(--r-card);
  text-align: center; font-size: 12px; color: var(--accent);
  background: color-mix(in srgb, var(--accent) 12%, transparent);
  animation: breathe 1.6s ease-in-out infinite;
}
@keyframes breathe { 50% { opacity: 0.55; } }

/* 命令面板 */
.cmdlist { display: flex; flex-direction: column; gap: 2px; }
.cmdrow {
  display: flex; align-items: center; gap: 12px;
  padding: 8px 12px; border-radius: var(--r-btn); cursor: pointer;
  transition: background 100ms;
  animation: rowIn 240ms var(--spring) both; /* M2-5：列表行错峰进入 */
}
@keyframes rowIn { from { opacity: 0; transform: translateY(-6px); } to { opacity: 1; transform: translateY(0); } }
.cmdrow.active { background: var(--card-hover); box-shadow: inset 0 0 0 1px var(--glass-border); }
.cmdrow .ticon { width: 30px; height: 30px; border-radius: 8px; font-size: 15px; }
.cinfo { flex: 1; min-width: 0; }
.cname { font-size: 13px; font-weight: 500; }
.cname mark { background: transparent; color: var(--accent); font-weight: 600; }
.cpath { font-size: 11px; opacity: 0.55; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ckbd { font-size: 10px; opacity: 0.4; flex-shrink: 0; }
.cmd-empty { text-align: center; padding: 40px 0; font-size: 13px; opacity: 0.5; }

/* 工具编辑模态（复用 settings-dialog 外壳，与添加工具同宽；头部为工具真实图标） */
.edit-dialog { width: min(460px, calc(100vw - 40px)); max-height: min(72vh, 560px); }
.edit-body { max-height: calc(min(72vh, 560px) - 96px); }
.edit-ico {
  width: 30px; height: 30px; border-radius: 8px; flex-shrink: 0;
  display: grid; place-items: center; font-size: 16px;
  background: var(--search-bg); border: 1px solid var(--hairline);
  overflow: hidden;
}
.edit-ico img { width: 100%; height: 100%; object-fit: cover; }

.btn {
  flex: 1; border: none; border-radius: var(--r-btn); padding: 8px 0;
  font-size: 12.5px; font-weight: 600; cursor: pointer; font-family: inherit;
  transition: transform 80ms ease-out, filter 120ms;
}
.btn:active { transform: scale(0.96); }
.btn-primary {
  /* 系统强调色偏浅时白字对比不足 → 混入 18% 黑压深，保持色相 */
  background: color-mix(in srgb, var(--accent) 82%, #000);
  color: #fff;
  box-shadow: 0 2px 8px color-mix(in srgb, var(--accent) 40%, transparent);
}
.btn-primary:hover {
  filter: brightness(1.12);
  box-shadow: 0 3px 12px color-mix(in srgb, var(--accent) 55%, transparent);
}
.btn-ghost {
  background: var(--search-bg); color: inherit;
  border: 1px solid var(--glass-border);
}
.btn-ghost:hover { background: var(--card-hover); border-color: var(--accent); }
.btn-danger {
  background: rgba(255, 69, 58, 0.12); color: #ff453a;
  flex: 0 0 auto; padding: 8px 14px; font-weight: 600;
}
.btn-danger:hover { background: rgba(255, 69, 58, 0.22); }
.btn-danger.arm { background: #ff453a; color: #fff; } /* 两段式确认态 */

/* 抽屉 */
/* 设置弹窗（居中模态，替代原右滑抽屉） */
.settings-mask {
  position: fixed; inset: 0; z-index: 30;
  background: rgba(0, 0, 0, 0.32);
  -webkit-backdrop-filter: blur(6px); backdrop-filter: blur(6px);
  animation: maskIn 180ms ease-out both;
}
.settings-mask.closing { animation: maskOut 180ms ease-in both; }
@keyframes maskIn { from { opacity: 0; } to { opacity: 1; } }
@keyframes maskOut { from { opacity: 1; } to { opacity: 0; } }
.settings-dialog {
  position: fixed; left: 50%; top: 50%; z-index: 31;
  transform: translate(-50%, -50%);
  width: min(460px, calc(100vw - 40px));
  max-height: min(72vh, 560px);
  overflow: hidden; /* 圆角裁剪内容 */
  display: flex; flex-direction: column;
  border-radius: 18px;
  background: var(--surface-1);
  -webkit-backdrop-filter: blur(32px) saturate(1.7);
  backdrop-filter: blur(32px) saturate(1.7);
  border: 1px solid var(--glass-border);
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.28);
  animation: dlgIn 260ms var(--spring) both;
}
.settings-dialog.closing { animation: dlgOut 180ms ease-in both; }
@keyframes dlgIn {
  from { opacity: 0; transform: translate(-50%, -50%) scale(0.88); }
  to   { opacity: 1; transform: translate(-50%, -50%) scale(1); }
}
@keyframes dlgOut {
  from { opacity: 1; transform: translate(-50%, -50%) scale(1); }
  to   { opacity: 0; transform: translate(-50%, -50%) scale(0.9); }
}
.settings-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 12px 10px 18px; flex-shrink: 0;
  border-bottom: 1px solid var(--hairline);
}
.drawer-title { font-size: 15px; font-weight: 700; letter-spacing: 0.3px; }
/* 弹窗标题通用（设置/添加工具） */
.dlg-title { display: flex; align-items: center; gap: 8px; font-size: 15px; font-weight: 700; letter-spacing: 0.3px; }
.dlg-ico {
  width: 22px; height: 22px; border-radius: 7px;
  background: var(--accent); color: #fff;
  display: grid; place-items: center;
  font-size: 14px; font-weight: 700; line-height: 1;
  box-shadow: 0 2px 6px color-mix(in srgb, var(--accent) 40%, transparent);
}
/* 表单行：左侧标签 + 右侧控件（对标 SpearX 添加表单） */
.form-row {
  display: grid; grid-template-columns: 72px 1fr;
  align-items: center; gap: 12px;
  padding: 6px 2px;
}
.form-row + .form-row { margin-top: 4px; }
.form-row > label {
  font-size: 12.5px; font-weight: 550; opacity: 0.75;
  text-align: right; white-space: nowrap;
}
.form-row input, .form-row select {
  width: 100%; border: 1px solid var(--glass-border);
  background: var(--field-bg); border-radius: var(--r-btn);
  padding: 8px 11px; font-size: 12.5px; outline: none;
  transition: border-color 120ms, box-shadow 120ms;
  box-sizing: border-box;
}
/* 下拉与输入框视觉统一：去原生外观，自绘箭头 */
.form-row select {
  -webkit-appearance: none; appearance: none;
  padding-right: 32px;
  background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='10' height='6' viewBox='0 0 10 6'><path d='M1 1l4 4 4-4' fill='none' stroke='%23888' stroke-width='1.6' stroke-linecap='round' stroke-linejoin='round'/></svg>");
  background-repeat: no-repeat;
  background-position: right 12px center;
  cursor: pointer;
}
.form-row input:focus, .form-row select:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 12%, transparent);
}
.form-hint {
  font-size: 11px; opacity: 0.45; padding: 10px 2px 2px; line-height: 1.5;
  margin-left: 84px; /* 与输入列对齐（72px 标签 + 12px 间距） */
}
/* 弹窗底部按钮区（不再复用带 opacity:0.4 的信息条样式——三个按钮发灰的根因） */
.dlg-actions {
  display: flex; justify-content: flex-end; align-items: center; gap: 10px;
  padding: 10px 18px; flex-shrink: 0;
  border-top: 1px solid var(--hairline);
}
.dlg-actions .btn { flex: 0 0 auto; padding: 8px 22px; }
.settings-body {
  flex: 1; min-height: 0; overflow-y: auto;
  /* 双保险：显式上限，不依赖 flex 收缩（WKWebView 下 flex+max-height 组合不可靠）。
     96px ≈ 头部(52) + 底部信息条(38) + 边框 */
  max-height: calc(min(72vh, 560px) - 96px);
  padding: 8px 14px 14px;
  display: flex; flex-direction: column; gap: 4px;
  scrollbar-width: thin;
  scrollbar-color: var(--text-3) transparent;
}
/* 关键修复：分组卡片禁止 flex 压缩。
   .group 自带 overflow:hidden，作为 flex 子元素时会失去最小内容高度保护，
   内容超高时被压缩裁切（此前"减弱动态效果"整行消失的根因）。
   禁止收缩后，内容超出 body 上限才会正确触发滚动。 */
.settings-body .group-title,
.settings-body .group { flex-shrink: 0; }

.settings-body::-webkit-scrollbar { width: 6px; }
.settings-body::-webkit-scrollbar-thumb { background: var(--text-3); border-radius: 3px; opacity: 0.4; }
.settings-body::-webkit-scrollbar-track { background: transparent; }
.group-title {
  font-size: 11px; opacity: 0.45; font-weight: 700;
  letter-spacing: 1px; margin: 12px 4px 6px;
}
.group {
  border: 1px solid var(--glass-border);
  border-radius: var(--r-card);
  overflow: hidden;
  background: var(--field-bg);
}
.row {
  display: flex; align-items: center; justify-content: space-between; gap: 14px;
  padding: 11px 14px;
  transition: background 140ms ease-out;
}
.row:hover { background: var(--card-bg); }
.row + .row { border-top: 1px solid var(--hairline); }
.rinfo { min-width: 0; }
.rlabel { font-size: 13px; font-weight: 550; }
.rsub { font-size: 11px; opacity: 0.5; margin-top: 2px; line-height: 1.4; }
.kbd-chips { display: flex; gap: 4px; flex-shrink: 0; }
.kbd-chips span {
  border: 1px solid var(--glass-border);
  background: var(--search-bg);
  border-radius: 6px; padding: 3px 9px;
  font-size: 12px; font-weight: 600;
  box-shadow: 0 1px 0 rgba(0,0,0,0.08);
}
.state-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.state-dot.on { background: #30d158; box-shadow: 0 0 6px rgba(48, 209, 88, 0.5); }

/* 运行环境表单（Java / Python 各自独立分组，版本名 + 路径 + 删除三列） */
.env-grid { display: flex; flex-direction: column; gap: 6px; padding: 10px 12px; }
.env-row {
  display: grid; grid-template-columns: 88px 1fr 26px;
  align-items: center; gap: 6px;
}
.env-row input {
  width: 100%; border: 1px solid var(--glass-border);
  background: var(--search-bg); border-radius: var(--r-btn);
  padding: 6px 10px; font-size: 12px; outline: none; min-width: 0;
  transition: border-color 120ms, box-shadow 120ms;
}
.env-row input:focus {
  border-color: var(--accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 12%, transparent);
}
.env-row input.env-name { font-weight: 600; }
.env-del {
  border: none; background: transparent; cursor: pointer;
  padding: 5px; border-radius: var(--r-btn); opacity: 0.3;
  display: grid; place-items: center;
  transition: opacity 120ms, background 120ms;
}
.env-del svg { width: 13px; height: 13px; fill: currentColor; }
.env-del:hover { opacity: 1; background: rgba(255, 69, 58, 0.12); color: #ff453a; }
.env-empty {
  border: 1px dashed var(--glass-border); border-radius: var(--r-btn);
  background: transparent; cursor: pointer; font: inherit;
  font-size: 11.5px; opacity: 0.5; padding: 9px;
  transition: opacity 120ms, border-color 120ms;
}
.env-empty:hover { opacity: 0.8; border-color: var(--accent); }
.env-foot { display: flex; justify-content: space-between; align-items: center; margin-top: 4px; }
.env-foot-left { display: inline-flex; align-items: center; gap: 2px; }
.env-add {
  border: none; background: transparent; cursor: pointer; font: inherit;
  font-size: 12px; font-weight: 550; color: var(--accent);
  padding: 5px 8px; border-radius: 6px; opacity: 0.85;
  transition: background 120ms, opacity 120ms;
}
.env-add:hover { background: color-mix(in srgb, var(--accent) 10%, transparent); opacity: 1; }
.env-save { flex: 0 0 auto; padding: 6px 24px; font-size: 12px; }
.env-save:disabled { opacity: 0.5; cursor: default; }
.group-hint {
  font-size: 11px; opacity: 0.45; line-height: 1.5;
  padding: 2px 4px 6px;
}

/* 分类管理 / 新建分类模态（复用 settings-dialog 外壳，宽度收窄） */
.cat-dialog { width: min(400px, calc(100vw - 40px)); max-height: min(48vh, 340px); }
.cat-body { max-height: calc(min(48vh, 340px) - 96px); gap: 2px; }

/* 「是否采用全局设置」开关（添加工具弹窗） */.switch-row { display: flex; align-items: center; gap: 10px; min-width: 0; }
.switch { position: relative; flex-shrink: 0; cursor: pointer; }
.switch input { position: absolute; opacity: 0; width: 0; height: 0; }
.switch .track {
  display: block; width: 36px; height: 21px; border-radius: 999px;
  background: var(--search-bg); border: 1px solid var(--glass-border);
  transition: background 180ms var(--spring-soft), border-color 180ms;
}
.switch .thumb {
  position: absolute; top: 2px; left: 2px; width: 17px; height: 17px;
  border-radius: 50%; background: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.25);
  transition: transform 200ms var(--spring);
}
.switch input:checked + .track { background: var(--accent); border-color: var(--accent); }
.switch input:checked + .track .thumb { transform: translateX(15px); }
.switch input:focus-visible + .track { box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 25%, transparent); }
.switch-label { font-size: 12px; font-weight: 550; }
.switch-sub { font-size: 10.5px; opacity: 0.5; margin-top: 1px; }
.seg {
  display: flex; gap: 2px; flex-shrink: 0;
  background: var(--search-bg);
  border: 1px solid var(--hairline);
  border-radius: 9px; padding: 3px;
}
.seg button {
  border: none; background: transparent; cursor: pointer;
  font: inherit; font-size: 12px; padding: 5px 13px;
  border-radius: 7px; white-space: nowrap; opacity: 0.65;
  transition: background 150ms var(--spring), opacity 150ms, transform 80ms ease-out;
}
.seg button:hover:not(.on) { opacity: 1; background: var(--card-hover); }
.seg button:active { transform: scale(0.95); }
.seg button.on {
  background: var(--accent); color: #fff; opacity: 1; font-weight: 600;
  box-shadow: 0 1px 4px color-mix(in srgb, var(--accent) 40%, transparent);
}
.settings-foot {
  padding: 10px 18px; flex-shrink: 0;
  border-top: 1px solid var(--hairline);
  font-size: 10.5px; opacity: 0.4;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}

/* Toast */
.toast {
  position: fixed; left: 50%; bottom: 40px; z-index: 40;
  transform: translateX(-50%) translateY(20px);
  background: var(--surface-2);
  -webkit-backdrop-filter: blur(24px); backdrop-filter: blur(24px);
  border: 1px solid var(--glass-border); border-radius: 999px;
  padding: 9px 18px; font-size: 12.5px; font-weight: 500;
  box-shadow: 0 16px 48px rgba(0,0,0,0.22);
  display: flex; align-items: center; gap: 8px;
  opacity: 0; pointer-events: none;
  transition: opacity 240ms var(--spring), transform 240ms var(--spring);
}
.toast.show { opacity: 1; transform: translateX(-50%) translateY(0); }
.toast .dot { width: 7px; height: 7px; border-radius: 50%; background: #30d158; flex-shrink: 0; }

/* 响应式 */
@media (max-width: 640px) {
  .rail { width: 64px; }
  .rail-item { font-size: 10px; }
  .grid { grid-template-columns: repeat(auto-fill, minmax(80px, 1fr)); }
  .searchbox .kbd { display: none; }
  .brand span { display: none; }
}
</style>
