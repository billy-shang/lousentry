# 配置文件

从 `v2.0.0` 起可以用 `-c` 指定 yaml / json 配置文件：

```
./lousentry -c /path/to/config.yaml
./lousentry -c /path/to/config.json
```

约定：**指定配置文件后，命令行里的监控/推送参数不再生效**。如果 SQLite 里已经写过监控或推送配置，启动时仍以数据库为准，`-c` 只在首次写入时作为种子。

## 文件格式

一般把 `config.example.yaml` 改一下即可。最简示例：

```yaml
db_conn: sqlite3://vuln_v3.sqlite3
sources: [ "avd", "chaitin", "ti", "oscs", "threatbook", "seebug", "struts2", "kev", "venustech" ]
interval: 30m
listen: ":8080"
no_filter: false
no_sleep: false
pusher:
  - type: dingding
    access_token: "xxxx"
    sign_secret: "yyyy"
```

推送只支持三种：`dingding`、`lark`、`wechatwork`。使用 Web 控制台时，webhook 保存在 SQLite 的 `web_pushers` 表，不会回写 yaml。

多个钉钉群可以写多条同类型配置：

```yaml
pusher:
  - type: dingding
    access_token: "xxxx"
    sign_secret: "yyyy"
  - type: dingding
    access_token: "pppp"
    sign_secret: "qqqq"
```
