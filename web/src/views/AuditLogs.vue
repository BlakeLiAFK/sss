<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title">
        <h1>{{ t('auditLogs.title') }}</h1>
        <p class="page-subtitle">{{ t('auditLogs.subtitle') }}</p>
      </div>
      <el-button @click="loadLogs" :loading="loading" class="refresh-btn">
        <el-icon><Refresh /></el-icon>
        <span class="btn-text">{{ t('common.refresh') }}</span>
      </el-button>
    </div>

    <!-- 统计卡片 -->
    <div class="stats-cards">
      <div class="stat-card">
        <div class="stat-value">{{ stats.total }}</div>
        <div class="stat-label">{{ t('auditLogs.totalRecords') }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ stats.today }}</div>
        <div class="stat-label">{{ t('auditLogs.todayOperations') }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-value" :class="{ 'text-danger': stats.failed > 0 }">{{ stats.failed }}</div>
        <div class="stat-label">{{ t('auditLogs.failedOperations') }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ pagination.page }}/{{ Math.ceil(pagination.total / pagination.limit) || 1 }}</div>
        <div class="stat-label">{{ t('auditLogs.currentPage') }}</div>
      </div>
    </div>

    <!-- 筛选器 -->
    <div class="filter-card">
      <div class="filter-row">
        <el-select v-model="filters.action" clearable :placeholder="t('auditLogs.operationType')" class="filter-item">
          <el-option-group :label="t('auditLogs.authRelated')">
            <el-option :label="t('auditLogs.actions.login')" value="login" />
            <el-option :label="t('auditLogs.actions.loginFailed')" value="login_failed" />
            <el-option :label="t('auditLogs.actions.logout')" value="logout" />
            <el-option :label="t('auditLogs.actions.passwordReset')" value="password_reset" />
          </el-option-group>
          <el-option-group :label="t('auditLogs.systemRelated')">
            <el-option :label="t('auditLogs.actions.systemInstall')" value="system_install" />
          </el-option-group>
          <el-option-group :label="t('auditLogs.bucketOps')">
            <el-option :label="t('auditLogs.actions.bucketCreate')" value="bucket_create" />
            <el-option :label="t('auditLogs.actions.bucketDelete')" value="bucket_delete" />
            <el-option :label="t('auditLogs.actions.bucketSetPublic')" value="bucket_set_public" />
            <el-option :label="t('auditLogs.actions.bucketSetPrivate')" value="bucket_set_private" />
          </el-option-group>
          <el-option-group :label="t('auditLogs.apikeyOps')">
            <el-option :label="t('auditLogs.actions.apikeyCreate')" value="apikey_create" />
            <el-option :label="t('auditLogs.actions.apikeyDelete')" value="apikey_delete" />
            <el-option :label="t('auditLogs.actions.apikeyUpdate')" value="apikey_update" />
            <el-option :label="t('auditLogs.actions.apikeyResetSecret')" value="apikey_reset_secret" />
            <el-option :label="t('auditLogs.actions.apikeySetPerm')" value="apikey_set_perm" />
            <el-option :label="t('auditLogs.actions.apikeyDelPerm')" value="apikey_del_perm" />
          </el-option-group>
        </el-select>
        <el-input v-model="filters.actor" clearable :placeholder="t('auditLogs.operator')" class="filter-item" />
        <el-input v-model="filters.ip" clearable :placeholder="t('auditLogs.ipAddress')" class="filter-item" />
        <el-select v-model="filters.success" clearable :placeholder="t('auditLogs.result')" class="filter-item filter-item-sm">
          <el-option :label="t('auditLogs.success')" value="true" />
          <el-option :label="t('auditLogs.failed')" value="false" />
        </el-select>
        <el-button type="primary" @click="handleSearch" class="primary-btn">{{ t('auditLogs.search') }}</el-button>
        <el-button @click="resetFilters">{{ t('auditLogs.reset') }}</el-button>
      </div>
    </div>

    <!-- 移动端日志卡片 -->
    <div class="mobile-logs">
      <div v-for="log in logs" :key="log.id" class="log-card">
        <div class="log-card-header">
          <el-tag :type="getActionType(log.action)" size="small">{{ getActionLabel(log.action) }}</el-tag>
          <el-icon v-if="log.success" class="success-icon"><CircleCheck /></el-icon>
          <el-icon v-else class="fail-icon"><CircleClose /></el-icon>
        </div>
        <div class="log-card-body">
          <div class="log-info-row">
            <span class="log-label">{{ t('auditLogs.operator') }}:</span>
            <span>{{ log.actor }}</span>
          </div>
          <div class="log-info-row">
            <span class="log-label">IP:</span>
            <span>{{ log.forwarded_ip || log.ip || '-' }}</span>
          </div>
          <div class="log-info-row" v-if="log.location">
            <span class="log-label">{{ t('auditLogs.location') }}:</span>
            <span>{{ log.location }}</span>
          </div>
          <div class="log-info-row" v-if="log.resource">
            <span class="log-label">{{ t('auditLogs.resource') }}:</span>
            <span>{{ log.resource }}</span>
          </div>
          <div class="log-info-row">
            <span class="log-label">{{ t('auditLogs.time') }}:</span>
            <span>{{ formatTime(log.timestamp) }}</span>
          </div>
        </div>
      </div>
      <el-empty v-if="!loading && logs.length === 0" :description="t('auditLogs.noLogs')" />
    </div>

    <!-- 桌面端日志表格 -->
    <div class="content-card desktop-table">
      <div class="table-wrapper">
        <el-table :data="logs" v-loading="loading" class="data-table">
          <el-table-column :label="t('auditLogs.time')" width="160">
            <template #default="{ row }">
              <span class="time-cell">{{ formatTime(row.timestamp) }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('auditLogs.operation')" width="110">
            <template #default="{ row }">
              <el-tag :type="getActionType(row.action)" size="small">
                {{ getActionLabel(row.action) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="actor" :label="t('auditLogs.operator')" width="90" />
          <el-table-column :label="t('auditLogs.ipAddress')" width="130">
            <template #default="{ row }">
              <el-tooltip
                v-if="row.forwarded_ip && row.forwarded_ip !== row.ip"
                :content="`${t('auditLogs.directIp')}: ${row.ip}`"
                placement="top"
              >
                <span class="ip-cell">{{ row.forwarded_ip }}</span>
              </el-tooltip>
              <span v-else class="ip-cell">{{ row.ip || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('auditLogs.location')" width="120" class-name="hide-on-tablet">
            <template #default="{ row }">
              <span v-if="row.location" class="location-cell">{{ row.location }}</span>
              <span v-else class="no-detail">-</span>
            </template>
          </el-table-column>
          <el-table-column prop="resource" :label="t('auditLogs.resource')" min-width="100" show-overflow-tooltip />
          <el-table-column :label="t('auditLogs.details')" min-width="150" class-name="hide-on-tablet">
            <template #default="{ row }">
              <span v-if="row.detail" class="detail-cell">{{ formatDetail(row.detail) }}</span>
              <span v-else class="no-detail">-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('auditLogs.result')" width="70" align="center">
            <template #default="{ row }">
              <el-icon v-if="row.success" class="success-icon"><CircleCheck /></el-icon>
              <el-icon v-else class="fail-icon"><CircleClose /></el-icon>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <!-- 分页 -->
    <div class="pagination-wrapper">
      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.limit"
        :page-sizes="[20, 50, 100]"
        :total="pagination.total"
        :layout="paginationLayout"
        :small="isMobile"
        @size-change="handleSizeChange"
        @current-change="handlePageChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Refresh, CircleCheck, CircleClose } from '@element-plus/icons-vue'
import { useAuthStore } from '../stores/auth'
import axios from 'axios'

const { t } = useI18n()

// 响应式状态
const windowWidth = ref(window.innerWidth)
const isMobile = computed(() => windowWidth.value < 768)
const paginationLayout = computed(() =>
  isMobile.value ? 'prev, pager, next' : 'total, sizes, prev, pager, next, jumper'
)

function handleResize() {
  windowWidth.value = window.innerWidth
}

onMounted(() => window.addEventListener('resize', handleResize))
onUnmounted(() => window.removeEventListener('resize', handleResize))

// 审计日志类型
interface AuditLog {
  id: number
  timestamp: string
  action: string
  actor: string
  ip: string
  forwarded_ip: string
  location: string
  resource: string
  detail: string
  success: boolean
  user_agent: string
}

const auth = useAuthStore()
const loading = ref(false)
const logs = ref<AuditLog[]>([])

// 统计数据
const stats = reactive({
  total: 0,
  today: 0,
  failed: 0
})

// 分页
const pagination = reactive({
  page: 1,
  limit: 50,
  total: 0
})

// 筛选条件
const filters = reactive({
  action: '',
  actor: '',
  ip: '',
  resource: '',
  success: ''
})

// 操作类型标签映射
const actionLabels: Record<string, string> = {
  login: 'auditLogs.actions.login',
  login_failed: 'auditLogs.actions.loginFailed',
  logout: 'auditLogs.actions.logout',
  password_reset: 'auditLogs.actions.passwordReset',
  system_install: 'auditLogs.actions.systemInstall',
  bucket_create: 'auditLogs.actions.bucketCreate',
  bucket_delete: 'auditLogs.actions.bucketDelete',
  bucket_set_public: 'auditLogs.actions.bucketSetPublic',
  bucket_set_private: 'auditLogs.actions.bucketSetPrivate',
  apikey_create: 'auditLogs.actions.apikeyCreate',
  apikey_delete: 'auditLogs.actions.apikeyDelete',
  apikey_update: 'auditLogs.actions.apikeyUpdate',
  apikey_reset_secret: 'auditLogs.actions.apikeyResetSecret',
  apikey_set_perm: 'auditLogs.actions.apikeySetPerm',
  apikey_del_perm: 'auditLogs.actions.apikeyDelPerm',
  object_upload: 'auditLogs.actions.objectUpload',
  object_delete: 'auditLogs.actions.objectDelete',
  batch_delete: 'auditLogs.actions.batchDelete'
}

// 操作类型颜色映射
const actionTypes: Record<string, string> = {
  login: 'success',
  login_failed: 'danger',
  logout: 'info',
  password_reset: 'warning',
  system_install: 'primary',
  bucket_create: 'success',
  bucket_delete: 'danger',
  bucket_set_public: 'warning',
  bucket_set_private: 'info',
  apikey_create: 'success',
  apikey_delete: 'danger',
  apikey_update: 'warning',
  apikey_reset_secret: 'warning',
  apikey_set_perm: 'info',
  apikey_del_perm: 'info'
}

function getActionLabel(action: string): string {
  const key = actionLabels[action]
  return key ? t(key) : action
}

function getActionType(action: string): string {
  return actionTypes[action] || 'info'
}

function formatTime(timestamp: string): string {
  const date = new Date(timestamp)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

function formatDetail(detail: string): string {
  try {
    const obj = JSON.parse(detail)
    return Object.entries(obj)
      .map(([k, v]) => `${k}: ${v}`)
      .join(', ')
  } catch {
    return detail
  }
}

function getHeaders() {
  return {
    'X-Admin-Token': auth.adminToken,
    'Content-Type': 'application/json'
  }
}

async function loadLogs() {
  loading.value = true
  try {
    const params = new URLSearchParams()
    params.append('page', String(pagination.page))
    params.append('limit', String(pagination.limit))

    if (filters.action) params.append('action', filters.action)
    if (filters.actor) params.append('actor', filters.actor)
    if (filters.ip) params.append('ip', filters.ip)
    if (filters.resource) params.append('resource', filters.resource)
    if (filters.success) params.append('success', filters.success)

    const response = await axios.get(`${auth.endpoint}/api/admin/audit?${params}`, {
      headers: getHeaders()
    })

    logs.value = response.data.logs || []
    pagination.total = response.data.total || 0
  } catch (error: any) {
    ElMessage.error(t('auditLogs.loadFailed') + ': ' + (error.response?.data?.message || error.message))
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  try {
    const response = await axios.get(`${auth.endpoint}/api/admin/audit/stats`, {
      headers: getHeaders()
    })
    stats.total = response.data.total || 0
    stats.today = response.data.today || 0
    stats.failed = response.data.failed || 0
  } catch (error) {
    console.error('Failed to load stats:', error)
  }
}

function handleSearch() {
  pagination.page = 1
  loadLogs()
}

function resetFilters() {
  filters.action = ''
  filters.actor = ''
  filters.ip = ''
  filters.resource = ''
  filters.success = ''
  pagination.page = 1
  loadLogs()
}

function handlePageChange(page: number) {
  pagination.page = page
  loadLogs()
}

function handleSizeChange(size: number) {
  pagination.limit = size
  pagination.page = 1
  loadLogs()
}

onMounted(() => {
  loadLogs()
  loadStats()
})
</script>

<style scoped>
.page-container {
  max-width: 1200px;
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

.primary-btn {
  background: #e67e22;
  border-color: #e67e22;
}

.primary-btn:hover {
  background: #d35400;
  border-color: #d35400;
}

/* 统计卡片 */
.stats-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}

.stat-card {
  background: #fff;
  border: 1px solid #eee;
  border-radius: 8px;
  padding: 14px;
  text-align: center;
}

.stat-value {
  font-size: 22px;
  font-weight: 700;
  color: #333;
}

.stat-value.text-danger {
  color: #f56c6c;
}

.stat-label {
  font-size: 12px;
  color: #888;
  margin-top: 4px;
}

/* 筛选器 */
.filter-card {
  background: #fff;
  border: 1px solid #eee;
  border-radius: 8px;
  padding: 14px;
  margin-bottom: 16px;
}

.filter-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
}

.filter-item {
  width: 140px;
}

.filter-item-sm {
  width: 100px;
}

/* 移动端日志卡片 */
.mobile-logs {
  display: none;
}

.log-card {
  background: #fff;
  border: 1px solid #eee;
  border-radius: 8px;
  padding: 12px;
  margin-bottom: 10px;
}

.log-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
  padding-bottom: 10px;
  border-bottom: 1px solid #f0f0f0;
}

.log-card-body {
  font-size: 13px;
}

.log-info-row {
  display: flex;
  margin-bottom: 6px;
}

.log-label {
  color: #888;
  width: 60px;
  flex-shrink: 0;
}

/* 桌面端表格 */
.content-card {
  background: #fff;
  border: 1px solid #eee;
  border-radius: 8px;
  overflow: hidden;
}

.table-wrapper {
  overflow-x: auto;
}

.data-table {
  width: 100%;
}

.time-cell {
  font-family: ui-monospace, monospace;
  font-size: 12px;
  color: #666;
}

.ip-cell {
  font-family: ui-monospace, monospace;
  font-size: 12px;
}

.detail-cell {
  font-size: 12px;
  color: #888;
}

.no-detail {
  color: #ccc;
}

.location-cell {
  font-size: 12px;
  color: #666;
}

.success-icon {
  color: #67c23a;
  font-size: 16px;
}

.fail-icon {
  color: #f56c6c;
  font-size: 16px;
}

.pagination-wrapper {
  margin-top: 16px;
  display: flex;
  justify-content: center;
}

/* 平板响应式 */
@media (max-width: 1024px) {
  :deep(.hide-on-tablet) {
    display: none !important;
  }
}

/* 移动端响应式 */
@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .btn-text {
    display: none;
  }

  .stats-cards {
    grid-template-columns: repeat(2, 1fr);
  }

  .stat-card {
    padding: 10px;
  }

  .stat-value {
    font-size: 18px;
  }

  .filter-row {
    flex-direction: column;
    align-items: stretch;
  }

  .filter-item,
  .filter-item-sm {
    width: 100%;
  }

  .mobile-logs {
    display: block;
  }

  .desktop-table {
    display: none;
  }
}
</style>
