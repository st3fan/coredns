package satehdns

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/nats-io/nats.go"
)

type UpsertRecordEvent struct {
	Name  string
	Type  string
	Value string
	TTL   int
}

func (s *SatehHandler) handleUpsertRecord(msg *nats.Msg) error {
	var event UpsertRecordEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return err
	}

	// This is hacky to get a PoC going. Needs nice database abstractions, etc.

	s.Lock()
	defer s.Unlock()

	if host, domain, found := strings.Cut(event.Name, "."); found {
		if zone, found := s.Zones[domain]; found {
			zone.Records[host] = mustParseIPv4(event.Value)
		}
	}

	return nil
}

type DeleteRecordEvent struct {
	Name string
}

func (s *SatehHandler) handleDeleteRecord(msg *nats.Msg) error {
	var event UpsertRecordEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return err
	}

	s.Lock()
	defer s.Unlock()

	if host, domain, found := strings.Cut(event.Name, "."); found {
		if zone, found := s.Zones[domain]; found {
			delete(zone.Records, host)
		}
	}

	return nil
}

type SnapshotAvailableEvent struct {
	URL string
}

func (s *SatehHandler) handleSnapshotAvailable(msg *nats.Msg) error {
	return nil
}

func (s *SatehHandler) fetchRecordChanges(sub *nats.Subscription) {
	for {
		msg, err := sub.NextMsg(25 * time.Millisecond)
		if err != nil {
			plugin.Error("satehdns", err) // What do we do now? Retry? Reconnect?
		}

		if msg != nil {
			eventType := msg.Header.Get("X-EventType")
			if eventType == "" {
				pluginLog.Error("Received event without X-EventType header")
				continue
			}

			eventVersion := msg.Header.Get("X-EventVersion")
			if eventType == "" {
				pluginLog.Error("Received event without X-EventVersion header")
				continue
			}

			if eventVersion != "2024-09-23" {
				pluginLog.Errorf("Received event with unsupported X-EventVersion: %s", eventVersion)
				continue
			}

			switch eventType {
			case "UpsertRecord":
				if err := s.handleUpsertRecord(msg); err != nil {
					pluginLog.Errorf("Failed to handle UpsertRecord event: %s", err)
				}
			case "DeleteRecord":
				if err := s.handleUpsertRecord(msg); err != nil {
					pluginLog.Errorf("Failed to handle DeleteRecord event: %s", err)
				}
			case "SnapshotAvailable":
				if err := s.handleSnapshotAvailable(msg); err != nil {
					pluginLog.Errorf("Failed to handle SnapshotAvailable event: %s", err)
				}
			default:
				pluginLog.Errorf("Received event with unknown X-Event header: %s", eventType)
			}
		}
	}
}
