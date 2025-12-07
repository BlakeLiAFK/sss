package storage

import (
	"database/sql"
	"time"
)

// AuditAction 审计操作类型
type AuditAction string

const (
	// 认证相关
	AuditActionLogin         AuditAction = "login"          // 登录
	AuditActionLoginFailed   AuditAction = "login_failed"   // 登录失败
	AuditActionLogout        AuditAction = "logout"         // 登出
	AuditActionPasswordReset AuditAction = "password_reset" // 重置密码

	// 系统相关
	AuditActionSystemInstall  AuditAction = "system_install"  // 系统安装
	AuditActionSettingsUpdate AuditAction = "settings_update" // 更新系统设置
	AuditActionPasswordChange AuditAction = "password_change" // 修改密码

	// Bucket 相关
	AuditActionBucketCreate     AuditAction = "bucket_create"      // 创建桶
	AuditActionBucketDelete     AuditAction = "bucket_delete"      // 删除桶
	AuditActionBucketSetPublic  AuditAction = "bucket_set_public"  // 设置桶公开
	AuditActionBucketSetPrivate AuditAction = "bucket_set_private" // 设置桶私有

	// 对象相关
	AuditActionObjectUpload AuditAction = "object_upload" // 上传对象
	AuditActionObjectDelete AuditAction = "object_delete" // 删除对象
	AuditActionObjectCopy   AuditAction = "object_copy"   // 复制对象
	AuditActionBatchDelete  AuditAction = "batch_delete"  // 批量删除

	// API Key 相关
	AuditActionAPIKeyCreate      AuditAction = "apikey_create"       // 创建 API Key
	AuditActionAPIKeyDelete      AuditAction = "apikey_delete"       // 删除 API Key
	AuditActionAPIKeyResetSecret AuditAction = "apikey_reset_secret" // 重置 Secret
	AuditActionAPIKeyUpdate      AuditAction = "apikey_update"       // 更新 API Key
	AuditActionAPIKeySetPerm     AuditAction = "apikey_set_perm"     // 设置权限
	AuditActionAPIKeyDelPerm     AuditAction = "apikey_del_perm"     // 删除权限

	// 迁移相关
	AuditActionMigrateCreate AuditAction = "migrate_create" // 创建迁移任务
	AuditActionMigrateCancel AuditAction = "migrate_cancel" // 取消迁移任务
)

// AuditLog 审计日志
type AuditLog struct {
	ID        int64       `json:"id"`
	Timestamp time.Time   `json:"timestamp"`
	Action    AuditAction `json:"action"`
	Actor     string      `json:"actor"`      // 操作者（用户名或 API Key ID）
	IP        string      `json:"ip"`         // 客户端 IP
	Resource  string      `json:"resource"`   // 资源（桶名、对象键等）
	Detail    string      `json:"detail"`     // 详细信息（JSON 格式）
	Success   bool        `json:"success"`    // 是否成功
	UserAgent string      `json:"user_agent"` // 客户端 User-Agent
}

// initAuditTable 初始化审计日志表
func (m *MetadataStore) initAuditTable() error {
	schema := `CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		action TEXT NOT NULL,
		actor TEXT NOT NULL DEFAULT '',
		ip TEXT NOT NULL DEFAULT '',
		resource TEXT NOT NULL DEFAULT '',
		detail TEXT NOT NULL DEFAULT '',
		success INTEGER NOT NULL DEFAULT 1,
		user_agent TEXT NOT NULL DEFAULT ''
	)`
	if _, err := m.db.Exec(schema); err != nil {
		return err
	}

	// 创建索引
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_logs(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_logs(actor)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_ip ON audit_logs(ip)`,
	}
	for _, idx := range indexes {
		if _, err := m.db.Exec(idx); err != nil {
			return err
		}
	}
	return nil
}

// WriteAuditLog 写入审计日志
func (m *MetadataStore) WriteAuditLog(log *AuditLog) error {
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now().UTC()
	}

	successInt := 0
	if log.Success {
		successInt = 1
	}

	_, err := m.db.Exec(`
		INSERT INTO audit_logs (timestamp, action, actor, ip, resource, detail, success, user_agent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		log.Timestamp, log.Action, log.Actor, log.IP, log.Resource, log.Detail, successInt, log.UserAgent,
	)
	return err
}

// AuditLogQuery 审计日志查询参数
type AuditLogQuery struct {
	Action    AuditAction // 操作类型（可选）
	Actor     string      // 操作者（可选）
	IP        string      // IP 地址（可选）
	Resource  string      // 资源（可选）
	StartTime *time.Time  // 开始时间（可选）
	EndTime   *time.Time  // 结束时间（可选）
	Success   *bool       // 是否成功（可选）
	Limit     int         // 返回数量限制
	Offset    int         // 偏移量
}

// QueryAuditLogs 查询审计日志
func (m *MetadataStore) QueryAuditLogs(query *AuditLogQuery) ([]AuditLog, int, error) {
	// 构建查询条件
	conditions := []string{}
	args := []interface{}{}

	if query.Action != "" {
		conditions = append(conditions, "action = ?")
		args = append(args, query.Action)
	}
	if query.Actor != "" {
		conditions = append(conditions, "actor LIKE ?")
		args = append(args, "%"+query.Actor+"%")
	}
	if query.IP != "" {
		conditions = append(conditions, "ip LIKE ?")
		args = append(args, "%"+query.IP+"%")
	}
	if query.Resource != "" {
		conditions = append(conditions, "resource LIKE ?")
		args = append(args, "%"+query.Resource+"%")
	}
	if query.StartTime != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, *query.StartTime)
	}
	if query.EndTime != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, *query.EndTime)
	}
	if query.Success != nil {
		successInt := 0
		if *query.Success {
			successInt = 1
		}
		conditions = append(conditions, "success = ?")
		args = append(args, successInt)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE "
		for i, cond := range conditions {
			if i > 0 {
				whereClause += " AND "
			}
			whereClause += cond
		}
	}

	// 查询总数
	var total int
	countSQL := "SELECT COUNT(*) FROM audit_logs " + whereClause
	if err := m.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 查询数据
	if query.Limit <= 0 {
		query.Limit = 100
	}
	if query.Limit > 1000 {
		query.Limit = 1000
	}

	dataSQL := "SELECT id, timestamp, action, actor, ip, resource, detail, success, user_agent FROM audit_logs " +
		whereClause + " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, query.Limit, query.Offset)

	rows, err := m.db.Query(dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var log AuditLog
		var successInt int
		if err := rows.Scan(&log.ID, &log.Timestamp, &log.Action, &log.Actor, &log.IP,
			&log.Resource, &log.Detail, &successInt, &log.UserAgent); err != nil {
			return nil, 0, err
		}
		log.Success = successInt == 1
		logs = append(logs, log)
	}

	return logs, total, nil
}

// GetRecentAuditLogs 获取最近的审计日志
func (m *MetadataStore) GetRecentAuditLogs(limit int) ([]AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := m.db.Query(`
		SELECT id, timestamp, action, actor, ip, resource, detail, success, user_agent
		FROM audit_logs ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var log AuditLog
		var successInt int
		if err := rows.Scan(&log.ID, &log.Timestamp, &log.Action, &log.Actor, &log.IP,
			&log.Resource, &log.Detail, &successInt, &log.UserAgent); err != nil {
			return nil, err
		}
		log.Success = successInt == 1
		logs = append(logs, log)
	}

	return logs, nil
}

// CleanOldAuditLogs 清理旧的审计日志
func (m *MetadataStore) CleanOldAuditLogs(beforeDays int) (int64, error) {
	if beforeDays <= 0 {
		beforeDays = 90 // 默认保留 90 天
	}

	cutoff := time.Now().AddDate(0, 0, -beforeDays)
	result, err := m.db.Exec("DELETE FROM audit_logs WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// GetAuditStats 获取审计统计
func (m *MetadataStore) GetAuditStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总数
	var total int
	if err := m.db.QueryRow("SELECT COUNT(*) FROM audit_logs").Scan(&total); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	stats["total"] = total

	// 今日数量
	today := time.Now().Truncate(24 * time.Hour)
	var todayCount int
	if err := m.db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE timestamp >= ?", today).Scan(&todayCount); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	stats["today"] = todayCount

	// 失败数量
	var failedCount int
	if err := m.db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE success = 0").Scan(&failedCount); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	stats["failed"] = failedCount

	// 按操作类型统计
	rows, err := m.db.Query("SELECT action, COUNT(*) FROM audit_logs GROUP BY action ORDER BY COUNT(*) DESC LIMIT 10")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	actionStats := make(map[string]int)
	for rows.Next() {
		var action string
		var count int
		if err := rows.Scan(&action, &count); err != nil {
			return nil, err
		}
		actionStats[action] = count
	}
	stats["by_action"] = actionStats

	return stats, nil
}
