//
//   date  : 2014-05-23 17:35
//   author: xjdrew
//

package main

import (
	"flag"
	"fmt"
	"log"
	"log/syslog"

	"encoding/json"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"net"
	"sync"
	"sync/atomic"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s [config]\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(1)
}

type Host struct {
	Addr   string
	Weight int

	addr *net.TCPAddr
}

type Settings struct {
	Hosts  []Host
	weight int
}

type Status struct {
	actives int32
}

type PF struct {
	// host list
	config_file string
	settings    *Settings

	// listen port
	localAddr string

	// listen port
	ln     *net.TCPListener
	wg     sync.WaitGroup
	status Status
}

var pf PF
var logger *log.Logger

func readSettings(config_file string) *Settings {
	fp, err := os.Open(config_file)
	if err != nil {
		logger.Printf("open config file failed:%s", err.Error())
		return nil
	}
	defer fp.Close()

	var settings Settings
	dec := json.NewDecoder(fp)
	err = dec.Decode(&settings)
	if err != nil {
		logger.Printf("decode config file failed:%s", err.Error())
		return nil
	}

	for i := range settings.Hosts {
		host := &settings.Hosts[i]
		host.addr, err = net.ResolveTCPAddr("tcp", host.Addr)
		if err != nil {
			logger.Printf("resolve local addr failed:%s", err.Error())
			return nil
		}
		settings.weight += host.Weight
	}

	logger.Printf("config:%v", settings)
	return &settings
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

func forward(source *net.TCPConn, dest *net.TCPConn) {
	defer func() {
		dest.Close()
	}()

	bufsz := 1 << 12
	cache := make([]byte, bufsz)
	for {
		// pump from source
		n, err1 := source.Read(cache)

		// pour into dest
		c := 0
		var err2 error
		for c < n {
			i, err2 := dest.Write(cache[c:n])
			if err2 != nil {
				break
			}
			c += i
		}

		if err1 != nil || err2 != nil {
			break
		}
	}
}

func handleClient(pf *PF, source *net.TCPConn) {
	atomic.AddInt32(&pf.status.actives, 1)
	defer func() {
		atomic.AddInt32(&pf.status.actives, -1)
		pf.wg.Done()
	}()

	settings := pf.settings
	host := chooseHost(settings.weight, settings.Hosts)
	if host == nil {
		source.Close()
		logger.Println("choose host failed")
		return
	}

	dest, err := net.DialTCP("tcp", nil, host.addr)
	if err != nil {
		source.Close()
		logger.Printf("connect to %s failed: %s", host.addr, err.Error())
		return
	}

	source.SetKeepAlive(true)
	source.SetKeepAlivePeriod(time.Second * 60)
	source.SetLinger(-1)
	dest.SetLinger(-1)

	go forward(source, dest)
	forward(dest, source)
	//logger.Printf("forward finished, %v -> %v", source.RemoteAddr(), host)
}

func start(pf *PF) {
	pf.wg.Add(1)
	defer func() {
		pf.wg.Done()
	}()

	laddr, err := net.ResolveTCPAddr("tcp", pf.localAddr)
	if err != nil {
		logger.Printf("resolve local addr failed:%s", err.Error())
		return
	}

	ln, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		logger.Printf("build listener failed:%s", err.Error())
		return
	}

	pf.ln = ln
	for {
		conn, err := pf.ln.AcceptTCP()
		if err != nil {
			logger.Printf("accept failed:%s", err.Error())
			if opErr, ok := err.(*net.OpError); ok {
				if !opErr.Temporary() {
					break
				}
			}
			continue
		}
		pf.wg.Add(1)
		go handleClient(pf, conn)
	}
}

const SIG_RELOAD = syscall.Signal(34)
const SIG_STATUS = syscall.Signal(35)

func reload() {
	settings := readSettings(pf.config_file)
	if settings == nil {
		logger.Println("reload failed")
		return
	}
	pf.settings = settings
	logger.Println("reload succeed")
}

func status() {
	logger.Printf("status: actives-> %d", pf.status.actives)
}

func handleSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, SIG_RELOAD, SIG_STATUS, syscall.SIGTERM)

	for sig := range c {
		switch sig {
		case SIG_RELOAD:
			reload()
		case SIG_STATUS:
			status()
		case syscall.SIGTERM:
			logger.Println("catch sigterm, ignore")
		}
	}
}

func init() {
	var err error
	logger, err = syslog.NewLogger(syslog.LOG_LOCAL0, 0)
	if err != nil {
		fmt.Printf("create logger failed:%s", err.Error())
		os.Exit(1)
	}
	logger.Println("are you lucky? go!")
	rand.Seed(time.Now().Unix())
}

func main() {
	flag.StringVar(&pf.localAddr, "listen_addr", "0.0.0.0:1248", "local listen port(0.0.0.0:1248)")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		logger.Println("config file is missing.")
		os.Exit(1)
	}

	pf.config_file = args[0]
	logger.Printf("config file is: %s", pf.config_file)

	pf.settings = readSettings(pf.config_file)
	if pf.settings == nil {
		logger.Println("parse config failed")
		os.Exit(1)
	}

	go handleSignal()

	// run
	start(&pf)
	pf.wg.Wait()
}
