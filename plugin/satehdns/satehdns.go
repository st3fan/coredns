package satehdns

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"sync"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"github.com/nats-io/nats.go"
)

type Zone struct {
	Records map[string]netip.Addr // We only support A records
}

type SatehHandler struct {
	sync.RWMutex // Probably not the right place but works for now
	Next         plugin.Handler
	Zones        map[string]Zone
	conn         *nats.Conn
	state        string // syncing, ready
}

func (sh *SatehHandler) findAddr(s string) (netip.Addr, bool) {
	sh.RLock()
	defer sh.RUnlock()

	if host, domain, found := strings.Cut(s, "."); found {
		if zone, found := sh.Zones[domain]; found {
			if addr, found := zone.Records[host]; found {
				return addr, true
			}
		}
	}

	return netip.Addr{}, false
}

func SOA(ctx context.Context, zone string, state request.Request) []dns.RR {
	Mbox := dnsutil.Join("hostmaster", zone)
	Ns := dnsutil.Join("ns1", zone) // Comes from config file or hostname?

	return []dns.RR{
		&dns.SOA{
			Hdr: dns.RR_Header{
				Name:   zone,
				Rrtype: dns.TypeSOA,
				Ttl:    300,
				Class:  dns.ClassINET,
			},
			Mbox:    Mbox,
			Ns:      Ns,
			Serial:  1234567890,
			Refresh: 7200,
			Retry:   1800,
			Expire:  86400,
			Minttl:  300, // Is this special in a SOA record?
		},
	}
}

func (h *SatehHandler) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	// We only respond to A records
	state := request.Request{W: w, Req: r}
	if state.QClass() != dns.ClassINET || state.QType() != dns.TypeA {
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}

	// Lookup the record. We have some static records too, so even if we're not synced
	// yet, findAddr could return a result.

	addr, found := h.findAddr(state.Name())
	if !found {
		m := new(dns.Msg)
		m.Authoritative = true
		m.Ns = SOA(ctx, "satehdns.com.", state)
		state.W.WriteMsg(m)

		if !h.Ready() {
			// If we are not ready then return SERVFAIL
			m.SetRcode(state.Req, dns.RcodeServerFailure)
		} else {
			// If we can't find the record then return NXDOMAIN
			m.SetRcode(state.Req, dns.RcodeNameError)
		}

		state.W.WriteMsg(m)
		return 0, nil
	}

	// We would a response to return the A record

	m := new(dns.Msg)
	m.SetReply(r)
	m.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   state.QName(),
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    300,
			},
			A: net.IP(addr.AsSlice()),
		},
	}

	w.WriteMsg(m)

	return 0, nil
}

func (h *SatehHandler) Name() string {
	return "satehdns"
}
