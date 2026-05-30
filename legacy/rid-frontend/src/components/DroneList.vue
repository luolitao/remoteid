<!-- src/components/DroneList.vue -->
<template>
  <div class="space-y-4">
    <div class="flex justify-between items-center">
      <h2 class="text-lg font-bold flex items-center">
        <DroneIcon class="mr-2 text-blue-500" />
        Active Drones ({{ recentDrones.length }})
      </h2>
      <button 
        @click="refreshData"
        class="p-1 hover:bg-gray-100 rounded transition"
        title="Refresh"
      >
        <RefreshIcon class="w-4 h-4 text-gray-600" />
      </button>
    </div>
    
    <div v-if="loading" class="text-center py-4">
      <Spinner class="w-6 h-6 mx-auto text-blue-500" />
    </div>
    
    <div v-else-if="recentDrones.length === 0" class="text-center text-gray-500 py-4">
      <NoDataIcon class="w-12 h-12 mx-auto mb-2 text-gray-400" />
      <p>No active drones detected</p>
    </div>
    
    <div v-else class="max-h-[400px] overflow-y-auto space-y-3">
      <div 
        v-for="drone in recentDrones" 
        :key="drone.mac"
        class="border rounded-lg p-3 hover:bg-blue-50 cursor-pointer transition"
        @click="selectDrone(drone)"
      >
        <div class="flex justify-between items-start">
          <div>
            <div class="font-bold text-blue-600 truncate max-w-[150px]">
              {{ drone.uas_id || 'Unknown Drone' }}
            </div>
            <div class="text-sm text-gray-600">MAC: {{ shortenMac(drone.mac) }}</div>
            <div class="flex flex-wrap gap-2 mt-1">
              <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800">
                {{ drone.ua_type || 'Unknown' }}
              </span>
              <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800">
                Alt: {{ drone.altitude ? drone.altitude.toFixed(1) : 'N/A' }}m
              </span>
            </div>
          </div>
          <div class="text-right">
            <div class="text-xs text-gray-500">
              {{ timeAgo(drone.last_seen) }}
            </div>
            <div class="mt-1 flex justify-end">
              <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium" 
                    :class="isRecent(drone.last_seen) ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'">
                {{ isRecent(drone.last_seen) ? 'Active' : 'Offline' }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue';
import { useDroneStore } from '@/stores/drones';
import DroneIcon from '@/components/icons/DroneIcon.vue';
import RefreshIcon from '@/components/icons/RefreshIcon.vue';
import Spinner from '@/components/icons/Spinner.vue';
import NoDataIcon from '@/components/icons/NoDataIcon.vue';

const store = useDroneStore();
const { activeDrones, alerts, recentDrones, loadActiveDrones, initialize } = store;
const loading = ref(true);

onMounted(() => {
  console.log('Initializing DroneList...');
  initialize();
  
  // 定期刷新
  const interval = setInterval(() => {
    loadActiveDrones();
  }, 5000);
  
  // 初始加载
  setTimeout(() => {
    loadActiveDrones().finally(() => {
      loading.value = false;
    });
  }, 100);
  
  return () => clearInterval(interval);
});

const refreshData = () => {
  loading.value = true;
  loadActiveDrones().finally(() => {
    loading.value = false;
  });
};

const selectDrone = (drone) => {
  console.log('Selected drone:', drone);
  // 未来：触发地图聚焦
};

const shortenMac = (mac) => {
  if (!mac) return 'Unknown';
  const parts = mac.split(':');
  return `${parts[0]}:${parts[1]}:${parts[2]}:...:${parts[3]}:${parts[4]}:${parts[5]}`;
};

const timeAgo = (timestamp) => {
  if (!timestamp) return 'Unknown';
  const now = new Date();
  const then = new Date(timestamp);
  if (isNaN(then.getTime())) return 'Invalid time';
  
  const diff = Math.floor((now - then) / 1000);
  
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff/60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff/3600)}h ago`;
  return `${Math.floor(diff/86400)}d ago`;
};

const isRecent = (timestamp) => {
  if (!timestamp) return false;
  const now = new Date();
  const then = new Date(timestamp);
  return (now - then) < 300000; // 5分钟
};
</script>
