// src/composables/useMap.js
import { ref, watch, onMounted, onUnmounted } from 'vue'
import * as L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { setupLeafletIcons, createDroneIcon } from '@/utils/leaflet-setup'
import { useDroneStore } from '@/stores/drones'

setupLeafletIcons()

const TDT_TOKEN = '66ad6f2f6216ab57401ed9907e94cd43'

export function useMap(mapContainerRef) {
  const map = ref(null)
  const droneMarkers = ref({})
  const currentMapType = ref('tianditu')
  const store = useDroneStore()

  const mapTypes = {
    amap: {
      url: '/amap-vec?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}',
      attribution: '&copy; 高德地图',
    },
    amapImg: {
      url: '/amap-img?style=6&x={x}&y={y}&z={z}',
      annotationUrl: '/amap-img?style=8&x={x}&y={y}&z={z}',
      attribution: '&copy; 高德地图',
    },
    osm: {
      url: 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',
      attribution: '&copy; OpenStreetMap',
    },
    tianditu: {
      url: `https://t{s}.tianditu.gov.cn/vec_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=vec&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
      annotationUrl: `https://t{s}.tianditu.gov.cn/cva_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=cva&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
      subdomains: ['0', '1', '2', '3', '4', '5', '6', '7'],
      attribution: '&copy; 天地图',
    },
    tiandituImg: {
      url: `https://t{s}.tianditu.gov.cn/img_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=img&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
      annotationUrl: `https://t{s}.tianditu.gov.cn/cva_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=cva&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}&tk=${TDT_TOKEN}`,
      subdomains: ['0', '1', '2', '3', '4', '5', '6', '7'],
      attribution: '&copy; 天地图',
    },
  }

  const initMap = () => {
    if (!mapContainerRef.value || map.value) return
    const rect = mapContainerRef.value.getBoundingClientRect()
    if (rect.width === 0 || rect.height === 0) {
      setTimeout(initMap, 100)
      return
    }

    map.value = L.map(mapContainerRef.value, {
      zoomControl: false,
      attributionControl: true,
    }).setView([23.14287, 113.26026], 18)
    L.control.zoom({ position: 'bottomright' }).addTo(map.value)
    updateMapLayer()
    setTimeout(() => map.value?.invalidateSize(), 300)
  }

  const updateMapLayer = () => {
    if (!map.value) return
    map.value.eachLayer((layer) => {
      if (layer instanceof L.TileLayer) map.value.removeLayer(layer)
    })

    const config = mapTypes[currentMapType.value]
    L.tileLayer(config.url, {
      subdomains: config.subdomains,
      attribution: config.attribution,
    }).addTo(map.value)
    if (config.annotationUrl) {
      L.tileLayer(config.annotationUrl, { subdomains: config.subdomains, attribution: '' }).addTo(
        map.value,
      )
    }
  }

  const updateMarkers = () => {
    if (!map.value) return
    const drones = store.activeDrones
    const activeMacs = new Set(drones.map((d) => d.mac))

    // 移除消失的无人机
    Object.keys(droneMarkers.value).forEach((mac) => {
      if (!activeMacs.has(mac)) {
        map.value.removeLayer(droneMarkers.value[mac])
        delete droneMarkers.value[mac]
      }
    })

    // 更新或创建 Marker
    drones.forEach((drone) => {
      if (!drone.latitude || !drone.longitude) return
      const color = drone.standard === 'ASTM F3411-22a' ? 'blue' : 'green'
      const popupContent = `<div style="font-size:13px;"><b>${drone.uas_id || 'Unknown'}</b><br>MAC: ${drone.mac}<br>Alt: ${drone.altitude ? drone.altitude.toFixed(1) : 'N/A'}m</div>`

      if (droneMarkers.value[drone.mac]) {
        const marker = droneMarkers.value[drone.mac]
        marker.setLatLng([drone.latitude, drone.longitude])
        marker.setIcon(createDroneIcon(color, 12))
        marker.setPopupContent(popupContent)
      } else {
        const marker = L.marker([drone.latitude, drone.longitude], {
          icon: createDroneIcon(color, 12),
        }).addTo(map.value)
        marker.bindPopup(popupContent)
        droneMarkers.value[drone.mac] = marker
      }
    })
  }

  // 监听 Store 中的无人机数据变化，自动更新地图
  watch(() => store.activeDrones, updateMarkers, { deep: true })

  onMounted(initMap)
  onUnmounted(() => {
    if (map.value) {
      map.value.remove()
      map.value = null
    }
  })

  return { map, currentMapType, updateMapLayer }
}
