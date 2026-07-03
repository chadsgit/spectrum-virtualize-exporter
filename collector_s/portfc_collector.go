// Copyright 2021-2024 IBM Corp. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collector_s

import (
	"fmt"

	"github.com/IBM/spectrum-virtualize-exporter/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
)

const prefix_portfc = "portfc_"

var (
	portfc_status         *prometheus.Desc
	portfc_attachment     *prometheus.Desc
	portfc_bytes_sent     *prometheus.Desc
	portfc_bytes_received *prometheus.Desc
	portfc_bb_credit_zero *prometheus.Desc
	portfc_link_failures  *prometheus.Desc
)

func init() {
	registerCollector("lsportfc", defaultEnabled, NewPortfcCollector)
}

// portfcCollector collects portfc setting metrics
type portfcCollector struct {
}

func NewPortfcCollector() (Collector, error) {
	labelnames_status := []string{"resource", "node_name", "port_id", "wwpn", "port_speed", "cluster_use"}
	labelnames_attachment := []string{"resource", "node_name", "port_id", "wwpn", "port_speed", "cluster_use"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames_status = append(labelnames_status, utils.ExtraLabelNames...)
		labelnames_attachment = append(labelnames_attachment, utils.ExtraLabelNames...)
	}
	labelnames_stats := []string{"resource", "node_name", "port_id", "wwpn"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames_stats = append(labelnames_stats, utils.ExtraLabelNames...)
	}
	portfc_status = prometheus.NewDesc(prefix_portfc+"status", "Indicates whether the port is configured to a device of Fibre Channel (FC) port. 0-active; 1-inactive_configured; 2-inactive_unconfigured.", labelnames_status, nil)
	portfc_attachment = prometheus.NewDesc(prefix_portfc+"attachment", "Indicates if the port is attached to a FC switch. 0-yes; 1-no.", labelnames_attachment, nil)
	portfc_bytes_sent = prometheus.NewDesc(prefix_portfc+"bytes_sent_total", "Total bytes sent on the FC port.", labelnames_stats, nil)
	portfc_bytes_received = prometheus.NewDesc(prefix_portfc+"bytes_received_total", "Total bytes received on the FC port.", labelnames_stats, nil)
	portfc_bb_credit_zero = prometheus.NewDesc(prefix_portfc+"bb_credit_zero_total", "Number of times BB credit has dropped to zero on the FC port. Non-zero values indicate fabric congestion.", labelnames_stats, nil)
	portfc_link_failures = prometheus.NewDesc(prefix_portfc+"link_failures_total", "Total link failure count on the FC port.", labelnames_stats, nil)
	return &portfcCollector{}, nil
}

// Describe describes the metrics
func (*portfcCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- portfc_status
	ch <- portfc_attachment
	ch <- portfc_bytes_sent
	ch <- portfc_bytes_received
	ch <- portfc_bb_credit_zero
	ch <- portfc_link_failures
}

// Collect collects metrics from Spectrum Virtualize Restful API
func (c *portfcCollector) Collect(sClient utils.SpectrumClient, ch chan<- prometheus.Metric) error {

	logger.Debugln("entering portfc collector ...")
	respData, err := sClient.CallSpectrumAPI("lsportfc", true)
	if err != nil {
		logger.Errorf("executing lsportfc cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lsportfc: ", respData)
	/* This is a sample output of lsportfc
	[
		{
			"id": "0",
			"fc_io_port_id": "1",
			"port_id": "1",
			"type": "fc",
			"port_speed": "16Gb",
			"node_id": "1",
			"node_name": "node1",
			"WWPN": "500507681011038D",
			"nportid": "010400",
			"status": "active",
			"attachment": "switch",
			"cluster_use": "local_partner",
			"adapter_location": "1",
			"adapter_port_id": "1"
		},
		...
		{
			"id": "16",
			"fc_io_port_id": "1",
			"port_id": "1",
			"type": "fc",
			"port_speed": "16Gb",
			"node_id": "2",
			"node_name": "node2",
			"WWPN": "500507681011039F",
			"nportid": "010600",
			"status": "active",
			"attachment": "switch",
			"cluster_use": "local_partner",
			"adapter_location": "1",
			"adapter_port_id": "1"
		},
		...
	] */
	if !gjson.Valid(respData) {
		return fmt.Errorf("invalid json for lsportfc:\n%v", respData)
	}
	jsonPorts := gjson.Parse(respData)
	jsonPorts.ForEach(func(key, port gjson.Result) bool {
		port_id := port.Get("port_id").String()
		node_name := port.Get("node_name").String()
		wwpn := port.Get("WWPN").String()
		port_speed := port.Get("port_speed").String()
		cluster_use := port.Get("cluster_use").String()
		status := port.Get("status").String() // ["active", "inactive_configured", "inactive_unconfigured"]
		attachment := port.Get("attachment").String()

		v_status := 0
		switch status {
		case "active":
			v_status = 0
		case "inactive_configured":
			v_status = 1
		case "inactive_unconfigured":
			v_status = 2
		}
		v_attachment := 0
		if attachment != "switch" {
			v_attachment = 1
		}

		labelvalues := []string{sClient.Hostname, node_name, port_id, wwpn, port_speed, cluster_use}
		if len(utils.ExtraLabelValues) > 0 {
			labelvalues = append(labelvalues, utils.ExtraLabelValues...)
		}

		ch <- prometheus.MustNewConstMetric(portfc_status, prometheus.GaugeValue, float64(v_status), labelvalues...)
		ch <- prometheus.MustNewConstMetric(portfc_attachment, prometheus.GaugeValue, float64(v_attachment), labelvalues...)
		return true
	})

	// fetch FC port I/O stats for BB credit zero and throughput counters
	statsData, err := sClient.CallSpectrumAPI("lsportfcstats", true)
	if err != nil {
		logger.Errorf("executing lsportfcstats cmd failed: %s", err.Error())
		return nil // non-fatal; lsportfc data already emitted
	}
	logger.Debugln("response of lsportfcstats: ", statsData)
	/* Sample output of lsportfcstats
	[
	  {
	    "node_id": "1",
	    "node_name": "node1",
	    "port_id": "1",
	    "WWPN": "500507681011038D",
	    "bytes_sent": "12345678",
	    "bytes_received": "87654321",
	    "frames_sent": "1000",
	    "frames_received": "1001",
	    "link_failures": "0",
	    "loss_of_sync_errors": "0",
	    "loss_of_signal_errors": "0",
	    "primitive_seq_protocol_errors": "0",
	    "invalid_xmission_words": "0",
	    "invalid_crcs": "0",
	    "bb_credit_zero": "0"
	  }
	] */
	if gjson.Valid(statsData) {
		gjson.Parse(statsData).ForEach(func(key, stat gjson.Result) bool {
			node_name := stat.Get("node_name").String()
			port_id := stat.Get("port_id").String()
			wwpn := stat.Get("WWPN").String()

			statLabels := []string{sClient.Hostname, node_name, port_id, wwpn}
			if len(utils.ExtraLabelValues) > 0 {
				statLabels = append(statLabels, utils.ExtraLabelValues...)
			}

			ch <- prometheus.MustNewConstMetric(portfc_bytes_sent, prometheus.CounterValue, stat.Get("bytes_sent").Float(), statLabels...)
			ch <- prometheus.MustNewConstMetric(portfc_bytes_received, prometheus.CounterValue, stat.Get("bytes_received").Float(), statLabels...)
			ch <- prometheus.MustNewConstMetric(portfc_bb_credit_zero, prometheus.CounterValue, stat.Get("bb_credit_zero").Float(), statLabels...)
			ch <- prometheus.MustNewConstMetric(portfc_link_failures, prometheus.CounterValue, stat.Get("link_failures").Float(), statLabels...)
			return true
		})
	}

	logger.Debugln("exit portfc exit")
	return nil
}
