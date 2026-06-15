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
if (!apiKey) { console.error('No API key'); process.exit(1); }

const body = JSON.stringify({
  config: { workingDir: '.', date: '2026-06-14', environment: 'cli', structure: [], isGitRepo: false, currentBranch: '', mainBranch: 'main', gitStatus: '', recentCommits: [] },
  memory: '', taste: null, skills: null, permissionMode: 'standard',
  params: {
    model: 'deepseek/deepseek-v4-flash',
    messages: [{ role: 'user', content: [{ type: 'text', text: 'what is 2+2, think step by step, then give final answer' }] }],
    tools: [], system: '', max_tokens: 64000, stream: true,
  },
});

const opts = {
  hostname: 'api.commandcode.ai', path: '/alpha/generate', method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${apiKey}`,
    'x-command-code-version': '0.37.2',
    'x-cli-environment': 'production',
    'Accept': 'text/event-stream',
  },
};

console.log('=== RAW CC STREAM ===\n');

const req = https.request(opts, (res) => {
  let buf = '';
  res.on('data', (chunk) => {
    buf += chunk.toString();
    const lines = buf.split('\n');
    buf = lines.pop() || '';
    for (const line of lines) {
      const t = line.trim();
      if (!t) continue;
      console.log(t);
    }
  });
  res.on('end', () => {
    if (buf.trim()) console.log(buf.trim());
    console.log('\n=== END ===');
  });
});
req.on('error', (e) => console.error('Error:', e.message));
req.write(body); req.end();

setTimeout(() => process.exit(0), 30000);
