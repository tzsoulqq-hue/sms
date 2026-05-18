# AGENTS.md

- 本仓库只承载 SMS 业务能力、provider adapter、业务内部契约和 SMS 服务实现。
- 公共契约来自 `contracts`；provider 私有 shape 留在本仓库内部 proto。
- 后端优先使用 Go，按 Clean Code、DI 和面向抽象设计组织代码。
- `gen/` 承载本仓 proto 生成物，随契约一起提交；检查报告、临时二进制和其他构建产物不提交。
- proto 变更后必须运行生成命令、格式化和 Go 检查。
