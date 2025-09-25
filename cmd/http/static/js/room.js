import { notifier } from "./common.js";

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

const messageRenderer = (roomID, authorID) => {
  const messageContainer = document.getElementById("messages");
  const messagesList = document.getElementById("messages-list");

  const nodeMsgLi = document.createElement("li");
  nodeMsgLi.classList.add("msg");

  const nodeAuthorSpan = document.createElement("span");
  nodeAuthorSpan.classList.add("author");

  const nodeDatetimeSpan = document.createElement("span");
  nodeDatetimeSpan.classList.add("datetime");

  const nodeContentPre = document.createElement("pre");
  nodeContentPre.classList.add("content");

  messageContainer.scrollTop = messageContainer.scrollHeight;

  const maxNewLinesAllowed = 5;
  // 10 here is an arbitrary number, trying to identify if the new lines
  // are *justified* to be there. This is not a foolproof way, but should
  // work in most cases.
  // e.g. if a message has 50 new lines, but also has 500 characters, then
  // it is justified to have that many new lines.
  const charsPerLinebreak = 10;

  function replaceNewlines(str, replacement = " ") {
    // find number of new lines
    const newLineCount = (str.match(/\r?\n/g) || []).length;
    if (newLineCount <= maxNewLinesAllowed) return str;
    if (str.length / newLineCount > charsPerLinebreak) return str;

    let seen = 0;
    // Match LF or CRLF; preserve the first 5 matches, replace the rest
    return str.replace(/\r?\n/g, (match) => {
      seen += 1;
      return seen <= maxNewLinesAllowed ? match : replacement;
    });
  }

  return {
    renderSingleMsg: function (message) {
      if (!message || !message.content) return;

      const msgLi = nodeMsgLi.cloneNode();
      const author = nodeAuthorSpan.cloneNode();
      author.classList.add(authorID === message?.author ? "you" : "other");
      author.innerText = message?.author;

      const at = nodeDatetimeSpan.cloneNode();

      if (message?.at) {
        const timestamp = new Date(message.at);
        at.dataset.datetime = timestamp.getTime();
        at.innerText = " " + timestamp.toLocaleString();
      }

      const content = nodeContentPre.cloneNode();
      content.innerText = replaceNewlines(message?.content, " \\n ");

      msgLi.appendChild(author);
      msgLi.appendChild(at);
      msgLi.appendChild(content);
      messagesList.appendChild(msgLi);

      /*
      The minus 512 is a buffer zone to identify if the scroll top is close to 
      the max possible, so that it auto scrolls when there are new messages and
      is already close to the bottom.
      */
      const maxPossibleScrollTop =
        messageContainer.scrollHeight - messageContainer.offsetHeight - 512;

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
    formatRenderedMsgs: function () {
      messageContainer.querySelectorAll("li.msg").forEach((el) => {
        const datecontainer = el.querySelector(".datetime");
        const timestamp = new Date(parseInt(datecontainer.dataset.datetime));
        datecontainer.innerText = timestamp.toLocaleString();

        const contentContainer = el.querySelector(".content");
        contentContainer.innerText = replaceNewlines(
          contentContainer.innerText,
          " \\n "
        );
      });
    },
  };
};

const messagesHandler = (roomID, authorID) => {
  const localstoreKey = "stored_messages";
  // TODO: implement local storage later
  const store = JSON.parse(localStorage.getItem(localstoreKey)) || {};
  const messagesList = document.getElementById("messages-list");
  const loading = document.getElementById("loading");
  const messageTextarea = document.getElementById("message");

  const msgRenderer = messageRenderer(roomID, authorID);
  msgRenderer.formatRenderedMsgs();

  const msgForm = document.getElementById("message-form");
  const sendMsgButton = msgForm?.querySelector("button[type='submit']");

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

  const clearTextArea = () => {
    // clear container only if sendmessage was successful
    // since the submission can be triggered by shift+enter, there's a new line
    // entered after prepAndSendMessage is executed. Setting a timeout helps
    // clear the text area properly. This can otherwise be handled using promises
    // and such to async clear this whole thing.
    window.setTimeout(() => {
      messageTextarea.value = "";
      messageTextarea.textContent = "";
      messageTextarea.focus();
    }, 5);
  };
  const send = (successCallback) => {
    const message = messageTextarea.value.trim();
    if (!message.length) return;

    loading.classList.add("active");
    sendMessage(message, (response) => {
      loading.classList.remove("active");
      if (response) {
        alert(response);
        return;
      }
      successCallback?.();
    });
  };

  if (msgForm) {
    const onMsgSubmit = () => {
      if (sendMsgButton.classList.contains("inactive")) return;

      send(clearTextArea);
    };

    msgForm.onsubmit = () => {
      onMsgSubmit();
      return false;
    };
    msgForm.addEventListener("keypress", (e) => {
      // if shift+enter is pressed, submit
      if (e.key === "Enter" && e.shiftKey) {
        if (document.getElementById("message").value.trim() === "/clear") {
          messages.clear(roomID);
        } else if (!sendMsgButton.classList.contains("inactive")) {
          send(clearTextArea);
        }
      }
    });
  }

  return {
    // push a message to the messages and render
    push: function (message, callback) {
      msgRenderer.renderSingleMsg(message);
      callback?.();
    },
    clear: function (roomID) {
      messagesList.querySelectorAll("li.msg").forEach((li) => li.remove());
      messagesList.querySelector(".init").setAttribute("style", "");
      clearTextArea();
    },
    // TODO: loadall should just render given list of messages, after checking for
    // duplicates.
    loadAll: function (messages) {
      if (!messages || !messages.length) return;

      const lastMsg = messagesList.querySelector("li.msg:last-child");
      let lastUnixMilli = 0;
      const dt =
        lastMsg?.querySelector(".datetime")?.dataset.datetime || undefined;
      if (dt && !isNaN(parseInt(dt))) {
        lastUnixMilli = parseInt(dt);
      } else if (lastMsg && !dt) {
        console.log("invalid last datetime found for the last message", dt);
        return;
      }

      const lastTimestamp = lastUnixMilli || 0;
      messages.forEach((message) => {
        // timestamp based check can be buggy based on concurrent messages delivered at same time
        const msgAt = new Date(message.ServerReceivedAt)?.getTime();
        if (!msgAt || isNaN(msgAt) || lastTimestamp >= msgAt) return;

        this.push({
          content: message.Content,
          at: message.ServerReceivedAt,
          author: message.Author.Name,
        });
      });
    },
  };
};

const memberHandler = (roomID) => {
  const membersList = document.getElementById("members-list");
  const memberCount = document.getElementById("members-count");
  const nodeLi = document.createElement("li");
  nodeLi.classList.add("member");
  const nodeAuthorSpan = document.createElement("span");
  nodeAuthorSpan.classList.add("author");

  const nodeBootButton = document.createElement("button");
  nodeBootButton.classList.add("ico", "boot");

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

  const addBootButton = (li, member) => {
    const bootButton = nodeBootButton.cloneNode();
    bootButton.title = `Boot '${member?.User?.Name}'`;
    bootButton.addEventListener("click", (e) => {
      e.stopPropagation();
      e.preventDefault();
      bootMember(member?.User?.ID);
    });
    li.appendChild(bootButton);
  };

  return {
    member: parsed,
    // AddMember is used to add a new member to the room
    AddMember: function (member) {
      if (!membersList || !member) return;
      const authorName = member?.User?.Name;
      const li = nodeLi.cloneNode();
      li.dataset.author = authorName;
      li.dataset.authorid = member?.User?.ID;

      const span = nodeAuthorSpan.cloneNode();
      if (member?.User?.ID === parsed?.User?.ID) {
        span.classList.add("you");
      }

      span.innerText = authorName;
      span.setAttribute("title", authorName);
      li.appendChild(span);

      // isOwner is declared outside of this file, directly in room.html
      if (isOwner) {
        addBootButton(li, member);
      }
      membersList.appendChild(li);
      memberCount.innerText = member.TotalMembers;
    },
    RemoveMember: function (member, currentUserID) {
      if (!membersList) return;
      const authorID = member?.User?.ID;
      membersList.querySelector(`[data-authorid="${authorID}"]`).remove();
      memberCount.innerText = member.TotalMembers;
      if (currentUserID == member?.User?.ID) {
        alert(`You were booted from this room`);
        window.location.href = window.location.href;
      }
    },
    loadAll: function (members) {
      if (!members || !members.length || !membersList) return;

      const existingAuthIDs = {};
      const allMemberIDs = {};
      members.forEach((member) => {
        allMemberIDs[member?.User?.ID] = true;
      });

      membersList.querySelectorAll(".member").forEach((memLi) => {
        const authorID = memLi.dataset.authorid || null;
        existingAuthIDs[authorID] = true;
        if (!allMemberIDs[authorID]) {
          if (authorID === parsed?.User?.ID) {
            this.RemoveMember(parsed, parsed?.User?.ID);
            return;
          }
          memLi.remove();
        }
      });

      members.forEach((member) => {
        if (existingAuthIDs[member?.User?.ID]) return;
        this.AddMember(member);
      });

      memberCount.innerText = members.length;
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
  const msgMemContainer = document.getElementById("msgs-members");
  if (!msgMemContainer) return null;

  const allElements = document.querySelectorAll("main > *");
  const findHeight = () => {
    let availableHeight = window.innerHeight;
    allElements.forEach((el) => {
      if (el === msgMemContainer) return;

      availableHeight -= el.getBoundingClientRect().height || 0;
      const style = getComputedStyle(el);
      availableHeight -= parseFloat(style.marginTop) || 0;
      availableHeight -= parseFloat(style.marginBottom) || 0;
    });
    return availableHeight;
  };

  const resize = () => {
    const vh = Math.max(
      document.documentElement.clientHeight || 0,
      window.innerHeight || 0
    );
    if (vh > 1020) {
      if (msgMemContainer.style.height) {
        msgMemContainer.style.height = null;
      }
      return;
    }

    msgMemContainer.style.height = `${findHeight() - 2}px`;
  };
  window.addEventListener("resize", resize);
  resize();
};

const room = async () => {
  const notifications = notifier();
  const roomID = window.location.pathname.split("/").pop();
  const memHandler = memberHandler(roomID);
  const authorID = memHandler.member?.User?.Name || "anonymous";
  const msgHandler = messagesHandler(roomID, authorID);
  const sendMsgButton = document.querySelector(
    "#message-form button[type='submit']"
  );
  const onlineViewers = document.getElementById("online-viewers");
  const onlineCountText = onlineViewers?.querySelector("span");

  const membersContainer = document.getElementById("members");
  if (membersContainer) {
    membersContainer.addEventListener("click", () => {
      membersContainer.classList.toggle("active");
    });
  }

  document.querySelectorAll(".pwa").forEach((el) => {
    const msg = `<a href="https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Guides/Installing" target="_blank">Check how to install PWA</a>`;
    el.addEventListener("click", () => {
      notifications.notify(msg, 3000);
    });
  });

  let loadAllInprogress = false;
  const loadAll = () => {
    if (loadAllInprogress) return;
    loadAllInprogress = true;

    fetch(`/rooms/${roomID}`, {
      headers: {
        "Content-Type": "application/json",
      },
      method: "GET",
    })
      .then((response) => response.json())
      .then((payload) => {
        if (!payload || !payload?.data) return;

        msgHandler.loadAll(payload.data.messages || []);
        memHandler.loadAll(payload.data.members || []);
      })
      .finally(() => {
        loadAllInprogress = false;
      });
  };

  SSE(
    roomID,
    (type, data) => {
      switch (type) {
        case "room_join":
          memHandler.AddMember(data);
          break;
        case "room_leave":
          memHandler.RemoveMember(data, authorID);
          break;

        case "room_message":
          msgHandler.push({
            content: data.Content,
            at: data.ServerReceivedAt,
            author: data.Author.Name,
          });
          break;

        case "room_viewers":
          onlineCountText.innerText = data;

          if (data > 1) {
            onlineViewers?.style?.setProperty("opacity", "1");
          } else {
            onlineViewers?.style?.setProperty("opacity", null);
          }
          break;
      }
    },
    (status) => {
      if (status == "inactive") {
        onlineViewers?.style?.setProperty("opacity", null);
      }
      loadAll();
    }
  );

  let timer = undefined;
  window.addEventListener("pageshow", (event) => {
    if (!event.persisted) return;
    // loadAll is executed only after a delay, because looks like SSE itself works in the
    // background for a while. This causes double rendering of same messages, members etc.
    if (timer) clearTimeout(timer);
    timer = window.setTimeout(() => {
      loadAll();
    }, 2000);
  });

  roomSizer();
  qrHandler(notifications);
};

room();
