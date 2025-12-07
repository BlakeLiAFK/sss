#!/bin/bash
# Admin API 回归测试脚本
# API 基础路径: /api/admin/

echo "========================================"
echo "Admin API 回归测试"
echo "========================================"

BASE_URL="http://localhost:8080"
PASSED=0
FAILED=0
COOKIE_FILE="/tmp/sss_admin_cookie.txt"
TOKEN=""

# 清理旧 cookie
rm -f "$COOKIE_FILE"

# 测试辅助函数
test_api() {
    local name="$1"
    local expected="$2"
    local actual="$3"

    echo -n "  $name: "
    if [[ "$actual" == *"$expected"* ]]; then
        echo "PASSED (HTTP $actual)"
        ((PASSED++))
    else
        echo "FAILED (期望包含 $expected, 实际 $actual)"
        ((FAILED++))
    fi
}

# ========================================
# 1. 认证测试
# ========================================
echo ""
echo "=== 1. 认证测试 ==="

# 1.1 未认证访问 Admin API
echo "1.1 未认证访问测试:"
code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/admin/buckets")
test_api "GET /api/admin/buckets (无认证)" "401" "$code"

code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/admin/stats/overview")
test_api "GET /api/admin/stats/overview (无认证)" "401" "$code"

code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/admin/apikeys")
test_api "GET /api/admin/apikeys (无认证)" "401" "$code"

# 1.2 错误凭证登录
echo ""
echo "1.2 错误凭证登录:"
code=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
    -H "Content-Type: application/json" \
    -d '{"username":"wrong","password":"wrong"}' \
    "$BASE_URL/api/admin/login")
test_api "POST /api/admin/login (错误凭证)" "401" "$code"

# 1.3 正确凭证登录
echo ""
echo "1.3 正确凭证登录:"
response=$(curl -s -w "\n%{http_code}" -c "$COOKIE_FILE" -X POST \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin"}' \
    "$BASE_URL/api/admin/login")
code=$(echo "$response" | tail -1)
body=$(echo "$response" | sed '$d')
test_api "POST /api/admin/login (正确凭证)" "200" "$code"

# 提取 token
TOKEN=$(echo "$body" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
if [ -n "$TOKEN" ]; then
    echo "    -> 返回 token: OK ($TOKEN)"
else
    echo "    -> 返回 token: MISSING"
fi

# 1.4 使用 Cookie/Token 访问受保护端点
echo ""
echo "1.4 认证访问测试:"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" "$BASE_URL/api/admin/buckets")
test_api "GET /api/admin/buckets (带 Cookie)" "200" "$code"

# 也可以使用 X-Admin-Token header
code=$(curl -s -o /dev/null -w "%{http_code}" -H "X-Admin-Token: $TOKEN" "$BASE_URL/api/admin/buckets")
test_api "GET /api/admin/buckets (带 Token Header)" "200" "$code"

# ========================================
# 2. 健康检查
# ========================================
echo ""
echo "=== 2. 健康检查 ==="
response=$(curl -s "$BASE_URL/api/health")
if echo "$response" | grep -q '"status":"ok"'; then
    echo "  GET /api/health: PASSED"
    ((PASSED++))
else
    echo "  GET /api/health: FAILED (response: $response)"
    ((FAILED++))
fi

# ========================================
# 3. 存储桶管理
# ========================================
echo ""
echo "=== 3. 存储桶管理 ==="

# 3.1 列出存储桶
echo "3.1 列出存储桶:"
response=$(curl -s -b "$COOKIE_FILE" "$BASE_URL/api/admin/buckets")
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" "$BASE_URL/api/admin/buckets")
test_api "GET /api/admin/buckets" "200" "$code"

# 3.2 创建存储桶
echo ""
echo "3.2 创建存储桶:"
TEST_BUCKET="admin-test-bucket-$$"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" -X POST \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"$TEST_BUCKET\"}" \
    "$BASE_URL/api/admin/buckets")
test_api "POST /api/admin/buckets (创建 $TEST_BUCKET)" "200" "$code"

# 3.3 获取单个存储桶信息
echo ""
echo "3.3 获取存储桶信息:"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" "$BASE_URL/api/admin/buckets/$TEST_BUCKET")
test_api "GET /api/admin/buckets/$TEST_BUCKET" "200" "$code"

# 3.4 更新存储桶访问权限
echo ""
echo "3.4 更新存储桶访问权限:"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" -X PUT \
    -H "Content-Type: application/json" \
    -d '{"isPublic":true}' \
    "$BASE_URL/api/admin/buckets/$TEST_BUCKET")
test_api "PUT /api/admin/buckets/$TEST_BUCKET (设为公开)" "200" "$code"

# 验证公开状态
response=$(curl -s -b "$COOKIE_FILE" "$BASE_URL/api/admin/buckets/$TEST_BUCKET")
if echo "$response" | grep -q '"isPublic":true'; then
    echo "    -> 公开状态验证: OK"
else
    echo "    -> 公开状态验证: FAILED"
fi

# 3.5 设回私有
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" -X PUT \
    -H "Content-Type: application/json" \
    -d '{"isPublic":false}' \
    "$BASE_URL/api/admin/buckets/$TEST_BUCKET")
test_api "PUT /api/admin/buckets/$TEST_BUCKET (设为私有)" "200" "$code"

# ========================================
# 4. 对象管理
# ========================================
echo ""
echo "=== 4. 对象管理 ==="

# 4.1 上传对象
echo "4.1 上传对象:"
echo "Admin API test content - $(date)" > /tmp/admin-test-file.txt
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" -X POST \
    -F "file=@/tmp/admin-test-file.txt" \
    "$BASE_URL/api/admin/buckets/$TEST_BUCKET/upload?key=test-upload.txt")
test_api "POST /api/admin/buckets/$TEST_BUCKET/upload (上传文件)" "200" "$code"

# 4.2 列出对象
echo ""
echo "4.2 列出对象:"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" \
    "$BASE_URL/api/admin/buckets/$TEST_BUCKET/objects")
test_api "GET /api/admin/buckets/$TEST_BUCKET/objects" "200" "$code"

# 4.3 获取对象预览
echo ""
echo "4.3 获取对象预览:"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" \
    "$BASE_URL/api/admin/buckets/$TEST_BUCKET/preview?key=test-upload.txt")
test_api "GET .../preview?key=test-upload.txt" "200" "$code"

# 4.4 下载对象
echo ""
echo "4.4 下载对象:"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" \
    "$BASE_URL/api/admin/buckets/$TEST_BUCKET/download?key=test-upload.txt")
test_api "GET .../download?key=test-upload.txt" "200" "$code"

# 4.5 删除对象
echo ""
echo "4.5 删除对象:"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" -X DELETE \
    "$BASE_URL/api/admin/buckets/$TEST_BUCKET/objects?key=test-upload.txt")
test_api "DELETE .../objects?key=test-upload.txt" "200" "$code"

# ========================================
# 5. API Key 管理
# ========================================
echo ""
echo "=== 5. API Key 管理 ==="

# 5.1 列出 API Keys
echo "5.1 列出 API Keys:"
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" "$BASE_URL/api/admin/apikeys")
test_api "GET /api/admin/apikeys" "200" "$code"

# 5.2 创建 API Key
echo ""
echo "5.2 创建 API Key:"
response=$(curl -s -w "\n%{http_code}" -b "$COOKIE_FILE" -X POST \
    -H "Content-Type: application/json" \
    -d '{"name":"test-key-admin-api","buckets":["*"]}' \
    "$BASE_URL/api/admin/apikeys")
code=$(echo "$response" | tail -1)
body=$(echo "$response" | sed '$d')
test_api "POST /api/admin/apikeys (创建 API Key)" "200" "$code"

# 提取 key ID
KEY_ID=$(echo "$body" | grep -o '"accessKeyId":"[^"]*"' | cut -d'"' -f4)
if [ -n "$KEY_ID" ]; then
    echo "    -> 创建的 Key ID: $KEY_ID"
fi

# 5.3 获取单个 API Key
echo ""
echo "5.3 获取单个 API Key:"
if [ -n "$KEY_ID" ]; then
    code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" "$BASE_URL/api/admin/apikeys/$KEY_ID")
    test_api "GET /api/admin/apikeys/$KEY_ID" "200" "$code"
else
    echo "  GET /api/admin/apikeys/ID: SKIPPED (无 Key ID)"
fi

# 5.4 删除 API Key
echo ""
echo "5.4 删除 API Key:"
if [ -n "$KEY_ID" ]; then
    code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" -X DELETE "$BASE_URL/api/admin/apikeys/$KEY_ID")
    test_api "DELETE /api/admin/apikeys/$KEY_ID" "200" "$code"
else
    echo "  DELETE /api/admin/apikeys/ID: SKIPPED (无 Key ID)"
fi

# ========================================
# 6. 统计信息
# ========================================
echo ""
echo "=== 6. 统计信息 ==="
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" "$BASE_URL/api/admin/stats/overview")
test_api "GET /api/admin/stats/overview" "200" "$code"

# 验证统计信息内容
response=$(curl -s -b "$COOKIE_FILE" "$BASE_URL/api/admin/stats/overview")
if echo "$response" | grep -q "bucketCount"; then
    echo "    -> 包含 bucketCount: OK"
else
    echo "    -> 包含 bucketCount: MISSING"
fi

# 最近对象
echo ""
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" "$BASE_URL/api/admin/stats/recent")
test_api "GET /api/admin/stats/recent" "200" "$code"

# ========================================
# 7. 删除测试存储桶（清理）
# ========================================
echo ""
echo "=== 7. 清理测试数据 ==="
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" -X DELETE \
    "$BASE_URL/api/admin/buckets/$TEST_BUCKET")
test_api "DELETE /api/admin/buckets/$TEST_BUCKET" "200" "$code"

# ========================================
# 8. 登出测试
# ========================================
echo ""
echo "=== 8. 登出测试 ==="
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" -X POST "$BASE_URL/api/admin/logout")
test_api "POST /api/admin/logout" "200" "$code"

# 验证登出后无法访问
code=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE_FILE" "$BASE_URL/api/admin/buckets")
test_api "GET /api/admin/buckets (登出后)" "401" "$code"

# ========================================
# 测试结果汇总
# ========================================
echo ""
echo "========================================"
echo "Admin API 测试结果: $PASSED 通过, $FAILED 失败"
echo "========================================"

# 清理
rm -f "$COOKIE_FILE" /tmp/admin-test-file.txt

exit $FAILED
