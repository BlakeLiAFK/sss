<template>
  <div class="apikeys-container">
    <div class="header">
      <h2>API Keys</h2>
      <el-button type="primary" @click="showCreateDialog">
        <el-icon><Plus /></el-icon>
        Create API Key
      </el-button>
    </div>

    <el-table :data="apiKeys" v-loading="loading" stripe>
      <el-table-column prop="access_key_id" label="Access Key ID" width="220">
        <template #default="{ row }">
          <code>{{ row.access_key_id }}</code>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="Description" min-width="150" />
      <el-table-column prop="created_at" label="Created At" width="180">
        <template #default="{ row }">
          {{ formatDate(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column prop="enabled" label="Status" width="100">
        <template #default="{ row }">
          <el-switch
            v-model="row.enabled"
            @change="toggleEnabled(row)"
          />
        </template>
      </el-table-column>
      <el-table-column label="Permissions" min-width="200">
        <template #default="{ row }">
          <div class="permissions-list">
            <el-tag
              v-for="perm in row.permissions"
              :key="perm.bucket_name"
              size="small"
              :type="getPermTagType(perm)"
              class="perm-tag"
            >
              {{ perm.bucket_name }}:
              {{ perm.can_read ? 'R' : '' }}{{ perm.can_write ? 'W' : '' }}
            </el-tag>
            <el-tag v-if="!row.permissions?.length" size="small" type="info">
              No permissions
            </el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="Actions" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="showPermDialog(row)">
            Permissions
          </el-button>
          <el-button size="small" type="danger" @click="deleteKey(row)">
            Delete
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Create API Key Dialog -->
    <el-dialog v-model="createDialogVisible" title="Create API Key" width="450px">
      <el-form :model="createForm" label-width="100px">
        <el-form-item label="Description">
          <el-input v-model="createForm.description" placeholder="e.g., Production Server" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">Cancel</el-button>
        <el-button type="primary" @click="createKey" :loading="creating">Create</el-button>
      </template>
    </el-dialog>

    <!-- Show Secret Key Dialog -->
    <el-dialog v-model="secretDialogVisible" title="API Key Created" width="550px" :close-on-click-modal="false">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px">
        Please save the Secret Access Key now. You won't be able to see it again!
      </el-alert>
      <el-descriptions :column="1" border>
        <el-descriptions-item label="Access Key ID">
          <code>{{ newKey.access_key_id }}</code>
          <el-button size="small" text @click="copyToClipboard(newKey.access_key_id)">
            <el-icon><CopyDocument /></el-icon>
          </el-button>
        </el-descriptions-item>
        <el-descriptions-item label="Secret Access Key">
          <code>{{ newKey.secret_access_key }}</code>
          <el-button size="small" text @click="copyToClipboard(newKey.secret_access_key)">
            <el-icon><CopyDocument /></el-icon>
          </el-button>
        </el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button type="primary" @click="secretDialogVisible = false">I've saved it</el-button>
      </template>
    </el-dialog>

    <!-- Permissions Dialog -->
    <el-dialog v-model="permDialogVisible" title="Manage Permissions" width="600px">
      <div v-if="selectedKey">
        <div class="perm-header">
          <span>API Key: <code>{{ selectedKey.access_key_id }}</code></span>
        </div>

        <el-table :data="selectedKey.permissions" size="small" style="margin-bottom: 16px">
          <el-table-column prop="bucket_name" label="Bucket" />
          <el-table-column prop="can_read" label="Read" width="80">
            <template #default="{ row }">
              <el-icon v-if="row.can_read" color="#67C23A"><Check /></el-icon>
              <el-icon v-else color="#909399"><Close /></el-icon>
            </template>
          </el-table-column>
          <el-table-column prop="can_write" label="Write" width="80">
            <template #default="{ row }">
              <el-icon v-if="row.can_write" color="#67C23A"><Check /></el-icon>
              <el-icon v-else color="#909399"><Close /></el-icon>
            </template>
          </el-table-column>
          <el-table-column label="" width="80">
            <template #default="{ row }">
              <el-button size="small" type="danger" text @click="removePerm(row.bucket_name)">
                Remove
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-divider>Add Permission</el-divider>

        <el-form :model="permForm" inline>
          <el-form-item label="Bucket">
            <el-select v-model="permForm.bucket_name" placeholder="Select bucket" style="width: 180px">
              <el-option label="* (All Buckets)" value="*" />
              <el-option
                v-for="bucket in buckets"
                :key="bucket.name"
                :label="bucket.name"
                :value="bucket.name"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="Read">
            <el-switch v-model="permForm.can_read" />
          </el-form-item>
          <el-form-item label="Write">
            <el-switch v-model="permForm.can_write" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="addPerm" :loading="addingPerm">Add</el-button>
          </el-form-item>
        </el-form>
      </div>
      <template #footer>
        <el-button @click="permDialogVisible = false">Close</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '../stores/auth'
import axios from 'axios'

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
    ElMessage.error('Failed to load API keys: ' + e.message)
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
    createDialogVisible.value = false
    secretDialogVisible.value = true
    await loadApiKeys()
    ElMessage.success('API Key created')
  } catch (e: any) {
    ElMessage.error('Failed to create API key: ' + e.message)
  } finally {
    creating.value = false
  }
}

async function toggleEnabled(key: APIKey) {
  try {
    await axios.put(`${auth.endpoint}/api/admin/apikeys/${key.access_key_id}`, {
      enabled: key.enabled
    }, {
      headers: getHeaders()
    })
    ElMessage.success(key.enabled ? 'API Key enabled' : 'API Key disabled')
  } catch (e: any) {
    key.enabled = !key.enabled // Revert
    ElMessage.error('Failed to update API key: ' + e.message)
  }
}

async function deleteKey(key: APIKey) {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete API Key "${key.access_key_id}"?`,
      'Confirm Delete',
      { type: 'warning' }
    )
    await axios.delete(`${auth.endpoint}/api/admin/apikeys/${key.access_key_id}`, {
      headers: getHeaders()
    })
    await loadApiKeys()
    ElMessage.success('API Key deleted')
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error('Failed to delete API key: ' + e.message)
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
    ElMessage.warning('Please select a bucket')
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
    // Update in list
    const idx = apiKeys.value.findIndex(k => k.access_key_id === selectedKey.value?.access_key_id)
    if (idx >= 0) {
      apiKeys.value[idx] = response.data
    }
    ElMessage.success('Permission added')
  } catch (e: any) {
    ElMessage.error('Failed to add permission: ' + e.message)
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
    // Update in list
    const idx = apiKeys.value.findIndex(k => k.access_key_id === selectedKey.value?.access_key_id)
    if (idx >= 0) {
      apiKeys.value[idx] = response.data
    }
    ElMessage.success('Permission removed')
  } catch (e: any) {
    ElMessage.error('Failed to remove permission: ' + e.message)
  }
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleString()
}

function getPermTagType(perm: Permission) {
  if (perm.can_read && perm.can_write) return 'success'
  if (perm.can_write) return 'warning'
  return 'info'
}

async function copyToClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('Copied to clipboard')
  } catch {
    ElMessage.error('Failed to copy')
  }
}

onMounted(() => {
  loadApiKeys()
  loadBuckets()
})
</script>

<style scoped>
.apikeys-container {
  padding: 20px;
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.header h2 {
  margin: 0;
}

.permissions-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.perm-tag {
  font-family: monospace;
}

.perm-header {
  margin-bottom: 16px;
  font-size: 14px;
}

.perm-header code {
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 4px;
}
</style>
