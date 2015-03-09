// list of unused ports

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
)

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
	var from, to int
	flag.StringVar(&ip, "ip", "0.0.0.0", "ip address to be bound")
	flag.IntVar(&from, "from", 10000, "port range begin")
	flag.IntVar(&to, "to", 10100, "port range end")
	flag.Usage = usage
	flag.Parse()

	for i := from; i < to; i++ {
		if isPortOpened(&ip, i) {
			fmt.Println(i)
		}
	}
}
