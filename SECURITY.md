# 安全政策

## 支持范围

本仓提供 SMS provider adapter 和公共 SMS 服务实现。安全修复优先覆盖默认分支和最新发布版本。

## 报告方式

请不要在公开 issue 中提交漏洞细节、真实 API key、token、验证码、手机号、代理凭据、上游响应或可复用会话材料。

推荐使用 GitHub Security Advisory 私下报告；如果仓库尚未开启该功能，请通过组织维护者提供的私有渠道联系。

## 处理原则

- 测试样例只能使用虚构值。
- 日志、错误和测试数据不得包含真实验证码、手机号、token 或上游凭据。
- provider 原始响应和 webhook payload 只保留必要字段。
