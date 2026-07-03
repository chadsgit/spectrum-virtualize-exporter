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

const prefix_quorum = "quorum_"

var (
	quorum_status *prometheus.Desc
	quorum_active *prometheus.Desc
)

func init() {
	registerCollector("lsquorum", defaultEnabled, NewQuorumCollector)
}

type quorumCollector struct{}

func NewQuorumCollector() (Collector, error) {
	labelnames := []string{"resource", "quorum_id", "object_name", "object_type"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames = append(labelnames, utils.ExtraLabelNames...)
	}
	quorum_status = prometheus.NewDesc(prefix_quorum+"status",
		"Status of the quorum disk. 0-online; 1-offline; 2-degraded.",
		labelnames, nil)
	quorum_active = prometheus.NewDesc(prefix_quorum+"active",
		"Whether this quorum disk is currently active. 1-active; 0-inactive.",
		labelnames, nil)
	return &quorumCollector{}, nil
}

func (*quorumCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- quorum_status
	ch <- quorum_active
}

func (c *quorumCollector) Collect(sClient utils.SpectrumClient, ch chan<- prometheus.Metric) error {
	logger.Debugln("entering quorum collector ...")
	respData, err := sClient.CallSpectrumAPI("lsquorum", true)
	if err != nil {
		logger.Errorf("executing lsquorum cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lsquorum: ", respData)
	/* Sample output of lsquorum
	[
	  {
	    "quorum_id": "0",
	    "status": "online",
	    "object_type": "mdisk",
	    "object_id": "0",
	    "object_name": "mdisk0",
	    "override": "no",
	    "site_id": "",
	    "site_name": ""
	  }
	] */
	if !gjson.Valid(respData) {
		return fmt.Errorf("invalid json for lsquorum:\n%v", respData)
	}

	gjson.Parse(respData).ForEach(func(key, quorum gjson.Result) bool {
		quorum_id := quorum.Get("quorum_id").String()
		object_name := quorum.Get("object_name").String()
		object_type := quorum.Get("object_type").String()
		status := quorum.Get("status").String()
		override := quorum.Get("override").String()

		labelvalues := []string{sClient.Hostname, quorum_id, object_name, object_type}
		if len(utils.ExtraLabelValues) > 0 {
			labelvalues = append(labelvalues, utils.ExtraLabelValues...)
		}

		v_status := 0
		switch status {
		case "online":
			v_status = 0
		case "offline":
			v_status = 1
		case "degraded":
			v_status = 2
		}
		ch <- prometheus.MustNewConstMetric(quorum_status, prometheus.GaugeValue, float64(v_status), labelvalues...)

		// override="yes" means this quorum is currently serving as the active tiebreaker
		v_active := 0.0
		if override == "yes" {
			v_active = 1.0
		}
		ch <- prometheus.MustNewConstMetric(quorum_active, prometheus.GaugeValue, v_active, labelvalues...)
		return true
	})

	logger.Debugln("exit quorum collector")
	return nil
}
