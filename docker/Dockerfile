FROM golang:latest AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates  \
    git \
    libssl-dev \
    netbase \
    && apt-get autoremove -y && apt-get autoclean -y

COPY . /app
WORKDIR /app

# Build the app
RUN GOOS=linux CGO_ENABLED=0 GOARCH=amd64 go build -a -v -ldflags '-s -w -extldflags "-static"' -o /app/bin/app ./

WORKDIR /app

# Start from busybox base image and copy the executable
FROM busybox:1.37
LABEL owner="Phispr"

COPY --from=builder /etc/ssl/certs /etc/ssl/certs

RUN export APP_BUILT_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ") && echo "APP_BUILT_AT=$APP_BUILT_AT" >> /etc/environment

RUN adduser --home "/home/appuser" --disabled-password appuser \
    --gecos "appuser,-,-,-"

USER appuser

COPY --from=builder /app/bin/app /home/appuser/app

WORKDIR /home/appuser

RUN printf "#!/bin/sh\nsource /etc/environment\n./app\n" > start.sh
RUN chmod +x start.sh

ENV ENVIRONMENT="live"

# Run the executable
CMD ["./start.sh"]