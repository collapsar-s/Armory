<script setup>
// 网格视图：工具卡片 + 添加卡片 + 空态/拖拽提示。
// Maye 式分组布局：侧栏只放主分组，子分类/标题在页内分节（sec-head 可折叠、
// 可右键管理、可作为拖放目标改归属、可拖拽排序）；无子分组时保持原平铺形态。
// 卡片可拖拽：拖到侧栏/节头=改归属；拖到同分组另一张卡=排序；支持右键菜单与长名提示。
import { computed, ref } from 'vue'
import { typeLabels, typeEmojis } from '../composables/useApi'

const props = defineProps({
  tools: { type: Array, required: true }, // 当前视图全部工具（子树，已含标签/搜索过滤）
  tagSource: { type: Array, default: null }, // 标签条数据源（未过滤列表）；空则回退 tools
  icons: { type: Object, required: true },
  activeCategoryName: { type: String, default: '' },
  dragging: { type: Boolean, default: false }, // 外部文件拖入
  internalDrag: { type: Boolean, default: false }, // 面板内拖拽（卡片/分组）
  catName: { type: Function, required: true },
  sections: { type: Array, default: null }, // 分组视图：[{ id, name, kind, tools }]；null=平铺
  foldedSecs: { type: Array, default: () => [] }, // 已折叠的分节 id
  dropOkId: { type: String, default: '' }, // 拖拽高亮的目标分类
  addAt: { type: String, default: '' }, // 「添加」卡片放在哪个分节（空=平铺视图显示在网格末尾）
  addRow: { type: String, default: '' }, // 非空=页面底部显示「添加工具到「X」」细行（直属区为空时）
  activeTag: { type: String, default: '' }, // 当前标签筛选
})
const emit = defineEmits([
  'launch', 'edit', 'add', 'menu', 'bg-menu', 'drag-start', 'drag-end', 'hover-name', 'leave-name',
  'toggle-sec', 'sec-menu', 'sec-drag-over', 'sec-drag-leave', 'sec-drop', 'sec-drag-start',
  'card-reorder', 'set-tag', 'tag-menu',
])

function onMouseMove(e) {
  const t = e.currentTarget
  const r = t.getBoundingClientRect()
  t.style.setProperty('--mx', ((e.clientX - r.left) / r.width) * 100 + '%')
  t.style.setProperty('--my', ((e.clientY - r.top) / r.height) * 100 + '%')
}

// 统一渲染：分组视图切成分节；平铺视图就是一个无节头的分节。
const displaySecs = computed(() => {
  if (props.sections?.length) {
    return props.sections.map((s) => ({
      key: s.id,
      id: s.id,
      header: !!s.name,
      name: s.name,
      tools: s.tools,
      folded: props.foldedSecs.includes(s.id),
      addCard: !!props.addAt && s.id === props.addAt,
    }))
  }
  return [{ key: '__flat__', id: '__flat__', header: false, tools: props.tools, folded: false, addCard: true }]
})

// 当前页可用标签（按出现次数降序）——标签筛选 chips。
// 注意：必须用【未按标签过滤】的列表计算（tagSource），否则选中某个标签后
// 其它标签的 chips 会随之消失，无法再切换。
const tagChips = computed(() => {
  const m = new Map()
  for (const t of props.tagSource ?? props.tools) for (const tag of t.tags || []) m.set(tag, (m.get(tag) || 0) + 1)
  return [...m.entries()].sort((a, b) => b[1] - a[1])
})

// 标签很多时筛选条会无限拉长：默认只显示前 8 个（按使用次数），
// 超出收进「更多 N」，点击展开/收起；当前选中的标签即使在折叠区也保持可见。
const TAG_VISIBLE = 8
const tagExpanded = ref(false)
const visibleTagChips = computed(() => {
  if (tagExpanded.value) return tagChips.value
  const top = tagChips.value.slice(0, TAG_VISIBLE)
  if (props.activeTag && !top.some(([t]) => t === props.activeTag)) {
    const hit = tagChips.value.find(([t]) => t === props.activeTag)
    if (hit) top.push(hit)
  }
  return top
})
const tagOverflow = computed(() => tagChips.value.length - visibleTagChips.value.length)

// 卡片间排序拖放：记录落点在目标卡片的上/下半（前/后插入）。
const cardDropAfter = ref(false)
function onCardDragOver(t, e) {
  if (!props.internalDrag) return // 外部文件拖放不拦截，交给窗口级处理器
  e.preventDefault()
  e.stopPropagation()
  const r = e.currentTarget.getBoundingClientRect()
  cardDropAfter.value = e.clientX >= r.left + r.width / 2
}
function onCardDrop(t, e) {
  if (!props.internalDrag) return
  e.preventDefault()
  e.stopPropagation()
  emit('card-reorder', t.id, !cardDropAfter.value)
}
</script>

<template>
  <div class="view enter-grid" @contextmenu.prevent="emit('bg-menu', $event)">
    <!-- 标签筛选条（当前页存在标签时出现；超过 8 个折叠进「更多」） -->
    <div v-if="tagChips.length" class="tag-bar">
      <button class="tag-chip" :class="{ on: !activeTag }" @click="emit('set-tag', '')">全部</button>
      <button
        v-for="[tag, n] in visibleTagChips"
        :key="tag"
        class="tag-chip"
        :class="{ on: activeTag === tag }"
        :title="'右键管理标签'"
        @click="emit('set-tag', tag)"
        @contextmenu.prevent.stop="emit('tag-menu', tag, $event)"
      >
        {{ tag }}<i>{{ n }}</i>
      </button>
      <button v-if="tagOverflow > 0 && !tagExpanded" class="tag-chip more" @click="tagExpanded = true">
        更多 {{ tagOverflow }} ▾
      </button>
      <button v-else-if="tagOverflow > 0" class="tag-chip more" @click="tagExpanded = false">收起 ▴</button>
    </div>
    <section v-for="sec in displaySecs" :key="sec.key" class="grid-sec">
      <!-- 节头：点击折叠/展开；右键管理；可拖拽排序；也是工具改归属的拖放目标 -->
      <div
        v-if="sec.header"
        class="sec-head grid-sec-head"
        :class="{ 'drop-ok': dropOkId === sec.key }"
        title="分组：可折叠、可拖入工具、可拖拽排序"
        draggable="true"
        @click="emit('toggle-sec', sec.key)"
        @contextmenu.prevent.stop="emit('sec-menu', sec, $event)"
        @dragstart.stop="emit('sec-drag-start', sec.id, $event)"
        @dragover.prevent="emit('sec-drag-over', sec.key)"
        @dragleave="emit('sec-drag-leave', sec.key)"
        @drop.prevent="emit('sec-drop', sec.key)"
      >
        <span class="sec-name">{{ sec.name }}</span>
        <span class="sec-count">{{ sec.tools.length }}</span>
        <svg class="sec-chev" :class="{ folded: sec.folded }" viewBox="0 0 24 24">
          <path d="M8 10l4 4 4-4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
      </div>
      <div v-show="!sec.folded" class="grid">
        <div
          v-for="t in sec.tools"
          :key="t.id"
          class="card"
          tabindex="0"
          draggable="true"
          :title="t.name"
          @click="emit('launch', t)"
          @contextmenu.prevent.stop="emit('menu', t, $event)"
          @mousemove="onMouseMove"
          @dragstart="emit('drag-start', t, $event)"
          @dragend="emit('drag-end')"
          @dragover="onCardDragOver(t, $event)"
          @drop="onCardDrop(t, $event)"
          @mouseenter="emit('hover-name', t, $event)"
          @mouseleave="emit('leave-name')"
          @keydown.enter.exact.prevent="emit('launch', t)"
          @keydown.meta.enter.prevent="emit('edit', t)"
        >
          <div class="ticon" :class="t.type">
            <img v-if="icons[t.id]" :src="icons[t.id]" :alt="t.name" />
            <span v-else>{{ typeEmojis[t.type] }}</span>
          </div>
          <div class="tname">{{ t.name }}</div>
          <div v-if="t.tags && t.tags.length" class="ttags">
            <span v-for="tag in t.tags.slice(0, 2)" :key="tag">{{ tag }}</span>
            <span v-if="t.tags.length > 2" class="more">+{{ t.tags.length - 2 }}</span>
          </div>
          <div v-else class="ttype">{{ typeLabels[t.type] }} · {{ catName(t.categoryId) }}</div>
        </div>
        <div v-if="sec.addCard" class="card add-card" tabindex="0" title="添加工具" @click="emit('add')" @keydown.enter.prevent="emit('add')">
          <div class="ticon add">＋</div>
          <div class="tname">添加</div>
          <div class="ttype">拖入或点击</div>
        </div>
      </div>
      <div v-if="sec.addCard && !sec.tools.length && !sec.folded" class="empty-hint">把 .app / .jar / .py / .sh 拖进面板即可收录</div>
    </section>
    <button v-if="addRow" class="grid-add-row" @click="emit('add')" @keydown.enter.prevent="emit('add')">
      ＋ 添加工具到「{{ addRow }}」（拖文件进面板也可）
    </button>
    <div v-if="dragging" class="dropzone-hint">松手即可收录到「{{ activeCategoryName }}」</div>
  </div>
</template>
