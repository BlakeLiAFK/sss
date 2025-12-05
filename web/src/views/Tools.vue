<template>
  <div class="tools-page">
    <el-row :gutter="20">
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>预签名URL生成器</span>
          </template>

          <el-form :model="presignForm" label-width="100px">
            <el-form-item label="存储桶">
              <el-select v-model="presignForm.bucket" placeholder="选择存储桶" @change="loadBucketObjects">
                <el-option v-for="b in buckets" :key="b.Name" :label="b.Name" :value="b.Name" />
              </el-select>
            </el-form-item>
            <el-form-item label="对象">
              <el-select v-model="presignForm.key" placeholder="选择对象" filterable>
                <el-option v-for="o in bucketObjects" :key="o.Key" :label="o.Key" :value="o.Key" />
              </el-select>
            </el-form-item>
            <el-form-item label="有效期">
              <el-select v-model="presignForm.expires">
                <el-option label="1小时" :value="3600" />
                <el-option label="6小时" :value="21600" />
                <el-option label="12小时" :value="43200" />
                <el-option label="24小时" :value="86400" />
                <el-option label="7天" :value="604800" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="generatePresignedUrl">生成URL</el-button>
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
import { listBuckets, listObjects, type Bucket, type S3Object } from '../api/s3'

const auth = useAuthStore()

const buckets = ref<Bucket[]>([])
const bucketObjects = ref<S3Object[]>([])
const presignedUrl = ref('')

const presignForm = reactive({
  bucket: '',
  key: '',
  expires: 3600
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

function generatePresignedUrl() {
  if (!presignForm.bucket || !presignForm.key) {
    ElMessage.warning('请选择存储桶和对象')
    return
  }

  // 生成预签名URL（简化实现）
  const now = new Date()
  const dateStr = now.toISOString().replace(/[:-]|\.\d{3}/g, '').slice(0, 8)
  const amzDate = now.toISOString().replace(/[:-]|\.\d{3}/g, '')

  const credential = `${auth.accessKeyId}/${dateStr}/${auth.region}/s3/aws4_request`
  const params = new URLSearchParams({
    'X-Amz-Algorithm': 'AWS4-HMAC-SHA256',
    'X-Amz-Credential': credential,
    'X-Amz-Date': amzDate,
    'X-Amz-Expires': presignForm.expires.toString(),
    'X-Amz-SignedHeaders': 'host',
    'X-Amz-Signature': 'placeholder' // 真正的签名需要后端计算
  })

  presignedUrl.value = `${auth.endpoint}/${presignForm.bucket}/${presignForm.key}?${params.toString()}`
  ElMessage.info('注意：此为示例URL，实际使用需要服务端生成签名')
}

function copyUrl() {
  navigator.clipboard.writeText(presignedUrl.value)
  ElMessage.success('已复制到剪贴板')
}

function copyAwsConfig() {
  navigator.clipboard.writeText(awsCliConfig.value)
  ElMessage.success('已复制到剪贴板')
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
