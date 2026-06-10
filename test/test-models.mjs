import http from 'http';
import fs from 'fs';
import path from 'path';
import os from 'os';

const authPath = path.join(os.homedir(), '.commandcode', 'auth.json');
let apiKey;
try {
  const auth = JSON.parse(fs.readFileSync(authPath, 'utf-8'));
  apiKey = auth.apiKey;
} catch {
  apiKey = process.env.COMMAND_CODE_API_KEY || '';
}
if (!apiKey) { console.error('No API key found'); process.exit(1); }

const tests = [
  // current format
  { model: 'deepseek-v4-flash', desc: 'alias -> deepseek/deepseek-v4-flash' },
  // full format directly
  { model: 'deepseek/deepseek-v4-flash', desc: 'full ID' },
  // just model name
  { model: 'deepseek-v4-flash', desc: 'short name' },
  // kimi opensource model  
  { model: 'kimi-k2.5', desc: 'alias -> moonshotai/Kimi-K2.5' },
  // glm opensource model
  { model: 'glm-5.1', desc: 'alias -> zai-org/GLM-5.1' },
];

async function test(model, desc) {
  return new Promise((resolve) => {
    const body = JSON.stringify({
      model,
      messages: [{ role: 'user', content: 'say hi' }],
      stream: false,
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
        try {
          const p = JSON.parse(data);
          console.log(`[${res.statusCode}] ${desc}: ${p.error ? p.error.message : 'OK: ' + (p.choices?.[0]?.message?.content || '')}`);
        } catch { console.log(`[${res.statusCode}] ${desc}: raw`); }
        resolve();
      });
    });
    req.on('error', (e) => { console.log(`[ERR] ${desc}: ${e.message}`); resolve(); });
    req.write(body);
    req.end();
  });
}

(async () => {
  for (const t of tests) await test(t.model, t.desc);
})();
