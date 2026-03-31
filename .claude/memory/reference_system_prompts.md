---
name: PM/Worker system prompt locations
description: lazyclaude の PM・Worker セッション用 system prompt テンプレートとビルド・注入の実装箇所
type: reference
---

## System Prompt テンプレート

- PM テンプレート: `internal/session/prompts/pm.md`
- Worker テンプレート: `internal/session/prompts/worker.md`

## ビルド・注入の実装

- `internal/session/role.go` — `//go:embed` でテンプレート埋め込み、`BuildPMPrompt()` / `BuildWorkerPrompt()` で動的値を展開
- `internal/session/manager.go` の `writeWorktreeLauncher()` — 一時スクリプトを生成し `claude --append-system-prompt` で注入
- `internal/session/worktree.go` の `BuildWorktreePrompt()` — PM/Worker でない通常 worktree 用の基本分離プロンプト

## セッション作成エントリポイント

- PM: `internal/gui/app_actions.go` `StartPMSession()` → `CreatePMSession()`
- Worker: `POST /msg/create` API (`internal/server/handler_msg.go`) → `CreateWorkerSession()`

## テスト

- `internal/session/role_test.go` — プロンプトビルドのテスト
- `internal/session/manager_test.go` — セッション作成・ロール割り当てのテスト
