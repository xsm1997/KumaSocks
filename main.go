// KumaSocks project main.go
package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/net/proxy"
)

var (
	configPath  string
	conf        Config
	proxyDialer proxy.Dialer
	running     bool
)

type Config struct {
	ListenAddr string `toml:"listen-addr"`
	ProxyAddr  string `toml:"proxy-addr"`
	IOCopyHack bool   `toml:"io-copy-hack"`
}

func readConf(path string) string {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("[ERROR] Read config error: %s.\n", err.Error())
		return ""
	}

	return string(buf)
}

func customCopy(dst io.Writer, src io.Reader, buf []byte) (err error) {
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			_, ew := dst.Write(buf[0:nr])
			if ew != nil {
				err = ew
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}

	return err
}

func handleConnection(conn *net.TCPConn) {
	conn.SetKeepAlive(true)
	conn.SetKeepAlivePeriod(20 * time.Second)
	conn.SetNoDelay(true)

	dstAddr, err := GetOriginalDST(conn)
	if err != nil {
		log.Printf("[ERROR] GetOriginalDST error: %s\n", err.Error())
		return
	}

	log.Printf("[INFO] Connect to %s.\n", dstAddr.String())

	proxy, err := proxyDialer.Dial("tcp", dstAddr.String())
	if err != nil {
		log.Printf("[ERROR] Dial proxy error: %s.\n", err.Error())
		return
	}

	proxy.(*net.TCPConn).SetKeepAlive(true)
	proxy.(*net.TCPConn).SetKeepAlivePeriod(20 * time.Second)
	proxy.(*net.TCPConn).SetNoDelay(true)

	copyEnd := false

	go func() {
		buf := make([]byte, 32*1024)

		var err error
		if conf.IOCopyHack {
			err = customCopy(proxy, conn, buf)
		} else {
			_, err = io.CopyBuffer(proxy, conn, buf)
		}

		if err != nil && !copyEnd {
			log.Printf("[ERROR] Copy error: %s.\n", err.Error())
		}
		copyEnd = true
		conn.Close()
		proxy.Close()
	}()

	buf := make([]byte, 32*1024)
	if conf.IOCopyHack {
		err = customCopy(conn, proxy, buf)
	} else {
		_, err = io.CopyBuffer(conn, proxy, buf)
	}

	if err != nil && !copyEnd {
		log.Printf("[ERROR] Copy error: %s.\n", err.Error())
	}
	copyEnd = true
	conn.Close()
	proxy.Close()
}

func main() {
	flag.StringVar(&configPath, "c", "/etc/kumasocks.toml", "Config file path")
	flag.Parse()

	configStr := readConf(configPath)
	if _, err := toml.Decode(configStr, &conf); err != nil {
		log.Fatalf("[ERROR] Decode config error: %s.\n", err.Error())
	}

	listener, err := net.Listen("tcp", conf.ListenAddr)
	if err != nil {
		log.Fatalf("[ERROR] Listen tcp error: %s.\n", err.Error())
	}

	defer listener.Close()

	proxyURL, err := url.Parse(conf.ProxyAddr)
	if err != nil {
		log.Fatalf("[ERROR] Parse proxy address error: %s.\n", err.Error())
	}

	proxyDialer, err = proxy.FromURL(proxyURL, &net.Dialer{})
	if err != nil {
		log.Fatalf("[ERROR] Create proxy error: %s.\n", err.Error())
	}

	log.Println("[INFO] Starting Kumasocks...")

	if conf.IOCopyHack {
		log.Println("[INFO] Using io.Copy hack.")
	}

	running = true

	go func() {
		for running {
			conn, err := listener.Accept()
			if err != nil {
				if running {
					log.Printf("[ERROR] TCP accept error: %s.\n", err.Error())
				}
				continue
			}
			go handleConnection(conn.(*net.TCPConn))
		}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-c

	log.Println("[INFO] Exiting Kumasocks...")

	running = false
	listener.Close()
}
