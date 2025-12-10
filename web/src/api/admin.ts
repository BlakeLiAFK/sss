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

// ============================================================
// 对象存储操作（替代 S3 API，使用管理接口）
// ============================================================

// 桶信息
export interface Bucket {
  name: string
  creation_date: string
  is_public: boolean
}

// 对象信息
export interface S3Object {
  key: string
  size: number
  last_modified: string
  etag: string
}

// 列出所有桶
export async function listBuckets(): Promise<Bucket[]> {
  const resp = await axios.get(`${getBaseUrl()}/api/admin/buckets`, {
    headers: getAdminHeaders()
  })
  return resp.data
}

// 列出对象
export async function listObjects(bucket: string, prefix = '', marker = ''): Promise<{ objects: S3Object[], is_truncated: boolean, next_marker: string }> {
  const params: Record<string, string> = {}
  if (prefix) params.prefix = prefix
  if (marker) params.marker = marker

  const resp = await axios.get(`${getBaseUrl()}/api/admin/buckets/${bucket}/objects`, {
    headers: getAdminHeaders(),
    params
  })
  return {
    objects: resp.data.objects || [],
    is_truncated: resp.data.is_truncated || false,
    next_marker: resp.data.next_marker || ''
  }
}

// 删除单个对象
export async function deleteObject(bucket: string, key: string): Promise<void> {
  await axios.delete(`${getBaseUrl()}/api/admin/buckets/${bucket}/objects`, {
    headers: getAdminHeaders(),
    params: { key }
  })
}

// 上传对象
export async function uploadObject(bucket: string, key: string, file: File, onProgress?: (percent: number) => void): Promise<void> {
  const formData = new FormData()
  formData.append('file', file)

  await axios.post(`${getBaseUrl()}/api/admin/buckets/${bucket}/upload`, formData, {
    headers: {
      'X-Admin-Token': useAuthStore().adminToken || ''
    },
    params: { key },
    onUploadProgress: (e) => {
      if (onProgress && e.total) {
        onProgress(Math.round((e.loaded / e.total) * 100))
      }
    }
  })
}

// 复制对象（用于重命名）
export async function copyObject(bucket: string, sourceKey: string, destKey: string): Promise<void> {
  await axios.post(`${getBaseUrl()}/api/admin/buckets/${bucket}/copy`, {
    source_key: sourceKey,
    dest_key: destKey
  }, {
    headers: getAdminHeaders()
  })
}

// 搜索对象
export async function searchObjects(bucket: string, keyword: string): Promise<{ objects: S3Object[], count: number }> {
  const resp = await axios.get(`${getBaseUrl()}/api/admin/buckets/${bucket}/search`, {
    headers: getAdminHeaders(),
    params: { q: keyword }
  })
  return {
    objects: resp.data.objects || [],
    count: resp.data.count || 0
  }
}

// 获取桶公开状态
export async function getBucketPublic(bucket: string): Promise<boolean> {
  const resp = await axios.get(`${getBaseUrl()}/api/admin/buckets/${bucket}/public`, {
    headers: getAdminHeaders()
  })
  return resp.data.is_public
}

// 获取对象下载 URL
export function getObjectUrl(bucket: string, key: string): string {
  return `${getBaseUrl()}/${bucket}/${key}`
}

// 系统设置
export interface SystemSettings {
  region: string
  max_object_size: number
  max_presign_upload_size: number
  cors_origin: string
  presign_scheme: string
  listen_addr: string
  data_dir: string
  db_path: string
  version: string
  installed_at: string
}

// 获取系统设置
export async function getSettings(): Promise<SystemSettings> {
  const resp = await axios.get(`${getBaseUrl()}/api/admin/settings`, {
    headers: getAdminHeaders()
  })
  return resp.data
}

// 预签名URL选项
interface PresignOptions {
  method?: string
  bucket: string
  key: string
  expiresMinutes?: number
  maxSizeMB?: number
  contentType?: string
}

// 预签名URL响应
interface PresignResponse {
  url: string
  method: string
  expires: number
}

// 生成预签名URL
export async function generatePresignedUrl(options: PresignOptions): Promise<PresignResponse> {
  const requestBody = {
    method: options.method || 'PUT',
    bucket: options.bucket,
    key: options.key,
    expiresMinutes: options.expiresMinutes || 60,
    maxSizeMB: options.maxSizeMB || 0,
    contentType: options.contentType || ''
  }

  const resp = await axios.post(`${getBaseUrl()}/api/presign`, requestBody, {
    headers: {
      'Content-Type': 'application/json'
    }
  })

  return resp.data
}

// ============================================================
// GeoStats API
// ============================================================

// GeoStats 配置
export interface GeoStatsConfig {
  enabled: boolean
  mode: string
  batch_size: number
  flush_interval: number
  retention_days: number
  geoip_enabled: boolean
}

// GeoStats 统计条目
export interface GeoStatEntry {
  id: number
  date: string
  country_code: string
  country: string
  city: string
  region: string
  request_count: number
  created_at: string
  updated_at: string
}

// GeoStats 聚合数据
export interface GeoStatsAggregated {
  country_code: string
  country: string
  city?: string
  region?: string
  total: number
}

// GeoStats 摘要
export interface GeoStatsSummary {
  total_requests: number
  country_count: number
  city_count: number
  start_date: string
  end_date: string
}

// 获取 GeoStats 配置
export async function getGeoStatsConfig(): Promise<GeoStatsConfig> {
  const resp = await axios.get(`${getBaseUrl()}/api/admin/geo-stats/config`, {
    headers: getAdminHeaders()
  })
  return resp.data
}

// 更新 GeoStats 配置
export async function updateGeoStatsConfig(config: Partial<GeoStatsConfig>): Promise<GeoStatsConfig> {
  const resp = await axios.put(`${getBaseUrl()}/api/admin/geo-stats/config`, config, {
    headers: getAdminHeaders()
  })
  return resp.data
}

// 获取 GeoStats 数据
export async function getGeoStatsData(params: {
  start_date?: string
  end_date?: string
  group_by?: string
  limit?: number
}): Promise<{ data: GeoStatsAggregated[], group_by: string, start_date: string, end_date: string }> {
  const resp = await axios.get(`${getBaseUrl()}/api/admin/geo-stats/data`, {
    headers: getAdminHeaders(),
    params
  })
  return resp.data
}

// 获取 GeoStats 摘要
export async function getGeoStatsSummary(params: {
  start_date?: string
  end_date?: string
}): Promise<GeoStatsSummary> {
  const resp = await axios.get(`${getBaseUrl()}/api/admin/geo-stats/summary`, {
    headers: getAdminHeaders(),
    params
  })
  return resp.data
}

// 清理 GeoStats 数据
export async function clearGeoStatsData(params?: {
  all?: boolean
  before_date?: string
}): Promise<{ success: boolean, message: string, affected?: number }> {
  const resp = await axios.delete(`${getBaseUrl()}/api/admin/geo-stats/data`, {
    headers: getAdminHeaders(),
    params
  })
  return resp.data
}
