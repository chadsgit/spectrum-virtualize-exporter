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

const prefix_portip = "portip_"

var (
	portip_link_state *prometheus.Desc
)

func init() {
	registerCollector("lsportip", defaultDisabled, NewPortipCollector)
}

// portipCollector collects IP port (iSCSI/iSER) status metrics
type portipCollector struct{}

func NewPortipCollector() (Collector, error) {
	labelnames := []string{"resource", "node_name", "port_id", "ip_address", "port_type", "speed"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames = append(labelnames, utils.ExtraLabelNames...)
	}
	portip_link_state = prometheus.NewDesc(
		prefix_portip+"link_state",
		"Link state of the IP port. 0-active; 1-inactive.",
		labelnames, nil)
	return &portipCollector{}, nil
}

// Describe describes the metrics
func (*portipCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- portip_link_state
}

// Collect collects metrics from Spectrum Virtualize Restful API
func (c *portipCollector) Collect(sClient utils.SpectrumClient, ch chan<- prometheus.Metric) error {
	logger.Debugln("entering portip collector ...")
	respData, err := sClient.CallSpectrumAPI("lsportip", true)
	if err != nil {
		logger.Errorf("executing lsportip cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lsportip: ", respData)
	/* Sample output of lsportip
	[
	  {
	    "id": "1",
	    "node_id": "1",
	    "node_name": "node1",
	    "IP_address": "192.168.1.10",
	    "mask": "255.255.255.0",
	    "gateway": "192.168.1.1",
	    "IP_address_6": "",
	    "prefix_6": "",
	    "gateway_6": "",
	    "MAC": "00:11:22:33:44:55",
	    "duplex": "full",
	    "state": "configured",
	    "speed": "10Gb",
	    "failover": "no",
	    "link_state": "active",
	    "host": "yes",
	    "port_type": "iscsi"
	  }
	] */
	if !gjson.Valid(respData) {
		return fmt.Errorf("invalid json for lsportip:\n%v", respData)
	}
	gjson.Parse(respData).ForEach(func(key, port gjson.Result) bool {
		// skip unconfigured ports (no IP assigned)
		if port.Get("state").String() == "unconfigured" {
			return true
		}

		port_id := port.Get("id").String()
		node_name := port.Get("node_name").String()
		ip_address := port.Get("IP_address").String()
		port_type := port.Get("port_type").String()
		speed := port.Get("speed").String()
		link_state := port.Get("link_state").String()

		v_link_state := 0
		if link_state != "active" {
			v_link_state = 1
		}

		labelvalues := []string{sClient.Hostname, node_name, port_id, ip_address, port_type, speed}
		if len(utils.ExtraLabelValues) > 0 {
			labelvalues = append(labelvalues, utils.ExtraLabelValues...)
		}
		ch <- prometheus.MustNewConstMetric(portip_link_state, prometheus.GaugeValue, float64(v_link_state), labelvalues...)
		return true
	})

	logger.Debugln("exit portip collector")
	return nil
}
