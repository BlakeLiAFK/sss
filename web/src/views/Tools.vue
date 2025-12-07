<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title">
        <h1>工具箱</h1>
        <p class="page-subtitle">开发工具与系统配置</p>
      </div>
    </div>

    <el-row :gutter="24">
      <!-- 垃圾回收工具 -->
      <el-col :span="24">
        <div class="content-card">
          <div class="card-header">
            <h3>Garbage Collection</h3>
            <div class="header-actions">
              <el-button
                type="primary"
                @click="handleScanGC"
                :loading="gcScanning"
                :icon="Search"
              >
                Scan
              </el-button>
              <el-button
                type="danger"
                @click="handleExecuteGC"
                :loading="gcExecuting"
                :disabled="!gcResult || (gcResult.orphan_count === 0 && gcResult.expired_count === 0)"
                :icon="Delete"
              >
                Clean
              </el-button>
            </div>
          </div>
          <div class="card-body">
            <el-form :inline="true" class="gc-form">
              <el-form-item label="Expired Upload Age">
                <el-select v-model="gcMaxAge" style="width: 150px">
                  <el-option label="1 hour" :value="1" />
                  <el-option label="6 hours" :value="6" />
                  <el-option label="24 hours" :value="24" />
                  <el-option label="7 days" :value="168" />
                  <el-option label="30 days" :value="720" />
                </el-select>
              </el-form-item>
            </el-form>

            <!-- GC 结果 -->
            <div v-if="gcResult" class="gc-result">
              <el-row :gutter="24" class="gc-stats">
                <el-col :span="8">
                  <div class="stat-card" :class="{ danger: gcResult.orphan_count > 0 }">
                    <div class="stat-value">{{ gcResult.orphan_count }}</div>
                    <div class="stat-label">Orphan Files</div>
                    <div class="stat-size" v-if="gcResult.orphan_size > 0">{{ formatSize(gcResult.orphan_size) }}</div>
                  </div>
                </el-col>
                <el-col :span="8">
                  <div class="stat-card" :class="{ warning: gcResult.expired_count > 0 }">
                    <div class="stat-value">{{ gcResult.expired_count }}</div>
                    <div class="stat-label">Expired Uploads</div>
                    <div class="stat-size" v-if="gcResult.expired_part_size > 0">{{ formatSize(gcResult.expired_part_size) }}</div>
                  </div>
                </el-col>
                <el-col :span="8">
                  <div class="stat-card" :class="{ success: gcResult.cleaned }">
                    <div class="stat-value">{{ gcResult.cleaned ? 'Yes' : 'No' }}</div>
                    <div class="stat-label">Cleaned</div>
                    <div class="stat-size" v-if="gcResult.cleaned_at">{{ formatTime(gcResult.cleaned_at) }}</div>
                  </div>
                </el-col>
              </el-row>

              <!-- 孤立文件列表 -->
              <div v-if="gcResult.orphan_files && gcResult.orphan_files.length > 0" class="gc-files">
                <h4>Orphan Files ({{ gcResult.orphan_files.length }})</h4>
                <el-table :data="gcResult.orphan_files.slice(0, 20)" size="small" max-height="300">
                  <el-table-column prop="path" label="Path" min-width="300" />
                  <el-table-column label="Size" width="100">
                    <template #default="{ row }">{{ formatSize(row.size) }}</template>
                  </el-table-column>
                  <el-table-column label="Modified" width="180">
                    <template #default="{ row }">{{ formatTime(row.modified_at) }}</template>
                  </el-table-column>
                </el-table>
                <div v-if="gcResult.orphan_files.length > 20" class="more-hint">
                  And {{ gcResult.orphan_files.length - 20 }} more files...
                </div>
              </div>

              <!-- 过期上传列表 -->
              <div v-if="gcResult.expired_uploads && gcResult.expired_uploads.length > 0" class="gc-files">
                <h4>Expired Uploads ({{ gcResult.expired_uploads.length }})</h4>
                <el-table :data="gcResult.expired_uploads.slice(0, 10)" size="small">
                  <el-table-column prop="" label="Upload ID">
                    <template #default="{ row }">{{ row }}</template>
                  </el-table-column>
                </el-table>
                <div v-if="gcResult.expired_uploads.length > 10" class="more-hint">
                  And {{ gcResult.expired_uploads.length - 10 }} more uploads...
                </div>
              </div>

              <!-- 无垃圾 -->
              <div v-if="gcResult.orphan_count === 0 && gcResult.expired_count === 0" class="gc-clean">
                <el-icon :size="48" color="#10b981"><CircleCheck /></el-icon>
                <p>No garbage found. Your storage is clean!</p>
              </div>
            </div>

            <!-- 未扫描提示 -->
            <div v-else class="gc-empty">
              <el-icon :size="48" color="#94a3b8"><Search /></el-icon>
              <p>Click "Scan" to find orphan files and expired uploads</p>
            </div>
          </div>
        </div>
      </el-col>

      <!-- 数据完整性检查 -->
      <el-col :span="24" style="margin-top: 24px">
        <div class="content-card">
          <div class="card-header">
            <h3>Data Integrity Check</h3>
            <div class="header-actions">
              <el-button
                type="primary"
                @click="handleCheckIntegrity"
                :loading="integrityChecking"
                :icon="Search"
              >
                Check
              </el-button>
              <el-button
                type="warning"
                @click="handleRepairIntegrity"
                :loading="integrityRepairing"
                :disabled="!integrityResult || integrityResult.issues_found === 0"
                :icon="Refresh"
              >
                Repair
              </el-button>
            </div>
          </div>
          <div class="card-body">
            <el-form :inline="true" class="gc-form">
              <el-form-item label="Verify ETag">
                <el-switch v-model="integrityVerifyEtag" />
              </el-form-item>
              <el-form-item label="Limit">
                <el-select v-model="integrityLimit" style="width: 150px">
                  <el-option label="100 objects" :value="100" />
                  <el-option label="500 objects" :value="500" />
                  <el-option label="1000 objects" :value="1000" />
                  <el-option label="5000 objects" :value="5000" />
                  <el-option label="All objects" :value="0" />
                </el-select>
              </el-form-item>
            </el-form>

            <!-- 完整性检查结果 -->
            <div v-if="integrityResult" class="gc-result">
              <el-row :gutter="24" class="gc-stats">
                <el-col :span="6">
                  <div class="stat-card">
                    <div class="stat-value">{{ integrityResult.total_checked }}</div>
                    <div class="stat-label">Total Checked</div>
                    <div class="stat-size">{{ integrityResult.duration.toFixed(2) }}s</div>
                  </div>
                </el-col>
                <el-col :span="6">
                  <div class="stat-card" :class="{ danger: integrityResult.missing_files > 0 }">
                    <div class="stat-value">{{ integrityResult.missing_files }}</div>
                    <div class="stat-label">Missing Files</div>
                  </div>
                </el-col>
                <el-col :span="6">
                  <div class="stat-card" :class="{ warning: integrityResult.etag_mismatches > 0 }">
                    <div class="stat-value">{{ integrityResult.etag_mismatches }}</div>
                    <div class="stat-label">ETag Mismatches</div>
                  </div>
                </el-col>
                <el-col :span="6">
                  <div class="stat-card" :class="{ success: integrityResult.repaired }">
                    <div class="stat-value">{{ integrityResult.repaired ? integrityResult.repaired_count : '-' }}</div>
                    <div class="stat-label">Repaired</div>
                  </div>
                </el-col>
              </el-row>

              <!-- 问题列表 -->
              <div v-if="integrityResult.issues && integrityResult.issues.length > 0" class="gc-files">
                <h4>Issues Found ({{ integrityResult.issues.length }})</h4>
                <el-table :data="integrityResult.issues.slice(0, 20)" size="small" max-height="300">
                  <el-table-column prop="bucket" label="Bucket" width="120" />
                  <el-table-column prop="key" label="Key" min-width="200" />
                  <el-table-column label="Issue Type" width="150">
                    <template #default="{ row }">
                      <el-tag :type="getIssueTagType(row.issue_type)" size="small">
                        {{ formatIssueType(row.issue_type) }}
                      </el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="Size" width="100">
                    <template #default="{ row }">{{ formatSize(row.size) }}</template>
                  </el-table-column>
                  <el-table-column label="Repairable" width="100">
                    <template #default="{ row }">
                      <el-tag :type="row.repairable ? 'success' : 'info'" size="small">
                        {{ row.repairable ? 'Yes' : 'No' }}
                      </el-tag>
                    </template>
                  </el-table-column>
                </el-table>
                <div v-if="integrityResult.issues.length > 20" class="more-hint">
                  And {{ integrityResult.issues.length - 20 }} more issues...
                </div>
              </div>

              <!-- 无问题 -->
              <div v-if="integrityResult.issues_found === 0" class="gc-clean">
                <el-icon :size="48" color="#10b981"><CircleCheck /></el-icon>
                <p>No integrity issues found. Your data is healthy!</p>
              </div>
            </div>

            <!-- 未检查提示 -->
            <div v-else class="gc-empty">
              <el-icon :size="48" color="#94a3b8"><Search /></el-icon>
              <p>Click "Check" to verify data integrity</p>
            </div>
          </div>
        </div>
      </el-col>

      <!-- 数据迁移工具 -->
      <el-col :span="24" style="margin-top: 24px">
        <div class="content-card">
          <div class="card-header">
            <h3>Data Migration</h3>
            <div class="header-actions">
              <el-button
                type="primary"
                @click="openMigrateDialog"
                :icon="Link"
              >
                New Migration
              </el-button>
              <el-button
                @click="loadMigrateJobs"
                :loading="migrateLoading"
                :icon="Refresh"
              >
                Refresh
              </el-button>
            </div>
          </div>
          <div class="card-body">
            <!-- 迁移任务列表 -->
            <div v-if="migrateJobs.length > 0" class="migrate-list">
              <el-table :data="migrateJobs" stripe>
                <el-table-column label="Job ID" width="120">
                  <template #default="{ row }">
                    <span class="job-id">{{ row.jobId.substring(0, 8) }}...</span>
                  </template>
                </el-table-column>
                <el-table-column label="Source" min-width="200">
                  <template #default="{ row }">
                    <div class="source-info">
                      <div class="source-endpoint">{{ row.config.sourceEndpoint }}</div>
                      <div class="source-bucket">{{ row.config.sourceBucket }}{{ row.config.sourcePrefix ? '/' + row.config.sourcePrefix : '' }}</div>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column label="Target" width="150">
                  <template #default="{ row }">
                    <span>{{ row.config.targetBucket }}{{ row.config.targetPrefix ? '/' + row.config.targetPrefix : '' }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="Progress" width="180">
                  <template #default="{ row }">
                    <div class="progress-cell">
                      <el-progress
                        :percentage="getMigrateProgress(row)"
                        :status="row.status === 'failed' ? 'exception' : row.status === 'completed' ? 'success' : ''"
                        :stroke-width="8"
                      />
                      <div class="progress-text">
                        {{ row.completed }}/{{ row.totalObjects }}
                        <span v-if="row.skipped > 0">({{ row.skipped }} skipped)</span>
                      </div>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column label="Status" width="110">
                  <template #default="{ row }">
                    <el-tag :type="getMigrateStatusType(row.status)" size="small">
                      {{ row.status }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="Duration" width="100">
                  <template #default="{ row }">
                    <span class="duration">{{ getMigrateDuration(row) }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="Actions" width="120" fixed="right">
                  <template #default="{ row }">
                    <el-button
                      v-if="row.status === 'running' || row.status === 'pending'"
                      type="warning"
                      size="small"
                      @click="handleCancelMigration(row)"
                      :icon="Close"
                    >
                      Cancel
                    </el-button>
                    <el-button
                      v-else
                      type="danger"
                      size="small"
                      @click="handleDeleteMigration(row)"
                      :icon="Delete"
                    >
                      Delete
                    </el-button>
                  </template>
                </el-table-column>
              </el-table>
            </div>

            <!-- 无任务提示 -->
            <div v-else class="gc-empty">
              <el-icon :size="48" color="#94a3b8"><Link /></el-icon>
              <p>No migration jobs. Click "New Migration" to start importing data from another S3 service.</p>
            </div>
          </div>
        </div>
      </el-col>

      <!-- 迁移配置对话框 -->
      <el-dialog v-model="showMigrateDialog" title="Create Migration Job" width="600px">
        <el-form :model="migrateForm" label-position="top">
          <el-divider content-position="left">Source S3 Service</el-divider>

          <el-row :gutter="16">
            <el-col :span="16">
              <el-form-item label="Endpoint URL" required>
                <el-input
                  v-model="migrateForm.sourceEndpoint"
                  placeholder="https://s3.amazonaws.com or MinIO endpoint"
                />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="Region">
                <el-input v-model="migrateForm.sourceRegion" placeholder="us-east-1" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="Access Key" required>
                <el-input v-model="migrateForm.sourceAccessKey" placeholder="Access Key ID" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="Secret Key" required>
                <el-input
                  v-model="migrateForm.sourceSecretKey"
                  type="password"
                  placeholder="Secret Access Key"
                  show-password
                />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="Source Bucket" required>
                <el-input v-model="migrateForm.sourceBucket" placeholder="bucket-name" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="Source Prefix">
                <el-input v-model="migrateForm.sourcePrefix" placeholder="Optional: folder/path/" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-button type="info" size="small" @click="handleTestConnection" :loading="migrateValidating">
            Test Connection
          </el-button>

          <el-divider content-position="left">Target (Local)</el-divider>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="Target Bucket" required>
                <el-select v-model="migrateForm.targetBucket" placeholder="Select local bucket" style="width: 100%">
                  <el-option v-for="b in buckets" :key="b.Name" :label="b.Name" :value="b.Name" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="Target Prefix">
                <el-input v-model="migrateForm.targetPrefix" placeholder="Optional: folder/path/" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item>
            <el-checkbox v-model="migrateForm.overwriteExist">
              Overwrite existing files
            </el-checkbox>
          </el-form-item>
        </el-form>

        <template #footer>
          <el-button @click="showMigrateDialog = false">Cancel</el-button>
          <el-button type="primary" @click="handleCreateMigration" :loading="migrateCreating">
            Start Migration
          </el-button>
        </template>
      </el-dialog>

      <!-- 预签名 URL 生成器 -->
      <el-col :span="12" style="margin-top: 24px">
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

      <el-col :span="12" style="margin-top: 24px">
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
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CopyDocument, Search, Delete, CircleCheck, Refresh, Link, Close, Timer, Check, Warning } from '@element-plus/icons-vue'
import { useAuthStore } from '../stores/auth'
import { listBuckets, listObjects, generatePresignedUrl, type Bucket, type S3Object } from '../api/s3'
import {
  scanGC, executeGC, type GCResult,
  checkIntegrity, repairIntegrity, type IntegrityResult,
  listMigrateJobs, createMigrateJob, getMigrateProgress, cancelMigrateJob, deleteMigrateJob, validateMigrateConfig,
  type MigrateConfig, type MigrateProgress
} from '../api/admin'

const auth = useAuthStore()

const buckets = ref<Bucket[]>([])
const bucketObjects = ref<S3Object[]>([])
const presignedUrl = ref('')
const generating = ref(false)

// GC 状态
const gcScanning = ref(false)
const gcExecuting = ref(false)
const gcMaxAge = ref(24)
const gcResult = ref<GCResult | null>(null)

// 完整性检查状态
const integrityChecking = ref(false)
const integrityRepairing = ref(false)
const integrityVerifyEtag = ref(false)
const integrityLimit = ref(1000)
const integrityResult = ref<IntegrityResult | null>(null)

// 迁移状态
const migrateJobs = ref<MigrateProgress[]>([])
const migrateLoading = ref(false)
const migrateCreating = ref(false)
const migrateValidating = ref(false)
const showMigrateDialog = ref(false)
const migratePollingTimer = ref<number | null>(null)
const migrateForm = reactive<MigrateConfig>({
  sourceEndpoint: '',
  sourceAccessKey: '',
  sourceSecretKey: '',
  sourceBucket: '',
  sourcePrefix: '',
  sourceRegion: 'us-east-1',
  targetBucket: '',
  targetPrefix: '',
  overwriteExist: false
})

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
  // 加载迁移任务
  loadMigrateJobs()
  // 启动轮询
  startMigratePolling()
})

onUnmounted(() => {
  stopMigratePolling()
})

// GC 扫描
async function handleScanGC() {
  gcScanning.value = true
  try {
    gcResult.value = await scanGC(gcMaxAge.value)
    if (gcResult.value.orphan_count === 0 && gcResult.value.expired_count === 0) {
      ElMessage.success('No garbage found')
    } else {
      ElMessage.info(`Found ${gcResult.value.orphan_count} orphan files and ${gcResult.value.expired_count} expired uploads`)
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || 'Failed to scan garbage')
  } finally {
    gcScanning.value = false
  }
}

// GC 执行
async function handleExecuteGC() {
  if (!gcResult.value) return

  try {
    await ElMessageBox.confirm(
      `This will permanently delete ${gcResult.value.orphan_count} orphan files (${formatSize(gcResult.value.orphan_size)}) and ${gcResult.value.expired_count} expired uploads. Continue?`,
      'Confirm Garbage Collection',
      {
        confirmButtonText: 'Clean',
        cancelButtonText: 'Cancel',
        type: 'warning'
      }
    )

    gcExecuting.value = true
    gcResult.value = await executeGC(gcMaxAge.value, false)
    ElMessage.success('Garbage collection completed')
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || 'Failed to clean garbage')
    }
  } finally {
    gcExecuting.value = false
  }
}

// 完整性检查
async function handleCheckIntegrity() {
  integrityChecking.value = true
  try {
    integrityResult.value = await checkIntegrity(integrityVerifyEtag.value, integrityLimit.value)
    if (integrityResult.value.issues_found === 0) {
      ElMessage.success('No integrity issues found')
    } else {
      ElMessage.warning(`Found ${integrityResult.value.issues_found} integrity issues`)
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || 'Failed to check integrity')
  } finally {
    integrityChecking.value = false
  }
}

// 修复完整性问题
async function handleRepairIntegrity() {
  if (!integrityResult.value || integrityResult.value.issues_found === 0) return

  try {
    await ElMessageBox.confirm(
      `This will repair ${integrityResult.value.issues_found} issues. Missing files will have their metadata deleted, and ETag mismatches will be recalculated. Continue?`,
      'Confirm Repair',
      {
        confirmButtonText: 'Repair',
        cancelButtonText: 'Cancel',
        type: 'warning'
      }
    )

    integrityRepairing.value = true
    integrityResult.value = await repairIntegrity(
      integrityResult.value.issues,
      integrityVerifyEtag.value,
      integrityLimit.value
    )
    ElMessage.success(`Repaired ${integrityResult.value.repaired_count} issues`)
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || 'Failed to repair integrity issues')
    }
  } finally {
    integrityRepairing.value = false
  }
}

// 获取问题类型标签颜色
function getIssueTagType(issueType: string): string {
  switch (issueType) {
    case 'missing_file':
      return 'danger'
    case 'etag_mismatch':
      return 'warning'
    case 'path_mismatch':
      return 'info'
    default:
      return 'info'
  }
}

// 格式化问题类型
function formatIssueType(issueType: string): string {
  switch (issueType) {
    case 'missing_file':
      return 'Missing File'
    case 'etag_mismatch':
      return 'ETag Mismatch'
    case 'path_mismatch':
      return 'Path Mismatch'
    default:
      return issueType
  }
}

// 格式化大小
function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// 格式化时间
function formatTime(timeStr: string): string {
  if (!timeStr) return '-'
  const date = new Date(timeStr)
  return date.toLocaleString()
}

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

// ========== 迁移功能 ==========

// 加载迁移任务列表
async function loadMigrateJobs() {
  migrateLoading.value = true
  try {
    migrateJobs.value = await listMigrateJobs()
  } catch (error: any) {
    console.error('Failed to load migrate jobs:', error)
  } finally {
    migrateLoading.value = false
  }
}

// 启动轮询
function startMigratePolling() {
  migratePollingTimer.value = window.setInterval(() => {
    // 只有当有进行中的任务时才轮询
    const hasRunning = migrateJobs.value.some(j => j.status === 'pending' || j.status === 'running')
    if (hasRunning) {
      loadMigrateJobs()
    }
  }, 2000)
}

// 停止轮询
function stopMigratePolling() {
  if (migratePollingTimer.value) {
    clearInterval(migratePollingTimer.value)
    migratePollingTimer.value = null
  }
}

// 打开创建迁移对话框
function openMigrateDialog() {
  resetMigrateForm()
  showMigrateDialog.value = true
}

// 重置迁移表单
function resetMigrateForm() {
  migrateForm.sourceEndpoint = ''
  migrateForm.sourceAccessKey = ''
  migrateForm.sourceSecretKey = ''
  migrateForm.sourceBucket = ''
  migrateForm.sourcePrefix = ''
  migrateForm.sourceRegion = 'us-east-1'
  migrateForm.targetBucket = ''
  migrateForm.targetPrefix = ''
  migrateForm.overwriteExist = false
}

// 测试连接
async function handleTestConnection() {
  if (!migrateForm.sourceEndpoint || !migrateForm.sourceAccessKey || !migrateForm.sourceSecretKey || !migrateForm.sourceBucket) {
    ElMessage.warning('Please fill in all required source fields')
    return
  }

  migrateValidating.value = true
  try {
    const result = await validateMigrateConfig(migrateForm)
    if (result.valid) {
      ElMessage.success('Connection successful!')
    } else {
      ElMessage.error(result.message || 'Connection failed')
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || 'Connection test failed')
  } finally {
    migrateValidating.value = false
  }
}

// 创建迁移任务
async function handleCreateMigration() {
  if (!migrateForm.sourceEndpoint || !migrateForm.sourceAccessKey || !migrateForm.sourceSecretKey || !migrateForm.sourceBucket) {
    ElMessage.warning('Please fill in all required source fields')
    return
  }
  if (!migrateForm.targetBucket) {
    ElMessage.warning('Please select a target bucket')
    return
  }

  migrateCreating.value = true
  try {
    const result = await createMigrateJob(migrateForm)
    ElMessage.success(`Migration job created: ${result.jobId.substring(0, 8)}...`)
    showMigrateDialog.value = false
    loadMigrateJobs()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || 'Failed to create migration job')
  } finally {
    migrateCreating.value = false
  }
}

// 取消迁移任务
async function handleCancelMigration(job: MigrateProgress) {
  try {
    await ElMessageBox.confirm(
      `Cancel migration job ${job.jobId.substring(0, 8)}...? This will stop the transfer.`,
      'Confirm Cancel',
      {
        confirmButtonText: 'Cancel Job',
        cancelButtonText: 'Keep Running',
        type: 'warning'
      }
    )

    await cancelMigrateJob(job.jobId)
    ElMessage.success('Migration cancelled')
    loadMigrateJobs()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || 'Failed to cancel migration')
    }
  }
}

// 删除迁移任务记录
async function handleDeleteMigration(job: MigrateProgress) {
  try {
    await ElMessageBox.confirm(
      `Delete migration record ${job.jobId.substring(0, 8)}...?`,
      'Confirm Delete',
      {
        confirmButtonText: 'Delete',
        cancelButtonText: 'Cancel',
        type: 'warning'
      }
    )

    await deleteMigrateJob(job.jobId)
    ElMessage.success('Migration record deleted')
    loadMigrateJobs()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || 'Failed to delete migration record')
    }
  }
}

// 获取任务状态标签类型
function getMigrateStatusType(status: string): string {
  switch (status) {
    case 'completed': return 'success'
    case 'running': return 'primary'
    case 'pending': return 'info'
    case 'failed': return 'danger'
    case 'cancelled': return 'warning'
    default: return 'info'
  }
}

// 计算进度百分比
function getMigrateProgress(job: MigrateProgress): number {
  if (job.totalObjects === 0) return 0
  return Math.round((job.completed / job.totalObjects) * 100)
}

// 计算已用时间
function getMigrateDuration(job: MigrateProgress): string {
  const start = new Date(job.startTime).getTime()
  const end = job.endTime ? new Date(job.endTime).getTime() : Date.now()
  const seconds = Math.floor((end - start) / 1000)

  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
}
</script>

<style scoped>
.page-container {
  max-width: 1200px;
}

.page-header {
  margin-bottom: 20px;
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

.content-card {
  background: #fff;
  border-radius: 10px;
  border: 1px solid #eee;
  overflow: hidden;
  margin-bottom: 16px;
}

@media (max-width: 768px) {
  .header-actions {
    flex-direction: column;
  }
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  border-bottom: 1px solid #f1f5f9;
}

.card-header h3 {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.card-body {
  padding: 24px;
}

.gc-form {
  margin-bottom: 20px;
}

.gc-stats {
  margin-bottom: 24px;
}

.stat-card {
  background: #f8fafc;
  border-radius: 8px;
  padding: 20px;
  text-align: center;
}

.stat-card.danger {
  background: #fef2f2;
}

.stat-card.warning {
  background: #fffbeb;
}

.stat-card.success {
  background: #f0fdf4;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: #1e293b;
}

.stat-card.danger .stat-value {
  color: #dc2626;
}

.stat-card.warning .stat-value {
  color: #d97706;
}

.stat-card.success .stat-value {
  color: #16a34a;
}

.stat-label {
  font-size: 13px;
  color: #64748b;
  margin-top: 4px;
}

.stat-size {
  font-size: 12px;
  color: #94a3b8;
  margin-top: 4px;
}

.gc-files {
  margin-top: 20px;
}

.gc-files h4 {
  font-size: 14px;
  font-weight: 600;
  color: #475569;
  margin: 0 0 12px;
}

.more-hint {
  font-size: 12px;
  color: #94a3b8;
  text-align: center;
  padding: 8px;
}

.gc-clean, .gc-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px;
  color: #64748b;
}

.gc-clean p, .gc-empty p {
  margin: 12px 0 0;
  font-size: 14px;
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

/* 迁移工具样式 */
.migrate-list {
  margin-top: 8px;
}

.job-id {
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
  font-size: 12px;
  color: #64748b;
}

.source-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.source-endpoint {
  font-size: 12px;
  color: #64748b;
  word-break: break-all;
}

.source-bucket {
  font-size: 13px;
  font-weight: 500;
  color: #1e293b;
}

.progress-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.progress-text {
  font-size: 11px;
  color: #64748b;
  text-align: center;
}

.duration {
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
  font-size: 12px;
  color: #64748b;
}
</style>
