# --------------------------------------------------------
# 1. 安裝外掛所需的工具 (git)
# --------------------------------------------------------
    USER root

    RUN apt-get update && \
        apt-get install -y --no-install-recommends git && \
        rm -rf /var/lib/apt/lists/*
    
    USER redmine
    
    # --------------------------------------------------------
    # 2. 安裝外掛
    # --------------------------------------------------------
    RUN \
      cd /usr/src/redmine/plugins && \
      \
      # 1. 安裝 Mermaid 外掛
      git clone https://github.com/taikii/redmine_mermaid_macro.git redmine_mermaid_macro && \
      \
    # 2. 安裝 Checklist (清單) 外掛 - 使用正確的 repository
    # 目前沒有 5/6 可支援的 checklist 插件 
    
    
    # --------------------------------------------------------
    # 3. 安裝外掛的相依套件
    # --------------------------------------------------------
    WORKDIR /usr/src/redmine
    
    RUN bundle install --without development test
    
    # --------------------------------------------------------
    # 4. 建立 repositories 目錄（給 redmine_github 使用）
    # --------------------------------------------------------
    USER root
    RUN mkdir -p /usr/src/redmine/repositories && \
        chown -R redmine:redmine /usr/src/redmine/repositories
    USER redmine
    "Dockerfile" 43L, 1447B                                                                                               43,12         Bot