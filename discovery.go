package networkwebsockets

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/richtr/bcrypt"
	"github.com/richtr/mdns"
)

const (
	network_ipv4mdns = "224.0.0.251"
	network_ipv6mdns = "ff02::fb"
	network_mdnsPort = 5353
)

var (
	network_ipv4Addr = &net.UDPAddr{
		IP:   net.ParseIP(network_ipv4mdns),
		Port: network_mdnsPort,
	}
	network_ipv6Addr = &net.UDPAddr{
		IP:   net.ParseIP(network_ipv6mdns),
		Port: network_mdnsPort,
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
	rand.Seed(time.Now().UTC().UnixNano())
	dnssdServiceId := fmt.Sprintf("%d", rand.Int())

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
	closed bool
}

func NewDiscoveryBrowser() *DiscoveryBrowser {
	discoveryBrowser := &DiscoveryBrowser{}

	return discoveryBrowser
}

func (ds *DiscoveryBrowser) Browse(service *NamedWebSocket_Service, timeoutSeconds int) {

	entries := make(chan *mdns.ServiceEntry, 255)

	unresolvedServiceRecords := make(map[string]*NamedWebSocket_DNSRecord)

	timeout := time.Duration(timeoutSeconds) * time.Second

	var targetIPv4 *net.UDPAddr
	var targetIPv6 *net.UDPAddr
	var group *NamedWebSocket_Service_Group

	targetIPv4 = network_ipv4Addr
	targetIPv6 = network_ipv6Addr

	group = service.networkSockets

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
		finish := time.After(timeout)

		// Wait for responses until timeout
		for !complete {
			select {
			case discoveredService, ok := <-entries:

				if !ok {
					continue
				}

				serviceRecord, err := NewNamedWebSocketRecordFromDNSRecord(discoveredService)
				if err != nil {
					log.Printf("err: %v", err)
					continue
				}

				// Ignore our own NetworkWebSocket services
				if isOwned := group.AdvertisedServiceHashes[serviceRecord.Hash_Base64]; isOwned {
					continue
				}

				// Ignore previously discovered NetworkWebSocket services
				if isResolved := group.ResolvedServiceRecords[serviceRecord.Hash_Base64]; isResolved != nil {
					continue
				}

				serviceName := ""
				localServicePath := ""

				// Resolve service hash provided against advertised services
				isKnown := false
				for knownServiceName := range group.knownServiceNames {
					if bcrypt.Match(knownServiceName, serviceRecord.Hash_BCrypt) {
						serviceName = knownServiceName
						localServicePath = fmt.Sprintf("/network/%s", knownServiceName)
						isKnown = true
						break
					}
				}

				if !isKnown {
					// Store as an unresolved DNS record
					unresolvedServiceRecords[serviceRecord.Hash_Base64] = serviceRecord
					continue
				}

				// Resolve websocket connection
				sock := group.Services[localServicePath]
				if sock == nil {
					sock = NewNamedWebSocket(service, serviceName, service.Port, false)
					group.Services[localServicePath] = sock
				}

				// Create proxy websocket connection
				if _, dErr := sock.dialDNSRecord(serviceRecord, serviceName); dErr != nil {
					log.Printf("err: %v", dErr)
					continue
				}

				// Set DNS record as resolved
				group.ResolvedServiceRecords[serviceRecord.Hash_Base64] = serviceRecord

			case <-finish:
				complete = true
			}
		}

		// Replace unresolved DNS records cache
		group.UnresolvedServiceRecords = unresolvedServiceRecords

	}()

	// Run the mDNS/DNS-SD query
	err := mdns.Query(params)

	if err != nil {
		log.Printf("Could not perform mDNS/DNS-SD query. %v", err)
		time.Sleep(time.Second * timeoutSeconds) // sleep until next loop is scheduled
		return
	}
}

func (ds *DiscoveryBrowser) Shutdown() {
	ds.closed = true
}

/** Named Web Socket DNS Record interface **/

type NamedWebSocket_DNSRecord struct {
	*mdns.ServiceEntry

	Path        string
	Hash_Base64 string
	Hash_BCrypt string
}

func NewNamedWebSocketRecordFromDNSRecord(serviceEntry *mdns.ServiceEntry) (*NamedWebSocket_DNSRecord, error) {
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
	newNamedWebSocketDNSRecord := &NamedWebSocket_DNSRecord{serviceEntry, servicePath, serviceHash_Base64, serviceHash_BCrypt}

	return newNamedWebSocketDNSRecord, nil
}
