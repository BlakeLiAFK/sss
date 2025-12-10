<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title">
        <h1>{{ t('dashboard.title') }}</h1>
        <p class="page-subtitle">{{ t('dashboard.subtitle') }}</p>
      </div>
      <div class="page-actions">
        <el-button @click="loadStats" :loading="loading" class="refresh-btn">
          <el-icon><Refresh /></el-icon>
          <span class="btn-text">{{ t('common.refresh') }}</span>
        </el-button>
      </div>
    </div>

    <!-- 统计卡片 -->
    <div class="stats-cards" v-loading="loading">
      <div class="stat-card">
        <div class="stat-icon buckets">
          <el-icon :size="24"><Folder /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ stats?.stats.total_buckets || 0 }}</div>
          <div class="stat-label">{{ t('dashboard.buckets') }}</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon objects">
          <el-icon :size="24"><Document /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ stats?.stats.total_objects || 0 }}</div>
          <div class="stat-label">{{ t('dashboard.objects') }}</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon size">
          <el-icon :size="24"><Coin /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ formatSize(stats?.stats.total_size || 0) }}</div>
          <div class="stat-label">{{ t('dashboard.totalSize') }}</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon disk">
          <el-icon :size="24"><Box /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ stats?.disk_file_count || 0 }}</div>
          <div class="stat-label">{{ t('dashboard.files') }}</div>
        </div>
      </div>
    </div>

    <!-- 图表区域 -->
    <div class="charts-row">
      <!-- 各桶存储占比 -->
      <div class="content-card chart-card">
        <div class="card-header">
          <h3>{{ t('dashboard.bucketDistribution') }}</h3>
        </div>
        <div class="chart-container" ref="bucketChartRef"></div>
        <el-empty v-if="!loading && (!stats?.stats.bucket_stats || stats.stats.bucket_stats.length === 0)"
                  :description="t('common.noData')" :image-size="80" />
      </div>

      <!-- 文件类型分布 -->
      <div class="content-card chart-card">
        <div class="card-header">
          <h3>{{ t('dashboard.fileTypeDistribution') }}</h3>
        </div>
        <div class="chart-container" ref="typeChartRef"></div>
        <el-empty v-if="!loading && (!stats?.stats.type_stats || stats.stats.type_stats.length === 0)"
                  :description="t('common.noData')" :image-size="80" />
      </div>
    </div>

    <!-- 访问来源地图 -->
    <div class="content-card map-card" v-if="geoStatsEnabled">
      <div class="card-header">
        <h3>{{ t('dashboard.accessSourceMap') }}</h3>
        <div class="header-actions">
          <el-select v-model="mapDateRange" size="small" style="width: 120px" @change="loadGeoStatsData">
            <el-option :label="t('dashboard.last7Days')" value="7" />
            <el-option :label="t('dashboard.last30Days')" value="30" />
            <el-option :label="t('dashboard.last90Days')" value="90" />
          </el-select>
        </div>
      </div>
      <div class="map-stats-row" v-if="geoStatsSummary">
        <div class="map-stat">
          <span class="map-stat-value">{{ geoStatsSummary.total_requests?.toLocaleString() || 0 }}</span>
          <span class="map-stat-label">{{ t('dashboard.totalRequests') }}</span>
        </div>
        <div class="map-stat">
          <span class="map-stat-value">{{ geoStatsSummary.country_count || 0 }}</span>
          <span class="map-stat-label">{{ t('dashboard.countryCount') }}</span>
        </div>
        <div class="map-stat">
          <span class="map-stat-value">{{ geoStatsSummary.city_count || 0 }}</span>
          <span class="map-stat-label">{{ t('dashboard.cityCount') }}</span>
        </div>
      </div>
      <div class="map-container" ref="mapContainerRef"></div>
      <el-empty v-if="!mapLoading && geoStatsData.length === 0"
                :description="t('dashboard.noGeoStatsData')" :image-size="80" />

      <!-- 国家排行榜 -->
      <div class="geo-ranking" v-if="geoStatsData.length > 0">
        <div class="ranking-header">{{ t('dashboard.topCountries') }}</div>
        <div class="ranking-list">
          <div class="ranking-item" v-for="(item, index) in geoStatsData.slice(0, 10)" :key="item.country_code">
            <span class="ranking-index">{{ index + 1 }}</span>
            <span class="ranking-country">{{ item.country || item.country_code }}</span>
            <span class="ranking-count">{{ item.total?.toLocaleString() }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 桶详情表格 -->
    <div class="content-card table-card">
      <div class="card-header">
        <h3>{{ t('dashboard.bucketDetails') }}</h3>
      </div>
      <div class="table-wrapper">
        <el-table :data="stats?.stats.bucket_stats || []" class="data-table">
          <el-table-column prop="name" :label="t('dashboard.bucket')" min-width="150">
            <template #default="{ row }">
              <router-link :to="{ name: 'Objects', params: { name: row.name } }" class="bucket-link">
                <el-icon><Folder /></el-icon>
                <span>{{ row.name }}</span>
              </router-link>
            </template>
          </el-table-column>
          <el-table-column prop="object_count" :label="t('dashboard.objectCount')" width="100" align="right">
            <template #default="{ row }">
              {{ row.object_count.toLocaleString() }}
            </template>
          </el-table-column>
          <el-table-column prop="total_size" :label="t('dashboard.capacity')" width="100" align="right">
            <template #default="{ row }">
              {{ formatSize(row.total_size) }}
            </template>
          </el-table-column>
          <el-table-column :label="t('dashboard.access')" width="80" align="center">
            <template #default="{ row }">
              <el-tag :type="row.is_public ? 'warning' : 'info'" size="small">
                {{ row.is_public ? t('dashboard.public') : t('dashboard.private') }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('dashboard.percentage')" min-width="120" class-name="hide-on-mobile">
            <template #default="{ row }">
              <el-progress
                :percentage="getPercentage(row.total_size)"
                :stroke-width="6"
                :show-text="false"
                :color="'#e67e22'"
              />
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <!-- 最近上传 -->
    <div class="content-card table-card">
      <div class="card-header">
        <h3>{{ t('dashboard.recentUploads') }}</h3>
      </div>
      <div class="table-wrapper">
        <el-table :data="recentObjects" class="data-table">
          <el-table-column prop="key" :label="t('dashboard.file')" min-width="200">
            <template #default="{ row }">
              <span class="object-path">{{ row.key }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="size" :label="t('dashboard.size')" width="100" align="right">
            <template #default="{ row }">
              {{ formatSize(row.size) }}
            </template>
          </el-table-column>
          <el-table-column prop="last_modified" :label="t('dashboard.time')" width="160" class-name="hide-on-mobile">
            <template #default="{ row }">
              {{ formatDate(row.last_modified) }}
            </template>
          </el-table-column>
        </el-table>
      </div>
      <el-empty v-if="!loading && recentObjects.length === 0"
                :description="t('dashboard.noUploadRecords')" :image-size="80" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Refresh, Folder, Document, Coin, Box } from '@element-plus/icons-vue'
import {
  getStorageStats,
  getRecentObjects,
  getGeoStatsConfig,
  getGeoStatsData,
  getGeoStatsSummary,
  type StatsResponse,
  type RecentObject,
  type GeoStatsAggregated,
  type GeoStatsSummary
} from '../api/admin'
import * as echarts from 'echarts'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'

const { t } = useI18n()

const loading = ref(false)
const stats = ref<StatsResponse | null>(null)
const recentObjects = ref<RecentObject[]>([])

const bucketChartRef = ref<HTMLElement>()
const typeChartRef = ref<HTMLElement>()
let bucketChart: echarts.ECharts | null = null
let typeChart: echarts.ECharts | null = null

// GeoStats 相关
const geoStatsEnabled = ref(false)
const mapDateRange = ref('30')
const geoStatsData = ref<GeoStatsAggregated[]>([])
const geoStatsSummary = ref<GeoStatsSummary | null>(null)
const mapLoading = ref(false)
const mapContainerRef = ref<HTMLElement>()
let leafletMap: L.Map | null = null
let markersLayer: L.LayerGroup | null = null

// 国家坐标映射（主要国家）
const countryCoordinates: Record<string, [number, number]> = {
  'CN': [35.86, 104.19],
  'US': [37.09, -95.71],
  'JP': [36.20, 138.25],
  'KR': [35.91, 127.77],
  'DE': [51.17, 10.45],
  'GB': [55.38, -3.44],
  'FR': [46.23, 2.21],
  'IN': [20.59, 78.96],
  'BR': [-14.24, -51.93],
  'RU': [61.52, 105.32],
  'CA': [56.13, -106.35],
  'AU': [-25.27, 133.78],
  'IT': [41.87, 12.57],
  'ES': [40.46, -3.75],
  'MX': [23.63, -102.55],
  'ID': [-0.79, 113.92],
  'NL': [52.13, 5.29],
  'SA': [23.89, 45.08],
  'TR': [38.96, 35.24],
  'CH': [46.82, 8.23],
  'PL': [51.92, 19.15],
  'SE': [60.13, 18.64],
  'BE': [50.50, 4.47],
  'TH': [15.87, 100.99],
  'AR': [-38.42, -63.62],
  'ZA': [-30.56, 22.94],
  'SG': [1.35, 103.82],
  'HK': [22.40, 114.11],
  'TW': [23.70, 121.00],
  'MY': [4.21, 101.98],
  'PH': [12.88, 121.77],
  'VN': [14.06, 108.28],
  'AE': [23.42, 53.85],
  'EG': [26.82, 30.80],
  'NG': [9.08, 8.68],
  'PK': [30.38, 69.35],
  'BD': [23.68, 90.36],
  'UA': [48.38, 31.17],
  'CZ': [49.82, 15.47],
  'AT': [47.52, 14.55],
  'NO': [60.47, 8.47],
  'DK': [56.26, 9.50],
  'FI': [61.92, 25.75],
  'IE': [53.14, -7.69],
  'PT': [39.40, -8.22],
  'GR': [39.07, 21.82],
  'NZ': [-40.90, 174.89],
  'IL': [31.05, 34.85],
  'CL': [-35.68, -71.54],
  'CO': [4.57, -74.30],
  'PE': [-9.19, -75.02]
}

// 格式化文件大小
function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// 格式化日期
function formatDate(dateStr: string): string {
  const date = new Date(dateStr)
  return date.toLocaleString()
}

// 计算百分比
function getPercentage(size: number): number {
  if (!stats.value || stats.value.stats.total_size === 0) return 0
  return Math.round((size / stats.value.stats.total_size) * 100)
}


// 加载统计数据
async function loadStats() {
  loading.value = true
  try {
    const [statsData, recent] = await Promise.all([
      getStorageStats(),
      getRecentObjects(10)
    ])
    stats.value = statsData
    recentObjects.value = recent
    renderCharts()
  } catch (e: any) {
    ElMessage.error(t('dashboard.loadStatsFailed') + ': ' + (e.response?.data?.message || e.message))
  } finally {
    loading.value = false
  }
}

// 渲染图表
function renderCharts() {
  if (!stats.value) return

  // 桶存储占比饼图
  if (bucketChartRef.value && stats.value.stats.bucket_stats?.length > 0) {
    if (!bucketChart) {
      bucketChart = echarts.init(bucketChartRef.value)
    }
    bucketChart.setOption({
      tooltip: {
        trigger: 'item',
        formatter: (params: any) => `${params.name}: ${formatSize(params.value)} (${params.percent}%)`
      },
      legend: {
        orient: 'vertical',
        right: 10,
        top: 'center'
      },
      series: [{
        type: 'pie',
        radius: ['40%', '70%'],
        center: ['40%', '50%'],
        avoidLabelOverlap: false,
        itemStyle: {
          borderRadius: 6,
          borderColor: '#fff',
          borderWidth: 2
        },
        label: { show: false },
        emphasis: {
          label: { show: true, fontSize: 14, fontWeight: 'bold' }
        },
        data: stats.value.stats.bucket_stats.map((b, i) => ({
          name: b.name,
          value: b.total_size,
          itemStyle: { color: getChartColor(i) }
        }))
      }]
    })
  }

  // 文件类型分布饼图
  if (typeChartRef.value && stats.value.stats.type_stats?.length > 0) {
    if (!typeChart) {
      typeChart = echarts.init(typeChartRef.value)
    }
    typeChart.setOption({
      tooltip: {
        trigger: 'item',
        formatter: (params: any) => `${params.name}: ${formatSize(params.value)} (${params.percent}%)`
      },
      legend: {
        orient: 'vertical',
        right: 10,
        top: 'center'
      },
      series: [{
        type: 'pie',
        radius: ['40%', '70%'],
        center: ['40%', '50%'],
        avoidLabelOverlap: false,
        itemStyle: {
          borderRadius: 6,
          borderColor: '#fff',
          borderWidth: 2
        },
        label: { show: false },
        emphasis: {
          label: { show: true, fontSize: 14, fontWeight: 'bold' }
        },
        data: stats.value.stats.type_stats.slice(0, 8).map((t, i) => ({
          name: t.extension,
          value: t.total_size,
          itemStyle: { color: getChartColor(i + 4) }
        }))
      }]
    })
  }
}

// 图表颜色
function getChartColor(index: number): string {
  const colors = [
    '#e67e22', '#10b981', '#3b82f6', '#ef4444',
    '#8b5cf6', '#06b6d4', '#ec4899', '#f97316',
    '#14b8a6', '#6366f1', '#84cc16', '#a855f7'
  ]
  return colors[index % colors.length]
}

// 窗口大小变化时重新渲染图表
function handleResize() {
  bucketChart?.resize()
  typeChart?.resize()
  leafletMap?.invalidateSize()
}

// 检查 GeoStats 是否启用
async function checkGeoStatsEnabled() {
  try {
    const config = await getGeoStatsConfig()
    geoStatsEnabled.value = config.enabled && config.geoip_enabled
    if (geoStatsEnabled.value) {
      await nextTick()
      initMap()
      loadGeoStatsData()
    }
  } catch {
    geoStatsEnabled.value = false
  }
}

// 初始化 Leaflet 地图
function initMap() {
  if (!mapContainerRef.value || leafletMap) return

  leafletMap = L.map(mapContainerRef.value, {
    center: [30, 0],
    zoom: 2,
    minZoom: 1,
    maxZoom: 10,
    attributionControl: false,
    zoomControl: true
  })

  // 使用 OpenStreetMap 瓦片
  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    maxZoom: 19
  }).addTo(leafletMap)

  // 创建标记图层
  markersLayer = L.layerGroup().addTo(leafletMap)
}

// 加载 GeoStats 数据
async function loadGeoStatsData() {
  if (!geoStatsEnabled.value) return

  mapLoading.value = true
  try {
    const days = parseInt(mapDateRange.value)
    const endDate = new Date()
    const startDate = new Date()
    startDate.setDate(startDate.getDate() - days)

    const params = {
      start_date: startDate.toISOString().split('T')[0],
      end_date: endDate.toISOString().split('T')[0],
      group_by: 'country',
      limit: 50
    }

    const [dataResp, summaryResp] = await Promise.all([
      getGeoStatsData(params),
      getGeoStatsSummary({ start_date: params.start_date, end_date: params.end_date })
    ])

    geoStatsData.value = dataResp.data || []
    geoStatsSummary.value = summaryResp

    // 更新地图标记
    updateMapMarkers()
  } catch (e: any) {
    console.error('加载地理统计失败:', e)
  } finally {
    mapLoading.value = false
  }
}

// 更新地图标记
function updateMapMarkers() {
  if (!markersLayer || !leafletMap) return

  markersLayer.clearLayers()

  if (geoStatsData.value.length === 0) return

  // 找出最大值用于计算比例
  const maxCount = Math.max(...geoStatsData.value.map(d => d.total))

  geoStatsData.value.forEach(item => {
    const coords = countryCoordinates[item.country_code]
    if (!coords) return

    // 根据请求数计算圆圈大小
    const ratio = item.total / maxCount
    const radius = Math.max(8, Math.min(40, 8 + ratio * 32))

    // 创建圆圈标记
    const circle = L.circleMarker(coords, {
      radius: radius,
      fillColor: '#e67e22',
      color: '#d35400',
      weight: 2,
      opacity: 0.9,
      fillOpacity: 0.6
    })

    // 添加弹窗
    const popupContent = `
      <div style="text-align: center; min-width: 120px;">
        <div style="font-weight: 600; font-size: 14px; margin-bottom: 4px;">
          ${item.country || item.country_code}
        </div>
        <div style="color: #e67e22; font-size: 18px; font-weight: 700;">
          ${item.total.toLocaleString()}
        </div>
        <div style="color: #888; font-size: 12px;">${t('dashboard.requests')}</div>
      </div>
    `
    circle.bindPopup(popupContent)

    // 添加到图层
    markersLayer!.addLayer(circle)
  })
}

// 销毁地图
function destroyMap() {
  if (leafletMap) {
    leafletMap.remove()
    leafletMap = null
    markersLayer = null
  }
}

onMounted(() => {
  loadStats()
  checkGeoStatsEnabled()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  bucketChart?.dispose()
  typeChart?.dispose()
  destroyMap()
})

watch(() => stats.value, () => {
  renderCharts()
})
</script>

<style scoped>
.page-container {
  max-width: 1400px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  flex-wrap: wrap;
  gap: 12px;
}

.page-title h1 {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
  color: #333;
}

.page-subtitle {
  margin: 4px 0 0;
  font-size: 13px;
  color: #888;
}

.refresh-btn {
  background: #e67e22;
  border-color: #e67e22;
  color: #fff;
}

.refresh-btn:hover {
  background: #d35400;
  border-color: #d35400;
}

.stats-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}

.stat-card {
  background: #fff;
  border-radius: 10px;
  padding: 20px;
  display: flex;
  align-items: center;
  gap: 14px;
  border: 1px solid #eee;
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  flex-shrink: 0;
}

.stat-icon.buckets { background: linear-gradient(135deg, #e67e22, #d35400); }
.stat-icon.objects { background: linear-gradient(135deg, #27ae60, #1e8449); }
.stat-icon.size { background: linear-gradient(135deg, #3498db, #2980b9); }
.stat-icon.disk { background: linear-gradient(135deg, #9b59b6, #8e44ad); }

.stat-content {
  flex: 1;
  min-width: 0;
}

.stat-value {
  font-size: 24px;
  font-weight: 700;
  color: #333;
  line-height: 1.2;
}

.stat-label {
  font-size: 13px;
  color: #888;
  margin-top: 2px;
}

.charts-row {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
  margin-bottom: 20px;
}

.chart-card {
  min-height: 320px;
}

.content-card {
  background: #fff;
  border-radius: 10px;
  border: 1px solid #eee;
  margin-bottom: 16px;
  overflow: hidden;
}

.table-card {
  margin-bottom: 16px;
}

.card-header {
  padding: 14px 16px;
  border-bottom: 1px solid #f0f0f0;
}

.card-header h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #333;
}

.chart-container {
  height: 260px;
  padding: 12px;
}

.table-wrapper {
  overflow-x: auto;
}

.data-table {
  width: 100%;
}

.bucket-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #e67e22;
  text-decoration: none;
  font-weight: 500;
}

.bucket-link:hover {
  color: #d35400;
}

.object-path {
  font-family: ui-monospace, monospace;
  font-size: 13px;
  color: #555;
  word-break: break-all;
}

/* 大屏幕响应式 */
@media (max-width: 1200px) {
  .stats-cards {
    grid-template-columns: repeat(2, 1fr);
  }
}

/* 平板响应式 */
@media (max-width: 900px) {
  .charts-row {
    grid-template-columns: 1fr;
  }
}

/* 移动端响应式 */
@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .page-title h1 {
    font-size: 20px;
  }

  .stats-cards {
    grid-template-columns: repeat(2, 1fr);
    gap: 12px;
  }

  .stat-card {
    padding: 14px;
    gap: 10px;
  }

  .stat-icon {
    width: 40px;
    height: 40px;
  }

  .stat-value {
    font-size: 20px;
  }

  .stat-label {
    font-size: 12px;
  }

  .chart-card {
    min-height: 280px;
  }

  .chart-container {
    height: 220px;
    padding: 8px;
  }

  .btn-text {
    display: none;
  }

  :deep(.hide-on-mobile) {
    display: none !important;
  }
}

@media (max-width: 480px) {
  .stats-cards {
    grid-template-columns: 1fr;
  }
}

/* 地图卡片样式 */
.map-card {
  margin-bottom: 16px;
}

.map-card .card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.map-card .header-actions {
  display: flex;
  gap: 8px;
}

.map-stats-row {
  display: flex;
  justify-content: center;
  gap: 40px;
  padding: 16px;
  background: #fafafa;
  border-bottom: 1px solid #f0f0f0;
}

.map-stat {
  text-align: center;
}

.map-stat-value {
  display: block;
  font-size: 24px;
  font-weight: 700;
  color: #e67e22;
}

.map-stat-label {
  display: block;
  font-size: 12px;
  color: #888;
  margin-top: 2px;
}

.map-container {
  height: 400px;
  background: #f5f5f5;
}

.geo-ranking {
  padding: 16px;
  border-top: 1px solid #f0f0f0;
}

.ranking-header {
  font-size: 14px;
  font-weight: 600;
  color: #333;
  margin-bottom: 12px;
}

.ranking-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 8px;
}

.ranking-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  background: #fafafa;
  border-radius: 6px;
}

.ranking-index {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  font-size: 12px;
  font-weight: 600;
  background: #e67e22;
  color: #fff;
}

.ranking-item:nth-child(1) .ranking-index { background: #f39c12; }
.ranking-item:nth-child(2) .ranking-index { background: #95a5a6; }
.ranking-item:nth-child(3) .ranking-index { background: #cd7f32; }
.ranking-item:nth-child(n+4) .ranking-index { background: #bdc3c7; color: #555; }

.ranking-country {
  flex: 1;
  font-size: 13px;
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ranking-count {
  font-size: 13px;
  font-weight: 600;
  color: #e67e22;
}

/* 地图响应式 */
@media (max-width: 768px) {
  .map-stats-row {
    gap: 20px;
    flex-wrap: wrap;
  }

  .map-stat-value {
    font-size: 20px;
  }

  .map-container {
    height: 300px;
  }

  .ranking-list {
    grid-template-columns: 1fr;
  }
}
</style>
