const notifier = () => {
  const container = document.getElementById("notification");
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

const serviceWorkerSetup = async () => {
  if (!("serviceWorker" in navigator)) {
    document.querySelectorAll(".pwa").forEach((el) => {
      el.remove();
    });
    return;
  }

  const registrations = await navigator.serviceWorker.getRegistrations();
  // Unregister all existing service workers
  const unregisterPromises = registrations.map((registration) => {
    return registration.unregister();
  });

  await Promise.all(unregisterPromises);
  navigator.serviceWorker
    .register("/static/js/serviceworker.js", {
      start_url: "/",
      scope: "/",
    })
    .then(() => console.log("Service Worker registered"))
    .catch((err) => console.error("Service Worker registration failed", err));
};

const home = () => {
  const newRoomPhantomBadge = document.querySelector("#new-room form .phantom");
  const defaultTitle = newRoomPhantomBadge.getAttribute("title");
  const chkBoxPhantom = document.getElementById("phantom");
  const chkBoxUnlisted = document.getElementById("unlisted");
  serviceWorkerSetup();
  if (
    !newRoomPhantomBadge ||
    !defaultTitle ||
    !chkBoxPhantom ||
    !chkBoxUnlisted
  )
    return;

  const adjustTitle = () => {
    if (chkBoxPhantom.checked && chkBoxUnlisted.checked) {
      newRoomPhantomBadge.setAttribute("title", "Phantom & unlisted room");
    } else {
      newRoomPhantomBadge.setAttribute("title", defaultTitle);
    }
  };
  chkBoxPhantom.addEventListener("change", adjustTitle);
  chkBoxUnlisted.addEventListener("change", adjustTitle);

  const notifications = notifier();
  document.querySelectorAll(".pwa").forEach((el) => {
    const msg = `<a href="https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Guides/Installing" target="_blank">Check how to install PWA</a>`;
    el.addEventListener("click", () => {
      notifications.notify(msg, 3000);
    });
  });
};
home();
