# Fork PR 检查的权限边界

PR #253 的 Actions 运行 39142324640 在 checkout 阶段失败：原 `pull.yml` 由 `pull_request_target` 触发，却检出 fork 的 head SHA。这个事件运行在上游仓库的可信上下文内，GitHub 因此拒绝检出不可信 PR 代码。

修复分为两个工作流：

- `pull.yml` 使用 `pull_request`，只授予 `contents: read`。默认检出 PR merge commit，关闭凭据持久化，再执行依赖准备与 Go lint。先 checkout 再 setup-go，避免缓存恢复时找不到依赖文件。checkout/setup-go 使用 Node.js 24 对应版本。
- `pull-policy.yml` 使用 `pull_request_target`，只授予关闭 PR 所需的 `pull-requests: write`。它只检查标题并执行原有关闭规则，不 checkout、不运行 fork 代码。监听 `edited`，使标题修改后也能重新执行规则。

不要通过 `allow-unsafe-pr-checkout: true` 绕过这个错误，也不要为 lint 作业授予写入权限或传入上游 secrets。

`pull_request_target` 使用目标分支里的工作流定义。更新 fork 的 PR 分支不会重写历史失败运行，也不会直接替换上游 main 的旧定义；上游维护者需要先将这个工作流修复应用到目标分支，旧的可信事件检查才会停止使用错误配置。

验证：

```sh
actionlint .github/workflows/pull.yml .github/workflows/pull-policy.yml
golangci-lint run --new-from-rev=origin/main ./cmd/music-dl ./core ./internal/... ./winres/...
```

失败页面：https://github.com/guohuiyuan/go-music-dl/actions/runs/39142324640
事件语义：https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows
