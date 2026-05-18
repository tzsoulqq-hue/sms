# 贡献指南

## 边界

本仓只承载 SMS 业务能力、provider adapter、业务内部 proto 和 SMS 服务实现。

以下内容不进入本仓：

- 公共 SMS 契约源文件；
- 其他业务域代码；
- 真实 provider 凭据、真实手机号或真实验证码；
- 生成代码。

## 开发流程

1. 公共能力在本仓 `proto/byte/v/forge/contracts/sms/v1/` 修改，并同步生成到 `gen/`。
2. provider 私有 shape 放在 `proto/byte/v/forge/sms/providers/<provider>/v1/`。
3. 业务内部模型放在 `proto/byte/v/forge/sms/internal/v1/`。
4. 外部 provider 调用必须设置超时，并按 provider 文档实现状态和错误映射。

## 验证

```sh
sh scripts/generate-proto.sh
go vet ./...
```

`gen/` 承载本仓契约生成物，随契约一起提交。
