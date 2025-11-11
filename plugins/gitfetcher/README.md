# GitFetcher

自動定時拉取 Git repositories 的輕量級工具，專為 Redmine 容器環境設計。

## 功能特性

- ✅ **可配置定時拉取**：每個 repo 可設定不同的同步間隔（支援秒、分、小時）
- ✅ **熱更新配置**：修改 config.yaml 後自動重新載入，無需重啟
- ✅ **Web UI 管理介面**：簡潔的 HTML 頁面查看狀態和手動觸發同步
- ✅ **SSH Key 支援**：透過 volume 掛載 SSH keys，避免進入容器操作
- ✅ **自動日誌記錄**：每次 fetch 結果記錄到日誌檔案
- ✅ **狀態監控**：即時顯示每個 repo 的同步狀態、成功率、下次同步時間

## 快速開始

### 1. 準備配置檔案

複製範例配置並編輯：

```bash
cp config.example.yaml config.yaml
```

編輯 `config.yaml`：

```yaml
repos:
  - name: "my-project"
    url: "git@github.com:username/repo.git"
    local_path: "/repos/my-project.git"
    interval: "5m"

  - name: "another-project"
    url: "git@github.com:username/another.git"
    local_path: "/repos/another-project.git"
    interval: "1h"

ssh_key_path: "/root/.ssh/id_rsa"
http_port: 8080
log_path: "./logs"
```

### 2. 準備 SSH Keys

將你的 SSH private key 放到 `./ssh_keys/` 目錄：

```bash
mkdir -p ssh_keys
cp ~/.ssh/id_rsa ssh_keys/
chmod 600 ssh_keys/id_rsa
```

### 3. 預先 Clone Repositories (重要！)

GitFetcher **只負責 fetch**，不會自動 clone。你需要先手動 clone bare repository：

```bash
# 建立 repos 目錄
mkdir -p repos

# Clone bare repository
git clone --mirror git@github.com:username/repo.git repos/my-project.git
```

### 4. 啟動服務

#### 使用 Docker Compose (推薦)

```bash
# 複製範例
cp docker-compose.example.yml docker-compose.yml

# 啟動服務
docker-compose up -d

# 查看日誌
docker-compose logs -f
```

#### 使用 Docker

```bash
# 建立 Docker image
docker build -t gitfetcher .

# 執行容器
docker run -d \
  --name gitfetcher \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -v $(pwd)/ssh_keys:/root/.ssh:ro \
  -v repos:/repos \
  -v $(pwd)/logs:/app/logs \
  gitfetcher
```

#### 本地執行 (開發用)

```bash
# 安裝依賴
go mod download

# 執行
go run main.go -config config.yaml

# 或建立執行檔
go build -o bin/gitfetcher
./bin/gitfetcher -config config.yaml
```

### 5. 訪問 Web UI

開啟瀏覽器：http://localhost:8080

你可以：
- 查看所有 repo 的同步狀態
- 手動觸發立即同步
- 監控成功率和錯誤訊息
- 頁面每 5 秒自動刷新

## 與 Redmine 整合

### Docker Compose 整合範例

```yaml
version: '3.8'

services:
  redmine:
    image: redmine:latest
    volumes:
      - repos:/usr/src/redmine/repos  # 共享 repo volume
    networks:
      - redmine_network

  gitfetcher:
    build: ./plugins/gitfetcher
    ports:
      - "8080:8080"
    volumes:
      - ./plugins/gitfetcher/config.yaml:/app/config.yaml:ro
      - ./ssh_keys:/root/.ssh:ro
      - repos:/repos  # 與 Redmine 共享
      - ./logs:/app/logs
    networks:
      - redmine_network
    restart: unless-stopped

volumes:
  repos:

networks:
  redmine_network:
```

### Redmine 中配置 Repository

1. 進入 Redmine 專案設定 → Repositories
2. 選擇 Git
3. 路徑設定為：`/usr/src/redmine/repos/my-project.git`
4. GitFetcher 會自動定時同步，Redmine 即可瀏覽最新代碼

## 配置說明

### 時間間隔格式

支援 Go duration 格式：
- `5s` - 5 秒
- `10m` - 10 分鐘
- `1h` - 1 小時
- `30m` - 30 分鐘
- `2h30m` - 2 小時 30 分鐘

### 配置檔案結構

| 欄位 | 類型 | 說明 | 必填 |
|------|------|------|------|
| `repos` | array | Repository 列表 | 是 |
| `repos[].name` | string | Repository 名稱（唯一識別） | 是 |
| `repos[].url` | string | Git SSH URL | 是 |
| `repos[].local_path` | string | 本地儲存路徑（bare repo） | 是 |
| `repos[].interval` | string | 同步間隔 | 是 |
| `ssh_key_path` | string | SSH private key 路徑 | 否 |
| `http_port` | int | Web UI port | 否（預設 8080） |
| `log_path` | string | 日誌目錄 | 否（預設 ./logs） |

## 熱更新配置

修改 `config.yaml` 後，GitFetcher 會自動偵測並重新載入：

1. 編輯 `config.yaml`
2. 儲存檔案
3. 等待數秒，自動生效（無需重啟）

查看日誌確認：
```bash
docker-compose logs -f gitfetcher
# 應該看到 "Config file changed, reloading..." 和 "Config reloaded successfully"
```

## 常見問題

### Q: 為什麼 fetch 失敗？

A: 常見原因：
1. SSH key 權限問題：確保 `chmod 600` 設定正確
2. Repository 路徑不存在：先手動 `git clone --mirror`
3. SSH host key 驗證：首次連線需要手動確認（或使用 `StrictHostKeyChecking=no`，已內建）

### Q: 如何查看詳細日誌？

A: 日誌會記錄在 `logs/fetch-YYYY-MM-DD.log`：

```bash
# 查看今天的日誌
cat logs/fetch-$(date +%Y-%m-%d).log

# 即時追蹤
tail -f logs/fetch-$(date +%Y-%m-%d).log
```

### Q: 支援 HTTPS URL 嗎？

A: 目前主要針對 SSH URL 設計。HTTPS URL 可能需要額外配置 credential helper。

### Q: 可以只 clone 不 fetch 嗎？

A: GitFetcher 目前只負責 fetch。Clone 需要手動執行，這是為了避免意外覆蓋本地 repo。

## 測試

專案包含完整的單元測試，覆蓋率超過 85%。

### 執行測試

```bash
# 執行所有測試
make test

# 詳細輸出
make test-verbose

# 測試覆蓋率報告
make test-coverage

# Race detector (檢測並發問題)
make test-race

# 測試特定模組
make test-config
make test-fetcher
make test-scheduler
make test-web
```

### 測試覆蓋率

| 模組 | 覆蓋率 | 說明 |
|------|--------|------|
| config | 91.7% | 配置解析、驗證、YAML 操作 |
| fetcher | 85.0% | Git fetch 執行、日誌記錄 |
| scheduler | 93.4% | 定時任務、熱更新、並發控制 |
| web | 75.0% | HTTP handlers、API 端點 |

所有測試都設計為可獨立執行，不需要依賴實際的 Redmine 環境。

## 技術架構

- **語言**：Go 1.23
- **Web 框架**：Gin
- **配置解析**：go-yaml/v3
- **檔案監控**：fsnotify
- **容器**：Alpine Linux + Git + OpenSSH
- **測試**：Go testing framework + httptest

## 目錄結構

```
gitfetcher/
├── main.go              # 主程式入口
├── config/
│   └── config.go        # 配置管理
├── fetcher/
│   └── fetcher.go       # Git fetch 邏輯
├── scheduler/
│   └── scheduler.go     # 定時任務調度
├── web/
│   ├── handler.go       # HTTP handlers
│   └── templates/
│       └── index.html   # Web UI
├── Dockerfile
├── docker-compose.example.yml
├── config.example.yaml
└── README.md
```

## License

MIT

## 貢獻

歡迎提交 Issue 和 Pull Request！

---

**製作者**: GitFetcher v1.0.0
**適用場景**: Redmine Git 整合、自動化 repo 同步
**設計理念**: MVP + KISS - 簡單、可靠、易維護
