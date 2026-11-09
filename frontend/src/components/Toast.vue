<script setup>
// 底部 toast 提示。
defineProps({
  show: { type: Boolean, default: false },
  msg: { type: String, default: '' },
})
</script>

<template>
  <div class="toast" :class="{ show }"><span class="dot"></span>{{ msg }}</div>
</template>
