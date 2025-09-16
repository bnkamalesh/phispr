import { notifier, serviceWorkerSetup } from "./common.js";

const SSE = async (roomID, onMessage, setStatusCallback) => {
  // lastMsgReceived is the timestamp of when the last *successful* message was received
  let lastMsgReceived = null;
  const sseStatuscontainer = document.querySelector(
    "#message-form button[type='submit']"
  );
  const statusContent = {
    inactive: {
      title: "disconnected (try refreshing to reconnect)",
    },
    active: {
      title: sseStatuscontainer?.dataset.liveViewers
        ? `Send (live: ${sseStatuscontainer?.dataset.liveViewers})`
        : "Send",
    },
  };
  const setStatus = (status) => {
    if (!sseStatuscontainer || sseStatuscontainer.classList.contains(status)) {
      return;
    }

    sseStatuscontainer.classList.remove("inactive");
    sseStatuscontainer.classList.remove("active");

    sseStatuscontainer.classList.add(status);
    sseStatuscontainer.setAttribute("title", statusContent[status].title);
    sseStatuscontainer.disabled = status === "inactive";
    setStatusCallback?.(status);
  };

  setStatus("active");
  const config = {
    url: `/rooms/${roomID}/messages`,
    onMessage: (data) => {
      try {
        const parsed = JSON.parse(data);
        onMessage?.(parsed.Type, parsed.Data);
      } catch (e) {}
    },
    onError: (err) => {
      console.log(err);
    },
    initialBackoff: 100,
    backoffStep: 1000,
  };

  const sseworker = new Worker("/static/js/min/sse.js");

  sseworker.onerror = (e) => {
    sseworker.terminate();
    setStatus("inactive");
  };

  sseworker.onmessage = (e) => {
    if (e?.data?.error) {
      setStatus("inactive");
      config.onError("SSE failed", e?.data);
    } else {
      lastMsgReceived = new Date();
      setStatus("active");
      config.onMessage(e?.data);
    }
  };

  sseworker.postMessage({
    url: config.url,
    initialBackoff: config.initialBackoff,
    backoffStep: config.backoffStep,
  });

  // broadcastDelay is declared outside of this file, directly in room.html
  // Server broadcasts room live count every 'broadcastDelay' milliseconds.
  if (broadcastDelay) {
    window.setInterval(() => {
      if (!lastMsgReceived) return;
      const now = new Date();
      const diff = now - lastMsgReceived;
      if (diff < broadcastDelay) {
        setStatus("active");
        return;
      }
      setStatus("inactive");
    }, broadcastDelay);
  }
};

const messagesHandler = (roomID, authorID) => {
  const localstoreKey = "stored_messages";
  // TODO: implement local storage later
  const store = JSON.parse(localStorage.getItem(localstoreKey)) || {};
  const messagesList = document.getElementById("messages-list");
  const messageContainer = document.getElementById("messages");
  const loading = document.getElementById("loading");
  const messageTextarea = document.getElementById("message");

  messageContainer.scrollTop = messageContainer.scrollHeight;

  const sendMessage = async (message, callback) => {
    const formData = new FormData();
    formData.append("message", message);
    try {
      fetch("/rooms/" + roomID + "/messages", {
        method: "POST",
        // Set the FormData instance as the request body
        body: formData,
      })
        .catch((reason) => {
          callback?.(reason);
        })
        .then((response) => {
          if (response?.ok) {
            callback?.();
            return;
          }
          response?.text().then((obj) => {
            callback?.(obj);
          });
        });
    } catch (e) {
      callback?.(e);
      console.error(e);
    }
  };

  const maxNewLinesAllowed = 5;
  function replaceNewlines(str, replacement = " ") {
    let seen = 0;
    // Match LF or CRLF; preserve the first 5 matches, replace the rest
    return str.replace(/\r?\n/g, (match) => {
      seen += 1;
      return seen <= maxNewLinesAllowed ? match : replacement;
    });
  }

  return {
    // push a message to the messages and render
    push: function (message, callback) {
      this.renderSingleMessage(message);
      callback?.();
    },
    renderSingleMessage: function (message) {
      if (!message || !message.content) return;
      const msgLi = document.createElement("li");
      msgLi.className = "msg";
      const author = document.createElement("span");
      const at = document.createElement("span");
      const content = document.createElement("p");
      content.className = "content";
      author.classList.add(
        "author",
        authorID === message?.author ? "you" : "other"
      );
      at.className = "datetime";
      author.innerText = message?.author;

      if (message?.at) {
        const timestamp = new Date(message.at);
        at.dataset.datetime = timestamp.getTime();
        at.innerText = " " + timestamp.toLocaleString();
      }

      content.innerText = replaceNewlines(message?.content, " \\n ");
      msgLi.appendChild(author);
      msgLi.appendChild(at);
      msgLi.appendChild(content);
      messagesList.appendChild(msgLi);

      /*
      The minus 320 is a buffer zone to identify if the scroll top is close to 
      the max possible, so that it auto scrolls when there are new messages and
      is already close to the bottom.
      */
      const maxPossibleScrollTop =
        messageContainer.scrollHeight - messageContainer.offsetHeight - 320;

      if (messageContainer.scrollTop >= maxPossibleScrollTop) {
        messageContainer.scrollTo({
          top: messageContainer.scrollHeight,
          behavior: "smooth",
        });
      }

      // this feels silly, but I couldn't find an easier way to do it.
      messagesList
        .querySelector(".init")
        .setAttribute("style", "display: none");
    },
    renderMessageTimestamps: function () {
      messageContainer.querySelectorAll("li.msg .datetime").forEach((el) => {
        const timestamp = new Date(parseInt(el.dataset.datetime));
        el.innerText = timestamp.toLocaleString();
      });
    },
    clearTextArea: function () {
      // clear container only if sendmessage was successful
      // since the submission can be triggered by shift+enter, there's a new line
      // entered after prepAndSendMessage is executed. Setting a timeout helps
      // clear the text area properly. This can otherwise be handled using promises
      // and such to async clear this whole thing.
      window.setTimeout(() => {
        messageTextarea.value = "";
        messageTextarea.textContent = "";
        messageTextarea.focus();
        messageTextarea.click();
      }, 5);
    },
    clear: function (roomID) {
      messagesList.querySelectorAll("li.msg").forEach((li) => li.remove());
      messagesList.querySelector(".init").setAttribute("style", "");
      this.clearTextArea();
    },
    send: function () {
      const message = messageTextarea.value.trim();
      if (!message.length) return;

      loading.classList.add("active");
      sendMessage(message, (response) => {
        loading.classList.remove("active");
        if (response) {
          alert(response);
          return;
        }
        // clear container only if sendmessage was successful
        this.clearTextArea();
      });
    },
    loadAll: function () {
      const lastMsg = messagesList.querySelector("li.msg:last-child");
      let lastDatetime = undefined;
      const dt =
        lastMsg?.querySelector(".datetime")?.dataset.datetime || undefined;
      if (dt && !isNaN(parseInt(dt))) {
        lastDatetime = parseInt(dt);
      } else if (!dt) {
        console.log("invalid last datetime found for the last message", dt);
        return;
      }

      const lastTimestamp = lastDatetime || 0;
      // this is a silly way of ignoring duplicate messages and prone to bugs.
      // but is the simplest way of avoiding duplicates.

      fetch(`/rooms/${roomID}`, {
        headers: {
          "Content-Type": "application/json",
        },
        method: "GET",
      })
        .then((response) => response.json())
        .then((payload) => {
          if (!payload || !payload?.data) return;
          const messages = payload?.data?.messages || [];
          messages.forEach((message) => {
            // timestamp based check can be buggy based on concurrent messages delivered at same time
            const msgAt = new Date(message.ServerReceivedAt);
            if (lastTimestamp >= msgAt.getTime()) return;

            this.renderSingleMessage({
              content: message.Content,
              at: message.ServerReceivedAt,
              author: message.Author.Name,
            });
          });
        });
    },
  };
};

const memberHandler = (roomID) => {
  const membersList = document.getElementById("members-list");
  const memberCount = document.getElementById("members-count");
  // jsCookieName is declared outside of this file, directly in room.html
  const cookieParts = document?.cookie
    .split("; ")
    .find((row) => row.startsWith(jsCookieName))
    ?.split("=");
  let cookieValue = "";
  if (cookieParts?.length > 1) {
    cookieValue = cookieParts[1];
  }

  let parsed = {};
  if (!cookieValue) {
    return parsed;
  }

  try {
    parsed = JSON.parse(atob(cookieValue));
  } catch (e) {
    console.error("Failed to parse cookie:", e);
  }

  const bootMember = function (userID, callback) {
    const url = `/rooms/${roomID}/${userID}`;
    try {
      fetch(url, {
        headers: {
          "Content-Type": "application/json",
        },
        method: "DELETE",
      })
        .catch((reason) => {
          callback?.(reason);
        })
        .then((response) => {
          if (!response.ok) {
            return;
          }

          response.text().then((obj) => {
            callback?.(obj);
          });
        });
    } catch (e) {
      callback?.(e);
      console.error(e);
    }
  };

  if (membersList) {
    membersList.querySelectorAll(".member").forEach((el) => {
      const userID = el.dataset.authorid;
      const button = el.querySelector(".boot");
      if (!button) return;
      button.addEventListener("click", (e) => {
        e.stopPropagation();
        e.preventDefault();
        if (!userID) return;
        bootMember(userID);
      });
    });
  }

  return {
    member: parsed,
    // AddMember is used to add a new member to the room
    AddMember: function (member) {
      if (!membersList) return;
      const authorName = member?.User?.Name;

      const li = document.createElement("li");
      li.dataset.author = authorName;
      const span = document.createElement("span");
      span.innerText = authorName + " ";
      span.setAttribute("title", span.innerText);
      li.appendChild(span);
      // isOwner is declared outside of this file, directly in room.html
      if (isOwner) {
        const bootButton = document.createElement("button");
        bootButton.classList.add("ico", "boot");
        bootButton.title = `Boot ${authorName}`;
        bootButton.addEventListener("click", (e) => {
          e.stopPropagation();
          e.preventDefault();
          bootMember(member?.User?.ID);
        });
        li.appendChild(bootButton);
      }
      membersList.appendChild(li);
      memberCount.innerText = member.TotalMembers;
    },
    RemoveMember: function (member, currentUserID) {
      if (!membersList) return;
      const authorName = member?.User?.Name;
      membersList.querySelector(`[data-author="${authorName}"]`).remove();
      memberCount.innerText = member.TotalMembers;
      if (currentUserID == member?.User?.ID) {
        alert(`You were booted from this room`);
        window.location.href = window.location.href;
      }
    },
    BootMember: bootMember,
  };
};

const qrHandler = (notifications) => {
  const qrCodeContainer = document.getElementById("qr-code-container");
  if (!qrCodeContainer) return;

  const qrCodeContainerButton = qrCodeContainer.querySelector("button");
  const qrCode = qrCodeContainer.querySelector("#qr-code");
  const sharedSuccessfully = async () => {
    notifications?.notify("copied", 2000);
    navigator.clipboard.writeText(qrCodeContainerButton.innerText);
  };

  qrCodeContainerButton.innerText = window.location.href;

  new QRCode(qrCode, window.location.href);

  qrCodeContainer.childNodes.forEach((child) => {
    child.addEventListener("click", (e) => {
      e.stopPropagation();
      sharedSuccessfully();
    });
  });

  qrCodeContainer.addEventListener("click", () => {
    qrCodeContainer.classList.toggle("active");
  });

  document.onkeydown = (e) => {
    if (e.key === "Escape") {
      qrCodeContainer.classList.remove("active");
    }
  };

  document.getElementById("room-name").addEventListener("click", () => {
    qrCodeContainer.classList.toggle("active");
  });
};

const roomSizer = () => {
  const msgMemContainer = document.querySelector("section.room.flex-container");
  if (!msgMemContainer) return null;
  const allElements = document.querySelectorAll("main > *");
  const findHeight = () => {
    let availableHeight = window.innerHeight;
    allElements.forEach((sib) => {
      if (sib === msgMemContainer) return;
      availableHeight -= sib.getBoundingClientRect().height || 0;
    });
    return availableHeight;
  };

  const resize = () => {
    msgMemContainer.style.height = `${findHeight() - 2}px`;
  };
  resize();
  window.addEventListener("resize", resize);
};

const room = async () => {
  serviceWorkerSetup();

  const notifications = notifier();
  const roomID = window.location.pathname.split("/").pop();
  const { member, AddMember, RemoveMember } = memberHandler(roomID);
  const authorID = member?.User?.Name || "anonymous";
  const messages = messagesHandler(roomID, authorID);
  const msgForm = document.getElementById("message-form");
  const sendMsgButton = document.querySelector(
    "#message-form button[type='submit']"
  );
  const members = document.getElementById("members");

  roomSizer();
  qrHandler(notifications);

  if (members) {
    members.addEventListener("click", () => {
      members.classList.toggle("active");
    });
  }

  messages.renderMessageTimestamps();

  msgForm.onsubmit = () => {
    if (!sendMsgButton.classList.contains("inactive")) {
      messages.send();
    }
    return false;
  };

  document.querySelectorAll(".pwa").forEach((el) => {
    const msg = `<a href="https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Guides/Installing" target="_blank">Check how to install PWA</a>`;
    el.addEventListener("click", () => {
      notifications.notify(msg, 3000);
    });
  });

  msgForm.addEventListener("keypress", (e) => {
    // if shift+enter is pressed, submit
    if (e.key === "Enter" && e.shiftKey) {
      if (
        document.getElementById("message").value.trim().startsWith("/clear")
      ) {
        messages.clear(roomID);
      } else if (!sendMsgButton.classList.contains("inactive")) {
        messages.send();
      }
    }
  });

  SSE(
    roomID,
    (type, data) => {
      switch (type) {
        case "room_join":
          AddMember(data);
          break;
        case "room_leave":
          RemoveMember(data, authorID);
          break;

        case "room_message":
          messages.push({
            content: data.Content,
            at: data.ServerReceivedAt,
            author: data.Author.Name,
          });
          break;

        case "room_viewers":
          if (data > 1) {
            sendMsgButton?.setAttribute("title", `Send (live: ${data})`);
          } else {
            sendMsgButton?.setAttribute("title", `Send`);
          }
          break;
      }
    },
    () => {
      messages.loadAll();
    }
  );

  window.addEventListener("pageshow", (event) => {
    if (event.persisted) {
      messages.loadAll();
    }
  });
};

room();
