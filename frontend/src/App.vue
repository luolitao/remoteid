<template>
  <div id="layout_container">
    <!-- 左侧地图区域 -->
    <MapArea 
      v-show="mapOpen" 
      ref="mapAreaRef"
      v-model:selectedDrone="selectedDrone"
      @show-trajectories="mapAreaRef?.showAllTrajectories()"
      @clear-trajectories="mapAreaRef?.clearTrajectories()"
    />

    <!-- 拖拽手柄 -->
    <Splitter v-model:width="sidebarWidth" />

    <!-- 右侧边栏 -->
    <Sidebar
      v-show="sidebarOpen"
      ref="sidebarRef"
      v-model:selectedDrone="selectedDrone"
      :initial-width="sidebarWidth"
      @show-trajectories="mapAreaRef?.showSelectedTrajectory()"
      @clear-trajectories="mapAreaRef?.clearTrajectories()"
    />

    <!-- 隐藏状态下的恢复按钮 -->
    <button
      v-if="!mapOpen"
      class="restore-btn"
      style="position: absolute; top: 12px; left: 12px; z-index: 10000"
      title="Show map"
      @click="mapOpen = true"
    >
      🗺 Show Map
    </button>

    <button
      v-if="!sidebarOpen"
      class="restore-btn"
      style="position: absolute; top: 12px; right: 12px; z-index: 10000"
      title="Show sidebar"
      @click="sidebarOpen = true"
    >
      ☰ Show Sidebar
    </button>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import MapArea from '@/components/MapArea.vue'
import Sidebar from '@/components/Sidebar.vue'
import Splitter from '@/components/Splitter.vue'
import { useDataSync } from '@/composables/useDataSync'

// 顶层 UI 状态
const mapOpen = ref(true)
const sidebarOpen = ref(true)
const sidebarWidth = ref(350)
const selectedDrone = ref(null)
const mapAreaRef = ref(null)

// 全局数据同步
const { startSync, stopSync } = useDataSync()

onMounted(() => startSync())
onUnmounted(() => stopSync())
</script>

<style>
/* 仅保留最基础的布局 CSS */
#layout_container {
  display: flex;
  height: 100vh;
  overflow: hidden;
  background: var(--BGCOLOR1, #f5f5f5);
}

.restore-btn {
  padding: 6px 12px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.95);
  border: 1px solid var(--BGCOLOR2, #ccc);
  cursor: pointer;
  color: var(--TXTCOLOR2, #666);
  font-weight: bold;
  font-size: 12px;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
  transition: all 0.2s ease;
}
.restore-btn:hover {
  background: var(--ACCENT, #3182ce);
  color: #fff;
}
</style>
