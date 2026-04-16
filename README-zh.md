# FRP STCP Visitor Helper

[English README](./README.md)

`frp-helper` 是一个跨平台 Go CLI，用来管理 FRP STCP 访问里的本地 `frpc visitor` 这一侧。

V1 聚焦一条最小可用链路：

1. 导入 JSON 清单
2. 安装或复用 `frpc`
3. 生成 `frpc.toml`
4. 前台启动 `frpc`
5. 验证本地监听端口并输出可直接使用的访问命令

## 当前能力范围

- 支持 `windows/amd64`、`darwin/arm64`、`darwin/amd64`、`linux/amd64`、`linux/arm64`
- 支持在单个 TOML 文件中生成多个 `[[visitors]]`
- 支持前台启动 `frpc`
- 持久化保存 manifest、生成后的配置、运行状态和日志
- 支持服务启用、禁用、删除，以及 manifest 的 merge/replace

V1 暂不包含：

- 系统服务安装
- 后台守护
- GUI
- `includes` 形式的分片配置
- 自动升级检查

## 编译

```bash
go build -o ./bin/frp-helper ./cmd/frp-helper
```

## 快速开始

开发时建议把运行数据落在项目目录下：

```bash
export FRP_HELPER_HOME="$(pwd)/.frp-helper-dev"
```

复制示例清单并修改：

```bash
cp ./examples/access.sample.json ./access.json
```

然后执行：

```bash
./bin/frp-helper apply -f ./access.json
./bin/frp-helper start
```

`start` 会保持前台运行。你可以在另一个终端里查看状态或停止：

```bash
export FRP_HELPER_HOME="$(pwd)/.frp-helper-dev"
./bin/frp-helper status
./bin/frp-helper endpoints
./bin/frp-helper stop
```

## Manifest 格式

顶层字段：

- `serverAddr`
- `serverPort`
- `authToken`
- `user`
- `services`

每个服务支持：

- `name`
- `serverName`
- `secretKey`
- `bindPort`
- `serverUser`
- `protocolHint`
- `disabled`
- `accessUser`

示例：

```json
{
  "serverAddr": "frps.example.com",
  "serverPort": 7000,
  "authToken": "YOUR_AUTH_TOKEN",
  "user": "ops",
  "services": [
    {
      "name": "ubuntu ssh",
      "serverName": "ubuntu_ssh",
      "secretKey": "YOUR_STCP_SECRET",
      "bindPort": 6000,
      "protocolHint": "ssh",
      "accessUser": "alice"
    }
  ]
}
```

## 命令列表

```bash
./bin/frp-helper help
./bin/frp-helper apply -f ./access.json [--merge|--replace]
./bin/frp-helper install [--version v0.68.0] [--archive /path/to/frp.tar.gz|--base-url URL]
./bin/frp-helper start
./bin/frp-helper stop
./bin/frp-helper restart
./bin/frp-helper status
./bin/frp-helper endpoints
./bin/frp-helper purge [--with-bin]
./bin/frp-helper service list
./bin/frp-helper service enable <service-key>
./bin/frp-helper service disable <service-key>
./bin/frp-helper service remove <service-key>
```

## 本地文件位置

默认情况下，程序会把数据存到当前平台的用户配置目录下。

开发时建议设置：

```bash
export FRP_HELPER_HOME="$(pwd)/.frp-helper-dev"
```

这样会在该目录下生成：

- `config/manifest.json`
- `config/frpc.toml`
- `state/runtime.json`
- `logs/frpc.log`
- `bin/frpc/<version>/frpc`

## 真实环境手测

1. 编译 CLI
2. 把 `examples/access.sample.json` 复制成 `access.json`
3. 用真实的 `serverAddr`、`authToken`、`serverName`、`secretKey` 替换示例值
4. 执行 `apply`
5. 执行 `start`
6. 确认输出的访问命令符合预期，例如：

```bash
ssh -p 6000 alice@127.0.0.1
```

如果你需要离线安装或者镜像源，可以用：

```bash
./bin/frp-helper install --archive /path/to/frp_0.68.0_darwin_arm64.tar.gz
```

或者：

```bash
./bin/frp-helper install --base-url https://your-mirror.example.com/releases/download
```

## 自动化测试

运行全部测试：

```bash
go test ./... -count=1
```

当前集成测试会自动构建并使用一个 stub `frpc`，不依赖真实 `frps` 服务。

## 默认 Visitor 模板在哪里改

当前没有单独的模板文件，默认 visitor 配置是代码渲染出来的，主要看这两个位置：

- `internal/model/render.go`：负责渲染 TOML 顶层字段和 `[[visitors]]`
- `internal/model/model.go`：负责默认常量，比如 `DefaultFRPCVersion` 和 `DefaultBindAddr`

如果你想改默认 `bindAddr`、visitor 输出字段、默认渲染规则，直接改这里即可。
