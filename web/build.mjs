import esbuild from 'esbuild';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.join(root, 'dist');
const devMode = process.env.NODE_ENV === 'development';
const version = process.env.VERSION || 'dev';

fs.rmSync(outDir, { recursive: true, force: true });
fs.mkdirSync(outDir, { recursive: true });

// Copy public/ → dist/
copyDir(path.join(root, 'public'), outDir);

// Copy CSS straight through.
fs.copyFileSync(path.join(root, 'src/styles.css'), path.join(outDir, 'styles.css'));

await esbuild.build({
  entryPoints: [path.join(root, 'src/main.jsx')],
  bundle: true,
  outfile: path.join(outDir, 'app.js'),
  format: 'esm',
  target: ['es2020'],
  jsx: 'automatic',
  jsxImportSource: 'preact',
  minify: !devMode,
  sourcemap: devMode,
  define: {
    'process.env.NODE_ENV': JSON.stringify(devMode ? 'development' : 'production'),
    '__APP_VERSION__': JSON.stringify(version),
  },
  logLevel: 'info',
});

console.log(`built → ${outDir} (version=${version})`);

function copyDir(src, dest) {
  if (!fs.existsSync(src)) return;
  for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
    const s = path.join(src, entry.name);
    const d = path.join(dest, entry.name);
    if (entry.isDirectory()) {
      fs.mkdirSync(d, { recursive: true });
      copyDir(s, d);
    } else {
      fs.copyFileSync(s, d);
    }
  }
}
