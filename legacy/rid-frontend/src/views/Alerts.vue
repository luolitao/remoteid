<template>
  <div class="alerts">
    <h1>System Alerts</h1>
    <div class="alerts-list">
      <div v-for="alert in alerts" :key="alert.id" class="alert-item">
        <h3>{{ alert.type }}</h3>
        <p>{{ alert.message }}</p>
        <small>{{ alert.timestamp }}</small>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const alerts = ref([
  {
    id: 1,
    type: 'Non-Compliant',
    message: 'Non-compliant drone detected: AA:BB:CC:DD:EE:FF',
    timestamp: '2025-12-20 12:00:00'
  }
])
</script>
