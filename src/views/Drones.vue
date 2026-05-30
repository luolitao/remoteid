<template>
  <div class="drones">
    <h1>Active Drones</h1>
    <div class="drones-list">
      <div v-for="drone in drones" :key="drone.mac" class="drone-card">
        <h3>{{ drone.uas_id }}</h3>
        <p>MAC: {{ drone.mac }}</p>
        <p>Type: {{ drone.ua_type }}</p>
        <p>Position: {{ drone.latitude }}, {{ drone.longitude }}</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'

const drones = ref([])

onMounted(() => {
  // 模拟数据，实际项目中从 API 获取
  drones.value = [
    {
      mac: 'AA:BB:CC:DD:EE:FF',
      uas_id: 'Drone-001',
      ua_type: 'Multirotor',
      latitude: 23.14287,
      longitude: 113.26026
    }
  ]
})
</script>
