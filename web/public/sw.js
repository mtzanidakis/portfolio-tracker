// Every JS/CSS asset shipped by the build has a content-hashed filename,
// so they are immutable and safe to cache-first forever. The few unhashed
// entry points (index.html, manifest.json, the icon) go network-first so
// deploys propagate immediately.
const CACHE = 'pt-static-v3';

const NETWORK_FIRST = new Set([
  '/',
  '/index.html',
  '/manifest.json',
]);

self.addEventListener('install', (e) => {
  e.waitUntil(
    caches.open(CACHE)
      .then((c) => c.addAll([...NETWORK_FIRST]))
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
