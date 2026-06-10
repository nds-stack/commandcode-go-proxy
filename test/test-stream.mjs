import http from 'http';
import fs from 'fs';
import path from 'path';
import os from 'os';

const auth = JSON.parse(fs.readFileSync(path.join(os.homedir(), '.commandcode', 'auth.json'), 'utf-8'));
const apiKey = auth.apiKey;

const body = JSON.stringify({
  model: 'deepseek/deepseek-v4-flash',
  messages: [{ role: 'user', content: 'what is 2+2, think step by step' }],
  stream: true,
});

const opts = {
  hostname: '127.0.0.1', port: 9173, path: '/v1/chat/completions', method: 'POST',
  headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${apiKey}`, 'Content-Length': Buffer.byteLength(body) },
};

console.log('=== STREAMING RAW CHUNKS ===\n');

const req = http.request(opts, (res) => {
  let buf = '';
  let chunkCount = 0;
  res.on('data', (chunk) => {
    buf += chunk.toString();
    const lines = buf.split('\n');
    buf = lines.pop() || '';
    for (const line of lines) {
      const t = line.trim();
      if (!t || !t.startsWith('data:')) continue;
      const json = t.slice(5).trim();
      if (json === '[DONE]') continue;
      try {
        const ev = JSON.parse(json);
        const delta = ev.choices?.[0]?.delta || {};
        chunkCount++;
        if (delta.reasoning_content) {
          process.stdout.write(`\x1b[33m${delta.reasoning_content}\x1b[0m`);
        }
        if (delta.content) {
          process.stdout.write(`\x1b[36m${delta.content}\x1b[0m`);
        }
      } catch {}
    }
  });
  res.on('end', () => {
    console.log(`\n\nTotal chunks: ${chunkCount}`);
  });
});
req.on('error', (e) => console.error(e));
req.write(body);
req.end();
