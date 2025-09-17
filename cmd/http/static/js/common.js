export const serviceWorkerSetup = async () => {
  if (!("serviceWorker" in navigator)) {
    document.querySelectorAll(".pwa")?.forEach((el) => {
      el.remove();
    });
    return;
  }

  const unregister = async () => {
    const registrations = await navigator.serviceWorker.getRegistrations();
    // Unregister all existing service workers
    const unregisterPromises = registrations.map((registration) => {
      return registration.unregister();
    });
    await Promise.all(unregisterPromises);
  };

  await assetVersionChecker(async () => {
    await unregister();

    navigator.serviceWorker
      .register("/static/js/min/serviceworker.js", {
        scope: "/",
      })
      .then(() => console.log("Service Worker registered"))
      .catch((err) => console.error("Service Worker registration failed", err));
  });
};

export const assetVersionChecker = async (callback) => {
  const assetVersion = localStorage.getItem("assetsVersion");
  fetch("/static-asset-version")
    .then((response) => response.text())
    .then((latestVersion) => {
      if (latestVersion && latestVersion !== assetVersion) {
        localStorage.setItem("assetsVersion", latestVersion);
        callback?.(latestVersion);
        // not sure if reload is needed, as static assets are reloaded
        // window.location.reload();
      }
    })
    .catch((err) => console.error("Asset version check failed", err));
};

export const notifier = () => {
  const container = document.getElementById("notification");
  if (!container) return;

  let timer = undefined;
  return {
    notify: function (msg, delay = 1000) {
      if (!container || !msg) return;
      if (timer) clearTimeout(timer);

      container.innerHTML = msg;
      container.style.zIndex = 15;
      container.style.top = 0;

      timer = window.setTimeout(() => {
        container.style.top = "-4em";
        container.style.zIndex = -1;
        // clearing content within timeout to avoid animation stutter
        window.setTimeout(() => {
          container.innerHTML = "";
        }, 1000);

        timer = undefined;
      }, delay);
    },
  };
};

function reloadCSS() {
  document.querySelectorAll('link[rel="stylesheet"]').forEach((link) => {
    const newLink = link.cloneNode();
    // Add a cache-busting query param
    newLink.href = link.href.split("?")[0] + "?v=" + Date.now();
    link.parentNode.insertBefore(newLink, link.nextSibling);
    link.remove();
  });
}

function reloadScript() {
  document.querySelectorAll("script").forEach((script) => {
    const newScript = document.createElement("script");
    newScript.src = script.src.split("?")[0] + "?v=" + Date.now();
    newScript.async = script.async; // maintain order if needed
    newScript.type = script.type;
    script.parentNode.insertBefore(newScript, script.nextSibling);
    script.remove();
  });
}

export const reloadStaticAssets = () => {
  assetVersionChecker(() => {
    reloadCSS();
    reloadScript();
  });
};
