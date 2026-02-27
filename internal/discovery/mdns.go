package discovery

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

const ServiceName = "_gophertunnel._tcp"

func StartServer(ctx context.Context, port int) {
	host, _ := os.Hostname()
	info := []string{"GopherTunnel-Peer"}
	service, err := mdns.NewMDNSService(host, ServiceName, "", "", port, nil, info)
	if err != nil {
		fmt.Println("Unable to create mdns service")
		return
	}
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		fmt.Println("Unable to create mdns server")
		return
	}
	defer server.Shutdown()

	// wait for ctrl + c
	<-ctx.Done()
}

func FindPeer(ctx context.Context) (string, error) {
	entriesCh := make(chan *mdns.ServiceEntry, 10)

	// Queryparams
	params := mdns.DefaultParams(ServiceName)
	params.Entries = entriesCh
	params.DisableIPv6 = true
	params.Timeout = time.Second * 10

	fmt.Println("🔍 Searching for peers on local network...")

	go mdns.Query(params)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second * 10):
			fmt.Println("Timeout....")
			return "", fmt.Errorf("No peers found")
		case entry := <-entriesCh:
			fmt.Println(entry)
			// 1. Check if the entry actually belongs to our service
			// Windows/Routers often broadcast generic services; we filter them out here.
			if !strings.Contains(entry.Name, ServiceName) {
				continue
			}

			// 2. Ignore common non-peer IPs (Routers usually end in .1)
			ip := entry.AddrV4.String()
			if entry.AddrV4.IsLoopback() || strings.HasSuffix(ip, ".1") {
				continue
			}

			fmt.Printf("✅ Verified Peer Found: %s at %s\n", entry.Name, ip)
			return ip, nil
		}
	}

}
