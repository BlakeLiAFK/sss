import axios from 'axios'
import { useAuthStore } from '../stores/auth'

// 存储统计类型定义
export interface BucketStat {
  name: string
  object_count: number
  total_size: number
  is_public: boolean
}

export interface TypeStat {
  content_type: string
  extension: string
  count: number
  total_size: number
}

export interface StorageStats {
  total_buckets: number
  total_objects: number
  total_size: number
  bucket_stats: BucketStat[]
  type_stats: TypeStat[]
}

export interface StatsResponse {
  stats: StorageStats
  disk_usage: number
  disk_file_count: number
}

export interface RecentObject {
  key: string
  size: number
  last_modified: string
  etag: string
}

// 获取管理 API 请求头
function getAdminHeaders() {
  const auth = useAuthStore()
  return {
    'X-Admin-Token': auth.adminToken || '',
    'Content-Type': 'application/json'
  }
}

// 获取 API 基础 URL
function getBaseUrl() {
  const auth = useAuthStore()
  return auth.endpoint
}

// 获取存储统计
export async function getStorageStats(): Promise<StatsResponse> {
  const resp = await axios.get(`${getBaseUrl()}/api/admin/stats/overview`, {
    headers: getAdminHeaders()
  })
  return resp.data
}

// 获取最近上传的对象
export async function getRecentObjects(limit = 10): Promise<RecentObject[]> {
  const resp = await axios.get(`${getBaseUrl()}/api/admin/stats/recent`, {
    headers: getAdminHeaders(),
    params: { limit }
  })
  return resp.data
}

// 孤立文件信息
export interface OrphanFile {
  path: string
  size: number
  modified_at: string
}

// GC 结果
export interface GCResult {
  orphan_files: OrphanFile[]
  orphan_count: number
  orphan_size: number
  expired_uploads: string[]
  expired_count: number
  expired_part_size: number
  cleaned: boolean
  cleaned_at: string | null
}

// 扫描垃圾（预览模式）
export async function scanGC(maxUploadAge = 24): Promise<GCResult> {
  const resp = await axios.get(`${getBaseUrl()}/api/admin/storage/gc`, {
    headers: getAdminHeaders(),
    params: { max_upload_age: maxUploadAge }
  })
  return resp.data
}

// 执行垃圾回收
export async function executeGC(maxUploadAge = 24, dryRun = false): Promise<GCResult> {
  const resp = await axios.post(`${getBaseUrl()}/api/admin/storage/gc`, {
    max_upload_age: maxUploadAge,
    dry_run: dryRun
  }, {
    headers: getAdminHeaders()
  })
  return resp.data
}

// 完整性问题
export interface IntegrityIssue {
  bucket: string
  key: string
  issue_type: string  // missing_file, etag_mismatch, path_mismatch
  expected: string
  actual: string
  size: number
  repairable: boolean
}

// 完整性检查结果
export interface IntegrityResult {
  total_checked: number
  issues_found: number
  issues: IntegrityIssue[]
  missing_files: number
  etag_mismatches: number
  path_mismatches: number
  checked_at: string
  duration: number
  repaired: boolean
  repaired_count: number
}

// 扫描完整性问题
export async function checkIntegrity(verifyEtag = false, limit = 1000): Promise<IntegrityResult> {
  const resp = await axios.get(`${getBaseUrl()}/api/admin/storage/integrity`, {
    headers: getAdminHeaders(),
    params: { verify_etag: verifyEtag, limit }
  })
  return resp.data
}

// 修复完整性问题
export async function repairIntegrity(issues?: IntegrityIssue[], verifyEtag = false, limit = 1000): Promise<IntegrityResult> {
  const resp = await axios.post(`${getBaseUrl()}/api/admin/storage/integrity`, {
    issues: issues || [],
    verify_etag: verifyEtag,
    limit
  }, {
    headers: getAdminHeaders()
  })
  return resp.data
}

// 批量删除结果
export interface BatchDeleteResult {
  deleted_count: number
  failed_count: number
  failed_keys: string[]
}

// 批量删除对象
export async function batchDeleteObjects(bucket: string, keys: string[]): Promise<BatchDeleteResult> {
  const resp = await axios.post(`${getBaseUrl()}/api/admin/buckets/${bucket}/batch/delete`, {
    keys
  }, {
    headers: getAdminHeaders()
  })
  return resp.data
}

// 批量下载对象（返回 ZIP 文件）
export async function batchDownloadObjects(bucket: string, keys: string[]): Promise<Blob> {
  const resp = await axios.post(`${getBaseUrl()}/api/admin/buckets/${bucket}/batch/download`, {
    keys
  }, {
    headers: getAdminHeaders(),
    responseType: 'blob'
  })
  return resp.data
}

// 迁移配置
export interface MigrateConfig {
  sourceEndpoint: string
  sourceAccessKey: string
  sourceSecretKey: string
  sourceBucket: string
  sourcePrefix?: string
  sourceRegion?: string
  targetBucket: string
  targetPrefix?: string
  overwriteExist: boolean
}

// 迁移进度
export interface MigrateProgress {
  jobId: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled'
  totalObjects: number
  completed: number
  failed: number
  skipped: number
  totalSize: number
  transferSize: number
  currentFile?: string
  startTime: string
  endTime?: string
  error?: string
  failedObjects?: string[]
  config: MigrateConfig
}

// 获取所有迁移任务
export async function listMigrateJobs(): Promise<MigrateProgress[]> {
  const resp = await axios.get(`${getBaseUrl()}/api/admin/migrate`, {
    headers: getAdminHeaders()
  })
  return resp.data.jobs || []
}

// 创建迁移任务
export async function createMigrateJob(config: MigrateConfig): Promise<{ jobId: string }> {
  const resp = await axios.post(`${getBaseUrl()}/api/admin/migrate`, config, {
    headers: getAdminHeaders()
  })
  return resp.data
}

// 获取迁移任务进度
export async function getMigrateProgress(jobId: string): Promise<MigrateProgress> {
  const resp = await axios.get(`${getBaseUrl()}/api/admin/migrate/${jobId}`, {
    headers: getAdminHeaders()
  })
  return resp.data
}

// 取消迁移任务
export async function cancelMigrateJob(jobId: string): Promise<void> {
  await axios.post(`${getBaseUrl()}/api/admin/migrate/${jobId}/cancel`, {}, {
    headers: getAdminHeaders()
  })
}

// 删除迁移任务记录
export async function deleteMigrateJob(jobId: string): Promise<void> {
  await axios.delete(`${getBaseUrl()}/api/admin/migrate/${jobId}`, {
    headers: getAdminHeaders()
  })
}

// 验证迁移配置（测试连接）
export async function validateMigrateConfig(config: MigrateConfig): Promise<{ valid: boolean; message: string }> {
  const resp = await axios.post(`${getBaseUrl()}/api/admin/migrate/validate`, config, {
    headers: getAdminHeaders()
  })
  return resp.data
}
