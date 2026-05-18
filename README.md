# sms

SMS provider 集成服务。

本仓库负责 SMS provider adapter、provider 私有契约、激活生命周期策略，以及公共 SMS gRPC 实现。

## 当前实现

- Go module：`github.com/byte-v-forge/sms`
- 公共契约 adapter：`internal/adapters/grpc`
- 核心生命周期服务：`internal/app`
- 领域模型和端口：`internal/core`
- 5Sim adapter：`internal/providers/fivesim`
- HeroSMS adapter：`internal/providers/herosms`
- SMSBower adapter：`internal/providers/smsbower`

provider 策略显式定义：

- HeroSMS 和 SMSBower：激活至少满 2 分钟后才允许取消，匹配 provider 的 early-cancel 行为。
- 公共激活对象暴露 `cancel_allowed_at`；调用方使用该字段表达 provider 专属时间规则。
- 默认本地激活 TTL 为 20 分钟，除非 route/provider policy 或请求 lease 更短。
- 5Sim 使用当前 Bearer-token REST API（`/v1/user/buy/activation`、`check`、`finish`、`cancel`、`profile`）；provider 返回订单过期时间时会保留该时间。
- SMSBower 普通购买使用 `getNumberV2`；当 route 需要文档中仅 `getNumber` 明确支持的 `phoneException` 或 `ref` 时，自动回退到 `getNumber`。

## 契约

- 公共 SMS 能力定义在本仓 `proto/byte/v/forge/contracts/sms/v1/`。
- provider 配置、上游 service key、上游 provider ID、provider country ID、原始上游响应、provider options 和 webhook payload 细节都是 SMS 内部模型。
- 共享内部 SMS 模型位于 `proto/byte/v/forge/sms/internal/v1/sms_internal.proto`。
- provider 专属 shape 放在独立 provider 目录，例如：
  - `proto/byte/v/forge/sms/providers/herosms/v1/herosms.proto`
  - `proto/byte/v/forge/sms/providers/smsbower/v1/smsbower.proto`
  - `proto/byte/v/forge/sms/providers/fivesim/v1/fivesim.proto`

编译检查：

```sh
protoc -I proto --descriptor_set_out=/tmp/sms-internal.pb \
  proto/byte/v/forge/contracts/sms/v1/sms.proto \
  proto/byte/v/forge/sms/internal/v1/sms_internal.proto \
  proto/byte/v/forge/sms/providers/fivesim/v1/fivesim.proto \
  proto/byte/v/forge/sms/providers/herosms/v1/herosms.proto \
  proto/byte/v/forge/sms/providers/smsbower/v1/smsbower.proto
```

生成 Go 代码：

```sh
sh scripts/generate-proto.sh
```

`gen/` 下的生成物随契约一起提交。

检查：

```sh
go vet ./...
```

## 贡献与安全

- 贡献规则见 `CONTRIBUTING.md`。
- 安全报告规则见 `SECURITY.md`。
- 本仓使用 Apache-2.0 许可证。

## 尚未实现

- PostgreSQL 持久化 adapter。
- 运行时配置加载和 secret 解析。
- 进程入口和 gRPC server bootstrap。
- 基于标准化 route/application mapping 的公共 catalog gRPC adapter。
