<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title">
        <h1>{{ t('settings.title') }}</h1>
        <p class="page-subtitle">{{ t('settings.subtitle') }}</p>
      </div>
    </div>

    <div class="settings-grid">
      <!-- 运行时参数（只读） -->
      <div class="settings-card">
        <div class="card-header">
          <el-icon class="card-icon"><Monitor /></el-icon>
          <h3>{{ t('settings.runtimeParams') }}</h3>
          <el-tag size="small" type="info">{{ t('settings.readonly') }}</el-tag>
        </div>
        <div class="card-body">
          <div class="info-item">
            <span class="info-label">{{ t('settings.listenAddress') }}</span>
            <span class="info-value mono">{{ settings.runtime.host }}:{{ settings.runtime.port }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">{{ t('settings.dataDir') }}</span>
            <span class="info-value mono">{{ settings.runtime.data_path }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">{{ t('settings.database') }}</span>
            <span class="info-value mono">{{ settings.runtime.db_path }}</span>
          </div>
          <p class="setting-hint">{{ t('settings.runtimeHint') }}</p>
        </div>
      </div>

      <!-- 存储配置（可修改） -->
      <div class="settings-card">
        <div class="card-header">
          <el-icon class="card-icon"><FolderOpened /></el-icon>
          <h3>{{ t('settings.storageConfig') }}</h3>
        </div>
        <div class="card-body">
          <div class="setting-item">
            <label>{{ t('settings.s3Region') }}</label>
            <el-input v-model="settings.storage.region" placeholder="us-east-1" :disabled="!editing" />
          </div>
          <div class="setting-item">
            <label>{{ t('settings.maxObjectSize') }}</label>
            <el-select v-model="maxObjectSizeOption" :disabled="!editing" style="width: 100%">
              <el-option label="1 GB" :value="1073741824" />
              <el-option label="2 GB" :value="2147483648" />
              <el-option label="5 GB" :value="5368709120" />
              <el-option label="10 GB" :value="10737418240" />
            </el-select>
            <span class="setting-hint">{{ t('settings.maxObjectSizeHint') }}</span>
          </div>
          <div class="setting-item">
            <label>{{ t('settings.presignUploadLimit') }}</label>
            <el-select v-model="maxUploadSizeOption" :disabled="!editing" style="width: 100%">
              <el-option label="100 MB" :value="104857600" />
              <el-option label="500 MB" :value="524288000" />
              <el-option label="1 GB" :value="1073741824" />
              <el-option label="2 GB" :value="2147483648" />
            </el-select>
            <span class="setting-hint">{{ t('settings.presignUploadLimitHint') }}</span>
          </div>
        </div>
      </div>

      <!-- 系统信息 -->
      <div class="settings-card">
        <div class="card-header">
          <el-icon class="card-icon"><InfoFilled /></el-icon>
          <h3>{{ t('settings.systemInfo') }}</h3>
        </div>
        <div class="card-body">
          <div class="info-item">
            <span class="info-label">{{ t('settings.version') }}</span>
            <div class="version-info">
              <span class="info-value">{{ settings.system.version || '-' }}</span>
              <el-button
                link
                type="primary"
                @click="checkForUpdate"
                :loading="checkingUpdate"
                class="check-update-btn"
              >
                {{ checkingUpdate ? t('settings.checking') : t('settings.checkUpdate') }}
              </el-button>
            </div>
          </div>
          <!-- 版本更新提示 -->
          <div v-if="updateInfo.hasUpdate" class="update-alert">
            <el-alert
              :title="t('settings.newVersionAvailable', { version: updateInfo.latestVersion })"
              type="warning"
              :closable="false"
              show-icon
            >
              <template #default>
                <div class="update-details">
                  <p v-if="updateInfo.publishedAt">{{ t('settings.publishedAt') }}: {{ formatDate(updateInfo.publishedAt) }}</p>
                  <a
                    v-if="updateInfo.releaseUrl"
                    :href="updateInfo.releaseUrl"
                    target="_blank"
                    class="release-link"
                  >
                    {{ t('settings.viewRelease') }}
                    <el-icon><Link /></el-icon>
                  </a>
                </div>
              </template>
            </el-alert>
          </div>
          <div v-else-if="updateInfo.checked && !updateInfo.hasUpdate" class="update-status">
            <el-tag type="success" size="small">
              <el-icon><CircleCheck /></el-icon>
              {{ t('settings.alreadyLatest') }}
            </el-tag>
          </div>
          <div class="info-item">
            <span class="info-label">{{ t('settings.installedAt') }}</span>
            <span class="info-value">{{ formatDate(settings.system.installed_at) }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">{{ t('settings.status') }}</span>
            <el-tag type="success" size="small">{{ t('settings.running') }}</el-tag>
          </div>
        </div>
      </div>

      <!-- GeoIP 配置 -->
      <div class="settings-card">
        <div class="card-header">
          <el-icon class="card-icon"><Location /></el-icon>
          <h3>{{ t('settings.geoipConfig') }}</h3>
          <el-button
            link
            type="primary"
            @click="showGeoipInfo = true"
            class="info-btn"
          >
            <el-icon><QuestionFilled /></el-icon>
          </el-button>
        </div>
        <div class="card-body">
          <div class="info-item">
            <span class="info-label">{{ t('common.status') }}</span>
            <el-tag :type="geoipStatus.enabled ? 'success' : 'info'" size="small">
              {{ geoipStatus.enabled ? t('settings.geoipEnabled') : t('settings.geoipDisabled') }}
            </el-tag>
          </div>
          <div class="info-item" v-if="geoipStatus.enabled">
            <span class="info-label">{{ t('settings.geoipPath') }}</span>
            <span class="info-value mono" style="font-size: 11px;">{{ geoipStatus.path }}</span>
          </div>
          <p class="setting-hint" style="margin-top: 12px;">{{ t('settings.geoipHint') }}</p>

          <div class="geoip-actions">
            <input
              ref="fileInputRef"
              type="file"
              accept=".mmdb"
              style="display: none;"
              @change="handleFileChange"
            />
            <el-button
              v-if="!geoipStatus.enabled"
              type="primary"
              @click="triggerFileUpload"
              :loading="geoipUploading"
              class="primary-btn"
            >
              <el-icon v-if="!geoipUploading"><Upload /></el-icon>
              {{ geoipUploading ? t('settings.geoipUploading') : t('settings.geoipUpload') }}
            </el-button>
            <template v-else>
              <el-button
                type="primary"
                @click="triggerFileUpload"
                :loading="geoipUploading"
                class="primary-btn"
              >
                <el-icon v-if="!geoipUploading"><Upload /></el-icon>
                {{ geoipUploading ? t('settings.geoipUploading') : t('settings.geoipUpload') }}
              </el-button>
              <el-popconfirm
                :title="t('settings.geoipDeleteConfirm')"
                :confirm-button-text="t('common.confirm')"
                :cancel-button-text="t('common.cancel')"
                @confirm="deleteGeoip"
              >
                <template #reference>
                  <el-button type="danger" plain>
                    <el-icon><Delete /></el-icon>
                    {{ t('settings.geoipDelete') }}
                  </el-button>
                </template>
              </el-popconfirm>
            </template>
            <a
              href="https://www.maxmind.com/en/geolite2/signup"
              target="_blank"
              class="download-link"
            >
              {{ t('settings.geoipDownloadHint') }}
              <el-icon><el-icon-link /></el-icon>
            </a>
          </div>
        </div>
      </div>

      <!-- GeoStats 配置 -->
      <div class="settings-card">
        <div class="card-header">
          <el-icon class="card-icon"><TrendCharts /></el-icon>
          <h3>{{ t('settings.geostatsConfig') }}</h3>
        </div>
        <div class="card-body">
          <!-- 依赖 GeoIP 提示 -->
          <el-alert
            v-if="!geoipStatus.enabled"
            type="warning"
            :closable="false"
            show-icon
            class="dependency-alert"
          >
            {{ t('settings.geostatsRequiresGeoip') }}
          </el-alert>

          <template v-else>
            <!-- 启用开关 -->
            <div class="setting-item">
              <div class="switch-row">
                <label>{{ t('settings.geostatsEnabled') }}</label>
                <el-switch
                  v-model="geoStatsConfig.enabled"
                  @change="handleGeoStatsToggle"
                  :loading="geoStatsUpdating"
                />
              </div>
              <span class="setting-hint">{{ t('settings.geostatsEnabledHint') }}</span>
            </div>

            <!-- 详细配置 -->
            <template v-if="geoStatsConfig.enabled">
              <div class="setting-item">
                <label>{{ t('settings.geostatsMode') }}</label>
                <el-select v-model="geoStatsConfig.mode" style="width: 100%" @change="updateGeoStatsConfig">
                  <el-option :label="t('settings.geostatsModeRealtime')" value="realtime" />
                  <el-option :label="t('settings.geostatsModeBatch')" value="batch" />
                </el-select>
                <span class="setting-hint">{{ t('settings.geostatsModeHint') }}</span>
              </div>

              <!-- 批量模式配置 -->
              <template v-if="geoStatsConfig.mode === 'batch'">
                <div class="setting-item">
                  <label>{{ t('settings.geostatsBatchSize') }}</label>
                  <el-input-number
                    v-model="geoStatsConfig.batch_size"
                    :min="10"
                    :max="1000"
                    :step="10"
                    style="width: 100%"
                    @change="updateGeoStatsConfig"
                  />
                  <span class="setting-hint">{{ t('settings.geostatsBatchSizeHint') }}</span>
                </div>

                <div class="setting-item">
                  <label>{{ t('settings.geostatsFlushInterval') }}</label>
                  <el-input-number
                    v-model="geoStatsConfig.flush_interval"
                    :min="10"
                    :max="600"
                    :step="10"
                    style="width: 100%"
                    @change="updateGeoStatsConfig"
                  />
                  <span class="setting-hint">{{ t('settings.geostatsFlushIntervalHint') }}</span>
                </div>
              </template>

              <div class="setting-item">
                <label>{{ t('settings.geostatsRetentionDays') }}</label>
                <el-input-number
                  v-model="geoStatsConfig.retention_days"
                  :min="7"
                  :max="365"
                  :step="7"
                  style="width: 100%"
                  @change="updateGeoStatsConfig"
                />
                <span class="setting-hint">{{ t('settings.geostatsRetentionDaysHint') }}</span>
              </div>

              <!-- 数据管理 -->
              <div class="setting-item" style="margin-top: 16px;">
                <el-popconfirm
                  :title="t('settings.geostatsClearConfirm')"
                  :confirm-button-text="t('common.confirm')"
                  :cancel-button-text="t('common.cancel')"
                  @confirm="clearGeoStatsData"
                >
                  <template #reference>
                    <el-button type="danger" plain :loading="geoStatsClearing">
                      <el-icon><Delete /></el-icon>
                      {{ t('settings.geostatsClearData') }}
                    </el-button>
                  </template>
                </el-popconfirm>
              </div>
            </template>
          </template>
        </div>
      </div>

      <!-- 安全设置 -->
      <div class="settings-card">
        <div class="card-header">
          <el-icon class="card-icon"><Lock /></el-icon>
          <h3>{{ t('settings.securitySettings') }}</h3>
        </div>
        <div class="card-body">
          <div class="setting-item">
            <label>{{ t('settings.corsOrigin') }}</label>
            <el-input v-model="settings.security.cors_origin" placeholder="*" :disabled="!editing" />
            <span class="setting-hint">{{ t('settings.corsOriginHint') }}</span>
          </div>
          <div class="setting-item">
            <label>{{ t('settings.presignScheme') }}</label>
            <el-select v-model="settings.security.presign_scheme" :disabled="!editing" style="width: 100%">
              <el-option label="HTTP" value="http" />
              <el-option label="HTTPS" value="https" />
            </el-select>
            <span class="setting-hint">{{ t('settings.presignSchemeHint') }}</span>
          </div>
          <div class="setting-item">
            <label>{{ t('settings.trustedProxies') }}</label>
            <div class="trusted-proxies-input">
              <el-input
                v-model="settings.security.trusted_proxies"
                type="textarea"
                :rows="3"
                :placeholder="t('settings.trustedProxiesPlaceholder')"
                :disabled="!editing"
              />
              <el-button
                v-if="editing"
                size="small"
                type="primary"
                plain
                @click="fillCloudflareIPs"
                class="preset-btn"
              >
                {{ t('settings.cloudflarePreset') }}
              </el-button>
            </div>
            <span class="setting-hint">{{ t('settings.trustedProxiesHint') }}</span>
          </div>
          <div class="setting-item" style="margin-top: 16px;">
            <el-button type="primary" @click="showPasswordDialog = true" class="full-width-btn">
              <el-icon><Key /></el-icon>
              {{ t('settings.changePassword') }}
            </el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- 操作按钮 -->
    <div class="action-bar">
      <template v-if="!editing">
        <el-button type="primary" @click="editing = true" class="primary-btn">
          <el-icon><Edit /></el-icon>
          {{ t('settings.editSettings') }}
        </el-button>
      </template>
      <template v-else>
        <el-button @click="cancelEdit">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveSettings" :loading="saving" class="primary-btn">
          <el-icon><Check /></el-icon>
          {{ t('settings.saveSettings') }}
        </el-button>
      </template>
    </div>

    <!-- GeoIP 信息对话框 -->
    <el-dialog v-model="showGeoipInfo" :title="t('settings.geoipInfoTitle')" width="500px">
      <p class="geoip-info-content">{{ t('settings.geoipInfoContent') }}</p>
      <div class="geoip-info-links">
        <a href="https://www.maxmind.com/en/geolite2/signup" target="_blank" class="info-link">
          MaxMind GeoLite2 (Free)
        </a>
        <a href="https://dev.maxmind.com/geoip/geolite2-free-geolocation-data" target="_blank" class="info-link">
          Documentation
        </a>
      </div>
      <template #footer>
        <el-button type="primary" @click="showGeoipInfo = false" class="primary-btn">
          {{ t('common.close') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 修改密码对话框 -->
    <el-dialog v-model="showPasswordDialog" :title="t('settings.changePassword')" width="400px" :close-on-click-modal="false">
      <el-form :model="passwordForm" label-position="top">
        <el-form-item :label="t('settings.currentPassword')" required>
          <el-input v-model="passwordForm.oldPassword" type="password" show-password :placeholder="t('settings.enterCurrentPassword')" />
        </el-form-item>
        <el-form-item :label="t('settings.newPassword')" required>
          <el-input v-model="passwordForm.newPassword" type="password" show-password :placeholder="t('settings.atLeast6Chars')" />
        </el-form-item>
        <el-form-item :label="t('settings.confirmNewPassword')" required>
          <el-input v-model="passwordForm.confirmPassword" type="password" show-password :placeholder="t('settings.enterNewPasswordAgain')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showPasswordDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="changePassword" :loading="changingPassword" class="primary-btn">
          {{ t('settings.confirmChange') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Monitor, FolderOpened, InfoFilled, Lock, Key, Edit, Check, Location, Upload, Delete, QuestionFilled, Link, CircleCheck, TrendCharts } from '@element-plus/icons-vue'
import { useAuthStore } from '../stores/auth'
import { getGeoStatsConfig, updateGeoStatsConfig as apiUpdateGeoStatsConfig, clearGeoStatsData as apiClearGeoStatsData } from '../api/admin'
import axios from 'axios'

const { t } = useI18n()
const auth = useAuthStore()

// 状态
const loading = ref(false)
const editing = ref(false)
const saving = ref(false)
const showPasswordDialog = ref(false)
const changingPassword = ref(false)
const geoipUploading = ref(false)
const showGeoipInfo = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)
const checkingUpdate = ref(false)

// 版本更新信息
const updateInfo = reactive({
  checked: false,
  hasUpdate: false,
  latestVersion: '',
  releaseUrl: '',
  releaseNotes: '',
  publishedAt: ''
})

// 设置数据
const settings = reactive({
  runtime: {
    host: '',
    port: 8080,
    data_path: '',
    db_path: ''
  },
  storage: {
    region: '',
    max_object_size: 0,
    max_upload_size: 0
  },
  security: {
    cors_origin: '*',
    presign_scheme: 'http',
    trusted_proxies: ''
  },
  system: {
    installed: false,
    installed_at: '',
    version: ''
  }
})

// GeoIP 状态
const geoipStatus = reactive({
  enabled: false,
  path: ''
})

// GeoStats 配置
const geoStatsConfig = reactive({
  enabled: false,
  mode: 'realtime',
  batch_size: 100,
  flush_interval: 60,
  retention_days: 90,
  geoip_enabled: false
})
const geoStatsUpdating = ref(false)
const geoStatsClearing = ref(false)

// 原始设置备份
const originalSettings = ref<typeof settings | null>(null)

// 大小选项
const maxObjectSizeOption = computed({
  get: () => settings.storage.max_object_size,
  set: (val) => { settings.storage.max_object_size = val }
})

const maxUploadSizeOption = computed({
  get: () => settings.storage.max_upload_size,
  set: (val) => { settings.storage.max_upload_size = val }
})

// 密码表单
const passwordForm = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})

function getHeaders() {
  return {
    'X-Admin-Token': auth.adminToken,
    'Content-Type': 'application/json'
  }
}

async function loadSettings() {
  loading.value = true
  try {
    const response = await axios.get(`${auth.endpoint}/api/admin/settings`, {
      headers: getHeaders()
    })
    Object.assign(settings.runtime, response.data.runtime)
    Object.assign(settings.storage, response.data.storage)
    if (response.data.security) {
      Object.assign(settings.security, response.data.security)
    }
    Object.assign(settings.system, response.data.system)
    originalSettings.value = JSON.parse(JSON.stringify(settings))
  } catch (error: any) {
    ElMessage.error(t('settings.loadFailed') + ': ' + (error.response?.data?.message || error.message))
  } finally {
    loading.value = false
  }
}

function cancelEdit() {
  if (originalSettings.value) {
    Object.assign(settings.storage, originalSettings.value.storage)
    Object.assign(settings.security, originalSettings.value.security)
  }
  editing.value = false
}

async function saveSettings() {
  saving.value = true
  try {
    const payload: any = {}

    if (originalSettings.value) {
      if (settings.storage.region !== originalSettings.value.storage.region) {
        payload.region = settings.storage.region
      }
      if (settings.storage.max_object_size !== originalSettings.value.storage.max_object_size) {
        payload.max_object_size = settings.storage.max_object_size
      }
      if (settings.storage.max_upload_size !== originalSettings.value.storage.max_upload_size) {
        payload.max_upload_size = settings.storage.max_upload_size
      }
      if (settings.security.cors_origin !== originalSettings.value.security.cors_origin) {
        payload.cors_origin = settings.security.cors_origin
      }
      if (settings.security.presign_scheme !== originalSettings.value.security.presign_scheme) {
        payload.presign_scheme = settings.security.presign_scheme
      }
      if (settings.security.trusted_proxies !== originalSettings.value.security.trusted_proxies) {
        payload.trusted_proxies = settings.security.trusted_proxies
      }
    }

    await axios.put(`${auth.endpoint}/api/admin/settings`, payload, {
      headers: getHeaders()
    })

    originalSettings.value = JSON.parse(JSON.stringify(settings))
    editing.value = false
    ElMessage.success(t('settings.saveSuccess'))
  } catch (error: any) {
    ElMessage.error(t('settings.saveFailed') + ': ' + (error.response?.data?.message || error.message))
  } finally {
    saving.value = false
  }
}

async function changePassword() {
  if (!passwordForm.oldPassword || !passwordForm.newPassword) {
    ElMessage.warning(t('settings.fillAllFields'))
    return
  }
  if (passwordForm.newPassword !== passwordForm.confirmPassword) {
    ElMessage.warning(t('settings.passwordMismatch'))
    return
  }
  if (passwordForm.newPassword.length < 6) {
    ElMessage.warning(t('settings.passwordTooShort'))
    return
  }

  changingPassword.value = true
  try {
    await axios.post(`${auth.endpoint}/api/admin/settings/password`, {
      old_password: passwordForm.oldPassword,
      new_password: passwordForm.newPassword
    }, {
      headers: getHeaders()
    })

    ElMessage.success(t('settings.passwordChanged'))
    showPasswordDialog.value = false
    passwordForm.oldPassword = ''
    passwordForm.newPassword = ''
    passwordForm.confirmPassword = ''
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || t('settings.changeFailed'))
  } finally {
    changingPassword.value = false
  }
}

function formatDate(dateStr: string): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN')
}

// Cloudflare IP 范围预设
const cloudflareIPs = [
  // IPv4
  '173.245.48.0/20',
  '103.21.244.0/22',
  '103.22.200.0/22',
  '103.31.4.0/22',
  '141.101.64.0/18',
  '108.162.192.0/18',
  '190.93.240.0/20',
  '188.114.96.0/20',
  '197.234.240.0/22',
  '198.41.128.0/17',
  '162.158.0.0/15',
  '104.16.0.0/13',
  '104.24.0.0/14',
  '172.64.0.0/13',
  '131.0.72.0/22',
  // IPv6
  '2400:cb00::/32',
  '2606:4700::/32',
  '2803:f800::/32',
  '2405:b500::/32',
  '2405:8100::/32',
  '2a06:98c0::/29',
  '2c0f:f248::/32'
]

function fillCloudflareIPs() {
  settings.security.trusted_proxies = cloudflareIPs.join('\n')
}

// GeoIP 相关函数
async function loadGeoipStatus() {
  try {
    const response = await axios.get(`${auth.endpoint}/api/admin/settings/geoip`, {
      headers: getHeaders()
    })
    geoipStatus.enabled = response.data.enabled
    geoipStatus.path = response.data.path
  } catch (error) {
    // 静默失败
  }
}

function triggerFileUpload() {
  fileInputRef.value?.click()
}

async function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return

  if (!file.name.toLowerCase().endsWith('.mmdb')) {
    ElMessage.warning(t('settings.geoipSelectFile'))
    input.value = ''
    return
  }

  geoipUploading.value = true
  try {
    const formData = new FormData()
    formData.append('file', file)

    await axios.post(`${auth.endpoint}/api/admin/settings/geoip`, formData, {
      headers: {
        'X-Admin-Token': auth.adminToken,
        'Content-Type': 'multipart/form-data'
      }
    })

    ElMessage.success(t('settings.geoipUploadSuccess'))
    await loadGeoipStatus()
  } catch (error: any) {
    ElMessage.error(t('settings.geoipUploadFailed') + ': ' + (error.response?.data?.message || error.message))
  } finally {
    geoipUploading.value = false
    input.value = ''
  }
}

async function deleteGeoip() {
  try {
    await axios.delete(`${auth.endpoint}/api/admin/settings/geoip`, {
      headers: getHeaders()
    })
    ElMessage.success(t('settings.geoipDeleteSuccess'))
    geoipStatus.enabled = false
    // GeoIP 被删除后，GeoStats 也应该禁用
    geoStatsConfig.enabled = false
  } catch (error: any) {
    ElMessage.error(t('settings.geoipDeleteFailed') + ': ' + (error.response?.data?.message || error.message))
  }
}

// GeoStats 相关函数
async function loadGeoStatsConfig() {
  try {
    const config = await getGeoStatsConfig()
    geoStatsConfig.enabled = config.enabled
    geoStatsConfig.mode = config.mode
    geoStatsConfig.batch_size = config.batch_size
    geoStatsConfig.flush_interval = config.flush_interval
    geoStatsConfig.retention_days = config.retention_days
    geoStatsConfig.geoip_enabled = config.geoip_enabled
  } catch (error) {
    // 静默失败
  }
}

async function handleGeoStatsToggle(enabled: boolean) {
  geoStatsUpdating.value = true
  try {
    await apiUpdateGeoStatsConfig({ enabled })
    ElMessage.success(enabled ? t('settings.geostatsEnableSuccess') : t('settings.geostatsDisableSuccess'))
  } catch (error: any) {
    // 回滚状态
    geoStatsConfig.enabled = !enabled
    ElMessage.error(error.response?.data?.message || t('settings.geostatsUpdateFailed'))
  } finally {
    geoStatsUpdating.value = false
  }
}

async function updateGeoStatsConfig() {
  try {
    await apiUpdateGeoStatsConfig({
      mode: geoStatsConfig.mode,
      batch_size: geoStatsConfig.batch_size,
      flush_interval: geoStatsConfig.flush_interval,
      retention_days: geoStatsConfig.retention_days
    })
    ElMessage.success(t('settings.geostatsConfigSaved'))
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || t('settings.geostatsUpdateFailed'))
  }
}

async function clearGeoStatsData() {
  geoStatsClearing.value = true
  try {
    const result = await apiClearGeoStatsData({ all: true })
    ElMessage.success(result.message || t('settings.geostatsClearSuccess'))
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || t('settings.geostatsClearFailed'))
  } finally {
    geoStatsClearing.value = false
  }
}

// 检测版本更新
async function checkForUpdate() {
  checkingUpdate.value = true
  try {
    const response = await axios.get(`${auth.endpoint}/api/admin/settings/check-update`, {
      headers: getHeaders()
    })
    updateInfo.checked = true
    updateInfo.hasUpdate = response.data.has_update
    updateInfo.latestVersion = response.data.latest_version
    updateInfo.releaseUrl = response.data.release_url || ''
    updateInfo.releaseNotes = response.data.release_notes || ''
    updateInfo.publishedAt = response.data.published_at || ''

    if (updateInfo.hasUpdate) {
      ElMessage.warning(t('settings.newVersionAvailable', { version: updateInfo.latestVersion }))
    } else {
      ElMessage.success(t('settings.alreadyLatest'))
    }
  } catch (error: any) {
    ElMessage.error(t('settings.checkUpdateFailed') + ': ' + (error.response?.data?.message || error.message))
  } finally {
    checkingUpdate.value = false
  }
}

onMounted(() => {
  loadSettings()
  loadGeoipStatus()
  loadGeoStatsConfig()
})
</script>

<style scoped>
.page-container {
  max-width: 1000px;
}

.page-header {
  margin-bottom: 20px;
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

.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.settings-card {
  background: #fff;
  border: 1px solid #eee;
  border-radius: 10px;
  overflow: hidden;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 18px;
  border-bottom: 1px solid #f0f0f0;
  background: #fafafa;
}

.card-icon {
  font-size: 18px;
  color: #e67e22;
}

.card-header h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #333;
}

.card-body {
  padding: 18px;
}

.setting-item {
  margin-bottom: 16px;
}

.setting-item:last-child {
  margin-bottom: 0;
}

.setting-item label {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: #666;
  margin-bottom: 6px;
}

.setting-hint {
  display: block;
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}

.info-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid #f5f5f5;
}

.info-item:last-child {
  border-bottom: none;
}

.info-label {
  color: #888;
  font-size: 13px;
}

.info-value {
  color: #333;
  font-size: 13px;
  font-weight: 500;
}

.info-value.mono {
  font-family: ui-monospace, monospace;
  font-size: 12px;
  color: #666;
}

.full-width-btn {
  width: 100%;
}

.trusted-proxies-input {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.preset-btn {
  align-self: flex-start;
}

.action-bar {
  margin-top: 20px;
  display: flex;
  gap: 10px;
  justify-content: flex-end;
}

.primary-btn {
  background: #e67e22;
  border-color: #e67e22;
}

.primary-btn:hover {
  background: #d35400;
  border-color: #d35400;
}

@media (max-width: 768px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
}

/* GeoIP 相关样式 */
.info-btn {
  margin-left: auto;
  padding: 4px;
}

.geoip-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 16px;
  flex-wrap: wrap;
}

.download-link {
  font-size: 12px;
  color: #e67e22;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-left: auto;
}

.download-link:hover {
  text-decoration: underline;
}

.geoip-info-content {
  color: #666;
  line-height: 1.8;
  white-space: pre-line;
  margin: 0;
}

.geoip-info-links {
  display: flex;
  gap: 16px;
  margin-top: 20px;
}

.info-link {
  color: #e67e22;
  text-decoration: none;
  font-size: 14px;
}

.info-link:hover {
  text-decoration: underline;
}

/* 版本更新相关样式 */
.version-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.check-update-btn {
  font-size: 12px;
  padding: 0;
}

.update-alert {
  margin: 12px 0;
}

.update-alert :deep(.el-alert__content) {
  width: 100%;
}

.update-details {
  margin-top: 8px;
}

.update-details p {
  margin: 0 0 8px;
  font-size: 12px;
  color: #666;
}

.release-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: #e67e22;
  text-decoration: none;
}

.release-link:hover {
  text-decoration: underline;
}

.update-status {
  margin: 12px 0;
}

.update-status .el-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

/* GeoStats 相关样式 */
.dependency-alert {
  margin-bottom: 0;
}

.switch-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.switch-row label {
  margin-bottom: 0;
}
</style>
