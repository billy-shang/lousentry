# 漏哨 LouSentry

当前版本：**v1.0.01**

采集高价值漏洞，按周期检查并推送到钉钉 / 飞书 / 企业微信。

- 代码仓库：https://github.com/billy-shang/lousentry
- Docker 镜像：https://hub.docker.com/r/billyshang/lousentry

`漏哨` 的意思是：像哨兵一样盯着当下真正需要注意的漏洞，有新情况立刻报上来。英文名 `LouSentry`，二进制为 `lousentry`。

CVE 库里绝大多数编号并没有现实威胁。这个项目从部分高质量公开源抓取信息，过滤出高价值漏洞再推送，避免被各类 RSS 和公众号淹没。采集与推送逻辑源自开源项目 WatchVuln，当前版本增加了 Web 控制台，并把推送渠道收敛为国内常用的三种机器人。

当前抓取了这几个站点的数据:

| 名称                         | 地址                                                                                              | 推送策略                                             |
|----------------------------|-------------------------------------------------------------------------------------------------|--------------------------------------------------|
| 阿里云漏洞库                     | https://avd.aliyun.com/high-risk/list                                                           | 等级为高危或严重                                         |
| 长亭漏洞库                      | https://stack.chaitin.com/vuldb/index                                                           | 等级为高危或严重**并且**标题含中文                              |
| OSCS开源安全情报预警               | https://www.oscs1024.com/cm                                                                     | 等级为高危或严重**并且**包含 `预警` 标签                         |
| 奇安信威胁情报中心                  | https://ti.qianxin.com/                                                                         | 等级为高危严重**并且**包含 `奇安信CERT验证` `POC公开` `技术细节公布`标签之一 |
| 微步在线研究响应中心(公众号)            | https://x.threatbook.com/v5/vulIntelligence                                                     | 等级为高危或严重                                         |
| 知道创宇Seebug漏洞库              | https://www.seebug.org/                                                                         | 等级为高危或严重                                         |
| 启明星辰漏洞通告                   | https://www.venustech.com.cn/new_type/aqtg/                                                     | 等级为高危或严重                                         |
| CISA KEV                   | https://www.cisa.gov/known-exploited-vulnerabilities-catalog                                    | 全部推送                                             |
| Struts2 Security Bulletins | [Struts2 Security Bulletins](https://cwiki.apache.org/confluence/display/WW/Security+Bulletins) | 等级为高危或严重                                         |

> 所有信息来自网站公开页面, 如果有侵权，请提交 issue, 我会删除相关源。
>
> 如果有更好的信息源也可以反馈给我，需要能够响应及时 & 有办法过滤出有价值的漏洞

具体来说，消息的推送有两种情况, 两种情况有内置去重，不会重复推送:

- 新建的漏洞符合推送策略，直接推送,
- 新建的漏洞不符合推送策略，但漏洞信息被更新后符合了推送策略，也会被推送

## Web 控制台

从 `v2.8.0` 起内置 Web 控制台，默认监听 `:8080`。打开浏览器访问 `http://服务器IP:8080` 即可登录。


默认账号：

- 用户名：`admin`
- 密码：`admin123`（建议登录后立刻在「设置 → 账户安全」修改，也可在此添加更多登录账号）

界面与登录页统一为浅灰底、蓝色主色、白顶栏和白卡片。品牌锁头横排显示「漏哨 LouSentry」，中英字号相同。页脚显示 `漏哨 LouSentry v1.0.01` 和 GitHub 项目地址。

控制台功能：

| 页面 | 说明 |
| --- | --- |
| 首页 | 查看今日新增、已推送/未推送统计，检索漏洞（按入库时间倒序，今日新增在最前并带「今日」标记），查看详情，手动推送，发送每日报告；可看到下次检查时间并立刻检查 |
| 日志 | 查看采集与推送运行日志，可按级别筛选 |
| 设置 / 监控设置 | 调整检查周期、数据源、黑白名单、过滤策略、代理 |
| 设置 / 推送设置 | 配置钉钉、飞书、企业微信机器人，勾选或取消后需点击「保存配置」才生效 |
| 设置 / 账户安全 | 添加/删除登录账号（管理员 / 只读），修改当前账号密码 |

检查与自动推送：

1. **启动建库**：第一次启动会把各数据源当前页面的漏洞写入 SQLite，这一步**不推送**历史漏洞。
2. **周期检查**：建库完成后立刻做一轮增量检查，之后按「检查间隔」（默认 30 分钟）循环抓取。00:00-07:00 默认休眠，可在监控设置里关闭。
3. **自动告警**：某一轮检查到「新出现」或「等级/标签升级」的漏洞，且通过高价值/黑白名单等策略后，会**自动推送**到已启用的钉钉、飞书、企业微信 Webhook，并标记为已推送（同一 CVE 默认只推一次）。
4. **手动补推**：首页可以把未推送的漏洞再发一遍，或发送每日报告；这不影响自动检查。

相关环境变量 / 参数：

| 环境变量 | 命令行 | 说明 | 默认值 |
| --- | --- | --- | --- |
| `LISTEN` | `--listen` / `-l` | 控制台监听地址，设为 `off` 可关闭 | `:8080` |
| `ADMIN_USER` | `--admin-user` | 控制台用户名 | `admin` |
| `ADMIN_PASSWORD` | `--admin-pass` | 首次启动或重置密码时使用 | `admin123` |

登录账号、监控设置、钉钉/飞书/企业微信 webhook 都保存在 **SQLite**（与漏洞库同一数据库，默认 `data/vuln_v3.sqlite3`）：

- `web_users`：控制台登录账号和权限（`admin` / `readonly`）
- `web_settings`：监控设置、登录令牌密钥
- `web_pushers`：钉钉 / 飞书 / 企业微信的 webhook、加签 Secret、机器人 Key

控制台账号、监控设置和钉钉/飞书/企业微信 webhook 只写 SQLite，不再使用 `data/app.yaml`。无控制台启动时仍可用 `-c config.yaml`。未配置任何推送渠道时也可以先打开控制台，再在页面里补齐。

![登录页](image/001.png)

![首页漏洞列表](image/002.png)

![监控设置](image/003.png)

![推送设置](image/004.png)

## 如何启动

需要本机已安装 **Go 1.21+**、**Node.js 20+**。PowerShell 里不要把多条命令用 `&&` 连在一起。

### 日常使用（推荐）

仓库里已有前端产物时，在项目根目录执行：

```powershell
cd d:\CURSOR\LouSentry
.\lousentry.exe --listen :8080 --db-conn sqlite3://data/vuln_v3.sqlite3 --no-start-message --interval 30m
```

没有 `lousentry.exe` 时先编译再启动：

```powershell
cd d:\CURSOR\LouSentry\webui
npm install
npm run build
cd d:\CURSOR\LouSentry
go build -o lousentry.exe .
.\lousentry.exe --listen :8080 --db-conn sqlite3://data/vuln_v3.sqlite3 --no-start-message --interval 30m
```

浏览器打开 `http://127.0.0.1:8080`，默认账号 `admin` / `admin123`。钉钉 / 飞书 / 企业微信可在「设置 → 推送设置」里再配。

改过 `webui` 后必须重新 `npm run build`，再 `go build -o lousentry.exe .`，然后重启进程，最后 **Ctrl+F5**。

### 开发前端

开两个终端。后端：

```powershell
cd d:\CURSOR\LouSentry
go run . --listen :8080 --db-conn sqlite3://data/vuln_v3.sqlite3 --no-start-message --interval 30m
```

前端（Vite 会把 `/api` 代理到 8080）：

```powershell
cd d:\CURSOR\LouSentry\webui
npm install
npm run dev
```

开发时访问 Vite 给出的地址（一般是 `http://127.0.0.1:5173`）。

### 不编译、直接跑

```powershell
cd d:\CURSOR\LouSentry
go run . --listen :8080 --db-conn sqlite3://data/vuln_v3.sqlite3 --no-start-message --interval 30m
```

这种方式仍依赖已经构建好的 `webui/dist`（由 `//go:embed` 打进程序）。`dist` 不存在或过旧时，控制台页面会空白或还是旧界面。

## Docker 部署

镜像默认用 **SQLite**，数据库文件在容器内的 `/app/data/vuln_v3.sqlite3`。把这个目录挂成 Docker 卷后，重启、升级镜像都不会丢账号、监控配置、推送 webhook 和已采集的漏洞。

漏洞列表属于可重建数据，可以清空后重新采集；`web_users` / `web_settings` / `web_pushers` 会保留在同一份 SQLite 里，随数据卷一起持久化。

### 一条命令启动

```powershell
docker run -d --name lousentry --restart unless-stopped \
  -p 8083:8080 \
  -v /data/lousentry:/app/data \
  billyshang/lousentry:latest
```

或使用仓库里的 `docker-compose.yaml`：

```powershell
docker compose up -d
```

浏览器打开 `http://服务器IP:8083`，默认账号 `admin` / `admin123`。推送渠道在「设置 → 推送设置」里填写，会写入数据卷，不必写进镜像。勾选或取消渠道后要点「保存配置」才会生效。

## 本机路径

工程目录为 `D:\CURSOR\LouSentry`。截图在 `image/`，数据库在 `data/`（不进 Git）。

## v1.0.01

- 页脚展示版本号和 GitHub 项目地址
- 推送设置按钮改为「保存配置」，保存成功后顶部提示「配置已保存」
- 取消勾选渠道不会立刻生效，需再点「保存配置」


