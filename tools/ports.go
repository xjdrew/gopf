// list of unused ports

package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(1)
}

func isPortOpened(ip *string, port int) bool {
	laddr := fmt.Sprintf("%s:%d", *ip, port)
	ln, err := net.Listen("tcp", laddr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func main() {
	var ip string
	var from, to, max int
	var random bool

	flag.StringVar(&ip, "ip", "0.0.0.0", "ip address to be bound")
	flag.IntVar(&from, "from", 10000, "port range begin")
	flag.IntVar(&to, "to", 10100, "port range end")
	flag.IntVar(&max, "max", 0, "max number of unused ports found before return")
	flag.BoolVar(&random, "random", true, "begin from rand[from, to)")

	flag.Usage = usage
	flag.Parse()

	scope := to - from
	if scope <= 0 {
		return
	}

	port := from
	if random {
		port = from + rand.Intn(scope)
	}

	n := 0
	for i := 0; i < scope; i++ {
		if port >= to {
			port = port - to + from
		}
		if isPortOpened(&ip, port) {
			fmt.Println(port)
			if max > 0 {
				n++
				if max == n {
					break
				}
			}
		}
		port++
	}
}
