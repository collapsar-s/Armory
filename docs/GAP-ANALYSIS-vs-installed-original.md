# 差异分析：当前工程 vs 已安装原版（权威基准）

> 分析日期：2026-11-03。
> **基准物**：`/Applications/armory.app`（二进制 2026-09-27 重打包，是最新原版；DMG 内是 09-26 旧快照）。
> 方法：从原版二进制中**原样提取内嵌前端**（`index.html` + `index-CWpXw6Ob.js` 133,135B + `index-ZDAhzKcN.css` 24,035B，
> 已存档于 `docs/orig-frontend-reference/`），美化后逐行逆向其行为；同时提取 Go 符号与全部业务字符串，
> 与当前工程 `frontend/src/`、`internal/` 逐项比对。所有结论均有代码级证据。
>
> ⚠️ 旧文档 `docs/DIFF-vs-original.md` 的「功能已 100% 对齐」结论**不成立**——它只比对了 methodID 与 CSS，
> 且其"458KB JS"记录有误。本文以实测证据为准。

---

## 结论速览

| 层面 | 状态 |
|---|---|
| 后端 API 面（39 方法） | ✅ 齐全，含 AddTitle/AddSubCategory/MoveCategory |
| 后端层级守卫 / UpdateTool 空值语义 | ✅ 与原版错误文案逐一吻合 |
| **前端行为** | ❌ **大量缺失**：命令视图不可达（死代码）、备份导入导出实际损坏、运行版本选择缺失、删除无确认、标题拖拽移动未接线、全套操作反馈 toast 缺失 |
| 样式 | ✅ 规则级等价（仅压缩格式差异） |
| 数据兼容（tools.json 顶层大写键 + 小写字段） | ✅ 一致 |

**根因**：当前前端是从 **DMG 旧版（09-26）** 的逆向源码重建的，而已安装原版是 **09-27 演进版**（新增两级层级/标题/移动等）。
后续虽然补齐了 API 面，但交互行为从未逐项对齐。

---

## 一、P0：功能缺失 / 实际损坏

### 1. 命令视图（Spotlight 搜索面板）整体不可达 —— 最核心体验缺失
- 原版：搜索词非空 → 自动切入 `cmd` 视图（`viewMode='cmd'`，bundle 中有 4 处视图赋值）；
  清空 → 回网格；Esc 退出；面板内 Enter 启动 / ⌘Enter 编辑 / 右键编辑 / ↓↑ 选择 + scrollIntoView。
- 当前：`viewMode` 初始化为 `'grid'` 后**再无任何赋值**（bundle 实测 0 处赋值）——
  `CmdList.vue` 是死代码。搜索只在网格里做文本过滤。
- 附带缺失：搜索匹配域（原版 = name **或** path **或** 类型中文标签，搜"App"能命中全部 .app 工具；
  当前只有 name/path）；每次输入 `selIdx=0`；区头 `link`（点击进入该分类）、区头右键菜单、区头计数。

### 2. 备份导出/导入实际损坏（双向）
- 导出：原版 `exportBackup()` **后端弹保存对话框**（默认名 `armory-backup-<20060102-150405>.zip`，
  错误文案"导出对话框不可用"），前端 toast `已导出到 <path>`。
  当前：`packBackup()` 写临时文件就返回，**用户永远拿不到文件**，也没有保存对话框。
- 导入：原版 `importBackup()` **无参**，后端自弹打开对话框（标题"选择备份包"）。
  当前 Go `ImportBackup(path string)` 要求传路径，而前端 `invoke('importBackup', [])` **不传** → 必然报错。
- zip 结构不兼容：原版 = `tools.json`（根）+ `icons/`（错误串"备份包缺少 tools.json"/"备份包缺少分类数据"）；
  当前 = `data.json` + `icons/` → **原版备份包在当前版本无法导入，反之亦然**。
- 导入成功后：原版重新拉取 accent + settings 并重应用主题/字体/动效/自启/置顶 + 刷新分类/工具/最近 + toast `已从备份还原`。当前只刷两个列表。

### 3. 添加对话框缺「运行版本」整块（jar/py）
- 原版：类型为 jar/py 时显示 `是否采用全局设置` 开关 + `全局版本` 下拉 +
  hint（`使用「设置 → 运行环境」中配置的版本` / `使用系统默认解释器`）；
  提交校验三连：`路径不能为空` → `请先在「设置 → 运行环境」中配置版本` → `请选择一个全局版本`；
  成功提交 `addByPath(path.trim(), categoryId, name.trim(), args.trim(), 采用全局?所选版本:'')`。
- 当前：AddDialog 无此区块，`envVersion` 恒为 `''`，无路径空校验 —— 收录 jar/py 时**无法指定运行版本**。

### 4. 删除全部无确认
- 原版：编辑对话框与分类对话框的删除按钮是**同一按钮两段式武装**（`删除` → 3 秒内变 `确认删除？` 再点才删，超时自动解除）；
  右键菜单「删除」= 预武装打开对话框。
- 当前：编辑框删除、右键删除、分类删除全部单击直删（分类删除仅靠 `confirmImport` 式 2.2 秒双击 hack 的是导入不是删除）。

### 5. 标题（title）拖拽移动未接线
- 原版：命令视图区头 `draggable = kind==='title'`，拖到侧栏项/其它区头 → `moveCategory(拖动id, 目标id)`，
  toast `标题已移动到「${name}」`；跨一级目录 → toast `只能在同一个一级目录范围内移动`（后端同样守卫）；
  drop 目标高亮 `.drop-ok`。提示文案 `拖拽可移动标题 · 右键管理` / `点击进入 · 右键管理`。
- 当前：`api.moveCategory` 在 useApi 中存在但**无任何调用点**；CmdList 区头无 draggable/drop，Sidebar 的 drop 只处理工具。

### 6. 网格视图分区结构缺失
- 原版：网格同样按当前分类的子结构分区渲染（区头 + 各分区尾部各插一张添加卡片；
  直属工具区名 `未分类`），区头带 `is-title`/`sub-sec`/`link`/`drop-ok` 样式。
- 当前：ToolGrid 完全平铺，无区头、无分区添加卡；分区名用了 `未分组`（原版 `未分类`）。

---

## 二、P1：交互反馈与细节缺失（原版有、当前无）

| # | 原版行为 | 当前状态 |
|---|---|---|
| 1 | 启动成功：toast `已启动 X` → **320ms 后 hideWindow()** → 刷新最近使用 | 启动后面板不隐藏、最近不刷新 |
| 2 | 设置各开关 toast：`已减弱动态效果/已恢复完整动画`、`已开启/关闭开机自启（登录时自动启动）`、`窗口已置顶，始终浮于其他窗口之上/已取消置顶`，失败回滚 + toast | 全部静默 |
| 3 | 环境保存：校验"有路径无名字的行" → toast `Java/Python有配置行未填版本名`；成功 → 重新加载 + `Java/Python 环境已保存，立即生效` | 无校验，toast 只有通用 `已保存`，保存后不重载 |
| 4 | 扫描版本：**按 path 去重合并**进现有列表（保留未保存手写行），toast `发现 N 个 JDK，已填入列表（记得保存）` / `未在本机发现新的 JDK`（Python 同款） | 直接**整体替换**数组（丢未保存行）、无 toast |
| 5 | Escape 分层：编辑框 → 添加框 → 分类框 → 设置框 → cmd 视图退回网格 → 隐藏面板；且 **Alt+Space 被显式排除**（交给全局热键） | ctx → 任意对话框（closeAll 一次全关）→ 隐藏；无 cmd 退回；未排除 Alt+Space |
| 6 | `panel-shown`：重放入场动画 + **聚焦搜索框** | 只重放动画，不聚焦 |
| 7 | 编辑对话框头部显示**真实图标**（有缓存显示 img，否则 emoji）；版本下拉只列**已命名**版本 | 只有 emoji；下拉列出全部行（含未命名的空行） |
| 8 | 分类下拉选项：顶层=名称，子级=`◇ 前缀 + 全路径`（`父/子`） | 平铺本名 |
| 9 | 命令行/卡片 cpath 显示分类**全路径** | 只显示本名 |
| 10 | 新建标题/子分类成功 toast：`已在「${全路径}」下创建标题/子分类` + 自动导航到该分类 | 统一 `分类已创建`，不导航 |
| 11 | 重命名成功 toast `已保存`；编辑保存各字段 `.trim()` | 无 toast；不 trim |
| 12 | 分类删除后：若当前视图在被删子树内 → 跳**父级**或 `all`，侧栏 active 项 `scrollIntoView` | 恒跳 all，无滚动 |
| 13 | name-tip：mouseenter 即触发、下方居中、放不下翻上方、页面滚动即隐藏 | 仅溢出时 + 300ms 延迟 + 固定下方 |
| 14 | 设置里「收录/拖拽收录」「真实图标（state-dot）」两个说明/状态行 | 缺失 |
| 15 | reduce-motion 叠加系统 `prefers-reduced-motion` | 只用设置值 |
| 16 | 网格空白处右键 = 当前分类菜单（仅网格视图） | 无 |
| 17 | 新建分类对话框：右键进入时锁定层级（disabled input `${标题|子分类} · 位于 ${父}`）；顶层标题可创建（`独立模块（侧栏顶层）` 选项）；placeholder 有父=`例如：Shiro 漏洞工具` | 层级三选一 + 上级下拉；**无法创建顶层标题**；placeholder=`例如：信息收集` |

---

## 三、后端差异（当前 Go vs 原版二进制证据）

| # | 项 | 原版证据（二进制字符串/符号） | 当前 | 处置建议 |
|---|---|---|---|---|
| 1 | 备份 zip 结构 | `tools.json`、`备份包缺少 tools.json`、文件名 `20060102-150405`、对话框标题 `选择备份包`、`导出对话框不可用` | `data.json`、`armory-backup-*.zip` 临时文件、无对话框 | **改为 tools.json + 时间戳命名 + 前后端各自弹原生对话框**（同时保留对 `data.json` 的读取兼容可不做） |
| 2 | 图标缓存目录 | `cache/icons`（实测 `~/.armory/cache/icons/` 有 09-26 原版写入的 png） | `~/.armory/icons` | **改回 `~/.armory/cache/icons`**，兼容已缓存图标 |
| 3 | GetIcon 返回 | 前端自行拼 `data:image/png;base64,` 前缀 → 后端返回**裸 base64** | 返回完整 data URL | 与前端方案配套二选一；若内嵌原版前端产物则必须返回裸 base64 |
| 4 | 收录路径校验 | `路径不存在: %s` | AddToolByPath 不校验存在性 | 补上 |
| 5 | 分类删除守卫 | `至少保留一个分类` | 无此守卫 | 补上 |
| 6 | 分类删除时工具去向 | 文案 `自动归入上级分类或其他模块` | 直接归入"第一个其它分类" | 改为**先父级后首个其它分类** |
| 7 | java 兜底 | 二进制含 `/usr/libexec/java_home` | 仅 `LookPath("java")`（macOS 上常缺失） | 兜底链补 `/usr/libexec/java_home --> Home/bin/java` |
| 8 | python 探测 | `/usr/bin/python3`、`/usr/local/bin/python3*`、`/opt/homebrew/bin/python3*` | LookPath + pyenv | 可补上述固定路径扫描（行为更接近） |
| 9 | MoveCategory 守卫 | `仅标题支持拖拽移动` | 无（仅前端约束） | 可选：后端加同款守卫 |
| 10 | 默认热键常量 | `alt+space`（小写） | `Alt+Space` | 外观级，可不改 |
| 11 | 开机自启 | `SMAppService` 与 plist 写入符并存 | LaunchAgent plist | 功能等价，保留 |
| 12 | 失焦隐藏 | 原版前端与二进制均无 blur 处理 | 无 | 双方一致，无需改 |

已确认**一致**的部分：UpdateTool 空串=保持原值（拖拽移动依赖此语义）✓、层级守卫全套文案 ✓、
`分类已存在`/`工具不存在`/`启动失败: %w` ✓、launch 命令拼装（open -a / java -jar / python / /bin/sh / 直启）✓、
JAVA_HOME/PATH 注入仅命中配置版本时 ✓、`至少`图标提取（.icns → sips → png）✓、readme 候选文件（README.md/说明.md/使用说明.md…）✓、
`panel-shown`/`files-dropped`/`toast` 事件管线 ✓、窗口 880×560（min 640×420）Frameless + Liquid Glass ✓、
GlobalShortcut 热键（⌥Space 呼出/隐藏）✓、`Toast`（dot）/`ctx-menu`/`kbd-chips` 等样式钩子 ✓。

---

## 四、修复方案（两个选项）

### 方案 A（推荐，保真度 100%，工作量小）：内嵌原版前端产物
`docs/orig-frontend-reference/` 内的三个文件就是**从原版二进制原样提取**的完整前端（字节级）。
1. 将其放入 `frontend/dist/`（替代 vite 产物）并 `go:embed`；
2. 后端按本表第三节的 1–7 项修正（备份对话框+tools.json 结构、GetIcon 裸 base64、
   图标目录 `cache/icons`、路径校验、至少保留一个分类、java_home 兜底）；
3. 构建脚本跳过 vite（或保留 `make frontend-legacy` 分支）。
- 优点：前端行为/文案/动效与原版**逐字节一致**，无回归面；后端仅 7 处小改。
- 缺点：前端回到"无源码"状态（后续迭代需逆向或重新工程化）。

### 方案 B（工程化路线，工作量大）：现有 Vue 源码逐项对齐
按本文第一、二节清单逐项修补现有组件（命令视图接管搜索、运行版本区块、两段式删除、
moveCategory 接线、网格分区、全套 toast、备份流程…约 25 项），后端同样修第三节 1–7。
- 优点：保留组件化源码与工程化成果。
- 缺点：改动面 ~14 个文件，细节极易再漂移；建议配一份「原版行为规格」作为验收清单逐条比对。

**建议**：先做方案 A 立即恢复原版体验（并留 `recovered/`、当前 frontend 源码于分支），
之后若要继续功能演进，再按第二节清单把行为移植回组件化源码。

---

## 五、证据与基准物索引

| 证据 | 位置 |
|---|---|
| 原版前端产物（字节级，含 index.html/js/css） | `docs/orig-frontend-reference/` |
| 原版美化 JS（逆向分析用） | `/tmp/orig-app.pretty.js`（临时；可从上面 js 重新生成） |
| 原版安装包二进制 | `/Applications/armory.app/Contents/MacOS/armory`（sha1 `9c0491dd…`，09-27 版，比 DMG 新） |
| 旧版（DMG）基准 | `armory.dmg`（sha1 `a0ad4d96…`）、`recovered/frontend/dist.original/`（09-26 旧前端，无 addTitle/moveCategory） |
| 原版二进制关键字符串 | `仅标题支持拖拽移动`、`只能在同一个一级目录范围内移动`、`目录最多支持两级`、`至少保留一个分类`、`备份包缺少 tools.json`、`20060102-150405`、`选择备份包`、`cache/icons`、`/usr/libexec/java_home`、`alt+space`、`#0A84FF` 等 |
| 用户真实数据 | `~/.armory/tools.json`（4 分类 + Java/Python 版本；`Tools` 目前为空——建议尽快用方案 A 修好收录功能后重新收录，或从原版备份导入） |
