// Network-first for HTML/JS/CSS (so deploys propagate immediately),
// cache-first for everything else (fonts, icons). API is always network.
const CACHE = 'pt-static-v2';

const PRECACHE = [
  '/',
  '/index.html',
  '/app.js',
  '/styles.css',
  '/manifest.json',
];

const NETWORK_FIRST = new Set([
  '/',
  '/index.html',
  '/app.js',
  '/styles.css',
]);

self.addEventListener('install', (e) => {
  e.waitUntil(
    caches.open(CACHE)
      .then((c) => c.addAll(PRECACHE))
      .catch(() => {})
  );
  self.skipWaiting();
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys().then((names) =>
      Promise.all(names.filter((n) => n !== CACHE).map((n) => caches.delete(n)))
    )
  );
  self.clients.claim();
});

self.addEventListener('fetch', (e) => {
  const url = new URL(e.request.url);
  if (url.pathname.startsWith('/api/')) return;
  if (e.request.method !== 'GET') return;
  if (url.origin !== self.location.origin) return;

  if (NETWORK_FIRST.has(url.pathname)) {
    e.respondWith(networkFirst(e.request));
    return;
  }
  e.respondWith(cacheFirst(e.request));
});

async function networkFirst(req) {
  try {
    const resp = await fetch(req);
    if (resp.ok) {
      const copy = resp.clone();
      caches.open(CACHE).then((c) => c.put(req, copy)).catch(() => {});
    }
    return resp;
  } catch {
    const cached = await caches.match(req);
    if (cached) return cached;
    return caches.match('/index.html');
  }
}

async function cacheFirst(req) {
  const cached = await caches.match(req);
  if (cached) return cached;
  try {
    const resp = await fetch(req);
    if (resp.ok) {
      const copy = resp.clone();
      caches.open(CACHE).then((c) => c.put(req, copy)).catch(() => {});
    }
    return resp;
  } catch {
    return caches.match('/index.html');
  }
}
