<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title">
        <h1>Tools</h1>
        <p class="page-subtitle">Developer utilities and configuration</p>
      </div>
    </div>

    <el-row :gutter="24">
      <el-col :span="12">
        <div class="content-card">
          <div class="card-header">
            <h3>Presigned URL Generator</h3>
          </div>
          <div class="card-body">
            <el-form :model="presignForm" label-position="top">
              <el-form-item label="Bucket">
                <el-select
                  v-model="presignForm.bucket"
                  placeholder="Select bucket"
                  @change="loadBucketObjects"
                  style="width: 100%"
                >
                  <el-option v-for="b in buckets" :key="b.Name" :label="b.Name" :value="b.Name" />
                </el-select>
              </el-form-item>

              <el-form-item label="Object Path">
                <el-input v-model="presignForm.key" placeholder="Enter or select object path">
                  <template #append>
                    <el-select
                      v-model="presignForm.key"
                      placeholder="Select"
                      style="width: 160px"
                      :disabled="!presignForm.bucket"
                    >
                      <el-option v-for="o in bucketObjects" :key="o.Key" :label="o.Key" :value="o.Key" />
                    </el-select>
                  </template>
                </el-input>
              </el-form-item>

              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="HTTP Method">
                    <el-select v-model="presignForm.method" style="width: 100%">
                      <el-option label="PUT (Upload)" value="PUT" />
                      <el-option label="GET (Download)" value="GET" />
                      <el-option label="DELETE (Delete)" value="DELETE" />
                      <el-option label="HEAD (Info)" value="HEAD" />
                    </el-select>
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="Expiration">
                    <el-select v-model="presignForm.expiresMinutes" style="width: 100%">
                      <el-option label="15 minutes" :value="15" />
                      <el-option label="1 hour" :value="60" />
                      <el-option label="6 hours" :value="360" />
                      <el-option label="12 hours" :value="720" />
                      <el-option label="24 hours" :value="1440" />
                      <el-option label="7 days" :value="10080" />
                    </el-select>
                  </el-form-item>
                </el-col>
              </el-row>

              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="Max Size (MB)">
                    <el-input-number
                      v-model="presignForm.maxSizeMB"
                      :min="0"
                      :max="1024"
                      placeholder="0 = No limit"
                      style="width: 100%"
                    />
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="Content Type">
                    <el-input v-model="presignForm.contentType" placeholder="e.g., image/jpeg" />
                  </el-form-item>
                </el-col>
              </el-row>

              <el-form-item>
                <el-button type="primary" @click="handleGeneratePresignedUrl" :loading="generating">
                  Generate URL
                </el-button>
                <el-button @click="clearForm">Clear</el-button>
              </el-form-item>
            </el-form>

            <div v-if="presignedUrl" class="result-box">
              <label>Generated URL</label>
              <el-input v-model="presignedUrl" type="textarea" :rows="3" readonly />
              <el-button type="primary" size="small" @click="copyUrl" style="margin-top: 12px">
                <el-icon><CopyDocument /></el-icon>
                Copy URL
              </el-button>
            </div>
          </div>
        </div>
      </el-col>

      <el-col :span="12">
        <div class="content-card">
          <div class="card-header">
            <h3>Server Information</h3>
          </div>
          <div class="card-body">
            <div class="info-grid">
              <div class="info-item">
                <label>Endpoint</label>
                <span>{{ auth.endpoint }}</span>
              </div>
              <div class="info-item">
                <label>Region</label>
                <span>{{ auth.region }}</span>
              </div>
              <div class="info-item">
                <label>Buckets</label>
                <span>{{ buckets.length }}</span>
              </div>
            </div>
          </div>
        </div>

        <div class="content-card" style="margin-top: 24px">
          <div class="card-header">
            <h3>AWS CLI Configuration</h3>
          </div>
          <div class="card-body">
            <div class="code-block">
              <pre>{{ awsCliConfig }}</pre>
            </div>
            <el-button type="primary" size="small" @click="copyAwsConfig" style="margin-top: 12px">
              <el-icon><CopyDocument /></el-icon>
              Copy Configuration
            </el-button>
          </div>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { CopyDocument } from '@element-plus/icons-vue'
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
aws_access_key_id = YOUR_ACCESS_KEY_ID
aws_secret_access_key = YOUR_SECRET_ACCESS_KEY

# ~/.aws/config
[profile sss]
region = ${auth.region}
output = json

# Usage examples
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
    ElMessage.warning('Please select bucket and object path')
    return
  }

  generating.value = true
  try {
    const result = await generatePresignedUrl({
      bucket: presignForm.bucket,
      key: presignForm.key,
      method: presignForm.method,
      expiresMinutes: presignForm.expiresMinutes,
      maxSizeMB: presignForm.maxSizeMB,
      contentType: presignForm.contentType
    })

    presignedUrl.value = result.url

    let info = `Generated ${result.method} presigned URL`
    if (presignForm.maxSizeMB > 0) {
      info += `, max ${presignForm.maxSizeMB}MB`
    }
    if (presignForm.contentType) {
      info += `, type: ${presignForm.contentType}`
    }
    ElMessage.success(info)
  } catch (error: any) {
    console.error('Failed to generate presigned URL:', error)
    ElMessage.error(error.response?.data?.message || 'Failed to generate presigned URL')
  } finally {
    generating.value = false
  }
}

function copyUrl() {
  navigator.clipboard.writeText(presignedUrl.value)
  ElMessage.success('Copied to clipboard')
}

function copyAwsConfig() {
  navigator.clipboard.writeText(awsCliConfig.value)
  ElMessage.success('Copied to clipboard')
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
.page-container {
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 24px;
}

.page-title h1 {
  font-size: 24px;
  font-weight: 600;
  color: #1e293b;
  margin: 0;
}

.page-subtitle {
  font-size: 14px;
  color: #64748b;
  margin: 4px 0 0;
}

.content-card {
  background: #ffffff;
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
  overflow: hidden;
}

.card-header {
  padding: 16px 24px;
  border-bottom: 1px solid #f1f5f9;
}

.card-header h3 {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
  margin: 0;
}

.card-body {
  padding: 24px;
}

.result-box {
  margin-top: 20px;
  padding: 16px;
  background: #f8fafc;
  border-radius: 8px;
}

.result-box label {
  display: block;
  font-size: 12px;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 8px;
}

.info-grid {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.info-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid #f1f5f9;
}

.info-item:last-child {
  border-bottom: none;
}

.info-item label {
  font-size: 14px;
  color: #64748b;
}

.info-item span {
  font-size: 14px;
  font-weight: 500;
  color: #1e293b;
}

.code-block {
  background: #1e293b;
  border-radius: 8px;
  padding: 16px;
  overflow-x: auto;
}

.code-block pre {
  margin: 0;
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
  font-size: 13px;
  color: #e2e8f0;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
