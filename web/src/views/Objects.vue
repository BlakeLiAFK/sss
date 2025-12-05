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
      </template>

      <el-table :data="objects" v-loading="loading" stripe @row-dblclick="handleRowClick">
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
        <el-table-column label="操作" width="260">
          <template #default="{ row }">
            <el-button size="small" @click="handleDownload(row.Key)">下载</el-button>
            <el-button size="small" @click="handleCopyLink(row.Key)">复制链接</el-button>
            <el-button type="danger" size="small" @click="handleDelete(row.Key)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination" v-if="isTruncated">
        <el-button @click="loadMore">加载更多</el-button>
      </div>
    </el-card>

    <!-- 上传设置对话框 -->
    <el-dialog v-model="uploadSettingVisible" title="上传设置" width="450px">
      <el-form label-width="100px">
        <el-form-item label="目标路径">
          <el-input
            v-model="uploadPath"
            placeholder="可选，例如: images/2024/"
          >
            <template #prepend>/</template>
          </el-input>
          <div class="form-tip">留空则上传到桶根目录，路径末尾可不带斜杠</div>
        </el-form-item>
        <el-form-item label="待上传文件">
          <div class="pending-files">
            <el-tag v-for="(file, index) in pendingFiles" :key="index" closable @close="removePendingFile(index)">
              {{ file.name }}
            </el-tag>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="cancelUpload">取消</el-button>
        <el-button type="primary" @click="startUpload" :disabled="pendingFiles.length === 0">开始上传</el-button>
      </template>
    </el-dialog>

    <!-- 上传进度 -->
    <el-dialog v-model="uploadDialogVisible" title="上传进度" :close-on-click-modal="false" width="400px">
      <div v-for="(file, index) in uploadFiles" :key="index" class="upload-item">
        <span class="upload-file-path">{{ file.path }}</span>
        <el-progress :percentage="file.progress" :status="file.status" />
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listObjects, deleteObject, getObjectUrl, uploadObject, generatePresignedUrl, getBucketPublic, type S3Object } from '../api/s3'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const auth = useAuthStore()

const bucketName = computed(() => route.params.name as string)
const prefix = ref('')
const objects = ref<S3Object[]>([])
const loading = ref(false)
const isTruncated = ref(false)
const nextMarker = ref('')
const isPublic = ref(false) // 桶是否公有

const uploadDialogVisible = ref(false)
const uploadFiles = ref<{ path: string; progress: number; status?: 'success' | 'exception' }[]>([])

// 上传设置相关
const uploadSettingVisible = ref(false)
const uploadPath = ref('')
const pendingFiles = ref<File[]>([])

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

// 加载桶的公有状态
async function loadBucketPublicStatus() {
  try {
    isPublic.value = await getBucketPublic(bucketName.value)
  } catch (e) {
    isPublic.value = false
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
      // 公有桶直接使用URL
      url = getObjectUrl(bucketName.value, key)
    } else {
      // 私有桶使用预签名URL
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

// 复制文件链接到剪贴板
async function handleCopyLink(key: string) {
  try {
    let url: string
    if (isPublic.value) {
      // 公有桶直接使用URL
      url = getObjectUrl(bucketName.value, key)
    } else {
      // 私有桶生成预签名URL（1小时有效）
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

// 收集待上传文件，显示设置对话框
function handleUpload(file: File) {
  pendingFiles.value.push(file)
  uploadSettingVisible.value = true
  return false // 阻止默认上传
}

// 从待上传列表移除文件
function removePendingFile(index: number) {
  pendingFiles.value.splice(index, 1)
  // 如果没有待上传文件了，关闭对话框
  if (pendingFiles.value.length === 0) {
    uploadSettingVisible.value = false
  }
}

// 取消上传
function cancelUpload() {
  pendingFiles.value = []
  uploadPath.value = ''
  uploadSettingVisible.value = false
}

// 开始上传
async function startUpload() {
  if (pendingFiles.value.length === 0) return

  // 关闭设置对话框，打开进度对话框
  uploadSettingVisible.value = false
  uploadDialogVisible.value = true

  // 处理路径前缀
  let pathPrefix = uploadPath.value.trim()
  if (pathPrefix && !pathPrefix.endsWith('/')) {
    pathPrefix += '/'
  }

  // 初始化上传文件列表
  const filesToUpload = [...pendingFiles.value]
  uploadFiles.value = filesToUpload.map(f => ({
    path: pathPrefix + f.name,
    progress: 0
  }))

  // 清空待上传列表
  pendingFiles.value = []
  uploadPath.value = ''

  // 逐个上传文件
  for (let i = 0; i < filesToUpload.length; i++) {
    const file = filesToUpload[i]
    const fullPath = uploadFiles.value[i].path

    try {
      await uploadObject(bucketName.value, fullPath, file, (percent) => {
        uploadFiles.value[i].progress = percent
      })
      uploadFiles.value[i].status = 'success'
    } catch (e: any) {
      uploadFiles.value[i].status = 'exception'
      ElMessage.error(`${fullPath} 上传失败: ` + e.message)
    }
  }

  // 刷新对象列表
  await loadObjects()

  // 上传完成后延迟关闭对话框
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

.pending-files {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
