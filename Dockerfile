FROM node:20-alpine AS fe

WORKDIR /web
COPY webui/package.json webui/package-lock.json ./
RUN npm ci
COPY webui/ ./
RUN npm run build


FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY . .
COPY --from=fe /web/dist /app/webui/dist
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -extldflags=-static" -o main .


FROM alpine:3

LABEL org.opencontainers.image.title="漏哨 LouSentry"
LABEL org.opencontainers.image.version="v1.0.03"
LABEL org.opencontainers.image.description="采集高价值漏洞并推送到钉钉 / 飞书 / 企业微信"

RUN apk add --update tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    apk del tzdata && \
    rm -rf /var/cache/apk/* && \
    mkdir -p /app/data

WORKDIR /app
COPY --from=builder /app/main /app/main

ENV LISTEN=":8080" \
    DB_CONN="sqlite3://data/vuln_v3.sqlite3" \
    INTERVAL="30m" \
    NO_START_MESSAGE="true" \
    ADMIN_USER="admin"

VOLUME ["/app/data"]
EXPOSE 8080
ENTRYPOINT ["/app/main"]
