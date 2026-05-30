<template>
  <div class="home">
    <h1>Remote ID Monitor</h1>
    <p>Real-time drone tracking and monitoring system</p>
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
      <div class="bg-white p-6 rounded-lg shadow md:col-span-2">
        <h3 class="text-xl font-bold mb-2">Live Map</h3>
        <div class="map-wrapper">
          <MapComponent />
        </div>
      </div>
      <div class="bg-white p-6 rounded-lg shadow">
        <h3 class="text-xl font-bold mb-2">Drone List</h3>
        <p>Active drones with details and status</p>
      </div>
      <div class="bg-white p-6 rounded-lg shadow md:col-span-3">
        <h3 class="text-xl font-bold mb-2">Alerts</h3>
        <p>Real-time alerts and notifications</p>
      </div>
    </div>
  </div>
</template>

<script setup>
// Home view component
import MapComponent from '@/components/MapComponent.vue'
</script>

<style scoped>
.map-wrapper {
  height: 400px;
  width: 100%;
}
</style>