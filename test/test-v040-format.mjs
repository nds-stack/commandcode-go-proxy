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

function makeRequest(label, body, headers = {}) {
  return new Promise((resolve, reject) => {
    const bodyStr = JSON.stringify(body);
    const defaultHeaders = {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${apiKey}`,
      'x-command-code-version': '0.40.0',
      'x-cli-environment': 'production',
      'x-session-id': generateUUID(),
      'x-project-slug': '.',
      'x-taste-learning': 'false',
      'x-co-flag': 'false',
      'Accept': 'text/event-stream',
      'Content-Length': Buffer.byteLength(bodyStr),
    };
    const opts = {
      hostname: 'api.commandcode.ai',
      path: '/alpha/generate',
      method: 'POST',
      headers: { ...defaultHeaders, ...headers },
    };

    console.log(`\n${'='.repeat(60)}`);
    console.log(`TEST: ${label}`);
    console.log(`${'='.repeat(60)}`);

    const req = https.request(opts, (res) => {
      let data = '';
      let chunkCount = 0;
      let firstChunk = null;
      let lastChunk = null;
      const timeout = setTimeout(() => {
        console.log('  [TIMEOUT] 30s - no more data');
        res.destroy();
        resolve({ label, chunkCount, firstChunk, lastChunk, data, status: res.statusCode });
      }, 30000);

      res.on('data', (chunk) => {
        chunkCount++;
        const text = chunk.toString();
        if (chunkCount === 1) firstChunk = text.substring(0, 200);
        lastChunk = text.substring(0, 200);
        data += text;
        if (chunkCount <= 5) {
          console.log(`  [chunk ${chunkCount}] ${text.substring(0, 150).replace(/\n/g, '\\n')}`);
        }
      });
      res.on('end', () => {
        clearTimeout(timeout);
        resolve({ label, chunkCount, firstChunk, lastChunk, data, status: res.statusCode });
      });
      res.on('error', (e) => {
        clearTimeout(timeout);
        reject(e);
      });
    });
    req.on('error', reject);
    req.write(bodyStr);
    req.end();
  });
}

async function main() {
  const messages = [{ role: 'user', content: 'say hi in one word' }];
  const baseConfig = { workingDir: '.', date: '2026-06-18', environment: 'cli', structure: [], isGitRepo: false, currentBranch: '', mainBranch: 'main', gitStatus: '', recentCommits: [] };

  const results = [];

  // Test 1: OLD format (what our proxy currently sends)
  try {
    const r = await makeRequest('OLD format (current proxy)', {
      config: baseConfig,
      permissionMode: 'standard',
      params: {
        model: 'deepseek/deepseek-v4-flash',
        messages,
        tools: [],
        system: '',
        max_tokens: 64000,
        temperature: 0,
        stream: true,
      },
    });
    results.push(r);
  } catch (e) { console.error('  ERROR:', e.message); }

  // Test 2: NEW format (what CC CLI v0.40.0 sends)
  try {
    const r = await makeRequest('NEW format (CC CLI v0.40.0)', {
      config: baseConfig,
      memory: '',
      taste: null,
      skills: null,
      permissionMode: 'standard',
      params: {
        model: 'deepseek/deepseek-v4-flash',
        messages,
        tools: [],
        system: '',
        max_tokens: 64000,
        stream: true,
        reasoning_effort: 'low',
      },
      threadId: generateUUID(),
    });
    results.push(r);
  } catch (e) { console.error('  ERROR:', e.message); }

  // Test 3: NEW format + deepseek-v4 (not flash)
  try {
    const r = await makeRequest('NEW format + deepseek-v4', {
      config: baseConfig,
      memory: '',
      taste: null,
      skills: null,
      permissionMode: 'standard',
      params: {
        model: 'deepseek-v4',
        messages,
        tools: [],
        system: '',
        max_tokens: 64000,
        stream: true,
        reasoning_effort: 'low',
      },
      threadId: generateUUID(),
    });
    results.push(r);
  } catch (e) { console.error('  ERROR:', e.message); }

  // Test 4: Only threadId (test if threadId alone fixes it)
  try {
    const r = await makeRequest('Only threadId added', {
      config: baseConfig,
      permissionMode: 'standard',
      params: {
        model: 'deepseek/deepseek-v4-flash',
        messages,
        tools: [],
        system: '',
        max_tokens: 64000,
        stream: true,
      },
      threadId: generateUUID(),
    });
    results.push(r);
  } catch (e) { console.error('  ERROR:', e.message); }

  // Test 5: Only memory/taste/skills (test if these fix it)
  try {
    const r = await makeRequest('Only memory/taste/skills added', {
      config: baseConfig,
      memory: '',
      taste: null,
      skills: null,
      permissionMode: 'standard',
      params: {
        model: 'deepseek/deepseek-v4-flash',
        messages,
        tools: [],
        system: '',
        max_tokens: 64000,
        stream: true,
      },
    });
    results.push(r);
  } catch (e) { console.error('  ERROR:', e.message); }

  // Summary
  console.log('\n' + '='.repeat(60));
  console.log('SUMMARY');
  console.log('='.repeat(60));
  for (const r of results) {
    const status = r.chunkCount > 0 ? 'OK' : 'EMPTY';
    console.log(`  ${status.padEnd(6)} | chunks: ${String(r.chunkCount).padStart(3)} | HTTP ${r.status} | ${r.label}`);
  }
}

main().catch(console.error);
