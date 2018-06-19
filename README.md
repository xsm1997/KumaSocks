# KumaSocks
A light-weighted transparent proxy to socks5 converter written in Golang, inspired by [Transocks](https://github.com/cybozu-go/transocks).
## Usage

You should create a file for the config, using the TOML format.
```
listen-addr = "0.0.0.0:1234"
proxy-addr = "socks5://192.168.1.210:1080"
```
The default conf file is located at /etc/kumasocks.toml. You can customize it using the -c parameter.

Then you can use iptables redirect to route traffics to KumaSocks, and it will directly route them to the socks5 server. Be aware, there is no encryption. Use at your own risk!

## Advanced Usage
When you want to run KumaSocks on some embedded devices, such as routers, you can reduce the size of executables by using following build command.
```
go build -ldflags "-s -w"
```
When you want to make KumaSocks more smaller, you can consider to use upx. Here is the command.
```
upx --best KumaSocks
```
