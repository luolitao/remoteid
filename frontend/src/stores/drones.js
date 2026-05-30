// src/stores/drones.js - 无人机状态管理 store
import { defineStore } from 'pinia'
import { fetchActiveDrones, fetchAlerts } from '@/utils/api'
import { isRecent } from '@/utils/helpers'

export const useDroneStore = defineStore('drones', {
  state: () => ({
    drones: [],
    selectedDrone: null,
    alerts: [],
    isPlaying: false,
    timelinePosition: 0,
    trajectoryData: []
  }),

  getters: {
    // 显示最近 30 分钟内活跃的无人机
    activeDrones: (state) => state.drones.filter(drone =>
      drone && isRecent(drone.last_seen, 30)
    ),

    recentDrones: (state) => state.drones.filter(drone =>
      drone && isRecent(drone.last_seen, 30)
    ),

    alertCount: (state) => state.alerts.length
  },

  actions: {
    async loadActiveDrones() {
      try {
        const drones = await fetchActiveDrones()
        // 过滤 null 元素并合并现有数据
        if (Array.isArray(drones)) {
          drones.filter(Boolean).forEach(droneData => {
            this.updateDrone(droneData)
          })
        }
        return drones || []
      } catch (error) {
        console.error('Error loading drones:', error)
        return []
      }
    },

    async loadAlerts() {
      try {
        const alerts = await fetchAlerts(10)
        // 过滤掉 null/undefined 元素，确保始终是数组
        this.alerts = Array.isArray(alerts) ? alerts.filter(Boolean) : []
        return this.alerts
      } catch (error) {
        console.error('Error loading alerts:', error)
        return []
      }
    },

    async initialize() {
      await Promise.all([this.loadActiveDrones(), this.loadAlerts()])
    },

    updateDrone(droneData) {
      if (!droneData || !droneData.mac) return
      const existingIndex = this.drones.findIndex(d => d.mac === droneData.mac)
      if (existingIndex !== -1) {
        this.drones[existingIndex] = { ...this.drones[existingIndex], ...droneData }
      } else {
        this.drones.push(droneData)
      }
    },

    removeDrone(mac) {
      this.drones = this.drones.filter(drone => drone.mac !== mac)
    },

    // 清理超过 30 分钟未活动的无人机
    cleanStaleDrones() {
      const cutoff = Date.now() - 30 * 60 * 1000
      this.drones = this.drones.filter(drone => {
        if (!drone || !drone.last_seen) return false
        const lastSeen = new Date(drone.last_seen).getTime()
        return !isNaN(lastSeen) && lastSeen > cutoff
      })
    },

    setAlerts(alerts) {
      this.alerts = alerts
    },

    setSelectedDrone(drone) {
      this.selectedDrone = drone
    },

    setTimelinePosition(position) {
      this.timelinePosition = position
    },

    setPlaying(isPlaying) {
      this.isPlaying = isPlaying
    },

    setTrajectoryData(trajectory) {
      this.trajectoryData = trajectory
    }
  }
})
