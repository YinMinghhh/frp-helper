# FRP STCP Visitor Helper

[English README](./README.md)

`frp-helper` 是一个跨平台 Go CLI，用来管理 FRP STCP 访问里的本地 `frpc visitor` 这一侧。

V1 聚焦一条最小可用链路：

1. 自动发现或导入 JSON 清单
2. 安装或复用 `frpc`
3. 生成 `frpc.toml`
4. 默认以后台方式启动 `frpc`
5. 验证本地监听端口并输出可直接使用的访问命令
6. 按需注册开机启动项

## 当前能力范围

- 支持 `windows/amd64`、`darwin/arm64`、`darwin/amd64`、`linux/amd64`、`linux/arm64`
- 支持在单个 TOML 文件中生成多个 `[[visitors]]`
- 支持通过 `run` 后台启动 `frpc`
- 支持通过 `start` 前台调试并直接看日志
- 支持 `run`，自动从当前目录或可执行文件同目录发现 `access.json`
- 支持开机启动开关
- 支持把二进制和真实配置一起打成组内分发包
- 持久化保存 manifest、生成后的配置、运行状态和日志
- 支持服务启用、禁用、删除，以及 manifest 的 merge/replace

V1 暂不包含：

- 系统服务安装
- 完整的守护进程 / 系统服务管理
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
./bin/frp-helper run -f ./access.json
```

`run` 会先应用 manifest，等本地监听就绪后打印访问命令，然后把终端还给你。

如果你想用前台方式排障看日志，可以执行：

```bash
./bin/frp-helper start
```

或者：

```bash
./bin/frp-helper run -f ./access.json --foreground
```

平时查看状态或停止：

```bash
export FRP_HELPER_HOME="$(pwd)/.frp-helper-dev"
./bin/frp-helper status
./bin/frp-helper endpoints
./bin/frp-helper startup status
./bin/frp-helper stop
```

如果你要打开开机启动，可以执行一次：

```bash
./bin/frp-helper startup enable
```

也可以在运行时一起打开：

```bash
./bin/frp-helper run -f ./access.json --enable-startup
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
./bin/frp-helper run [-f ./access.json] [--merge|--replace] [--enable-startup|--disable-startup] [--foreground]
./bin/frp-helper start
./bin/frp-helper stop
./bin/frp-helper restart
./bin/frp-helper status
./bin/frp-helper endpoints
./bin/frp-helper startup status
./bin/frp-helper startup enable
./bin/frp-helper startup disable
./bin/frp-helper package -f ./access.json -o ./dist/bundle [--startup=true]
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
4. 执行 `run`
6. 确认输出的访问命令符合预期，例如：

```bash
./bin/frp-helper run -f ./access.json --enable-startup
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

## 组内私下分发

如果你不希望终端用户手动敲 CLI 参数，也不想让他们再显式传 `-f access.json`，可以直接用 `package`：

```bash
./bin/frp-helper package -f ./access.json -o ./dist/macos-arm64
```

这个命令会生成一个私有分发目录，里面包含：

- `frp-helper` 可执行文件
- 打包进去的真实 `access.json`
- 平台对应的启动/停止脚本
- 一份简单的 bundle 说明

默认情况下，生成的启动脚本第一次运行时会顺手打开开机启动，而且会在后台拉起 `frpc` 后立即返回。这比让组员自己执行 `apply -f` 更适合组内分发。

## 与用户现有 FRP 配置是否冲突

`frp-helper` 使用自己的二进制、自己的生成配置、自己的状态文件和日志目录，不会去修改用户机器上其他 FRP 配置文件。

真正共享的风险主要是本地端口冲突：如果用户已有其他 FRP visitor 或其他程序占用了同一个 `bindPort`，`frp-helper` 会直接报错并拒绝启动，而不是覆盖掉对方配置。

## 默认 Visitor 模板在哪里改

当前没有单独的模板文件，默认 visitor 配置是代码渲染出来的，主要看这两个位置：

- `internal/model/render.go`：负责渲染 TOML 顶层字段和 `[[visitors]]`
- `internal/model/model.go`：负责默认常量，比如 `DefaultFRPCVersion` 和 `DefaultBindAddr`

如果你想改默认 `bindAddr`、visitor 输出字段、默认渲染规则，直接改这里即可。
