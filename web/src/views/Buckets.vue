<template>
  <div class="buckets-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>存储桶列表</span>
          <el-button type="primary" @click="showCreateDialog = true">
            <el-icon><Plus /></el-icon>
            创建存储桶
          </el-button>
        </div>
      </template>

      <el-table :data="buckets" v-loading="loading" stripe>
        <el-table-column prop="Name" label="名称">
          <template #default="{ row }">
            <router-link :to="{ name: 'Objects', params: { name: row.Name } }" class="bucket-link">
              <el-icon><Folder /></el-icon>
              {{ row.Name }}
            </router-link>
          </template>
        </el-table-column>
        <el-table-column prop="CreationDate" label="创建时间" width="200">
          <template #default="{ row }">
            {{ formatDate(row.CreationDate) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button type="danger" size="small" @click="handleDelete(row.Name)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="showCreateDialog" title="创建存储桶" width="400px">
      <el-form :model="createForm" label-width="80px">
        <el-form-item label="名称">
          <el-input v-model="createForm.name" placeholder="bucket-name" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="handleCreate" :loading="creating">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listBuckets, createBucket, deleteBucket, type Bucket } from '../api/s3'

const buckets = ref<Bucket[]>([])
const loading = ref(false)
const showCreateDialog = ref(false)
const creating = ref(false)
const createForm = reactive({ name: '' })

onMounted(() => loadBuckets())

async function loadBuckets() {
  loading.value = true
  try {
    buckets.value = await listBuckets()
  } catch (e: any) {
    ElMessage.error('加载失败: ' + e.message)
  } finally {
    loading.value = false
  }
}

async function handleCreate() {
  if (!createForm.name.trim()) {
    ElMessage.warning('请输入存储桶名称')
    return
  }
  creating.value = true
  try {
    await createBucket(createForm.name.trim())
    ElMessage.success('创建成功')
    showCreateDialog.value = false
    createForm.name = ''
    await loadBuckets()
  } catch (e: any) {
    ElMessage.error('创建失败: ' + e.message)
  } finally {
    creating.value = false
  }
}

async function handleDelete(name: string) {
  try {
    await ElMessageBox.confirm(`确定要删除存储桶 "${name}" 吗？`, '确认删除', { type: 'warning' })
    await deleteBucket(name)
    ElMessage.success('删除成功')
    await loadBuckets()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error('删除失败: ' + e.message)
    }
  }
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
