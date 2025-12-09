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
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Refresh, Folder, Document, Coin, Box } from '@element-plus/icons-vue'
import { getStorageStats, getRecentObjects, type StatsResponse, type RecentObject } from '../api/admin'
import * as echarts from 'echarts'

const { t } = useI18n()

const loading = ref(false)
const stats = ref<StatsResponse | null>(null)
const recentObjects = ref<RecentObject[]>([])

const bucketChartRef = ref<HTMLElement>()
const typeChartRef = ref<HTMLElement>()
let bucketChart: echarts.ECharts | null = null
let typeChart: echarts.ECharts | null = null

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
}

onMounted(() => {
  loadStats()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  bucketChart?.dispose()
  typeChart?.dispose()
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
</style>
