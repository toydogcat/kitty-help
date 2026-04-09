self.addEventListener('install', (e) => {
  e.waitUntil(
    caches.open('kitty-help-v1').then((cache) => {
      // Basic offline support for the landing and manifest
      return cache.addAll([
        '/',
        '/manifest.json',
        'https://cdn-icons-png.flaticon.com/512/1864/1864509.png'
      ]);
    })
  );
});

self.addEventListener('fetch', (e) => {
  e.respondWith(
    caches.match(e.request).then((response) => {
      return response || fetch(e.request);
    })
  );
});
