// src/stores/drones.js
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchActiveDrones, fetchAlerts } from '@/utils/api' // 请确保你的 api.js 中有这两个方法
import logger from '@/utils/logger'

export const useDroneStore = defineStore('drones', () => {
  // ✅ 核心修复：确保初始化为空数组，彻底杜绝 .filter() 报错
  const activeDrones = ref([])
  const alerts = ref([])

  // 加载活跃无人机列表
  const loadActiveDrones = async () => {
    try {
      const response = await fetchActiveDrones()
      // 兼容后端返回 { drones: [...] } 或直接返回 [...] 的情况
      const list = Array.isArray(response) ? response : (response?.drones || [])
      activeDrones.value = list
      return list
    } catch (e) {
      logger.error('Failed to load active drones:', e)
      return []
    }
  }

  // 清理过期（超过60秒未更新）的无人机
  const cleanStaleDrones = () => {
    const now = Date.now()
    activeDrones.value = activeDrones.value.filter(d => {
      if (!d.last_seen) return true
      const lastSeenTime = new Date(d.last_seen).getTime()
      return (now - lastSeenTime) < 60000 // 60秒
    })
  }

  // 加载警报列表
  const loadAlerts = async () => {
    try {
      const response = await fetchAlerts()
      const list = Array.isArray(response) ? response : (response?.alerts || response?.data || [])
      alerts.value = list
    } catch (e) {
      logger.error('Failed to load alerts:', e)
      alerts.value = []
    }
  }

  // 供 WebSocket 实时更新单架无人机数据使用
  const updateDrone = (mac, data) => {
    const idx = activeDrones.value.findIndex(d => d.mac === mac)
    const updatedData = { ...data, last_seen: new Date().toISOString() }
    
    if (idx !== -1) {
      // 更新现有无人机
      activeDrones.value[idx] = { ...activeDrones.value[idx], ...updatedData }
    } else {
      // 新增无人机
      activeDrones.value.push({ mac, ...updatedData })
    }
  }

  return {
    activeDrones,
    alerts,
    loadActiveDrones,
    cleanStaleDrones,
    loadAlerts,
    updateDrone
  }
})