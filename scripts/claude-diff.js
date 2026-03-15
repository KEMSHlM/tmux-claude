#!/usr/bin/env node
'use strict';

// claude-diff.js - Claude Code format diff viewer
// Args: old_file_path window
// new_file_contents passed via env: TMUX_CLAUDE_NEW_CONTENTS (base64)

const fs = require('fs');
const { execSync, spawnSync } = require('child_process');

const OLD_PATH = process.argv[2];
const WINDOW   = process.argv[3];
const newContents = Buffer.from(process.env.TMUX_CLAUDE_NEW_CONTENTS ?? '', 'base64').toString('utf8');

// Write new contents to temp
const tmpNew = `/tmp/tmux-claude-diff-${Date.now()}.tmp`;
fs.writeFileSync(tmpNew, newContents);

// Run git diff
let diffOutput = '';
try {
  execSync(`git diff --unified=3 --no-index -- ${JSON.stringify(OLD_PATH)} ${JSON.stringify(tmpNew)}`, { encoding: 'utf8' });
} catch (e) {
  diffOutput = e.stdout ?? '';
}

// Get bat syntax-highlighted lines
function getHighlightedLines(filePath, displayPath) {
  const args = ['--color=always', '--plain', '--paging=never'];
  if (displayPath && displayPath !== filePath) args.push('--file-name', displayPath);
  args.push(filePath);
  try {
    const r = spawnSync('bat', args, { encoding: 'utf8' });
    if (r.status === 0 && r.stdout) return r.stdout.split('\n');
  } catch {}
  try { return fs.readFileSync(filePath, 'utf8').split('\n'); } catch { return []; }
}

// Parse unified diff hunks
function parseDiff(text) {
  const hunks = [];
  let hunk = null;
  for (const line of text.split('\n')) {
    const m = line.match(/^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
    if (m) { hunk = { oldStart: +m[1], newStart: +m[2], lines: [] }; hunks.push(hunk); }
    else if (hunk && /^[ \-+]/.test(line)) hunk.lines.push(line);
  }
  return hunks;
}

// ANSI helpers
const A        = (c) => `\x1b[${c}m`;
const R        = A(0);
const DIM      = A(2);
const BOLD     = A(1);
const RESET_FG = A(39);             // fg のみリセット、bg は保持
const CYAN     = A('38;2;23;146;153');
const GREEN    = A('38;2;64;160;43');
const RED      = A('38;2;192;72;72');
const YELLOW   = A('38;2;223;142;29');
const LINE_ADD = A('38;2;80;200;80');   // #50C850 追加行の行番号色
const LINE_DEL = A('38;2;220;90;90');   // #DC5A5A 削除行の行番号色
const BG_RED   = A('48;2;61;1;0');      // #3D0100 削除行背景（行番号含む全行）
const BG_GREEN = A('48;2;2;40;0');      // #022800 追加行背景（行番号含む全行）
const BULLET   = '\u23fa'; // ⏺
const CORNER   = '\u23bf'; // ⎿

function stripAnsi(s) { return s.replace(/\x1b\[[0-9;]*m/g, ''); }

function buildLines(hunks, oldHighlighted, newHighlighted, COLS) {
  let added = 0, removed = 0;
  for (const h of hunks)
    for (const l of h.lines) { if (l[0] === '+') added++; if (l[0] === '-') removed++; }

  const out = [];
  out.push(` ${BULLET} ${BOLD}Update(${OLD_PATH})${R}`);
  out.push(`  ${CYAN}${CORNER}${R}  ${DIM}Added ${added} line${added !== 1 ? 's' : ''}, removed ${removed} line${removed !== 1 ? 's' : ''}${R}`);
  out.push('');

  const PAD = 6;

  for (const h of hunks) {
    let oldLine = h.oldStart, newLine = h.newStart, lastRemoved = null;

    for (const line of h.lines) {
      const type = line[0];
      const oldContent = (oldHighlighted[oldLine - 1] ?? line.slice(1)).replace(/\r?\n$/, '');
      const newContent = (newHighlighted[newLine - 1] ?? line.slice(1)).replace(/\r?\n$/, '');

      if (type === ' ') {
        out.push(`${DIM}${String(oldLine).padStart(PAD)}${R}  ${oldContent}`);
        oldLine++; newLine++; lastRemoved = null;
      } else if (type === '-') {
        // 削除行: プレーンテキスト（シンタックスハイライトなし）on 赤背景
        const raw = stripAnsi(oldContent);
        const fill = ' '.repeat(Math.max(0, COLS - PAD - 2 - raw.length));
        out.push(`${BG_RED}${LINE_DEL}${String(oldLine).padStart(PAD)} -${RESET_FG}${raw}${fill}${R}`);
        lastRemoved = oldLine; oldLine++;
      } else if (type === '+') {
        // 追加行: シンタックスハイライトあり on 緑背景
        // bat の \x1b[0m（全リセット）の後に BG_GREEN を再適用して背景を維持
        const highlighted = newContent.replace(/\x1b\[0m/g, `\x1b[0m${BG_GREEN}`);
        const visibleLen = stripAnsi(newContent).length;
        const fill = ' '.repeat(Math.max(0, COLS - PAD - 2 - visibleLen));
        const num = lastRemoved ?? newLine;
        out.push(`${BG_GREEN}${LINE_ADD}${String(num).padStart(PAD)} +${RESET_FG}${highlighted}${fill}${R}`);
        lastRemoved = null; newLine++;
      }
    }
    out.push('');
  }
  return out;
}

// CSI シーケンスの終端バイトか判定 (0x40-0x7E)
function isCSIFinal(code) { return code >= 0x40 && code <= 0x7e; }

// Interactive scrollable viewer — y/a/n bar always visible at bottom
function interactiveView(lines, COLS, ROWS) {
  const viewHeight = Math.max(1, ROWS - 4);
  const halfPage   = Math.max(1, Math.floor(viewHeight / 2));
  const maxScroll  = Math.max(0, lines.length - viewHeight);
  let scrollPos    = 0;
  let lastKey      = null; // gg 検出用

  // Alternate screen + hide cursor + mouse (SGR mode)
  process.stdout.write('\x1b[?1049h\x1b[?25l\x1b[?1000h\x1b[?1006h');

  function draw() {
    process.stdout.write('\x1b[H');
    const visible = lines.slice(scrollPos, scrollPos + viewHeight);
    for (const l of visible) process.stdout.write(l + '\x1b[K\n');
    for (let i = visible.length; i < viewHeight; i++) process.stdout.write('\x1b[K\n');

    const pct = maxScroll > 0 ? `  ${DIM}${Math.round(scrollPos / maxScroll * 100)}%${R}` : '';
    process.stdout.write('─'.repeat(COLS) + '\x1b[K\n');
    process.stdout.write(`  ${GREEN}${BOLD}y${R}  Yes        ${YELLOW}${BOLD}a${R}  Allow all in session        ${RED}${BOLD}n${R}  No${pct}\x1b[K\n`);
    process.stdout.write(`  ${BOLD}❯${R} \x1b[K`);
  }

  function cleanup() {
    process.stdout.write('\x1b[?1000l\x1b[?1006l\x1b[?25h\x1b[?1049l');
  }

  function scroll(delta) {
    scrollPos = Math.max(0, Math.min(maxScroll, scrollPos + delta));
    draw();
  }

  draw();

  return new Promise((resolve) => {
    process.stdin.setRawMode(true);
    process.stdin.resume();

    let buf = '';
    process.stdin.on('data', (chunk) => {
      buf += chunk.toString();

      while (buf.length > 0) {
        if (buf[0] === '\x1b') {
          if (buf.length < 2) break;

          if (buf[1] === '[') {
            // CSI シーケンス — 終端バイトまで読む
            let i = 2;
            while (i < buf.length && !isCSIFinal(buf.charCodeAt(i))) i++;
            if (i >= buf.length) break; // 未完、続きを待つ
            const seq = buf.slice(0, i + 1);
            buf = buf.slice(i + 1);

            if      (seq === '\x1b[A')  { scroll(-1); }
            else if (seq === '\x1b[B')  { scroll(1); }
            else if (seq === '\x1b[5~') { scroll(-viewHeight); }
            else if (seq === '\x1b[6~') { scroll(viewHeight); }
            else if (seq.startsWith('\x1b[<')) {
              // SGR マウスイベント
              const btn = parseInt(seq.slice(3));
              if      (btn === 64) { scroll(-3); } // ホイール上
              else if (btn === 65) { scroll(3);  } // ホイール下
            }
            lastKey = null;
          } else {
            buf = buf.slice(2); // その他の ESC シーケンスをスキップ
            lastKey = null;
          }
        } else {
          const ch = buf[0];
          buf = buf.slice(1);

          if      (ch === 'y' || ch === 'Y') { cleanup(); done(resolve, '1'); return; }
          else if (ch === 'a' || ch === 'A') { cleanup(); done(resolve, '2'); return; }
          else if (ch === 'n' || ch === 'N' || ch === '\x03') { cleanup(); done(resolve, '3'); return; }
          else if (ch === 'j' || ch === '\x0e') { scroll(1); lastKey = ch; }          // j / Ctrl+N
          else if (ch === 'k' || ch === '\x10') { scroll(-1); lastKey = ch; }          // k / Ctrl+P
          else if (ch === 'd' || ch === '\x04') { scroll(halfPage); lastKey = ch; }    // d / Ctrl+D
          else if (ch === 'u' || ch === '\x15') { scroll(-halfPage); lastKey = ch; }   // u / Ctrl+U
          else if (ch === 'f' || ch === '\x06') { scroll(viewHeight); lastKey = ch; }  // f / Ctrl+F
          else if (ch === 'b' || ch === '\x02') { scroll(-viewHeight); lastKey = ch; } // b / Ctrl+B
          else if (ch === 'G') { scrollPos = maxScroll; draw(); lastKey = 'G'; }       // G → 末尾
          else if (ch === 'g') {
            if (lastKey === 'g') { scrollPos = 0; draw(); lastKey = null; }            // gg → 先頭
            else { lastKey = 'g'; }
          }
          else { lastKey = ch; }
        }
      }
    });
  });
}

function done(resolve, choice) {
  process.stdin.setRawMode(false);
  process.stdout.write('\n');
  resolve(choice);
}

// --- Main ---
const COLS = process.stdout.columns || Number(process.env.COLUMNS) || 80;
const ROWS = process.stdout.rows    || Number(process.env.LINES)   || 24;

const oldHighlighted = getHighlightedLines(OLD_PATH, OLD_PATH);
const newHighlighted = getHighlightedLines(tmpNew, OLD_PATH);
try { fs.unlinkSync(tmpNew); } catch { /* ok */ }

const hunks = parseDiff(diffOutput);

(async () => {
  let choice;
  if (hunks.length === 0) {
    process.stdout.write(`${DIM}(no changes)${R}\n`);
    process.stdout.write('─'.repeat(COLS) + '\n');
    process.stdout.write(`  ${GREEN}${BOLD}y${R}  Yes        ${YELLOW}${BOLD}a${R}  Allow all in session        ${RED}${BOLD}n${R}  No\n`);
    process.stdout.write(`  ${BOLD}❯${R} `);
    process.stdin.setRawMode(true);
    process.stdin.resume();
    choice = await new Promise(resolve =>
      process.stdin.once('data', key => {
        const ch = key.toString();
        done(resolve, ch === 'y' || ch === 'Y' ? '1' : ch === 'a' || ch === 'A' ? '2' : '3');
      })
    );
  } else {
    const diffLines = buildLines(hunks, oldHighlighted, newHighlighted, COLS);
    choice = await interactiveView(diffLines, COLS, ROWS);
  }

  if (WINDOW) spawnSync('tmux', ['send-keys', '-t', `claude:=${WINDOW}`, choice, 'Enter']);
  process.exit(0);
})();