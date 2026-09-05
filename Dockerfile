#第一段 工地--只负责编译
FROM golang:1.24 AS build
RUN go env -w GO111MODULE=on  \
    GOPROXY=https://goproxy.cn,direct
WORKDIR /build
#依赖先COPY + 下载， 命中缓存
COPY go.mod go.sum ./
RUN go mod download
#再拷全部源码，编译
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o  edu.mall.backend main.go
#第二段： 空房间 -- 只放产物
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" >/etc/timezone
WORKDIR /app
COPY --from=build /build/edu.mall.backend /app/edu.mall.backend
EXPOSE 8089
ENTRYPOINT ["/app/edu.mall.backend"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
CMD wget -q -O /dev/null http://127.0.0.1:8089/ping || exit 1