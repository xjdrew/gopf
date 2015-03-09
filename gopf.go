//
//   date  : 2014-05-23 17:35
//   author: xjdrew
//

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

/*
import _ "net/http/pprof"
*/

type Host struct {
	Addr   string
	Weight int
}

type Backend struct {
	TrafficUrl string
	Hosts      []Host
	weight     int
}

type Options struct {
	config  string
	backend Backend
}

type Daemon struct {
	name         string
	downloadChan chan uint64
	uploadChan   chan uint64
}

var options Options
var daemon Daemon
var mpool *MPool

func usage() {
	log.Printf("usage: %s config\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(1)
}

func reloadConfig() error {
	fp, err := os.Open(options.config)
	if err != nil {
		return err
	}
	defer fp.Close()

	var backend Backend
	dec := json.NewDecoder(fp)
	err = dec.Decode(&backend)
	if err != nil {
		return err
	}

	for i := range backend.Hosts {
		host := &backend.Hosts[i]
		backend.weight += host.Weight
	}

	log.Printf("config:%v", backend)
	options.backend = backend
	return nil
}

func chooseHost(weight int, hosts []Host) *Host {
	if weight <= 0 {
		return nil
	}

	v := rand.Intn(weight)
	for _, host := range hosts {
		if host.Weight >= v {
			return &host
		}
		v -= host.Weight
	}
	return nil
}

func forward(source *net.TCPConn, dest *net.TCPConn, c chan uint64) {
	defer dest.CloseWrite()
	defer source.CloseRead()

	var written uint64
	buf := mpool.Get()
	for {
		n, err := source.Read(buf)
		if err != nil {
			break
		}
		written += uint64(n)
		_, err = dest.Write(buf[:n])
		if err != nil {
			break
		}
	}
	c <- written
}

func handleConn(source *net.TCPConn) {
	host := chooseHost(options.backend.weight, options.backend.Hosts)
	if host == nil {
		source.Close()
		log.Println("choose host failed")
		return
	}

	dest, err := net.Dial("tcp", host.Addr)
	if err != nil {
		source.Close()
		log.Printf("connect to %s failed: %s", host.Addr, err.Error())
		return
	}

	source.SetKeepAlive(true)
	source.SetKeepAlivePeriod(time.Second * 60)

	go forward(source, dest.(*net.TCPConn), daemon.uploadChan)
	forward(dest.(*net.TCPConn), source, daemon.downloadChan)
}

const SIG_RELOAD = syscall.Signal(35)
const SIG_STATUS = syscall.Signal(36)

func status() {
	log.Printf("num goroutines: %d", runtime.NumGoroutine())
	buf := make([]byte, 32768)
	runtime.Stack(buf, true)
	log.Printf("!!!!!stack!!!!!:%s", buf)
}

func reload() {
	err := reloadConfig()
	if err != nil {
		log.Printf("reload failed:%v", err)
	} else {
		log.Printf("reload succeed")
	}
}

func handleSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, SIG_RELOAD, SIG_STATUS, syscall.SIGTERM, syscall.SIGHUP)

	for sig := range c {
		switch sig {
		case SIG_RELOAD:
			reload()
		case SIG_STATUS:
			status()
		default:
			log.Printf("catch siginal: %v, ignored", sig)
		}
	}
}

var uploadTag = "upload"
var downloadTag = "download"
var uploadStats map[string]interface{}
var downloadStats map[string]interface{}
var updateThreshold uint64 = 1048576

func initStats() {
	uploadStats = make(map[string]interface{})
	uploadStats["name"] = daemon.name

	downloadStats = make(map[string]interface{})
	downloadStats["name"] = daemon.name
}

func updateStats(tag string, amount uint64) {
	trafficUrl := options.backend.TrafficUrl
	if trafficUrl == "" {
		return
	}

	var v interface{}
	if tag == uploadTag {
		uploadStats[tag] = amount
		v = uploadStats
	} else {
		downloadStats[tag] = amount
		v = downloadStats
	}
	chunk, _ := json.Marshal(v)
	reader := bytes.NewReader(chunk)
	resp, err := http.Post(trafficUrl, "application/json", reader)
	if err != nil {
		log.Printf("post traffic stats failed:%s", err.Error())
		return
	}
	resp.Body.Close()
}

func handleDaemon() {
	initStats()

	var spanUpload, spanDownload uint64
	for {
		var upload uint64
		var download uint64
		select {
		case upload = <-daemon.uploadChan:
			spanUpload += upload
			if spanUpload > updateThreshold {
				updateStats(uploadTag, spanUpload)
				spanUpload = 0
			}
		case download = <-daemon.downloadChan:
			spanDownload += download
			if spanDownload > updateThreshold {
				updateStats(downloadTag, spanDownload)
				spanDownload = 0
			}
		case <-time.After(60 * time.Second):
			if spanUpload > 0 {
				updateStats(uploadTag, spanUpload)
				spanUpload = 0
			}
			if spanDownload > 0 {
				updateStats(downloadTag, spanDownload)
				spanDownload = 0
			}
		}
	}
}

func handlePprof() {
	/*
	   log.Println(http.ListenAndServe("localhost:6060", nil))
	*/
}

func init() {
	rand.Seed(time.Now().Unix())
	mpool = NewMPool(4096)
}

func main() {
	var listen string
	var name string
	flag.StringVar(&listen, "listen", ":1248", "local listen port")
	flag.StringVar(&name, "name", "", "access point name(\"\")")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		log.Println("config file is missed.")
		return
	}

	options.config = args[0]
	if err := reloadConfig(); err != nil {
		log.Printf("load config failed:%v", err)
		return
	}
	go handlePprof()
	go handleSignal()

	// run
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		log.Printf("build listener failed:%s", err.Error())
		return
	}
	defer ln.Close()

	if name == "" {
		pos := strings.LastIndex(listen, ":")
		name = listen[pos+1:]
	}
	daemon.name = name
	daemon.downloadChan = make(chan uint64, 1024)
	daemon.uploadChan = make(chan uint64, 1024)
	go handleDaemon()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept failed:%s", err.Error())
			if opErr, ok := err.(*net.OpError); ok {
				if !opErr.Temporary() {
					break
				}
			}
			continue
		}
		go handleConn(conn.(*net.TCPConn))
	}
}
