<template>
  <div id="splitter" title="Drag to resize" @mousedown="startResize"></div>
</template>

<script setup>
const props = defineProps({
  width: { type: Number, default: 350 },
})
const emit = defineEmits(['update:width'])

// ---- 侧边栏拖拽（增强版：防止文本选中中断） ----
let isResizing = false

const startResize = (e) => {
  isResizing = true
  // 🎯 核心修复：拖拽时禁用文本选中，并改变鼠标指针
  document.body.style.userSelect = 'none'
  document.body.style.cursor = 'ew-resize'

  document.addEventListener('mousemove', doResize)
  document.addEventListener('mouseup', stopResize)
  e.preventDefault() // 阻止默认行为
}

const doResize = (e) => {
  if (!isResizing) return
  // 计算右侧边栏宽度，并限制在 250px 到 600px 之间
  const w = Math.max(250, Math.min(600, window.innerWidth - e.clientX))
  sidebarWidth.value = w
}

const stopResize = () => {
  isResizing = false
  // 🎯 核心修复：恢复文本选中和默认鼠标指针
  document.body.style.userSelect = ''
  document.body.style.cursor = ''

  document.removeEventListener('mousemove', doResize)
  document.removeEventListener('mouseup', stopResize)
}
</script>

<style scoped>
/* ---- 拖拽手柄 ---- */
#splitter {
  flex-shrink: 0;
  width: 6px; /* 稍微加宽一点，更容易抓取 */
  background: var(--BGCOLOR2);
  cursor: ew-resize;
  position: relative; /* 🎯 核心修复：确保 z-index 生效 */
  z-index: 1000; /* 🎯 核心修复：提高层级，防止被地图或侧边栏遮挡 */
  transition: background 0.2s;
}

#splitter:hover {
  background: var(--ACCENT); /* 鼠标悬停时高亮，提示可拖拽 */
}
</style>
