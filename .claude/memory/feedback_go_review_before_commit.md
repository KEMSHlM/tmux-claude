---
name: go-review-before-commit
description: コミット前に必ず /go-review を実行する
type: feedback
---

コミット前に必ず /go-review を実行すること。

**Why:** ユーザーが明示的に要求。レビューなしのコミットは許可されない。
**How to apply:** `git commit` を実行する前に、go-reviewer agent でコードレビューを行い、CRITICAL/HIGH がゼロであることを確認する。