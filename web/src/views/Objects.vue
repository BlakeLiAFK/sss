<template>
  <div class="objects-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <div class="breadcrumb">
            <router-link to="/" class="back-link">存储桶</router-link>
            <el-icon><ArrowRight /></el-icon>
            <span>{{ bucketName }}</span>
            <span v-if="prefix" class="prefix">/ {{ prefix }}</span>
          </div>
          <div class="header-actions">
            <el-input
              v-model="searchKeyword"
              placeholder="搜索文件名..."
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
                上传文件
              </el-button>
            </el-upload>
          </div>
        </div>
      </template>

      <!-- 搜索模式提示 -->
      <div v-if="isSearchMode" class="search-info">
        <el-tag type="info" effect="plain">
          <el-icon><Search /></el-icon>
          搜索 "{{ searchKeyword }}" 找到 {{ searchCount }} 个结果
        </el-tag>
        <el-button text type="primary" @click="handleSearchClear">清除搜索</el-button>
      </div>

      <el-table :data="displayObjects" v-loading="loading" stripe @row-dblclick="handleRowClick">
        <el-table-column prop="Key" label="名称">
          <template #default="{ row }">
            <div class="object-name">
              <el-icon><Document /></el-icon>
              {{ row.Key }}
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="Size" label="大小" width="120">
          <template #default="{ row }">
            {{ formatSize(row.Size) }}
          </template>
        </el-table-column>
        <el-table-column prop="LastModified" label="修改时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.LastModified) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="320">
          <template #default="{ row }">
            <el-button size="small" @click="handleDownload(row.Key)">下载</el-button>
            <el-button size="small" @click="handleCopyLink(row.Key)">复制链接</el-button>
            <el-button size="small" @click="handleRename(row.Key)">重命名</el-button>
            <el-button type="danger" size="small" @click="handleDelete(row.Key)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination" v-if="isTruncated">
        <el-button @click="loadMore">加载更多</el-button>
      </div>
    </el-card>

    <!-- 上传设置对话框 - GitHub风格 -->
    <el-dialog v-model="uploadSettingVisible" title="上传文件" width="600px">
      <div class="upload-github-style">
        <p class="upload-tip">
          <el-icon><InfoFilled /></el-icon>
          可以为每个文件指定完整目标路径（类似 GitHub）
        </p>
        <div class="file-list">
          <div v-for="(item, index) in pendingFilesWithPath" :key="index" class="file-item">
            <div class="file-info">
              <el-icon><Document /></el-icon>
              <span class="file-original-name">{{ item.file.name }}</span>
              <span class="file-size">({{ formatSize(item.file.size) }})</span>
            </div>
            <el-input
              v-model="item.targetPath"
              placeholder="输入完整目标路径，如: images/2024/photo.jpg"
              class="file-path-input"
            >
              <template #prepend>/</template>
            </el-input>
            <el-button type="danger" text @click="removePendingFile(index)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="cancelUpload">取消</el-button>
        <el-button type="primary" @click="startUpload" :disabled="pendingFilesWithPath.length === 0">
          开始上传 ({{ pendingFilesWithPath.length }} 个文件)
        </el-button>
      </template>
    </el-dialog>

    <!-- 上传进度 -->
    <el-dialog v-model="uploadDialogVisible" title="上传进度" :close-on-click-modal="false" width="500px">
      <div v-for="(file, index) in uploadFiles" :key="index" class="upload-item">
        <span class="upload-file-path">{{ file.path }}</span>
        <el-progress :percentage="file.progress" :status="file.status" />
      </div>
    </el-dialog>

    <!-- 重命名对话框 -->
    <el-dialog v-model="renameDialogVisible" title="重命名/移动文件" width="500px">
      <el-form label-width="80px">
        <el-form-item label="原路径">
          <el-input :model-value="renameOldKey" disabled />
        </el-form-item>
        <el-form-item label="新路径">
          <el-input v-model="renameNewKey" placeholder="输入新的完整路径">
            <template #prepend>/</template>
          </el-input>
          <div class="form-tip">修改路径可以移动文件到不同目录</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="renameDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmRename" :loading="renaming">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowRight, Upload, Document, Delete, InfoFilled, Search } from '@element-plus/icons-vue'
import { listObjects, deleteObject, getObjectUrl, uploadObject, generatePresignedUrl, getBucketPublic, copyObject, searchObjects, type S3Object } from '../api/s3'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const auth = useAuthStore()

const bucketName = computed(() => route.params.name as string)
const prefix = ref('')
const objects = ref<S3Object[]>([])
const loading = ref(false)
const isTruncated = ref(false)
const nextMarker = ref('')
const isPublic = ref(false)

const uploadDialogVisible = ref(false)
const uploadFiles = ref<{ path: string; progress: number; status?: 'success' | 'exception' }[]>([])

// 搜索相关
const searchKeyword = ref('')
const searchResults = ref<S3Object[]>([])
const searchCount = ref(0)
const isSearchMode = computed(() => searchKeyword.value.trim().length > 0)
const displayObjects = computed(() => isSearchMode.value ? searchResults.value : objects.value)
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null

// 上传设置相关 - GitHub风格
const uploadSettingVisible = ref(false)
interface PendingFileWithPath {
  file: File
  targetPath: string
}
const pendingFilesWithPath = ref<PendingFileWithPath[]>([])

// 重命名相关
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

// 搜索处理函数（带防抖）
function handleSearchInput() {
  // 清除之前的定时器
  if (searchDebounceTimer) {
    clearTimeout(searchDebounceTimer)
  }

  const keyword = searchKeyword.value.trim()
  if (!keyword) {
    // 清空搜索时立即恢复
    searchResults.value = []
    searchCount.value = 0
    return
  }

  // 防抖：300ms后执行搜索
  searchDebounceTimer = setTimeout(async () => {
    loading.value = true
    try {
      const result = await searchObjects(bucketName.value, keyword)
      searchResults.value = result.objects
      searchCount.value = result.count
    } catch (e: any) {
      ElMessage.error('搜索失败: ' + e.message)
    } finally {
      loading.value = false
    }
  }, 300)
}

// 清除搜索
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
    ElMessage.error('加载失败: ' + e.message)
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
    ElMessage.error('获取下载链接失败: ' + e.message)
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
    ElMessage.success(isPublic.value ? '链接已复制' : '预签名链接已复制（1小时有效）')
  } catch (e: any) {
    ElMessage.error('复制链接失败: ' + e.message)
  }
}

async function handleDelete(key: string) {
  try {
    await ElMessageBox.confirm(`确定要删除 "${key}" 吗？`, '确认删除', { type: 'warning' })
    await deleteObject(bucketName.value, key)
    ElMessage.success('删除成功')
    await loadObjects()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error('删除失败: ' + e.message)
    }
  }
}

// 重命名/移动文件
function handleRename(key: string) {
  renameOldKey.value = key
  renameNewKey.value = key
  renameDialogVisible.value = true
}

async function confirmRename() {
  const oldKey = renameOldKey.value.trim()
  const newKey = renameNewKey.value.trim()

  if (!newKey) {
    ElMessage.warning('请输入新路径')
    return
  }

  if (oldKey === newKey) {
    ElMessage.info('路径未改变')
    renameDialogVisible.value = false
    return
  }

  // 检查目标路径是否已存在（在已加载的列表中）
  const existingObject = objects.value.find(obj => obj.Key === newKey)
  if (existingObject) {
    try {
      await ElMessageBox.confirm(
        `文件 "${newKey}" 已存在，是否覆盖？`,
        '名称冲突',
        { confirmButtonText: '覆盖', cancelButtonText: '取消', type: 'warning' }
      )
    } catch {
      return // 用户取消
    }
  }

  renaming.value = true
  try {
    // 复制到新路径
    await copyObject(bucketName.value, oldKey, bucketName.value, newKey)
    // 删除旧文件
    await deleteObject(bucketName.value, oldKey)
    ElMessage.success('重命名成功')
    renameDialogVisible.value = false
    await loadObjects()
  } catch (e: any) {
    ElMessage.error('重命名失败: ' + e.message)
  } finally {
    renaming.value = false
  }
}

// 收集待上传文件，显示设置对话框
function handleUpload(file: File) {
  pendingFilesWithPath.value.push({
    file: file,
    targetPath: file.name
  })
  uploadSettingVisible.value = true
  return false
}

// 从待上传列表移除文件
function removePendingFile(index: number) {
  pendingFilesWithPath.value.splice(index, 1)
  if (pendingFilesWithPath.value.length === 0) {
    uploadSettingVisible.value = false
  }
}

// 取消上传
function cancelUpload() {
  pendingFilesWithPath.value = []
  uploadSettingVisible.value = false
}

// 开始上传
async function startUpload() {
  if (pendingFilesWithPath.value.length === 0) return

  // 验证路径
  for (const item of pendingFilesWithPath.value) {
    if (!item.targetPath.trim()) {
      ElMessage.warning('请为所有文件指定目标路径')
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
      ElMessage.error(`${targetPath} 上传失败: ` + e.message)
    }
  }

  await loadObjects()

  const hasError = uploadFiles.value.some(f => f.status === 'exception')
  const successCount = uploadFiles.value.filter(f => f.status === 'success').length

  if (successCount > 0) {
    ElMessage.success(`成功上传 ${successCount} 个文件`)
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
  return new Date(dateStr).toLocaleString('zh-CN')
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
  align-items: center;
  gap: 12px;
}

.search-input {
  width: 220px;
}

.search-info {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
  padding: 8px 12px;
  background: #f0f9eb;
  border-radius: 4px;
}

.breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
}

.back-link {
  color: #409EFF;
  text-decoration: none;
}

.back-link:hover {
  text-decoration: underline;
}

.prefix {
  color: #909399;
}

.object-name {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pagination {
  margin-top: 20px;
  text-align: center;
}

.upload-item {
  margin-bottom: 12px;
}

.upload-file-path {
  display: block;
  margin-bottom: 4px;
  font-size: 13px;
  color: #606266;
  word-break: break-all;
}

.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

/* GitHub风格上传 */
.upload-github-style {
  max-height: 400px;
  overflow-y: auto;
}

.upload-tip {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 16px;
  padding: 10px 12px;
  background: #f0f9eb;
  color: #67c23a;
  border-radius: 4px;
  font-size: 13px;
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
  background: #f5f7fa;
  border-radius: 6px;
  position: relative;
}

.file-item .el-button {
  position: absolute;
  top: 8px;
  right: 8px;
}

.file-info {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #606266;
}

.file-original-name {
  font-weight: 500;
}

.file-size {
  color: #909399;
  font-size: 12px;
}

.file-path-input {
  width: 100%;
}
</style>
