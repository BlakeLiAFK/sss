import axios from 'axios'
import { useAuthStore } from '../stores/auth'

// AWS Signature V4
async function sign(method: string, path: string, headers: Record<string, string>, body?: string): Promise<{ headers: Record<string, string>, url: string }> {
  const auth = useAuthStore()
  const now = new Date()
  const dateStr = now.toISOString().replace(/[:-]|\.\d{3}/g, '').slice(0, 8)
  const amzDate = now.toISOString().replace(/[:-]|\.\d{3}/g, '')

  const signedHeaders = 'host;x-amz-content-sha256;x-amz-date'
  const payloadHash = body ? await sha256(body) : 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'

  const url = new URL(path, auth.endpoint)
  const host = url.host

  // 注意：host头部由浏览器自动设置，但签名仍需要包含它
  headers['x-amz-date'] = amzDate
  headers['x-amz-content-sha256'] = payloadHash

  // 规范请求
  const canonicalUri = url.pathname || '/'
  const canonicalQuery = url.search ? url.search.slice(1).split('&').sort().join('&') : ''
  const canonicalHeaders = `host:${host}\nx-amz-content-sha256:${payloadHash}\nx-amz-date:${amzDate}\n`
  const canonicalRequest = [method, canonicalUri, canonicalQuery, canonicalHeaders, signedHeaders, payloadHash].join('\n')

  // 待签名字符串
  const scope = `${dateStr}/${auth.region}/s3/aws4_request`
  const stringToSign = ['AWS4-HMAC-SHA256', amzDate, scope, await sha256(canonicalRequest)].join('\n')

  // 计算签名
  const kDate = await hmacSHA256(new TextEncoder().encode('AWS4' + auth.secretAccessKey), dateStr)
  const kRegion = await hmacSHA256(kDate, auth.region)
  const kService = await hmacSHA256(kRegion, 's3')
  const kSigning = await hmacSHA256(kService, 'aws4_request')
  const signature = await hmacSHA256Hex(kSigning, stringToSign)

  headers['Authorization'] = `AWS4-HMAC-SHA256 Credential=${auth.accessKeyId}/${scope}, SignedHeaders=${signedHeaders}, Signature=${signature}`

  return { headers, url: url.toString() }
}

async function sha256(message: string): Promise<string> {
  const msgBuffer = new TextEncoder().encode(message)
  const hashBuffer = await crypto.subtle.digest('SHA-256', msgBuffer)
  return Array.from(new Uint8Array(hashBuffer)).map(b => b.toString(16).padStart(2, '0')).join('')
}

async function hmacSHA256(key: Uint8Array | ArrayBuffer, message: string): Promise<Uint8Array> {
  const cryptoKey = await crypto.subtle.importKey('raw', key, { name: 'HMAC', hash: 'SHA-256' }, false, ['sign'])
  const sig = await crypto.subtle.sign('HMAC', cryptoKey, new TextEncoder().encode(message))
  return new Uint8Array(sig)
}

async function hmacSHA256Hex(key: Uint8Array, message: string): Promise<string> {
  const sig = await hmacSHA256(key, message)
  return Array.from(sig).map(b => b.toString(16).padStart(2, '0')).join('')
}

export interface Bucket {
  Name: string
  CreationDate: string
  IsPublic?: boolean
  toggling?: boolean // 加载状态
}

export interface S3Object {
  Key: string
  LastModified: string
  ETag: string
  Size: number
}

// 列出所有桶
export async function listBuckets(): Promise<Bucket[]> {
  const auth = useAuthStore()
  const { headers, url } = await sign('GET', '/', {})

  const resp = await axios.get(url, { headers })
  const parser = new DOMParser()
  const doc = parser.parseFromString(resp.data, 'text/xml')

  const buckets: Bucket[] = []
  doc.querySelectorAll('Bucket').forEach(node => {
    buckets.push({
      Name: node.querySelector('Name')?.textContent || '',
      CreationDate: node.querySelector('CreationDate')?.textContent || '',
      IsPublic: false // 默认为私有，稍后更新
    })
  })

  // 并行获取每个桶的公有状态
  await Promise.all(buckets.map(async (bucket) => {
    try {
      bucket.IsPublic = await getBucketPublic(bucket.Name)
    } catch (e) {
      // 获取失败，保持默认值
      console.warn(`Failed to get bucket ${bucket.Name} public status:`, e)
    }
  }))

  return buckets
}

// 创建桶
export async function createBucket(name: string): Promise<void> {
  const { headers, url } = await sign('PUT', `/${name}`, {})
  await axios.put(url, null, { headers })
}

// 删除桶
export async function deleteBucket(name: string): Promise<void> {
  const { headers, url } = await sign('DELETE', `/${name}`, {})
  await axios.delete(url, { headers })
}

// 列出对象
export async function listObjects(bucket: string, prefix = '', marker = ''): Promise<{ objects: S3Object[], isTruncated: boolean, nextMarker: string }> {
  let path = `/${bucket}?list-type=2&max-keys=100`
  if (prefix) path += `&prefix=${encodeURIComponent(prefix)}`
  if (marker) path += `&continuation-token=${encodeURIComponent(marker)}`

  const { headers, url } = await sign('GET', path, {})
  const resp = await axios.get(url, { headers })

  const parser = new DOMParser()
  const doc = parser.parseFromString(resp.data, 'text/xml')

  const objects: S3Object[] = []
  doc.querySelectorAll('Contents').forEach(node => {
    objects.push({
      Key: node.querySelector('Key')?.textContent || '',
      LastModified: node.querySelector('LastModified')?.textContent || '',
      ETag: node.querySelector('ETag')?.textContent || '',
      Size: parseInt(node.querySelector('Size')?.textContent || '0')
    })
  })

  return {
    objects,
    isTruncated: doc.querySelector('IsTruncated')?.textContent === 'true',
    nextMarker: doc.querySelector('NextContinuationToken')?.textContent || ''
  }
}

// 删除对象
export async function deleteObject(bucket: string, key: string): Promise<void> {
  const { headers, url } = await sign('DELETE', `/${bucket}/${key}`, {})
  await axios.delete(url, { headers })
}

// 获取对象下载URL（简单实现，实际应使用预签名URL）
export function getObjectUrl(bucket: string, key: string): string {
  const auth = useAuthStore()
  return `${auth.endpoint}/${bucket}/${key}`
}

// 上传对象
export async function uploadObject(bucket: string, key: string, file: File, onProgress?: (percent: number) => void): Promise<void> {
  const headers: Record<string, string> = {
    'Content-Type': file.type || 'application/octet-stream'
  }

  // 读取文件内容用于签名
  const content = await file.arrayBuffer()
  const contentHash = await sha256ArrayBuffer(content)

  // 使用专用的上传签名函数，因为内容哈希不同
  const { headers: signedHeaders, url } = await signUpload('PUT', `/${bucket}/${key}`, headers, contentHash)

  await axios.put(url, content, {
    headers: signedHeaders,
    onUploadProgress: (e) => {
      if (onProgress && e.total) {
        onProgress(Math.round((e.loaded / e.total) * 100))
      }
    }
  })
}

// 专用于上传的签名函数，接受预计算的内容哈希
async function signUpload(method: string, path: string, headers: Record<string, string>, contentHash: string): Promise<{ headers: Record<string, string>, url: string }> {
  const auth = useAuthStore()
  const now = new Date()
  const dateStr = now.toISOString().replace(/[:-]|\.\d{3}/g, '').slice(0, 8)
  const amzDate = now.toISOString().replace(/[:-]|\.\d{3}/g, '')

  const signedHeaders = 'content-type;host;x-amz-content-sha256;x-amz-date'

  const url = new URL(path, auth.endpoint)
  const host = url.host

  headers['x-amz-date'] = amzDate
  headers['x-amz-content-sha256'] = contentHash

  const canonicalHeaders = `content-type:${headers['Content-Type']}\nhost:${host}\nx-amz-content-sha256:${contentHash}\nx-amz-date:${amzDate}\n`
  const canonicalRequest = [method, url.pathname, '', canonicalHeaders, signedHeaders, contentHash].join('\n')

  const scope = `${dateStr}/${auth.region}/s3/aws4_request`
  const stringToSign = ['AWS4-HMAC-SHA256', amzDate, scope, await sha256(canonicalRequest)].join('\n')

  const kDate = await hmacSHA256(new TextEncoder().encode('AWS4' + auth.secretAccessKey), dateStr)
  const kRegion = await hmacSHA256(kDate, auth.region)
  const kService = await hmacSHA256(kRegion, 's3')
  const kSigning = await hmacSHA256(kService, 'aws4_request')
  const signature = await hmacSHA256Hex(kSigning, stringToSign)

  headers['Authorization'] = `AWS4-HMAC-SHA256 Credential=${auth.accessKeyId}/${scope}, SignedHeaders=${signedHeaders}, Signature=${signature}`

  return { headers, url: url.toString() }
}

async function sha256ArrayBuffer(buffer: ArrayBuffer): Promise<string> {
  const hashBuffer = await crypto.subtle.digest('SHA-256', buffer)
  return Array.from(new Uint8Array(hashBuffer)).map(b => b.toString(16).padStart(2, '0')).join('')
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
  const auth = useAuthStore()

  const requestBody = {
    method: options.method || 'PUT',
    bucket: options.bucket,
    key: options.key,
    expiresMinutes: options.expiresMinutes || 60,
    maxSizeMB: options.maxSizeMB || 0,
    contentType: options.contentType || ''
  }

  const resp = await axios.post(`${auth.endpoint}/api/presign`, requestBody, {
    headers: {
      'Content-Type': 'application/json'
    }
  })

  return resp.data
}

// 设置桶公有/私有状态
export async function setBucketPublic(bucketName: string, isPublic: boolean): Promise<boolean> {
  const auth = useAuthStore()

  const resp = await axios.put(`${auth.endpoint}/api/bucket/${bucketName}/public`, {
    is_public: isPublic
  }, {
    headers: {
      'Content-Type': 'application/json'
    }
  })

  return resp.data.is_public
}

// 获取桶公有/私有状态
export async function getBucketPublic(bucketName: string): Promise<boolean> {
  const auth = useAuthStore()

  const resp = await axios.get(`${auth.endpoint}/api/bucket/${bucketName}/public`)
  return resp.data.is_public
}

// 搜索对象（模糊匹配）
export async function searchObjects(bucket: string, keyword: string): Promise<{ objects: S3Object[], count: number }> {
  const auth = useAuthStore()
  const resp = await axios.get(`${auth.endpoint}/api/bucket/${bucket}/search`, {
    params: { q: keyword }
  })
  return {
    objects: resp.data.objects || [],
    count: resp.data.count || 0
  }
}

// 检查对象是否存在
export async function checkObjectExists(bucket: string, key: string): Promise<{ exists: boolean, size?: number, lastModified?: string }> {
  const auth = useAuthStore()
  const resp = await axios.get(`${auth.endpoint}/api/bucket/${bucket}/head`, {
    params: { key }
  })
  return resp.data
}

// 复制对象（用于重命名/移动）
export async function copyObject(srcBucket: string, srcKey: string, destBucket: string, destKey: string): Promise<void> {
  const auth = useAuthStore()
  const now = new Date()
  const dateStr = now.toISOString().replace(/[:-]|\.\d{3}/g, '').slice(0, 8)
  const amzDate = now.toISOString().replace(/[:-]|\.\d{3}/g, '')

  // 构建复制源路径
  const copySource = `/${srcBucket}/${srcKey}`

  const url = new URL(`/${destBucket}/${destKey}`, auth.endpoint)
  const host = url.host

  // 空 payload 的 hash
  const payloadHash = 'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855'

  // 签名头部（按字母顺序排列）
  const signedHeaders = 'host;x-amz-content-sha256;x-amz-copy-source;x-amz-date'

  // 规范头部
  const canonicalHeaders = `host:${host}\nx-amz-content-sha256:${payloadHash}\nx-amz-copy-source:${copySource}\nx-amz-date:${amzDate}\n`

  // 规范请求
  const canonicalRequest = ['PUT', url.pathname, '', canonicalHeaders, signedHeaders, payloadHash].join('\n')

  // 待签名字符串
  const scope = `${dateStr}/${auth.region}/s3/aws4_request`
  const stringToSign = ['AWS4-HMAC-SHA256', amzDate, scope, await sha256(canonicalRequest)].join('\n')

  // 计算签名
  const kDate = await hmacSHA256(new TextEncoder().encode('AWS4' + auth.secretAccessKey), dateStr)
  const kRegion = await hmacSHA256(kDate, auth.region)
  const kService = await hmacSHA256(kRegion, 's3')
  const kSigning = await hmacSHA256(kService, 'aws4_request')
  const signature = await hmacSHA256Hex(kSigning, stringToSign)

  const headers: Record<string, string> = {
    'x-amz-date': amzDate,
    'x-amz-content-sha256': payloadHash,
    'x-amz-copy-source': copySource,
    'Authorization': `AWS4-HMAC-SHA256 Credential=${auth.accessKeyId}/${scope}, SignedHeaders=${signedHeaders}, Signature=${signature}`
  }

  await axios.put(url.toString(), null, { headers })
}
