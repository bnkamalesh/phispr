const CACHE_NAME = "phispr-cache-v19";
const ASSETS = [
  "/",
  "/static/css/normalize.min.css",
  "/static/css/main.min.css",
  "/static/css/themes.css",

  "/static/images/github.svg",
  "/static/images/home.svg",
  "/static/images/leave.svg",
  "/static/images/phispr-active.svg",
  "/static/images/phispr.svg",
  "/static/images/refresh.svg",

  "/static/images/icon-192.png",
  "/static/images/icon-512.png",

  "/static/js/home.min.js",
  "/static/js/sse.min.js",
  "/static/js/qr.min.js",
  "/static/js/room.min.js",
];

// Install: cache everything
self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => cache.addAll(ASSETS))
  );
  self.skipWaiting();
});

// Activate: cleanup old caches
self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(
        keys.map((key) => {
          if (key !== CACHE_NAME) return caches.delete(key);
        })
      )
    )
  );
  self.clients.claim();
});

// Fetch: Network First - only use cache when offline
self.addEventListener("fetch", (event) => {
  event.respondWith(
    fetch(event.request)
      .then((networkResponse) => {
        // Network success - cache the response and return it
        if (
          networkResponse &&
          networkResponse.status <= 300 &&
          event.request.method === "GET"
        ) {
          const responseToCache = networkResponse.clone();
          caches.open(CACHE_NAME).then((cache) => {
            cache.put(event.request, responseToCache);
          });
        }
        return networkResponse;
      })
      .catch(() => {
        // Network failed - try cache as fallback (offline mode)
        return caches.match(event.request).then((cachedResponse) => {
          if (cachedResponse) {
            return cachedResponse;
          }

          // If no exact match and it's a document request, fallback to index
          if (event.request.destination === "document") {
            return caches.match("/");
          }

          // For other resources, return null (will show broken image/404)
          return null;
        });
      })
  );
});
