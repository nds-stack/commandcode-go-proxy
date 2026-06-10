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

let reasonCount = 0, textCount = 0;

const req = http.request(opts, (res) => {
  let buf = '';
  let reasoningFull = '';
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
        if (delta.reasoning_content) {
          reasonCount++;
          reasoningFull += delta.reasoning_content;
        }
        if (delta.content) textCount++;
      } catch {}
    }
  });
  res.on('end', () => {
    console.log('Reasoning chunks:', reasonCount);
    console.log('Text chunks:', textCount);
    console.log('\nFull reasoning text:\n' + reasoningFull);
  });
});
req.on('error', (e) => console.error(e));
req.write(body);
req.end();
