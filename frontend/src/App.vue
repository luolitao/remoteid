<template>
  <div id="layout_container">
    <!-- 左侧：地图区域 -->
    <MapArea v-show="mapOpen" ref="mapAreaRef" v-model:selected-drone="selectedDrone" />

    <!-- 中间：拖拽分隔条 -->
    <Splitter v-show="sidebarOpen && mapOpen" v-model:width="sidebarWidth" />

    <!-- 右侧：侧边栏 -->
    <Sidebar
      v-show="sidebarOpen"
      v-model:selected-drone="selectedDrone"
      :initial-width="sidebarWidth"
      @show-trajectories="mapAreaRef?.showAllTrajectories()"
      @clear-trajectories="mapAreaRef?.clearTrajectories()"
    />

    <!-- 隐藏状态下的恢复按钮 -->
    <button v-if="!mapOpen" class="restore-btn left-2" title="Show map" @click="mapOpen = true">
      🗺 Map
    </button>
    <button
      v-if="!sidebarOpen"
      class="restore-btn right-2"
      title="Show sidebar"
      @click="sidebarOpen = true"
    >
      ☰ Sidebar
    </button>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import MapArea from '@/components/MapArea.vue'
import Sidebar from '@/components/Sidebar.vue'
import Splitter from '@/components/Splitter.vue'
import { useDataSync } from '@/composables/useDataSync' // 见下方补充

// ---- 顶层 UI 状态 ----
const mapOpen = ref(true)
const sidebarOpen = ref(true)
const sidebarWidth = ref(350)

// ---- 跨组件共享状态 ----
// 当侧边栏或地图选中无人机时，双向绑定同步状态
const selectedDrone = ref(null)
const mapAreaRef = ref(null)

// ---- 初始化全局数据同步 (WebSocket + 轮询) ----
// 将原有的 refreshData 和 initWebSocket 逻辑抽离到此 composable 中
const { startSync, stopSync } = useDataSync()

onMounted(() => {
  startSync()
})

onUnmounted(() => {
  stopSync()
})
</script>

<style>
/* 仅保留最顶层的布局 CSS，其余已移至各组件内部 */
#layout_container {
  display: flex;
  height: 100vh;
  width: 100vw;
  overflow: hidden;
  background: var(--BGCOLOR1, #f5f5f5);
  position: relative;
}

.restore-btn {
  position: absolute;
  top: 12px;
  z-index: 9999;
  padding: 6px 12px;
  border-radius: 4px;
  background: var(--ACCENT, #3182ce);
  color: #fff;
  border: none;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
  cursor: pointer;
  font-size: 13px;
  font-weight: bold;
  transition: opacity 0.2s;
}
.restore-btn:hover {
  opacity: 0.9;
}
</style>
