<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title">
        <h1>{{ t('buckets.title') }}</h1>
        <p class="page-subtitle">{{ t('buckets.subtitle') }}</p>
      </div>
      <div class="page-actions">
        <el-button @click="handleRefresh" :loading="loading" class="action-btn">
          <el-icon><Refresh /></el-icon>
          <span class="btn-text">{{ t('common.refresh') }}</span>
        </el-button>
        <el-button type="primary" @click="showCreateDialog = true" class="primary-btn">
          <el-icon><Plus /></el-icon>
          <span class="btn-text">{{ t('buckets.createBucket') }}</span>
        </el-button>
      </div>
    </div>

    <!-- 移动端卡片视图 -->
    <div class="mobile-cards" v-if="buckets.length > 0">
      <div v-for="bucket in buckets" :key="bucket.name" class="bucket-card" @click="goToBucket(bucket.name)">
        <div class="bucket-card-header">
          <div class="bucket-icon">
            <el-icon><Folder /></el-icon>
          </div>
          <div class="bucket-info">
            <div class="bucket-name">{{ bucket.name }}</div>
            <div class="bucket-date">{{ formatDate(bucket.creation_date) }}</div>
          </div>
          <el-tag :type="bucket.is_public ? 'warning' : 'info'" size="small">
            {{ bucket.is_public ? t('buckets.public') : t('buckets.private') }}
          </el-tag>
        </div>
        <div class="bucket-card-actions">
          <el-button size="small" @click.stop="handleTogglePublic(bucket.name, !bucket.is_public)">
            {{ bucket.is_public ? t('buckets.setPrivate') : t('buckets.setPublic') }}
          </el-button>
          <el-button size="small" type="danger" @click.stop="handleDelete(bucket.name)">
            {{ t('common.delete') }}
          </el-button>
        </div>
      </div>
    </div>

    <!-- 桌面端表格视图 -->
    <div class="content-card desktop-table">
      <div class="table-wrapper">
        <el-table :data="buckets" v-loading="loading" class="data-table">
          <el-table-column prop="name" :label="t('buckets.bucketName')" min-width="180">
            <template #default="{ row }">
              <router-link :to="{ name: 'Objects', params: { name: row.name } }" class="bucket-link">
                <div class="bucket-icon-sm">
                  <el-icon><Folder /></el-icon>
                </div>
                <span class="bucket-name">{{ row.name }}</span>
              </router-link>
            </template>
          </el-table-column>
          <el-table-column prop="creation_date" :label="t('buckets.createdAt')" width="160">
            <template #default="{ row }">
              <span class="date-text">{{ formatDate(row.creation_date) }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('buckets.access')" width="100" align="center">
            <template #default="{ row }">
              <el-tag
                :type="row.is_public ? 'warning' : 'info'"
                size="small"
                class="access-tag"
                @click="handleTogglePublic(row.name, !row.is_public)"
              >
                {{ row.is_public ? t('buckets.public') : t('buckets.private') }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('common.actions')" width="100" align="center">
            <template #default="{ row }">
              <el-button type="danger" text size="small" @click="handleDelete(row.name)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <el-empty v-if="!loading && buckets.length === 0" :description="t('buckets.noBuckets')">
        <el-button type="primary" @click="showCreateDialog = true">
          {{ t('buckets.createFirst') }}
        </el-button>
      </el-empty>
    </div>

    <el-dialog
      v-model="showCreateDialog"
      :title="t('buckets.createBucketTitle')"
      :width="dialogWidth"
      :close-on-click-modal="false"
    >
      <el-form :model="createForm" label-position="top">
        <el-form-item :label="t('buckets.bucketName')">
          <el-input
            v-model="createForm.name"
            :placeholder="t('buckets.bucketNamePlaceholder')"
            @keyup.enter="handleCreate"
          />
          <div class="form-hint">{{ t('buckets.bucketNameHint') }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleCreate" :loading="creating" class="primary-btn">
          {{ t('common.create') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Plus, Folder, Delete } from '@element-plus/icons-vue'
import { useAuthStore } from '../stores/auth'
import axios from 'axios'

const { t } = useI18n()

interface Bucket {
  name: string
  creation_date: string
  is_public: boolean
  toggling?: boolean
}

const router = useRouter()
const auth = useAuthStore()
const buckets = ref<Bucket[]>([])
const loading = ref(false)
const showCreateDialog = ref(false)
const creating = ref(false)
const createForm = reactive({ name: '' })

const dialogWidth = computed(() => window.innerWidth < 500 ? '90%' : '420px')

function getHeaders() {
  return auth.getAdminHeaders()
}

function goToBucket(name: string) {
  router.push({ name: 'Objects', params: { name } })
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
    ElMessage.error(t('buckets.loadFailed') + ': ' + e.message)
  } finally {
    loading.value = false
  }
}

async function handleCreate() {
  const name = createForm.name.trim().toLowerCase()
  if (!name) {
    ElMessage.warning(t('buckets.pleaseEnterName'))
    return
  }
  creating.value = true
  try {
    await axios.post(`${auth.endpoint}/api/admin/buckets`, {
      name: name
    }, {
      headers: getHeaders()
    })
    ElMessage.success(t('buckets.createSuccess'))
    showCreateDialog.value = false
    createForm.name = ''
    await loadBuckets()
  } catch (e: any) {
    ElMessage.error(t('buckets.createFailed') + ': ' + (e.response?.data?.Message || e.message))
  } finally {
    creating.value = false
  }
}

async function handleDelete(name: string) {
  try {
    await ElMessageBox.confirm(
      t('buckets.deleteConfirm', { name }),
      t('buckets.deleteBucket'),
      {
        type: 'warning',
        confirmButtonText: t('common.delete'),
        confirmButtonClass: 'el-button--danger'
      }
    )
    await axios.delete(`${auth.endpoint}/api/admin/buckets/${name}`, {
      headers: getHeaders()
    })
    ElMessage.success(t('buckets.deleteSuccess'))
    await loadBuckets()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(t('buckets.deleteFailed') + ': ' + (e.response?.data?.Message || e.message))
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
    ElMessage.success(isPublic ? t('buckets.nowPublic') : t('buckets.nowPrivate'))
  } catch (e: any) {
    ElMessage.error(t('buckets.updateAccessFailed') + ': ' + (e.response?.data?.Message || e.message))
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
  max-width: 1000px;
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

.mobile-cards {
  display: none;
}

.bucket-card {
  background: #fff;
  border: 1px solid #eee;
  border-radius: 10px;
  padding: 14px;
  margin-bottom: 12px;
}

.bucket-card-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.bucket-icon {
  width: 40px;
  height: 40px;
  background: #fff5f0;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #e67e22;
  flex-shrink: 0;
}

.bucket-info {
  flex: 1;
  min-width: 0;
}

.bucket-name {
  font-weight: 600;
  color: #333;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bucket-date {
  font-size: 12px;
  color: #888;
  margin-top: 2px;
}

.bucket-card-actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #f0f0f0;
}

.content-card {
  background: #fff;
  border-radius: 10px;
  border: 1px solid #eee;
  overflow: hidden;
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
  gap: 10px;
  color: #333;
  text-decoration: none;
}

.bucket-link:hover {
  color: #e67e22;
}

.bucket-icon-sm {
  width: 32px;
  height: 32px;
  background: #fff5f0;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #e67e22;
}

.date-text {
  color: #888;
  font-size: 13px;
}

.access-tag {
  cursor: pointer;
}

.form-hint {
  font-size: 12px;
  color: #999;
  margin-top: 6px;
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .btn-text {
    display: none;
  }

  .mobile-cards {
    display: block;
  }

  .desktop-table {
    display: none;
  }
}
</style>
