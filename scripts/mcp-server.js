#!/usr/bin/env node
'use strict';

/**
 * tmux-claude MCP server
 *
 * Persistent WebSocket MCP server that Claude CLI connects to.
 * Started once from .zshrc via tmux-claude.zsh.
 *
 * Detects openDiff calls (file writes) and shows a tmux popup
 * so the user can review/interact with Claude.
 *
 * Runtime files:
 *   /tmp/tmux-claude-mcp.pid   - server PID
 *   /tmp/tmux-claude-mcp.port  - listening port
 *   /tmp/tmux-claude-mcp.token - auth token
 *   ~/.claude/ide/<port>.lock  - Claude discovery lock file
 */

const net = require('node:net');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const { exec, spawn } = require('node:child_process');
const { promisify } = require('node:util');

// --- async exec helpers (Phase 1.1) ---

const execPromise = promisify(exec);

/**
 * Run a shell command asynchronously with a timeout.
 * Returns stdout string on success, null on error or timeout.
 */
async function execAsync(cmd, timeoutMs = 3000) {
  try {
    const { stdout } = await execPromise(cmd, { encoding: 'utf8', timeout: timeoutMs });
    return stdout;
  } catch {
    return null;
  }
}

/**
 * Run a shell command asynchronously; returns trimmed stdout or null.
 */
async function execQuiet(cmd, timeoutMs = 3000) {
  const out = await execAsync(cmd, timeoutMs);
  return out === null ? null : out.trim();
}

// --- constants ---

const LOCK_DIR = path.join(os.homedir(), '.claude', 'ide');
const PID_FILE = '/tmp/tmux-claude-mcp.pid';
const PORT_FILE = '/tmp/tmux-claude-mcp.port';
const TOKEN_FILE = '/tmp/tmux-claude-mcp.token';
const WS_MAGIC = '258EAFA5-E914-47DA-95CA-C5AB0DC85B11';

// 再起動をまたいで token を保持（同じ token で Claude Code が自動再接続できる）
const AUTH_TOKEN = process.env.TMUX_CLAUDE_TOKEN || (() => {
  try { return fs.readFileSync(TOKEN_FILE, 'utf8').trim(); } catch {}
  return crypto.randomUUID();
})();

// シェルのシングルクォートエスケープ（-E オプション等でシェル経由で実行される文字列に使用）
function shellQuote(s) {
  return "'" + String(s).replace(/'/g, "'\\''") + "'";
}

// diff popup の選択を permission dialog の send-keys に渡すための一時保管
const pendingDiffChoices = new Map(); // window → choice ('1'|'2'|'3')

// PreToolUse hook から受け取ったツール情報（Notification 時に popup に渡す）
const pendingToolInfo = new Map(); // window → {tool_name, tool_input, ts}

// 期限切れエントリを定期削除（メモリリーク防止）
setInterval(() => {
  const cutoff = Date.now() - 15000;
  for (const [key, val] of pendingToolInfo) {
    if (val.ts < cutoff) pendingToolInfo.delete(key);
  }
}, 60_000).unref();

// --- WebSocket helpers ---

function wsAccept(key) {
  return crypto.createHash('sha1').update(key + WS_MAGIC).digest('base64');
}

function send101(socket, key) {
  socket.write(
    'HTTP/1.1 101 Switching Protocols\r\n' +
    'Upgrade: websocket\r\n' +
    'Connection: Upgrade\r\n' +
    `Sec-WebSocket-Accept: ${wsAccept(key)}\r\n` +
    '\r\n',
  );
}

function sendHttpError(socket, status, msg) {
  socket.end(`HTTP/1.1 ${status} ${msg}\r\n\r\n`);
}

function sendText(socket, text) {
  const payload = Buffer.from(text, 'utf8');
  let header;
  if (payload.length < 126) {
    header = Buffer.from([0x81, payload.length]);
  } else if (payload.length < 65536) {
    header = Buffer.allocUnsafe(4);
    header[0] = 0x81;
    header[1] = 126;
    header.writeUInt16BE(payload.length, 2);
  } else {
    header = Buffer.allocUnsafe(10);
    header[0] = 0x81;
    header[1] = 127;
    header.writeBigUInt64BE(BigInt(payload.length), 2);
  }
  socket.write(Buffer.concat([header, payload]));
}

function sendPong(socket, payload) {
  const header = Buffer.from([0x8a, payload.length]);
  socket.write(Buffer.concat([header, payload]));
}

function parseFrame(buf) {
  if (buf.length < 2) return null;

  const masked = (buf[1] & 0x80) !== 0;
  let len = buf[1] & 0x7f;
  let offset = 2;

  if (len === 126) {
    if (buf.length < 4) return null;
    len = buf.readUInt16BE(2);
    offset = 4;
  } else if (len === 127) {
    if (buf.length < 10) return null;
    len = Number(buf.readBigUInt64BE(2));
    offset = 10;
  }

  const total = offset + (masked ? 4 : 0) + len;
  if (buf.length < total) return null;

  let payload;
  if (masked) {
    const mask = buf.slice(offset, offset + 4);
    offset += 4;
    payload = Buffer.allocUnsafe(len);
    for (let i = 0; i < len; i++) payload[i] = buf[offset + i] ^ mask[i % 4];
  } else {
    payload = buf.slice(offset, offset + len);
  }

  return { opcode: buf[0] & 0x0f, payload, consumed: total };
}

// --- JSON-RPC helper ---

function reply(socket, id, result) {
  sendText(socket, JSON.stringify({ jsonrpc: '2.0', id, result }));
}

// --- Per-connection state ---

const socketState = new Map(); // socket → { pid: number | null }
const pidToWindow = new Map(); // pid → window name
// Per-socket message queue: ensures handleMcpMessage calls are sequential per connection.
// Without this, fire-and-forget async calls could process ide_connected and openDiff out of order.
const socketMsgQueue = new Map(); // socket → Promise chain

// --- Popup ---

async function findActiveClient() {
  // タブ区切りでパース（セッション名にスペースが含まれる場合を考慮）
  const out = await execAsync('tmux list-clients -F "#{client_name}\t#{client_session}\t#{client_activity}"');
  if (!out) return null;
  const clients = out.trim().split('\n')
    .filter(Boolean)
    .map(l => { const [name, sess, act] = l.split('\t'); return { name, sess, activity: Number(act) }; })
    .sort((a, b) => b.activity - a.activity);
  // claude セッションのクライアントを優先（Claude Code が動いているセッション）
  return (clients.find(c => c.sess === 'claude') ?? clients[0])?.name ?? null;
}

async function findTmuxWindowForPid(pid) {
  const paneMap = new Map();
  const paneOut = await execAsync('tmux list-panes -a -F "#{pane_pid}\t#{session_name}\t#{window_name}"');
  if (!paneOut) return null;
  for (const line of paneOut.trim().split('\n').filter(Boolean)) {
    const [panePid, session, window] = line.split('\t');
    paneMap.set(panePid, { session, window });
  }

  let current = String(pid);
  for (let i = 0; i < 15; i++) {
    if (paneMap.has(current)) return paneMap.get(current);
    // Validate current is numeric before passing to shell (injection prevention)
    if (!/^\d+$/.test(current)) break;
    const ppid = await execQuiet(`ps -o ppid= -p ${current} 2>/dev/null`);
    if (!ppid || ppid === '1' || ppid === '0' || ppid === current) break;
    current = ppid;
  }
  return null;
}

// PID から tmux window 名を解決（WebSocket 接続なしでも動作する）
async function resolveWindow(rawPid) {
  // 1. pidToWindow（WebSocket ide_connected で登録済み）を優先
  let current = rawPid;
  for (let i = 0; i < 15; i++) {
    if (pidToWindow.has(current)) return pidToWindow.get(current);
    // Validate current is numeric before passing to shell (injection prevention)
    if (!/^\d+$/.test(String(current))) break;
    const ppid = await execQuiet(`ps -o ppid= -p ${current} 2>/dev/null`);
    if (!ppid || ppid === '1' || ppid === '0' || ppid === String(current)) break;
    current = Number(ppid);
  }
  // 2. フォールバック: tmux ペインを直接スキャン
  const info = await findTmuxWindowForPid(rawPid);
  return info?.session === 'claude' ? info.window : null;
}

// Permission dialog の選択肢数を capture-pane の出力から検出
// Claude Code のダイアログは "1." "2." ... のように番号付きで表示される
function detectMaxOption(paneContent) {
  let max = 0;
  for (const line of paneContent.split('\n')) {
    const m = line.match(/^\s*(?:[❯>]\s+)?(\d+)[.)]/);
    if (m) max = Math.max(max, Number(m[1]));
  }
  return max > 0 ? max : 3; // 検出失敗時は 3 をデフォルト（Edit/Write 等）
}

// ツール名・入力から popup サイズ (%単位) を推定
function estimateToolPopupSize(toolName, toolInput, termW, termH, hasCwd = false, cwdLen = 0) {
  let lines = 1; // ヘッダー行（tool name）
  // アクションバー最低幅: "  Yes: y  |  Allow always: a  |  No: n  |  cancel: Esc" ≈ 54 chars
  let maxLen = 54;

  const clampLines = (arr, limit) => Math.min(arr.length, limit);
  const maxLineLen = (arr) => arr.reduce((m, l) => Math.max(m, l.length), 0);

  switch (toolName) {
    case 'Bash': {
      const cmd = (toolInput.command ?? '').split('\n');
      lines += clampLines(cmd, 20);
      maxLen = Math.max(maxLen, maxLineLen(cmd.slice(0, 20)));
      break;
    }
    case 'Read':
      lines += 1;
      maxLen = Math.max(maxLen, (toolInput.file_path ?? '').length);
      break;
    case 'Write':
      lines += 2;
      maxLen = Math.max(maxLen, (toolInput.file_path ?? '').length);
      break;
    case 'Edit': {
      const fp = toolInput.file_path ?? '';
      const old = (toolInput.old_string ?? '').split('\n').slice(0, 5);
      const nw  = (toolInput.new_string ?? '').split('\n').slice(0, 5);
      lines += 1 + 1 + old.length + 1 + nw.length;
      maxLen = Math.max(maxLen, fp.length, maxLineLen(old), maxLineLen(nw));
      break;
    }
    case 'Agent':
    case 'Task': {
      const prompt = (toolInput.prompt ?? toolInput.description ?? '').split('\n').slice(0, 10);
      lines += prompt.length;
      maxLen = Math.max(maxLen, maxLineLen(prompt));
      break;
    }
    default: {
      const entries = Object.entries(toolInput ?? {}).slice(0, 8);
      lines += entries.length;
      for (const [k, v] of entries) {
        const val = typeof v === 'string' ? v : JSON.stringify(v);
        maxLen = Math.max(maxLen, k.length + 2 + val.split('\n')[0].length);
      }
    }
  }

  if (hasCwd) {
    lines += 1; // CWD 行（claude-tool-popup.js が先頭に追加）
    maxLen = Math.max(maxLen, cwdLen); // CWD 行の長さも幅に反映
  }
  lines += 3; // セパレーター + アクションバー + プロンプト行

  const wPct = termW > 0 ? Math.min(95, Math.max(25, Math.round((maxLen + 8) / termW * 100))) : 70;
  // +3: tmux ボーダー上下(2) + ❯partial行(1) 分、ceil で切り上げて不足を防ぐ
  const hPct = termH > 0 ? Math.min(90, Math.max(10, Math.ceil((lines + 3)    / termH * 100))) : 60;
  return { wPct, hPct };
}

async function getNotifyType() {
  const val = await execQuiet('tmux show-option -gv @claude-notify-type 2>/dev/null');
  return val === 'menu' ? 'menu' : 'popup';
}

// Builds new file contents from tool_input for diff popup.
// Returns { newContents, oldContent } — oldContent is null for Write or on read failure.
// newContents is null when not computable (fallback to tool popup).
function buildNewContents(toolName, toolInput) {
  try {
    if (toolName === 'Write') {
      return { newContents: toolInput.content ?? null, oldContent: null };
    }
    if (toolName === 'Edit') {
      const filePath = toolInput.file_path;
      if (!filePath) return { newContents: null, oldContent: null };
      if (!path.isAbsolute(filePath) || filePath.split(path.sep).includes('..')) return { newContents: null, oldContent: null };
      const oldStr = toolInput.old_string ?? '';
      const newStr = toolInput.new_string ?? '';
      if (!oldStr) return { newContents: null, oldContent: null };
      try {
        const oldContent = fs.readFileSync(filePath, 'utf8');
        const newContents = toolInput.replace_all
          ? oldContent.replaceAll(oldStr, newStr)
          : oldContent.replace(oldStr, newStr);
        return { newContents, oldContent };
      } catch {
        // File not locally readable (remote SSH) — show old_string → new_string diff directly
        return { newContents: newStr, oldContent: oldStr };
      }
    }
  } catch { /* fallthrough */ }
  return { newContents: null, oldContent: null };
}

// Attaches a close handler to a popup proc: reads choice file and sends key to Claude.
function installChoiceHandler(proc, window, choiceFile, logTag) {
  proc.on('close', () => {
    console.log(`[mcp] ${logTag} closed`);
    if (!window) return;
    try {
      const c = fs.readFileSync(choiceFile, 'utf8').trim();
      fs.unlinkSync(choiceFile);
      if (['1', '2', '3'].includes(c)) {
        setTimeout(async () => {
          // Use execAsync instead of spawnSync to avoid blocking event loop
          const safeTarget = `claude:=${shellQuote(window)}`;
          const paneOut = await execAsync(`tmux capture-pane -t ${safeTarget} -p`) ?? '';
          const maxOption = detectMaxOption(paneOut);
          const key = Number(c) > maxOption ? String(maxOption) : c;
          console.log(`[mcp] send-keys ${logTag}-choice=${c} key=${key} to ${window}`);
          // fire-and-forget: send-keys does not need a return value
          execAsync(`tmux send-keys -t ${safeTarget} ${shellQuote(key)}`).catch(() => {});
        }, 100);
      }
    } catch { /* no choice file — user closed popup without selecting */ }
  });
}

// Edit/Write の diff popup を非同期で起動（WebSocket openDiff の代替）
// httpSocket の end() は本関数が責任を持つ（どの経路でも必ず呼ぶ）
async function triggerDiffPopupForWindow(window, toolName, toolInput, httpSocket) {
  const endSocket = () => { if (!httpSocket.destroyed) httpSocket.end('HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n'); };

  try {
    const filePath = toolInput.file_path;
    const { newContents, oldContent } = buildNewContents(toolName, toolInput);

    if (!filePath || newContents === null) {
      console.log(`[mcp] diff fallback to tool-popup for ${toolName}`);
      endSocket();
      triggerPopupForWindow(window, toolName, toolInput).catch(e => console.warn('[mcp] triggerPopupForWindow error', e.message));
      return;
    }

    const client = await findActiveClient();
    if (!client) {
      console.log('[mcp] diff popup: no active client');
      endSocket();
      return;
    }

    const diffScript = path.join(__dirname, 'claude-diff.js');
    const ts = Date.now();
    const tmpNew = `/tmp/tmux-claude-diff-${ts}.tmp`;
    fs.writeFileSync(tmpNew, newContents, 'utf8');

    // For remote files (not locally accessible), write old content to a tmp file for diffing
    const fileLocal = (() => { try { fs.accessSync(filePath); return true; } catch { return false; } })();
    let tmpOld = null;
    if (!fileLocal && oldContent) {
      tmpOld = `/tmp/tmux-claude-diff-old-${ts}.tmp`;
      fs.writeFileSync(tmpOld, oldContent, 'utf8');
    }

    const safeWin = (window ?? '').replace(/[^a-zA-Z0-9_-]/g, '_');
    const choiceFile = `/tmp/tmux-claude-diff-choice-${safeWin}.txt`;

    // reuse oldContent from buildNewContents (Edit) to avoid a second file read
    const oldLines = oldContent ? oldContent.split('\n') : (() => { try { return fs.readFileSync(filePath, 'utf8').split('\n'); } catch { return []; } })();
    const newLines = newContents.split('\n');
    const diffLineCount = Math.abs(newLines.length - oldLines.length) + Math.min(newLines.length, oldLines.length);
    const maxLineLen = Math.max(
      newLines.reduce((m, l) => Math.max(m, l.length), 0),
      oldLines.reduce((m, l) => Math.max(m, l.length), 0),
      40
    );

    // Step 1.2: Batch size + CWD into one display-message call
    const dimOut = await execAsync(`tmux display-message -c ${shellQuote(client)} -p '#{client_width} #{client_height}'`);
    const [termW, termH] = (dimOut ?? '').trim().split(' ').map(Number);
    const wPct = termW > 0 ? Math.min(95, Math.max(70, Math.round((maxLineLen + 12) / termW * 100))) : 90;
    const hPct = termH > 0 ? Math.min(95, Math.max(50, Math.round((diffLineCount + 8) / termH * 100))) : 80;

    const cwdOut = window
      ? await execAsync(`tmux display-message -t 'claude:=${window}' -p '#{pane_current_path}'`)
      : null;
    const diffCwd = (cwdOut ?? '').trim();

    console.log(`[mcp] diff popup ${toolName} file=${filePath} size=${wPct}%x${hPct}%`);

    // HTTP 200 を即返してから diff popup を非同期起動（hook timeout 対策）
    endSocket();

    const diffArgs = [shellQuote(filePath), shellQuote(window ?? ''), shellQuote(tmpNew)];
    if (tmpOld) diffArgs.push(shellQuote(tmpOld));
    const proc = spawn('tmux', [
      'display-popup', '-c', client, `-w${wPct}%`, `-h${hPct}%`, '-E',
      `TOOL_CWD=${shellQuote(diffCwd)} node ${shellQuote(diffScript)} ${diffArgs.join(' ')}`,
    ], { detached: false });
    proc.stderr.on('data', d => console.warn(`[mcp] diff popup stderr: ${d.toString().trim()}`));
    proc.on('error', e => console.warn(`[mcp] diff popup error: ${e.message}`));
    installChoiceHandler(proc, window, choiceFile, 'diff-popup');
  } catch (e) {
    console.warn(`[mcp] triggerDiffPopupForWindow error: ${e.message}`);
    endSocket();
  }
}

// Launches the tool confirmation popup asynchronously.
// After the popup closes, reads CHOICE_FILE and sends the key to Claude.
async function triggerPopupForWindow(window, toolName, toolInput) {
  const client = await findActiveClient();
  if (!client) { console.warn('[mcp] no active client for popup'); return; }

  const type = await getNotifyType();
  console.log(`[mcp] popup type=${type} window=${window ?? '?'} tool=${toolName || '?'} client=${client}`);

  if (type === 'menu') {
    const popupScript = path.join(__dirname, 'claude-popup.sh');
    const popupCmd = window ? `${shellQuote(popupScript)} ${shellQuote(window)}` : shellQuote(popupScript);
    // Wrap display-menu spawn in a Promise so we can await without blocking event loop
    await new Promise((resolve) => {
      const menuProc = spawn('tmux', [
        'display-menu', '-c', client,
        '-T', 'Claude: permission required',
        'Open Claude', 'o', `display-popup -c ${shellQuote(client)} -w90% -h80% -E ${popupCmd}`,
        'Dismiss',     'd', '',
      ], { detached: false });
      menuProc.on('close', resolve);
      menuProc.on('error', resolve); // resolve on error too — don't hang
    });
    return;
  }

  // popup path: launch display-popup asynchronously so HTTP 200 can be sent immediately
  // claude-tool-popup.js handles keyboard input itself via process.stdin.on('data', ...)
  // (same approach as claude-diff.js — display-popup provides a PTY for stdin)
  const toolPopupScript = path.join(__dirname, 'claude-tool-popup.js');
  const toolInputJson = JSON.stringify(toolInput ?? {});
  const safeWin = (window ?? '').replace(/[^a-zA-Z0-9_-]/g, '_');
  const choiceFile = `/tmp/tmux-claude-tool-choice-${safeWin}.txt`;

  // Step 1.2: Batch CWD + size into a single display-message call
  // Format: "#{pane_current_path}\t#{client_width}\t#{client_height}" — tab-separated to handle paths with spaces
  const dimCwd = window
    ? await execAsync(`tmux display-message -t 'claude:=${window}' -c ${shellQuote(client)} -p '#{pane_current_path}\t#{client_width}\t#{client_height}'`)
    : await execAsync(`tmux display-message -c ${shellQuote(client)} -p '\t#{client_width}\t#{client_height}'`);
  const dimCwdParts = (dimCwd ?? '').trim().split('\t');
  const toolCwd = (dimCwdParts[0] ?? '').trim();
  const termW = Number(dimCwdParts[1] ?? 0);
  const termH = Number(dimCwdParts[2] ?? 0);

  // CWD は claude-tool-popup.js 内で ~ 置換されるので近似値として使用
  const cwdDisplayLen = toolCwd ? toolCwd.replace(os.homedir(), '~').length : 0;
  const { wPct, hPct } = estimateToolPopupSize(toolName, toolInput ?? {}, termW, termH, !!toolCwd, cwdDisplayLen);
  console.log(`[mcp] popup size=${wPct}%x${hPct}% (term=${termW}x${termH})`);

  const popupCmd = `TOOL_NAME=${shellQuote(toolName || '')} TOOL_INPUT=${shellQuote(toolInputJson)} TOOL_CWD=${shellQuote(toolCwd)} node ${shellQuote(toolPopupScript)} ${shellQuote(window ?? '')}`;
  const proc = spawn('tmux', [
    'display-popup', '-c', client, `-w${wPct}%`, `-h${hPct}%`, '-E', popupCmd,
  ], { detached: false });
  proc.stderr.on('data', d => console.warn(`[mcp] display-popup stderr: ${d.toString().trim()}`));
  proc.on('error', e => console.warn(`[mcp] display-popup error: ${e.message}`));

  installChoiceHandler(proc, window, choiceFile, 'tool-popup');
}

async function triggerPopup(socket) {
  const window = socketState.get(socket)?.window ?? null;
  await triggerPopupForWindow(window, '', {});
}

// --- MCP message handler ---

async function handleMcpMessage(socket, msg) {
  const { id, method, params } = msg;

  switch (method) {
    case 'initialize':
      reply(socket, id, {
        protocolVersion: params?.protocolVersion ?? '2025-03-26',
        capabilities: { tools: {} },
        serverInfo: { name: 'tmux-claude', version: '1.0.0' },
      });
      break;

    case 'ide_connected':
      if (params?.pid) {
        const pid = params.pid;
        // async: does not block frame processing for other sockets
        const localWindowInfo = await findTmuxWindowForPid(pid);
        const localWindow = localWindowInfo?.window ?? null;
        let remoteWindow = null;
        // Only consume pending remote window if this PID has no local tmux window
        if (!localWindow) {
          const pendingFile = '/tmp/tmux-claude-next-remote-window';
          try {
            remoteWindow = fs.readFileSync(pendingFile, 'utf8').trim() || null;
            if (remoteWindow) fs.unlinkSync(pendingFile);
          } catch { /* no pending remote window */ }
        }
        const window = localWindow ?? remoteWindow;
        socketState.set(socket, { pid, window });
        pidToWindow.set(pid, window);
        console.log(`[mcp] ide_connected pid=${pid}${localWindow ? ` local-window=${localWindow}` : ''}${remoteWindow ? ` remote-window=${remoteWindow}` : ''}`);
      }
      break;

    case 'tools/list':
      reply(socket, id, { tools: [] });
      break;

    case 'tools/call':
      if (params?.name === 'openDiff') {
        const args = params.arguments ?? {};
        const oldPath = args.old_file_path;
        const newContents = args.new_file_contents;
        const window = socketState.get(socket)?.window ?? null;
        console.log(`[mcp] openDiff called oldPath=${oldPath} window=${window ?? 'null'}`);
        if (oldPath && newContents != null && window) {
          let diffReply = 'TAB_CLOSED';
          try {
            const client = await findActiveClient();
            const diffScript = path.join(__dirname, 'claude-diff.js');
            if (client) {
              // ターミナルサイズを取得してポップアップサイズを動的に決定
              const dimOut = await execAsync(`tmux display-message -c ${shellQuote(client)} -p '#{client_width} #{client_height}'`);
              const [termW, termH] = (dimOut ?? '').trim().split(' ').map(Number);

              // ローカルにファイルがない場合 (remote SSH) は pendingToolInfo から旧内容を再構築
              const oldFileAccessible = (() => { try { fs.accessSync(oldPath); return true; } catch { return false; } })();
              let oldContent = null;
              if (!oldFileAccessible) {
                // pendingToolInfo: get() then immediately delete() to prevent double-processing
                const info = pendingToolInfo.get(window);
                pendingToolInfo.delete(window);
                if (info?.tool_name === 'Edit' && info.tool_input?.old_string != null && info.tool_input?.new_string != null) {
                  const oldStr = info.tool_input.old_string;
                  const newStr = info.tool_input.new_string;
                  oldContent = info.tool_input.replace_all
                    ? newContents.replaceAll(newStr, oldStr)
                    : newContents.replace(newStr, oldStr);
                  console.log(`[mcp] openDiff reconstructed old content from pendingToolInfo (${oldContent.split('\n').length} lines)`);
                }
              } else {
                try { oldContent = fs.readFileSync(oldPath, 'utf8'); } catch { /* ok */ }
              }

              // diff の行数と最長行からサイズを推定
              const oldLines = oldContent ? oldContent.split('\n') : [];
              const newLines = newContents.split('\n');
              const diffLineCount = Math.abs(newLines.length - oldLines.length) + Math.min(newLines.length, oldLines.length);
              const maxLineLen = Math.max(
                newLines.reduce((m, l) => Math.max(m, l.length), 0),
                oldLines.reduce((m, l) => Math.max(m, l.length), 0),
                40
              );

              const wPct = termW > 0 ? Math.min(95, Math.max(70, Math.round((maxLineLen + 12) / termW * 100))) : 90;
              const hPct = termH > 0 ? Math.min(95, Math.max(50, Math.round((diffLineCount + 8) / termH * 100))) : 80;

              const ts = Date.now();
              const tmpNewWs = `/tmp/tmux-claude-diff-${ts}.tmp`;
              fs.writeFileSync(tmpNewWs, newContents, 'utf8');
              let tmpOldWs = null;
              if (!oldFileAccessible && oldContent) {
                tmpOldWs = `/tmp/tmux-claude-diff-old-${ts}.tmp`;
                fs.writeFileSync(tmpOldWs, oldContent, 'utf8');
              }
              const safeWindow = window.replace(/[^a-zA-Z0-9_-]/g, '_');
              const choiceFile = `/tmp/tmux-claude-diff-choice-${safeWindow}.txt`;
              const diffArgs = [shellQuote(oldPath), shellQuote(window), shellQuote(tmpNewWs)];
              if (tmpOldWs) diffArgs.push(shellQuote(tmpOldWs));

              // Phase 5.1: spawn + Promise wrap — replaces blocking spawnSync display-popup
              const choice = await new Promise((resolve) => {
                const popupProc = spawn('tmux', [
                  'display-popup', '-c', client, `-w${wPct}%`, `-h${hPct}%`, '-E',
                  `node ${shellQuote(diffScript)} ${diffArgs.join(' ')}`,
                ], { detached: false });
                popupProc.stderr.on('data', d => console.warn(`[mcp] openDiff popup stderr: ${d.toString().trim()}`));
                popupProc.on('error', e => { console.warn('[mcp] openDiff popup error', e.message); resolve(null); });
                popupProc.on('close', () => {
                  // Read choice file atomically after popup closes
                  try {
                    const c = fs.readFileSync(choiceFile, 'utf8').trim();
                    fs.unlinkSync(choiceFile);
                    resolve(['1', '2', '3'].includes(c) ? c : null);
                  } catch {
                    resolve(null); // no choice file — user closed without selecting
                  }
                });
              });

              if (choice) {
                pendingDiffChoices.set(window, choice);
                if (choice === '3') diffReply = 'REJECTED';
                else if (choice === '2') diffReply = 'ALWAYS_ALLOW';
              }
            }
          } catch (e) {
            console.warn('[mcp] openDiff error', e.message);
            await triggerPopup(socket).catch(() => {});
          }
          // socket.writable guard: popup await may have taken time; WebSocket may have disconnected
          if (id != null && socket.writable) {
            reply(socket, id, { content: [{ type: 'text', text: diffReply }] });
          }
        } else {
          // No window resolved — fall back to triggerPopup (fire-and-forget)
          triggerPopup(socket).catch(e => console.warn('[mcp] triggerPopup error', e.message));
          if (id != null && socket.writable) {
            reply(socket, id, { content: [{ type: 'text', text: 'TAB_CLOSED' }] });
          }
        }
        break;
      }
      if (id != null) reply(socket, id, {});
      break;

    default:
      if (id != null) reply(socket, id, {});
      break;
  }
}

// --- Connection handler ---

const connections = new Set();

function handleConnection(socket) {
  socketState.set(socket, { pid: null });
  let upgraded = false;
  let buf = Buffer.alloc(0);

  socket.on('data', chunk => {
buf = Buffer.concat([buf, chunk]);

    if (!upgraded) {
      const end = buf.indexOf('\r\n\r\n');
      if (end === -1) return;

      const headerText = buf.slice(0, end).toString('utf8');
      buf = buf.slice(end + 4);

      const headers = {};
      for (const line of headerText.split('\r\n').slice(1)) {
        const colon = line.indexOf(': ');
        if (colon !== -1) headers[line.slice(0, colon).toLowerCase()] = line.slice(colon + 2);
      }

      if (headers['x-claude-code-ide-authorization'] !== AUTH_TOKEN) {
        console.warn('[mcp] auth failed: token mismatch');
        sendHttpError(socket, 401, 'Unauthorized');
        return;
      }

      const key = headers['sec-websocket-key'];
      if (!key) {
        // HTTP POST /notify
        const requestLine = headerText.split('\r\n')[0];
        if (requestLine.startsWith('POST /notify')) {
          const contentLength = parseInt(headers['content-length'] || '0', 10);
          const readBody = (cb) => {
            if (buf.length >= contentLength) {
              cb(buf.slice(0, contentLength));
            } else {
              socket.once('data', chunk => {
                buf = Buffer.concat([buf, chunk]);
                readBody(cb);
              });
            }
          };
          readBody(async (body) => {
            try {
              const data = JSON.parse(body.toString('utf8'));
              console.log('[mcp] /notify raw:', JSON.stringify(data));
              const rawPid = Number(data.pid);

              if (Number.isInteger(rawPid) && rawPid > 0) {
                // Step 1.3: resolveWindow called exactly once per /notify (dedup)
                const window = await resolveWindow(rawPid);

                // PreToolUse hook からのツール情報を保存
                if (data.type === 'tool_info') {
                  if (window) {
                    pendingToolInfo.set(window, { tool_name: data.tool_name, tool_input: data.tool_input, ts: Date.now() });
                    console.log(`[mcp] tool_info stored window=${window} tool=${data.tool_name}`);
                  }
                  if (!socket.destroyed) socket.end('HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n');
                  return;
                }

                if (window) {
                  console.log(`[mcp] /notify pid=${data.pid} window=${window}`);
                  const pendingChoice = pendingDiffChoices.get(window);
                  if (pendingChoice) {
                    pendingDiffChoices.delete(window);
                    if (!socket.destroyed) socket.end('HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n');
                    setTimeout(async () => {
                      console.log(`[mcp] send-keys diff-choice=${pendingChoice} to ${window}`);
                      // fire-and-forget send-keys (async, non-blocking)
                      execAsync(`tmux send-keys -t ${shellQuote('claude:=' + window)} ${shellQuote(pendingChoice)}`).catch(() => {});
                    }, 50);
                    return;
                  }
                  // ツール情報を取得（pendingToolInfo → message parse の順でフォールバック）
                  // pendingToolInfo: get() then immediately delete() to prevent double-processing
                  const info = pendingToolInfo.get(window);
                  pendingToolInfo.delete(window);
                  const fresh = info && Date.now() - info.ts < 15000;
                  const msgTool = (data.message || '').match(/\buse (\w+)$/)?.[1] || '';
                  const toolName  = (fresh ? info.tool_name  : null) || data.tool_name || msgTool || '';
                  const toolInput = (fresh ? info.tool_input : null) || data.tool_input || {};
                  console.log(`[mcp] tool resolve: pending=${fresh ? info.tool_name : 'none'} msg=${msgTool} => ${toolName}`);
                  // Edit/Write は diff popup で処理（WebSocket 不要、tool_input から直接 diff を生成）
                  const DIFF_TOOLS = new Set(['Edit', 'Write', 'MultiEdit', 'NotebookEdit']);
                  if (DIFF_TOOLS.has(toolName)) {
                    // triggerDiffPopupForWindow sends HTTP 200 internally (before or after popup)
                    await triggerDiffPopupForWindow(window, toolName, toolInput, socket).catch(e => {
                      console.warn('[mcp] triggerDiffPopupForWindow error', e.message);
                      if (!socket.destroyed) socket.end('HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n');
                    });
                    return;
                  }
                  // fire-and-forget for non-diff tools (popup launched async)
                  triggerPopupForWindow(window, toolName, toolInput).catch(e => console.warn('[mcp] triggerPopupForWindow error', e.message));
                }
              }
            } catch (e) {
              console.warn('[mcp] /notify parse error', e.message);
            }
            if (!socket.destroyed) socket.end('HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n');
          });
        } else {
          sendHttpError(socket, 400, 'Bad Request');
        }
        return;
      }

      send101(socket, key);
      upgraded = true;
      connections.add(socket);
      console.log(`[mcp] client connected (total: ${connections.size})`);
    }

    while (buf.length > 0) {
      const frame = parseFrame(buf);
      if (!frame) break;
      buf = buf.slice(frame.consumed);

      switch (frame.opcode) {
        case 0x1: {
          let msg;
          try { msg = JSON.parse(frame.payload.toString('utf8')); } catch { break; }
          // Per-socket sequential queue: ensures ide_connected completes before openDiff runs.
          // Frame parsing loop stays non-blocking; messages are chained per connection.
          const prev = socketMsgQueue.get(socket) ?? Promise.resolve();
          const next = prev.then(() => handleMcpMessage(socket, msg)).catch(e => console.warn('[mcp] handleMcpMessage error', e.message));
          socketMsgQueue.set(socket, next);
          break;
        }
        case 0x8: socket.destroy(); break;
        case 0x9: sendPong(socket, frame.payload); break;
      }
    }
  });

  socket.on('close', () => {
    const pid = socketState.get(socket)?.pid;
    if (pid) {
      pidToWindow.delete(pid);
    }
    connections.delete(socket);
    socketState.delete(socket);
    socketMsgQueue.delete(socket);
    console.log(`[mcp] client disconnected (total: ${connections.size})`);
  });

  socket.on('error', () => {
    connections.delete(socket);
    socketState.delete(socket);
    socketMsgQueue.delete(socket);
  });
}

// --- Remote lock file update ---

// ポートが変わった際に claude セッションのリモートウィンドウのロックファイルを更新する。
// SSH の OSC 7 パス (pane_path) からホスト名を取得し、ssh コマンドで書き換える。
async function updateRemoteLockFiles(newPort, oldPort) {
  const lockContent = JSON.stringify({
    pid: process.pid,
    workspaceFolders: [],
    ideName: 'tmux-claude',
    transport: 'ws',
    authToken: AUTH_TOKEN,
  });
  const b64 = Buffer.from(lockContent).toString('base64');

  const windowsOut = await execAsync(
    'tmux list-windows -t claude -F "#{window_name}\t#{pane_current_command}\t#{pane_path}"'
  );
  if (!windowsOut) return;

  const windows = windowsOut.trim().split('\n').filter(Boolean);
  const seen = new Set();
  const tasks = [];

  for (const line of windows) {
    const [, cmd, oscPath] = line.split('\t');
    if (cmd !== 'ssh') continue;

    // OSC 7: file://hostname/path → extract hostname
    const hostMatch = oscPath?.match(/^file:\/\/([^/]+)/);
    if (!hostMatch) continue;
    const host = hostMatch[1];
    const localHost = os.hostname();
    if (host === localHost || host === localHost.split('.')[0]) continue;
    if (seen.has(host)) continue;
    seen.add(host);

    // 新ポートのロックファイルを書き込み、旧ポートのロックファイルを削除
    const rmOld = oldPort ? `rm -f ~/.claude/ide/${oldPort}.lock && ` : '';
    const remoteCmd = `mkdir -p ~/.claude/ide && ${rmOld}printf '%s' ${b64} | base64 -d > ~/.claude/ide/${newPort}.lock`;
    // Run all SSH updates in parallel
    tasks.push(
      execAsync(`ssh -o ConnectTimeout=3 -o BatchMode=yes ${shellQuote(host)} ${shellQuote(remoteCmd)}`, 8000)
        .then(out => {
          if (out !== null) {
            console.log(`[mcp] updated remote lock file on ${host}: port ${oldPort} → ${newPort}`);
          } else {
            console.warn(`[mcp] failed to update remote lock file on ${host}`);
          }
        })
        .catch(e => console.warn(`[mcp] SSH error for ${host}: ${e.message}`))
    );
  }

  await Promise.all(tasks);
}

// --- Lock file ---

function writeLockFile(port) {
  fs.mkdirSync(LOCK_DIR, { recursive: true });
  const lockPath = path.join(LOCK_DIR, `${port}.lock`);
  fs.writeFileSync(lockPath, JSON.stringify({
    pid: process.pid,
    workspaceFolders: [],
    ideName: 'tmux-claude',
    transport: 'ws',
    authToken: AUTH_TOKEN,
  }), { mode: 0o600 });
  return lockPath;
}

function deleteLockFile(port) {
  try { fs.unlinkSync(path.join(LOCK_DIR, `${port}.lock`)); } catch { /* ok */ }
}

// --- Main ---

const server = net.createServer(handleConnection);

// 環境変数でポートを固定可能（再起動後も同じポートで起動し Claude Code が自動再接続できる）
const LISTEN_PORT = parseInt(process.env.TMUX_CLAUDE_PORT || '0', 10);

function onListening() {
  const { port } = server.address();
  const lockPath = writeLockFile(port);

  fs.writeFileSync(PID_FILE,   String(process.pid), { mode: 0o600 });
  fs.writeFileSync(PORT_FILE,  String(port),        { mode: 0o600 });
  fs.writeFileSync(TOKEN_FILE, AUTH_TOKEN,           { mode: 0o600 });
  fs.chmodSync(TOKEN_FILE, 0o600);

  console.log(`[mcp] started  port=${port}  pid=${process.pid}`);
  console.log(`[mcp] lock     ${lockPath}`);

  // ポートが変わった場合、リモート Claude セッションのロックファイルを更新する（fire-and-forget）
  if (LISTEN_PORT !== 0 && port !== LISTEN_PORT) {
    console.log(`[mcp] port changed ${LISTEN_PORT} → ${port}, updating remote lock files`);
    updateRemoteLockFiles(port, LISTEN_PORT).catch(e => console.warn('[mcp] updateRemoteLockFiles error', e.message));
  }

  function cleanup() {
    deleteLockFile(port);
    try { fs.unlinkSync(PID_FILE); } catch { /* ok */ }
    try { fs.unlinkSync(PORT_FILE); } catch { /* ok */ }
    // TOKEN_FILE は削除しない — 再起動後も同じ token で Claude Code が自動再接続できる
    console.log('[mcp] stopped');
    process.exit(0);
  }

  process.on('SIGTERM', cleanup);
  process.on('SIGINT', cleanup);
}

// EADDRINUSE の場合は最大 5 回リトライして同ポートを確保する
// （TIME_WAIT 状態のソケットが残っている場合など）
let retries = 0;
server.on('error', (err) => {
  if (err.code === 'EADDRINUSE' && LISTEN_PORT !== 0 && retries < 5) {
    retries++;
    console.warn(`[mcp] port ${LISTEN_PORT} in use, retry ${retries}/5 in 500ms...`);
    setTimeout(() => server.listen(LISTEN_PORT, '127.0.0.1', onListening), 500);
  } else {
    console.error(`[mcp] listen error: ${err.message}`);
    process.exit(1);
  }
});

server.listen(LISTEN_PORT, '127.0.0.1', onListening);
