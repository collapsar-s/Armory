<script setup>
// 长名称悬浮提示（.name-tip）：由 App 控制，fixed 定位在目标元素下方/上方。
defineProps({
  show: { type: Boolean, default: false },
  text: { type: String, default: '' },
  x: { type: Number, default: 0 },
  y: { type: Number, default: 0 },
})
</script>

<template>
  <div class="name-tip" :class="{ show }" :style="{ left: x + 'px', top: y + 'px' }">{{ text }}</div>
</template>
