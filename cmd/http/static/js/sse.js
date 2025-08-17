const sse = (url, config = {}) => {
  const {
    onMessage,
    onError,
    initialBackoff = 100, // milliseconds
    maxBackoff = 15 * 1000, // 15 seconds
    backoffStep = 100, // milliseconds
  } = config;

  let backoff = initialBackoff,
    sseRetryTimeout = null;

  const start = () => {
    const source = new EventSource(url);
    const configState = { initialBackoff, maxBackoff, backoffStep, backoff };

    source.onopen = () => {
      clearTimeout(sseRetryTimeout);
      // reset backoff to initial, so further failures will again start with initial backoff
      // instead of previous duration
      backoff = initialBackoff;
      configState.backoff = backoff;
    };

    source.onmessage = (event) => {
      onMessage && onMessage(event, configState);
    };

    source.onerror = (err, ...all) => {
      source.close();
      if (!backoffStep) {
        onError && onError(err, { configState, all });
        return;
      }

      clearTimeout(sseRetryTimeout);
      // reattempt connecting with *linear* backoff
      sseRetryTimeout = self.setTimeout(() => {
        start(url, onMessage);
        if (backoff < maxBackoff) {
          backoff += backoffStep;
          if (backoff > maxBackoff) {
            backoff = maxBackoff;
          }
        }
      }, backoff);
      onError && onError(err, { configState, all });
    };
  };
  return start;
};

onmessage = (e) => {
  sse(e?.data?.url, {
    onMessage: (event) => {
      postMessage(event?.data);
    },
    onError: (err, attrs) => {
      postMessage({
        error: "SSE worker failed",
        ...attrs,
      });
    },
  })();
};
