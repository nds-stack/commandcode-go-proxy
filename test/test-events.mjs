import https from 'https';
import fs from 'fs';
import path from 'path';
import os from 'os';

const authPath = path.join(os.homedir(), '.commandcode', 'auth.json');
let apiKey;
try {
  const auth = JSON.parse(fs.readFileSync(authPath, 'utf-8'));
  apiKey = auth.apiKey;
} catch { apiKey = ''; }
if (!apiKey) { process.exit(1); }

const body = JSON.stringify({
  config: { workingDir: '.', date: '2026-06-09', environment: 'cli', structure: [], isGitRepo: false, currentBranch: '', mainBranch: 'main', gitStatus: '', recentCommits: [] },
  memory: '', taste: null, skills: null, permissionMode: 'standard',
  params: {
    model: 'deepseek/deepseek-v4-flash',
    messages: [{ role: 'user', content: [{ type: 'text', text: 'what is 2+2, think step by step first then answer' }] }],
    tools: [], system: '', max_tokens: 64000, stream: true,
  },
});

const opts = {
  hostname: 'api.commandcode.ai', path: '/alpha/generate', method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${apiKey}`,
    'x-command-code-version': '0.33.2',
    'x-cli-environment': 'production',
    'Accept': 'text/event-stream',
  },
};

const req = https.request(opts, (res) => {
  let buf = '';
  res.on('data', (chunk) => {
    buf += chunk.toString();
    const lines = buf.split('\n');
    buf = lines.pop() || '';
    for (const line of lines) {
      const t = line.trim();
      if (!t) continue;
      try {
        const ev = JSON.parse(t);
        if (ev.type && !['text-delta', 'start', 'start-step', 'tool-use', 'tool-delta'].includes(ev.type)) {
          console.log('EVENT TYPE:', ev.type, JSON.stringify(ev).substring(0, 200));
        }
      } catch {}
    }
  });
  res.on('end', () => {
    console.log('=== DONE ===');
  });
});
req.on('error', (e) => console.error(e));
req.write(body); req.end();

setTimeout(() => process.exit(0), 10000);
