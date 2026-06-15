import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchActiveDrones, fetchAlerts } from '@/utils/api'
import logger from '@/utils/logger'

export const useDroneStore = defineStore('drones', () => {
  const activeDrones = ref([])
  const alerts = ref([]) // ✅ 核心修复：确保初始化为数组

  const loadActiveDrones = async () => {
    try {
      const response = await fetchActiveDrones()
      const list = Array.isArray(response) ? response : response?.drones || []
      activeDrones.value = list
      return list
    } catch (e) {
      logger.error('Failed to load active drones:', e)
      return []
    }
  }

  const cleanStaleDrones = () => {
    const now = Date.now()
    activeDrones.value = activeDrones.value.filter((d) => {
      if (!d.last_seen) return true
      const lastSeenTime = new Date(d.last_seen).getTime()
      return now - lastSeenTime < 60000
    })
  }

  const loadAlerts = async () => {
    try {
      const response = await fetchAlerts()
      const list = Array.isArray(response) ? response : response?.alerts || response?.data || []
      alerts.value = list
    } catch (e) {
      logger.error('Failed to load alerts:', e)
      alerts.value = []
    }
  }

  const updateDrone = (mac, data) => {
    const idx = activeDrones.value.findIndex((d) => d.mac === mac)
    const updatedData = { ...data, last_seen: new Date().toISOString() }
    if (idx !== -1) {
      activeDrones.value[idx] = { ...activeDrones.value[idx], ...updatedData }
    } else {
      activeDrones.value.push({ mac, ...updatedData })
    }
  }

  return { activeDrones, alerts, loadActiveDrones, cleanStaleDrones, loadAlerts, updateDrone }
})
