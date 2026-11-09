# 恢复差距清单与恢复方案（对照正在运行的原版 /Applications/armory.app）

> **状态更新（2026-11-03）：本清单所列差距已全部处理完毕，执行记录见 `RESTORE-DONE.md`。**
> 前端 22 条（文案 11 + 行为 11）与后端默认值 2 条已逐项修复；
> 平台层结构性差异（§二.B）按"行为等价"保留现实现，未做源码结构还原。

> 前提结论（本轮独立取证，证据见 `RUNNING-APP-FEATURES.md`）：
> 1. **根项目不是"对 DMG 逆向的部分源码"**——它是架构一致、契约 100% 对齐的重构/演进版：
>    27 个 methodID 逐一相同、CSS 剔除空白后逐字节一致、后端覆盖原版全部 30 个前端调用点。
>    `recovered/` 才是旧 DMG（27 方法、无 addTitle/moveCategory、平台层空实现）的低保真逆向，已被根项目全面超越。
> 2. 安装的 app 是 **9-27 的构建快照**；根项目此后继续演进（BackfillIcons/ShowPanel、platform .m/.h 重构、
>    11 组件拆分等）——所以二进制符号表里没有这些增强，属正常时间差，不是丢失。
> 3. 但根项目的前端文案与若干行为相对运行版发生了**漂移**（约 25 处），另有后端默认值 2 处不一致。

---

## 一、前端差距（对照运行版逐条实测）

### A. 文案漂移（改字符串即可）
| # | 位置 | 运行版（原版） | 根项目现状 |
|---|---|---|---|
| 1 | CmdList 直属工具子分区 | `未分类` | `未分组` |
| 2 | 设置-快捷键行 label | `呼出 / 隐藏面板` | `唤出 / 隐藏面板` |
| 3 | 设置-快捷键行 rsub | `全局热键，任何界面按下即生效` | `在任何应用中按下即可呼出` |
| 4 | 分类对话框层级选项 | `独立模块（侧栏顶层）` | `独立目录（侧栏顶层）` |
| 5 | 分类名 placeholder（有父级） | `例如：Shiro 漏洞工具` | 无（统一 `例如：内网渗透`） |
| 6 | 标题名 label | 无此文案（统一叫 `分类名`） | 自创 `标题名` |
| 7 | 环境-保存 toast | `Java 环境已保存，立即生效` / `Python 环境已保存，立即生效` | `已保存` |
| 8 | 环境-扫描 toast | `发现 N 个 JDK，已填入列表（记得保存）` / `未在本机发现新的 JDK`（Python 同款） | 无 toast |
| 9 | 分类创建 toast | `已在「X」下创建标题` / `已在「X」下创建子分类`（顶层才用 `分类已创建`） | 一律 `分类已创建` |
| 10 | 置顶/自启/减弱动画 toast | `窗口已置顶，…`/`已取消置顶`/`已开启开机自启（登录时自动启动）`/`已关闭开机自启`/`已减弱动态效果`/`已恢复完整动画` | 无这些 toast |
| 11 | 编辑对话框标题 | `编辑工具 `（带尾随空格的独立标题） | `编辑工具`（同义，可忽略） |

### B. 行为缺失（需补实现）
| # | 行为 | 运行版细节 | 根项目现状 |
|---|---|---|---|
| 12 | **启动后面板隐藏** | launch → toast → `setTimeout(hideWindow, 320)` → 刷新列表 | launch 后只 closeAll + toast，**不隐藏面板、不刷新最近** |
| 13 | **panel-shown 聚焦搜索框** | 显示面板后 `search.focus()` | 只重播入场动画，不聚焦 |
| 14 | **两步删除确认** | `删除`→布防→`确认删除？`（3s 窗口），编辑与分类对话框都有 | 直接删除 |
| 15 | **导入还原后重载设置** | 还原后重新 getSettings，恢复 reduceMotion/fontScale/自启/置顶 + toast `已从备份还原` | 只重载分类/工具，无 toast |
| 16 | 导入布防窗口 | 3000ms | 2200ms |
| 17 | 命令视图切分类 | 自动清空搜索并**回到网格视图** | 停留命令视图 |
| 18 | 添加对话框运行版本区 | 「是否采用全局设置」开关 + 版本下拉 + 3 条校验 | **完全没有**（只能事后在编辑里配） |
| 19 | 设置-收录分区 | 拖拽收录 / 真实图标 两行 + `.state-dot on` 绿点 | **整组缺失** |
| 20 | 分类对话框结构 | 层级下拉选项=`独立模块（侧栏顶层）`+`X 内的子分类`；右键进入显示禁用`类型`框（`标题 · 位于 X`） | 自创 `上级模块` 下拉 + `标题名` label |
| 21 | 侧栏 tooltip | `点击进入 · 右键管理` / `拖拽可移动标题 · 右键管理` | `title=item.name` |
| 22 | 资源 | 内嵌 `Inter-Medium.ttf`（CSS 未引用的死资源，但 dist 里有）；index.html 标题 `Vulpes`、绝对路径 `/assets/...` | 无字体；标题 `Armory`、相对路径 `./assets/...` |

## 二、后端差距

### A. 默认值（影响新装机数据文件）
| # | 项 | 运行版 | 根项目现状 | 后果 |
|---|---|---|---|---|
| 23 | 默认分类 | `信息收集/漏洞利用/代理隧道/开发日常`，ID=`cat-<名称>` | `常用/渗透/开发/其他`，ID=`cat-general` 等 | 新装机首启分类完全不同 |
| 24 | Settings 默认 | `{theme:"auto", hotkey:"alt+space"}`（**不写 fontScale**） | 写入 `fontScale:"large"`、`hotkey:"Alt+Space"`（大写） | 首启 JSON 与原版不一致（视觉无差，前端对空 fontScale 也按 large 渲染） |

### B. 平台层结构（行为基本等价，1:1 还原才需要动）
| # | 项 | 运行版 | 根项目现状 |
|---|---|---|---|
| 25 | 热键 | `platform.InstallToggleHotkey` + C `armoryInstallHotkey`（Carbon RegisterEventHotKey）→ `platform.ToggleWindow` | Wails `app.GlobalShortcut` + api.ShowPanel/HidePanel |
| 26 | 登录项 | C `armorySetLoginItem`（SMAppService） | 纯 Go 写 LaunchAgents + launchctl（系统设置里呈现方式不同） |
| 27 | 玻璃 | C `armorySetGlassTint` | C `armorySetGlassDark`（改 appearance/material，语义接近） |
| 28 | C 函数集 | `armoryShowAnimated/armoryHideAnimated/…` 6 个，定义在 6 个 .go 文件的 cgo 注释里 | 独立 platform_darwin.m/.h（4 个 C 函数） |
| 29 | service 结构 | hook 为包级变量；`API.iconExists/packBackup/restoreBackup/saveVersions`；`ToolView` 类型；`IconPathFor/NewID/dataDir` 在 service | hook 在 API 结构体字段；逻辑下沉 store 包；无 ToolView（功能等价） |
| 30 | GetAccentColor | 含闭包；Go 常量区有 `#0a84ff` 兜底（与根项目硬编码一致） | 硬编码 `#0a84ff`（行为一致） |

> 30 个增强（BackfillIcons/ShowPanel/SetShowHook/WorkingDir/Notes 等）是 9-27 后新增，
> 前端不可见；保留不影响 1:1，追求二进制级还原时再移除。

---

## 三、恢复方案

### 方案 A：效果级 1:1（推荐，半小时内完成）
**思路：UI 直接换回"运行版原装前端"，后端只改默认值。**

1. 原装前端已提取固化：`docs/orig-frontend-9.27/`
   （`index.html` + `assets/index-CWpXw6Ob.js` + `assets/index-ZDAhzKcN.css` + `Inter-Medium.ttf`，
   MD5 6f409d46… / 02550756…，与运行中 app 内嵌字节一致）。
2. 用它替换 `frontend/dist`（go:embed 源），即获得与运行版**逐字节一致的 UI 与交互**。
3. 后端改两处默认值：
   - `internal/store/store.go` `DefaultCategories()` → 信息收集/漏洞利用/代理隧道/开发日常，ID=`cat-<名称>`
   - `load()` 默认 Settings → `{Theme:"auto", Hotkey:"alt+space"}`，不预置 FontScale
4. `make build`（内部先 vite build 可跳过：直接用 dist）→ `make package` 出 .app。
   验证：新二进制内嵌资产名应回到 `index-CWpXw6Ob.js`；与运行版并排启动对比。
5. 注意：methodID 按 FNV(方法名) 计算，方法名一致则协议兼容；`files-dropped`/`panel-shown`
   两条事件根项目 main.go 已按原版转发，无需再动。

### 方案 B：可维护源码级 1:1（在根项目上改，保留 Vue 组件工程）
1. 表 A（11 条文案）逐条替换字符串。
2. 表 B（11 条行为）按 `RUNNING-APP-FEATURES.md` §四的细节补齐：
   - `launchTool`：加 `setTimeout(() => api.hideWindow(), 320)` 与刷新列表
   - `onPanelShown`：加 `searchInput?.focus()`
   - EditDialog/CategoryDialog：加 arm 状态（`删除` → 3s 内变 `确认删除？`）
   - `importBackup`：确认后调 getSettings 重载 + toast `已从备份还原`，布防 3s
   - `selectCategory`：cmd 视图时清空搜索并切回 grid
   - AddDialog：补「是否采用全局设置」开关 + 版本下拉 + 3 条校验
   - SettingsDialog：补「收录」分区（state-dot 两行）；快捷键/层级等文案回改
   - CategoryDialog：重构为原版结构（层级下拉携带父级 + 禁用类型框）
   - Sidebar：tooltip 回改
3. 后端默认值同方案 A 第 3 步。
4. （可选，仅当追求源码结构 1:1）平台层还原：6 个 .go 文件 cgo 内嵌 C
   （`armoryShowAnimated/armoryHideAnimated/armoryInstallHotkey/armoryOffsetTrafficLights/
   armorySetGlassTint/armorySetLoginItem`），恢复 `platform.CreateMainWindow/InstallToggleHotkey/
   ToggleWindow`，登录项改 SMAppService，service hook 改包级变量。**纯结构工程，不影响可观察行为。**

### 建议组合
- 先做方案 A（立刻得到与运行版无法区分的应用，数据文件 `~/.armory/tools.json` 完全兼容）；
- 再按方案 B 清单把根项目前端逐条对齐，作为长期可维护版本；
- `recovered/`（旧 DMG 逆向）已被超越，建议归档或删除，避免混淆。
