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

	// We try up to 3 times automatically
	for attempt := 1; attempt <= 3; attempt++ {
		fmt.Printf("🔍 Searching for peers (Attempt %d/3)...\n", attempt)

		entriesCh := make(chan *mdns.ServiceEntry, 10)
		params := mdns.DefaultParams(ServiceName)
		params.Entries = entriesCh
		params.DisableIPv6 = true
		params.Timeout = time.Second * 10

		go mdns.Query(params)

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case entry := <-entriesCh:
			fmt.Println(entry)
			// 1. Check if the entry actually belongs to our service
			// Windows/Routers often broadcast generic services; we filter them out here.
			if !strings.Contains(entry.Name, ServiceName) {
				continue
			}

			// 2. Ignore common non-peer IPs
			ip := entry.AddrV4.String()
			if entry.AddrV4.IsLoopback() {
				continue
			}

			fmt.Printf("✅ Verified Peer Found: %s at %s\n", entry.Name, ip)
			return ip, nil
		case <-time.After(time.Second * 10):
			fmt.Println("Timeout....")
			// No peer found in this burst, loop will retry
			continue
		}
	}
	return "", fmt.Errorf("❌ No GopherTunnel peer found after 3 attempts")

}
