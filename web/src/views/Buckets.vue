<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title">
        <h1>Buckets</h1>
        <p class="page-subtitle">Manage your storage buckets</p>
      </div>
      <div class="page-actions">
        <el-button @click="handleRefresh" :loading="loading">
          <el-icon><Refresh /></el-icon>
          Refresh
        </el-button>
        <el-button type="primary" @click="showCreateDialog = true">
          <el-icon><Plus /></el-icon>
          Create Bucket
        </el-button>
      </div>
    </div>

    <div class="content-card">
      <el-table
        :data="buckets"
        v-loading="loading"
        class="data-table"
        :header-cell-style="{ background: '#f8fafc', color: '#475569', fontWeight: 600 }"
      >
        <el-table-column prop="name" label="Bucket Name" min-width="200">
          <template #default="{ row }">
            <router-link :to="{ name: 'Objects', params: { name: row.name } }" class="bucket-link">
              <div class="bucket-icon">
                <el-icon><Folder /></el-icon>
              </div>
              <span class="bucket-name">{{ row.name }}</span>
            </router-link>
          </template>
        </el-table-column>
        <el-table-column prop="creation_date" label="Created" width="180">
          <template #default="{ row }">
            <span class="date-text">{{ formatDate(row.creation_date) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Access" width="140" align="center">
          <template #default="{ row }">
            <el-tag
              :type="row.is_public ? 'success' : 'info'"
              size="small"
              class="access-tag"
              @click="handleTogglePublic(row.name, !row.is_public)"
              :loading="row.toggling"
            >
              {{ row.is_public ? 'Public' : 'Private' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Actions" width="100" align="center">
          <template #default="{ row }">
            <el-button
              type="danger"
              text
              size="small"
              @click="handleDelete(row.name)"
            >
              <el-icon><Delete /></el-icon>
              Delete
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && buckets.length === 0" description="No buckets yet">
        <el-button type="primary" @click="showCreateDialog = true">
          Create your first bucket
        </el-button>
      </el-empty>
    </div>

    <el-dialog
      v-model="showCreateDialog"
      title="Create Bucket"
      width="420px"
      :close-on-click-modal="false"
    >
      <el-form :model="createForm" label-position="top">
        <el-form-item label="Bucket Name">
          <el-input
            v-model="createForm.name"
            placeholder="Enter bucket name (lowercase, no spaces)"
            size="large"
            @keyup.enter="handleCreate"
          />
          <div class="form-hint">Use lowercase letters, numbers, and hyphens only</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">Cancel</el-button>
        <el-button type="primary" @click="handleCreate" :loading="creating">
          Create Bucket
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Plus, Folder, Delete } from '@element-plus/icons-vue'
import { useAuthStore } from '../stores/auth'
import axios from 'axios'

interface Bucket {
  name: string
  creation_date: string
  is_public: boolean
  toggling?: boolean
}

const auth = useAuthStore()
const buckets = ref<Bucket[]>([])
const loading = ref(false)
const showCreateDialog = ref(false)
const creating = ref(false)
const createForm = reactive({ name: '' })

function getHeaders() {
  return auth.getAdminHeaders()
}

onMounted(() => loadBuckets())

async function loadBuckets() {
  loading.value = true
  try {
    const response = await axios.get(`${auth.endpoint}/api/admin/buckets`, {
      headers: getHeaders()
    })
    buckets.value = response.data || []
  } catch (e: any) {
    ElMessage.error('Failed to load buckets: ' + e.message)
  } finally {
    loading.value = false
  }
}

async function handleCreate() {
  const name = createForm.name.trim().toLowerCase()
  if (!name) {
    ElMessage.warning('Please enter a bucket name')
    return
  }
  creating.value = true
  try {
    await axios.post(`${auth.endpoint}/api/admin/buckets`, {
      name: name
    }, {
      headers: getHeaders()
    })
    ElMessage.success('Bucket created successfully')
    showCreateDialog.value = false
    createForm.name = ''
    await loadBuckets()
  } catch (e: any) {
    ElMessage.error('Failed to create bucket: ' + (e.response?.data?.Message || e.message))
  } finally {
    creating.value = false
  }
}

async function handleDelete(name: string) {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete bucket "${name}"? This action cannot be undone.`,
      'Delete Bucket',
      {
        type: 'warning',
        confirmButtonText: 'Delete',
        confirmButtonClass: 'el-button--danger'
      }
    )
    await axios.delete(`${auth.endpoint}/api/admin/buckets/${name}`, {
      headers: getHeaders()
    })
    ElMessage.success('Bucket deleted successfully')
    await loadBuckets()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error('Failed to delete bucket: ' + (e.response?.data?.Message || e.message))
    }
  }
}

async function handleTogglePublic(bucketName: string, isPublic: boolean) {
  const bucket = buckets.value.find(b => b.name === bucketName)
  if (bucket) {
    bucket.toggling = true
  }

  try {
    await axios.put(`${auth.endpoint}/api/admin/buckets/${bucketName}/public`, {
      is_public: isPublic
    }, {
      headers: getHeaders()
    })
    if (bucket) {
      bucket.is_public = isPublic
    }
    ElMessage.success(`Bucket is now ${isPublic ? 'public' : 'private'}`)
  } catch (e: any) {
    ElMessage.error('Failed to update access: ' + (e.response?.data?.Message || e.message))
  } finally {
    if (bucket) {
      bucket.toggling = false
    }
  }
}

async function handleRefresh() {
  await loadBuckets()
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}
</script>

<style scoped>
.page-container {
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.page-title h1 {
  font-size: 24px;
  font-weight: 600;
  color: #1e293b;
  margin: 0;
}

.page-subtitle {
  font-size: 14px;
  color: #64748b;
  margin: 4px 0 0;
}

.page-actions {
  display: flex;
  gap: 12px;
}

.content-card {
  background: #ffffff;
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
  overflow: hidden;
}

.data-table {
  width: 100%;
}

.data-table :deep(.el-table__header th) {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.bucket-link {
  display: flex;
  align-items: center;
  gap: 12px;
  color: #1e293b;
  text-decoration: none;
  transition: color 0.2s;
}

.bucket-link:hover {
  color: #3b82f6;
}

.bucket-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  background: #eff6ff;
  border-radius: 8px;
  color: #3b82f6;
}

.bucket-name {
  font-weight: 500;
}

.date-text {
  color: #64748b;
  font-size: 13px;
}

.access-tag {
  cursor: pointer;
  transition: transform 0.2s;
}

.access-tag:hover {
  transform: scale(1.05);
}

.form-hint {
  font-size: 12px;
  color: #94a3b8;
  margin-top: 6px;
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
</style>
