import http from 'http';
import fs from 'fs';
import path from 'path';
import os from 'os';

const authPath = path.join(os.homedir(), '.commandcode', 'auth.json');
let apiKey;
try {
  const auth = JSON.parse(fs.readFileSync(authPath, 'utf-8'));
  apiKey = auth.apiKey;
} catch { apiKey = ''; }
if (!apiKey) { console.error('No API key'); process.exit(1); }

// Test 1: proxy with stream: true
function testProxy(stream) {
  return new Promise((resolve) => {
    const body = JSON.stringify({
      model: 'deepseek/deepseek-v4-flash',
      messages: [{ role: 'user', content: 'say hi in one word' }],
      stream,
    });
    const opts = {
      hostname: '127.0.0.1', port: 9173, path: '/v1/chat/completions',
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${apiKey}`, 'Content-Length': Buffer.byteLength(body) },
    };
    const req = http.request(opts, (res) => {
      let data = '';
      res.on('data', (c) => { data += c; });
      res.on('end', () => {
        console.log(`[stream=${stream}] Status: ${res.statusCode}`);
        try {
          const p = JSON.parse(data);
          if (p.error) console.log('  ERROR:', p.error.message?.substring(0, 200));
          else console.log('  OK:', p.choices?.[0]?.message?.content || 'no content');
        } catch { console.log('  Raw (first 200):', data.substring(0, 200)); }
        resolve();
      });
    });
    req.on('error', (e) => { console.log(`[stream=${stream}] Error:`, e.message); resolve(); });
    req.write(body); req.end();
  });
}

(async () => {
  await testProxy(true);
  await testProxy(false);
})();
