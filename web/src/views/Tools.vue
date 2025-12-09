<template>
  <div class="page-container">
    <div class="page-header">
      <div class="page-title">
        <h1>{{ t('tools.title') }}</h1>
        <p class="page-subtitle">{{ t('tools.subtitle') }}</p>
      </div>
    </div>

    <el-row :gutter="24">
      <!-- 垃圾回收工具 -->
      <el-col :span="24">
        <div class="content-card">
          <div class="card-header">
            <h3>{{ t('tools.garbageCollection') }}</h3>
            <div class="header-actions">
              <el-button
                type="primary"
                class="primary-btn"
                @click="handleScanGC"
                :loading="gcScanning"
                :icon="Search"
              >
                {{ t('tools.scan') }}
              </el-button>
              <el-button
                type="danger"
                @click="handleExecuteGC"
                :loading="gcExecuting"
                :disabled="!gcResult || (gcResult.orphan_count === 0 && gcResult.expired_count === 0)"
                :icon="Delete"
              >
                {{ t('tools.clean') }}
              </el-button>
            </div>
          </div>
          <div class="card-body">
            <el-form :inline="true" class="gc-form">
              <el-form-item :label="t('tools.expiredUploadAge')">
                <el-select v-model="gcMaxAge" style="width: 150px">
                  <el-option :label="t('tools.hours', { count: 1 })" :value="1" />
                  <el-option :label="t('tools.hours', { count: 6 })" :value="6" />
                  <el-option :label="t('tools.hours', { count: 24 })" :value="24" />
                  <el-option :label="t('tools.days', { count: 7 })" :value="168" />
                  <el-option :label="t('tools.days', { count: 30 })" :value="720" />
                </el-select>
              </el-form-item>
            </el-form>

            <!-- GC 结果 -->
            <div v-if="gcResult" class="gc-result">
              <el-row :gutter="24" class="gc-stats">
                <el-col :span="8">
                  <div class="stat-card" :class="{ danger: gcResult.orphan_count > 0 }">
                    <div class="stat-value">{{ gcResult.orphan_count }}</div>
                    <div class="stat-label">{{ t('tools.orphanFiles') }}</div>
                    <div class="stat-size" v-if="gcResult.orphan_size > 0">{{ formatSize(gcResult.orphan_size) }}</div>
                  </div>
                </el-col>
                <el-col :span="8">
                  <div class="stat-card" :class="{ warning: gcResult.expired_count > 0 }">
                    <div class="stat-value">{{ gcResult.expired_count }}</div>
                    <div class="stat-label">{{ t('tools.expiredUploads') }}</div>
                    <div class="stat-size" v-if="gcResult.expired_part_size > 0">{{ formatSize(gcResult.expired_part_size) }}</div>
                  </div>
                </el-col>
                <el-col :span="8">
                  <div class="stat-card" :class="{ success: gcResult.cleaned }">
                    <div class="stat-value">{{ gcResult.cleaned ? t('tools.yes') : t('tools.no') }}</div>
                    <div class="stat-label">{{ t('tools.cleaned') }}</div>
                    <div class="stat-size" v-if="gcResult.cleaned_at">{{ formatTime(gcResult.cleaned_at) }}</div>
                  </div>
                </el-col>
              </el-row>

              <!-- 孤立文件列表 -->
              <div v-if="gcResult.orphan_files && gcResult.orphan_files.length > 0" class="gc-files">
                <h4>{{ t('tools.orphanFilesCount', { count: gcResult.orphan_files.length }) }}</h4>
                <el-table :data="gcResult.orphan_files.slice(0, 20)" size="small" max-height="300">
                  <el-table-column prop="path" :label="t('tools.path')" min-width="300" />
                  <el-table-column :label="t('tools.size')" width="100">
                    <template #default="{ row }">{{ formatSize(row.size) }}</template>
                  </el-table-column>
                  <el-table-column :label="t('tools.modified')" width="180">
                    <template #default="{ row }">{{ formatTime(row.modified_at) }}</template>
                  </el-table-column>
                </el-table>
                <div v-if="gcResult.orphan_files.length > 20" class="more-hint">
                  {{ t('tools.andMoreFiles', { count: gcResult.orphan_files.length - 20 }) }}
                </div>
              </div>

              <!-- 过期上传列表 -->
              <div v-if="gcResult.expired_uploads && gcResult.expired_uploads.length > 0" class="gc-files">
                <h4>{{ t('tools.expiredUploadsCount', { count: gcResult.expired_uploads.length }) }}</h4>
                <el-table :data="gcResult.expired_uploads.slice(0, 10)" size="small">
                  <el-table-column prop="" :label="t('tools.uploadId')">
                    <template #default="{ row }">{{ row }}</template>
                  </el-table-column>
                </el-table>
                <div v-if="gcResult.expired_uploads.length > 10" class="more-hint">
                  {{ t('tools.andMoreUploads', { count: gcResult.expired_uploads.length - 10 }) }}
                </div>
              </div>

              <!-- 无垃圾 -->
              <div v-if="gcResult.orphan_count === 0 && gcResult.expired_count === 0" class="gc-clean">
                <el-icon :size="48" color="#10b981"><CircleCheck /></el-icon>
                <p>{{ t('tools.noGarbageFound') }}</p>
              </div>
            </div>

            <!-- 未扫描提示 -->
            <div v-else class="gc-empty">
              <el-icon :size="48" color="#94a3b8"><Search /></el-icon>
              <p>{{ t('tools.clickScanHint') }}</p>
            </div>
          </div>
        </div>
      </el-col>

      <!-- 数据完整性检查 -->
      <el-col :span="24" style="margin-top: 24px">
        <div class="content-card">
          <div class="card-header">
            <h3>{{ t('tools.dataIntegrityCheck') }}</h3>
            <div class="header-actions">
              <el-button
                type="primary"
                class="primary-btn"
                @click="handleCheckIntegrity"
                :loading="integrityChecking"
                :icon="Search"
              >
                {{ t('tools.check') }}
              </el-button>
              <el-button
                type="warning"
                @click="handleRepairIntegrity"
                :loading="integrityRepairing"
                :disabled="!integrityResult || integrityResult.issues_found === 0"
                :icon="Refresh"
              >
                {{ t('tools.repair') }}
              </el-button>
            </div>
          </div>
          <div class="card-body">
            <el-form :inline="true" class="gc-form">
              <el-form-item :label="t('tools.verifyEtag')">
                <el-switch v-model="integrityVerifyEtag" />
              </el-form-item>
              <el-form-item :label="t('tools.limit')">
                <el-select v-model="integrityLimit" style="width: 150px">
                  <el-option :label="t('tools.objectsCount', { count: 100 })" :value="100" />
                  <el-option :label="t('tools.objectsCount', { count: 500 })" :value="500" />
                  <el-option :label="t('tools.objectsCount', { count: 1000 })" :value="1000" />
                  <el-option :label="t('tools.objectsCount', { count: 5000 })" :value="5000" />
                  <el-option :label="t('tools.allObjects')" :value="0" />
                </el-select>
              </el-form-item>
            </el-form>

            <!-- 完整性检查结果 -->
            <div v-if="integrityResult" class="gc-result">
              <el-row :gutter="24" class="gc-stats">
                <el-col :span="6">
                  <div class="stat-card">
                    <div class="stat-value">{{ integrityResult.total_checked }}</div>
                    <div class="stat-label">{{ t('tools.totalChecked') }}</div>
                    <div class="stat-size">{{ integrityResult.duration.toFixed(2) }}s</div>
                  </div>
                </el-col>
                <el-col :span="6">
                  <div class="stat-card" :class="{ danger: integrityResult.missing_files > 0 }">
                    <div class="stat-value">{{ integrityResult.missing_files }}</div>
                    <div class="stat-label">{{ t('tools.missingFiles') }}</div>
                  </div>
                </el-col>
                <el-col :span="6">
                  <div class="stat-card" :class="{ warning: integrityResult.etag_mismatches > 0 }">
                    <div class="stat-value">{{ integrityResult.etag_mismatches }}</div>
                    <div class="stat-label">{{ t('tools.etagMismatches') }}</div>
                  </div>
                </el-col>
                <el-col :span="6">
                  <div class="stat-card" :class="{ success: integrityResult.repaired }">
                    <div class="stat-value">{{ integrityResult.repaired ? integrityResult.repaired_count : '-' }}</div>
                    <div class="stat-label">{{ t('tools.repaired') }}</div>
                  </div>
                </el-col>
              </el-row>

              <!-- 问题列表 -->
              <div v-if="integrityResult.issues && integrityResult.issues.length > 0" class="gc-files">
                <h4>{{ t('tools.issuesFound', { count: integrityResult.issues.length }) }}</h4>
                <el-table :data="integrityResult.issues.slice(0, 20)" size="small" max-height="300">
                  <el-table-column prop="bucket" :label="t('tools.bucket')" width="120" />
                  <el-table-column prop="key" :label="t('tools.key')" min-width="200" />
                  <el-table-column :label="t('tools.issueType')" width="150">
                    <template #default="{ row }">
                      <el-tag :type="getIssueTagType(row.issue_type)" size="small">
                        {{ formatIssueType(row.issue_type) }}
                      </el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column :label="t('tools.size')" width="100">
                    <template #default="{ row }">{{ formatSize(row.size) }}</template>
                  </el-table-column>
                  <el-table-column :label="t('tools.repairable')" width="100">
                    <template #default="{ row }">
                      <el-tag :type="row.repairable ? 'success' : 'info'" size="small">
                        {{ row.repairable ? t('tools.yes') : t('tools.no') }}
                      </el-tag>
                    </template>
                  </el-table-column>
                </el-table>
                <div v-if="integrityResult.issues.length > 20" class="more-hint">
                  {{ t('tools.andMoreIssues', { count: integrityResult.issues.length - 20 }) }}
                </div>
              </div>

              <!-- 无问题 -->
              <div v-if="integrityResult.issues_found === 0" class="gc-clean">
                <el-icon :size="48" color="#10b981"><CircleCheck /></el-icon>
                <p>{{ t('tools.noIntegrityIssues') }}</p>
              </div>
            </div>

            <!-- 未检查提示 -->
            <div v-else class="gc-empty">
              <el-icon :size="48" color="#94a3b8"><Search /></el-icon>
              <p>{{ t('tools.clickCheckHint') }}</p>
            </div>
          </div>
        </div>
      </el-col>

      <!-- 数据迁移工具 -->
      <el-col :span="24" style="margin-top: 24px">
        <div class="content-card">
          <div class="card-header">
            <h3>{{ t('tools.dataMigration') }}</h3>
            <div class="header-actions">
              <el-button
                type="primary"
                class="primary-btn"
                @click="openMigrateDialog"
                :icon="Link"
              >
                {{ t('tools.newMigration') }}
              </el-button>
              <el-button
                @click="loadMigrateJobs"
                :loading="migrateLoading"
                :icon="Refresh"
              >
                {{ t('common.refresh') }}
              </el-button>
            </div>
          </div>
          <div class="card-body">
            <!-- 迁移任务列表 -->
            <div v-if="migrateJobs.length > 0" class="migrate-list">
              <el-table :data="migrateJobs" stripe>
                <el-table-column :label="t('tools.jobId')" width="120">
                  <template #default="{ row }">
                    <span class="job-id">{{ row.jobId.substring(0, 8) }}...</span>
                  </template>
                </el-table-column>
                <el-table-column :label="t('tools.source')" min-width="200">
                  <template #default="{ row }">
                    <div class="source-info">
                      <div class="source-endpoint">{{ row.config.sourceEndpoint }}</div>
                      <div class="source-bucket">{{ row.config.sourceBucket }}{{ row.config.sourcePrefix ? '/' + row.config.sourcePrefix : '' }}</div>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column :label="t('tools.target')" width="150">
                  <template #default="{ row }">
                    <span>{{ row.config.targetBucket }}{{ row.config.targetPrefix ? '/' + row.config.targetPrefix : '' }}</span>
                  </template>
                </el-table-column>
                <el-table-column :label="t('tools.progress')" width="180">
                  <template #default="{ row }">
                    <div class="progress-cell">
                      <el-progress
                        :percentage="getMigrateProgress(row)"
                        :status="row.status === 'failed' ? 'exception' : row.status === 'completed' ? 'success' : ''"
                        :stroke-width="8"
                      />
                      <div class="progress-text">
                        {{ row.completed }}/{{ row.totalObjects }}
                        <span v-if="row.skipped > 0">({{ row.skipped }} {{ t('tools.skipped') }})</span>
                      </div>
                    </div>
                  </template>
                </el-table-column>
                <el-table-column :label="t('tools.status')" width="110">
                  <template #default="{ row }">
                    <el-tag :type="getMigrateStatusType(row.status)" size="small">
                      {{ row.status }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column :label="t('tools.duration')" width="100">
                  <template #default="{ row }">
                    <span class="duration">{{ getMigrateDuration(row) }}</span>
                  </template>
                </el-table-column>
                <el-table-column :label="t('common.actions')" width="120" fixed="right">
                  <template #default="{ row }">
                    <el-button
                      v-if="row.status === 'running' || row.status === 'pending'"
                      type="warning"
                      size="small"
                      @click="handleCancelMigration(row)"
                      :icon="Close"
                    >
                      {{ t('common.cancel') }}
                    </el-button>
                    <el-button
                      v-else
                      type="danger"
                      size="small"
                      @click="handleDeleteMigration(row)"
                      :icon="Delete"
                    >
                      {{ t('common.delete') }}
                    </el-button>
                  </template>
                </el-table-column>
              </el-table>
            </div>

            <!-- 无任务提示 -->
            <div v-else class="gc-empty">
              <el-icon :size="48" color="#94a3b8"><Link /></el-icon>
              <p>{{ t('tools.noMigrationJobs') }}</p>
            </div>
          </div>
        </div>
      </el-col>

      <!-- 迁移配置对话框 -->
      <el-dialog v-model="showMigrateDialog" :title="t('tools.createMigrationJob')" width="600px">
        <el-form :model="migrateForm" label-position="top">
          <el-divider content-position="left">{{ t('tools.sourceS3Service') }}</el-divider>

          <el-row :gutter="16">
            <el-col :span="16">
              <el-form-item :label="t('tools.endpointUrl')" required>
                <el-input
                  v-model="migrateForm.sourceEndpoint"
                  :placeholder="t('tools.endpointPlaceholder')"
                />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item :label="t('tools.region')">
                <el-input v-model="migrateForm.sourceRegion" :placeholder="t('tools.regionPlaceholder')" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item :label="t('tools.accessKey')" required>
                <el-input v-model="migrateForm.sourceAccessKey" :placeholder="t('tools.accessKeyPlaceholder')" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item :label="t('tools.secretKey')" required>
                <el-input
                  v-model="migrateForm.sourceSecretKey"
                  type="password"
                  :placeholder="t('tools.secretKeyPlaceholder')"
                  show-password
                />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item :label="t('tools.sourceBucket')" required>
                <el-input v-model="migrateForm.sourceBucket" :placeholder="t('tools.bucketPlaceholder')" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item :label="t('tools.sourcePrefix')">
                <el-input v-model="migrateForm.sourcePrefix" :placeholder="t('tools.optionalPrefix')" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-button type="info" size="small" @click="handleTestConnection" :loading="migrateValidating">
            {{ t('tools.testConnection') }}
          </el-button>

          <el-divider content-position="left">{{ t('tools.targetLocal') }}</el-divider>

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item :label="t('tools.targetBucket')" required>
                <el-select v-model="migrateForm.targetBucket" :placeholder="t('tools.selectLocalBucket')" style="width: 100%">
                  <el-option v-for="b in buckets" :key="b.name" :label="b.name" :value="b.name" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item :label="t('tools.targetPrefix')">
                <el-input v-model="migrateForm.targetPrefix" :placeholder="t('tools.optionalPrefix')" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item>
            <el-checkbox v-model="migrateForm.overwriteExist">
              {{ t('tools.overwriteExisting') }}
            </el-checkbox>
          </el-form-item>
        </el-form>

        <template #footer>
          <el-button @click="showMigrateDialog = false">{{ t('common.cancel') }}</el-button>
          <el-button type="primary" class="primary-btn" @click="handleCreateMigration" :loading="migrateCreating">
            {{ t('tools.startMigration') }}
          </el-button>
        </template>
      </el-dialog>

      <!-- 预签名 URL 生成器 -->
      <el-col :span="12" style="margin-top: 24px">
        <div class="content-card">
          <div class="card-header">
            <h3>{{ t('tools.presignedUrlGenerator') }}</h3>
          </div>
          <div class="card-body">
            <el-form :model="presignForm" label-position="top">
              <el-form-item :label="t('tools.bucket')">
                <el-select
                  v-model="presignForm.bucket"
                  :placeholder="t('tools.selectBucket')"
                  @change="loadBucketObjects"
                  style="width: 100%"
                >
                  <el-option v-for="b in buckets" :key="b.name" :label="b.name" :value="b.name" />
                </el-select>
              </el-form-item>

              <el-form-item :label="t('tools.objectPath')">
                <el-input v-model="presignForm.key" :placeholder="t('tools.enterOrSelectPath')">
                  <template #append>
                    <el-select
                      v-model="presignForm.key"
                      :placeholder="t('tools.select')"
                      style="width: 160px"
                      :disabled="!presignForm.bucket"
                    >
                      <el-option v-for="o in bucketObjects" :key="o.key" :label="o.key" :value="o.key" />
                    </el-select>
                  </template>
                </el-input>
              </el-form-item>

              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item :label="t('tools.httpMethod')">
                    <el-select v-model="presignForm.method" style="width: 100%">
                      <el-option :label="t('tools.putUpload')" value="PUT" />
                      <el-option :label="t('tools.getDownload')" value="GET" />
                      <el-option :label="t('tools.deleteRemove')" value="DELETE" />
                      <el-option :label="t('tools.headInfo')" value="HEAD" />
                    </el-select>
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item :label="t('tools.expiration')">
                    <el-select v-model="presignForm.expiresMinutes" style="width: 100%">
                      <el-option :label="t('tools.minutes', { count: 15 })" :value="15" />
                      <el-option :label="t('tools.hours', { count: 1 })" :value="60" />
                      <el-option :label="t('tools.hours', { count: 6 })" :value="360" />
                      <el-option :label="t('tools.hours', { count: 12 })" :value="720" />
                      <el-option :label="t('tools.hours', { count: 24 })" :value="1440" />
                      <el-option :label="t('tools.days', { count: 7 })" :value="10080" />
                    </el-select>
                  </el-form-item>
                </el-col>
              </el-row>

              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item :label="t('tools.maxSizeMB')">
                    <el-input-number
                      v-model="presignForm.maxSizeMB"
                      :min="0"
                      :max="1024"
                      :placeholder="t('tools.noLimit')"
                      style="width: 100%"
                    />
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item :label="t('tools.contentType')">
                    <el-input v-model="presignForm.contentType" :placeholder="t('tools.contentTypePlaceholder')" />
                  </el-form-item>
                </el-col>
              </el-row>

              <el-form-item>
                <el-button type="primary" class="primary-btn" @click="handleGeneratePresignedUrl" :loading="generating">
                  {{ t('tools.generateUrl') }}
                </el-button>
                <el-button @click="clearForm">{{ t('tools.clear') }}</el-button>
              </el-form-item>
            </el-form>

            <div v-if="presignedUrl" class="result-box">
              <label>{{ t('tools.generatedUrl') }}</label>
              <el-input v-model="presignedUrl" type="textarea" :rows="3" readonly />
              <el-button type="primary" class="primary-btn" size="small" @click="copyUrl" style="margin-top: 12px">
                <el-icon><CopyDocument /></el-icon>
                {{ t('tools.copyUrl') }}
              </el-button>
            </div>
          </div>
        </div>
      </el-col>

      <el-col :span="12" style="margin-top: 24px">
        <div class="content-card">
          <div class="card-header">
            <h3>{{ t('tools.serverInfo') }}</h3>
          </div>
          <div class="card-body">
            <div class="info-grid">
              <div class="info-item">
                <label>{{ t('tools.endpoint') }}</label>
                <span>{{ auth.endpoint }}</span>
              </div>
              <div class="info-item">
                <label>{{ t('tools.region') }}</label>
                <span>{{ serverRegion }}</span>
              </div>
              <div class="info-item">
                <label>{{ t('tools.bucketsCount') }}</label>
                <span>{{ buckets.length }}</span>
              </div>
            </div>
          </div>
        </div>

        <div class="content-card" style="margin-top: 24px">
          <div class="card-header">
            <h3>{{ t('tools.awsCliConfig') }}</h3>
          </div>
          <div class="card-body">
            <div class="code-block">
              <pre>{{ awsCliConfig }}</pre>
            </div>
            <el-button type="primary" class="primary-btn" size="small" @click="copyAwsConfig" style="margin-top: 12px">
              <el-icon><CopyDocument /></el-icon>
              {{ t('tools.copyConfig') }}
            </el-button>
          </div>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CopyDocument, Search, Delete, CircleCheck, Refresh, Link, Close } from '@element-plus/icons-vue'
import { useAuthStore } from '../stores/auth'
import { listBuckets, listObjects, generatePresignedUrl, type Bucket, type S3Object } from '../api/admin'
import {
  scanGC, executeGC, type GCResult,
  checkIntegrity, repairIntegrity, type IntegrityResult,
  listMigrateJobs, createMigrateJob, cancelMigrateJob, deleteMigrateJob, validateMigrateConfig,
  type MigrateConfig, type MigrateProgress,
  getSettings
} from '../api/admin'

const { t } = useI18n()
const auth = useAuthStore()

const buckets = ref<Bucket[]>([])
const bucketObjects = ref<S3Object[]>([])
const presignedUrl = ref('')
const generating = ref(false)
const serverRegion = ref('us-east-1')

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
region = ${serverRegion.value}
output = json

# Usage examples
aws --endpoint-url ${auth.endpoint} --profile sss s3 ls
aws --endpoint-url ${auth.endpoint} --profile sss s3 mb s3://my-bucket
aws --endpoint-url ${auth.endpoint} --profile sss s3 cp file.txt s3://my-bucket/`
})

onMounted(async () => {
  try {
    const [bucketsData, settings] = await Promise.all([
      listBuckets(),
      getSettings()
    ])
    buckets.value = bucketsData
    serverRegion.value = settings.region
  } catch (e) {
    // ignore
  }
  loadMigrateJobs()
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
      ElMessage.success(t('tools.noGarbageFound'))
    } else {
      ElMessage.info(t('tools.foundGarbage', { orphan: gcResult.value.orphan_count, expired: gcResult.value.expired_count }))
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || t('tools.scanFailed'))
  } finally {
    gcScanning.value = false
  }
}

// GC 执行
async function handleExecuteGC() {
  if (!gcResult.value) return

  try {
    await ElMessageBox.confirm(
      t('tools.confirmClean', { orphan: gcResult.value.orphan_count, orphanSize: formatSize(gcResult.value.orphan_size), expired: gcResult.value.expired_count }),
      t('tools.confirmGarbageCollection'),
      {
        confirmButtonText: t('tools.clean'),
        cancelButtonText: t('common.cancel'),
        type: 'warning'
      }
    )

    gcExecuting.value = true
    gcResult.value = await executeGC(gcMaxAge.value, false)
    ElMessage.success(t('tools.gcCompleted'))
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || t('tools.cleanFailed'))
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
      ElMessage.success(t('tools.noIntegrityIssues'))
    } else {
      ElMessage.warning(t('tools.foundIssues', { count: integrityResult.value.issues_found }))
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || t('tools.checkFailed'))
  } finally {
    integrityChecking.value = false
  }
}

// 修复完整性问题
async function handleRepairIntegrity() {
  if (!integrityResult.value || integrityResult.value.issues_found === 0) return

  try {
    await ElMessageBox.confirm(
      t('tools.confirmRepair', { count: integrityResult.value.issues_found }),
      t('tools.confirmRepairTitle'),
      {
        confirmButtonText: t('tools.repair'),
        cancelButtonText: t('common.cancel'),
        type: 'warning'
      }
    )

    integrityRepairing.value = true
    integrityResult.value = await repairIntegrity(
      integrityResult.value.issues,
      integrityVerifyEtag.value,
      integrityLimit.value
    )
    ElMessage.success(t('tools.repairedCount', { count: integrityResult.value.repaired_count }))
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || t('tools.repairFailed'))
    }
  } finally {
    integrityRepairing.value = false
  }
}

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

function formatIssueType(issueType: string): string {
  switch (issueType) {
    case 'missing_file':
      return t('tools.missingFile')
    case 'etag_mismatch':
      return t('tools.etagMismatch')
    case 'path_mismatch':
      return t('tools.pathMismatch')
    default:
      return issueType
  }
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

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
    ElMessage.warning(t('tools.selectBucketAndPath'))
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

    let info = t('tools.generatedPresignedUrl', { method: result.method })
    if (presignForm.maxSizeMB > 0) {
      info += `, max ${presignForm.maxSizeMB}MB`
    }
    if (presignForm.contentType) {
      info += `, type: ${presignForm.contentType}`
    }
    ElMessage.success(info)
  } catch (error: any) {
    console.error('Failed to generate presigned URL:', error)
    ElMessage.error(error.response?.data?.message || t('tools.generateFailed'))
  } finally {
    generating.value = false
  }
}

function copyToClipboard(text: string) {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.style.position = 'fixed'
      textarea.style.left = '-9999px'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    ElMessage.success(t('common.copied'))
  } catch {
    ElMessage.error(t('common.copyFailed'))
  }
}

function copyUrl() {
  copyToClipboard(presignedUrl.value)
}

function copyAwsConfig() {
  copyToClipboard(awsCliConfig.value)
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

function startMigratePolling() {
  migratePollingTimer.value = window.setInterval(() => {
    const hasRunning = migrateJobs.value.some(j => j.status === 'pending' || j.status === 'running')
    if (hasRunning) {
      loadMigrateJobs()
    }
  }, 2000)
}

function stopMigratePolling() {
  if (migratePollingTimer.value) {
    clearInterval(migratePollingTimer.value)
    migratePollingTimer.value = null
  }
}

function openMigrateDialog() {
  resetMigrateForm()
  showMigrateDialog.value = true
}

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

async function handleTestConnection() {
  if (!migrateForm.sourceEndpoint || !migrateForm.sourceAccessKey || !migrateForm.sourceSecretKey || !migrateForm.sourceBucket) {
    ElMessage.warning(t('tools.fillRequiredFields'))
    return
  }

  migrateValidating.value = true
  try {
    const result = await validateMigrateConfig(migrateForm)
    if (result.valid) {
      ElMessage.success(t('tools.connectionSuccess'))
    } else {
      ElMessage.error(result.message || t('tools.connectionFailed'))
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || t('tools.connectionTestFailed'))
  } finally {
    migrateValidating.value = false
  }
}

async function handleCreateMigration() {
  if (!migrateForm.sourceEndpoint || !migrateForm.sourceAccessKey || !migrateForm.sourceSecretKey || !migrateForm.sourceBucket) {
    ElMessage.warning(t('tools.fillRequiredFields'))
    return
  }
  if (!migrateForm.targetBucket) {
    ElMessage.warning(t('tools.selectTargetBucket'))
    return
  }

  migrateCreating.value = true
  try {
    const result = await createMigrateJob(migrateForm)
    ElMessage.success(t('tools.migrationCreated', { id: result.jobId.substring(0, 8) }))
    showMigrateDialog.value = false
    loadMigrateJobs()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || t('tools.createMigrationFailed'))
  } finally {
    migrateCreating.value = false
  }
}

async function handleCancelMigration(job: MigrateProgress) {
  try {
    await ElMessageBox.confirm(
      t('tools.cancelMigrationConfirm', { id: job.jobId.substring(0, 8) }),
      t('tools.confirmCancel'),
      {
        confirmButtonText: t('tools.cancelJob'),
        cancelButtonText: t('tools.keepRunning'),
        type: 'warning'
      }
    )

    await cancelMigrateJob(job.jobId)
    ElMessage.success(t('tools.migrationCancelled'))
    loadMigrateJobs()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || t('tools.cancelFailed'))
    }
  }
}

async function handleDeleteMigration(job: MigrateProgress) {
  try {
    await ElMessageBox.confirm(
      t('tools.deleteMigrationConfirm', { id: job.jobId.substring(0, 8) }),
      t('tools.confirmDelete'),
      {
        confirmButtonText: t('common.delete'),
        cancelButtonText: t('common.cancel'),
        type: 'warning'
      }
    )

    await deleteMigrateJob(job.jobId)
    ElMessage.success(t('tools.migrationDeleted'))
    loadMigrateJobs()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || t('tools.deleteFailed'))
    }
  }
}

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

function getMigrateProgress(job: MigrateProgress): number {
  if (job.totalObjects === 0) return 0
  return Math.round((job.completed / job.totalObjects) * 100)
}

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

.primary-btn {
  background: #e67e22;
  border-color: #e67e22;
}

.primary-btn:hover {
  background: #d35400;
  border-color: #d35400;
}
</style>
