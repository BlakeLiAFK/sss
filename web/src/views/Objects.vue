<template>
  <div
    class="page-container"
    @dragover.prevent
    @dragenter.prevent="handleDragEnter"
    @dragleave.prevent="handleDragLeave"
    @drop.prevent="handleDrop"
  >
    <!-- Drag Overlay -->
    <Transition name="fade">
      <div v-if="isDragging" class="drop-overlay">
        <div class="drop-content">
          <el-icon :size="64"><Upload /></el-icon>
          <h3>{{ t('objects.dropFilesHere') }}</h3>
          <p>{{ t('objects.filesAndFoldersSupported') }}</p>
        </div>
      </div>
    </Transition>

    <div class="page-header">
      <div class="page-title">
        <div class="breadcrumb">
          <router-link to="/buckets" class="breadcrumb-link">{{ t('buckets.title') }}</router-link>
          <el-icon class="breadcrumb-separator"><ArrowRight /></el-icon>
          <span class="breadcrumb-current">{{ bucketName }}</span>
          <span v-if="prefix" class="breadcrumb-prefix">/ {{ prefix }}</span>
        </div>
        <p class="page-subtitle">{{ t('objects.subtitle') }}</p>
      </div>
      <div class="page-actions">
        <el-input
          v-model="searchKeyword"
          :placeholder="t('objects.searchPlaceholder')"
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
          <el-button type="primary" class="primary-btn">
            <el-icon><Upload /></el-icon>
            {{ t('objects.upload') }}
          </el-button>
        </el-upload>
      </div>
    </div>

    <!-- Search mode indicator -->
    <div v-if="isSearchMode" class="search-info">
      <el-tag type="info" effect="plain" size="small">
        <el-icon><Search /></el-icon>
        {{ t('objects.foundResults', { count: searchCount, keyword: searchKeyword }) }}
      </el-tag>
      <el-button text type="primary" size="small" @click="handleSearchClear">{{ t('objects.clearSearch') }}</el-button>
    </div>

    <!-- Batch Action Toolbar -->
    <div v-if="selectedRows.length > 0" class="batch-toolbar">
      <div class="batch-info">
        <el-tag type="primary" effect="light">
          {{ t('objects.selected', { count: selectedRows.length }) }}
        </el-tag>
      </div>
      <div class="batch-actions">
        <el-button type="primary" plain size="small" @click="handleBatchDownload" :loading="batchDownloading">
          <el-icon><Download /></el-icon>
          {{ t('objects.downloadAsZip') }}
        </el-button>
        <el-button type="danger" plain size="small" @click="handleBatchDelete">
          <el-icon><Delete /></el-icon>
          {{ t('objects.deleteSelected') }}
        </el-button>
        <el-button text size="small" @click="clearSelection">
          {{ t('objects.clear') }}
        </el-button>
      </div>
    </div>

    <div class="content-card">
      <el-table
        ref="tableRef"
        :data="displayObjects"
        v-loading="loading"
        class="data-table"
        :header-cell-style="{ background: '#f8fafc', color: '#475569', fontWeight: 600 }"
        @row-dblclick="handleRowClick"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="55" />
        <el-table-column prop="key" :label="t('objects.name')" min-width="300">
          <template #default="{ row }">
            <div class="file-cell">
              <div class="file-icon">
                <el-icon><Document /></el-icon>
              </div>
              <span class="file-name">{{ row.key }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="size" :label="t('objects.size')" width="120">
          <template #default="{ row }">
            <span class="size-text">{{ formatSize(row.size) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="last_modified" :label="t('objects.modified')" width="180">
          <template #default="{ row }">
            <span class="date-text">{{ formatDate(row.last_modified) }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('objects.actions')" width="320" align="center">
          <template #default="{ row }">
            <el-button size="small" text @click="handlePreview(row)">
              <el-icon><View /></el-icon>
              {{ t('objects.preview') }}
            </el-button>
            <el-button size="small" text @click="handleDownload(row.key)">
              <el-icon><Download /></el-icon>
              {{ t('objects.download') }}
            </el-button>
            <el-button size="small" text @click="handleCopyLink(row.key)">
              <el-icon><Link /></el-icon>
            </el-button>
            <el-button size="small" text @click="handleRename(row.key)">
              <el-icon><Edit /></el-icon>
            </el-button>
            <el-button size="small" text type="danger" @click="handleDelete(row.key)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && displayObjects.length === 0" :description="t('objects.noFiles')">
        <el-upload
          :show-file-list="false"
          :before-upload="handleUpload"
          :accept="'*/*'"
          multiple
        >
          <el-button type="primary" class="primary-btn">{{ t('objects.uploadFirstFile') }}</el-button>
        </el-upload>
      </el-empty>

      <div class="pagination" v-if="isTruncated && !isSearchMode">
        <el-button @click="loadMore">{{ t('objects.loadMore') }}</el-button>
      </div>
    </div>

    <!-- Upload Settings Dialog -->
    <el-dialog
      v-model="uploadSettingVisible"
      :title="t('objects.uploadFiles')"
      width="600px"
      :close-on-click-modal="false"
    >
      <div class="upload-settings">
        <el-alert type="info" :closable="false" class="upload-tip">
          <template #title>
            {{ t('objects.specifyTargetPath') }}
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
              :placeholder="t('objects.targetPathPlaceholder')"
              class="file-path-input"
            >
              <template #prepend>/</template>
            </el-input>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="cancelUpload">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" class="primary-btn" @click="startUpload" :disabled="pendingFilesWithPath.length === 0">
          {{ t('objects.uploadCount', { count: pendingFilesWithPath.length }) }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Upload Progress Dialog -->
    <el-dialog
      v-model="uploadDialogVisible"
      :title="t('objects.uploadProgress')"
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
      :title="t('objects.renameMove')"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-form label-position="top">
        <el-form-item :label="t('objects.currentPath')">
          <el-input :model-value="renameOldKey" disabled />
        </el-form-item>
        <el-form-item :label="t('objects.newPath')">
          <el-input v-model="renameNewKey" :placeholder="t('objects.enterNewPath')">
            <template #prepend>/</template>
          </el-input>
          <div class="form-hint">{{ t('objects.changePathHint') }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="renameDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" class="primary-btn" @click="confirmRename" :loading="renaming">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- Preview Dialog -->
    <el-dialog
      v-model="previewDialogVisible"
      :title="t('objects.previewTitle', { name: previewKey })"
      width="80%"
      :close-on-click-modal="true"
      class="preview-dialog"
      destroy-on-close
    >
      <div v-loading="previewLoading" class="preview-container">
        <!-- 图片预览 -->
        <div v-if="previewType === 'image'" class="preview-image-wrapper">
          <img :src="previewUrl" :alt="previewKey" class="preview-image" />
        </div>

        <!-- 视频预览 -->
        <div v-else-if="previewType === 'video'" class="preview-video-wrapper">
          <video :src="previewUrl" controls class="preview-video">
            {{ t('objects.browserNoVideoSupport') }}
          </video>
        </div>

        <!-- 音频预览 -->
        <div v-else-if="previewType === 'audio'" class="preview-audio-wrapper">
          <div class="audio-icon">
            <el-icon :size="80"><Document /></el-icon>
          </div>
          <div class="audio-filename">{{ previewKey }}</div>
          <audio :src="previewUrl" controls class="preview-audio">
            {{ t('objects.browserNoAudioSupport') }}
          </audio>
        </div>

        <!-- PDF 预览 -->
        <div v-else-if="previewType === 'pdf'" class="preview-pdf-wrapper">
          <iframe :src="previewUrl" class="preview-pdf"></iframe>
        </div>

        <!-- 文本预览 -->
        <div v-else-if="previewType === 'text'" class="preview-text-wrapper">
          <pre class="preview-text">{{ previewContent }}</pre>
        </div>

        <!-- 不支持的格式 -->
        <div v-else class="preview-unsupported">
          <el-icon :size="64" color="#94a3b8"><Document /></el-icon>
          <h3>{{ t('objects.previewNotAvailable') }}</h3>
          <p>{{ t('objects.fileTypeCannotPreview') }}</p>
          <p class="file-info">
            <strong>{{ t('objects.file') }}:</strong> {{ previewKey }}<br />
            <strong>{{ t('objects.size') }}:</strong> {{ formatSize(previewSize) }}
          </p>
          <el-button type="primary" class="primary-btn" @click="handleDownload(previewKey)">
            <el-icon><Download /></el-icon>
            {{ t('objects.downloadFile') }}
          </el-button>
        </div>
      </div>
      <template #footer>
        <div class="preview-footer">
          <span class="preview-size">{{ formatSize(previewSize) }}</span>
          <div class="preview-actions">
            <el-button @click="handleCopyLink(previewKey)">
              <el-icon><Link /></el-icon>
              {{ t('objects.copyLink') }}
            </el-button>
            <el-button type="primary" class="primary-btn" @click="handleDownload(previewKey)">
              <el-icon><Download /></el-icon>
              {{ t('objects.download') }}
            </el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox, type TableInstance } from 'element-plus'
import { ArrowRight, Upload, Document, Delete, Search, Download, Link, Edit, Close, View } from '@element-plus/icons-vue'
import { listObjects, deleteObject, getObjectUrl, uploadObject, generatePresignedUrl, getBucketPublic, copyObject, searchObjects, batchDeleteObjects, batchDownloadObjects, type S3Object } from '../api/admin'

const { t } = useI18n()
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

// Batch operation related
const tableRef = ref<TableInstance>()
const selectedRows = ref<S3Object[]>([])
const batchDownloading = ref(false)

// Drag and drop related
const isDragging = ref(false)
let dragCounter = 0

// Preview related
const previewDialogVisible = ref(false)
const previewLoading = ref(false)
const previewUrl = ref('')
const previewKey = ref('')
const previewType = ref<'image' | 'video' | 'audio' | 'text' | 'pdf' | 'unsupported'>('unsupported')
const previewContent = ref('')
const previewSize = ref(0)
const MAX_TEXT_PREVIEW_SIZE = 512 * 1024

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
      ElMessage.error(t('objects.searchFailed') + ': ' + e.message)
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
    isTruncated.value = result.is_truncated
    nextMarker.value = result.next_marker
  } catch (e: any) {
    ElMessage.error(t('objects.loadFailed') + ': ' + e.message)
  } finally {
    loading.value = false
  }
}

function loadMore() {
  loadObjects(true)
}

function handleRowClick(row: S3Object) {
  handleDownload(row.key)
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
    ElMessage.error(t('objects.getDownloadLinkFailed') + ': ' + e.message)
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
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(url)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = url
      textarea.style.position = 'fixed'
      textarea.style.left = '-9999px'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    ElMessage.success(isPublic.value ? t('objects.linkCopied') : t('objects.presignedLinkCopied'))
  } catch (e: any) {
    ElMessage.error(t('objects.copyLinkFailed') + ': ' + e.message)
  }
}

async function handleDelete(key: string) {
  try {
    await ElMessageBox.confirm(
      t('objects.deleteConfirm', { name: key }),
      t('objects.deleteFile'),
      { type: 'warning', confirmButtonText: t('common.delete'), confirmButtonClass: 'el-button--danger' }
    )
    await deleteObject(bucketName.value, key)
    ElMessage.success(t('objects.deleteSuccess'))
    await loadObjects()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(t('objects.deleteFailed') + ': ' + e.message)
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
    ElMessage.warning(t('objects.pleaseEnterNewPath'))
    return
  }

  if (oldKey === newKey) {
    ElMessage.info(t('objects.pathUnchanged'))
    renameDialogVisible.value = false
    return
  }

  const existingObject = objects.value.find(obj => obj.key === newKey)
  if (existingObject) {
    try {
      await ElMessageBox.confirm(
        t('objects.fileExistsOverwrite', { name: newKey }),
        t('objects.nameConflict'),
        { confirmButtonText: t('objects.overwrite'), cancelButtonText: t('common.cancel'), type: 'warning' }
      )
    } catch {
      return
    }
  }

  renaming.value = true
  try {
    await copyObject(bucketName.value, oldKey, newKey)
    await deleteObject(bucketName.value, oldKey)
    ElMessage.success(t('objects.renameSuccess'))
    renameDialogVisible.value = false
    await loadObjects()
  } catch (e: any) {
    ElMessage.error(t('objects.renameFailed') + ': ' + e.message)
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
      ElMessage.warning(t('objects.pleaseSpecifyTargetPath'))
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
      ElMessage.error(`${targetPath} ${t('objects.uploadFailed')}: ` + e.message)
    }
  }

  await loadObjects()

  const hasError = uploadFiles.value.some(f => f.status === 'exception')
  const successCount = uploadFiles.value.filter(f => f.status === 'success').length

  if (successCount > 0) {
    ElMessage.success(t('objects.uploadSuccessCount', { count: successCount }))
  }

  setTimeout(() => {
    uploadDialogVisible.value = false
    uploadFiles.value = []
  }, hasError ? 3000 : 1500)
}

function handleDragEnter(_e: DragEvent) {
  dragCounter++
  isDragging.value = true
}

function handleDragLeave(_e: DragEvent) {
  dragCounter--
  if (dragCounter <= 0) {
    dragCounter = 0
    isDragging.value = false
  }
}

async function handleDrop(e: DragEvent) {
  dragCounter = 0
  isDragging.value = false

  const items = e.dataTransfer?.items
  if (!items || items.length === 0) return

  const files: PendingFileWithPath[] = []
  const promises: Promise<void>[] = []

  for (let i = 0; i < items.length; i++) {
    const item = items[i]
    if (item.kind !== 'file') continue

    const entry = item.webkitGetAsEntry?.()
    if (entry) {
      promises.push(processEntry(entry, '', files))
    } else {
      const file = item.getAsFile()
      if (file) {
        files.push({ file, targetPath: file.name })
      }
    }
  }

  await Promise.all(promises)

  if (files.length === 0) {
    ElMessage.warning(t('objects.noFilesFound'))
    return
  }

  pendingFilesWithPath.value = [...pendingFilesWithPath.value, ...files]
  uploadSettingVisible.value = true

  ElMessage.success(t('objects.addedFilesForUpload', { count: files.length }))
}

async function processEntry(entry: FileSystemEntry, basePath: string, files: PendingFileWithPath[]): Promise<void> {
  if (entry.isFile) {
    const fileEntry = entry as FileSystemFileEntry
    return new Promise((resolve) => {
      fileEntry.file((file) => {
        const targetPath = basePath ? `${basePath}/${entry.name}` : entry.name
        files.push({ file, targetPath })
        resolve()
      }, () => resolve())
    })
  } else if (entry.isDirectory) {
    const dirEntry = entry as FileSystemDirectoryEntry
    const dirPath = basePath ? `${basePath}/${entry.name}` : entry.name
    const reader = dirEntry.createReader()

    return new Promise((resolve) => {
      const readAllEntries = (allEntries: FileSystemEntry[] = []) => {
        reader.readEntries(async (entries) => {
          if (entries.length === 0) {
            const subPromises = allEntries.map(e => processEntry(e, dirPath, files))
            await Promise.all(subPromises)
            resolve()
          } else {
            readAllEntries([...allEntries, ...entries])
          }
        }, () => resolve())
      }
      readAllEntries()
    })
  }
}

function handleSelectionChange(rows: S3Object[]) {
  selectedRows.value = rows
}

async function handleBatchDelete() {
  if (selectedRows.value.length === 0) return

  try {
    await ElMessageBox.confirm(
      t('objects.batchDeleteConfirm', { count: selectedRows.value.length }),
      t('objects.batchDelete'),
      { type: 'warning', confirmButtonText: t('objects.deleteAll'), confirmButtonClass: 'el-button--danger' }
    )

    const keys = selectedRows.value.map(row => row.key)
    const result = await batchDeleteObjects(bucketName.value, keys)

    if (result.deleted_count > 0) {
      ElMessage.success(t('objects.deletedCount', { count: result.deleted_count }))
    }
    if (result.failed_count > 0) {
      ElMessage.warning(t('objects.failedToDeleteCount', { count: result.failed_count }))
    }

    clearSelection()
    await loadObjects()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(t('objects.batchDeleteFailed') + ': ' + e.message)
    }
  }
}

async function handleBatchDownload() {
  if (selectedRows.value.length === 0) return

  batchDownloading.value = true
  try {
    const keys = selectedRows.value.map(row => row.key)
    const blob = await batchDownloadObjects(bucketName.value, keys)

    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `${bucketName.value}-${Date.now()}.zip`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)

    ElMessage.success(t('objects.downloadedAsZip', { count: keys.length }))
  } catch (e: any) {
    ElMessage.error(t('objects.batchDownloadFailed') + ': ' + e.message)
  } finally {
    batchDownloading.value = false
  }
}

function clearSelection() {
  selectedRows.value = []
  tableRef.value?.clearSelection()
}

function getPreviewType(key: string): 'image' | 'video' | 'audio' | 'text' | 'pdf' | 'unsupported' {
  const ext = key.toLowerCase().split('.').pop() || ''

  const imageExts = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico']
  if (imageExts.includes(ext)) return 'image'

  const videoExts = ['mp4', 'webm', 'ogg', 'mov', 'm4v']
  if (videoExts.includes(ext)) return 'video'

  const audioExts = ['mp3', 'wav', 'ogg', 'flac', 'm4a', 'aac']
  if (audioExts.includes(ext)) return 'audio'

  if (ext === 'pdf') return 'pdf'

  const textExts = ['txt', 'md', 'json', 'xml', 'html', 'css', 'js', 'ts', 'jsx', 'tsx',
                    'vue', 'go', 'py', 'java', 'c', 'cpp', 'h', 'hpp', 'rs', 'rb', 'php',
                    'sh', 'bash', 'yaml', 'yml', 'toml', 'ini', 'conf', 'log', 'csv', 'sql']
  if (textExts.includes(ext)) return 'text'

  return 'unsupported'
}

async function handlePreview(row: S3Object) {
  previewKey.value = row.key
  previewSize.value = row.size
  previewType.value = getPreviewType(row.key)
  previewContent.value = ''
  previewUrl.value = ''
  previewDialogVisible.value = true
  previewLoading.value = true

  try {
    let url: string
    if (isPublic.value) {
      url = getObjectUrl(bucketName.value, row.key)
    } else {
      const result = await generatePresignedUrl({
        bucket: bucketName.value,
        key: row.key,
        method: 'GET',
        expiresMinutes: 60
      })
      url = result.url
    }
    previewUrl.value = url

    if (previewType.value === 'text') {
      if (row.size > MAX_TEXT_PREVIEW_SIZE) {
        previewContent.value = t('objects.fileTooLargeToPreview', { size: formatSize(row.size), maxSize: formatSize(MAX_TEXT_PREVIEW_SIZE) })
      } else {
        const response = await fetch(url)
        if (response.ok) {
          previewContent.value = await response.text()
        } else {
          previewContent.value = t('objects.failedToLoadContent')
        }
      }
    }
  } catch (e: any) {
    ElMessage.error(t('objects.loadPreviewFailed') + ': ' + e.message)
    previewDialogVisible.value = false
  } finally {
    previewLoading.value = false
  }
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
  color: #e67e22;
  text-decoration: none;
  transition: color 0.2s;
}

.breadcrumb-link:hover {
  color: #d35400;
  text-decoration: underline;
}

.primary-btn {
  background: #e67e22;
  border-color: #e67e22;
}

.primary-btn:hover {
  background: #d35400;
  border-color: #d35400;
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

.batch-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  margin-bottom: 16px;
  background: linear-gradient(135deg, #eff6ff, #eef2ff);
  border: 1px solid #bfdbfe;
  border-radius: 10px;
}

.batch-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.batch-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.preview-dialog :deep(.el-dialog__body) {
  padding: 0;
  max-height: 70vh;
  overflow: auto;
}

.preview-container {
  min-height: 300px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.preview-image-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: #f8fafc;
}

.preview-image {
  max-width: 100%;
  max-height: 65vh;
  object-fit: contain;
  border-radius: 4px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.preview-video-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: #000;
  width: 100%;
}

.preview-video {
  max-width: 100%;
  max-height: 65vh;
}

.preview-audio-wrapper {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  width: 100%;
  background: linear-gradient(135deg, #f8fafc, #e2e8f0);
}

.audio-icon {
  color: #64748b;
  margin-bottom: 16px;
}

.audio-filename {
  font-size: 16px;
  font-weight: 500;
  color: #1e293b;
  margin-bottom: 24px;
  word-break: break-all;
  text-align: center;
  max-width: 80%;
}

.preview-audio {
  width: 100%;
  max-width: 400px;
}

.preview-pdf-wrapper {
  width: 100%;
  height: 70vh;
}

.preview-pdf {
  width: 100%;
  height: 100%;
  border: none;
}

.preview-text-wrapper {
  width: 100%;
  padding: 20px;
  background: #1e293b;
  overflow: auto;
  max-height: 70vh;
}

.preview-text {
  margin: 0;
  padding: 16px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 13px;
  line-height: 1.6;
  color: #e2e8f0;
  white-space: pre-wrap;
  word-wrap: break-word;
  background: transparent;
}

.preview-unsupported {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  text-align: center;
  color: #64748b;
}

.preview-unsupported h3 {
  margin: 16px 0 8px;
  color: #1e293b;
  font-size: 18px;
}

.preview-unsupported p {
  margin: 0 0 8px;
  font-size: 14px;
}

.preview-unsupported .file-info {
  margin: 16px 0 24px;
  padding: 12px 20px;
  background: #f8fafc;
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.8;
}

.preview-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.preview-size {
  color: #64748b;
  font-size: 13px;
}

.preview-actions {
  display: flex;
  gap: 8px;
}

.drop-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(230, 126, 34, 0.15);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  pointer-events: none;
}

.drop-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 64px;
  background: white;
  border-radius: 20px;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.25);
  border: 3px dashed #e67e22;
  text-align: center;
}

.drop-content .el-icon {
  color: #e67e22;
  margin-bottom: 16px;
  animation: bounce 1s infinite;
}

.drop-content h3 {
  margin: 0 0 8px;
  font-size: 24px;
  font-weight: 600;
  color: #1e293b;
}

.drop-content p {
  margin: 0;
  font-size: 14px;
  color: #64748b;
}

@keyframes bounce {
  0%, 100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-10px);
  }
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
