<template>
  <div class="buckets-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>Buckets</span>
          <div class="header-actions">
            <el-button type="success" @click="handleRefresh" :loading="loading">
              <el-icon><Refresh /></el-icon>
              Refresh
            </el-button>
            <el-button type="primary" @click="showCreateDialog = true">
              <el-icon><Plus /></el-icon>
              Create Bucket
            </el-button>
          </div>
        </div>
      </template>

      <el-table :data="buckets" v-loading="loading" stripe>
        <el-table-column prop="name" label="Name">
          <template #default="{ row }">
            <router-link :to="{ name: 'Objects', params: { name: row.name } }" class="bucket-link">
              <el-icon><Folder /></el-icon>
              {{ row.name }}
            </router-link>
          </template>
        </el-table-column>
        <el-table-column prop="creation_date" label="Created At" width="200">
          <template #default="{ row }">
            {{ formatDate(row.creation_date) }}
          </template>
        </el-table-column>
        <el-table-column label="Access" width="150">
          <template #default="{ row }">
            <el-switch
              v-model="row.is_public"
              @change="(value: boolean) => handleTogglePublic(row.name, value)"
              active-text="Public"
              inactive-text="Private"
              :loading="row.toggling"
            />
          </template>
        </el-table-column>
        <el-table-column label="Actions" width="120">
          <template #default="{ row }">
            <el-button type="danger" size="small" @click="handleDelete(row.name)">
              Delete
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="showCreateDialog" title="Create Bucket" width="400px">
      <el-form :model="createForm" label-width="80px">
        <el-form-item label="Name">
          <el-input v-model="createForm.name" placeholder="bucket-name" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">Cancel</el-button>
        <el-button type="primary" @click="handleCreate" :loading="creating">Create</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Plus, Folder } from '@element-plus/icons-vue'
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
    ElMessage.error('Failed to load: ' + e.message)
  } finally {
    loading.value = false
  }
}

async function handleCreate() {
  if (!createForm.name.trim()) {
    ElMessage.warning('Please enter bucket name')
    return
  }
  creating.value = true
  try {
    await axios.post(`${auth.endpoint}/api/admin/buckets`, {
      name: createForm.name.trim()
    }, {
      headers: getHeaders()
    })
    ElMessage.success('Created successfully')
    showCreateDialog.value = false
    createForm.name = ''
    await loadBuckets()
  } catch (e: any) {
    ElMessage.error('Failed to create: ' + (e.response?.data?.Message || e.message))
  } finally {
    creating.value = false
  }
}

async function handleDelete(name: string) {
  try {
    await ElMessageBox.confirm(`Are you sure to delete bucket "${name}"?`, 'Confirm Delete', { type: 'warning' })
    await axios.delete(`${auth.endpoint}/api/admin/buckets/${name}`, {
      headers: getHeaders()
    })
    ElMessage.success('Deleted successfully')
    await loadBuckets()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error('Failed to delete: ' + (e.response?.data?.Message || e.message))
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
    ElMessage.success(`Set to ${isPublic ? 'Public' : 'Private'}`)
  } catch (e: any) {
    if (bucket) {
      bucket.is_public = !isPublic
    }
    ElMessage.error('Failed: ' + (e.response?.data?.Message || e.message))
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
  return new Date(dateStr).toLocaleString()
}
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.bucket-link {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #409EFF;
  text-decoration: none;
}

.bucket-link:hover {
  text-decoration: underline;
}
</style>
