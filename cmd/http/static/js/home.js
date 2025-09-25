import { notifier, reloadStaticAssets } from "./common.js";

let loadAllInprogress = false;
const loadAll = () => {
  if (loadAllInprogress) return;
  loadAllInprogress = true;

  const icoPhantom = document.createElement("span");
  const icoUnlistedPhantom = document.createElement("span");
  icoPhantom.classList.add("ico", "phantom");
  icoUnlistedPhantom.classList.add("ico", "active", "phantom");

  const publicRooms = document.getElementById("public-chat-rooms");
  let existingPublicRooms = {};
  publicRooms?.querySelectorAll("li.room").forEach((room) => {
    existingPublicRooms[room.dataset.roomid] = true;
  });

  const unlistedRooms = document.getElementById("unlisted-chat-rooms");
  let existingUnlistedRooms = {};
  unlistedRooms?.querySelectorAll("li.room").forEach((room) => {
    existingUnlistedRooms[room.dataset.roomid] = true;
  });

  const roomCounts = document.getElementById("live-rooms-count");
  const currentRelease = document.getElementById("current-release");

  const fetcher = (callback) => {
    fetch("/", {
      headers: {
        "Content-Type": "application/json",
      },
      method: "GET",
    })
      .then((response) => response.json())
      .then((payload) => {
        if (!payload || !payload?.data) return;
        callback?.(payload?.data);
      })
      .catch((err) => console.error("Failed fetching info", err));
  };

  const diffFillRoomList = (roomList, container, existingRoomIDs) => {
    const allNodes = [];

    roomList?.forEach((room) => {
      if (!room || existingRoomIDs[room.ID]) return;
      const li = document.createElement("li");
      li.classList.add("room");
      li.dataset.roomid = room.ID;

      const a = document.createElement("a");
      a.classList.add("room-link");
      a.href = `/rooms/${room.ID}`;

      const roomName = document.createElement("span");
      roomName.classList.add("room-name");
      roomName.innerText = room.Name;

      const roomMemCount = document.createElement("span");
      roomMemCount.classList.add("room-members-count");
      roomMemCount.innerText = `(${Array(room.Members).length} / ${
        room.Capacity
      })`;

      if (room.Phantom) {
        if (!room.Listed) {
          a.appendChild(icoUnlistedPhantom.cloneNode());
        } else {
          a.appendChild(icoPhantom.cloneNode());
        }
      }

      a.append(roomName, roomMemCount);
      li.appendChild(a);

      allNodes.push(li);
    });

    if (allNodes.length) container.append(...allNodes);
  };

  const renderRooms = (payload) => {
    roomCounts.innerText = `${payload.live_rooms || 0}/${
      payload.total_rooms || 0
    }`;

    currentRelease.innerText =
      payload.current_release || currentRelease.innerText;

    diffFillRoomList(payload?.rooms, publicRooms, existingPublicRooms);
    diffFillRoomList(
      payload?.unlisted_rooms,
      unlistedRooms,
      existingUnlistedRooms
    );
  };

  fetcher(renderRooms);
  loadAllInprogress = false;
};

const newRoomForm = () => {
  const newRoomPhantomBadge = document.querySelector("#new-room form .phantom");
  const defaultTitle = newRoomPhantomBadge.getAttribute("title");
  const chkBoxPhantom = document.getElementById("phantom");
  const chkBoxUnlisted = document.getElementById("unlisted");

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
};

const visitroomForm = () => {
  const input = document.getElementById("join-room-url");
  if (!input) return;

  document.getElementById("visit-room-form").onsubmit = (e) => {
    e.preventDefault();
    const roomId = input.value.trim();
    if (!roomId) return;
    if (roomId.startsWith("https://") || roomId.startsWith("http://")) {
      window.location.href = roomId;
    } else {
      window.location.href = `/rooms/${roomId}`;
      return;
    }
  };
};

let pageShowtimer = undefined;
const home = () => {
  newRoomForm();
  visitroomForm();
  const notifications = notifier();

  document.querySelectorAll(".pwa").forEach((el) => {
    const msg = `<a href="https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Guides/Installing" target="_blank">Check how to install PWA</a>`;
    el.addEventListener("click", () => {
      notifications.notify(msg, 3000);
    });
  });

  window.addEventListener("pageshow", (event) => {
    if (!event.persisted) return;
    loadAll();
  });
};

home();
