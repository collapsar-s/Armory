<script setup>
// 命令视图：按「主分组 → 工具」分区渲染（搜索中结果按命中工具各自的主分组归节）。
// 分区头（sec-head）交互：
//   - 点击进入该分类（命令视图下会切回网格视图，由 App 处理）；
//   - 右键打开分类菜单；非当前分区有 tooltip（点击进入 · 右键管理）；
//   - 可作为拖放目标（工具拖入改归属）。
// 名称命中搜索词用 <mark> 高亮；键盘上下选择在 flatTools 线性顺序上进行。
import { typeEmojis } from '../composables/useApi'

defineProps({
  sections: { type: Array, required: true }, // [{id,name,kind,parentId,tools}]
  icons: { type: Object, required: true },
  selIdx: { type: Number, default: 0 },
  query: { type: String, default: '' },
  activeCategory: { type: String, default: 'all' },
  catName: { type: Function, required: true },
  catPath: { type: Function, required: true },
  markName: { type: Function, required: true },
  flatIndex: { type: Function, required: true },
})
const emit = defineEmits(['launch', 'edit', 'select', 'nav', 'cat-menu', 'drag-over', 'drag-leave', 'drop-on'])
</script>

<template>
  <div class="view enter-cmd">
    <div class="cmdlist">
      <template v-for="sec in sections" :key="sec.id">
        <!-- 分区标题 -->
        <div
          v-if="sec.name"
          class="sec-head"
          :class="{ link: sec.id !== activeCategory }"
          :title="sec.id !== activeCategory ? '点击进入 · 右键管理' : ''"
          @click="emit('nav', sec)"
          @contextmenu.prevent.stop="emit('cat-menu', sec, $event)"
          @dragover.prevent.stop="emit('drag-over', sec.id, $event)"
          @dragleave="emit('drag-leave', sec.id)"
          @drop.prevent.stop="emit('drop-on', sec.id, $event)"
        >
          <span class="sec-name">{{ sec.name }}</span>
          <span class="sec-count">{{ sec.tools.length }}</span>
        </div>
        <!-- 分区工具 -->
        <div
          v-for="t in sec.tools"
          :key="t.id"
          class="cmdrow"
          :class="{ active: flatIndex(t) === selIdx }"
          :style="{ animationDelay: Math.min(flatIndex(t), 8) * 40 + 'ms' }"
          @click="emit('launch', t)"
          @mouseenter="emit('select', flatIndex(t))"
          @contextmenu.prevent="emit('edit', t, $event)"
        >
          <div class="ticon" :class="t.type">
            <img v-if="icons[t.id]" :src="icons[t.id]" :alt="t.name" />
            <span v-else>{{ typeEmojis[t.type] }}</span>
          </div>
          <div class="cinfo">
            <div class="cname" v-html="markName(t.name)"></div>
            <div class="cpath">{{ catPath(t.categoryId) }} · {{ t.path }}<template v-if="t.args">{{ t.args }}</template></div>
          </div>
          <span class="ckbd">⏎ 启动 · ⌘⏎ 编辑</span>
        </div>
      </template>
      <div v-if="!sections.length || sections.every((s) => !s.tools.length)" class="cmd-empty">没有匹配「{{ query }}」的工具</div>
    </div>
  </div>
</template>
