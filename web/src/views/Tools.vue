<template>
  <div class="tools-page">
    <el-row :gutter="20">
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>预签名URL生成器</span>
          </template>

          <el-form :model="presignForm" label-width="120px">
            <el-form-item label="存储桶">
              <el-select v-model="presignForm.bucket" placeholder="选择存储桶" @change="loadBucketObjects">
                <el-option v-for="b in buckets" :key="b.Name" :label="b.Name" :value="b.Name" />
              </el-select>
            </el-form-item>

            <el-form-item label="对象路径">
              <el-input v-model="presignForm.key" placeholder="输入或选择对象路径">
                <template #append>
                  <el-select v-model="presignForm.key" placeholder="选择对象" style="width: 200px">
                    <el-option v-for="o in bucketObjects" :key="o.Key" :label="o.Key" :value="o.Key" />
                  </el-select>
                </template>
              </el-input>
            </el-form-item>

            <el-form-item label="HTTP方法">
              <el-select v-model="presignForm.method">
                <el-option label="PUT (上传)" value="PUT" />
                <el-option label="GET (下载)" value="GET" />
                <el-option label="DELETE (删除)" value="DELETE" />
                <el-option label="HEAD (信息)" value="HEAD" />
              </el-select>
            </el-form-item>

            <el-form-item label="有效期">
              <el-select v-model="presignForm.expiresMinutes">
                <el-option label="15分钟" :value="15" />
                <el-option label="1小时" :value="60" />
                <el-option label="6小时" :value="360" />
                <el-option label="12小时" :value="720" />
                <el-option label="24小时" :value="1440" />
                <el-option label="7天" :value="10080" />
              </el-select>
            </el-form-item>

            <el-form-item label="大小限制(MB)">
              <el-input-number
                v-model="presignForm.maxSizeMB"
                :min="0"
                :max="1024"
                placeholder="0=不限制"
                style="width: 200px"
              />
            </el-form-item>

            <el-form-item label="内容类型">
              <el-input v-model="presignForm.contentType" placeholder="例如: image/jpeg" />
            </el-form-item>

            <el-form-item>
              <el-button type="primary" @click="handleGeneratePresignedUrl" :loading="generating">生成URL</el-button>
              <el-button @click="clearForm">清空</el-button>
            </el-form-item>
          </el-form>

          <div v-if="presignedUrl" class="result-box">
            <el-input v-model="presignedUrl" type="textarea" :rows="3" readonly />
            <el-button type="primary" size="small" @click="copyUrl" style="margin-top: 10px">
              复制
            </el-button>
          </div>
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card>
          <template #header>
            <span>服务器信息</span>
          </template>

          <el-descriptions :column="1" border>
            <el-descriptions-item label="Endpoint">
              {{ auth.endpoint }}
            </el-descriptions-item>
            <el-descriptions-item label="Region">
              {{ auth.region }}
            </el-descriptions-item>
            <el-descriptions-item label="Access Key ID">
              {{ auth.accessKeyId }}
            </el-descriptions-item>
            <el-descriptions-item label="存储桶数量">
              {{ buckets.length }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card style="margin-top: 20px">
          <template #header>
            <span>AWS CLI 配置</span>
          </template>

          <el-input
            type="textarea"
            :rows="8"
            readonly
            :value="awsCliConfig"
          />
          <el-button type="primary" size="small" @click="copyAwsConfig" style="margin-top: 10px">
            复制配置
          </el-button>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '../stores/auth'
import { listBuckets, listObjects, generatePresignedUrl, type Bucket, type S3Object } from '../api/s3'

const auth = useAuthStore()

const buckets = ref<Bucket[]>([])
const bucketObjects = ref<S3Object[]>([])
const presignedUrl = ref('')
const generating = ref(false)

const presignForm = reactive({
  bucket: '',
  key: '',
  method: 'PUT',
  expiresMinutes: 60,
  maxSizeMB: 0,
  contentType: ''
})

const awsCliConfig = computed(() => {
  return `# ~/.aws/credentials
[sss]
aws_access_key_id = ${auth.accessKeyId}
aws_secret_access_key = ${auth.secretAccessKey}

# ~/.aws/config
[profile sss]
region = ${auth.region}
output = json

# 使用示例
aws --endpoint-url ${auth.endpoint} --profile sss s3 ls
aws --endpoint-url ${auth.endpoint} --profile sss s3 mb s3://my-bucket
aws --endpoint-url ${auth.endpoint} --profile sss s3 cp file.txt s3://my-bucket/`
})

onMounted(async () => {
  try {
    buckets.value = await listBuckets()
  } catch (e) {
    // ignore
  }
})

async function loadBucketObjects() {
  if (!presignForm.bucket) return
  try {
    const result = await listObjects(presignForm.bucket)
    bucketObjects.value = result.objects
  } catch (e) {
    bucketObjects.value = []
  }
}

async function handleGeneratePresignedUrl() {
  if (!presignForm.bucket || !presignForm.key) {
    ElMessage.warning('请输入存储桶和对象路径')
    return
  }

  generating.value = true
  try {
    // 调用从 api/s3 导入的 generatePresignedUrl 函数
    const result = await generatePresignedUrl({
      bucket: presignForm.bucket,
      key: presignForm.key,
      method: presignForm.method,
      expiresMinutes: presignForm.expiresMinutes,
      maxSizeMB: presignForm.maxSizeMB,
      contentType: presignForm.contentType
    })

    presignedUrl.value = result.url

    // 显示额外信息
    let info = `成功生成${result.method}预签名URL`
    if (presignForm.maxSizeMB > 0) {
      info += `，最大限制${presignForm.maxSizeMB}MB`
    }
    if (presignForm.contentType) {
      info += `，内容类型${presignForm.contentType}`
    }
    ElMessage.success(info)
  } catch (error: any) {
    console.error('生成预签名URL失败:', error)
    ElMessage.error(error.response?.data?.message || '生成预签名URL失败')
  } finally {
    generating.value = false
  }
}

function copyUrl() {
  navigator.clipboard.writeText(presignedUrl.value)
  ElMessage.success('已复制到剪贴板')
}

function copyAwsConfig() {
  navigator.clipboard.writeText(awsCliConfig.value)
  ElMessage.success('已复制到剪贴板')
}

function clearForm() {
  presignForm.bucket = ''
  presignForm.key = ''
  presignForm.method = 'PUT'
  presignForm.expiresMinutes = 60
  presignForm.maxSizeMB = 0
  presignForm.contentType = ''
  presignedUrl.value = ''
  bucketObjects.value = []
}
</script>

<style scoped>
.tools-page {
  max-width: 1200px;
}

.result-box {
  margin-top: 20px;
  padding: 15px;
  background-color: #f5f7fa;
  border-radius: 4px;
}
</style>
