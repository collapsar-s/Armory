<script setup>
// 右键浮动上下文菜单（.ctx-menu）。由 App 控制显隐与位置，菜单项由 App 传入。
// items: [{ label, danger?, disabled?, action }]
// 定位与原版一致：先渲染在 -9999px 处测量实际尺寸，再钳制到视口内
// （底部/右侧放不下时上移/左移，永不溢出面板边缘）。
// 任意菜单项点击后自动关闭（原版每个 action 都先关菜单再执行）。
import { ref, watch, nextTick } from 'vue'

const props = defineProps({
  show: { type: Boolean, default: false },
  title: { type: String, default: '' },
  x: { type: Number, default: 0 },
  y: { type: Number, default: 0 },
  items: { type: Array, default: () => [] },
})
const emit = defineEmits(['close'])

const el = ref(null)
const pos = ref({ left: '-9999px', top: '0px' })

// 每次打开：先放到触发点渲染，再自校准位置。
// 背景：应用在 <html> 上设置了 zoom（fontScale，默认 1.1），而 .ctx-menu 处于
// zoom 子树内——style 里的 top/left 是本地单位（可视 = 本地 × zoom），
// offsetHeight 也是本地单位，直接拿 window.innerHeight 钳制会差一个 zoom 比例。
// 自校准：getBoundingClientRect 永远返回可视坐标，与 offsetWidth 之比即实测
// zoom；按可视溢出量 ÷ zoom 折算回本地修正——对任何 zoom 值都成立。
watch(
  () => props.show,
  (v) => {
    if (!v) return
    pos.value = { left: props.x + 'px', top: props.y + 'px' }
    nextTick(() => {
      const m = el.value
      if (!m) return
      const rect = m.getBoundingClientRect()
      const z = rect.width / m.offsetWidth || 1 // 实测 zoom 比例
      let dx = 0
      let dy = 0
      if (rect.left < 8) dx = rect.left - 8
      else if (rect.right > window.innerWidth - 8) dx = rect.right - (window.innerWidth - 8)
      if (rect.top < 8) dy = rect.top - 8
      else if (rect.bottom > window.innerHeight - 8) dy = rect.bottom - (window.innerHeight - 8)
      if (dx || dy) {
        pos.value = { left: props.x - dx / z + 'px', top: props.y - dy / z + 'px' }
      }
    })
  },
)

function onItem(it) {
  emit('close') // 先关菜单再执行动作（与原版一致，避免删除后菜单残留）
  it.action && it.action()
}
</script>

<template>
  <template v-if="show">
    <div class="ctx-mask" @click="emit('close')" @contextmenu.prevent="emit('close')"></div>
    <div ref="el" class="ctx-menu" :style="pos">
      <div class="ctx-title">{{ title }}</div>
      <button
        v-for="(it, i) in items"
        :key="i"
        :class="{ danger: it.danger }"
        :disabled="it.disabled"
        @click="onItem(it)"
      >{{ it.label }}</button>
    </div>
  </template>
</template>
