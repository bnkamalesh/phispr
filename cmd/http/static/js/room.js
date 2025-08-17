const messagesHandler = (roomID, authorID) => {
  const localstoreKey = "stored_messages";
  // TODO: implement local storage later
  const store = JSON.parse(localStorage.getItem(localstoreKey)) || {};
  const messagesList = document.getElementById("messages-list");
  const messageContainer = document.getElementById("messages");

  messageContainer.scrollTop = messageContainer.scrollHeight;
  return {
    All: function (roomID) {
      return store[roomID] || [];
    },
    // Load is used for bulk load messages to a room storage
    Load: function (roomID, messages) {
      store[roomID] = messages;
    },
    Push: function (roomID, message, callback) {
      this.RenderSingleMessage(message);
      callback?.();
    },
    RenderSingleMessage: function (message) {
      if (!message) return;

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
      author.innerText = message?.author + ", ";
      if (message?.at) {
        at.dataset.datetime = message.at;
        const timestamp = new Date(message.at);
        at.innerText = timestamp.toLocaleString();
      }

      content.innerText = message?.content;
      msgLi.appendChild(author);
      msgLi.appendChild(at);
      msgLi.appendChild(content);
      messagesList.appendChild(msgLi);

      /*
      The minus 64 is a buffer zone to identify if the scroll top is close to 
      the max possible, so that it auto scrolls when there are new messages.
      */
      const maxPossibleScrollTop =
        messageContainer.scrollHeight - messageContainer.offsetHeight - 64;

      if (messageContainer.scrollTop >= maxPossibleScrollTop) {
        messageContainer.scrollTo({
          top: messageContainer.scrollHeight,
          behavior: "smooth",
        });
      }
    },
    RenderMessageTimestamps: function () {
      messageContainer.querySelectorAll("li.msg .datetime").forEach((el) => {
        const timestamp = new Date(el.dataset.datetime);
        el.innerText = timestamp.toLocaleString();
      });
    },
    Clear: function (roomID) {
      messagesList.querySelectorAll("li.msg").forEach((li) => li.remove());
      messagesList.querySelector(".init").setAttribute("style", "");
    },
  };
};

const memberHandler = (roomID) => {
  const cookieParts = document?.cookie.split("; ");
  let cookieValue = "";
  if (cookieParts.length > 1) {
    cookieValue = cookieParts
      .find((row) => row.startsWith(`${btoa(roomID + "_js")}=`))
      ?.split("=")[1];
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

  return {
    member: parsed,
  };
};

const SSE = async (roomID, onMessage) => {
  const statusContent = {
    inactive: {
      text: [
        `(╯°□°）╯︵ ┻━┻`,
        `(╯ರ ~ ರ）╯︵ ┻━┻`,
        `┻━┻ ︵ ¯\(ツ)/¯ ︵ ┻━┻`,
        `┻━┻︵ \(°□°)/ ︵ ┻━┻`,
      ],
      title: "disconnected (try sending a message or refreshing to reconnect)",
    },
    active: {
      text: [`(✿◠‿◠)`, `(◕‿◕)`, `(ﾉ◕ヮ◕)ﾉ*:･ﾟ✧`, `(ノ^_^)ノ`],
      title: "connected",
    },
  };
  const sseStatus = document.getElementById("sse-status");
  const setStatus = (status) => {
    const currentStatus = sseStatus.className;
    if (currentStatus === status) {
      return;
    }

    sseStatus.className = status;
    sseStatus.innerText =
      statusContent[status].text[
        Math.floor(Math.random() * statusContent[status].text.length)
      ];
    sseStatus.setAttribute("title", statusContent[status].title);
  };

  setStatus("active");
  const config = {
    url: `/rooms/${roomID}/messages`,
    onMessage: (data) => {
      setStatus("active");
      try {
        const parsed = JSON.parse(data);
        onMessage?.(parsed);
      } catch (e) {}
    },
    onError: (err) => {
      console.log(err);
      setStatus("inactive");
    },
    initialBackoff: 100,
    backoffStep: 1000,
  };

  const sseworker = new Worker("/static/js/sse.js");

  sseworker.onerror = (e) => {
    sseworker.terminate();
    sseStatus.className = "inactive";
  };

  sseworker.onmessage = (e, ...attrs) => {
    if (e?.data?.error) {
      config.onError("SSE failed", e?.data);
    } else {
      config.onMessage(e?.data);
    }
  };

  sseworker.postMessage({
    url: config.url,
    initialBackoff: config.initialBackoff,
    backoffStep: config.backoffStep,
  });
};

const room = async () => {
  const roomID = window.location.pathname.split("/").pop();
  const { member } = memberHandler(roomID);

  const authorID = member?.User?.Name || "anonymous";
  const messages = messagesHandler(roomID, authorID);

  const messageTextarea = document.getElementById("message");
  const clearMessages = document.getElementById("clear-messages");
  const msgForm = document.getElementById("message-form");
  msgForm.setAttribute("action", "/rooms/" + roomID + "/messages");

  clearMessages.onclick = () => {
    messages.Clear(roomID);
  };

  messages.RenderMessageTimestamps();

  msgForm.onsubmit = () => {
    prepAndSendMessage();
    return false;
  };

  msgForm.addEventListener("keypress", (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
      prepAndSendMessage();
    }
  });

  prepAndSendMessage = () => {
    const message = messageTextarea.value.trim();
    if (!message) return;
    // clear container only if sendmessage was successful
    sendMessage(message, (response) => {
      if (response) {
        alert(response);
        return;
      }

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
    });
  };

  sendMessage = async (message, callback) => {
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
          if (response.ok) {
            return;
          }
          response.text().then((obj) => {
            callback?.(obj);
          });
        });
    } catch (e) {
      // console.error(e);
      console.log(JSON.stringify(e));
    }

    callback?.();
  };

  SSE(roomID, (data) => {
    messages.Push(roomID, {
      content: data.Content,
      at: data.ServerReceivedAt,
      author: data.Author.Name,
    });
  });
};
room();
