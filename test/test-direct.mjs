import http from 'https';
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
if (!apiKey) { console.error('No API key'); process.exit(1); }

const body = JSON.stringify({
  config: { workingDir: '.', date: '2026-06-09', environment: 'cli', structure: [], isGitRepo: false, currentBranch: '', mainBranch: 'main', gitStatus: '', recentCommits: [] },
  memory: '',
  taste: null,
  skills: null,
  permissionMode: 'standard',
  params: {
    model: 'deepseek/deepseek-v4-flash',
    messages: [{ role: 'user', content: [{ type: 'text', text: 'say hi in one word' }] }],
    tools: [],
    system: '',
    max_tokens: 64000,
    temperature: 0.3,
    stream: true,
  },
});

const opts = {
  hostname: 'api.commandcode.ai',
  path: '/alpha/generate',
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${apiKey}`,
    'x-command-code-version': '0.33.2',
    'x-cli-environment': 'production',
    'Host': 'api.commandcode.ai',
    'Accept': 'text/event-stream',
    'Content-Length': Buffer.byteLength(body),
  },
};

const req = http.request(opts, (res) => {
  let data = '';
  res.on('data', (c) => { data += c; });
  res.on('end', () => {
    console.log('Status:', res.statusCode);
    console.log('Headers:', JSON.stringify(res.headers));
    try {
      const p = JSON.parse(data);
      console.log('Response:', JSON.stringify(p, null, 2));
    } catch { console.log('Raw:', data.substring(0, 500)); }
  });
});
req.on('error', (e) => console.error('Error:', e.message));
req.write(body);
req.end();
