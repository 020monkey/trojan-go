FROM golang:latest AS builder

WORKDIR /
RUN git clone --depth=1 https://github.com/p4gefau1t/trojan-go.git
WORKDIR /trojan-go
RUN go build -tags "server auth_config" -ldflags "-s -w" -o /trojan

FROM debian:buster-slim
WORKDIR /root/
COPY --from=builder /trojan .
CMD ["./trojan"]
