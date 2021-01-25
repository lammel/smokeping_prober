// Copyright 2018 Ben Kochie <superq@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"github.com/go-ping/ping"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

const (
	namespace = "smokeping"
)

var (
	labelNames = []string{"ip", "host"}

	pingResponseTTL = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "response_ttl",
			Help:      "The last response Time To Live (TTL).",
		},
		labelNames,
	)

	pingPacketsSent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "packets_sent_total",
			Help:      "Number of ping packets sent",
		},
		labelNames,
	)

	pingPacketsRecv = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "packets_recv_total",
			Help:      "Number of ping packets received (to calculate loss)",
		},
		labelNames,
	)

	pingPacketLoss = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "packet_loss_percent",
			Help:      "The packet loss in percent",
		},
		labelNames,
	)
	
)

func init() {
	prometheus.MustRegister(pingResponseTTL)
	prometheus.MustRegister(pingPacketsSent)
	prometheus.MustRegister(pingPacketsRecv)
	prometheus.MustRegister(pingPacketLoss)
}

func newPingResponseHistogram(buckets []float64) *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "response_duration_seconds",
			Help:      "A histogram of latencies for ping responses.",
			Buckets:   buckets,
		},
		labelNames,
	)
}

// SmokepingCollector collects metrics from the pinger.
type SmokepingCollector struct {
	pingers *[]*ping.Pinger

	requestsSent *prometheus.Desc
}

// NewSmokepingCollector return a SmokepingCollector
func NewSmokepingCollector(pingers *[]*ping.Pinger, pingResponseSeconds prometheus.HistogramVec) *SmokepingCollector {
	for _, pinger := range *pingers {
		pinger.OnRecv = func(pkt *ping.Packet) {
			pingResponseSeconds.WithLabelValues(pkt.IPAddr.String(), pkt.Addr).Observe(pkt.Rtt.Seconds())
			pingResponseTTL.WithLabelValues(pkt.IPAddr.String(), pkt.Addr).Set(float64(pkt.Ttl))
			log.Debugf("%d bytes from %s: icmp_seq=%d time=%v ttl=%v\n",
				pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt, pkt.Ttl)
		}
		pinger.OnFinish = func(stats *ping.Statistics) {
			log.Debugf("\n--- %s ping statistics ---\n", stats.Addr)
			log.Debugf("%d packets transmitted, %d packets received, %v%% packet loss\n",
				stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss)
			log.Debugf("round-trip min/avg/max/stddev = %v/%v/%v/%v\n",
				stats.MinRtt, stats.AvgRtt, stats.MaxRtt, stats.StdDevRtt)
		}
	}

	return &SmokepingCollector{
		pingers: pingers,
		requestsSent: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "requests_total"),
			"Number of ping requests sent",
			labelNames,
			nil,
		),
	}
}

func (s *SmokepingCollector) Describe(ch chan<- *prometheus.Desc) {
 	ch <- s.requestsSent
}

func (s *SmokepingCollector) Collect(ch chan<- prometheus.Metric) {
	for _, pinger := range *s.pingers {
		stats := pinger.Statistics()

		pingPacketsSent.WithLabelValues(stats.IPAddr.String(), stats.Addr).Set(float64(stats.PacketsSent))
		pingPacketsRecv.WithLabelValues(stats.IPAddr.String(), stats.Addr).Set(float64(stats.PacketsRecv))
		pingPacketLoss.WithLabelValues(stats.IPAddr.String(), stats.Addr).Set(stats.PacketLoss)

		ch <- prometheus.MustNewConstMetric(
			s.requestsSent,
			prometheus.CounterValue,
			float64(stats.PacketsSent),
			stats.IPAddr.String(),
			stats.Addr,
		)
	}
}
