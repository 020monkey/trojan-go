# Trojan-Go

Trojan proxy written in golang. It is compatiable with the original Trojan protocol and config file. 

It's still currently in heavy development.

[README_zh_cn(中文)](README_zh_cn.md)

## Features

### Compatible

It's fully compatible with the Trojan protocol and configuration file, so that you can safely replace your client and server program with trojan-go, or even just replace one of them, without additional configuration.

### Easy to use

Trojan-go's configuration file format is compatible with Trojan's, while it's being simplyfied. You can launch your server and client much more easily. Here's an example:

server.json
```
{
	"run_type": "server",
	"local_addr": "0.0.0.0",
	"local_port": 4445,
	"remote_addr": "127.0.0.1",
	"remote_port": 80,
	"password": [
		"your_awesome_password"
	],
	"ssl": {
		"cert": "your_cert.crt",
		"key": "your_key.crt",
	}
}

```

client.json
```
{
    "run_type": "client",
    "local_addr": "127.0.0.1",
    "local_port": 1080,
    "remote_addr": "your_awesome_server",
    "remote_port": 443,
    "password": [
	    "your_awesome_password"
    ],
    "ssl": {
        "cert": "your_cert.crt",
        "sni": "your_awesome_domain_name",
    }
}
```

run_type supported by Trojan-Go (the same as Trojan):

- Client

- Server

- NAT (transparent proxy, see [here](https://github.com/shadowsocks/shadowsocks-libev/tree/v3.3.1#transparent-proxy))

- Forward

For more infomation, see Trojan's [docs](https://trojan-gfw.github.io/trojan/config) about the configuration file.

### Multiplexing

TLS handshaking may takes much time in a poor network condition.
Trojan-go supports multiplexing([smux](https://github.com/xtaci/smux)), which imporves the performance in the high-concurrency scenario by forcing one single TLS tunnel connection carries mutiple TCP connections.

Enabling multiplexing does not increase the bandwidth you get from a speed test, but it will speed up the network experience when you have a large number of concurrent requests, such as browsing web pages containing a large number of images, etc.

Note that this feature is not compatible with the original Trojan , so for compatibility reasons, this feature is turned off by default. But you can enable it by setting the "mux" field in the tcp options. as follows

```
"tcp": {
    "mux": true
}
```
for example

client.json
```
{
    "run_type": "client",
    "local_addr": "127.0.0.1",
    "local_port": 1080,
    "remote_addr": "your_awesome_server",
    "remote_port": 443,
    "password": [
	    "your_awesome_password"
    ],
    "ssl": {
        "cert": "server.crt",
        "sni": "your_awesome_domain_name",
    },
    "tcp": {
        "mux": true
    }
}
```

You only need to set the client's configuration file, and the server will automatically detect whether to enable multiplexing.

### Portable

It's written in Golang, and Golang compiles statically by default, which means that you can execute the compiled single executable directly on the target machine without having to consider dependencies. You can easily compile (or cross compile) it and deploy it on your server, PC, Raspberry Pi, or even a router.

## Build

Just make sure your golang version >= 1.11


```
git clone https://github.com/p4gefau1t/trojan-go.git
cd trojan-go
go build
```

You can cross-compile it by setting up the environment vars, for example
```
GOOS=windows GOARCH=amd64 go build -o trojan-go.exe
```

or

```
GOOS=linux GOARCH=arm go build -o trojan-go
```

## Usage

```
./trojan-go -config your_awesome_config_file.json
```

The format of the configuration file is compatible, see [here](https://trojan-gfw.github.io/trojan/config).


## TODOs

- [x] Server
- [x] Forward
- [x] NAT
- [x] Client
- [x] UDP Tunneling
- [x] Transparent proxy
- [x] Log
- [x] Mux
- [ ] TLS Settings
- [x] TLS redirecting
- [ ] non-TLS redirecting
- [ ] Cert utils
- [ ] Database support
- [x] Traffic stats
- [ ] TCP Settings
