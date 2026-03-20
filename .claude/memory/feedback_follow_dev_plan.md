---
name: follow-dev-plan
description: 開発計画の Phase 順序を飛ばさずに実装する
type: feedback
---

開発計画 (go-migration-plan.md) の Phase 順に実装すること。
Phase 内の未完了タスクがあるのに次の Phase に進まない。

**Why:** ユーザーが「次のフェーズ」と言った場合、現在の Phase の残タスクを先に完了する。
**How to apply:** Phase の進捗を確認し、未完了タスクがあればそちらを先に実装する。