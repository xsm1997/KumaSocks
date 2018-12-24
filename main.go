// KumaSocks project main.go
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/net/proxy"
)

var (
	configPath  string
	conf        Config
	proxyDialer proxy.Dialer
)

type Config struct {
	ListenAddr string `toml:"listen-addr"`
	ProxyAddr  string `toml:"proxy-addr"`
}

func readConf(path string) string {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
		os.Exit(0)
		return ""
	}

	return string(buf)
}

func handleConnection(conn *net.TCPConn) {
	defer conn.Close()

	dstAddr, err := GetOriginalDST(conn)
	if err != nil {
		log.Fatal(err)
		return
	}

	//log.Println("Incoming connection to " + dstAddr.String())

	proxy, err := proxyDialer.Dial("tcp", dstAddr.String())
	if err != nil {
		log.Println(err)
		return
	}
	defer proxy.Close()

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		//io.Copy(proxy, conn)
		buf := make([]byte, 1500)
		for {
			nr, er := conn.Read(buf)
			if nr > 0 {
				_, ew := proxy.Write(buf[:nr])
				if ew != nil {
					break
				}
			}
			if er != nil {
				break
			}
		}
		conn.Close()
		proxy.Close()
		wg.Done()
	}()

	go func() {
		//io.Copy(conn, proxy)
		buf := make([]byte, 1500)
		for {
			nr, er := proxy.Read(buf)
			if nr > 0 {
				_, ew := conn.Write(buf[:nr])
				if ew != nil {
					break
				}
			}
			if er != nil {
				break
			}
		}
		conn.Close()
		proxy.Close()
		wg.Done()
	}()

	wg.Wait()
}

func main() {
	flag.StringVar(&configPath, "c", "/etc/kumasocks.toml", "Config file path")
	flag.Parse()

	configStr := readConf(configPath)
	if _, err := toml.Decode(configStr, &conf); err != nil {
		log.Fatal(err)
		return
	}

	listener, err := net.Listen("tcp", conf.ListenAddr)
	if err != nil {
		log.Fatal(err)
		return
	}

	defer listener.Close()

	proxyURL, err := url.Parse(conf.ProxyAddr)
	if err != nil {
		log.Fatal(err)
		return
	}

	dialer := &net.Dialer{
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}

	proxyDialer, err = proxy.FromURL(proxyURL, dialer)
	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Println("Starting Kumasocks...")

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				break
			}
			go handleConnection(conn.(*net.TCPConn))
		}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-c

	fmt.Println("Exiting Kumasocks...")

	listener.Close()
}
