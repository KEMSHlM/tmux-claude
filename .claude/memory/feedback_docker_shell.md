---
name: VHS Docker shell entry command
description: VHS Docker コンテナにシェルで入る正しいコマンドパターン
type: feedback
---

VHS Docker コンテナにシェルで入るには `entrypoint` を上書きせず、`run --rm vhs bash` を使う:

```bash
docker compose -p lazyclaude-e2e-$(git rev-parse --short HEAD) \
  -f vis_e2e_tests/docker-compose.ssh.yml down --volumes --remove-orphans 2>/dev/null

docker compose -p lazyclaude-e2e-$(git rev-parse --short HEAD) \
  -f vis_e2e_tests/docker-compose.ssh.yml build --no-cache

docker compose -p lazyclaude-e2e-$(git rev-parse --short HEAD) \
  -f vis_e2e_tests/docker-compose.ssh.yml run --rm vhs bash
```

**Why:** `--entrypoint /bin/bash-real` のような上書きは不要。`bash` コマンドを渡すだけでよい。`down --volumes --remove-orphans` でクリーンアップしてから。

**How to apply:** Docker 環境での手動テストやデバッグ時に使用。
