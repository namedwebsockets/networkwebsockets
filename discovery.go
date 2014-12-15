package networkwebsockets

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/richtr/bcrypt"
	"github.com/richtr/mdns"
)

const (
	ipv4mdns = "224.0.0.251"
	ipv6mdns = "ff02::fb"
	mdnsPort = 5406 // operate on our own multicast port (standard mDNS port is 5353)
)

var (
	network_ipv4Addr = &net.UDPAddr{
		IP:   net.ParseIP(ipv4mdns),
		Port: mdnsPort,
	}
	network_ipv6Addr = &net.UDPAddr{
		IP:   net.ParseIP(ipv6mdns),
		Port: mdnsPort,
	}
)

/** Named Web Socket DNS-SD Discovery Client interface **/

type DiscoveryService struct {
	Name string
	Hash string
	Path string
	Port int

	server *mdns.Server
}

func NewDiscoveryService(name, hash, path string, port int) *DiscoveryService {
	discoveryService := &DiscoveryService{
		Name: name,
		Hash: hash,
		Path: path,
		Port: port,
	}

	return discoveryService
}

func (dc *DiscoveryService) Register(domain string) {
	dnssdServiceId := GenerateId()

	s := &mdns.MDNSService{
		Instance: dnssdServiceId,
		Service:  "_nws._tcp",
		Domain:   domain,
		Port:     dc.Port,
		Info:     fmt.Sprintf("hash=%s,path=%s", dc.Hash, dc.Path),
	}

	if err := s.Init(); err != nil {
		log.Printf("Could not register service on network. %v", err)
		return
	}

	var mdnsClientConfig *mdns.Config

	// Advertise service to the correct endpoint (local or network)
	mdnsClientConfig = &mdns.Config{
		IPv4Addr: network_ipv4Addr,
		IPv6Addr: network_ipv6Addr,
	}

	// Add the DNS zone record to advertise
	mdnsClientConfig.Zone = s

	serv, err := mdns.NewServer(mdnsClientConfig)

	if err != nil {
		log.Printf("Failed to create new mDNS server. %v", err)
		return
	}

	dc.server = serv

	log.Printf("New '%s' channel advertised as '%s' in %s network", dc.Name, fmt.Sprintf("%s._nws._tcp", dnssdServiceId), domain)
}

func (dc *DiscoveryService) Shutdown() {
	if dc.server != nil {
		dc.server.Shutdown()
	}
}

/** Named Web Socket DNS-SD Discovery Server interface **/

type DiscoveryBrowser struct {
	// Network Web Socket DNS-SD records currently unresolved by this proxy instance
	cachedDNSRecords map[string]*NetworkWebSocket_DNSRecord
	inprogress bool
	closed bool
}

func NewDiscoveryBrowser() *DiscoveryBrowser {
	discoveryBrowser := &DiscoveryBrowser{
		cachedDNSRecords: make(map[string]*NetworkWebSocket_DNSRecord),
		inprogress:       false,
		closed:           false,
	}

	return discoveryBrowser
}

func (ds *DiscoveryBrowser) Browse(service *NetworkWebSocket_Service, intervalSeconds, timeoutSeconds int) {

	// Don't run two browse processes at the same time
	if ds.inprogress {
		return
	}

	ds.inprogress = true

	entries := make(chan *mdns.ServiceEntry, 255)

	recordsCache := make(map[string]*NetworkWebSocket_DNSRecord)

	timeout := time.Duration(timeoutSeconds) * time.Second
	interval := time.Duration(intervalSeconds) * time.Second

	var targetIPv4 *net.UDPAddr
	var targetIPv6 *net.UDPAddr

	targetIPv4 = network_ipv4Addr
	targetIPv6 = network_ipv6Addr

	// Only look for Named Web Socket DNS-SD services
	params := &mdns.QueryParam{
		Service:  "_nws._tcp",
		Domain:   "local",
		Timeout:  timeout,
		Entries:  entries,
		IPv4mdns: targetIPv4,
		IPv6mdns: targetIPv6,
	}

	go func() {
		complete := false
		timeoutFinish := time.After(timeout)
		intervalFinish := time.After(interval)

		// Wait for responses until timeout
		for !complete {
			select {
			case discoveredService, ok := <-entries:

				if !ok {
					continue
				}

				serviceRecord, err := NewNetworkWebSocketRecordFromDNSRecord(discoveredService)
				if err != nil {
					log.Printf("err: %v", err)
					continue
				}

				// Ignore our own NetworkWebSocket services
				if service.isOwnProxyService(serviceRecord) {
					continue
				}

				// Ignore previously discovered NetworkWebSocket proxy services
				if service.isActiveProxyService(serviceRecord) {
					continue
				}

				// Resolve discovered service hash provided against available services
				var sock *NetworkWebSocket
				for _, knownService := range service.Channels {
					if bcrypt.Match(knownService.serviceName, serviceRecord.Hash_BCrypt) {
						sock = knownService
						break
					}
				}

				if sock != nil {
					// Create new web socket connection toward discovered proxy
					if _, dErr := sock.dialFromDNSRecord(serviceRecord); dErr != nil {
						log.Printf("err: %v", dErr)
						continue
					}
				} else {
					// Store as an unresolved DNS-SD record
					recordsCache[serviceRecord.Hash_Base64] = serviceRecord
					continue
				}

			case <-timeoutFinish:
				// Replace unresolved DNS records cache
				ds.cachedDNSRecords = recordsCache

				ds.inprogress = false
			case <-intervalFinish:
				complete = true
			}
		}
	}()

	// Run the mDNS/DNS-SD query
	err := mdns.Query(params)

	if err != nil {
		log.Printf("Could not perform mDNS/DNS-SD query. %v", err)
		time.Sleep(interval) // sleep until next loop is scheduled
		return
	}
}

func (ds *DiscoveryBrowser) Shutdown() {
	ds.closed = true
}

/** Named Web Socket DNS Record interface **/

type NetworkWebSocket_DNSRecord struct {
	*mdns.ServiceEntry

	Path        string
	Hash_Base64 string
	Hash_BCrypt string
}

func NewNetworkWebSocketRecordFromDNSRecord(serviceEntry *mdns.ServiceEntry) (*NetworkWebSocket_DNSRecord, error) {
	servicePath := ""
	serviceHash_Base64 := ""
	serviceHash_BCrypt := ""

	if serviceEntry.Info == "" {
		return nil, errors.New("Could not find associated TXT record for advertised Named Web Socket service")
	}

	serviceParts := strings.FieldsFunc(serviceEntry.Info, func(r rune) bool {
		return r == '=' || r == ',' || r == ';' || r == ' '
	})
	if len(serviceParts) > 1 {
		for i := 0; i < len(serviceParts); i += 2 {
			if strings.ToLower(serviceParts[i]) == "path" {
				servicePath = serviceParts[i+1]
			}
			if strings.ToLower(serviceParts[i]) == "hash" {
				serviceHash_Base64 = serviceParts[i+1]

				if res, err := base64.StdEncoding.DecodeString(serviceHash_Base64); err == nil {
					serviceHash_BCrypt = string(res)
				}
			}
		}
	}

	if servicePath == "" || serviceHash_Base64 == "" || serviceHash_BCrypt == "" {
		return nil, errors.New("Could not resolve the provided Named Web Socket DNS Record")
	}

	// Create and return a new Named Web Socket DNS Record with the parsed information
	newNetworkWebSocketDNSRecord := &NetworkWebSocket_DNSRecord{serviceEntry, servicePath, serviceHash_Base64, serviceHash_BCrypt}

	return newNetworkWebSocketDNSRecord, nil
}
