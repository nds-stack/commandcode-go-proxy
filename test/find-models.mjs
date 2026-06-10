import fs from 'fs';
import path from 'path';
import os from 'os';
const c = fs.readFileSync(path.join(os.homedir(), '.npm-global', 'node_modules', 'command-code', 'dist', 'index.mjs'), 'utf-8');
const idx = c.indexOf('un=');
console.log('un at', idx);
let end = c.indexOf('};', idx);
let content = c.substring(idx, Math.min(end + 5, idx + 5000));
const matches = content.match(/id:"[^"]+"/g);
if (matches) {
  console.log('\nModel IDs:');
  matches.forEach(m => console.log('  ', m));
}
