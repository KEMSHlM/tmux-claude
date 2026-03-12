'use strict';

const { describe, test, before, after } = require('node:test');
const assert = require('node:assert/strict');
const net = require('node:net');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const { spawn } = require('node:child_process');

const SCRIPT = path.join(__dirname, 'mcp-server.js');
const LOCK_DIR = path.join(os.homedir(), '.claude', 'ide');
const PID_FILE = '/tmp/tmux-claude-mcp.pid';
const PORT_FILE = '/tmp/tmux-claude-mcp.port';
const TOKEN_FILE = '/tmp/tmux-claude-mcp.token';
const WS_MAGIC = '258EAFA5-E914-47DA-95CA-C5AB0DC85B11';

// --- helpers ---

async function waitForFile(filePath, timeout = 3000) {
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
    setTimeout(() => { socket.off('data', onData); reject(new Error('rpc timeout')); }, 3000);
  });
}

// --- test suite ---

describe('mcp-server Phase 1', () => {
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

    const ok = await waitForFile(PORT_FILE, 4000);
    assert.ok(ok, 'server should start within 4 seconds');

    port = parseInt(fs.readFileSync(PORT_FILE, 'utf8').trim(), 10);
    token = fs.readFileSync(TOKEN_FILE, 'utf8').trim();
  });

  after(async () => {
    proc?.kill('SIGTERM');
    await new Promise(r => setTimeout(r, 500));
  });

  test('lock file is created with correct JSON schema', () => {
    const lockPath = path.join(LOCK_DIR, `${port}.lock`);
    assert.ok(fs.existsSync(lockPath), 'lock file must exist');
    const data = JSON.parse(fs.readFileSync(lockPath, 'utf8'));
    assert.equal(typeof data.pid, 'number');
    assert.ok(data.pid > 0);
    assert.ok(Array.isArray(data.workspaceFolders));
    assert.equal(data.ideName, 'tmux-claude');
    assert.equal(data.transport, 'ws');
    assert.equal(typeof data.authToken, 'string');
    assert.ok(data.authToken.length > 0);
  });

  test('pid / port / token temp files are created', () => {
    assert.ok(fs.existsSync(PID_FILE));
    assert.ok(fs.existsSync(PORT_FILE));
    assert.ok(fs.existsSync(TOKEN_FILE));
    assert.ok(port > 0 && port < 65536);
  });

  test('valid token is accepted (HTTP 101)', async () => {
    const socket = await connect(port, token);
    socket.destroy();
  });

  test('invalid token is rejected (HTTP 401)', async () => {
    await assert.rejects(() => connect(port, 'wrong-token'), /401/);
  });

  test('initialize returns serverInfo with name tmux-claude', async () => {
    const socket = await connect(port, token);
    const res = await rpc(socket, 'initialize', { protocolVersion: '2025-03-26', capabilities: {} });
    assert.equal(res.result.serverInfo.name, 'tmux-claude');
    socket.destroy();
  });

  test('tools/list returns empty tools array', async () => {
    const socket = await connect(port, token);
    const res = await rpc(socket, 'tools/list', {}, 2);
    assert.ok(Array.isArray(res.result?.tools), 'tools must be an array');
    socket.destroy();
  });

  test('multiple simultaneous connections are accepted', async () => {
    const [s1, s2] = await Promise.all([connect(port, token), connect(port, token)]);
    const [r1, r2] = await Promise.all([
      rpc(s1, 'initialize', { protocolVersion: '2025-03-26', capabilities: {} }, 10),
      rpc(s2, 'initialize', { protocolVersion: '2025-03-26', capabilities: {} }, 11),
    ]);
    assert.equal(r1.result.serverInfo.name, 'tmux-claude');
    assert.equal(r2.result.serverInfo.name, 'tmux-claude');
    s1.destroy();
    s2.destroy();
  });

  test('lock file is deleted after SIGTERM', async () => {
    const lockPath = path.join(LOCK_DIR, `${port}.lock`);
    assert.ok(fs.existsSync(lockPath));
    proc.kill('SIGTERM');
    await new Promise(r => setTimeout(r, 800));
    assert.ok(!fs.existsSync(lockPath));
    assert.ok(!fs.existsSync(PID_FILE));
  });
});

