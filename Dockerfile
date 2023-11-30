FROM golang:alpine AS builder
ENV CGO_ENABLED 0
ENV GOPROXY https://goproxy.cn,direct
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

RUN apk update --no-cache && apk add --no-cache tzdata

WORKDIR /build
COPY msg msg
ADD main.go .
ADD go.mod .
ADD go.sum .
RUN go mod tidy && go build -ldflags="-s -w" -o /app/kp main.go

FROM scratch

WORKDIR /app
COPY --from=builder /app/kp /app/kp

CMD ["/app/kp"]