# 恢复执行记录（2026-11-03）

> 按前一轮分析（`RESTORE-GAPS.md` / `RUNNING-APP-FEATURES.md`）执行了方案 A 与方案 B，
> 全部完成并验证通过。
>
> **第二轮补齐（同日晚）**：对照运行版二进制又核实并修复了 7 处后端差异（见 §六），
> 其中**导入备份流程在根项目里原本是跑不通的**（后端要求 path 参数、前端无参调用），
> 已按原版语义重构为后端自行弹对话框。

## 一、方案 A：效果级 1:1（已完成 ✅）

1. 从运行中二进制提取的原版前端（`docs/orig-frontend-9.27/`，MD5 与运行版内嵌字节一致）
   已替换 `frontend/dist`，并修正两处后端不一致后构建为 **`bin/armory.orig-ui`**——
   UI 与交互和正在运行的原版逐字节一致。
2. 构建验证：内嵌资产名回到 `index-CWpXw6Ob.js` / `index-ZDAhzKcN.css` + `Inter-Medium.ttf`；
   默认分类四类指纹齐备；`go vet` 干净；`go test` 通过；冒烟运行正常，`~/.armory/tools.json` 未受影响。

## 二、方案 B：可维护源码对齐（已完成 ✅）

### 后端（3 处）
| 文件 | 改动 |
|---|---|
| `internal/store/store.go` | 默认分类改为 信息收集/漏洞利用/代理隧道/开发日常（ID `cat-<名称>`）；首次默认 Settings 只写 `theme:auto` + `hotkey:alt+space`（不再预置 fontScale）；新建目录不再写 `kind:"dir"`（与原版 JSON 输出一致）；`UpdateTool` 的 args 为空时保持原值（避免拖拽改归属清掉启动参数） |
| `internal/service/api.go` | `GetIcon` 改为返回**裸 base64**（原版语义：前端拼接 `data:image/png;base64,` 前缀）；`AddToolByPath` 未指定分类时归入**第一个**分类（原版 firstCategoryID 语义）；移除后端版本名校验（原版由前端把关，文案在前端） |

### 前端（8 个文件）
| 文件 | 对齐内容 |
|---|---|
| `App.vue` | 启动后 320ms 自动收起面板 + 全量刷新（最近/分类/工具）；panel-shown 后聚焦搜索框；Esc 优先级与原版 Qt() 一致（编辑→添加→分类→设置→命令视图回网格→隐藏），Alt+Space 放行；命令视图切分类自动回网格并清空搜索；分类分区「未分类」；拖拽改归属增加"同一一级目录范围内"校验 + `标题已移动到` 文案；内部拖拽不触发"松手收录"提示；图标 data URL 前缀前端拼接；搜索命中类型 emoji；滚动收起长名提示；设置开关 toast 全套（置顶/自启/减弱动画）；扫描按 path 合并去重 + `发现 N 个 JDK…` toast；保存环境校验与 `…已保存，立即生效` toast（含原版三元表达式原样行为）；导出备份/导入还原 toast，导入布防 3s 且还原后重载并应用全部设置；分类创建 toast（已在「X」下创建标题/子分类）并切换到父级；重命名变更才提交 + `已保存`；右键菜单点击时预校验（二级目录下不能再建子目录）；网格空白右键打开当前分类菜单；编辑时自动补载运行环境列表 |
| `AddDialog.vue` | 重写：所属分类子级显示「父 / 子」路径（标题加 ◇ 前缀）；新增「运行版本」开关（是否采用全局设置）+ 全局版本下拉（含 `尚未在设置中配置 …版本` 空选项）；`路径不能为空` / `请先在「设置 → 运行环境」中配置版本` / `请选择一个全局版本` 三条校验；envVersion 随表单提交 |
| `EditDialog.vue` | 两步删除确认（删除 → 布防 3s → `确认删除？`）；标题图标显示真实 .app 图标（有缓存时） |
| `CategoryDialog.vue` | 重构为原版结构：层级下拉选项 = `独立模块（侧栏顶层）` + `X 内的子分类`（父级由选项携带）；右键进入显示禁用「类型」框（`标题 · 位于 X`）；placeholder 按层级区分（`例如：Shiro 漏洞工具` / `例如：内网渗透`）；移除自创的「上级模块」下拉与「标题名」标签；编辑删除两步确认 |
| `SettingsDialog.vue` | 分区顺序改为原版：通用 → 外观 → Java 环境(+提示) → Python 环境(+提示) → 快捷键 → **收录（拖拽收录/真实图标，绿色 state-dot）** → 数据 → 页脚；快捷键文案 `呼出 / 隐藏面板` / `全局热键，任何界面按下即生效`；补充 Python group-hint |
| `CmdList.vue` | 分区头交互：点击进入、右键分类菜单、标题可拖拽（tooltip `点击进入 · 右键管理` / `拖拽可移动标题 · 右键管理`）；路径行改为完整路径 `父 / 子 · 路径` |
| `ToolGrid.vue` | 网格空白右键打开分类菜单；卡片右键阻断冒泡；dragend 归位 |
| `TopBar.vue` | 搜索框固定 `id="search-input"`（供 panel-shown 聚焦） |

## 三、验证（实测）

- `bash -lc 'source ~/.nvm/nvm.sh && cd frontend && vite build'` ✓（新文案 11/11 命中，旧漂移文案 0 残留）
- `go vet ./...` ✓ · `go test ./internal/...` ✓ · 冒烟运行 ✓（进程存活、无报错、数据无恙）
- `wails3 task package` ✓ → **`bin/armory.app`**（ad-hoc 签名，Info.plist 与原版一致）

## 四、交付物

| 产物 | 说明 |
|---|---|
| `frontend/src/**` + `internal/**` | 对齐后的可维护源码（最终形态） |
| `bin/armory` / `bin/armory.app` | 方案 B 构建（源码对齐版，可直接替换 /Applications/armory.app 使用） |
| `bin/armory.orig-ui` | 方案 A 构建（内嵌原版前端字节，UI 与运行版完全一致） |
| `docs/orig-frontend-9.27/` | 运行版原装前端产物（含字体），可随时还原 dist |
| `docs/RUNNING-APP-FEATURES.md` / `docs/RESTORE-GAPS.md` | 功能全景与差距清单（差距清单所列 30 项已全部处理，平台层结构性差异按"行为等价"保留现实现） |

## 五、说明与遗留

1. **数据完全兼容**：新旧构建读写同一份 `~/.armory/tools.json`；live 数据（信息收集等四类）即原版默认分类。
2. 平台层保留根项目现有实现（Wails GlobalShortcut 热键 / LaunchAgents 登录项 / `armorySetGlassDark` 玻璃），
   与原版 Carbon/SMAppService 方案**行为等价**；若追求源码结构 1:1 可按 `RESTORE-GAPS.md` §二.B 再做。
3. 保留的增强（`BackfillIcons`/`ShowPanel` 等）对前端不可见，不影响 1:1 效果。
4. `recovered/`（旧 DMG 逆向）已被全面超越，建议归档。

## 六、第二轮补齐：二进制核实的 7 处后端差异（已全部修复 ✅）

全部来自对运行版二进制的字符串取证（逐条在 Go 常量区命中原文）：

| # | 差异 | 原版行为（二进制证据） | 修复 |
|---|---|---|---|
| 1 | 图标缓存目录 | `~/.armory/cache/icons`（`cache/icons` 字符串 ×2） | `store.IconDir()` 改为 `DataDir()/cache/icons` |
| 2 | 导入备份签名 | 无参方法，后端自己弹对话框，标题 `选择备份包（.zip）`；对话框缺失报 `文件对话框不可用` | `API.ImportBackup()` 改为无参 + OpenFile 对话框（**原根项目 ImportBackup(path) 配无参前端调用根本跑不通**） |
| 3 | 导出备份流程 | 保存对话框（标题 `导出备份`）+ 默认名 `armory-backup-20060102-150405.zip`；对话框缺失报 `导出对话框不可用` | `API.ExportBackup()` 重构：buildBackup 临时 zip → SaveFile → 拷贝落盘，取消返回空串 |
| 4 | zip 结构 | 数据条目是 `tools.json`，缺失报 `备份包缺少 tools.json，不是有效的 Armory 备份`（不是 data.json） | 打包改写 `tools.json`；还原同时兼容 `icons/` 与 `cache/icons/` 两种图标前缀 |
| 5 | 收录/启动路径校验 | `路径为空`、`路径不存在: %s` | AddToolByPath 空路径/不存在即拒；launcher 五种类型全部校验，错误文案统一 |
| 6 | 解释器兜底链 | `/usr/libexec/java_home`、`/usr/local/bin/python3` | javaBin：PATH 失败后经 java_home 定位 JDK；pythonBin：追加 `/usr/local/bin/python3` 候选 |
| 7 | 分类守卫与回退 | `至少保留一个分类`；删除后工具"归入上级分类或其他模块" | DeleteCategory：仅剩 1 个分类时拒绝；工具优先归入被删分类的上级，无上级才归第一个其它分类 |

另同步修正（同批二进制证据）：`NewRegistry` 错误文案 `读取工具库失败: %w`、解析失败文案 `tools.json 损坏: %w`、说明文档候选新增 `使用说明.md`。

**验证**：13 项新指纹在两版二进制（`bin/armory` / `bin/armory.orig-ui`）中全部命中；`go vet` 干净；
`go test ./internal/...` 通过（backup 测试已随 cache/icons 与 buildBackup 签名同步更新）；
A/B 两版冒烟运行正常；`bin/armory.app` 已用最新源码重新打包。

## 七、第三轮：测试与优化（2026-11-03 深夜，已全部完成 ✅）

### 新增回归测试（`internal/service/restore_align_test.go`，9 项全过）
覆盖第二轮全部对齐行为：图标目录 `cache/icons`、收录路径校验（路径为空/不存在）、
分类删除守卫（至少保留一个分类 + 工具优先归入上级）、备份 zip 结构（tools.json 条目 + icons/），
导出/导入对话框全流程（默认文件名 `armory-backup-<ts>.zip`、取消返回空串、hook 缺失报原版文案、
兼容 `cache/icons/` 前缀还原）、readme 候选含使用说明.md、启动路径校验、默认分类与原版逐字一致。

### 浏览器走查（vite dev + 浏览器自动化，发现并修复 2 处真实缺失）
| # | 发现 | 修复 |
|---|---|---|
| 1 | **命令视图无法进入**：原版是"搜索驱动视图切换"（`It()`：输入搜索词自动进命令视图、清空自动回网格），根项目完全没有这条链路 | App.vue 增加 `watch(query)`：有词→cmd、清词→grid、复位键盘选择、cmd 下刷新（对齐原版） |
| 2 | 分类名空值校验原版是**前端预检**（原版 JS 有 `分类名不能为空`），根项目依赖后端报错 | `submitCat` 提交前预检（真机文案一致，行为逐字对齐） |

**走查通过项**：主界面渲染、搜索↔命令视图双向切换、命令视图空态文案、
设置对话框分区顺序（通用→外观→Java→Python→快捷键→收录→数据→页脚）与全部文案、
添加对话框字段/占位符/「是否采用全局设置」开关联动（提示文案、全局版本下拉、空配置选项）、
校验链（路径不能为空 → 请先配置版本 → 请选择一个全局版本）、分类对话框结构（层级下拉/占位符/创建按钮，
旧版"上级模块/标题名"零残留）、空名预检。
（备注：走查中 `fill("")` 与 `zoom:1.1` 是自动化工具的兼容性怪癖，非应用缺陷——真实键入路径全部正常。）

### 优化
- `Makefile`：新增 `make orig-ui`（一键构建原装 UI 版）与 `make test`（`go test -count=1`），help 同步
- `EditDialog` / `CategoryDialog`：删除布防定时器在组件卸载时清理（防泄漏）
- 交付物全量重建：`bin/armory`（源码版）、`bin/armory.orig-ui`（原装 UI 版）、`bin/armory.app`（最新打包）

### 最终验收
`go vet` ✓ · `go test -count=1` ✓（含 9 项新回归）· 前端构建 ✓ · 双二进制指纹 ✓ ·
真机启动 + 截屏验收 ✓（四分类侧栏/搜索框+⌥Space/添加卡片/空态提示全部正常）·
测试进程清理干净（仅保留用户正在运行的原版实例）。

## 八、第四轮：运行时扫描对齐（2026-11-03，用户实测反馈 ✅）

**用户报告**：重建版点「⟳ 扫描本机」多扫出一条 `Java 26.0.2 / /usr/bin/java`，原版扫不出。

**根因**：根项目的 `DetectJavaVersions` 多了一步"扫 PATH 里的 java"；`/usr/bin/java` 是 macOS
的**系统存根**（转发到默认 JDK 执行），`java -version` 会如实报出默认 JDK 版本（26.0.2），
于是存根被当成独立版本收录。原版扫描**从不查 PATH**。

**修复**（依据 9-26 会话快照中的原版 `envprobe.go` 源码，逐字对齐）：
1. Java 扫描 = 首选 `/usr/libexec/java_home -V` 解析（含其引号解析的原版行为），
   无结果则兜底扫 `/Library/Java/JavaVirtualMachines` **与 `~/Library/Java/JavaVirtualMachines`**
   （用户级目录，根项目原先漏了）；**没有 PATH 步骤**——存根永远扫不出来。
2. 目录扫描命名 = `Java + 完整目录名`（保留 `.jdk` 后缀，如 `Java jdk-26.jdk`；
   根项目原先错误地剥掉了后缀，会导致扫描出的名字与原版历史数据不一致）。
3. `majorVersion` 按原版实现（两位小数："21.0.5"→"21.0"、"3.12.7"→"3.12"，
   这正是历史数据里 `Python 3.11` 这种命名的来源）。
4. Python 扫描按原版算法：固定位置（/usr/bin、/usr/local）+ LookPath + 三个 glob
   （homebrew/pyenv），实际执行 `--version` 命名，按名称**降序**排序（与历史数据
   3.9/3.8/3.11 的顺序吻合）。
5. `fileExists` 对齐原版：目录不再算"存在"。

**验证**：新增回归测试（扫描无 `/usr/bin/java`、命名形态、majorVersion 原版行为）全过；
实测扫描输出与原版历史数据**逐字一致**（Java 7 条同名字同路径；Python 命名与排序一致）；
全量测试通过；双二进制与 `bin/armory.app` 已重建。用户数据无需清理（存根条目未被保存）。

## 九、第五轮：收录/启动/图标链路修复（2026-11-03，用户实测反馈 ✅）

**用户报告**：收录 `.app` 工具后显示类型 emoji 而非真实图标。

**根因**：上一轮把 `fileExists` 对齐为"目录不算存在"，但 `.app` 包本身就是目录——
收录校验、启动校验、图标提取三处对 `.app` 的判断全部被误拒（第四轮的二进制在真机上
无法收录 .app；截图中的工具是在双实例混跑下由旧状态渲染的）。

**修复**（依据 9-26 快照原版源码，逐项对齐）：
1. **收录流**（`AddToolByPath`）：TrimSpace → 空报 `路径为空` → **剥掉拖拽的 `file://` 前缀**
   （原版有、重建版漏了）→ `os.Stat` 校验（**目录合法**）→ 默认名 baseName/.app 用
   bundleName → 手动名优先 → 空分类归第一个分类 → 落库后对 `.app` 尽力提取图标；
   路径按原版**存原始输入**（不预展开，使用时再展开）。
2. **图标提取**（`iconextract.go`）：按原版重写——`defaults read` 读 Info.plist 声明的
   `CFBundleIconFile`（这才是应用的真图标），兜底取 Resources 第一个 icns，`sips` 直接
   转写到目标路径；不再要求 `.app` 后缀、不再随意抓第一个 icns。
3. **启动**（`launcher.go`）：校验改回 `os.Stat`（目录合法）；`.app` 用 `open <path>`
   （原版不带 `-a`），有参数时以 `--args` 分隔；`DetectType` 补 `.command`/`.zsh` → sh。
4. **参数语义**：后端 `UpdateTool` 恢复原版"args 整体覆盖（允许显式清空）"，
   前端拖拽改归属时**带上原 args**——编辑框可清空参数，拖拽也不会误清（原版拖拽会
   误清参数，属原版缺陷，此处以前端补参方式两全）。
5. **删除工具**时同步删除图标缓存（原版行为）；移除不再使用的 SetToolIcon/IconExists/statIcon。
6. 新增回归：收录 .app 目录放行、file:// 前缀剥离、**真实包提取**（对本机
   `/Applications/云资产管理工具.app` 实测提出合法 PNG）、fileExists 目录语义保留于
   envprobe 等文件型判断。

**⚠️ 双实例数据互踩提醒**：原版与重建版同时运行时共用 `~/.armory/tools.json`，
后落盘者整文件覆盖（原版设计即无跨进程锁）。用户已收录的工具因此丢失过一次。
**请只运行一个实例**：退出 /Applications/armory.app（旧版），改用 bin/armory.app
（或将其替换进 /Applications）。

## 十、第六轮：平台层结构还原（2026-11-06，用户实测验收 ✅）

按用户要求把 §五.2 遗留的"平台层结构性差异"也还原掉。核心材料：从会话恢复产物抢救的
**9-26 原版源码快照**（已落盘固化 `docs/orig-src-snapshot-9.26/`，含 platform 6 个
cgo .go + 原版 main.go；/tmp 重启即失效，抢救时 MD5 核对一致）。

### 平台层重组（internal/platform/，8 文件）
| 文件 | 内容 | 校准点 |
|---|---|---|
| `window_options.go` | 窗口配置（三构建变体共享） | 9-27 实测值：880×560/min 640×420、圆角 18、44px 隐形标题栏（不用快照旧值 760×520/圆角 24） |
| `window.go` | `CreateMainWindow(app)`（darwin cgo 入口） | 同上 |
| `window_anim.go` | `armoryShowAnimated/armoryHideAnimated`（cgo 内嵌 C，NSAnimationContext alpha+frame 动画） | 9-27 实测：0.18s EaseOut / 0.15s EaseIn + 2% 缩放（快照旧值 0.22/0.16 纯 alpha 不采） |
| `glass_tint.go` | `SetGlassDark`：`NSClassFromString(@"NSGlassEffectView")` + `setTintColor:`，深色 tint {22,24,30,α0.78} | **修复真实缺陷**：旧实现找 NSVisualEffectView（macOS 26 Liquid Glass 下不存在），深色主题玻璃实际无效果；快照原样移植 |
| `trafficlights.go` | `OffsetTrafficLights(w)`：恢复三键可见/可用（Wails frameless 会隐藏它们），内部 200ms×5s 重试 | 行为同 9-27（"启动后校正一次"） |
| `hotkey.go` | `InstallToggleHotkey` + `ToggleWindow` + `//export armoryHotkeyFired`：Carbon `RegisterEventHotKey`，signature `'vulp'`，⌥(optionKey)+Space(49) | 快照原样，错误文案 `RegisterEventHotKey failed, OSStatus=%d` 与原版二进制逐字一致 |
| `loginitem.go` | `SetLaunchAtLogin(app, enabled)` → Wails `app.Autostart` | **二进制取证修正**：9-27 原版登录项不是快照的自写 cgo SMAppService，而是 Wails beta.26 内置 Autostart（二进制含 `autostart_darwin_smappservice.go` 路径、`smAppService*` 符号、`autostart: write plist: %w` 等逐字文案；.app+macOS 13+ 走 SMAppService，否则回退 LaunchAgent，禁用双路径清理）——快照是 9-26 早期实现，9-27 已切换 |
| `platform_darwin_nocgo.go` / `platform_stub_other.go` | 非 cgo/非 darwin 回退：新签名全量适配，登录项保留完整功能（Autostart 不依赖 cgo） | — |

删除：`platform_darwin.m` / `platform_darwin.h` / `platform_cgo_darwin.go`（.m/.h 方案整体退役）。

### main.go 装配重写（对齐快照结构）
`win := platform.CreateMainWindow(app)`；热键改 `InstallToggleHotkey` + `ToggleWindow`
（弃 Wails GlobalShortcut）；hook 闭包传 `win`；登录项 hook 走 `platform.SetLaunchAtLogin(app, …)`。
service 层零改动；BackfillIcons/ShowPanel 等项目增强保留。

### 验证（全部实测）
- `go vet` ✓（项目包）；`go test ./internal/... -count=1` ✓（22 项回归全过）
- 符号指纹：`_RegisterEventHotKey`/`_armoryHotkeyFired`/`_UnregisterEventHotKey`（Carbon）、
  `smAppServiceRegister/Status/Unregister` + `_OBJC_CLASS_$_SMAppService`（Autostart）✓；
  旧实现零残留（`armorySetGlassDark`/`com.armory.launcher.plist`/`launchctl load` 均 0 命中）
- 冒烟：新构建面板渲染正常（四分类/shiro 子级/真实图标/添加卡）；**合成 ⌥Space 完整往返**
  （Carbon 热键隐藏 → 呼出，panel-shown 后搜索框自动聚焦）；无错误日志；`tools.json` 未动
- **已替换 /Applications/armory.app**（新构建 ad-hoc 签名，com.armory.launcher），
  替换前原版完整备份于 `bin/armory.original-9.27.app`（sha 9c0491dd… 与安装版一致）；
  替换后再次启动 + 双 ⌥Space 切换验证通过
- 用户待手验：设置 → 外观 → 深色（玻璃 tint 效果）；设置 → 通用 → 开机自启（系统设置 →
  登录项应出现原生 Armory 条目）


## 十一、第六轮补丁：右键菜单两处缺陷修复（2026-11-06，用户反馈 ✅）

用户实测反馈三处问题，逐项修复：

| # | 现象 | 根因 | 修复 |
|---|---|---|---|
| 1 | 右键侧栏底部分类，菜单向下溢出面板、删除项被截断 | openCtxMenu 用固定 160px 估算菜单高钳制，实际菜单 ≈200px+ | ContextMenu.vue 改为原版 je() 语义：先渲染在 -9999px → nextTick 测实际宽高 → 钳制 `innerHeight - 高 - 8` |
| 2 | 菜单项（删除）执行后菜单残留 | 菜单项点击只执行动作不关菜单 | ContextMenu.vue onItem 统一先 emit('close') 再执行 action（原版每个 action 均先 Me()） |
| 3 | 对话框点「确认删除？」toast `分类不存在: `（空 id），分类实际存在 | **上一项改动引入的回归**：CategoryDialog emit 裸 id 字符串，而 App.deleteCategory 按 `{id}` 对象取值 → 后端收到 undefined | deleteCategory 兼容两种入参（`typeof payload === 'string' ? payload : payload?.id`）；onDelete 加 category?.id 防线 |

顺带对齐原版 Kt() 删除后导航：若当前视图在被删子树内 → 跳父级（`parentID || 'all'`），否则保持当前视图。
右键「删除」语义同步原版 Re()：打开分类管理对话框并预武装（按钮直接「确认删除？」，3s 窗口）。

**运维教训**：`cp -R bin/armory.app /Applications/armory.app` 目标已存在时**不会覆盖** Contents/MacOS 可执行文件
（macOS cp 合并语义）——曾导致替换后 sha 未变装的是旧版。替换 .app 必须先 `rm -rf` 再拷，并以 sha 比对确认。

## 十二、补丁：右键菜单溢出的真根因——根 zoom 坐标系（2026-11-07）

上轮的「测高钳制」实际从未生效（此前验证点的菜单恰好自然放得下，误判为修复）。真根因：
应用在 `<html>` 上设 `zoom: 1.1`（fontScale 默认 large，App.vue:670），而 `.ctx-menu` 在
zoom 子树内——style 的 top/left 是本地单位（可视 = 本地 × zoom），`offsetHeight` 也是本地
单位，与 `window.innerHeight`（可视空间）混用导致钳制恒偏差一个 zoom 比例（≈10%，
底部菜单溢出约 47px =「删除」项高度）。**原版同款 bug**（原版固定 160px 估算 + zoom，溢出更甚）。

修复（ContextMenu.vue + App.vue hoverName 自校准）：渲染后用
`getBoundingClientRect()`（恒为可视坐标）与 `offsetWidth`（本地坐标）之比**实测 zoom**，
按可视溢出量 ÷ 实测 zoom 反折修正——对 zoom=1/1.1/1.2 任意值成立，不依赖坐标系假设。

**教训**：① 根 zoom 下 fixed 元素的定位/测量必须统一坐标系，或渲染后自校准；
② 验证"修好了"必须用最坏情况（最底部菜单项）实测，不能拿自然放得下的位置当证据；
③ 运行中的进程不加载重建后的二进制——替换后必须重启实例才算交付。
