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

const prefix_rcopy = "rcrelationship_"

var (
	rcopy_status   *prometheus.Desc
	rcopy_state    *prometheus.Desc
	rcopy_progress *prometheus.Desc
)

func init() {
	registerCollector("lsrcrelationship", defaultEnabled, NewRCopyCollector)
}

// rcopyCollector collects remote copy relationship metrics
type rcopyCollector struct{}

func NewRCopyCollector() (Collector, error) {
	// Labels: resource, relationship_name, copy_type (metro/global), primary (master/aux)
	labelnames := []string{"resource", "relationship_name", "copy_type", "primary"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames = append(labelnames, utils.ExtraLabelNames...)
	}
	rcopy_status = prometheus.NewDesc(prefix_rcopy+"status",
		"Status of the remote copy relationship. 0-online; 1-offline.",
		labelnames, nil)
	rcopy_state = prometheus.NewDesc(prefix_rcopy+"state",
		"State of the remote copy relationship. 0-consistent_synchronized; 1-consistent_copying; 2-consistent_disconnected; 3-idling; 4-inconsistent_copying; 5-inconsistent_disconnected; 6-other.",
		labelnames, nil)
	rcopy_progress = prometheus.NewDesc(prefix_rcopy+"progress",
		"Background copy progress percentage (0-100) for the remote copy relationship.",
		labelnames, nil)
	return &rcopyCollector{}, nil
}

// Describe describes the metrics
func (*rcopyCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- rcopy_status
	ch <- rcopy_state
	ch <- rcopy_progress
}

// Collect collects metrics from Spectrum Virtualize Restful API
func (c *rcopyCollector) Collect(sClient utils.SpectrumClient, ch chan<- prometheus.Metric) error {
	logger.Debugln("entering rcopy collector ...")
	respData, err := sClient.CallSpectrumAPI("lsrcrelationship", true)
	if err != nil {
		logger.Errorf("executing lsrcrelationship cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lsrcrelationship: ", respData)
	/* Sample output of lsrcrelationship
	[
	  {
	    "id": "0",
	    "name": "rcrelationship0",
	    "master_cluster_id": "0000020060E02716",
	    "master_cluster_name": "FlashSystem1",
	    "master_vdisk_id": "0",
	    "master_vdisk_name": "vol0",
	    "aux_cluster_id": "0000020060E02717",
	    "aux_cluster_name": "FlashSystem2",
	    "aux_vdisk_id": "0",
	    "aux_vdisk_name": "vol0",
	    "primary": "master",
	    "consistency_group_id": "",
	    "consistency_group_name": "",
	    "state": "consistent_synchronized",
	    "bg_copy_priority": "50",
	    "progress": "0",
	    "status": "online",
	    "sync": "yes",
	    "copy_type": "metro",
	    "cycling_mode": "none"
	  }
	] */
	if !gjson.Valid(respData) {
		return fmt.Errorf("invalid json for lsrcrelationship:\n%v", respData)
	}

	gjson.Parse(respData).ForEach(func(key, rel gjson.Result) bool {
		name := rel.Get("name").String()
		copy_type := rel.Get("copy_type").String()
		primary := rel.Get("primary").String()
		status := rel.Get("status").String()
		state := rel.Get("state").String()
		progress := rel.Get("progress").Float()

		labelvalues := []string{sClient.Hostname, name, copy_type, primary}
		if len(utils.ExtraLabelValues) > 0 {
			labelvalues = append(labelvalues, utils.ExtraLabelValues...)
		}

		v_status := 0
		if status == "offline" {
			v_status = 1
		}
		ch <- prometheus.MustNewConstMetric(rcopy_status, prometheus.GaugeValue, float64(v_status), labelvalues...)

		v_state := 6 // other/unknown
		switch state {
		case "consistent_synchronized":
			v_state = 0
		case "consistent_copying":
			v_state = 1
		case "consistent_disconnected":
			v_state = 2
		case "idling":
			v_state = 3
		case "inconsistent_copying":
			v_state = 4
		case "inconsistent_disconnected":
			v_state = 5
		}
		ch <- prometheus.MustNewConstMetric(rcopy_state, prometheus.GaugeValue, float64(v_state), labelvalues...)

		ch <- prometheus.MustNewConstMetric(rcopy_progress, prometheus.GaugeValue, progress, labelvalues...)
		return true
	})

	logger.Debugln("exit rcopy collector")
	return nil
}
