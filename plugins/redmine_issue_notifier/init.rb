Redmine::Plugin.register :redmine_issue_notifier do
  name 'Redmine Issue Notifier'
  author 'CLCS Admin'
  description 'Notifies external services when issues are created or updated'
  version '1.0.0'
  url 'https://github.com/clcs/redmine_issue_notifier'
  author_url 'https://clcs.example.com'

  requires_redmine version_or_higher: '4.0.0'
end

require_relative 'lib/issue_notifier_hook'
