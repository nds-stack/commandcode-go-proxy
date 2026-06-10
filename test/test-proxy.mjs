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

if (!apiKey) {
  console.error('No API key found');
  process.exit(1);
}

const models = ['deepseek-v4-flash', 'kimi-k2.5'];

async function testModel(model) {
  return new Promise((resolve) => {
    const body = JSON.stringify({
      model,
      messages: [{ role: 'user', content: 'say hi in one word' }],
      stream: false,
    });

    const options = {
      hostname: '127.0.0.1',
      port: 9173,
      path: '/v1/chat/completions',
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${apiKey}`,
        'Content-Length': Buffer.byteLength(body),
      },
    };

    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', (chunk) => { data += chunk; });
      res.on('end', () => {
        console.log(`\n=== Model: ${model} ===`);
        console.log('Status:', res.statusCode);
        try {
          const parsed = JSON.parse(data);
          if (parsed.error) {
            console.log('ERROR:', parsed.error.message);
          } else {
            console.log('Response:', parsed.choices?.[0]?.message?.content);
          }
        } catch {
          console.log('Raw:', data.substring(0, 200));
        }
        resolve();
      });
    });

    req.on('error', (e) => {
      console.log(`\n=== Model: ${model} ===`);
      console.log('Request failed:', e.message);
      resolve();
    });
    req.write(body);
    req.end();
  });
}

(async () => {
  for (const model of models) {
    await testModel(model);
  }
})();
