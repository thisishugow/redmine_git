# Redmine Issue Notifier

輕量級 Redmine plugin，用於在 issue 建立或更新時，即時通知外部服務（如 GitHub Sync）。

## 功能特性

- ✅ 監聽 issue 建立和更新事件
- ✅ 只通知有設定「目標 GitHub Repo」的 issues
- ✅ 非同步發送 webhook（不阻塞 Redmine）
- ✅ 支援 HMAC-SHA256 簽章驗證
- ✅ 完整的錯誤處理和日誌記錄
- ✅ 兼容 Redmine 4.0+（包括 6.1）

## 安裝

### 1. 複製 plugin 到 Redmine plugins 目錄

在 Dockerfile 中已經包含：

```dockerfile
# 複製 issue notifier plugin（用於即時 webhook）
COPY plugins/redmine_issue_notifier /usr/src/redmine/plugins/redmine_issue_notifier
```

### 2. 設定環境變數

在 `docker-compose.yml` 中設定：

```yaml
redmine:
  environment:
    # GitHub Sync Webhook 設定
    GITHUB_SYNC_WEBHOOK_URL: "http://github-sync:8080/webhook/issue-changed"
    GITHUB_SYNC_WEBHOOK_SECRET: "your-secret-key-here"  # 可選，用於驗證
    REDMINE_TARGET_REPO_FIELD_ID: "11"  # 你的「目標 GitHub Repo」custom field ID
```

### 3. 重啟 Redmine

```bash
docker compose restart redmine
```

### 4. 驗證安裝

進入 Redmine 管理介面：
- 前往 **Administration > Plugins**
- 應該會看到 **Redmine Issue Notifier** plugin

## 運作原理

```
┌─────────────┐
│   Redmine   │
│             │
│  User 儲存  │
│   Issue     │
└──────┬──────┘
       │
       │ (controller_issues_edit_after_save hook)
       │
       ▼
┌─────────────────────┐
│ IssueNotifierHook   │
│                     │
│ 1. 檢查是否有設定    │
│    target_repo      │
│ 2. 準備 JSON payload│
│ 3. 非同步發送 HTTP  │
│    POST             │
└──────┬──────────────┘
       │
       │ HTTP POST
       │
       ▼
┌─────────────────────┐
│   GitHub Sync       │
│   :8080/webhook/    │
│   issue-changed     │
│                     │
│ 1. 驗證簽章（可選）  │
│ 2. 觸發即時同步      │
└─────────────────────┘
```

## Webhook Payload 格式

```json
{
  "issue_id": 123,
  "project_identifier": "my-project",
  "target_repo": "myorg/backend",
  "action": "updated",
  "timestamp": "2025-11-14T10:30:00Z"
}
```

## 簽章驗證

如果設定了 `GITHUB_SYNC_WEBHOOK_SECRET`，plugin 會在 HTTP header 加上簽章：

```
X-Webhook-Signature: sha256=<HMAC-SHA256 hex digest>
```

接收端可以用相同的 secret 驗證請求來源。

## 環境變數

| 變數 | 說明 | 預設值 | 必填 |
|------|------|--------|------|
| `GITHUB_SYNC_WEBHOOK_URL` | Webhook 目標 URL | `http://github-sync:8080/webhook/issue-changed` | 否 |
| `GITHUB_SYNC_WEBHOOK_SECRET` | HMAC 簽章密鑰 | `` | 否 |
| `REDMINE_TARGET_REPO_FIELD_ID` | 「目標 GitHub Repo」custom field 的 ID | - | **是** |

## 日誌記錄

Plugin 會記錄以下資訊到 Redmine logs：

```
[IssueNotifier] Issue #123 has target repo: myorg/backend, notifying github-sync...
[IssueNotifier] Successfully notified github-sync for issue #123
```

如果發生錯誤：

```
[IssueNotifier] Failed to notify github-sync: 500 Internal Server Error
[IssueNotifier] Error notifying github-sync: Errno::ECONNREFUSED - Connection refused
```

## 故障排除

### Plugin 沒有出現在 Plugins 列表

1. 檢查檔案路徑是否正確：
   ```bash
   docker exec -it super_redmine ls -la /usr/src/redmine/plugins/redmine_issue_notifier
   ```

2. 檢查 Redmine logs：
   ```bash
   docker logs super_redmine | grep -i notifier
   ```

3. 重啟 Redmine：
   ```bash
   docker compose restart redmine
   ```

### Webhook 沒有觸發

1. 檢查環境變數是否設定：
   ```bash
   docker exec -it super_redmine env | grep GITHUB_SYNC
   ```

2. 檢查 issue 是否有設定 target_repo：
   - 進入 issue 編輯頁面
   - 確認「目標 GitHub Repo」欄位有值

3. 查看 Redmine logs：
   ```bash
   docker logs -f super_redmine
   ```

### Connection refused 錯誤

檢查 github-sync 服務是否啟動：

```bash
docker compose ps github-sync
docker logs github-sync | grep "Webhook server"
```

## 效能考量

- **非同步執行**：Webhook 發送在獨立 thread 中進行，不會阻塞 Redmine UI
- **Timeout 保護**：HTTP 請求設定 5 秒 timeout
- **錯誤處理**：即使 webhook 失敗，也不會影響 Redmine 正常運作

## 開發說明

### 測試 Hook

可以在 Rails console 中測試：

```ruby
# 進入 Rails console
docker exec -it super_redmine bundle exec rails console -e production

# 手動觸發 hook
issue = Issue.find(123)
hook = IssueNotifierHook.new
hook.controller_issues_edit_after_save({issue: issue})
```

### 修改後重新載入

```bash
# 重啟 Redmine
docker compose restart redmine

# 或只重新載入 plugin（開發模式）
touch tmp/restart.txt
```

## 授權

MIT License

## 維護者

CLCS Admin
