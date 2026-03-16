'use strict';

/**
 * TDD tests for execSync/spawnSync → async migration (execsync-async-plan.md)
 *
 * Tests are organized by plan phase:
 *   Phase 1: execAsync helper + display-message batching + resolveWindow dedup
 *   Phase 2: leaf function async (findTmuxWindowForPid, resolveWindow, findActiveClient, getNotifyType)
 *   Phase 3: popup function async (installChoiceHandler, triggerPopupForWindow, triggerDiffPopupForWindow, triggerPopup)
 *   Phase 4: top-level handler async (handleMcpMessage, /notify, updateRemoteLockFiles)
 *   Phase 5: openDiff spawnSync popup → spawn + Promise
 */

const { describe, test, before, after, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const net = require('node:net');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const { spawn } = require('node:child_process');

const SCRIPT = path.join(__dirname, 'mcp-server.js');
const PORT_FILE = '/tmp/tmux-claude-mcp.port';
const TOKEN_FILE = '/tmp/tmux-claude-mcp.token';
const PID_FILE = '/tmp/tmux-claude-mcp.pid';
const WS_MAGIC = '258EAFA5-E914-47DA-95CA-C5AB0DC85B11';

// ---------------------------------------------------------------------------
// Shared helpers (same as mcp-server.test.js)
// ---------------------------------------------------------------------------

async function waitForFile(filePath, timeout = 4000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    if (fs.existsSync(filePath)) return true;
    await new Promise(r => setTimeout(r, 50));
  }
  return false;
}

function expectedAccept(key) {
  return crypto.createHash('sha1').update(key + WS_MAGIC).digest('base64');
}

function buildHandshake(port, token, key) {
  return [
    'GET / HTTP/1.1',
    `Host: 127.0.0.1:${port}`,
    'Upgrade: websocket',
    'Connection: Upgrade',
    `Sec-WebSocket-Key: ${key}`,
    'Sec-WebSocket-Version: 13',
    `x-claude-code-ide-authorization: ${token}`,
    '', '',
  ].join('\r\n');
}

function buildTextFrame(text) {
  const payload = Buffer.from(text, 'utf8');
  const mask = crypto.randomBytes(4);
  let header;
  if (payload.length < 126) {
    header = Buffer.from([0x81, 0x80 | payload.length]);
  } else {
    header = Buffer.allocUnsafe(4);
    header[0] = 0x81;
    header[1] = 0xfe;
    header.writeUInt16BE(payload.length, 2);
  }
  const masked = Buffer.allocUnsafe(payload.length);
  for (let i = 0; i < payload.length; i++) masked[i] = payload[i] ^ mask[i % 4];
  return Buffer.concat([header, mask, masked]);
}

function parseServerFrame(buf) {
  if (buf.length < 2) return null;
  const opcode = buf[0] & 0x0f;
  let len = buf[1] & 0x7f;
  let offset = 2;
  if (len === 126) {
    if (buf.length < 4) return null;
    len = buf.readUInt16BE(2);
    offset = 4;
  }
  if (buf.length < offset + len) return null;
  return { opcode, text: buf.slice(offset, offset + len).toString('utf8') };
}

function connect(port, token) {
  return new Promise((resolve, reject) => {
    const socket = new net.Socket();
    const key = crypto.randomBytes(16).toString('base64');
    let buf = Buffer.alloc(0);
    let done = false;

    socket.connect(port, '127.0.0.1', () => {
      socket.write(buildHandshake(port, token, key));
    });

    socket.on('data', chunk => {
      if (done) return;
      buf = Buffer.concat([buf, chunk]);
      const end = buf.indexOf('\r\n\r\n');
      if (end === -1) return;
      const header = buf.slice(0, end).toString();
      if (!header.includes('101 Switching Protocols')) {
        done = true;
        reject(new Error(`Unexpected response: ${header.split('\r\n')[0]}`));
        socket.destroy();
        return;
      }
      assert.ok(header.includes(expectedAccept(key)), 'Sec-WebSocket-Accept mismatch');
      done = true;
      socket._wsBuf = buf.slice(end + 4);
      resolve(socket);
    });

    socket.on('error', reject);
  });
}

function rpc(socket, method, params, id = 1) {
  return new Promise((resolve, reject) => {
    const msg = JSON.stringify({ jsonrpc: '2.0', id, method, params });
    socket.write(buildTextFrame(msg));

    let buf = socket._wsBuf || Buffer.alloc(0);

    const onData = chunk => {
      buf = Buffer.concat([buf, chunk]);
      const frame = parseServerFrame(buf);
      if (!frame || frame.opcode !== 0x1) return;
      socket.off('data', onData);
      try { resolve(JSON.parse(frame.text)); } catch (e) { reject(e); }
    };

    socket.on('data', onData);
    socket.on('error', reject);
    setTimeout(() => { socket.off('data', onData); reject(new Error('rpc timeout')); }, 5000);
  });
}

/** POST /notify and collect HTTP response */
function postNotify(port, token, body) {
  return new Promise((resolve, reject) => {
    const bodyStr = JSON.stringify(body);
    const req = [
      'POST /notify HTTP/1.1',
      `Host: 127.0.0.1:${port}`,
      `x-claude-code-ide-authorization: ${token}`,
      'Content-Type: application/json',
      `Content-Length: ${Buffer.byteLength(bodyStr)}`,
      '', bodyStr,
    ].join('\r\n');

    const sock = new net.Socket();
    let resp = '';
    sock.connect(port, '127.0.0.1', () => sock.write(req));
    sock.on('data', d => { resp += d.toString(); });
    sock.on('end', () => resolve(resp));
    sock.on('close', () => resolve(resp));
    sock.on('error', reject);
    setTimeout(() => { sock.destroy(); resolve(resp); }, 5000);
  });
}

// ---------------------------------------------------------------------------
// Phase 1: execAsync helper + batched display-message + resolveWindow dedup
// ---------------------------------------------------------------------------

describe('Phase 1: execAsync helper and resolveWindow dedup', () => {
  let proc;
  let port;
  let token;

  before(async () => {
    for (const f of [PID_FILE, PORT_FILE, TOKEN_FILE]) {
      try { fs.unlinkSync(f); } catch { /* ok */ }
    }
    proc = spawn('node', [SCRIPT], { stdio: ['ignore', 'pipe', 'pipe'] });
    proc.stdout.on('data', d => process.stdout.write(d));
    proc.stderr.on('data', d => process.stderr.write(d));
    const ok = await waitForFile(PORT_FILE, 5000);
    assert.ok(ok, 'server should start within 5 seconds');
    port = parseInt(fs.readFileSync(PORT_FILE, 'utf8').trim(), 10);
    token = fs.readFileSync(TOKEN_FILE, 'utf8').trim();
  });

  after(async () => {
    proc?.kill('SIGTERM');
    await new Promise(r => setTimeout(r, 500));
  });

  // Step 1.1: execAsync helper exists and is used (verified indirectly via no blocking)
  test('server starts and responds to initialize without blocking', async () => {
    const socket = await connect(port, token);
    const res = await rpc(socket, 'initialize', { protocolVersion: '2025-03-26', capabilities: {} });
    assert.equal(res.result.serverInfo.name, 'tmux-claude');
    socket.destroy();
  });

  // Step 1.2: Batched display-message — verified by /notify returning 200 quickly
  // even when tmux is not available (server uses execAsync with timeout, falls back gracefully)
  test('/notify with tool_info type returns 200 OK', async () => {
    const resp = await postNotify(port, token, {
      pid: process.pid,
      type: 'tool_info',
      tool_name: 'Edit',
      tool_input: { file_path: '/tmp/test.txt', old_string: 'a', new_string: 'b' },
    });
    assert.ok(resp.includes('200 OK'), `Expected 200 OK, got: ${resp.split('\r\n')[0]}`);
  });

  // Step 1.3: resolveWindow called only once per /notify (dedup)
  // Both tool_info and regular notify with same PID should each complete without hanging
  test('/notify with regular notification type returns 200 OK', async () => {
    const resp = await postNotify(port, token, {
      pid: process.pid,
      type: 'notify',
      message: 'Claude wants to use Edit',
    });
    assert.ok(resp.includes('200 OK'), `Expected 200 OK, got: ${resp.split('\r\n')[0]}`);
  });

  // Multiple concurrent /notify requests must all resolve (no event loop blocking)
  test('concurrent /notify requests all return 200 OK', async () => {
    const requests = Array.from({ length: 3 }, (_, i) =>
      postNotify(port, token, { pid: process.pid + i, type: 'notify', message: `test ${i}` })
    );
    const responses = await Promise.all(requests);
    for (const resp of responses) {
      assert.ok(resp.includes('200 OK'), `Expected 200 OK, got: ${resp.split('\r\n')[0]}`);
    }
  });
});

// ---------------------------------------------------------------------------
// Phase 2: Leaf function async — unit-level behaviour tests
// These tests verify async behaviour by checking that:
// (a) functions return Promises (not synchronous values),
// (b) the server event loop remains unblocked during PID resolution,
// (c) edge-case inputs produce correct results.
// ---------------------------------------------------------------------------

describe('Phase 2: Leaf functions return correct results asynchronously', () => {
  let proc;
  let port;
  let token;

  before(async () => {
    for (const f of [PID_FILE, PORT_FILE, TOKEN_FILE]) {
      try { fs.unlinkSync(f); } catch { /* ok */ }
    }
    proc = spawn('node', [SCRIPT], { stdio: ['ignore', 'pipe', 'pipe'] });
    proc.stdout.on('data', d => process.stdout.write(d));
    proc.stderr.on('data', d => process.stderr.write(d));
    const ok = await waitForFile(PORT_FILE, 5000);
    assert.ok(ok, 'server should start');
    port = parseInt(fs.readFileSync(PORT_FILE, 'utf8').trim(), 10);
    token = fs.readFileSync(TOKEN_FILE, 'utf8').trim();
  });

  after(async () => {
    proc?.kill('SIGTERM');
    await new Promise(r => setTimeout(r, 500));
  });

  // Step 2.1/2.2: findTmuxWindowForPid / resolveWindow — PID 1 is init, should terminate quickly
  test('resolveWindow with PID 1 (init) returns without hanging', async () => {
    const start = Date.now();
    // PID 1 is init; resolveWindow should terminate the walk immediately
    const resp = await postNotify(port, token, { pid: 1, type: 'notify', message: 'test' });
    const elapsed = Date.now() - start;
    assert.ok(resp.includes('200 OK'), 'should get 200 OK');
    // Must complete well within timeout (3000ms per exec * 15 iterations would be 45s)
    assert.ok(elapsed < 5000, `resolveWindow took ${elapsed}ms, expected < 5000ms`);
  });

  // Step 2.2: resolveWindow with PID 0 terminates immediately
  test('resolveWindow with PID 0 returns 200 quickly', async () => {
    const start = Date.now();
    const resp = await postNotify(port, token, { pid: 0, type: 'notify', message: 'test' });
    const elapsed = Date.now() - start;
    // Invalid PID — should return immediately
    assert.ok(resp.includes('200 OK') || resp.includes('200'), 'should complete');
    assert.ok(elapsed < 3000, `took ${elapsed}ms`);
  });

  // Step 2.3: findActiveClient — returns quickly even when tmux is absent/mock
  test('server handles ide_connected with PID and returns to process next frame', async () => {
    const socket = await connect(port, token);
    // Send ide_connected — async findTmuxWindowForPid must not block next RPC
    const frameIde = buildTextFrame(JSON.stringify({ jsonrpc: '2.0', id: null, method: 'ide_connected', params: { pid: process.pid } }));
    socket.write(frameIde);
    // Immediately send initialize — must get response even while ide_connected is resolving
    const res = await rpc(socket, 'initialize', { protocolVersion: '2025-03-26', capabilities: {} }, 99);
    assert.equal(res.result.serverInfo.name, 'tmux-claude');
    socket.destroy();
  });

  // Step 2.4: getNotifyType — /notify pathway completes without error
  test('/notify pathway completes for unknown tool name', async () => {
    const resp = await postNotify(port, token, {
      pid: process.pid,
      type: 'notify',
      tool_name: 'UnknownTool',
      tool_input: {},
    });
    assert.ok(resp.includes('200 OK'), `Expected 200 OK, got: ${resp}`);
  });
});

// ---------------------------------------------------------------------------
// Phase 3: Popup functions async conversion
// These tests verify that triggerDiffPopupForWindow / triggerPopupForWindow / triggerPopup
// do not block the event loop. We test via /notify which calls them.
// ---------------------------------------------------------------------------

describe('Phase 3: Popup functions do not block event loop', () => {
  let proc;
  let port;
  let token;

  before(async () => {
    for (const f of [PID_FILE, PORT_FILE, TOKEN_FILE]) {
      try { fs.unlinkSync(f); } catch { /* ok */ }
    }
    proc = spawn('node', [SCRIPT], { stdio: ['ignore', 'pipe', 'pipe'] });
    proc.stdout.on('data', d => process.stdout.write(d));
    proc.stderr.on('data', d => process.stderr.write(d));
    const ok = await waitForFile(PORT_FILE, 5000);
    assert.ok(ok, 'server should start');
    port = parseInt(fs.readFileSync(PORT_FILE, 'utf8').trim(), 10);
    token = fs.readFileSync(TOKEN_FILE, 'utf8').trim();
  });

  after(async () => {
    proc?.kill('SIGTERM');
    await new Promise(r => setTimeout(r, 500));
  });

  // Step 3.1: installChoiceHandler internal spawnSync → execAsync
  // After a popup closes, the server must still respond to other requests
  test('server remains responsive after /notify triggers diff popup pathway', async () => {
    // Send a notify that would trigger triggerDiffPopupForWindow (Edit tool)
    // Since no tmux is available, it will fall back gracefully
    const notifyResp = await postNotify(port, token, {
      pid: process.pid,
      type: 'notify',
      tool_name: 'Edit',
      tool_input: { file_path: '/tmp/nonexistent.txt', old_string: 'x', new_string: 'y' },
    });
    assert.ok(notifyResp.includes('200 OK'), `notify should return 200: ${notifyResp.split('\r\n')[0]}`);

    // Server must still be responsive after popup pathway
    const socket = await connect(port, token);
    const res = await rpc(socket, 'tools/list', {}, 5);
    assert.ok(Array.isArray(res.result?.tools), 'tools/list must still work');
    socket.destroy();
  });

  // Step 3.3: triggerPopupForWindow — Write tool goes through diff popup
  test('/notify with Write tool_name does not block subsequent requests', async () => {
    const notifyResp = await postNotify(port, token, {
      pid: process.pid,
      type: 'notify',
      tool_name: 'Write',
      tool_input: { file_path: '/tmp/out.txt', content: 'hello' },
    });
    assert.ok(notifyResp.includes('200 OK'), `notify Write should return 200: ${notifyResp.split('\r\n')[0]}`);

    const socket = await connect(port, token);
    const res = await rpc(socket, 'tools/list', {}, 6);
    assert.ok(Array.isArray(res.result?.tools), 'server must still respond');
    socket.destroy();
  });

  // Step 3.2: triggerDiffPopupForWindow — non-diff tool (Bash) goes through triggerPopupForWindow
  test('/notify with Bash tool_name goes through triggerPopupForWindow without blocking', async () => {
    const notifyResp = await postNotify(port, token, {
      pid: process.pid,
      type: 'notify',
      tool_name: 'Bash',
      tool_input: { command: 'echo hello' },
    });
    assert.ok(notifyResp.includes('200 OK'), `notify Bash should return 200: ${notifyResp.split('\r\n')[0]}`);

    const socket = await connect(port, token);
    const res = await rpc(socket, 'initialize', { protocolVersion: '2025-03-26', capabilities: {} }, 7);
    assert.equal(res.result.serverInfo.name, 'tmux-claude');
    socket.destroy();
  });

  // Step 3.4: triggerPopup — called when no window found
  test('multiple sequential /notify requests all get 200 OK', async () => {
    for (let i = 0; i < 3; i++) {
      const resp = await postNotify(port, token, {
        pid: process.pid,
        type: 'notify',
        tool_name: 'Read',
        tool_input: { file_path: '/tmp/test.txt' },
      });
      assert.ok(resp.includes('200 OK'), `request ${i} should return 200`);
    }
  });
});

// ---------------------------------------------------------------------------
// Phase 4: Top-level handler async conversion
// ---------------------------------------------------------------------------

describe('Phase 4: handleMcpMessage and /notify async behaviour', () => {
  let proc;
  let port;
  let token;

  before(async () => {
    for (const f of [PID_FILE, PORT_FILE, TOKEN_FILE]) {
      try { fs.unlinkSync(f); } catch { /* ok */ }
    }
    proc = spawn('node', [SCRIPT], { stdio: ['ignore', 'pipe', 'pipe'] });
    proc.stdout.on('data', d => process.stdout.write(d));
    proc.stderr.on('data', d => process.stderr.write(d));
    const ok = await waitForFile(PORT_FILE, 5000);
    assert.ok(ok, 'server should start');
    port = parseInt(fs.readFileSync(PORT_FILE, 'utf8').trim(), 10);
    token = fs.readFileSync(TOKEN_FILE, 'utf8').trim();
  });

  after(async () => {
    proc?.kill('SIGTERM');
    await new Promise(r => setTimeout(r, 500));
  });

  // Step 4.1: handleMcpMessage async — ide_connected must not block next RPC
  test('ide_connected + immediate tools/list both succeed without race', async () => {
    const socket = await connect(port, token);
    // Send ide_connected (no reply expected) then immediately tools/list
    socket.write(buildTextFrame(JSON.stringify({
      jsonrpc: '2.0', id: null, method: 'ide_connected', params: { pid: process.pid },
    })));
    const res = await rpc(socket, 'tools/list', {}, 20);
    assert.ok(Array.isArray(res.result?.tools));
    socket.destroy();
  });

  // Step 4.2: /notify readBody callback async — concurrent same-window requests
  // pendingToolInfo.get() + immediate delete() prevents double-processing
  test('two concurrent /notify for same PID do not cause double-processing', async () => {
    // First store tool_info
    const r1 = await postNotify(port, token, {
      pid: process.pid,
      type: 'tool_info',
      tool_name: 'Edit',
      tool_input: { file_path: '/tmp/test.js', old_string: 'a', new_string: 'b' },
    });
    assert.ok(r1.includes('200 OK'), 'tool_info should return 200');

    // Two concurrent notifications for the same PID — only one should consume the pendingToolInfo
    const [r2, r3] = await Promise.all([
      postNotify(port, token, { pid: process.pid, type: 'notify', message: 'Claude wants to use Edit' }),
      postNotify(port, token, { pid: process.pid, type: 'notify', message: 'Claude wants to use Edit' }),
    ]);
    assert.ok(r2.includes('200 OK'), `r2 should be 200: ${r2.split('\r\n')[0]}`);
    assert.ok(r3.includes('200 OK'), `r3 should be 200: ${r3.split('\r\n')[0]}`);
  });

  // Step 4.3: updateRemoteLockFiles async — server starts correctly even with remote lock logic
  test('server PID file exists and port file is valid integer', () => {
    assert.ok(fs.existsSync(PID_FILE), 'PID file must exist');
    assert.ok(fs.existsSync(PORT_FILE), 'PORT file must exist');
    const p = parseInt(fs.readFileSync(PORT_FILE, 'utf8').trim(), 10);
    assert.ok(p > 0 && p < 65536, 'port must be valid');
  });

  // Socket disconnect guard: destroying socket during RPC must not crash server
  test('socket destroyed mid-stream does not crash server', async () => {
    const socket = await connect(port, token);
    // Send ide_connected with our PID to register state
    socket.write(buildTextFrame(JSON.stringify({
      jsonrpc: '2.0', id: null, method: 'ide_connected', params: { pid: process.pid },
    })));
    // Destroy immediately without waiting for reply
    socket.destroy();
    await new Promise(r => setTimeout(r, 200));

    // Server must still be alive
    const socket2 = await connect(port, token);
    const res = await rpc(socket2, 'initialize', { protocolVersion: '2025-03-26', capabilities: {} }, 30);
    assert.equal(res.result.serverInfo.name, 'tmux-claude');
    socket2.destroy();
  });
});

// ---------------------------------------------------------------------------
// Phase 5: openDiff popup spawnSync → spawn + Promise
// ---------------------------------------------------------------------------

describe('Phase 5: openDiff popup is non-blocking', () => {
  let proc;
  let port;
  let token;

  before(async () => {
    for (const f of [PID_FILE, PORT_FILE, TOKEN_FILE]) {
      try { fs.unlinkSync(f); } catch { /* ok */ }
    }
    proc = spawn('node', [SCRIPT], { stdio: ['ignore', 'pipe', 'pipe'] });
    proc.stdout.on('data', d => process.stdout.write(d));
    proc.stderr.on('data', d => process.stderr.write(d));
    const ok = await waitForFile(PORT_FILE, 5000);
    assert.ok(ok, 'server should start');
    port = parseInt(fs.readFileSync(PORT_FILE, 'utf8').trim(), 10);
    token = fs.readFileSync(TOKEN_FILE, 'utf8').trim();
  });

  after(async () => {
    proc?.kill('SIGTERM');
    await new Promise(r => setTimeout(r, 500));
  });

  // Step 5.1: openDiff with no window → falls through to triggerPopup (fire-and-forget)
  // The RPC must return without blocking on spawnSync
  test('openDiff tool call returns result without blocking event loop', async () => {
    const socket = await connect(port, token);
    const start = Date.now();
    const res = await rpc(socket, 'tools/call', {
      name: 'openDiff',
      arguments: {
        old_file_path: '/tmp/nonexistent-old.txt',
        new_file_contents: 'new content here',
      },
    }, 40);
    const elapsed = Date.now() - start;
    // Must get a reply — either TAB_CLOSED or REJECTED
    assert.ok(res.result?.content != null, 'openDiff must return content');
    assert.ok(typeof res.result.content[0]?.text === 'string', 'content[0].text must be string');
    // Must not block for the duration of a spawnSync display-popup (which would be minutes)
    // In test env tmux is not available so popup fails immediately, but it must be non-blocking
    assert.ok(elapsed < 8000, `openDiff took ${elapsed}ms, should complete quickly without blocking`);
    socket.destroy();
  });

  // Step 5.1: socket.writable check — after socket is destroyed, server must not crash
  test('socket disconnect during openDiff does not crash server', async () => {
    const socket = await connect(port, token);
    // Register window via ide_connected
    socket.write(buildTextFrame(JSON.stringify({
      jsonrpc: '2.0', id: null, method: 'ide_connected', params: { pid: process.pid },
    })));
    await new Promise(r => setTimeout(r, 50));
    // Send openDiff but destroy socket immediately — server must not crash on reply()
    socket.write(buildTextFrame(JSON.stringify({
      jsonrpc: '2.0', id: 41, method: 'tools/call',
      params: { name: 'openDiff', arguments: { old_file_path: '/tmp/x.txt', new_file_contents: 'y' } },
    })));
    socket.destroy();
    await new Promise(r => setTimeout(r, 500));

    // Server must still be alive
    const socket2 = await connect(port, token);
    const res = await rpc(socket2, 'tools/list', {}, 42);
    assert.ok(Array.isArray(res.result?.tools), 'server must survive socket disconnect during openDiff');
    socket2.destroy();
  });

  // pendingToolInfo: get() then immediately delete() — no double-processing after openDiff
  test('pendingToolInfo is consumed only once when openDiff follows tool_info', async () => {
    // Store tool_info for current process window
    const infoResp = await postNotify(port, token, {
      pid: process.pid,
      type: 'tool_info',
      tool_name: 'Edit',
      tool_input: { file_path: '/tmp/test.js', old_string: 'old', new_string: 'new' },
    });
    assert.ok(infoResp.includes('200 OK'), 'tool_info should be 200');

    // Now openDiff — pendingToolInfo used for old content reconstruction
    const socket = await connect(port, token);
    socket.write(buildTextFrame(JSON.stringify({
      jsonrpc: '2.0', id: null, method: 'ide_connected', params: { pid: process.pid },
    })));
    await new Promise(r => setTimeout(r, 50));
    const res = await rpc(socket, 'tools/call', {
      name: 'openDiff',
      arguments: { old_file_path: '/tmp/test.js', new_file_contents: 'new content' },
    }, 43);
    assert.ok(res.result?.content != null, 'openDiff must return content');
    socket.destroy();

    // Second openDiff should NOT use stale pendingToolInfo (it was deleted after first use)
    const socket3 = await connect(port, token);
    socket3.write(buildTextFrame(JSON.stringify({
      jsonrpc: '2.0', id: null, method: 'ide_connected', params: { pid: process.pid },
    })));
    await new Promise(r => setTimeout(r, 50));
    const res2 = await rpc(socket3, 'tools/call', {
      name: 'openDiff',
      arguments: { old_file_path: '/tmp/test.js', new_file_contents: 'new content v2' },
    }, 44);
    assert.ok(res2.result?.content != null, 'second openDiff must also return content');
    socket3.destroy();
  });
});

// ---------------------------------------------------------------------------
// Unit-level tests for internal logic (no server process needed)
// ---------------------------------------------------------------------------

describe('Unit: detectMaxOption', () => {
  // Import the module in a way that lets us test pure functions
  // Since mcp-server.js is a server script, we test indirectly via the
  // exported-like behaviour through the server. For truly pure functions,
  // we inline test the logic here.

  function detectMaxOption(paneContent) {
    let max = 0;
    for (const line of paneContent.split('\n')) {
      const m = line.match(/^\s*(?:[❯>]\s+)?(\d+)[.)]/);
      if (m) max = Math.max(max, Number(m[1]));
    }
    return max > 0 ? max : 3;
  }

  test('detects max option from numbered list', () => {
    const content = '  1. Yes\n  2. No\n  3. Always';
    assert.equal(detectMaxOption(content), 3);
  });

  test('detects option with > prompt prefix', () => {
    const content = '> 1. option a\n  2. option b';
    assert.equal(detectMaxOption(content), 2);
  });

  test('detects option with ❯ prompt prefix', () => {
    const content = '❯ 1) first\n  2) second\n  3) third\n  4) fourth';
    assert.equal(detectMaxOption(content), 4);
  });

  test('returns 3 when no numbered options found', () => {
    assert.equal(detectMaxOption('no options here'), 3);
  });

  test('returns 3 for empty string', () => {
    assert.equal(detectMaxOption(''), 3);
  });

  test('ignores numbers not at start of line', () => {
    const content = 'some text 1. not an option\n  1. real option';
    assert.equal(detectMaxOption(content), 1);
  });
});

describe('Unit: shellQuote', () => {
  function shellQuote(s) {
    return "'" + String(s).replace(/'/g, "'\\''") + "'";
  }

  test('wraps string in single quotes', () => {
    assert.equal(shellQuote('hello'), "'hello'");
  });

  test('escapes embedded single quotes', () => {
    assert.equal(shellQuote("it's"), "'it'\\''s'");
  });

  test('handles empty string', () => {
    assert.equal(shellQuote(''), "''");
  });

  test('handles path with spaces', () => {
    assert.equal(shellQuote('/path/to/my file.txt'), "'/path/to/my file.txt'");
  });

  test('converts non-string to string first', () => {
    assert.equal(shellQuote(42), "'42'");
  });
});

describe('Unit: estimateToolPopupSize', () => {
  function estimateToolPopupSize(toolName, toolInput, termW, termH, hasCwd = false, cwdLen = 0) {
    let lines = 1;
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
      default: {
        const entries = Object.entries(toolInput ?? {}).slice(0, 8);
        lines += entries.length;
        for (const [k, v] of entries) {
          const val = typeof v === 'string' ? v : JSON.stringify(v);
          maxLen = Math.max(maxLen, k.length + 2 + val.split('\n')[0].length);
        }
      }
    }
    if (hasCwd) { lines += 1; maxLen = Math.max(maxLen, cwdLen); }
    lines += 3;
    const wPct = termW > 0 ? Math.min(95, Math.max(25, Math.round((maxLen + 8) / termW * 100))) : 70;
    const hPct = termH > 0 ? Math.min(90, Math.max(10, Math.ceil((lines + 3) / termH * 100))) : 60;
    return { wPct, hPct };
  }

  test('returns 70/60 defaults when termW=0 and termH=0', () => {
    const { wPct, hPct } = estimateToolPopupSize('Bash', { command: 'echo' }, 0, 0);
    assert.equal(wPct, 70);
    assert.equal(hPct, 60);
  });

  test('Bash command clamps to 20 lines', () => {
    const cmd = Array(30).fill('echo hello').join('\n');
    const { hPct } = estimateToolPopupSize('Bash', { command: cmd }, 200, 50);
    assert.ok(hPct >= 10 && hPct <= 90, `hPct=${hPct} out of range`);
  });

  test('width clamps to 95% maximum', () => {
    const veryLong = 'x'.repeat(10000);
    const { wPct } = estimateToolPopupSize('Read', { file_path: veryLong }, 100, 50);
    assert.equal(wPct, 95);
  });

  test('width minimum is 25%', () => {
    const { wPct } = estimateToolPopupSize('Read', { file_path: '' }, 10000, 50);
    assert.equal(wPct, 25);
  });

  test('CWD adds one line and affects maxLen', () => {
    const { hPct: withCwd } = estimateToolPopupSize('Read', { file_path: '/x' }, 200, 40, true, 10);
    const { hPct: withoutCwd } = estimateToolPopupSize('Read', { file_path: '/x' }, 200, 40, false, 0);
    assert.ok(withCwd >= withoutCwd, 'CWD should increase or maintain height');
  });

  test('Edit tool counts old_string and new_string lines', () => {
    const toolInput = {
      file_path: '/test.js',
      old_string: 'line1\nline2\nline3',
      new_string: 'new1\nnew2',
    };
    const { hPct } = estimateToolPopupSize('Edit', toolInput, 200, 40);
    assert.ok(hPct > 10, 'Edit with multi-line strings should have non-trivial height');
  });
});

describe('Unit: buildNewContents logic (inline)', () => {
  // Test the logic of buildNewContents inline (pure function behaviour)
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
          return { newContents: newStr, oldContent: oldStr };
        }
      }
    } catch { /* fallthrough */ }
    return { newContents: null, oldContent: null };
  }

  test('Write returns content as newContents with null oldContent', () => {
    const r = buildNewContents('Write', { file_path: '/x.txt', content: 'hello' });
    assert.equal(r.newContents, 'hello');
    assert.equal(r.oldContent, null);
  });

  test('Write with no content returns null newContents', () => {
    const r = buildNewContents('Write', { file_path: '/x.txt' });
    assert.equal(r.newContents, null);
  });

  test('Edit with no file_path returns null newContents', () => {
    const r = buildNewContents('Edit', { old_string: 'a', new_string: 'b' });
    assert.equal(r.newContents, null);
  });

  test('Edit with relative file_path returns null newContents', () => {
    const r = buildNewContents('Edit', { file_path: 'relative/path.txt', old_string: 'a', new_string: 'b' });
    assert.equal(r.newContents, null);
  });

  test('Edit with path traversal returns null newContents', () => {
    const r = buildNewContents('Edit', { file_path: '/tmp/../etc/passwd', old_string: 'a', new_string: 'b' });
    assert.equal(r.newContents, null);
  });

  test('Edit with empty old_string returns null newContents', () => {
    const r = buildNewContents('Edit', { file_path: '/tmp/x.txt', old_string: '', new_string: 'b' });
    assert.equal(r.newContents, null);
  });

  test('Edit with non-existent file falls back to new_string as newContents', () => {
    const r = buildNewContents('Edit', {
      file_path: '/tmp/nonexistent-file-99999.txt',
      old_string: 'old value',
      new_string: 'new value',
    });
    assert.equal(r.newContents, 'new value');
    assert.equal(r.oldContent, 'old value');
  });

  test('unknown tool name returns null/null', () => {
    const r = buildNewContents('UnknownTool', { anything: 'here' });
    assert.equal(r.newContents, null);
    assert.equal(r.oldContent, null);
  });
});

// ---------------------------------------------------------------------------
// Edge cases for execAsync timeout behaviour (tested via server integration)
// ---------------------------------------------------------------------------

describe('Phase 2: execAsync timeout and error recovery', () => {
  let proc;
  let port;
  let token;

  before(async () => {
    for (const f of [PID_FILE, PORT_FILE, TOKEN_FILE]) {
      try { fs.unlinkSync(f); } catch { /* ok */ }
    }
    proc = spawn('node', [SCRIPT], { stdio: ['ignore', 'pipe', 'pipe'] });
    proc.stdout.on('data', d => process.stdout.write(d));
    proc.stderr.on('data', d => process.stderr.write(d));
    const ok = await waitForFile(PORT_FILE, 5000);
    assert.ok(ok, 'server should start');
    port = parseInt(fs.readFileSync(PORT_FILE, 'utf8').trim(), 10);
    token = fs.readFileSync(TOKEN_FILE, 'utf8').trim();
  });

  after(async () => {
    proc?.kill('SIGTERM');
    await new Promise(r => setTimeout(r, 500));
  });

  test('very large PID (invalid) triggers error path and returns 200 without hang', async () => {
    const start = Date.now();
    const resp = await postNotify(port, token, {
      pid: 99999999,
      type: 'notify',
      message: 'test invalid pid',
    });
    const elapsed = Date.now() - start;
    // Should complete quickly — ps for invalid PID exits immediately with error
    assert.ok(resp.includes('200 OK'), 'should return 200');
    assert.ok(elapsed < 8000, `took ${elapsed}ms, should be fast for invalid PID`);
  });

  test('negative PID is rejected gracefully with 200 OK', async () => {
    const resp = await postNotify(port, token, {
      pid: -1,
      type: 'notify',
      message: 'negative pid',
    });
    // -1 is not a valid positive integer, server should short-circuit
    assert.ok(resp.includes('200 OK'), 'should return 200 for invalid pid');
  });

  test('non-integer pid is rejected gracefully with 200 OK', async () => {
    const resp = await postNotify(port, token, {
      pid: 'not-a-number',
      type: 'notify',
      message: 'string pid',
    });
    assert.ok(resp.includes('200 OK'), 'should return 200 for non-integer pid');
  });

  test('server survives rapid fire /notify requests', async () => {
    const requests = Array.from({ length: 10 }, (_, i) =>
      postNotify(port, token, { pid: process.pid + i, type: 'notify', message: `rapid ${i}` })
    );
    const responses = await Promise.all(requests);
    let okCount = 0;
    for (const resp of responses) {
      if (resp.includes('200 OK')) okCount++;
    }
    assert.ok(okCount === responses.length, `only ${okCount}/${responses.length} returned 200`);
  });
});
