import https from 'https';
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

function generateUUID() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
    const r = Math.random() * 16 | 0;
    return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
  });
}

const body = JSON.stringify({
  config: {
    workingDir: '.', date: '2026-06-18', environment: 'cli',
    structure: [], isGitRepo: false, currentBranch: '',
    mainBranch: 'main', gitStatus: '', recentCommits: []
  },
  memory: '',
  taste: null,
  skills: null,
  permissionMode: 'standard',
  params: {
    model: 'deepseek/deepseek-v4-flash',
    messages: [
      { role: 'user', content: [{ type: 'text', text: 'what is 2+2? answer in one sentence.' }] },
    ],
    tools: [],
    system: 'You are a helpful assistant. Reply concisely.',
    max_tokens: 1024,
    stream: true,
    reasoning_effort: 'low',
  },
  threadId: generateUUID(),
});

const opts = {
  hostname: 'api.commandcode.ai',
  path: '/alpha/generate',
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${apiKey}`,
    'x-command-code-version': '0.40.0',
    'x-cli-environment': 'production',
    'x-session-id': generateUUID(),
    'x-project-slug': '.',
    'x-taste-learning': 'false',
    'x-co-flag': 'false',
    'Accept': 'text/event-stream',
    'Content-Length': Buffer.byteLength(body),
  },
};

console.log('=== FULL STREAM TEST (v0.40.0 format) ===\n');
console.log('Request body:');
console.log(JSON.parse(body));
console.log('\n--- Raw SSE Events ---\n');

const req = https.request(opts, (res) => {
  console.log(`HTTP ${res.statusCode}`);
  console.log(`Headers:`, JSON.stringify(res.headers, null, 2));
  console.log();

  if (res.statusCode !== 200) {
    let errBuf = '';
    res.on('data', (chunk) => {
      errBuf += chunk.toString();
    });
    res.on('end', () => {
      console.log('Error response:');
      try {
        console.log(JSON.stringify(JSON.parse(errBuf), null, 2));
      } catch {
        console.log(errBuf || '(empty body)');
      }
    });
    return;
  }

  let buf = '';
  let eventCount = 0;
  const events = {};

  res.on('data', (chunk) => {
    buf += chunk.toString();
    const lines = buf.split('\n');
    buf = lines.pop() || '';

    for (const line of lines) {
      const t = line.trim();
      if (!t) continue;

      let parsed;
      try {
        parsed = JSON.parse(t);
      } catch {
        console.log(`  [raw] ${t.substring(0, 200)}`);
        continue;
      }

      eventCount++;
      const type = parsed.type || 'unknown';
      events[type] = (events[type] || 0) + 1;

      if (parsed.error || parsed.success === false) {
        console.log(`  [ERROR] ${JSON.stringify(parsed).substring(0, 500)}`);
        continue;
      }

      const preview = JSON.stringify(parsed).substring(0, 200);
      console.log(`  [${String(eventCount).padStart(2)}] ${type.padEnd(18)} ${preview}`);

      if (type === 'text-delta' && parsed.text) {
        process.stdout.write(`\x1b[36m${parsed.text}\x1b[0m`);
      }
      if (type === 'reasoning-delta' && parsed.text) {
        process.stdout.write(`\x1b[33m${parsed.text}\x1b[0m`);
      }
    }
  });

  res.on('end', () => {
    console.log('\n\n--- Event Summary ---');
    console.log(`Total events: ${eventCount}`);
    for (const [type, count] of Object.entries(events)) {
      console.log(`  ${type.padEnd(20)} ${count}`);
    }
    console.log('\nDone.');
  });
});

req.on('error', (e) => console.error('Error:', e.message));
req.write(body);
req.end();
