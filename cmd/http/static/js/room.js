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
        onMessage?.(parsed.Type, parsed.Data);
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

const messagesHandler = (roomID, authorID) => {
  const localstoreKey = "stored_messages";
  // TODO: implement local storage later
  const store = JSON.parse(localStorage.getItem(localstoreKey)) || {};
  const messagesList = document.getElementById("messages-list");
  const messageContainer = document.getElementById("messages");
  const loading = document.getElementById("loading");
  const messageTextarea = document.getElementById("message");

  messageContainer.scrollTop = messageContainer.scrollHeight;

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
      console.error(JSON.stringify(e));
    }

    callback?.();
  };

  return {
    // push a message to the messages and render
    push: function (message, callback) {
      this.renderSingleMessage(message);
      callback?.();
    },
    renderSingleMessage: function (message) {
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
        messageContainer.scrollHeight - messageContainer.offsetHeight - 128;

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
        const timestamp = new Date(el.dataset.datetime);
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
      loading.className = "active";
      const message = messageTextarea.value.trim();
      // clear container only if sendmessage was successful
      sendMessage(message, (response) => {
        if (response) {
          alert(response);
          loading.className = "";
          return;
        }
        loading.className = "";
        this.clearTextArea();
      });
    },
  };
};

const memberHandler = (roomID) => {
  const membersList = document.getElementById("members-list");
  const cookieParts = document?.cookie
    .split("; ")
    .find((row) => row.startsWith(`${roomID + "_js"}=`))
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

  return {
    member: parsed,
    // AddMember is used to add a new member to the room
    AddMember: function (member) {
      if (!membersList) return;
      const li = document.createElement("li");
      const span = document.createElement("span");
      span.innerText = member?.User?.Name;
      span.setAttribute("title", span.innerText);
      li.appendChild(span);
      membersList.appendChild(li);
    },
  };
};

const room = async () => {
  const roomID = window.location.pathname.split("/").pop();
  const { member, AddMember } = memberHandler(roomID);
  const authorID = member?.User?.Name || "anonymous";
  const viewerCount = document.getElementById("live-viewers");
  const messages = messagesHandler(roomID, authorID);
  const msgForm = document.getElementById("message-form");
  msgForm.setAttribute("action", "/rooms/" + roomID + "/messages");

  messages.renderMessageTimestamps();

  msgForm.onsubmit = () => {
    messages.send();
    return false;
  };

  msgForm.addEventListener("keypress", (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
      // if shift+enter is pressed, just add a new line
      if (
        document.getElementById("message").value.trim().startsWith("/clear")
      ) {
        messages.clear(roomID);
      } else {
        messages.send();
      }
    }
  });

  SSE(roomID, (type, data) => {
    switch (type) {
      case "room_join":
        AddMember(data);
        break;
      case "room_message":
        messages.push({
          content: data.Content,
          at: data.ServerReceivedAt,
          author: data.Author.Name,
        });
        break;
      case "room_viewers":
        viewerCount.innerText = data.Viewers;
        viewerCount.setAttribute(
          "title",
          `${data.Viewers} user(s) are viewing this room`
        );
        break;
    }
  });
};
room();
