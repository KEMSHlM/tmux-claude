const fs = require('fs');
const path = require('path');
const http = require('http');

const log = m => fs.appendFileSync('/tmp/lazyclaude-hook.log', new Date().toISOString() + ' PRETOOLUSE ' + m + '\n');

let d = '';
process.stdin.on('data', c => d += c);
process.stdin.on('end', () => {
  try {
    log('stdin=' + d);
    const i = JSON.parse(d);
    const home = require('os').homedir();
    const lockDir = path.join(home, '.claude', 'ide');
    let locks = [];
    try {
      locks = fs.readdirSync(lockDir).filter(f => f.endsWith('.lock'));
    } catch (e) {
      log('readdir error: ' + e);
    }
    log('locks=' + JSON.stringify(locks));
    if (locks.length) {
      const lock = JSON.parse(fs.readFileSync(path.join(lockDir, locks[0]), 'utf8'));
      const port = parseInt(locks[0], 10);
      log('port=' + port + ' token=' + lock.authToken.slice(0, 8) + '...');
      const body = JSON.stringify({
        type: 'tool_info',
        pid: process.ppid,
        tool_name: i.tool_name || '',
        tool_input: i.tool_input || {}
      });
      const req = http.request({
        hostname: '127.0.0.1',
        port,
        path: '/notify',
        method: 'POST',
        timeout: 300,
        headers: {
          'Content-Type': 'application/json',
          'Content-Length': Buffer.byteLength(body),
          'X-Claude-Code-Ide-Authorization': lock.authToken
        }
      });
      req.on('error', e => { log('POST error: ' + e); });
      req.on('timeout', () => { log('POST timeout'); req.destroy(); });
      req.on('response', res => { log('POST response: ' + res.statusCode); });
      req.write(body);
      req.end();
      log('POST sent');
    }
  } catch (e) {
    log('CATCH: ' + e);
  }
});
