require 'net/http'
require 'uri'
require 'json'

class IssueNotifierHook < Redmine::Hook::Listener
  # Hook 會在 issue 儲存後觸發（包括建立和更新）
  def controller_issues_edit_after_save(context = {})
    issue = context[:issue]

    # 讀取配置（從環境變數或設定檔）
    webhook_url = ENV['GITHUB_SYNC_WEBHOOK_URL'] || 'http://github-sync:8080/webhook/issue-changed'
    webhook_secret = ENV['GITHUB_SYNC_WEBHOOK_SECRET'] || ''

    # 取得 target_repo_id（從環境變數或使用預設值）
    # 注意：在實際部署時，這個 ID 應該從配置中讀取
    target_repo_field_id = ENV['REDMINE_TARGET_REPO_FIELD_ID']&.to_i

    # 如果沒設定 field ID，記錄警告並跳過
    if target_repo_field_id.nil?
      Rails.logger.warn "[IssueNotifier] REDMINE_TARGET_REPO_FIELD_ID not set, skipping notification"
      return
    end

    # 檢查 issue 是否有設定「目標 GitHub Repo」
    target_repo = issue.custom_field_value(target_repo_field_id)

    # 如果沒有設定目標 repo，不需要通知
    return if target_repo.blank?

    Rails.logger.info "[IssueNotifier] Issue ##{issue.id} has target repo: #{target_repo}, notifying github-sync..."

    # 非同步發送通知（避免阻塞 Redmine 請求）
    Thread.new do
      begin
        uri = URI(webhook_url)

        # 準備 payload
        payload = {
          issue_id: issue.id,
          project_identifier: issue.project.identifier,
          target_repo: target_repo,
          action: context[:action] || 'updated',
          timestamp: Time.now.utc.iso8601
        }

        # 建立 HTTP request
        request = Net::HTTP::Post.new(uri.path, {'Content-Type' => 'application/json'})
        request.body = payload.to_json

        # 如果有設定 secret，加上簽章
        if webhook_secret.present?
          signature = OpenSSL::HMAC.hexdigest('SHA256', webhook_secret, request.body)
          request['X-Webhook-Signature'] = "sha256=#{signature}"
        end

        # 發送請求（timeout 5 秒）
        response = Net::HTTP.start(uri.hostname, uri.port, use_ssl: uri.scheme == 'https',
                                   open_timeout: 5, read_timeout: 5) do |http|
          http.request(request)
        end

        if response.is_a?(Net::HTTPSuccess)
          Rails.logger.info "[IssueNotifier] Successfully notified github-sync for issue ##{issue.id}"
        else
          Rails.logger.error "[IssueNotifier] Failed to notify github-sync: #{response.code} #{response.message}"
        end

      rescue => e
        Rails.logger.error "[IssueNotifier] Error notifying github-sync: #{e.class} - #{e.message}"
        Rails.logger.error e.backtrace.join("\n") if Rails.env.development?
      end
    end
  end

  # 也可以監聽 bulk edit
  def controller_issues_bulk_edit_before_save(context = {})
    # Bulk edit 會對多個 issues 操作
    # 這裡我們先不處理，因為 edit_after_save 會對每個 issue 都觸發
    # 如果需要優化，可以在這裡批次處理
  end
end
