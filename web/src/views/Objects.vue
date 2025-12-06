<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title">
        <div class="breadcrumb">
          <router-link to="/" class="breadcrumb-link">Buckets</router-link>
          <el-icon class="breadcrumb-separator"><ArrowRight /></el-icon>
          <span class="breadcrumb-current">{{ bucketName }}</span>
          <span v-if="prefix" class="breadcrumb-prefix">/ {{ prefix }}</span>
        </div>
        <p class="page-subtitle">Browse and manage files</p>
      </div>
      <div class="page-actions">
        <el-input
          v-model="searchKeyword"
          placeholder="Search files..."
          clearable
          class="search-input"
          @input="handleSearchInput"
          @clear="handleSearchClear"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-upload
          :show-file-list="false"
          :before-upload="handleUpload"
          :accept="'*/*'"
          multiple
        >
          <el-button type="primary">
            <el-icon><Upload /></el-icon>
            Upload
          </el-button>
        </el-upload>
      </div>
    </div>

    <!-- Search mode indicator -->
    <div v-if="isSearchMode" class="search-info">
      <el-tag type="info" effect="plain" size="small">
        <el-icon><Search /></el-icon>
        Found {{ searchCount }} results for "{{ searchKeyword }}"
      </el-tag>
      <el-button text type="primary" size="small" @click="handleSearchClear">Clear search</el-button>
    </div>

    <div class="content-card">
      <el-table
        :data="displayObjects"
        v-loading="loading"
        class="data-table"
        :header-cell-style="{ background: '#f8fafc', color: '#475569', fontWeight: 600 }"
        @row-dblclick="handleRowClick"
      >
        <el-table-column prop="Key" label="Name" min-width="300">
          <template #default="{ row }">
            <div class="file-cell">
              <div class="file-icon">
                <el-icon><Document /></el-icon>
              </div>
              <span class="file-name">{{ row.Key }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="Size" label="Size" width="120">
          <template #default="{ row }">
            <span class="size-text">{{ formatSize(row.Size) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="LastModified" label="Modified" width="180">
          <template #default="{ row }">
            <span class="date-text">{{ formatDate(row.LastModified) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Actions" width="280" align="center">
          <template #default="{ row }">
            <el-button size="small" text @click="handleDownload(row.Key)">
              <el-icon><Download /></el-icon>
              Download
            </el-button>
            <el-button size="small" text @click="handleCopyLink(row.Key)">
              <el-icon><Link /></el-icon>
              Copy Link
            </el-button>
            <el-button size="small" text @click="handleRename(row.Key)">
              <el-icon><Edit /></el-icon>
            </el-button>
            <el-button size="small" text type="danger" @click="handleDelete(row.Key)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && displayObjects.length === 0" description="No files yet">
        <el-upload
          :show-file-list="false"
          :before-upload="handleUpload"
          :accept="'*/*'"
          multiple
        >
          <el-button type="primary">Upload your first file</el-button>
        </el-upload>
      </el-empty>

      <div class="pagination" v-if="isTruncated && !isSearchMode">
        <el-button @click="loadMore">Load More</el-button>
      </div>
    </div>

    <!-- Upload Settings Dialog -->
    <el-dialog
      v-model="uploadSettingVisible"
      title="Upload Files"
      width="600px"
      :close-on-click-modal="false"
    >
      <div class="upload-settings">
        <el-alert type="info" :closable="false" class="upload-tip">
          <template #title>
            Specify the full target path for each file
          </template>
        </el-alert>
        <div class="file-list">
          <div v-for="(item, index) in pendingFilesWithPath" :key="index" class="file-item">
            <div class="file-info">
              <el-icon><Document /></el-icon>
              <span class="file-original-name">{{ item.file.name }}</span>
              <span class="file-size">({{ formatSize(item.file.size) }})</span>
              <el-button type="danger" text size="small" class="remove-btn" @click="removePendingFile(index)">
                <el-icon><Close /></el-icon>
              </el-button>
            </div>
            <el-input
              v-model="item.targetPath"
              placeholder="e.g., images/2024/photo.jpg"
              class="file-path-input"
            >
              <template #prepend>/</template>
            </el-input>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="cancelUpload">Cancel</el-button>
        <el-button type="primary" @click="startUpload" :disabled="pendingFilesWithPath.length === 0">
          Upload {{ pendingFilesWithPath.length }} file(s)
        </el-button>
      </template>
    </el-dialog>

    <!-- Upload Progress Dialog -->
    <el-dialog
      v-model="uploadDialogVisible"
      title="Upload Progress"
      :close-on-click-modal="false"
      width="500px"
    >
      <div v-for="(file, index) in uploadFiles" :key="index" class="upload-item">
        <span class="upload-file-path">{{ file.path }}</span>
        <el-progress :percentage="file.progress" :status="file.status" />
      </div>
    </el-dialog>

    <!-- Rename Dialog -->
    <el-dialog
      v-model="renameDialogVisible"
      title="Rename / Move File"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-form label-position="top">
        <el-form-item label="Current Path">
          <el-input :model-value="renameOldKey" disabled />
        </el-form-item>
        <el-form-item label="New Path">
          <el-input v-model="renameNewKey" placeholder="Enter new path">
            <template #prepend>/</template>
          </el-input>
          <div class="form-hint">Change the path to move the file to a different directory</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="renameDialogVisible = false">Cancel</el-button>
        <el-button type="primary" @click="confirmRename" :loading="renaming">Confirm</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowRight, Upload, Document, Delete, Search, Download, Link, Edit, Close } from '@element-plus/icons-vue'
import { listObjects, deleteObject, getObjectUrl, uploadObject, generatePresignedUrl, getBucketPublic, copyObject, searchObjects, type S3Object } from '../api/s3'

const route = useRoute()

const bucketName = computed(() => route.params.name as string)
const prefix = ref('')
const objects = ref<S3Object[]>([])
const loading = ref(false)
const isTruncated = ref(false)
const nextMarker = ref('')
const isPublic = ref(false)

const uploadDialogVisible = ref(false)
const uploadFiles = ref<{ path: string; progress: number; status?: 'success' | 'exception' }[]>([])

// Search related
const searchKeyword = ref('')
const searchResults = ref<S3Object[]>([])
const searchCount = ref(0)
const isSearchMode = computed(() => searchKeyword.value.trim().length > 0)
const displayObjects = computed(() => isSearchMode.value ? searchResults.value : objects.value)
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null

// Upload settings
const uploadSettingVisible = ref(false)
interface PendingFileWithPath {
  file: File
  targetPath: string
}
const pendingFilesWithPath = ref<PendingFileWithPath[]>([])

// Rename related
const renameDialogVisible = ref(false)
const renameOldKey = ref('')
const renameNewKey = ref('')
const renaming = ref(false)

onMounted(() => {
  loadObjects()
  loadBucketPublicStatus()
})

watch(() => route.params.name, () => {
  objects.value = []
  prefix.value = ''
  loadObjects()
  loadBucketPublicStatus()
})

async function loadBucketPublicStatus() {
  try {
    isPublic.value = await getBucketPublic(bucketName.value)
  } catch (e) {
    isPublic.value = false
  }
}

function handleSearchInput() {
  if (searchDebounceTimer) {
    clearTimeout(searchDebounceTimer)
  }

  const keyword = searchKeyword.value.trim()
  if (!keyword) {
    searchResults.value = []
    searchCount.value = 0
    return
  }

  searchDebounceTimer = setTimeout(async () => {
    loading.value = true
    try {
      const result = await searchObjects(bucketName.value, keyword)
      searchResults.value = result.objects
      searchCount.value = result.count
    } catch (e: any) {
      ElMessage.error('Search failed: ' + e.message)
    } finally {
      loading.value = false
    }
  }, 300)
}

function handleSearchClear() {
  searchKeyword.value = ''
  searchResults.value = []
  searchCount.value = 0
  if (searchDebounceTimer) {
    clearTimeout(searchDebounceTimer)
    searchDebounceTimer = null
  }
}

async function loadObjects(append = false) {
  loading.value = true
  try {
    const result = await listObjects(bucketName.value, prefix.value, append ? nextMarker.value : '')
    if (append) {
      objects.value = [...objects.value, ...result.objects]
    } else {
      objects.value = result.objects
    }
    isTruncated.value = result.isTruncated
    nextMarker.value = result.nextMarker
  } catch (e: any) {
    ElMessage.error('Failed to load: ' + e.message)
  } finally {
    loading.value = false
  }
}

function loadMore() {
  loadObjects(true)
}

function handleRowClick(row: S3Object) {
  handleDownload(row.Key)
}

async function handleDownload(key: string) {
  try {
    let url: string
    if (isPublic.value) {
      url = getObjectUrl(bucketName.value, key)
    } else {
      const result = await generatePresignedUrl({
        bucket: bucketName.value,
        key: key,
        method: 'GET',
        expiresMinutes: 60
      })
      url = result.url
    }
    window.open(url, '_blank')
  } catch (e: any) {
    ElMessage.error('Failed to get download link: ' + e.message)
  }
}

async function handleCopyLink(key: string) {
  try {
    let url: string
    if (isPublic.value) {
      url = getObjectUrl(bucketName.value, key)
    } else {
      const result = await generatePresignedUrl({
        bucket: bucketName.value,
        key: key,
        method: 'GET',
        expiresMinutes: 60
      })
      url = result.url
    }
    await navigator.clipboard.writeText(url)
    ElMessage.success(isPublic.value ? 'Link copied' : 'Presigned link copied (valid for 1 hour)')
  } catch (e: any) {
    ElMessage.error('Failed to copy link: ' + e.message)
  }
}

async function handleDelete(key: string) {
  try {
    await ElMessageBox.confirm(
      `Are you sure you want to delete "${key}"?`,
      'Delete File',
      { type: 'warning', confirmButtonText: 'Delete', confirmButtonClass: 'el-button--danger' }
    )
    await deleteObject(bucketName.value, key)
    ElMessage.success('Deleted successfully')
    await loadObjects()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error('Failed to delete: ' + e.message)
    }
  }
}

function handleRename(key: string) {
  renameOldKey.value = key
  renameNewKey.value = key
  renameDialogVisible.value = true
}

async function confirmRename() {
  const oldKey = renameOldKey.value.trim()
  const newKey = renameNewKey.value.trim()

  if (!newKey) {
    ElMessage.warning('Please enter a new path')
    return
  }

  if (oldKey === newKey) {
    ElMessage.info('Path unchanged')
    renameDialogVisible.value = false
    return
  }

  const existingObject = objects.value.find(obj => obj.Key === newKey)
  if (existingObject) {
    try {
      await ElMessageBox.confirm(
        `File "${newKey}" already exists. Overwrite?`,
        'Name Conflict',
        { confirmButtonText: 'Overwrite', cancelButtonText: 'Cancel', type: 'warning' }
      )
    } catch {
      return
    }
  }

  renaming.value = true
  try {
    await copyObject(bucketName.value, oldKey, bucketName.value, newKey)
    await deleteObject(bucketName.value, oldKey)
    ElMessage.success('Renamed successfully')
    renameDialogVisible.value = false
    await loadObjects()
  } catch (e: any) {
    ElMessage.error('Failed to rename: ' + e.message)
  } finally {
    renaming.value = false
  }
}

function handleUpload(file: File) {
  pendingFilesWithPath.value.push({
    file: file,
    targetPath: file.name
  })
  uploadSettingVisible.value = true
  return false
}

function removePendingFile(index: number) {
  pendingFilesWithPath.value.splice(index, 1)
  if (pendingFilesWithPath.value.length === 0) {
    uploadSettingVisible.value = false
  }
}

function cancelUpload() {
  pendingFilesWithPath.value = []
  uploadSettingVisible.value = false
}

async function startUpload() {
  if (pendingFilesWithPath.value.length === 0) return

  for (const item of pendingFilesWithPath.value) {
    if (!item.targetPath.trim()) {
      ElMessage.warning('Please specify target path for all files')
      return
    }
  }

  uploadSettingVisible.value = false
  uploadDialogVisible.value = true

  const filesToUpload = [...pendingFilesWithPath.value]
  uploadFiles.value = filesToUpload.map(item => ({
    path: item.targetPath,
    progress: 0
  }))

  pendingFilesWithPath.value = []

  for (let i = 0; i < filesToUpload.length; i++) {
    const { file, targetPath } = filesToUpload[i]
    try {
      await uploadObject(bucketName.value, targetPath, file, (percent) => {
        uploadFiles.value[i].progress = percent
      })
      uploadFiles.value[i].status = 'success'
    } catch (e: any) {
      uploadFiles.value[i].status = 'exception'
      ElMessage.error(`${targetPath} upload failed: ` + e.message)
    }
  }

  await loadObjects()

  const hasError = uploadFiles.value.some(f => f.status === 'exception')
  const successCount = uploadFiles.value.filter(f => f.status === 'success').length

  if (successCount > 0) {
    ElMessage.success(`Successfully uploaded ${successCount} file(s)`)
  }

  setTimeout(() => {
    uploadDialogVisible.value = false
    uploadFiles.value = []
  }, hasError ? 3000 : 1500)
}

function formatSize(size: number): string {
  if (size < 1024) return size + ' B'
  if (size < 1024 * 1024) return (size / 1024).toFixed(1) + ' KB'
  if (size < 1024 * 1024 * 1024) return (size / 1024 / 1024).toFixed(1) + ' MB'
  return (size / 1024 / 1024 / 1024).toFixed(2) + ' GB'
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

.page-title {
  flex: 1;
}

.breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 20px;
  font-weight: 600;
  color: #1e293b;
}

.breadcrumb-link {
  color: #3b82f6;
  text-decoration: none;
  transition: color 0.2s;
}

.breadcrumb-link:hover {
  color: #1d4ed8;
  text-decoration: underline;
}

.breadcrumb-separator {
  color: #94a3b8;
}

.breadcrumb-current {
  color: #1e293b;
}

.breadcrumb-prefix {
  color: #64748b;
  font-weight: 400;
}

.page-subtitle {
  font-size: 14px;
  color: #64748b;
  margin: 4px 0 0;
}

.page-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

.search-input {
  width: 220px;
}

.search-info {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  padding: 8px 12px;
  background: #f0fdf4;
  border-radius: 8px;
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

.file-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.file-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  background: #f1f5f9;
  border-radius: 6px;
  color: #64748b;
}

.file-name {
  font-weight: 500;
  color: #1e293b;
  word-break: break-all;
}

.size-text {
  color: #64748b;
  font-size: 13px;
}

.date-text {
  color: #64748b;
  font-size: 13px;
}

.pagination {
  padding: 20px;
  text-align: center;
  border-top: 1px solid #f1f5f9;
}

.upload-settings {
  max-height: 400px;
  overflow-y: auto;
}

.upload-tip {
  margin-bottom: 16px;
}

.file-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.file-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  background: #f8fafc;
  border-radius: 8px;
}

.file-info {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: #475569;
}

.file-original-name {
  font-weight: 500;
  flex: 1;
}

.file-size {
  color: #94a3b8;
  font-size: 12px;
}

.remove-btn {
  margin-left: auto;
}

.file-path-input {
  width: 100%;
}

.upload-item {
  margin-bottom: 16px;
}

.upload-file-path {
  display: block;
  margin-bottom: 6px;
  font-size: 13px;
  color: #475569;
  word-break: break-all;
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
