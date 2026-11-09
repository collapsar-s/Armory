<script setup>
// Armory 主界面容器：编排状态、加载数据、串联子组件。
// 功能与运行版（/Applications/armory.app，9-27 构建）逐项对齐：两级分类（目录/标题/子分类）、
// 右键上下文菜单、拖拽改归属（同一一级目录范围内）、命令视图按分类分区 + 搜索高亮、
// 启动后自动收起面板、panel-shown 聚焦搜索框、两步删除确认、备份导入还原后重载设置等。
import { ref, computed, reactive, onMounted, watch, nextTick } from 'vue'
import { api, typeEmojis, typeLabels } from './composables/useApi'
import { renderMarkdown } from './composables/useMarkdown'
import { detectType } from './composables/useToolType'
import Sidebar from './components/Sidebar.vue'
import TopBar from './components/TopBar.vue'
import ToolGrid from './components/ToolGrid.vue'
import CmdList from './components/CmdList.vue'
import AddDialog from './components/AddDialog.vue'
import EditDialog from './components/EditDialog.vue'
import CategoryDialog from './components/CategoryDialog.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import PromptDialog from './components/PromptDialog.vue'
import SettingsDialog from './components/SettingsDialog.vue'
import ContextMenu from './components/ContextMenu.vue'
import NameTip from './components/NameTip.vue'
import Toast from './components/Toast.vue'

// ---- 状态 ----
const categories = ref([])
const tools = ref([])
const recent = ref([])
const icons = reactive({})
const activeCategory = ref('all')
const query = ref('')
const viewMode = ref('grid')
const selIdx = ref(0)
const dragging = ref(false)
// 全量工具元数据（侧栏计数 / 标签补全用）
const allToolsMeta = ref([])
// 侧栏计数徽标：由全量元数据 + 分类子树实时推导（响应式，任何数据变化即时反映）。
const toolCounts = computed(() => {
  const counts = {}
  for (const m of topItems.value) {
    const ids = subtreeIdSet(m.id)
    counts[m.id] = allToolsMeta.value.filter((t) => ids.has(t.categoryId)).length
  }
  return counts
})
const allTags = computed(() => {
  const s = new Set()
  for (const t of allToolsMeta.value) for (const tag of t.tags || []) s.add(tag)
  return [...s].sort()
})
const toastShow = ref(false)
const toastMsg = ref('')
const hotkey = ref('Alt+Space') // 固定为 Option+Space，不支持自定义（与原版一致）

// ---- 分类树辅助 ----
const catById = (id) => categories.value.find((c) => c.id === id)
const childrenOf = (id) => categories.value.filter((c) => c.parentId === id)
const topItems = computed(() => categories.value.filter((c) => !c.parentId))

// 侧栏条目（Maye 式布局）：只放 全部/最近 + 主分组（一级），子分组在右侧页内分节。
// count = 主分组子树内的工具数（侧栏徽标）。
const railItems = computed(() => {
  const items = [{ id: 'all', name: '全部', kind: 'all', parentId: '' }]
  if (recent.value.length) items.push({ id: 'recent', name: '最近', kind: 'recent', parentId: '' })
  for (const m of topItems.value) {
    items.push({ id: m.id, name: m.name, kind: 'dir', parentId: '', count: toolCounts.value[m.id] || 0 })
  }
  return items
})

const activeCategoryName = computed(() => {
  const c = catById(activeCategory.value)
  return c?.name ?? (activeCategory.value === 'all' ? '全部' : activeCategory.value === 'recent' ? '最近' : '')
})

// ---- 数据加载 ----
async function loadCategories() {
  categories.value = (await api.listCategories()) ?? []
}
async function loadTools() {
  if (viewMode.value !== 'cmd' && activeCategory.value === 'recent') {
    tools.value = recent.value
  } else {
    tools.value = (await api.listTools(viewMode.value === 'cmd' ? 'all' : activeCategory.value)) ?? []
  }
  for (const t of tools.value) {
    if (t.hasIcon && !icons[t.id]) {
      // 后端返回裸 base64，前端拼接 data URL（与原版管线一致）。
      api.getIcon(t.id).then((raw) => {
        if (raw) icons[t.id] = 'data:image/png;base64,' + raw
      })
    }
  }
}
async function loadRecent() {
  recent.value = (await api.listRecent(12)) ?? []
}
// 全量刷新：最近 + 分类 + 工具（对应原版 $e()）。
// 侧栏计数不再在此手工统计——toolCounts 是响应式 computed，allToolsMeta 一到即生效。
async function refreshAll() {
  await loadRecent()
  await Promise.all([loadCategories(), loadTools()])
  // 全量工具元数据：侧栏计数 + 标签补全（数据量小，代价可忽略）。
  allToolsMeta.value = (await api.listTools('')) ?? []
}
function catName(id) {
  const c = catById(id)
  return c?.name ?? ''
}
// 分类完整路径（"父 / 子"，与原版 H() 一致），用于提示与对话作文案。
function catPath(id) {
  const c = catById(id)
  if (!c) return ''
  const parts = [c.name]
  let p = c.parentId
  while (p) {
    const pc = catById(p)
    if (!pc) break
    parts.unshift(pc.name)
    p = pc.parentId
  }
  return parts.join(' / ')
}
// 分类所属的一级目录 id（原版 We()）。
function rootOfCat(id) {
  let c = catById(id)
  let guard = 0
  while (c?.parentId && guard++ < 10) c = catById(c.parentId)
  return c?.id ?? ''
}

// ---- 搜索过滤 ----
// 原版匹配范围：名称、路径、类型 emoji（🚀☕🐍⌨️🔧 均可命中）；扩展：标签。
// activeTag：页内标签筛选 chips 选中的标签（切分类时清空）。
const activeTag = ref('')
const filtered = computed(() =>
  tools.value.filter((t) => {
    if (activeTag.value && !(t.tags || []).includes(activeTag.value)) return false
    const q = query.value.trim().toLowerCase()
    if (!q) return true
    return (
      t.name.toLowerCase().includes(q) ||
      t.path.toLowerCase().includes(q) ||
      (t.tags || []).some((tag) => tag.toLowerCase().includes(q)) ||
      (typeEmojis[t.type] || '').toLowerCase().includes(q) ||
      (typeLabels[t.type] || '').toLowerCase().includes(q)
    )
  })
)

// 搜索命中高亮：返回已转义并包 <mark> 的 HTML（命令视图用 v-html）。
function markName(name) {
  const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  const q = query.value.trim()
  if (!q) return esc(name)
  const lower = name.toLowerCase()
  const idx = lower.indexOf(q.toLowerCase())
  if (idx < 0) return esc(name)
  return esc(name.slice(0, idx)) + '<mark>' + esc(name.slice(idx, idx + q.length)) + '</mark>' + esc(name.slice(idx + q.length))
}

// ---- 主网格分组（Maye 式）：子分类/标题在页内分节，数据零迁移 ----
// ListTools 返回整个子树，这里按 categoryId 切节；搜索过滤（filtered）对每节同样生效。
// 已折叠分节 id（localStorage 持久化：跨会话记住折叠状态）。
const collapsedSecs = ref((() => {
  try { return JSON.parse(localStorage.getItem('armory-folded-secs') || '[]') } catch { return [] }
})())
watch(collapsedSecs, (v) => {
  try { localStorage.setItem('armory-folded-secs', JSON.stringify(v)) } catch { /* 隐私模式等忽略 */ }
}, { deep: true })
function toggleSec(id) {
  const i = collapsedSecs.value.indexOf(id)
  if (i >= 0) collapsedSecs.value.splice(i, 1)
  else collapsedSecs.value.push(id)
}
function subtreeIdSet(id) {
  const s = new Set([id])
  for (const ch of childrenOf(id)) s.add(ch.id)
  return s
}
const gridSections = computed(() => {
  const cur = activeCategory.value
  if (cur === 'recent') return []
  const list = filtered.value
  if (cur === 'all') {
    // 全部视图：按主分组分节（含其子树工具），空组不显示
    return topItems.value
      .map((m) => {
        const ids = subtreeIdSet(m.id)
        return { id: m.id, name: m.name, kind: 'dir', tools: list.filter((t) => ids.has(t.categoryId)) }
      })
      .filter((s) => s.tools.length)
  }
  const cat = catById(cur)
  if (!cat || cat.parentId) return [] // 子级/标题被直接选中（命令视图跳转）→ 平铺
  const children = childrenOf(cur)
  if (!children.length) return [] // 无子分组 → 保持原平铺形态
  const direct = list.filter((t) => t.categoryId === cur)
  const out = []
  // 直属区为空时不渲染（避免一张孤零零的添加卡占一大块），改由页面底部细行入口承接。
  if (direct.length) out.push({ id: cur, name: '', kind: 'dir', tools: direct })
  for (const ch of children) {
    out.push({ id: ch.id, name: ch.name, kind: 'dir', tools: list.filter((t) => t.categoryId === ch.id) })
  }
  return out
})
// 「添加」卡片位置：分组视图放在直属分节（id=当前主分组）；平铺视图不传（ToolGrid 默认显示）。
const gridAddAt = computed(() => (gridSections.value.length && activeCategory.value !== 'all' ? activeCategory.value : ''))
// 分组页直属区为空时，页面底部显示细的「添加到当前分组」行。
const gridAddRow = computed(() => {
  const cur = activeCategory.value
  if (cur === 'all' || cur === 'recent') return ''
  const cat = catById(cur)
  if (!cat || cat.parentId || !childrenOf(cur).length) return ''
  return gridSections.value.some((s) => s.id === cur) ? '' : cat.name
})
// 添加工具弹窗预选分类：当前停在某个分组页时预选它（平铺空组视图同理会带出）。
const addPresetCat = computed(() =>
  activeCategory.value === 'all' || activeCategory.value === 'recent' ? '' : activeCategory.value
)


// ---- 命令视图分区 ----
// 命令视图加载的是全量工具（loadTools('all')，与原版一致的全局搜索），
// 因此搜索中按「命中工具各自的主分组」归节；无搜索词时才按当前主分组子树分节。
const sections = computed(() => {
  const cur = activeCategory.value
  const isSpecial = cur === 'all' || cur === 'recent'
  const list = filtered.value
  if (isSpecial) return [{ id: cur, name: '', kind: '', parentId: '', tools: list }]
  if (query.value.trim()) {
    // 全局搜索：结果跨主分组 → 按主分组归节，避免只显示当前分组子树丢命中。
    const out = []
    const known = new Set()
    for (const m of topItems.value) {
      const ids = subtreeIdSet(m.id)
      const hits = list.filter((t) => ids.has(t.categoryId))
      for (const h of hits) known.add(h.id)
      if (hits.length) out.push({ id: m.id, name: m.name, kind: 'dir', parentId: '', tools: hits })
    }
    const orphans = list.filter((t) => !known.has(t.id)) // 分类已删的兜底
    if (orphans.length) out.push({ id: '__other__', name: '其他', kind: 'dir', parentId: '', tools: orphans })
    return out
  }
  const children = childrenOf(cur)
  if (!children.length) return [{ id: cur, name: '', kind: '', parentId: '', tools: list }]
  const out = []
  for (const ch of children) {
    out.push({ id: ch.id, name: ch.name, kind: 'dir', parentId: ch.parentId || '', tools: list.filter((t) => t.categoryId === ch.id) })
  }
  const direct = list.filter((t) => t.categoryId === cur)
  if (direct.length) out.unshift({ id: cur, name: '未分类', kind: 'dir', parentId: '', tools: direct })
  return out
})

// 线性顺序（用于键盘上下选择）
const flatTools = computed(() => {
  const arr = []
  for (const s of sections.value) for (const t of s.tools) arr.push(t)
  return arr
})
function flatIndex(t) {
  return flatTools.value.findIndex((x) => x.id === t.id)
}

// ---- 分类切换 / 右键 ----
// 与原版 jt() 一致：命令视图里切换分类会清空搜索并回到网格视图。
function selectCategory(item) {
  activeCategory.value = item.id
  activeTag.value = '' // 切分类清空标签筛选
  if (viewMode.value === 'cmd') {
    query.value = ''
    viewMode.value = 'grid'
  }
  loadTools()
}
function openCategoryMenu(item, ev) {
  if (item.id === 'all' || item.id === 'recent') return
  openCtxMenu(item, ev, 'category')
}

// ---- 启动 / 编辑 ----
async function launchTool(t) {
  if (!t) return
  try {
    await api.launch(t.id)
    toast(`已启动 ${t.name}`)
    // Spotlight 行为：启动后稍候收起面板，并刷新"最近"列表。
    setTimeout(() => api.hideWindow(), 320)
    await refreshAll()
  } catch (e) {
    toast(`启动失败: ${e?.message ?? e}`)
  }
}

// ---- 添加工具 ----
const showAdd = ref(false)
function openAdd() {
  showAdd.value = true
}
async function submitAdd(form) {
  try {
    const t = await api.addByPath(form.path, form.name, form.args, form.jvmArgs, form.categoryId, form.envVersion || '')
    showAdd.value = false
    await refreshAll()
    toast(`已收录 ${t?.name ?? ''}`)
  } catch (e) {
    toast(`收录失败: ${e?.message ?? e}`)
  }
}

// ---- 编辑工具 ----
const showEdit = ref(false)
const editingTool = ref(null)
const editReadme = ref('')
function editTool(t) {
  if (!t) return
  editingTool.value = t
  editReadme.value = ''
  api.getReadme(t.id).then((md) => (editReadme.value = renderMarkdown(md)))
  // 与原版一致：编辑时若运行环境列表尚未加载，先补加载（供版本下拉使用）。
  if (!javaVersions.value.length && !pythonVersions.value.length) reloadEnvLists()
  showEdit.value = true
}
async function saveEdit(payload) {
  try {
    await api.update(payload.id, payload.name, payload.path, payload.args, payload.jvmArgs, payload.tags, payload.term, payload.categoryId, payload.envVersion, payload.workingDir ?? '')
  } catch (e) {
    toast(`保存失败: ${e?.message ?? e}`)
    return
  }
  showEdit.value = false
  await refreshAll()
  toast('已保存')
}
async function deleteTool(id) {
  try {
    await api.remove(id)
  } catch (e) {
    toast(`删除失败: ${e?.message ?? e}`)
    return
  }
  showEdit.value = false
  await refreshAll()
  toast('已删除')
}

// ---- 分类管理（新建/编辑） ----
const showCat = ref(false)
const catMode = ref('catadd')
const editingCat = ref(null)
const presetLevel = ref('') // 右键进入时的预设类型：'' | 'title' | 'sub'
const presetParent = ref('') // 预设父级（右键）/ 预选父级（＋ 且当前是顶层目录）
const presetArmed = ref(false) // 编辑态删除预武装（右键「删除」进入时为 true）

// ---- 危险操作确认弹窗（通用）：{ title, name, path, consequence, onConfirm } ----
const confirmBox = ref(null)
async function askDeleteCategory(cat) {
  if (!cat?.id) return
  // 统计将被移动的工具数（本分类直属 + 其下标题名下的），给删除确认一个明确的代价。
  // 子分类名下的工具跟着子分类走（顶层删除时子分组整体提升为顶层），不计入。
  let direct = 0
  try {
    const all = (await api.listTools('')) || []
    direct = all.filter((t) => t.categoryId === cat.id).length
  } catch {}
  const childCats = categories.value.filter((c) => c.parentId === cat.id)
  const fallbackName = catById(cat.parentId)?.name || ''
  let consequence
  if (childCats.length) {
    consequence = `直属 ${direct} 个工具将归入「${fallbackName || '其他模块'}」，${childCats.length} 个子分组将提升为顶层，此操作不可撤销`
  } else if (direct) {
    consequence = `其中 ${direct} 个工具将归入「${fallbackName || '其他模块'}」，此操作不可撤销`
  } else {
    consequence = '该分组为空，此操作不可撤销'
  }
  confirmBox.value = { title: '删除分类', name: cat.name, path: catPath(cat.id), consequence, onConfirm: () => deleteCategory(cat.id) }
}
async function onConfirmBox() {
  const b = confirmBox.value
  confirmBox.value = null
  try {
    await b?.onConfirm?.()
  } catch (e) {
    toast(e?.message ?? String(e))
  }
}
function openCatAdd() {
  catMode.value = 'catadd'
  editingCat.value = null
  presetLevel.value = ''
  presetArmed.value = false
  // 当前选中项是主分组时，预选它作为父级。
  const cur = catById(activeCategory.value)
  presetParent.value = cur && !cur.parentId ? cur.id : ''
  showCat.value = true
}
function openCatAddAs(level, parentId) {
  catMode.value = 'catadd'
  editingCat.value = null
  presetLevel.value = level
  presetParent.value = parentId || ''
  presetArmed.value = false
  showCat.value = true
}
function openCatEdit(cat, armed = false) {
  catMode.value = 'catedit'
  editingCat.value = { id: cat.id, name: cat.name, parentId: cat.parentId || '', kind: cat.kind || 'dir' }
  presetLevel.value = ''
  presetParent.value = ''
  presetArmed.value = !!armed // 右键「删除」= 预武装打开（原版 z(e, !0) 语义）
  showCat.value = true
}
async function submitCat(payload) {
  try {
    if (payload.mode === 'catadd') {
      // 与原版 Wt() 一致：提交前预检分类名。
      if (!payload.name.trim()) {
        toast('分类名不能为空')
        return
      }
      if (payload.level === 'sub') {
        await api.addSubCategory(payload.parentId || '', payload.name)
        toast(`已在「${catPath(payload.parentId)}」下创建分组`)
        if (payload.parentId) activeCategory.value = payload.parentId
      } else {
        await api.addCategory(payload.name)
        toast('分类已创建')
        activeCategory.value = 'all'
      }
    } else if (payload.name !== payload.originalName) {
      await api.renameCategory(payload.id, payload.name)
      toast('已保存')
    }
    showCat.value = false
    await refreshAll()
  } catch (e) {
    toast(e?.message ?? String(e))
  }
}
// payload 兼容两种来源：CategoryDialog emit 裸 id；旧调用点传 { id } 对象。
async function deleteCategory(payload) {
  const id = typeof payload === 'string' ? payload : payload?.id
  if (!id) return
  try {
    // 原版 Kt()：先记录父级；删除后若当前视图在被删子树内 → 跳父级，否则保持。
    const parentID = catById(id)?.parentId ?? ''
    await api.deleteCategory(id)
    const activeRoot = rootOfCat(activeCategory.value)
    if (activeCategory.value === id || (activeRoot && activeRoot === id)) {
      activeCategory.value = parentID || 'all'
    }
    showCat.value = false
    await refreshAll()
    toast('分类已删除')
  } catch (e) {
    toast(e?.message ?? String(e))
  }
}

// ---- 右键上下文菜单 ----
const ctx = reactive({ show: false, title: '', x: 0, y: 0, items: [], target: null, kind: '' })
function openCtxMenu(target, ev, kind) {
  ctx.target = target
  ctx.kind = kind
  ctx.title = catPath(target.id) || target.name
  ctx.items = buildCtxItems(target, kind)
  // 位置钳制在 ContextMenu 内按菜单实际尺寸完成（原版 je() 语义），
  // 这里只记录原始触发点。
  ctx.x = ev.clientX
  ctx.y = ev.clientY
  ctx.show = true
}
function closeCtx() {
  ctx.show = false
}
function buildCtxItems(target, kind) {
  if (kind === 'tag') {
    // 标签条 chip 右键：跨工具批量管理标签
    return [
      { label: '重命名标签', action: () => renameTag(target.tag) },
      { label: '删除标签', danger: true, action: () => askDeleteTag(target.tag) },
    ]
  }
  if (kind === 'tool') {
    return [
      { label: '打开', action: () => launchTool(target) },
      { label: '编辑', action: () => editTool(target) },
      { label: '删除', danger: true, action: () => { api.remove(target.id).then(() => { refreshAll(); toast('已删除') }).catch((e) => toast(`删除失败: ${e?.message ?? e}`)) } },
    ]
  }
  // category（概念统一后只有 主分组/分组 两种，不再区分标题）
  const items = []
  if (!target.parentId) {
    items.push({ label: '新建分组', action: () => openCatAddAs('sub', target.id) })
  }
  items.push({ label: '重命名', action: () => openCatEdit(target) })
  // 右键「删除」→ 专用确认弹窗（用户优化：不走「分类管理」对话框）。
  items.push({ label: '删除', danger: true, action: () => askDeleteCategory(target) })
  return items
}
function onToolMenu(t) {
  // 右键工具 → 直接打开"编辑工具"对话框（与原版一致）
  editTool(t)
}

// ---- 标签管理（跨工具批量）：标签条 chip 右键 → 重命名 / 删除 ----
const promptBox = ref(null) // { title, label, value, onConfirm }；null = 关闭
function onTagMenu(tag, ev) {
  openCtxMenu({ id: '', name: `标签：${tag}`, tag }, ev, 'tag')
}
function renameTag(tag) {
  promptBox.value = {
    title: '重命名标签',
    label: '新名称',
    value: tag,
    onConfirm: async (nv) => {
      try {
        await api.renameTag(tag, nv)
        await refreshAll()
        toast(`标签已重命名为「${nv}」`)
      } catch (e) {
        toast(e?.message ?? String(e))
      }
    },
  }
}
function askDeleteTag(tag) {
  const n = allToolsMeta.value.filter((t) => (t.tags || []).includes(tag)).length
  confirmBox.value = {
    title: '删除标签',
    name: tag,
    path: `跨工具标签（${n} 个工具在用）`,
    consequence: `将从 ${n} 个工具上移除该标签，工具本身不受影响，此操作不可撤销`,
    onConfirm: async () => {
      await api.deleteTag(tag)
      await refreshAll()
      toast('标签已删除')
    },
  }
}
function onPromptConfirm(v) {
  const p = promptBox.value
  promptBox.value = null
  p?.onConfirm?.(v)
}
// 网格空白处右键 → 打开当前分类的右键菜单（原版 He()）。
function onGridBackgroundMenu(ev) {
  if (viewMode.value !== 'grid') return
  const cur = catById(activeCategory.value)
  if (cur) openCtxMenu(cur, ev, 'category')
}

// ---- 长名称悬浮提示 ----
const tip = reactive({ show: false, text: '', x: 0, y: 0 })
let tipTimer = null
function hoverName(t, ev) {
  const el = ev.currentTarget
  // 仅当文本被截断时才显示（宽度受限）
  if (el.scrollWidth <= el.clientWidth + 1) return
  clearTimeout(tipTimer)
  tipTimer = setTimeout(() => {
    const r = el.getBoundingClientRect()
    tip.text = t.name
    tip.x = Math.max(8, Math.min(r.left + r.width / 2 - 100, window.innerWidth - 220))
    tip.y = r.bottom + 6
    tip.show = true
    // 自校准：html 上有 zoom（默认 1.1），fixed 提示的 top/left 是本地单位、
    // 渲染时被放大——按可视差量 ÷ 实测 zoom 修正（与 ContextMenu 同机制）。
    nextTick(() => {
      const ntp = document.querySelector('.name-tip')
      if (!ntp) return
      const rect = ntp.getBoundingClientRect()
      const z = rect.width / ntp.offsetWidth || 1
      let dx = 0
      let dy = 0
      if (rect.left < 8) dx = rect.left - 8
      else if (rect.right > window.innerWidth - 8) dx = rect.right - (window.innerWidth - 8)
      if (rect.top < 8) dy = rect.top - 8
      else if (rect.bottom > window.innerHeight - 8) dy = rect.bottom - (window.innerHeight - 8)
      if (dx) tip.x = tip.x - dx / z
      if (dy) tip.y = tip.y - dy / z
    })
  }, 300)
}
function leaveName() {
  clearTimeout(tipTimer)
  tip.show = false
}

// ---- 拖拽改归属（工具 / 分组，限同一一级目录范围内） ----
const dragTool = ref(null) // 被拖动对象（tool 或分组 id）
const dragKind = ref('') // 'tool' | 'cat'
const dragSrcRoot = ref('') // 拖动源所属一级目录
const internalDrag = ref(false) // 面板内部的拖拽（区别于外部文件拖入）
const dropOkId = ref('') // 高亮的目标分类
function onDragStart(t, ev) {
  dragTool.value = t
  dragKind.value = 'tool'
  dragSrcRoot.value = rootOfCat(t.categoryId)
  internalDrag.value = true
  ev.dataTransfer?.setData('text/plain', t.id)
  if (ev.dataTransfer) ev.dataTransfer.effectAllowed = 'move'
}
function endDrag() {
  internalDrag.value = false
  dragKind.value = ''
  dragTool.value = null
}
// 分组拖拽起点（侧栏主分组 / 页内分节头）：用于同层排序。
function onCatDragStart(id, ev) {
  dragTool.value = { id }
  dragKind.value = 'cat'
  dragSrcRoot.value = ''
  internalDrag.value = true
  ev.dataTransfer?.setData('text/plain', id)
  if (ev.dataTransfer) ev.dataTransfer.effectAllowed = 'move'
}
// 同层分组重排的通用实现见 onRailDrop 的 cat 分支（侧栏与页内分节头共用）。
// 工具卡片拖放重排：拖到同分组某卡片的上/下半 → 插到其前/后；跨分组 → 改归属。
async function reorderToolCard(targetToolId, before) {
  if (dragKind.value !== 'tool') return
  const dragged = dragTool.value
  if (!dragged || dragged.id === targetToolId) return
  const target = tools.value.find((t) => t.id === targetToolId)
  if (!target) return
  try {
    if (target.categoryId !== dragged.categoryId) {
      // 跨分组：等价于拖到该分组的节头（改归属）→ 全量刷新（侧栏计数随之更新）
      await api.update(dragged.id, '', '', dragged.args ?? '', '', [], false, target.categoryId, dragged.envVersion ?? '')
      toast(`已移动到「${catName(target.categoryId)}」`)
      await refreshAll()
    } else {
      const seq = tools.value
        .filter((t) => t.categoryId === target.categoryId)
        .map((t) => t.id)
        .filter((id) => id !== dragged.id)
      let ti = seq.indexOf(targetToolId)
      if (!before && ti >= 0) ti += 1
      seq.splice(ti < 0 ? seq.length : ti, 0, dragged.id)
      await api.reorderTools(seq)
    }
    await loadTools()
  } catch (e) {
    toast(e?.message ?? String(e))
  }
}
function onRailDragOver(id) {
  if (!internalDrag.value) return
  if (!catById(id) || id === dragTool.value?.id) return
  dropOkId.value = id
}
function onRailDragLeave(id) {
  if (dropOkId.value === id) dropOkId.value = ''
}
async function onRailDrop(catId) {
  const src = dragTool.value
  const kind = dragKind.value
  endDrag()
  dropOkId.value = ''
  if (!src || !catId || catId === src.id || !catById(catId)) return
  try {
    if (kind === 'cat') {
      // 分组排序：仅同层（同为侧栏主分组，或同属一个主分组的分节）内生效。
      const srcParent = catById(src.id)?.parentId ?? ''
      if (srcParent !== (catById(catId)?.parentId ?? '')) return
      const seq = categories.value
        .filter((c) => (c.parentId || '') === srcParent)
        .map((c) => c.id)
        .filter((id) => id !== src.id)
      const ti = seq.indexOf(catId)
      seq.splice(ti < 0 ? seq.length : ti, 0, src.id)
      await api.reorderCategories(seq)
      toast('排序已保存')
    } else {
      // 工具改归属：带上原参数占位，后端按语义保留 args/jvmArgs/tags/term。
      await api.update(src.id, '', '', src.args ?? '', '', [], false, catId, src.envVersion ?? '')
      toast(`已移动到「${catName(catId)}」`)
    }
    await refreshAll()
  } catch (e) {
    toast(e?.message ?? String(e))
  }
}

// ---- 设置 ----
const showSettings = ref(false)
const settings = reactive({ theme: 'auto', fontScale: 'large', reduceMotion: false, alwaysOnTop: false, launchOnLogin: false })
const javaVersions = ref([])
const pythonVersions = ref([])
const savingEnv = ref('')
const confirmImport = ref(false)
const fontScales = { standard: 1, large: 1.1, xlarge: 1.2 }

async function openSettings() {
  showSettings.value = true
  try {
    const s = await api.getSettings()
    if (!s) return
    Object.assign(settings, {
      theme: s.theme || 'auto',
      fontScale: s.fontScale || 'large',
      reduceMotion: !!s.reduceMotion,
      alwaysOnTop: !!s.alwaysOnTop,
      launchOnLogin: !!s.launchOnLogin,
    })
    hotkey.value = 'Alt+Space' // 固定 Option+Space
    javaVersions.value = (s.javaVersions || []).map((v) => ({ ...v }))
    pythonVersions.value = (s.pythonVersions || []).map((v) => ({ ...v }))
  } catch (e) {
    console.error('加载设置失败', e)
  }
}
async function reloadEnvLists() {
  try {
    const s = await api.getSettings()
    javaVersions.value = (s.javaVersions || []).map((v) => ({ ...v }))
    pythonVersions.value = (s.pythonVersions || []).map((v) => ({ ...v }))
  } catch (e) {
    console.error('加载运行环境失败', e)
  }
}
function setTheme(v) { settings.theme = v; api.setTheme(v); applyTheme(v) }
function setFontScale(v) { settings.fontScale = v; api.setFontScale(v); document.documentElement.style.setProperty('zoom', String(fontScales[v] || 1.1)) }
function applyReduceMotion(v) {
  // 与原版一致：设置开启 或 系统开启"减弱动态效果"时都降级动画。
  const system = !!window.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches
  document.documentElement.classList.toggle('reduce-motion', v || system)
}
function setReduceMotion(v) {
  if (settings.reduceMotion === v) return
  settings.reduceMotion = v
  api.setReduceMotion(v)
  applyReduceMotion(v)
  toast(v ? '已减弱动态效果' : '已恢复完整动画')
}
async function setAlwaysOnTop(v) {
  if (settings.alwaysOnTop === v) return
  settings.alwaysOnTop = v
  try {
    await api.setAlwaysOnTop(v)
    toast(v ? '窗口已置顶，始终浮于其他窗口之上' : '已取消置顶')
  } catch (e) {
    settings.alwaysOnTop = !v
    toast(e?.message ?? String(e))
  }
}
async function setLaunchOnLogin(v) {
  if (settings.launchOnLogin === v) return
  settings.launchOnLogin = v
  try {
    await api.setLaunchAtLogin(v)
    toast(v ? '已开启开机自启（登录时自动启动）' : '已关闭开机自启')
  } catch (e) {
    settings.launchOnLogin = !v
    toast(e?.message ?? String(e))
  }
}
// 扫描结果按 path 合并（不覆盖已有行），返回新增数量（与原版 Et() 一致）。
function mergeVersions(list, detected) {
  let added = 0
  for (const d of detected || []) {
    if (list.some((v) => v.path === d.path)) continue
    list.push({ name: d.name, path: d.path })
    added++
  }
  return added
}
async function scanJava() {
  try {
    const n = mergeVersions(javaVersions.value, (await api.detectJavaVersions()) ?? [])
    toast(n ? `发现 ${n} 个 JDK，已填入列表（记得保存）` : '未在本机发现新的 JDK')
  } catch (e) {
    toast(e?.message ?? String(e))
  }
}
async function scanPython() {
  try {
    const n = mergeVersions(pythonVersions.value, (await api.detectPythonVersions()) ?? [])
    toast(n ? `发现 ${n} 个 Python，已填入列表（记得保存）` : '未在本机发现新的 Python')
  } catch (e) {
    toast(e?.message ?? String(e))
  }
}
function addEnv(list) { list.push({ name: '', path: '' }) }
function delEnv(list, i) { list.splice(i, 1) }
async function saveEnv(kind) {
  const list = kind === 'java' ? javaVersions.value : pythonVersions.value
  if (list.some((v) => !v.name.trim() && v.path.trim())) {
    // 文案与原版二进制逐字一致（原版此处的三元表达式即如此）。
    toast(kind === 'java' ? 'Java' : 'Python有配置行未填版本名')
    return
  }
  savingEnv.value = kind
  try {
    const payload = list.map((v) => ({ name: v.name.trim(), path: v.path.trim() }))
    if (kind === 'java') await api.saveJavaVersions(payload)
    else await api.savePythonVersions(payload)
    await reloadEnvLists()
    toast(kind === 'java' ? 'Java 环境已保存，立即生效' : 'Python 环境已保存，立即生效')
  } catch (e) {
    toast(e?.message ?? String(e))
  } finally {
    savingEnv.value = ''
  }
}
async function exportBackup() {
  try {
    const p = await api.exportBackup()
    if (p) toast(`已导出到 ${p}`)
  } catch (e) {
    toast(e?.message ?? String(e))
  }
}
async function importBackup() {
  // 两步确认布防窗口 3 秒（与原版一致）。
  if (!confirmImport.value) { confirmImport.value = true; setTimeout(() => (confirmImport.value = false), 3000); return }
  confirmImport.value = false
  try {
    await api.importBackup()
    showSettings.value = false
    await refreshAll()
    // 还原后重载设置并立即应用（主题/字号/动画/自启/置顶）。
    const s = await api.getSettings()
    if (s) {
      Object.assign(settings, {
        theme: s.theme || 'auto',
        fontScale: s.fontScale || 'large',
        reduceMotion: !!s.reduceMotion,
        alwaysOnTop: !!s.alwaysOnTop,
        launchOnLogin: !!s.launchOnLogin,
      })
    }
    applyTheme(settings.theme)
    document.documentElement.style.setProperty('zoom', String(fontScales[settings.fontScale] || 1.1))
    applyReduceMotion(settings.reduceMotion)
    toast('已从备份还原')
  } catch (e) {
    toast(e?.message ?? String(e))
  }
}

// ---- 主题 / 外观 ----
function applyTheme(v) {
  const dark = v === 'dark' || (v !== 'light' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.dataset.theme = dark ? 'dark' : 'light'
  api.setGlassTheme(dark)
}
async function initAppearance() {
  try {
    const accent = await api.getAccentColor()
    if (accent) document.documentElement.style.setProperty('--accent', accent)
  } catch (e) { console.error('读取强调色失败', e) }
  let s = null
  try { s = await api.getSettings() } catch (e) { console.error('读取设置失败', e) }
  hotkey.value = 'Alt+Space' // 固定 Option+Space，不支持自定义
  applyTheme(s?.theme || 'auto')
  document.documentElement.style.setProperty('zoom', String(fontScales[s?.fontScale] || 1.1))
  applyReduceMotion(!!s?.reduceMotion)
  if (window.matchMedia) {
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      if (settings.theme === 'auto') applyTheme('auto')
    })
    window.matchMedia('(prefers-reduced-motion: reduce)').addEventListener('change', () => {
      applyReduceMotion(settings.reduceMotion)
    })
  }
}

// ---- 搜索键盘 ----
function onSearchKeydown(e) {
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault()
    const n = flatTools.value.length
    if (!n) return
    selIdx.value = (selIdx.value + (e.key === 'ArrowDown' ? 1 : -1) + n) % n
  } else if (e.key === 'Enter') {
    e.preventDefault()
    const t = flatTools.value[selIdx.value]
    if (!t) return
    if (e.metaKey) editTool(t)
    else launchTool(t)
  }
}

// ---- 文件拖入收录 ----
let lastDropSig = ''
let lastDropAt = 0
async function onFilesDropped(files) {
  if (!files || !files.length) return
  // 去重：files-dropped（后端转发）与 common:WindowFilesDropped（原生）可能同时触发，
  // 相同文件在极短时间内只处理一次，避免重复收录。
  const sig = files.join('|')
  const now = Date.now()
  if (sig === lastDropSig && now - lastDropAt < 400) return
  lastDropSig = sig
  lastDropAt = now
  const cat = activeCategory.value === 'all' || activeCategory.value === 'recent' ? '' : activeCategory.value
  let ok = 0
  for (const f of files) {
    try { await api.addByPath(f.path || f, '', '', '', cat, ''); ok++ }
    catch (e) { toast(`收录失败: ${e?.message ?? e}`) }
  }
  if (ok) { await refreshAll(); toast(`已收录 ${ok} 个工具`) }
}
let dragDepth = 0
function onDragEnter() {
  // 面板内部拖拽（工具/标题改归属）不显示"松手收录"提示（与原版一致）。
  if (internalDrag.value) return
  dragDepth++
  dragging.value = true
}
function onDragLeave() { dragDepth = Math.max(0, dragDepth - 1); if (!dragDepth) dragging.value = false }
function onDropReset() { dragDepth = 0; dragging.value = false; endDrag() }

// ---- 工具函数 ----
function toast(msg) {
  toastMsg.value = msg
  toastShow.value = true
  setTimeout(() => (toastShow.value = false), 2200)
}
function closeAll() {
  showAdd.value = false
  showEdit.value = false
  showCat.value = false
  showSettings.value = false
  ctx.show = false
  confirmBox.value = null
  promptBox.value = null
}

// ---- 初始化 ----
onMounted(async () => {
  try { await initAppearance() } catch (e) { console.error(e) }
  // 启动即走全量刷新（含分类/工具/最近/计数元数据），保证侧栏计数首屏就有值。
  try { await refreshAll() } catch (e) { console.error('初始化加载失败', e) }
  // 与原版 Qt() 一致：Alt+Space 交给全局热键；Esc 按 编辑→添加→分类→设置→命令视图→隐藏 的优先级。
  document.addEventListener('keydown', (e) => {
    if (e.altKey && e.code === 'Space') return
    if (e.key !== 'Escape') return
    if (promptBox.value) { promptBox.value = null; return }
    if (confirmBox.value) { confirmBox.value = null; return }
    if (showEdit.value) { showEdit.value = false; return }
    if (showAdd.value) { showAdd.value = false; return }
    if (showCat.value) { showCat.value = false; return }
    if (showSettings.value) { showSettings.value = false; return }
    if (ctx.show) { closeCtx(); return }
    if (viewMode.value === 'cmd') {
      query.value = ''
      viewMode.value = 'grid'
      loadTools()
      return
    }
    api.hideWindow()
  })
  api.onPanelShown(() => {
    const app = document.querySelector('.app')
    if (app) { app.classList.remove('enter-grid', 'enter-cmd'); void app.offsetWidth; app.classList.add('enter-grid') }
    nextTick(() => document.getElementById('search-input')?.focus())
  })
  api.onFilesDropped(onFilesDropped)
  api.onToast((msg) => toast(msg))
  document.addEventListener('dragenter', onDragEnter)
  document.addEventListener('dragleave', onDragLeave)
  document.addEventListener('dragover', (e) => e.preventDefault())
  document.addEventListener('drop', onDropReset)
  document.addEventListener('dragend', onDropReset)
  // 滚动时收起长名提示（与原版一致）。
  document.addEventListener('scroll', () => { tip.show = false }, true)
})

// ---- 搜索驱动的视图切换（与原版 It() 一致）----
// 输入搜索词自动进入命令视图；清空搜索自动回到网格视图；
// 每次搜索词变化复位键盘选择索引。命令视图加载的是全量列表，
// 进入/退出时取一次即可，逐键只做本地过滤（避免每键一次 IPC）。
watch(query, () => {
  const has = !!query.value.trim()
  if (has && viewMode.value === 'grid') {
    viewMode.value = 'cmd'
    loadTools()
  }
  if (!has && viewMode.value === 'cmd') {
    viewMode.value = 'grid'
    loadTools()
  }
  selIdx.value = 0
})
</script>

<template>
  <div class="app" :class="{ dragging }" data-file-drop-target>
    <Sidebar
      :items="railItems"
      :active-category="activeCategory"
      :drop-ok-id="dropOkId"
      @select="selectCategory"
      @contextmenu="openCategoryMenu"
      @add="openCatAdd"
      @drag-over="onRailDragOver"
      @drag-leave="onRailDragLeave"
      @drop-on="onRailDrop"
      @cat-drag-start="onCatDragStart"
    />

    <div class="main">
      <TopBar
        :query="query"
        @update:query="query = $event"
        @keydown="onSearchKeydown"
        @add="openAdd"
        @settings="openSettings"
      />

      <main class="content">
        <ToolGrid
          v-if="viewMode === 'grid'"
          :tools="filtered"
          :tag-source="tools"
          :sections="gridSections"
          :folded-secs="collapsedSecs"
          :drop-ok-id="dropOkId"
          :add-at="gridAddAt"
          :add-row="gridAddRow"
          :active-tag="activeTag"
          :internal-drag="internalDrag"
          :icons="icons"
          :active-category-name="activeCategoryName"
          :dragging="dragging"
          :cat-name="catName"
          @launch="launchTool"
          @edit="editTool"
          @add="openAdd"
          @menu="onToolMenu"
          @bg-menu="onGridBackgroundMenu"
          @drag-start="onDragStart"
          @drag-end="endDrag"
          @hover-name="hoverName"
          @leave-name="leaveName"
          @toggle-sec="toggleSec"
          @sec-menu="(sec, ev) => openCtxMenu(catById(sec.id), ev, 'category')"
          @sec-drag-over="onRailDragOver"
          @sec-drag-leave="onRailDragLeave"
          @sec-drop="onRailDrop"
          @sec-drag-start="onCatDragStart"
          @card-reorder="reorderToolCard"
          @set-tag="activeTag = $event"
          @tag-menu="onTagMenu"
        />

        <CmdList
          v-else
          :sections="sections"
          :icons="icons"
          :sel-idx="selIdx"
          :query="query"
          :active-category="activeCategory"
          :cat-name="catName"
          :cat-path="catPath"
          :mark-name="markName"
          :flat-index="flatIndex"
          @launch="launchTool"
          @edit="onToolMenu"
          @select="selIdx = $event"
          @nav="selectCategory"
          @cat-menu="openCtxMenu"
          @drag-over="onRailDragOver"
          @drag-leave="onRailDragLeave"
          @drop-on="onRailDrop"
        />
      </main>
    </div>

    <AddDialog
      v-if="showAdd"
      :categories="categories"
      :preset-category="addPresetCat"
      :java-versions="javaVersions"
      :python-versions="pythonVersions"
      @close="showAdd = false"
      @submit="submitAdd"
      @toast="toast"
    />

    <EditDialog
      v-if="showEdit && editingTool"
      :tool="editingTool"
      :categories="categories"
      :java-versions="javaVersions"
      :python-versions="pythonVersions"
      :all-tags="allTags"
      :icon="icons[editingTool.id] || ''"
      :readme="editReadme"
      @close="showEdit = false"
      @save="saveEdit"
      @delete="deleteTool"
      @toast="toast"
    />

    <CategoryDialog
      v-if="showCat"
      :mode="catMode"
      :category="editingCat"
      :modules="topItems"
      :preset-level="presetLevel"
      :preset-parent="presetParent"
      :preset-armed="presetArmed"
      :cat-path="catPath"
      @close="showCat = false"
      @submit="submitCat"
      @delete="deleteCategory"
    />

    <ConfirmDialog
      v-if="confirmBox"
      :title="confirmBox.title"
      :target="confirmBox"
      confirm-label="删除"
      @confirm="onConfirmBox"
      @cancel="confirmBox = null"
    />

    <PromptDialog
      v-if="promptBox"
      :title="promptBox.title"
      :label="promptBox.label"
      :value="promptBox.value"
      @close="promptBox = null"
      @confirm="onPromptConfirm"
    />

    <SettingsDialog
      v-if="showSettings"
      :settings="settings"
      :hotkey="hotkey"
      :java-versions="javaVersions"
      :python-versions="pythonVersions"
      :saving-env="savingEnv"
      :confirm-import="confirmImport"
      @close="showSettings = false"
      @set-theme="setTheme"
      @set-font-scale="setFontScale"
      @set-reduce-motion="setReduceMotion"
      @set-always-on-top="setAlwaysOnTop"
      @set-launch-on-login="setLaunchOnLogin"
      @scan-java="scanJava"
      @scan-python="scanPython"
      @add-env="addEnv"
      @del-env="delEnv"
      @save-env="saveEnv"
      @export-backup="exportBackup"
      @import-backup="importBackup"
    />

    <ContextMenu
      :show="ctx.show"
      :title="ctx.title"
      :x="ctx.x"
      :y="ctx.y"
      :items="ctx.items"
      @close="closeCtx"
    />

    <NameTip :show="tip.show" :text="tip.text" :x="tip.x" :y="tip.y" />

    <Toast :show="toastShow" :msg="toastMsg" />
  </div>
</template>
