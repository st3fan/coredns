package satehdns

import (
	"net/netip"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/log"
	"github.com/nats-io/nats.go"
)

const pluginName = "satehdns"

var pluginLog = log.NewWithPlugin(pluginName)

func init() {
	plugin.Register("satehdns", setup)
}

func setup(c *caddy.Controller) error {
	pluginLog.Info("Hello this is setup()")

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		s := &SatehHandler{
			Next: next,
			Zones: map[string]Zone{
				"satehdns.com.": {
					Records: map[string]netip.Addr{
						"localhost": mustParseIPv4("127.0.0.1"),
						"one":       mustParseIPv4("1.1.1.1"),
						"two":       mustParseIPv4("2.2.2.2"),
					},
				},
			},
		}

		c.OnStartup(func() error {
			pluginLog.Info("Hello this is OnStartup")

			conn, err := nats.Connect("tls://connect.ngs.global", nats.UserCredentials("NGS-dns-ns1.creds"))
			if err != nil {
				return plugin.Error("satehdns", err)
			}

			s.conn = conn

			// Subscribe to records.changes and start processing those changes in the background

			subscription, err := s.conn.SubscribeSync("records.changes")
			if err != nil {
				return plugin.Error("satehdns", err)
			}

			go s.fetchRecordChanges(subscription)

			return nil
		})

		c.OnShutdown(func() error {
			pluginLog.Info("Hello this is OnShutdown")

			if s.conn != nil {
				if err := s.conn.Drain(); err != nil {
					return plugin.Error("satehdns", err)
				}
			}

			return nil
		})
		return s
	})

	return nil
}
