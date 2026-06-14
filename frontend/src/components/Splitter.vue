<template>
  <div id="splitter" title="Drag to resize" @mousedown="startResize"></div>
</template>

<script setup>
const props = defineProps({
  width: { type: Number, default: 350 },
})
const emit = defineEmits(['update:width'])
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
