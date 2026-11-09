<script setup>
// 左侧分类栏（Maye 式）：品牌 + 全部/最近 + 主分组（可拖拽排序、计数徽标）+ 新建入口。
//   - 每一项都支持右键菜单（emit contextmenu）与工具拖入改归属（emit drop-on）。
//   - 主分组自身可拖拽（emit cat-drag-start）在同层内排序。
//   - rail-pill 滑动高亮跟随当前激活项。
import { ref, watch, nextTick, onMounted } from 'vue'

// items: [{ id, name, kind:'all'|'recent'|'dir', count }]
const props = defineProps({
  items: { type: Array, required: true },
  activeCategory: { type: String, required: true },
  dropOkId: { type: String, default: '' },
})
const emit = defineEmits(['select', 'contextmenu', 'add', 'drop-on', 'drag-over', 'drag-leave', 'cat-drag-start'])

const railEl = ref(null)
const pill = ref({ top: '0px', height: '0px' })
const hidden = ref(false)

function syncPill() {
  const rail = railEl.value
  if (!rail) return
  const el = rail.querySelector('.rail-item.active')
  if (!el) {
    hidden.value = true
    return
  }
  hidden.value = false
  pill.value = { top: el.offsetTop + 'px', height: el.offsetHeight + 'px' }
}

function onClick(item, e) {
  emit('select', item)
  const el = e.currentTarget
  if (el) {
    hidden.value = false
    pill.value = { top: el.offsetTop + 'px', height: el.offsetHeight + 'px' }
  }
}

function onDragStart(item, e) {
  // 仅主分组可拖拽排序（全部/最近固定）
  if (item.kind !== 'dir') return
  emit('cat-drag-start', item.id, e)
}

// items 为 computed 数组：任何变化（重排/计数更新）都会产生新引用 → 同步 pill。
// 只 watch length 会漏掉「条目数不变的重排」导致高亮条位置过期。
watch(() => props.items, () => nextTick(syncPill))
watch(() => props.activeCategory, () => nextTick(syncPill))
onMounted(() => nextTick(syncPill))
</script>

<template>
  <div class="sidebar">
    <div class="brand"><span>Armory</span></div>
    <nav ref="railEl" class="rail">
      <div class="rail-pill" :class="{ hidden }" :style="pill"></div>
      <template v-for="item in items" :key="item.id">
        <!-- 全部/最近：图标 -->
        <button
          v-if="item.kind === 'all' || item.kind === 'recent'"
          class="rail-item"
          :class="{ active: item.id === activeCategory }"
          @click="onClick(item, $event)"
        >
          <svg v-if="item.kind === 'all'" viewBox="0 0 24 24">
            <path d="M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z" />
          </svg>
          <svg v-else viewBox="0 0 24 24">
            <path d="M12 2a10 10 0 100 20 10 10 0 000-20zm0 2a8 8 0 110 16 8 8 0 010-16zm-1 3v6l5 2.5.8-1.6-4.3-2.1V7H11z" />
          </svg>
          <span>{{ item.name }}</span>
        </button>
        <!-- 主分组：文件夹图标 + 计数 + 拖拽排序 + 工具拖入 -->
        <button
          v-else
          class="rail-item"
          :class="{ active: item.id === activeCategory, 'drop-ok': dropOkId === item.id }"
          draggable="true"
          @click="onClick(item, $event)"
          @contextmenu.prevent="emit('contextmenu', item, $event)"
          @dragstart="onDragStart(item, $event)"
          @dragover.prevent="emit('drag-over', item.id, $event)"
          @dragleave="emit('drag-leave', item.id)"
          @drop.prevent="emit('drop-on', item.id, $event)"
        >
          <svg class="folder-ico" viewBox="0 0 24 24" aria-hidden="true">
            <path d="M2.5 6.2c0-.9.7-1.7 1.7-1.7h4.9c.5 0 .9.2 1.2.5l1.9 2h7.6c.9 0 1.7.8 1.7 1.7v.8H2.5V6.2z" fill="#3E8FD8" />
            <rect x="2.5" y="8.2" width="19" height="11.3" rx="2" fill="#5FB2F0" />
            <rect x="2.5" y="8.2" width="19" height="3" rx="1.5" fill="#ffffff" opacity=".28" />
          </svg>
          <span>{{ item.name }}</span>
          <!-- 计数徽标：有工具才显示（0 不显示，空分组保持简洁） -->
          <span v-if="item.count" class="rail-count">{{ item.count }}</span>
        </button>
      </template>
      <button class="rail-item rail-add" title="新建分类" @click="emit('add')">
        <svg viewBox="0 0 24 24"><path d="M11 5h2v6h6v2h-6v6h-2v-6H5v-2h6z" /></svg>
        <span>新建</span>
      </button>
    </nav>
  </div>
</template>
