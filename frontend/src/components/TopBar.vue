<script setup>
// 顶栏：搜索框（含放大镜图标与 ⌥Space 提示）+ 添加 / 设置 按钮（内联 SVG，原版 1:1）。
defineProps({
  query: { type: String, required: true },
})
const emit = defineEmits(['update:query', 'keydown', 'add', 'settings'])
</script>

<template>
  <header class="topbar" style="--wails-draggable: drag" data-wails-draggable>
    <div class="searchbox">
      <svg viewBox="0 0 24 24">
        <path d="M10 2a8 8 0 105.3 14l4.9 4.9 1.4-1.4-4.9-4.9A8 8 0 0010 2zm0 2a6 6 0 110 12 6 6 0 010-12z" />
      </svg>
      <input
        id="search-input"
        :value="query"
        type="text"
        placeholder="搜索工具，或直接键入启动…"
        autocomplete="off"
        spellcheck="false"
        @input="emit('update:query', $event.target.value)"
        @keydown="emit('keydown', $event)"
      />
      <span class="kbd">⌥Space</span>
    </div>
    <button class="iconbtn" data-tip="添加工具" title="添加工具" @click="emit('add')">
      <svg viewBox="0 0 24 24"><path d="M11 5h2v6h6v2h-6v6h-2v-6H5v-2h6z" /></svg>
    </button>
    <button class="iconbtn" data-tip="设置" title="设置" @click="emit('settings')">
      <svg viewBox="0 0 24 24">
        <path d="M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.09.63-.09.94s.02.64.07.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z" />
      </svg>
    </button>
  </header>
</template>
