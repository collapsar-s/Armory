# Armory

> macOS 渗透工具收纳 / 启动器（Wails v3 + Vue 3，Spotlight 式无边框悬浮面板）。
> 本工程由 `recovered/`（从 `armory.dmg` 逆向恢复的源码）重构而来：功能与原版**完全一致**，
> 但在架构、代码质量、工程化与可维护性上做了系统性优化。

---

## 1. 这是什么

把 `.app / .jar / .py / .sh / 二进制` 收录成卡片，可搜索、分类、拖拽收录、指定
Java/Python 运行时启动；支持图标提取、说明文档渲染、备份导入导出、主题/字体/动效设置。
全局热键（默认 `⌥Space`）呼出/隐藏面板。

- 应用标识：`com.armory.launcher` · 版本 `0.1.0` · 名称 `Armory`（早期代号 Vulpes）
- 数据目录：`~/.armory/`（`tools.json` + `icons/*.png`）
- 兼容性：**直接读取原版 `~/.armory/tools.json`**（字段 / JSON tag 完全一致）

---

## 2. 快速开始

### 直接用打包好的产物
```
bin/armory.app     # 双击运行（已 ad-hoc 签名）
bin/armory.dmg     # 安装镜像：拖入 Applications 即可
bin/armory         # 裸可执行文件
```

### 从源码构建
本环境工具链：Go 1.26.5、Node 24（nvm）、pnpm 11、wails3 beta.26、Xcode CLT、sips。
本沙箱下 `GOCACHE` / npm 缓存需重定向到可写目录，并显式开启 CGO（详见 `docs/CONVENTIONS.md` 第7节）。

```bash
# 一键构建（已封装上述环境变量）
make deps        # 安装前端依赖
make frontend    # 前端 vite build -> frontend/dist
make build       # go build -tags production -> bin/armory
make package     # 生成 bin/armory.app + ad-hoc 签名
make dmg         # 生成 bin/armory.dmg（hdiutil）

# 或使用 wails3 Taskfile
wails3 task package      # 构建前端 + 编译 + 打包 .app
wails3 task dmg          # 追加 .dmg
wails3 task dev          # 热重载开发
```

> 环境变量（`make` 已内置）：
> `export PATH="$HOME/go/bin:$PATH" GOCACHE=/tmp/gocache-armory CGO_ENABLED=1`
> `source ~/.nvm/nvm.sh && export npm_config_cache=/tmp/npmcache-armory`

---

## 3. 目录结构

```
Armory/
├── main.go                     # 应用装配：窗口/热键/资产/服务 + 平台 hook 注入
├── go.mod / go.sum
├── internal/
│   ├── model/model.go          # 数据模型（Tool/Category/RuntimeVersion/Settings/RegistryData）
│   ├── store/store.go          # 持久化层：RWMutex + 原子写 + ~/.armory 兼容
│   ├── service/                # 业务层
│   │   ├── api.go              # Wails 绑定对象（27 API + hook 注入）
│   │   ├── registry.go         # registry 门面（委托 store）
│   │   ├── launcher.go         # 按类型/运行时版本拼装命令并启动
│   │   ├── iconextract.go      # 从 .app 提取图标（sips → png）
│   │   ├── envprobe.go         # 探测本机 Java / Python
│   │   ├── readme.go           # 读取工具说明文档
│   │   ├── backup.go           # 导出/导入 zip 备份
│   │   └── util.go             # 路径/序列化辅助
│   └── platform/               # macOS 原生能力（cgo Objective-C + 纯 Go 回退）
│       ├── platform_cgo_darwin.go    # //go:build darwin && cgo
│       ├── platform_darwin.m/.h      # NSWindow 动画/红绿灯/玻璃真实现
│       ├── platform_darwin_nocgo.go  # //go:build darwin && !cgo
│       └── platform_stub_other.go    # //go:build !darwin
├── frontend/
│   ├── index.html  package.json  vite.config.js  .npmrc  pnpm-workspace.yaml
│   ├── src/
│   │   ├── main.js  App.vue           # 容器编排（470 行，原 679 行）
│   │   ├── components/                 # 9 个组件
│   │   ├── composables/useApi.js       # 27 API + 3 事件（官方 bindings / 回退）
│   │   ├── composables/useMarkdown.js  # Markdown 渲染
│   │   ├── composables/useToolType.js  # 类型判断
│   │   ├── styles/style.css            # 原版样式（与产物 sha1 一致）
│   │   └── bindings/                   # wails3 生成的官方 bindings
│   └── dist/                           # 构建产物（go:embed）
├── build/                      # wails3 打包脚手架（Info.plist / icons.icns / dmg 资源）
├── Taskfile.yml  Makefile
├── docs/                       # CONVENTIONS.md / INTERFACE.md / 本重构说明
└── armory.dmg                  # 原始安装包（对照用）
```

---

## 4. 相对 recovered 版的优化

### 4.1 结构缺陷修复
| 问题（recovered） | 处理 |
|---|---|
| `ToolView` 重复嵌入 `HasIcon`，与 `model.Tool.HasIcon` 冲突 | 删除 `ToolView`，列表直接返回 `[]model.Tool` |
| 大量无用函数（`itoa/containsName/prependPath/prependPathEnv/majorVersion/filepathAbs/homeDir/decodeUniEscapes`） | 全部删除 |
| `main.go` 未用 import（`events`）、占位逻辑 | 清理，事件按真实语义接线 |
| 平台层全是纯 Go 空壳（动画/红绿灯/玻璃/热键均 no-op） | 用 **Objective-C + cgo 真实现**，并保留纯 Go 回退 |
| `addByPath` 参数顺序在恢复源码里被标错（name/args ↔ category 错位） | 依原始编译产物修正为 `(path, categoryID, name, args, envVersion)`（详见 §4.4） |

### 4.2 架构分层
- **持久化从业务中剥离**：新增 `internal/store`（RWMutex 读写分离、`.tmp`+`rename` 原子写、
  深拷贝快照、旧数据自动补默认分类），`service.Registry` 仅为门面。
- **业务与平台解耦**：service 通过 hook 接口（Glass/Hide/Show/Top/Login/Dialog）调用平台能力，
  `main.go` 负责把 Wails 窗口/对话框接到 hook 上。
- **错误不再被吞**：I/O、启动、删除、备份等全部返回有意义错误和中文上下文。

### 4.3 前端工程化
- **组件拆分**：679 行单文件 `App.vue` → 470 行容器 + 9 个组件
  （Sidebar / TopBar / ToolGrid / CmdList / AddDialog / EditDialog / CategoryDialog / SettingsDialog / Toast）
  + 3 个 composable（useApi / useMarkdown / useToolType）。
- **官方 bindings**：改用 `wails3 generate bindings` 产物（`@wailsio/runtime` 的 `$Call.ByID`），
  不再手写 methodID；同时保留 `window._wails.invoke(methodID)` 回退，二者 methodID 完全一致。
- **样式零漂移**：`style.css` 原样迁移，构建产物 CSS 与原版 **sha1 完全一致**。
- **事件接线补全**：`files-dropped`（拖拽收录）/ `toast` / `panel-shown` 及原生拖拽高亮态
  按原版编译产物的语义接好（recovered 的 `frontend-src` 遗漏了这段）。

### 4.4 保真度验证（关键证据）
- **方法 ID 逐一吻合**：`wails3 generate bindings` 生成的 27 个 API 的 `$Call.ByID(...)` methodID
  与原始二进制内嵌前端里的 methodID **完全一致**（如 `ListTools=3183816730`、`HidePanel=3214670595`、
  `SetGlassTheme=772031016`、`AddToolByPath=2651014998`）。由于 methodID = FNV(`armory/internal/service.API.<Method>`)
  的哈希，这证明包路径 / 类型名 / 方法名与原版逐一对应。
- **参数顺序修正的依据**：原始编译产物中，添加对话框调用
  `addByPath(path, categoryId, name, args, env)`、拖拽收录调用
  `addByPath(path, categoryId, '', '')`，据此确定后端第 2 参为分类、第 3 参为名称；
  已按此修正 Go 签名并在 frontend 线路层重排，单测覆盖验证。
- **数据兼容**：直接加载用户真实的 `~/.armory/tools.json`（4 个分类 + Java/Python 版本配置）零错误。
- **样式一致**：产物 `index-*.css` sha1 与原版构建产物一致。

---

## 5. 绑定 API（27 方法 + 3 事件）

前端方法名（`useApi` 暴露）→ Go 导出名：

```
listCategories   → ListCategories        listTools        → ListTools
addByPath        → AddToolByPath*        update           → UpdateTool
remove           → DeleteTool            addCategory      → AddCategory
renameCategory   → RenameCategory        deleteCategory   → DeleteCategory
launch           → Launch                getIcon          → GetIcon
getSettings      → GetSettings           setTheme         → SetTheme
setReduceMotion  → SetReduceMotion       setLaunchAtLogin → SetLaunchAtLogin
setAlwaysOnTop   → SetAlwaysOnTop        setFontScale     → SetFontScale
getAccentColor   → GetAccentColor        setGlassTheme    → SetGlassTheme
saveJavaVersions → SaveJavaVersions      savePythonVersions → SavePythonVersions
detectJavaVersions → DetectJavaVersions  detectPythonVersions → DetectPythonVersions
listRecent       → ListRecentTools       getReadme        → GetToolReadme
exportBackup     → ExportBackup          importBackup     → ImportBackup
hideWindow       → HidePanel
```
`*` AddToolByPath 真实参数顺序为 `(path, categoryID, name, args, envVersion)`（见 §4.4）。

事件：`files-dropped`（data.files：[]string）、`toast`（data：string）、`panel-shown`（无参）。

---

## 6. 验证清单（均已通过）

- [x] `go build ./...`（`-tags production`，CGO_ENABLED=1）通过
- [x] `go vet ./...` 通过；`gofmt -l` 无输出
- [x] `vite build` 通过，产物 `frontend/dist/assets/*.js|css`
- [x] `wails3 task package` → `bin/armory.app`（ad-hoc 签名，`com.armory.launcher`）
- [x] `wails3 tool package --format dmg` → `bin/armory.dmg`（含 Applications 链接 / 背景 / 卷图标）
- [x] 运行时冒烟：`.app` 启动无 panic；加载真实用户 `~/.armory/tools.json` 成功
- [x] 业务单测（临时）：默认分类、类型判断、收录/更新/删除、设置、分类 CRUD、
      版本校验、备份 zip 往返、`SplitArgs` 引号切分、`GetIcon` 缺失语义、`addByPath` 参数顺序 —— 全 PASS

---

## 7. 已知约束 / 说明
- `.dmg` 由 `hdiutil` 创建；在本机某些**受限文件沙箱**下 `hdiutil create` 会被拒绝，
  需在具备磁盘服务权限的环境中执行（本工程已成功产出 `bin/armory.dmg`）。
- 平台层窗口动画/红绿灯/玻璃通过 `NSApp` 的 keyWindow/mainWindow 兜底获取窗口；
  已预留 `armorySetMainWindow` 供 main 显式注入指针以增强稳定性。
- `SMAppService` 未直接链接，开机自启采用等价的 `~/Library/LaunchAgents` LaunchAgent 方案。
- 目标运行环境：macOS 26+（原生 Liquid Glass），arm64。

---

## 8. 与 recovered 的关系
`recovered/` 保留在本目录作为**功能基线对照**，不参与本工程构建。
本工程（根目录及各 `internal/ frontend/`）是重构后的正式源码，两者行为一致。
详细协作与契约见 `docs/CONVENTIONS.md`、`docs/INTERFACE.md`。
