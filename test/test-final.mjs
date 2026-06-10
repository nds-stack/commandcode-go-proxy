import http from 'http';
import fs from 'fs';
import path from 'path';
import os from 'os';

const auth = JSON.parse(fs.readFileSync(path.join(os.homedir(), '.commandcode', 'auth.json'), 'utf-8'));
const apiKey = auth.apiKey;

const body = JSON.stringify({
  model: 'deepseek/deepseek-v4-flash',
  messages: [{ role: 'user', content: 'what is 2+2, think step by step' }],
  stream: false,
});

const opts = {
  hostname: '127.0.0.1', port: 9173, path: '/v1/chat/completions', method: 'POST',
  headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${apiKey}`, 'Content-Length': Buffer.byteLength(body) },
};

const req = http.request(opts, (res) => {
  let data = '';
  res.on('data', (c) => { data += c; });
  res.on('end', () => {
    const p = JSON.parse(data);
    console.log('=== CONTENT ===');
    console.log(p.choices[0].message.content);
    console.log('\n=== REASONING ===');
    console.log(p.choices[0].message.reasoning_content || '(none)');
    console.log('\n=== FULL RESPONSE STRUCTURE ===');
    console.log(JSON.stringify(p.choices[0].message, null, 2));
  });
});
req.on('error', (e) => console.error(e));
req.write(body);
req.end();
