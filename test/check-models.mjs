import fs from 'fs';
import path from 'path';
import os from 'os';
const c = fs.readFileSync(path.join(os.homedir(), '.npm-global', 'node_modules', 'command-code', 'dist', 'index.mjs'), 'utf-8');
const start = c.indexOf('un={');
let depth = 0, i = start, obj = '';
while (i < c.length) {
  const ch = c[i];
  obj += ch;
  if (ch === '{') depth++;
  if (ch === '}') { depth--; if (depth === 0) break; }
  i++;
}
const ids = obj.match(/id:"[^"]+"/g);
console.log('Models:');
ids.forEach(id => console.log('  ' + id));
