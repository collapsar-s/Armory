# 正在运行的 Armory（/Applications/armory.app，9-27 构建）功能全景

> 证据来源（全部实测，非推测）：
> 1. 从运行中二进制解析 Go embed FS，**原样提取内嵌前端**：`index.html`(413B)、
>    `index-CWpXw6Ob.js`(133,135B)、`index-ZDAhzKcN.css`(24,035B)、`Inter-Medium.ttf`(315,132B)
>    —— 已固化在本目录 `docs/orig-frontend-9.27/`。
> 2. 从二进制 pclntab 提取**全部 208 个 armory 自有 Go 符号**（strip 后仍保留函数名）。
> 3. 读取 live 数据 `~/.armory/tools.json`（由原版应用写出）。
> 4. 逐字符串分析前端 bundle 的 46 条中文文案与交互上下文。
>
> 版本指纹：CFBundleIdentifier `com.armory.launcher` · v0.1.0 · macOS 26+
> （Info.plist 文案 "Armory - 渗透作业的狐狸洞"；内嵌 index.html 标题为旧代号 **Vulpes**）

---

## 一、应用形态与窗口

- **Wails v3 beta.26 + Go 1.26.5 + Vue 3**，Spotlight 式无边框悬浮面板，Liquid Glass 毛玻璃。
- 主窗口 880×560（min 640×420），Frameless + MacBackdropLiquidGlass + 44px 隐形标题栏，圆角。
- 红绿灯（关闭/最小化/缩放）保留系统原生位置与行为，启动后校正一次（350ms 后再校正一次）。
- **显示/隐藏动画**：cgo 调 C 函数 `armoryShowAnimated` / `armoryHideAnimated`
  （NSAnimationContext 对 alphaValue + frame 缩放做 0.18s 淡入 / 0.15s 淡出，缩放 2%）。
- **全局热键**：平台层 C 函数 `armoryInstallHotkey`（Carbon `RegisterEventHotKey`）注册
  ⌥Space，回调 `armoryHotkeyFired` → `platform.ToggleWindow` 切换显示/隐藏（不走 Wails GlobalShortcut）。
- **开机自启**：C 函数 `armorySetLoginItem`（macOS 13+ `SMAppService`）。
- **玻璃明暗**：C 函数 `armorySetGlassTint`，前端主题切换时调用。
- **文件拖放**：后端监听 `WindowFilesDropped` → 以自定义事件 `files-dropped` `{files:[...]}` 转发前端
  （二进制 Go 区同时含 `files-dropped`、`panel-shown` 字符串，证实为后端发出）。

## 二、后端符号清单（pclntab 实测，与前端契约 1:1）

- `service.API` 绑定方法（前端经 methodID 调用，30 个调用点）：
  `ListCategories, ListTools, AddToolByPath, UpdateTool, DeleteTool, AddCategory, AddSubCategory,
  AddTitle, MoveCategory, RenameCategory, DeleteCategory, Launch, GetIcon, GetSettings, SetTheme,
  SetReduceMotion, SetLaunchAtLogin, SetAlwaysOnTop, SetFontScale, GetAccentColor, SetGlassTheme,
  SaveJavaVersions, SavePythonVersions, DetectJavaVersions, DetectPythonVersions, ListRecentTools,
  GetToolReadme, ExportBackup, ImportBackup, HidePanel`
  + hook 注入器：`SetDialogHooks, SetGlassHook, SetHideHook, SetTopHook, SetLoginHook`
  + API 内部：`iconExists, packBackup, restoreBackup, saveVersions`
  （**没有** BackfillIcons / ShowPanel / SetShowHook —— 那些是后续演进的增强）
- `service.Registry`：`AddCategory, AddSubCategory, AddTitle, AddTool, DeleteCategory, DeleteTool,
  GetSettings, GetTool, ListCategories, ListRecent, ListTools, MarkRun, MoveCategory, RenameCategory,
  ReplaceAll, UpdateSettings, UpdateTool, indexOfCat, saveLocked`
- `service.Launcher`：`Launch, javaBin, pythonBin`
- 包级：`DetectType, ExpandPath, ExtractAppIcon, IconPathFor, NewID, SplitArgs, ToolView,
  defaultCategories, dataDir, homeDir, decodeUniEscapes, containsName, descendantIDs, firstCategoryID,
  rootID, findVersion, itoa, javaEnv, majorVersion, normVerName, prependPath, prependPathEnv,
  resolveVersionPath, baseName, bundleName, fileExists, glassHook, hideHook, topHook, loginHook,
  openFileHook, saveFileHook`（hook 为**包级变量**）
- `platform`（6 个 .go 文件，cgo 内嵌 C）：`CreateMainWindow, InstallToggleHotkey, ToggleWindow,
  ShowWindowAnimated, HideWindowAnimated, OffsetTrafficLights, SetGlassDark, SetLaunchAtLogin,
  armoryHotkeyFired` + C 桥：`armoryShowAnimated, armoryHideAnimated, armoryInstallHotkey,
  armoryOffsetTrafficLights, armorySetGlassTint, armorySetLoginItem`
- `main`：main + func1..func7（hide/glass/top/login/对话框等 hook 闭包 + 拖放转发 + 红绿灯校正）

## 三、数据模型（live tools.json 实测 + 反射指纹）

- 顶层：`{"Tools":…, "Categories":…, "Settings":…}`（大写键，Go 默认序列化）
- `Category`：`{id, name, order}`（+ 可选 `parentId`/`kind`，title 类型才有 kind）
  **默认分类 = 信息收集 / 漏洞利用 / 代理隧道 / 开发日常，ID 方案 `cat-<名称>`**
- `Tool`（前端实际读取）：`id, name, path, args, type(app|jar|py|sh|bin), categoryId, envVersion, hasIcon`
  （runCount/lastRunAt 供"最近"排序，前端不读；workingDir/notes/iconPath 前端不读）
- `Settings`（live 实测）：`theme:"auto"`, `hotkey:"alt+space"`, `javaVersions[]`, `pythonVersions[]`
  （reduceMotion/alwaysOnTop/fontScale/launchOnLogin 按需出现；**首次默认不写 fontScale**）

## 四、前端功能与交互细节（从 bundle 逐条核实）

### 主界面
- 侧栏：品牌区 + 「全部」「最近」+ 两级分类树（子分类缩进，`.rail-sub` 前缀 `·`）+ 底部「＋ 新建」。
  - 条目 tooltip：目录 `点击进入 · 右键管理`；标题 `拖拽可移动标题 · 右键管理`
  - 右键分类 → `.ctx-menu` 上下文菜单（新建子分类/新建标题/重命名/删除）
  - 标题（kind=title）可拖拽移动（限同一一级目录范围内，moveCategory）
  - 工具卡片拖到侧栏分类 → 改归属（`.drop-ok` 高亮）
- 顶栏：搜索框 placeholder `搜索工具，或直接键入启动…`，右侧 ⌥Space kbd 提示 + 添加工具 + 设置按钮。
  - 搜索命中 `<mark>` 高亮（名称/路径），键盘 ↑↓ 选择、Enter 启动、⌘Enter 编辑
- 网格视图：工具卡片（图标/名称/类型 emoji），尾部 `添加` 卡（`拖入或点击`）；空态提示
  `把 .app / .jar / .py / .sh 拖进面板即可收录`；拖拽悬停时 `松手即可收录到「分类名」`
- 命令视图：按模块分区（`.sec-head`）+ 标题子分区（`.sub-sec`）；**直属工具归入「未分类」子分区**；
  切换分类时若在命令视图 → 自动清空搜索并**回到网格视图**
- 右键工具 → 直接打开编辑对话框；长名称截断时悬浮 `.name-tip`（300ms 延迟）
- Esc 优先级：编辑→添加→分类→设置→（命令视图清空搜索）→隐藏面板；Alt+Space 不拦截
- panel-shown 事件 → 重播入场动画 + **聚焦搜索框**
- 启动工具 → toast `已启动 X` → **320ms 后自动隐藏面板** → 刷新列表（更新"最近"）

### 添加工具对话框
- 字段：工具名称（`选填，留空自动识别`）/ 路径（`必填，如 /Applications/X.app 或 ~/Tools/x.jar`）/
  启动参数（`选填，如 -sV -p 1-65535`）/ 所属分类（显示 label 计算值）
- **运行版本区**：开关「是否采用全局设置」（.switch）；开启后 jar→Java 版本、py→Python 版本下拉
  （选项含 `使用「设置 → Java 环境」中配置的版本` / `使用系统默认解释器`）
- 校验 toast：`路径不能为空` / `请先在「设置 → 运行环境」中配置版本` / `请选择一个全局版本`
- 成功：`已收录 <名称>`

### 编辑工具对话框
- 同上字段 + Java 版本/Python 版本下拉（`自动（系统默认）`）+ 说明文档区
  （getReadme 读取工具目录下 README.md/README.txt/说明.txt/说明.md 等，Markdown 渲染，可折叠）
- **两步删除**：`删除` → 点击后变 `确认删除？`（3 秒窗口）→ 再点执行
- 按钮：删除 / 取消 / 保存（`已保存`）

### 分类对话框
- 从侧栏「＋」进入：`所属层级` 下拉 = `独立模块（侧栏顶层）` + 每个一级模块一项 `X 内的子分类`
  （**没有独立的"上级模块"下拉**，父级直接由下拉选项携带）
- 从右键「新建子分类/新建标题」进入：显示禁用输入框 `类型` = `标题 · 位于 X` / `子分类 · 位于 X`
- 名称 label：`分类名`（新建）/ `重命名`（编辑）；placeholder：有父级 `例如：Shiro 漏洞工具`，
  无父级 `例如：内网渗透`
- 编辑态提示：`修改后保存立即生效；删除后，其中的工具会自动归入上级分类或其他模块`
- 删除同样两步确认（`确认删除？`）
- 校验/提示：`分类名不能为空`、`标题下不能再创建子目录`、`目录最多支持两级，无法在二级目录下再新建子目录`、
  `标题下不能再创建任何内容`、`只能在同一个一级目录范围内移动`
- 成功 toast：`已在「X」下创建标题` / `已在「X」下创建子分类` / `分类已创建`

### 设置对话框
分区（含 state-dot 行的静态 HTML 组）：
- **通用**：开机自启（开/关按钮对，toast `已开启开机自启（登录时自动启动）`/`已关闭开机自启`）、
  窗口置顶（`面板始终浮于其他窗口之上（Spotlight 式），关闭后与其他窗口正常争抢层级`，
  toast `窗口已置顶，始终浮于其他窗口之上`/`已取消置顶`）
- **快捷键**：`呼出 / 隐藏面板` · `全局热键，任何界面按下即生效` · ⌥Space kbd-chips（固定，不可改）
- **外观**：主题（自动/浅色/深色，切换调 setGlassTheme）、界面字体（标准/大/特大，zoom 1/1.1/1.2，
  **前端兜底：fontScale 为空也按 large 渲染**）、减弱动态效果（toast `已减弱动态效果`/`已恢复完整动画`）
- **收录**（静态 HTML 行 + 绿色 state-dot）：`拖拽收录`（把 .app / .jar / .py / .sh 拖进面板）、
  `真实图标`（自动提取 .app 原版图标并缓存）
- **Java 环境 / Python 环境**：版本行（版本名 + 可执行文件路径 + 删除此版本）、`＋ 添加版本`、
  `⟳ 扫描本机`；保存 toast `Java 环境已保存，立即生效` / `Python 环境已保存，立即生效`；
  扫描 toast `发现 N 个 JDK，已填入列表（记得保存）` / `未在本机发现新的 JDK`（Python 同款）
- **数据**：导出备份（zip，toast `已导出到 <路径>`）、导入还原（**两步确认**，首次点击布防 3 秒，
  确认后覆盖全部数据；完成后 toast `已从备份还原` 并**重载设置**（动画/字号/自启/置顶状态全部恢复））
- 页脚：`Armory v0.1.0 · 个人专用 · 数据存储于 ~/.armory`

### 启动器语义（launcher.go 行为）
- app → `open -a <path> [args]`；jar → 配置版本 java（含 JAVA_HOME + PATH 前置注入）或系统 java；
  py → 配置版本 python3（PATH 前置）或系统默认；sh → `/bin/sh <path>`；bin → 直接执行
- args 按 shell 风格切分（支持单双引号）；路径支持 `~` 与环境变量展开
- 启动成功 → MarkRun（runCount+1、lastRunAt 刷新）→ 出现在「最近」（默认 n=12）

### 图标
- `.app` 收录时经 `sips` 提取原图标缓存到 `~/.armory/icons/<toolID>.png`，`getIcon` 以 base64 data URL 下发
