# Genesis

Every project has a beginning, but this one has a genesis I particularly enjoy. Over the years, as a senior software developer, I’ve conducted countless interviews & I tend to skip live coding rounds. Not out of laziness, but because I’ve never enjoyed doing them myself. Instead, I focus on system design, and I've been sticking to this scenario for years now.

> Design a public chatroom where anyone can join, no registration required. The system must ensure usernames are unique among online users, and messages are purely textual.

As candidates concoct their designs, I start introducing scaling challenges and more features, gradually testing whether the system can hold up under more ~ambitious~ presumptuous conditions. In 45 to 60 minutes, the interview concludes: either the candidate fumbles, or they produce a _theoretically_ robust design.

This project is my personal take on that same problem—at a very small scale. I’m particularly proud of building it without WebSockets, relying instead on [SSE](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events). SSE is used for subscribing to all live changes and [HTTP POST](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Methods/POST) for sending messages. Modern browsers are capable of _reusing_ HTTP connections to make multiple requests, thanks to [HTTP2](https://developer.mozilla.org/en-US/docs/Glossary/HTTP_2). It's an absolute pleasure when you can rely on modern browser features and Internet in general.
