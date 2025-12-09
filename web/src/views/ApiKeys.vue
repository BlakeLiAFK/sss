<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title">
        <h1>{{ t('apiKeys.title') }}</h1>
        <p class="page-subtitle">{{ t('apiKeys.subtitle') }}</p>
      </div>
      <div class="page-actions">
        <el-button type="primary" @click="showCreateDialog" class="primary-btn">
          <el-icon><Plus /></el-icon>
          <span class="btn-text">{{ t('apiKeys.create') }}</span>
        </el-button>
      </div>
    </div>

    <div class="content-card">
      <el-table
        :data="apiKeys"
        v-loading="loading"
        class="data-table"
        :header-cell-style="{ background: '#f8fafc', color: '#475569', fontWeight: 600 }"
      >
        <el-table-column prop="access_key_id" label="Access Key ID" width="220">
          <template #default="{ row }">
            <div class="key-cell">
              <code class="key-code">{{ row.access_key_id }}</code>
              <el-button
                text
                size="small"
                class="copy-btn"
                @click="copyToClipboard(row.access_key_id)"
              >
                <el-icon><CopyDocument /></el-icon>
              </el-button>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="description" :label="t('apiKeys.description')" min-width="150">
          <template #default="{ row }">
            <span class="desc-text">{{ row.description || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" :label="t('apiKeys.created')" width="160">
          <template #default="{ row }">
            <span class="date-text">{{ formatDate(row.created_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="enabled" :label="t('apiKeys.status')" width="100" align="center">
          <template #default="{ row }">
            <el-switch
              v-model="row.enabled"
              @change="toggleEnabled(row)"
              :active-text="''"
              :inactive-text="''"
            />
          </template>
        </el-table-column>
        <el-table-column :label="t('apiKeys.permissions')" min-width="220">
          <template #default="{ row }">
            <div class="permissions-list">
              <el-tag
                v-for="perm in row.permissions"
                :key="perm.bucket_name"
                size="small"
                :type="getPermTagType(perm)"
                class="perm-tag"
              >
                <span class="perm-bucket">{{ perm.bucket_name }}</span>
                <span class="perm-flags">{{ perm.can_read ? 'R' : '' }}{{ perm.can_write ? 'W' : '' }}</span>
              </el-tag>
              <span v-if="!row.permissions?.length" class="no-perm">{{ t('apiKeys.noPermissions') }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="260" align="center" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="showPermDialog(row)">
              <el-icon><Setting /></el-icon>
              {{ t('apiKeys.permissions') }}
            </el-button>
            <el-button size="small" type="warning" text @click="resetSecret(row)">
              <el-icon><Refresh /></el-icon>
              {{ t('apiKeys.reset') }}
            </el-button>
            <el-button size="small" type="danger" text @click="deleteKey(row)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && apiKeys.length === 0" :description="t('apiKeys.noKeys')">
        <el-button type="primary" @click="showCreateDialog">
          {{ t('apiKeys.createFirst') }}
        </el-button>
      </el-empty>
    </div>

    <!-- Create API Key Dialog -->
    <el-dialog
      v-model="createDialogVisible"
      :title="t('apiKeys.createTitle')"
      width="450px"
      :close-on-click-modal="false"
    >
      <el-form :model="createForm" label-position="top">
        <el-form-item :label="t('apiKeys.description')">
          <el-input
            v-model="createForm.description"
            :placeholder="t('apiKeys.descriptionPlaceholder')"
            size="large"
          />
          <div class="form-hint">{{ t('apiKeys.descriptionHint') }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" class="primary-btn" @click="createKey" :loading="creating">
          {{ t('apiKeys.createKey') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Show Secret Key Dialog -->
    <el-dialog
      v-model="secretDialogVisible"
      :title="secretDialogTitle"
      width="550px"
      :close-on-click-modal="false"
      :show-close="false"
    >
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        class="secret-warning"
      >
        <template #title>
          <strong>{{ t('apiKeys.saveSecretNow') }}</strong>
        </template>
        {{ t('apiKeys.secretOnlyOnce') }}
      </el-alert>

      <div class="secret-info">
        <div class="secret-row">
          <label>Access Key ID</label>
          <div class="secret-value">
            <code>{{ newKey.access_key_id }}</code>
            <el-button text size="small" @click="copyToClipboard(newKey.access_key_id)">
              <el-icon><CopyDocument /></el-icon>
            </el-button>
          </div>
        </div>
        <div class="secret-row">
          <label>Secret Access Key</label>
          <div class="secret-value">
            <code class="secret-key">{{ newKey.secret_access_key }}</code>
            <el-button text size="small" @click="copyToClipboard(newKey.secret_access_key)">
              <el-icon><CopyDocument /></el-icon>
            </el-button>
          </div>
        </div>
      </div>

      <template #footer>
        <el-button type="primary" class="primary-btn" @click="secretDialogVisible = false">
          {{ t('apiKeys.savedMyKey') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Permissions Dialog -->
    <el-dialog
      v-model="permDialogVisible"
      :title="t('apiKeys.managePermissions')"
      width="600px"
      :close-on-click-modal="false"
    >
      <div v-if="selectedKey" class="perm-dialog-content">
        <div class="perm-key-info">
          <label>API Key:</label>
          <code>{{ selectedKey.access_key_id }}</code>
        </div>

        <div class="perm-section">
          <h4>{{ t('apiKeys.currentPermissions') }}</h4>
          <el-table
            :data="selectedKey.permissions"
            size="small"
            class="perm-table"
            v-if="selectedKey.permissions?.length"
          >
            <el-table-column prop="bucket_name" :label="t('apiKeys.bucket')" />
            <el-table-column :label="t('apiKeys.read')" width="80" align="center">
              <template #default="{ row }">
                <el-icon v-if="row.can_read" color="#10b981" :size="18"><CircleCheck /></el-icon>
                <el-icon v-else color="#94a3b8" :size="18"><CircleClose /></el-icon>
              </template>
            </el-table-column>
            <el-table-column :label="t('apiKeys.write')" width="80" align="center">
              <template #default="{ row }">
                <el-icon v-if="row.can_write" color="#10b981" :size="18"><CircleCheck /></el-icon>
                <el-icon v-else color="#94a3b8" :size="18"><CircleClose /></el-icon>
              </template>
            </el-table-column>
            <el-table-column width="80" align="center">
              <template #default="{ row }">
                <el-button size="small" type="danger" text @click="removePerm(row.bucket_name)">
                  {{ t('apiKeys.remove') }}
                </el-button>
              </template>
            </el-table-column>
          </el-table>
          <div v-else class="no-perm-msg">{{ t('apiKeys.noPermissionsConfigured') }}</div>
        </div>

        <el-divider />

        <div class="perm-section">
          <h4>{{ t('apiKeys.addPermission') }}</h4>
          <el-form :model="permForm" inline class="add-perm-form">
            <el-form-item :label="t('apiKeys.bucket')">
              <el-select v-model="permForm.bucket_name" :placeholder="t('apiKeys.selectBucket')" style="width: 180px">
                <el-option :label="t('apiKeys.allBuckets')" value="*" />
                <el-option
                  v-for="bucket in buckets"
                  :key="bucket.name"
                  :label="bucket.name"
                  :value="bucket.name"
                />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('apiKeys.read')">
              <el-switch v-model="permForm.can_read" />
            </el-form-item>
            <el-form-item :label="t('apiKeys.write')">
              <el-switch v-model="permForm.can_write" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" class="primary-btn" @click="addPerm" :loading="addingPerm">
                {{ t('apiKeys.add') }}
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </div>
      <template #footer>
        <el-button @click="permDialogVisible = false">{{ t('common.close') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, CopyDocument, Setting, Delete, CircleCheck, CircleClose, Refresh } from '@element-plus/icons-vue'
import { useAuthStore } from '../stores/auth'
import axios from 'axios'

const { t } = useI18n()

interface Permission {
  access_key_id: string
  bucket_name: string
  can_read: boolean
  can_write: boolean
}

interface APIKey {
  access_key_id: string
  secret_access_key?: string
  description: string
  created_at: string
  enabled: boolean
  permissions: Permission[]
}

interface Bucket {
  name: string
  creation_date: string
}

const auth = useAuthStore()
const loading = ref(false)
const creating = ref(false)
const addingPerm = ref(false)

const apiKeys = ref<APIKey[]>([])
const buckets = ref<Bucket[]>([])

const createDialogVisible = ref(false)
const secretDialogVisible = ref(false)
const permDialogVisible = ref(false)

const createForm = reactive({ description: '' })
const newKey = reactive({ access_key_id: '', secret_access_key: '' })
const secretDialogTitle = ref(t('apiKeys.keyCreated'))
const selectedKey = ref<APIKey | null>(null)
const permForm = reactive({
  bucket_name: '',
  can_read: true,
  can_write: false
})

function getHeaders() {
  return auth.getAdminHeaders()
}

async function loadApiKeys() {
  loading.value = true
  try {
    const response = await axios.get(`${auth.endpoint}/api/admin/apikeys`, {
      headers: getHeaders()
    })
    apiKeys.value = response.data || []
  } catch (e: any) {
    ElMessage.error(t('apiKeys.loadFailed') + ': ' + e.message)
  } finally {
    loading.value = false
  }
}

async function loadBuckets() {
  try {
    const response = await axios.get(`${auth.endpoint}/api/admin/buckets`, {
      headers: getHeaders()
    })
    buckets.value = response.data || []
  } catch (e: any) {
    console.error('Failed to load buckets:', e)
  }
}

function showCreateDialog() {
  createForm.description = ''
  createDialogVisible.value = true
}

async function createKey() {
  creating.value = true
  try {
    const response = await axios.post(`${auth.endpoint}/api/admin/apikeys`, createForm, {
      headers: getHeaders()
    })
    newKey.access_key_id = response.data.access_key_id
    newKey.secret_access_key = response.data.secret_access_key
    secretDialogTitle.value = t('apiKeys.keyCreated')
    createDialogVisible.value = false
    secretDialogVisible.value = true
    await loadApiKeys()
    ElMessage.success(t('apiKeys.createSuccess'))
  } catch (e: any) {
    ElMessage.error(t('apiKeys.createFailed') + ': ' + e.message)
  } finally {
    creating.value = false
  }
}

async function resetSecret(key: APIKey) {
  try {
    await ElMessageBox.confirm(
      t('apiKeys.resetConfirm'),
      t('apiKeys.resetSecretKey'),
      {
        type: 'warning',
        confirmButtonText: t('apiKeys.reset'),
        confirmButtonClass: 'el-button--warning'
      }
    )
    const response = await axios.post(
      `${auth.endpoint}/api/admin/apikeys/${key.access_key_id}/reset-secret`,
      {},
      { headers: getHeaders() }
    )
    newKey.access_key_id = response.data.access_key_id
    newKey.secret_access_key = response.data.secret_access_key
    secretDialogTitle.value = t('apiKeys.secretKeyReset')
    secretDialogVisible.value = true
    await loadApiKeys()
    ElMessage.success(t('apiKeys.resetSuccess'))
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(t('apiKeys.resetFailed') + ': ' + e.message)
    }
  }
}

async function toggleEnabled(key: APIKey) {
  try {
    await axios.put(`${auth.endpoint}/api/admin/apikeys/${key.access_key_id}`, {
      enabled: key.enabled
    }, {
      headers: getHeaders()
    })
    ElMessage.success(key.enabled ? t('apiKeys.enabled') : t('apiKeys.disabled'))
  } catch (e: any) {
    key.enabled = !key.enabled
    ElMessage.error(t('apiKeys.updateFailed') + ': ' + e.message)
  }
}

async function deleteKey(key: APIKey) {
  try {
    await ElMessageBox.confirm(
      t('apiKeys.deleteConfirm'),
      t('apiKeys.deleteKey'),
      {
        type: 'warning',
        confirmButtonText: t('common.delete'),
        confirmButtonClass: 'el-button--danger'
      }
    )
    await axios.delete(`${auth.endpoint}/api/admin/apikeys/${key.access_key_id}`, {
      headers: getHeaders()
    })
    await loadApiKeys()
    ElMessage.success(t('apiKeys.deleteSuccess'))
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(t('apiKeys.deleteFailed') + ': ' + e.message)
    }
  }
}

function showPermDialog(key: APIKey) {
  selectedKey.value = key
  permForm.bucket_name = ''
  permForm.can_read = true
  permForm.can_write = false
  permDialogVisible.value = true
}

async function addPerm() {
  if (!selectedKey.value || !permForm.bucket_name) {
    ElMessage.warning(t('apiKeys.pleaseSelectBucket'))
    return
  }

  addingPerm.value = true
  try {
    const response = await axios.post(
      `${auth.endpoint}/api/admin/apikeys/${selectedKey.value.access_key_id}/permissions`,
      permForm,
      { headers: getHeaders() }
    )
    selectedKey.value = response.data
    const idx = apiKeys.value.findIndex(k => k.access_key_id === selectedKey.value?.access_key_id)
    if (idx >= 0) {
      apiKeys.value[idx] = response.data
    }
    ElMessage.success(t('apiKeys.permissionAdded'))
  } catch (e: any) {
    ElMessage.error(t('apiKeys.addPermissionFailed') + ': ' + e.message)
  } finally {
    addingPerm.value = false
  }
}

async function removePerm(bucketName: string) {
  if (!selectedKey.value) return

  try {
    const response = await axios.delete(
      `${auth.endpoint}/api/admin/apikeys/${selectedKey.value.access_key_id}/permissions?bucket_name=${encodeURIComponent(bucketName)}`,
      { headers: getHeaders() }
    )
    selectedKey.value = response.data
    const idx = apiKeys.value.findIndex(k => k.access_key_id === selectedKey.value?.access_key_id)
    if (idx >= 0) {
      apiKeys.value[idx] = response.data
    }
    ElMessage.success(t('apiKeys.permissionRemoved'))
  } catch (e: any) {
    ElMessage.error(t('apiKeys.removePermissionFailed') + ': ' + e.message)
  }
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}

function getPermTagType(perm: Permission) {
  if (perm.can_read && perm.can_write) return 'success'
  if (perm.can_write) return 'warning'
  return 'info'
}

async function copyToClipboard(text: string) {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.style.position = 'fixed'
      textarea.style.left = '-9999px'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    ElMessage.success(t('common.copied'))
  } catch {
    ElMessage.error(t('common.copyFailed'))
  }
}

onMounted(() => {
  loadApiKeys()
  loadBuckets()
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
  font-size: 22px;
  font-weight: 600;
  color: #333;
  margin: 0;
}

.page-subtitle {
  font-size: 13px;
  color: #888;
  margin: 4px 0 0;
}

.page-actions {
  display: flex;
  gap: 10px;
}

.primary-btn {
  background: #e67e22;
  border-color: #e67e22;
}

.primary-btn:hover {
  background: #d35400;
  border-color: #d35400;
}

.content-card {
  background: #fff;
  border-radius: 10px;
  border: 1px solid #eee;
  overflow: hidden;
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
  }
  .btn-text {
    display: none;
  }
}

.data-table {
  width: 100%;
}

.data-table :deep(.el-table__header th) {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.key-cell {
  display: flex;
  align-items: center;
  gap: 4px;
}

.key-code {
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
  font-size: 13px;
  background: #f1f5f9;
  padding: 4px 8px;
  border-radius: 4px;
  color: #334155;
}

.copy-btn {
  opacity: 0.5;
  transition: opacity 0.2s;
}

.key-cell:hover .copy-btn {
  opacity: 1;
}

.desc-text {
  color: #64748b;
}

.date-text {
  color: #64748b;
  font-size: 13px;
}

.permissions-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.perm-tag {
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
  font-size: 11px;
}

.perm-bucket {
  margin-right: 4px;
}

.perm-flags {
  font-weight: 600;
}

.no-perm {
  color: #94a3b8;
  font-size: 13px;
}

.form-hint {
  font-size: 12px;
  color: #94a3b8;
  margin-top: 6px;
}

.secret-warning {
  margin-bottom: 20px;
}

.secret-info {
  background: #f8fafc;
  border-radius: 8px;
  padding: 16px;
}

.secret-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 16px;
}

.secret-row:last-child {
  margin-bottom: 0;
}

.secret-row label {
  font-size: 12px;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.secret-value {
  display: flex;
  align-items: center;
  gap: 8px;
}

.secret-value code {
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
  font-size: 14px;
  background: #ffffff;
  padding: 8px 12px;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
  flex: 1;
  word-break: break-all;
}

.secret-key {
  color: #dc2626;
}

.perm-dialog-content {
  padding: 0;
}

.perm-key-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 20px;
  font-size: 14px;
}

.perm-key-info label {
  color: #64748b;
}

.perm-key-info code {
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
  background: #f1f5f9;
  padding: 4px 8px;
  border-radius: 4px;
}

.perm-section h4 {
  font-size: 14px;
  font-weight: 600;
  color: #1e293b;
  margin: 0 0 12px;
}

.perm-table {
  margin-bottom: 8px;
}

.no-perm-msg {
  color: #94a3b8;
  font-size: 13px;
  text-align: center;
  padding: 20px;
  background: #f8fafc;
  border-radius: 6px;
}

.add-perm-form {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

:deep(.el-dialog__header) {
  padding: 20px 24px;
  border-bottom: 1px solid #f1f5f9;
}

:deep(.el-dialog__body) {
  padding: 24px;
}

:deep(.el-dialog__footer) {
  padding: 16px 24px;
  border-top: 1px solid #f1f5f9;
}

:deep(.el-divider) {
  margin: 20px 0;
}
</style>
