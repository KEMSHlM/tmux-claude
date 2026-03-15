# Claude Code Permission Dialog の Diff フォーマット再現調査

## 対象フォーマットの仕様

Claude Code の permission dialog に表示される diff フォーマットの特徴は以下の通り:

```
 Update(~/.local/share/tmux/plugins/tmux-claude/scripts/mcp-server.js)
  ⎿  Added 1 line, removed 1 line
      24  const os = require('node:os');
      25  const { execSync, spawnSync } = require('node:child_process');
      26
      27 -// --- constants ---
      27 +// --- constants --- (diff test)
      28
      29  const LOCK_DIR = path.join(os.homedir(), '.claude', 'ide');
      30  const PID_FILE = '/tmp/tmux-claude-mcp.pid';
```

| 要素 | 仕様 |
|------|------|
| 行番号 | 右揃え、固定幅（6桁程度） |
| unchanged 行 | `{linenum}  {code}` (2スペース) |
| 削除行 | `{linenum} -{code}` (スペース + ハイフン) |
| 追加行 | `{linenum} +{code}` (スペース + プラス、行番号は削除行と同じ) |
| コンテキスト | 変更前後3行 |
| ヘッダー | `Update(filepath)` + `⎿ Added N line, removed N line` |

削除行と追加行が**同じ行番号**を共有する点が特殊で、標準的な unified diff とは異なる。これは「この行番号の内容が置き換えられた」という表現方式。

---

## 1. delta による再現

### 概要

[delta](https://github.com/dandavison/delta) は git/diff 出力のシンタックスハイライトページャ。行番号は2カラム（旧ファイル番号 / 新ファイル番号）表示が標準。

### 行番号フォーマットオプション

| オプション | デフォルト値 | 説明 |
|-----------|------------|------|
| `--line-numbers-left-format` | `'{nm:^4}⋮'` | 左カラム（旧ファイル行番号） |
| `--line-numbers-right-format` | `'{np:^4}│'` | 右カラム（新ファイル行番号） |
| `--line-numbers-left-style` | - | 左カラムのスタイル |
| `--line-numbers-right-style` | - | 右カラムのスタイル |
| `--line-numbers-minus-style` | - | 削除行番号のスタイル |
| `--line-numbers-zero-style` | - | 変更なし行番号のスタイル |
| `--line-numbers-plus-style` | - | 追加行番号のスタイル |

### フォーマット文字列のプレースホルダ

- `{nm}`: 旧ファイル（minus）の行番号
- `{np}`: 新ファイル（plus）の行番号
- 整列: `<` 左寄せ、`^` 中央、`>` 右寄せ（Rust の string formatting 構文）

### Claude Code フォーマットへの近似設定

```gitconfig
[delta]
    line-numbers = true
    line-numbers-left-format = "{nm:>4} "
    line-numbers-right-format = ""
    line-numbers-minus-style = "red"
    line-numbers-plus-style = "green"
    line-numbers-zero-style = "normal"
```

**制約**: delta は左右2カラム構造が基本。削除行と追加行が同一行番号を示す Claude Code 方式（`27 -` / `27 +`）を delta で完全再現することは困難。削除行には `{nm}` のみ、追加行には `{np}` のみが表示されるため、番号が異なってしまう。

### 現実的な近似

delta の標準2カラム表示:

```
旧番号 ⋮ 新番号 │ unchanged line
  27  ⋮       │ - removed line
      ⋮  27   │ + added line
```

完全な一致ではないが視覚的に類似した効果は得られる。

---

## 2. 他の CLI diff ツール

### diff-so-fancy

[diff-so-fancy](https://github.com/so-fancy/diff-so-fancy) は git diff のヘッダーを整形するが、**行番号表示機能はない**。フォーマット再現には不向き。

### colordiff

[colordiff](https://www.colordiff.org/) は diff 出力に色を付けるラッパーだが、フォーマット変換機能はなく、行番号の付与もできない。

### showlinenum.awk

[showlinenum](https://github.com/jay/showlinenum) は git diff 出力に行番号を付加する gawk スクリプト。

出力フォーマット:
```
[path:]<line number>:<diff line>
```

- 追加・変更なし行: 新ファイルの行番号を表示
- 削除行: 行番号の代わりにスペース（パディング）
- 削除ファイルの行: `~` を表示

**制約**: Claude Code フォーマットとはレイアウトが大きく異なる。削除行に旧行番号を表示し、追加行と同一番号を共有する方式は実現できない。

### bat

`bat` は構文ハイライト付きファイルビューアだが diff モード (`bat --diff`) では標準的な unified diff を色付けするだけで行番号フォーマットのカスタマイズは限定的。

### 結論

既製ツールでは Claude Code フォーマットを完全に再現できない。カスタム実装が必要。

---

## 3. Node.js での実装

### npm `diff` パッケージ

[diff](https://www.npmjs.com/package/diff) (kpdecker/jsdiff) が最適。

#### structuredPatch API

```javascript
structuredPatch(oldFileName, newFileName, oldStr, newStr[, oldHeader[, newHeader[, options]]])
```

**戻り値の構造**:
```javascript
{
  oldFileName: 'path/to/old',
  newFileName: 'path/to/new',
  hunks: [{
    oldStart: 24,   // 旧ファイルの開始行番号
    oldLines: 7,    // 旧ファイルの行数
    newStart: 24,   // 新ファイルの開始行番号
    newLines: 7,    // 新ファイルの行数
    lines: [
      ' const os = require(...)',   // 先頭スペース = unchanged
      '-// --- constants ---',       // 先頭 - = removed
      '+// --- constants --- (diff)',// 先頭 + = added
    ]
  }]
}
```

**オプション**:
- `context`: コンテキスト行数（デフォルト 4、Claude Code は 3）

#### diffLines API

```javascript
diffLines(oldStr, newStr[, options])
```

**戻り値**: change オブジェクトの配列

```javascript
[
  { value: 'unchanged line\n', count: 1, added: false, removed: false },
  { value: 'old line\n',       count: 1, added: false, removed: true  },
  { value: 'new line\n',       count: 1, added: true,  removed: false },
]
```

### Claude Code フォーマットのレンダラ実装例

```javascript
const { structuredPatch } = require('diff');

function renderClaudeCodeDiff(oldStr, newStr, filePath) {
  const patch = structuredPatch('', '', oldStr, newStr, '', '', { context: 3 });

  // Added / removed 行数を集計
  let addedCount = 0;
  let removedCount = 0;
  for (const hunk of patch.hunks) {
    for (const line of hunk.lines) {
      if (line.startsWith('+')) addedCount++;
      else if (line.startsWith('-')) removedCount++;
    }
  }

  const header = `Update(${filePath})`;
  const summary = `  ⎿  Added ${addedCount} line${addedCount !== 1 ? 's' : ''}, removed ${removedCount} line${removedCount !== 1 ? 's' : ''}`;

  const lines = [header, summary];

  for (const hunk of patch.hunks) {
    let oldLine = hunk.oldStart;
    let newLine = hunk.newStart;

    for (const line of hunk.lines) {
      const type = line[0];    // ' ', '-', '+'
      const content = line.slice(1);
      const NUM_WIDTH = 6;

      if (type === ' ') {
        // unchanged: 行番号は旧=新で同一
        lines.push(`${String(oldLine).padStart(NUM_WIDTH)}  ${content}`);
        oldLine++;
        newLine++;
      } else if (type === '-') {
        // 削除行: 旧ファイルの行番号 + ' -'
        lines.push(`${String(oldLine).padStart(NUM_WIDTH)} -${content}`);
        oldLine++;
      } else if (type === '+') {
        // 追加行: 削除行と同じ行番号（newLine を使わない）を表示する場合は
        // oldLine - 1 を使うか、前の削除行の行番号を保持する
        // ここでは newLine を表示する（標準的な解釈）
        lines.push(`${String(newLine).padStart(NUM_WIDTH)} +${content}`);
        newLine++;
      }
    }
  }

  return lines.join('\n');
}
```

#### 削除行と追加行が同一行番号を共有する実装

Claude Code フォーマットでは `27 -` と `27 +` が同じ行番号を持つ。これを実現するには、削除行の行番号を追加行でも再利用する:

```javascript
} else if (type === '-') {
  lines.push(`${String(oldLine).padStart(NUM_WIDTH)} -${content}`);
  // oldLine を増やさず保持 → 次の '+' でも同じ番号を使う
  lastRemovedLine = oldLine;
  oldLine++;
} else if (type === '+') {
  // 直前の削除行と同じ行番号を共有
  const displayLine = lastRemovedLine !== null ? lastRemovedLine : newLine;
  lines.push(`${String(displayLine).padStart(NUM_WIDTH)} +${content}`);
  lastRemovedLine = null;
  newLine++;
}
```

---

## 4. git diff による近似

### 標準的な unified diff

```bash
git diff --unified=3
```

標準出力は `@@ -27,1 +27,1 @@` 形式のヘッダーを持ち、行番号は明示されない。

### カスタムフォーマットへの変換

git diff 単体では Claude Code フォーマットを出力できない。外部ツール（awk/delta）との組み合わせが必要。

#### delta を使った近似（.gitconfig）

```gitconfig
[core]
    pager = delta

[delta]
    line-numbers = true
    line-numbers-left-format = "{nm:>4} "
    line-numbers-right-format = ""
    syntax-theme = none
    file-style = omit
    hunk-header-style = omit
```

この設定では:
- 左カラムのみ表示（右カラム非表示）
- 右揃え行番号（4桁）
- ファイルヘッダー・hunkヘッダーを非表示

#### awk スクリプトによる変換

```bash
git diff --unified=3 | awk '
/^@@/ {
  # @@ -oldStart,oldLines +newStart,newLines @@ を解析
  match($0, /-([0-9]+)/, a); oldLine = a[1]
  match($0, /\+([0-9]+)/, b); newLine = b[1]
  next
}
/^-/ {
  printf "%6d -%s\n", oldLine++, substr($0, 2)
  next
}
/^\+/ {
  # Claude Code 方式: 直前の削除行番号を再利用（簡易版は newLine を使用）
  printf "%6d +%s\n", newLine++, substr($0, 2)
  next
}
/^ / {
  printf "%6d  %s\n", oldLine++, substr($0, 2)
  newLine++
  next
}
'
```

**制約**: `@@` ヘッダーの解析が複雑で、マルチハンクの diff では行番号がずれる可能性がある。showlinenum.awk のような既製スクリプトを使う方が安全。

---

## まとめ

| アプローチ | Claude Code フォーマットへの近似度 | 実装コスト |
|-----------|--------------------------------|-----------|
| delta（2カラム） | 中（行番号2分割が違う） | 低 |
| delta（左カラムのみ） | 中（削除/追加の同一番号共有が難しい） | 低 |
| diff-so-fancy | 低（行番号なし） | 低 |
| showlinenum.awk | 低（フォーマットが大きく異なる） | 低 |
| npm `diff` + カスタムレンダラ | 高（完全再現可能） | 中 |
| git diff + awk | 中（近似可能だが堅牢性に課題） | 中 |

**推奨**: Node.js での実装が必要な場合は `npm diff` の `structuredPatch` を使ったカスタムレンダラが最も忠実な再現を実現できる。ターミナルで手軽に確認したい場合は delta の2カラム表示が最も近い。

---

## 参考文献

- [dandavison/delta - GitHub](https://github.com/dandavison/delta)
- [delta 公式ドキュメント - Line Numbers](https://dandavison.github.io/delta/line-numbers.html)
- [delta full --help output](https://dandavison.github.io/delta/full---help-output.html)
- [kpdecker/jsdiff - GitHub](https://github.com/kpdecker/jsdiff)
- [diff package API documentation](https://npmdoc.github.io/node-npmdoc-diff/build/apidoc.html)
- [jay/showlinenum - GitHub](https://github.com/jay/showlinenum)
- [git-scm.com - diff-format documentation](https://git-scm.com/docs/diff-format)