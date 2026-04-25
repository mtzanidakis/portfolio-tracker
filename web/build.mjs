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

// Copy public/ → dist/, but hold index.html until we've picked hashed
// filenames for the JS + CSS and can inject them.
copyDir(path.join(root, 'public'), outDir, (name) => name !== 'index.html');

// Bundle JS with a content-hashed filename.
const jsResult = await esbuild.build({
  entryPoints: [path.join(root, 'src/main.jsx')],
  bundle: true,
  outdir: outDir,
  entryNames: 'app-[hash]',
  format: 'esm',
  target: ['es2020'],
  jsx: 'automatic',
  jsxImportSource: 'preact',
  minify: !devMode,
  sourcemap: devMode,
  metafile: true,
  define: {
    'process.env.NODE_ENV': JSON.stringify(devMode ? 'development' : 'production'),
    '__APP_VERSION__': JSON.stringify(version),
  },
  logLevel: 'info',
});

const appOutputKey = Object.keys(jsResult.metafile.outputs)
  .find((p) => p.endsWith('.js'));
if (!appOutputKey) throw new Error('esbuild produced no .js output');
const appName = path.basename(appOutputKey);

// Bundle CSS through esbuild so that @import (fontsource) and url()
// references to woff2 files in node_modules are resolved + emitted
// alongside the CSS with content-hashed filenames.
const cssResult = await esbuild.build({
  entryPoints: [path.join(root, 'src/styles.css')],
  bundle: true,
  outdir: outDir,
  entryNames: 'styles-[hash]',
  assetNames: 'fonts/[name]-[hash]',
  loader: { '.woff2': 'file', '.woff': 'file' },
  minify: !devMode,
  metafile: true,
  logLevel: 'info',
});
const cssOutputKey = Object.keys(cssResult.metafile.outputs)
  .find((p) => p.endsWith('.css'));
if (!cssOutputKey) throw new Error('esbuild produced no .css output');
const stylesName = path.basename(cssOutputKey);

// Rewrite index.html with the hashed references.
const htmlTemplate = fs.readFileSync(path.join(root, 'public/index.html'), 'utf8');
const html = htmlTemplate
  .replace('/app.js', `/${appName}`)
  .replace('/styles.css', `/${stylesName}`);
fs.writeFileSync(path.join(outDir, 'index.html'), html);

console.log(`built → ${outDir} (js=${appName}, css=${stylesName}, version=${version})`);

function copyDir(src, dest, filter = () => true) {
  if (!fs.existsSync(src)) return;
  for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
    if (!filter(entry.name)) continue;
    const s = path.join(src, entry.name);
    const d = path.join(dest, entry.name);
    if (entry.isDirectory()) {
      fs.mkdirSync(d, { recursive: true });
      copyDir(s, d, filter);
    } else {
      fs.copyFileSync(s, d);
    }
  }
}
