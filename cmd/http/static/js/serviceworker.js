const CACHE_NAME = "phispr-cache-v23";
const NETWORK_TIMEOUT = 500; // milliseconds

const ASSETS = [
  "/",
  "/static/css/normalize.min.css",
  "/static/css/main.min.css",
  "/static/css/themes.css",

  "/static/images/boot.svg",
  "/static/images/github-dark.svg",
  "/static/images/github.svg",
  "/static/images/home.svg",
  "/static/images/leave.svg",
  "/static/images/phispr-active.svg",
  "/static/images/phispr-dark.svg",
  "/static/images/phispr.svg",
  "/static/images/refresh.svg",

  "/static/images/icon-48.png",
  "/static/images/icon-72.png",
  "/static/images/icon-96.png",
  "/static/images/icon-144.png",
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

// Helper function to create a timeout promise
function timeoutPromise(ms) {
  return new Promise((_, reject) => {
    setTimeout(() => reject(new Error("Network timeout")), ms);
  });
}

// Helper function to get from cache
function getFromCache(request) {
  return caches.match(request).then((cachedResponse) => {
    if (cachedResponse) {
      return cachedResponse;
    }

    // If no exact match and it's a document request, fallback to index
    if (request.destination === "document") {
      return caches.match("/");
    }

    // For other resources, return null
    return null;
  });
}

// Fetch: Network First with timeout fallback to cache
self.addEventListener("fetch", (event) => {
  event.respondWith(
    Promise.race([
      // Network request
      fetch(event.request).then((networkResponse) => {
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
      }),

      // Timeout promise that rejects after NETWORK_TIMEOUT ms
      timeoutPromise(NETWORK_TIMEOUT),
    ]).catch(() => {
      // Either network failed or timed out - try cache as fallback
      return getFromCache(event.request).then((cachedResponse) => {
        if (cachedResponse) {
          return cachedResponse;
        }

        // If no cache available, try one more network request without timeout
        // This handles cases where cache is empty and we need to wait for slow network
        return fetch(event.request).catch(() => {
          // Complete failure - return null for non-document requests
          if (event.request.destination !== "document") {
            return new Response("", { status: 404 });
          }

          // For document requests, try to return cached index as last resort
          return caches.match("/") || new Response("Offline", { status: 503 });
        });
      });
    })
  );
});
