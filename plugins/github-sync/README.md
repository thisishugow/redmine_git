# Redmine-GitHub Issue Sync

自動將 Redmine issue 同步到 GitHub 的服務。

## 功能特性

- ✅ 單向同步：Redmine → GitHub
- ✅ 支援多專案、多 repo 對應
- ✅ Custom field 選擇目標 repo
- ✅ 自動回寫 GitHub issue URL
- ✅ PostgreSQL 獨立 schema，不影響 Redmine
- ✅ 配置檔熱更新（不需重啟）
- ✅ Docker Compose 部署
- ✅ 錯誤記錄與通知

## 架構說明

```
┌─────────┐         ┌──────────────┐         ┌─────────┐
│ Redmine │ ←─輪詢─→│ GitHub Sync  │ ──建立─→ │ GitHub  │
│         │         │   Service    │  issue  │         │
└─────────┘         └──────────────┘         └─────────┘
     │                      │
     └──────┬───────────────┘
            ↓
    ┌──────────────┐
    │  PostgreSQL  │
    │ (獨立 schema)│
    └──────────────┘
```

## 前置準備

### 1. Redmine 設定

#### 1.1 建立 Custom Fields

進入 Redmine 管理介面 > 自訂欄位 > 議題，建立以下兩個欄位：

**欄位 1：目標 GitHub Repo**
- 名稱：`目標 GitHub Repo` 或 `Target GitHub Repo`
- 格式：**下拉選單 (List)**
- 選項值範例：
  ```
  mycompany/backend
  mycompany/frontend
  mycompany/mobile-app
  ```
- 設定：
  - ✓ 作為篩選條件
  - ✓ 可搜尋
  - ✗ 必填（沒選就不同步）

**欄位 2：GitHub Issue URL**
- 名稱：`GitHub Issue`
- 格式：**連結 (Link)**
- 設定：
  - ✓ 作為篩選條件
  - ✗ 唯讀（API 會自動寫入）

建立後，記下兩個欄位的 **ID**（從 API 回應或 URL 取得）。

#### 1.2 取得 API Key

1. 登入 Redmine
2. 個人設定 > API access key
3. 點擊「顯示」並複製 API key

### 2. GitHub 設定

#### 2.1 建立 Personal Access Token

1. GitHub Settings > Developer settings > Personal access tokens > Tokens (classic)
2. Generate new token (classic)
3. 選擇權限：
   - ✓ `repo` (完整權限)
4. 複製 token（只會顯示一次）

### 3. 部署設定

#### 3.1 建立配置檔

```bash
# 複製範例配置檔
cp github-sync-config.example.yaml github-sync-config.yaml

# 編輯配置檔
vim github-sync-config.yaml
```

填入實際的配置值：

```yaml
redmine:
  # Redmine API URL（用於 API 呼叫）
  url: "https://your-redmine.com"

  # Redmine 顯示 URL（可選，用於 GitHub issue 中的連結）
  # 使用場景：當 API URL 是內部網路位址時
  # 例如：url: "http://redmine:3000", display_url: "http://192.168.1.100:3000"
  display_url: ""

  api_key: "your-redmine-api-key"
  projects:
    - identifier: "my-project"
      custom_fields:
        target_repo_id: 10      # 你的欄位 ID
        github_issue_url_id: 11 # 你的欄位 ID

github:
  token: "ghp_xxxxxxxxxxxx"
  base_url: "https://github.com"

sync:
  interval: "5m"
  title_format: "[Redmine #%d] %s"
```

#### 3.2 啟動服務

```bash
# 啟動（包含 Redmine、GitFetcher、GitHub Sync）
docker-compose up -d

# 只啟動 GitHub Sync
docker-compose up -d github-sync

# 查看 logs
docker-compose logs -f github-sync
```

## 使用方法

### 1. 在 Redmine 開 Issue

1. 建立新 issue
2. 在「目標 GitHub Repo」欄位選擇 repo（例如 `mycompany/backend`）
3. 儲存 issue

### 2. 自動同步

- 服務會每 5 分鐘檢查一次
- 發現新 issue 後，會自動建立 GitHub issue
- GitHub issue title 格式：`[Redmine #123] 原標題`
- 同步完成後，Redmine 的「GitHub Issue」欄位會自動填入連結

### 3. 查看結果

- Redmine issue 頁面可看到 GitHub issue 連結
- GitHub issue 的 body 會包含 Redmine 的詳細資訊

## 配置熱更新

修改配置檔後，服務會自動偵測並重新載入（不需重啟容器）：

```bash
# 修改配置檔
vim github-sync-config.yaml

# 等待約 1 秒，服務會自動重載
docker-compose logs -f github-sync
# 會看到：Config file changed: /app/config.yaml
#        Config reloaded successfully
```

**可熱更新的項目**：
- `sync.interval` - 同步間隔
- `sync.title_format` - 標題格式
- `redmine.projects` - 專案列表
- `redmine.display_url` - 顯示 URL（下方有詳細說明）
- GitHub/Redmine 設定

**不可熱更新的項目**（需重啟）：
- 資料庫連線資訊

## 進階配置

### Display URL 配置

當你的 Redmine 部署在 Docker 內部網路時，會遇到以下問題：

**問題場景**：
- API 呼叫需要使用內部 URL：`http://redmine:3000`
- 但 GitHub issue 中的連結需要讓外部用戶能訪問：`http://192.168.181.245:13000`

**解決方案**：使用 `display_url` 配置

```yaml
redmine:
  # API 呼叫用（Docker 內部網路）
  url: "http://redmine:3000"

  # GitHub issue 連結顯示用（外部可訪問）
  display_url: "http://192.168.181.245:13000"
```

**效果**：
- 服務使用 `url` 來呼叫 Redmine API ✅
- GitHub issue body 中的連結使用 `display_url` ✅
- 如果不設定 `display_url`，會自動使用 `url` 的值

**範例**：

GitHub issue body 會顯示：
```markdown
**From Redmine Issue #123**
...
---
*Synced from Redmine: http://192.168.181.245:13000/issues/123*
```

而不是內部無法訪問的 `http://redmine:3000/issues/123`

## 資料庫說明

### Schema 隔離

服務使用獨立的 PostgreSQL schema (`redmine_github_sync`)，與 Redmine 的 `public` schema 完全隔離：

```sql
-- 查看 schemas
\dn

-- 查看同步記錄
SELECT * FROM redmine_github_sync.sync_records;

-- 查看錯誤記錄
SELECT * FROM redmine_github_sync.sync_errors WHERE resolved = FALSE;
```

### Tables 說明

**sync_records** - 同步記錄
- `redmine_issue_id` - Redmine issue ID（唯一）
- `github_repo` - GitHub repo（如 `mycompany/backend`）
- `github_issue_number` - GitHub issue 編號
- `github_issue_url` - GitHub issue 完整 URL
- `synced_at` - 同步時間

**sync_errors** - 錯誤記錄
- `redmine_issue_id` - 發生錯誤的 issue ID
- `error_message` - 錯誤訊息
- `occurred_at` - 發生時間
- `resolved` - 是否已解決

## 故障排除

### 檢查服務狀態

```bash
# 查看容器狀態
docker-compose ps

# 查看即時 logs
docker-compose logs -f github-sync

# 進入容器
docker-compose exec github-sync sh
```

### 常見問題

**1. Issue 沒有同步**

檢查項目：
- [ ] 是否有填「目標 GitHub Repo」
- [ ] 「GitHub Issue URL」欄位是否為空
- [ ] 查看 logs 是否有錯誤

```bash
docker-compose logs github-sync | grep "issue #"
```

**2. API 權限錯誤**

檢查：
- Redmine API key 是否正確
- GitHub token 是否有 `repo` 權限
- GitHub repo 是否存在且有寫入權限

**3. 資料庫連線失敗**

檢查：
- PostgreSQL 容器是否啟動
- 環境變數是否正確
- 網路連線

```bash
docker-compose exec github-sync ping postgres_db
```

**4. 配置檔熱更新失效**

某些編輯器（如 vim）會先寫暫存檔再 move，可能導致 fsnotify 失效。

解決方法：重啟容器
```bash
docker-compose restart github-sync
```

### 查看資料庫狀態

```bash
# 進入 PostgreSQL
docker-compose exec postgres_db psql -U redmine -d redmine

# 查看統計
SELECT
  (SELECT COUNT(*) FROM redmine_github_sync.sync_records) as total_synced,
  (SELECT COUNT(*) FROM redmine_github_sync.sync_records WHERE synced_at >= CURRENT_DATE) as today_synced,
  (SELECT COUNT(*) FROM redmine_github_sync.sync_errors WHERE resolved = FALSE) as unresolved_errors;
```

## 開發說明

### 專案結構

```
plugins/github-sync/
├── cmd/
│   └── sync/
│       └── main.go              # 主程式
├── internal/
│   ├── config/
│   │   └── config.go            # 配置管理（Viper + 熱更新）
│   ├── storage/
│   │   └── postgres.go          # PostgreSQL 儲存
│   ├── redmine/
│   │   └── client.go            # Redmine API client
│   ├── github/
│   │   └── client.go            # GitHub API client
│   └── sync/
│       ├── syncer.go            # 核心同步邏輯
│       └── scheduler.go         # 定時排程器
├── Dockerfile
├── go.mod
└── README.md
```

### 本地開發

```bash
cd plugins/github-sync

# 安裝依賴
go mod download

# 執行（需先啟動 PostgreSQL）
go run cmd/sync/main.go -config ../../github-sync-config.yaml

# 編譯
go build -o sync cmd/sync/main.go

# 測試
go test ./...
```

### 環境變數

| 變數 | 說明 | 預設值 |
|------|------|--------|
| `CONFIG_PATH` | 配置檔路徑 | `./config.yaml` |
| `POSTGRES_HOST` | PostgreSQL 主機 | `localhost` |
| `POSTGRES_PORT` | PostgreSQL 埠號 | `5432` |
| `POSTGRES_DB` | 資料庫名稱 | `redmine` |
| `POSTGRES_USER` | 資料庫使用者 | `redmine` |
| `POSTGRES_PASSWORD` | 資料庫密碼 | - |
| `POSTGRES_SCHEMA` | Schema 名稱 | `redmine_github_sync` |

## MVP 限制

目前版本（MVP）的限制：

- ❌ 不支援更新同步（Redmine 改了不會更新 GitHub）
- ❌ 不支援狀態同步（關閉/重開）
- ❌ 不支援雙向同步（GitHub → Redmine）
- ❌ 不支援 assignee 映射
- ❌ 不支援 tag/milestone 同步

未來可擴充的功能請參考 issue tracker。

## 授權

MIT License

## 維護者

CLCS Admin
