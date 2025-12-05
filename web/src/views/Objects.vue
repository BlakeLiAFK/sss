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
        <el-table-column label="操作" width="200">
          <template #default="{ row }">
            <el-button size="small" @click="handleDownload(row.Key)">下载</el-button>
            <el-button type="danger" size="small" @click="handleDelete(row.Key)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination" v-if="isTruncated">
        <el-button @click="loadMore">加载更多</el-button>
      </div>
    </el-card>

    <!-- 上传进度 -->
    <el-dialog v-model="uploadDialogVisible" title="上传进度" :close-on-click-modal="false" width="400px">
      <div v-for="(file, index) in uploadFiles" :key="index" class="upload-item">
        <span>{{ file.name }}</span>
        <el-progress :percentage="file.progress" />
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listObjects, deleteObject, getObjectUrl, uploadObject, type S3Object } from '../api/s3'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const auth = useAuthStore()

const bucketName = computed(() => route.params.name as string)
const prefix = ref('')
const objects = ref<S3Object[]>([])
const loading = ref(false)
const isTruncated = ref(false)
const nextMarker = ref('')

const uploadDialogVisible = ref(false)
const uploadFiles = ref<{ name: string; progress: number }[]>([])

onMounted(() => loadObjects())

watch(() => route.params.name, () => {
  objects.value = []
  prefix.value = ''
  loadObjects()
})

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

function handleDownload(key: string) {
  // 直接打开下载链接（需要预签名URL才能真正工作，这里简化处理）
  const url = getObjectUrl(bucketName.value, key)
  window.open(url, '_blank')
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

async function handleUpload(file: File) {
  uploadFiles.value.push({ name: file.name, progress: 0 })
  uploadDialogVisible.value = true
  const index = uploadFiles.value.length - 1

  try {
    await uploadObject(bucketName.value, file.name, file, (percent) => {
      uploadFiles.value[index].progress = percent
    })
    ElMessage.success(`${file.name} 上传成功`)
    await loadObjects()
  } catch (e: any) {
    ElMessage.error(`${file.name} 上传失败: ` + e.message)
  }

  // 所有文件上传完成后关闭对话框
  if (uploadFiles.value.every(f => f.progress === 100)) {
    setTimeout(() => {
      uploadDialogVisible.value = false
      uploadFiles.value = []
    }, 1000)
  }

  return false // 阻止默认上传
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
</style>
