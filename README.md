# 漏哨 LouSentry

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

界面与登录页统一为浅灰底、蓝色主色、白顶栏和白卡片。品牌锁头横排显示「漏哨 LouSentry」，中英字号相同。

控制台功能：

| 页面 | 说明 |
| --- | --- |
| 首页 | 查看今日新增、已推送/未推送统计，检索漏洞（按入库时间倒序，今日新增在最前并带「今日」标记），查看详情，手动推送，发送每日报告；可看到下次检查时间并立刻检查 |
| 日志 | 查看采集与推送运行日志，可按级别筛选 |
| 设置 / 监控设置 | 调整检查周期、数据源、黑白名单、过滤策略、代理 |
| 设置 / 推送设置 | 配置钉钉、飞书、企业微信机器人，并支持测试推送 |
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

## 如何启动

需要本机已安装 **Go 1.21+**、**Node.js 20+**。PowerShell 里不要把多条命令用 `&&` 连在一起。

### 日常使用（推荐）

仓库里已有前端产物时，在项目根目录执行：

```powershell
cd d:\CURSOR\watchvuln
.\lousentry.exe --listen :8080 --db-conn sqlite3://data/vuln_v3.sqlite3 --no-start-message --interval 30m
```

没有 `lousentry.exe` 时先编译再启动：

```powershell
cd d:\CURSOR\watchvuln\webui
npm install
npm run build
cd d:\CURSOR\watchvuln
go build -o lousentry.exe .
.\lousentry.exe --listen :8080 --db-conn sqlite3://data/vuln_v3.sqlite3 --no-start-message --interval 30m
```

浏览器打开 `http://127.0.0.1:8080`，默认账号 `admin` / `admin123`。钉钉 / 飞书 / 企业微信可在「设置 → 推送设置」里再配。

改过 `webui` 后必须重新 `npm run build`，再 `go build -o lousentry.exe .`，然后重启进程，最后 **Ctrl+F5**。

### 开发前端

开两个终端。后端：

```powershell
cd d:\CURSOR\watchvuln
go run . --listen :8080 --db-conn sqlite3://data/vuln_v3.sqlite3 --no-start-message --interval 30m
```

前端（Vite 会把 `/api` 代理到 8080）：

```powershell
cd d:\CURSOR\watchvuln\webui
npm install
npm run dev
```

开发时访问 Vite 给出的地址（一般是 `http://127.0.0.1:5173`）。

### 不编译、直接跑

```powershell
cd d:\CURSOR\watchvuln
go run . --listen :8080 --db-conn sqlite3://data/vuln_v3.sqlite3 --no-start-message --interval 30m
```

这种方式仍依赖已经构建好的 `webui/dist`（由 `//go:embed` 打进程序）。`dist` 不存在或过旧时，控制台页面会空白或还是旧界面。

## Docker 部署

镜像默认用 **SQLite**，数据库文件在容器内的 `/app/data/vuln_v3.sqlite3`。把这个目录挂成 Docker 卷后，重启、升级镜像都不会丢账号、监控配置、推送 webhook 和已采集的漏洞。

漏洞列表属于可重建数据，可以清空后重新采集；`web_users` / `web_settings` / `web_pushers` 会保留在同一份 SQLite 里，随数据卷一起持久化。

### 一条命令启动

```powershell
docker run -d --name lousentry --restart unless-stopped `
  -p 8080:8080 `
  -e DB_CONN=sqlite3://data/vuln_v3.sqlite3 `
  -e INTERVAL=30m `
  -e NO_START_MESSAGE=true `
  -v lousentry-data:/app/data `
  billyshang/lousentry:latest
```

或使用仓库里的 `docker-compose.yaml`：

```powershell
docker compose up -d
```

浏览器打开 `http://服务器IP:8080`，默认账号 `admin` / `admin123`。推送渠道在「设置 → 推送设置」里填写，会写入数据卷，不必写进镜像。

备份数据库：

```powershell
docker run --rm -v lousentry-data:/data -v ${PWD}:/backup alpine tar czf /backup/lousentry-data.tgz -C /data .
```

### 自己构建

```powershell
docker build -t lousentry:local .
```

## 快速使用

推送只支持这三种渠道：

- [钉钉群组机器人](https://open.dingtalk.com/document/robots/custom-robot-access)
- [微信企业版群组机器人](https://open.work.weixin.qq.com/help2/pc/14931)
- [飞书群组机器人](https://open.feishu.cn/document/ukTMukTMukTM/ucTM5YjL3ETO24yNxkjN)

### 使用 Docker

Docker 方式推荐使用环境变量来配置服务参数

| 环境变量名                   | 说明                                                                                | 默认值                                               |
|-------------------------|-----------------------------------------------------------------------------------|---------------------------------------------------|
| `DB_CONN`               | 数据库链接字符串，详情见 [数据库连接](#数据库连接)                                                      | `sqlite3://data/vuln_v3.sqlite3`                  |
| `DINGDING_ACCESS_TOKEN` | 钉钉机器人 url 的 `access_token` 部分                                                     |                                                   |
| `DINGDING_SECRET`       | 钉钉机器人的加签值 （仅支持加签方式）                                                               |                                                   |
| `LARK_ACCESS_TOKEN`     | 飞书机器人 url 的 `/open-apis/bot/v2/hook/` 后的部分, 也支持直接指定完整的 url 来访问私有部署的飞书             |                                                   |
| `LARK_SECRET`           | 飞书机器人的加签值 （仅支持加签方式）                                                               |                                                   |
| `WECHATWORK_KEY `       | 微信机器人 url 的 `key` 部分                                                              |                                                   |
| `SOURCES`               | 启用哪些漏洞信息源，逗号分隔：`avd`,`chaitin`,`ti`,`oscs`,`threatbook`,`seebug`,`struts2`,`kev`,`venustech` | `avd,chaitin,ti,oscs,threatbook,seebug,struts2,kev,venustech` |
| `INTERVAL`              | 检查周期，支持秒 `60s`, 分钟 `10m`, 小时 `1h`, 最低 `1m`                                        | `30m`                                             |
| `ENABLE_CVE_FILTER`     | 启用 CVE 过滤，开启后多个数据源的统一 CVE 将只推送一次                                                  | `true`                                            |
| `NO_FILTER`             | 禁用上述推送过滤策略，所有新发现的漏洞都会被推送                                                          | `false`                                           |
| `NO_START_MESSAGE`      | 禁用服务启动的提示信息                                                                       | `false`                                           |
| `WHITELIST_FILE`        | 指定推送漏洞的白名单列表文件, 详情见 [推送内容筛选](#推送内容筛选)                                             |                                                   |
| `BLACKLIST_FILE`        | 指定推送漏洞的黑名单列表文件, 详情见 [推送内容筛选](#推送内容筛选)                                             |                                                   |
| `DIFF`                  | 跳过初始化阶段，转而直接检查漏洞更新并推送                                                             |                                                   |
| `HTTPS_PROXY`           | 给所有请求配置代理, 详情见 [配置代理](#配置代理)                                                      |                                                   |
| `GO_SKIP_TLS_CHECK`     | 跳过 tls 校验，详情见 [配置代理](#配置代理)                                                       | `false`                                           |
| `NO_SLEEP`              | 禁用夜晚休眠，全天24小时无休！[其他](#其他)                                                         | `false`                                           |
| `LISTEN`                | Web 控制台监听地址，设为 `off` 关闭                                                                 | `:8080`                                           |
| `ADMIN_USER`            | 控制台用户名                                                                                      | `admin`                                           |
| `ADMIN_PASSWORD`        | 首次启动或重置控制台密码                                                                            | `admin123`                                        |

比如使用钉钉机器人

```bash
docker run --restart always -d \
  -e DINGDING_ACCESS_TOKEN=xxxx \
  -e DINGDING_SECRET=xxxx \
  -e INTERVAL=30m \
  -e ENABLE_CVE_FILTER=true \
  billyshang/lousentry:latest
```

当然，你可以仓靠使用本仓库的 `docker-compose.yaml` 文件，使用 `docker-compose` 来启动容器。

每次更新记得重新拉镜像:

```
docker pull billyshang/lousentry:latest
```


<details><summary>使用企业微信群组机器人</summary>

```bash
docker run --restart always -d \
  -e WECHATWORK_KEY=xxxx \
  -e INTERVAL=30m \
  billyshang/lousentry:latest
```

</details>

<details><summary>飞书群组机器人</summary>

```bash
docker run --restart always -d \
  -e LARK_ACCESS_TOKEN=xxxx \
  -e LARK_SECRET=xxxx \
  -e INTERVAL=30m \
  billyshang/lousentry:latest
```

</details>

<details><summary>使用多种服务</summary>

如果配置了多种服务的密钥，那么每个服务都会生效， 比如使用钉钉和企业微信:

```bash
docker run --restart always -d \
  -e DINGDING_ACCESS_TOKEN=xxxx \
  -e DINGDING_SECRET=xxxx \
  -e WECHATWORK_KEY=xxxx \
  -e INTERVAL=30m \
  billyshang/lousentry:latest
```

</details>


初次运行会在本地建立全量数据库，大约需要 1 分钟，可以使用 `docker logs -f [containerId]` 来查看进度,
完成后会在群内收到一个提示消息，表示服务已经在正常运行了。

### 使用二进制

前往 Release 下载对应平台的二进制，然后在命令行执行。命令行参数请参考 Docker 环境变量部分的说明，可以一一对应。

```bash
USAGE:
   lousentry [global options] command [command options] [arguments...]

GLOBAL OPTIONS:
   --config value, -c value  config file path, support json or yaml

   [Push Options]

   --blacklist-file value, --bf value         specify a file that contains some keywords, vulns with these products will NOT be pushed
   --dingding-access-token value, --dt value  webhook access token of dingding bot
   --dingding-sign-secret value, --ds value   sign secret of dingding bot
   --lark-access-token value, --lt value      webhook access token/url of lark
   --lark-sign-secret value, --ls value       sign secret of lark
   --wechatwork-key value, --wk value         webhook key of wechat work
   --whitelist-file value, --wf value         specify a file that contains some keywords, vulns with these keywords will be pushed

   [Launch Options]

   --db-conn value, --db value  database connection string (default: "sqlite3://vuln_v3.sqlite3")
   --diff                       skip init vuln db, push new vulns then exit (default: false)
   --enable-cve-filter          enable a filter that vulns from multiple sources with same cve id will be sent only once (default: true)
   --interval value, -i value   checking every [interval], supported format like 30s, 30m, 1h (default: "30m")
   --no-filter, --nf            ignore the valuable filter and push all discovered vulns (default: false)
   --no-github-search, --ng     don't search github repos and pull requests for every cve vuln (default: false)
   --no-sleep, --ns             don't sleep in night, run every interval (default: false)
   --no-start-message, --nm     disable the hello message when server starts (default: false)
   --proxy value, -x value      set request proxy, support socks5://xxx or http(s)://
   --sources value, -s value    set vuln sources (default: "avd,chaitin,ti,oscs,threatbook,seebug,struts2,kev,venustech")

   [Other Options]

   --debug, -d     set log level to debug, print more details (default: false)
   --help, -h      show help (default: false)
   --insecure, -k  allow insecure server connections when using SSL/TLS (default: false)
   --test, -T      use to test message pusher, three mocked messages will be pushed (default: false)
   --version, -v   print the version (default: false)
```

在参数中指定相关 Token 即可, 比如使用钉钉群组机器人

```
$ ./lousentry --dt DINGDING_ACCESS_TOKEN --ds DINGDING_SECRET -i 30m
```

<details><summary>使用企业微信群组机器人</summary>

```
$ ./lousentry --wk WECHATWORK_KEY -i 30m
```

</details>

<details><summary>使用飞书群组机器人</summary>

```bash
$ ./lousentry --lt LARK_ACCESS_TOKEN --ls LARK_SECRET -i 30m
```

</details>

<details><summary>使用多种服务</summary>

如果配置了多种服务的密钥，那么每个服务都会生效， 比如使用钉钉和企业微信:

```
$ ./lousentry --dt DINGDING_ACCESS_TOKEN --ds DINGDING_SECRET --wk WECHATWORK_KEY -i 30m
```

</details>

## 配置文件

进入查看详情 [使用配置文件](CONFIG.md)

## 数据库连接

默认使用 sqlite3 作为数据库，数据库文件为 `vuln_v3.sqlite3`，如果需要使用其他数据库，可以通过 `--db`
参数或是环境变量 `DB_CONN` 指定连接字符串，当前支持的数据库有:

- `sqlite3://filename`
- `mysql://user:pass@host:port/dbname`
- `postgres://user:pass@host:port/dbname`

注意：该项目不做数据向后兼容保证，版本升级可能存在数据不兼容的情况，如果报错需要删库重来。

## 配置代理

漏哨支持配置上游代理来绕过网络限制，支持两种方式:

- 环境变量 `HTTPS_PROXY`
- 命令行参数 `--proxy`/`-x`

支持 `socks5://xxxx` 或者 `http(s)://xxkx` 两种代理形式。

参数 `-k/--insecure` 或者环境变量 `GO_SKIP_TLS_CHECK=1` 可以禁用 tls 校验，即会设置 `InSecureSkipVerify` 为 `true`
，在抓包调试时会
比较有用。

## 推送内容筛选

如果你只想推送某些产品的漏洞，可以通过配置白名单或者黑名单来实现。这两个参数传入的都是一个文件，文件格式为每行一个产品名，比如:

```txt
Apache
泛微
```

温馨提示：如果你使用 `Docker` 来运行，可以通过挂载目录的方式将文件映射到容器内，比如:

```bash
echo "Apache" > whitelist.txt

docker run -v $(pwd):/config \
  -e WHITELIST_FILE=/config/whitelist.txt \
  -e xxxx=xxxxx
  billyshang/lousentry:latest
```

### 白名单过滤

通过命令行参数 `-wf` 或者环境变量 `WHITELIST_FILE` 来指定白名单文件。在发现新漏洞时，将检查漏洞的 **标题** 和 **描述**
是否包含白名单的任意一行，全都不在的将不推送漏洞。

### 黑名单过滤

通过命令行参数 `-bf` 或者环境变量 `BLACKLIST_FILE` 来指定黑名单文件。在发现新漏洞时，将检查漏洞的 **标题** 是否包含黑名单的任意一行，
包含的将不推送漏洞。为了避免非预期的漏掉推送，黑名单**不会**检查漏洞的 **描述** 是否匹配。

### 为什么xxx漏洞没有推送
1. 检查日志看看对应漏洞是否正常抓取到
2. 看看是否因为被判定为低价值漏洞而跳过了，会有 `not valuable` 字样
4. 看看是否有推送相关的报错
5. 如果都有没有，这个漏洞可能已经在漏洞库初始化时入库了，这种不会被判定为新漏洞自然也不会推送
6. 如果还有问题可以 issue 或者交流群提问

> 建议问问题之前先自行探索一下，避免问 docker 怎么看日志这种问题

## 其他

为了减少内卷，该工具在 00:00 到 07:00 间会去 sleep 不会运行, 请确保你的服务器是正确的时间！可在控制台勾选「夜间不休眠」或设置 `NO_SLEEP=true` 关闭该行为。

## 已知限制与后续改进

- 产品名已改为 **漏哨 / LouSentry**，二进制为 `lousentry`；本地目录名仍可以是 `watchvuln`，不影响运行。旧浏览器登录态会自动迁到 `lousentry-*` 键。
- 推送只保留钉钉、飞书、企业微信；旧配置里的 Telegram / Slack / Bark 等类型启动时会被忽略并打警告。
- 已删除一次性 `admin.json` 迁移和 `--data-dir` / `DATA_DIR`，账号只从 SQLite `web_users` 读取。
- 部分代理 / TLS 变更对已经创建的 HTTP 客户端立即生效程度有限，极端情况下需要重启进程。
- 漏洞列表的「来源」筛选按链接特征匹配，个别自定义来源可能显示为原始 URL。
- 控制台表格在窄屏会横向滚动，以保证 CVE、日期、状态不换行错位；多个标签会在单元格内换行显示。
- 账号、监控配置、钉钉/飞书/企业微信 webhook 都以 SQLite 为准（`web_users` / `web_settings` / `web_pushers`）。`data/app.yaml` 已废弃删除；命令行或 `-c` 指定的配置文件只在数据库尚未写入对应项时作为初始值。
- 登录页与控制台统一为浅底蓝主色：白顶栏、白卡片、蓝色强调；品牌同时显示「漏哨 / LouSentry」。
- 推送渠道取消勾选后会立刻清空并删除已保存的 webhook；账号可设为管理员或只读。
- 同时启用多个 Webhook 时，企业微信 / 钉钉支持粘贴完整 URL（会自动取出 key）；任一渠道成功即视为推送成功，失败渠道只记日志。
- 首页漏洞列表按入库时间从新到旧排列，同日再按披露时间；「今日新增」统计的是今天第一次写入库的漏洞，即使披露日期较早也会出现在列表顶部并带「今日」标记。
- 「更新时间」只在源站内容（标题、描述、等级、标签等）变化时改写；重复巡检同一条漏洞不再把时间刷成当前时刻。

扫码加我拉进讨论群，请备注申请理由为：问题反馈与讨论，否则不通过

<img src="https://github.com/user-attachments/assets/362d2079-4cfa-4764-819f-a4aa70580c1d" width="300" />
