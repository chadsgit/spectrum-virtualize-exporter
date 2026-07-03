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

const prefix_replication = "replication_"

var (
	replication_relationship_state *prometheus.Desc
)

func init() {
	registerCollector("lsreplicationrelationship", defaultEnabled, NewReplicationCollector)
}

type replicationCollector struct{}

func NewReplicationCollector() (Collector, error) {
	labelnames := []string{"resource", "relationship_name", "policy_name", "primary_volume", "aux_volume"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames = append(labelnames, utils.ExtraLabelNames...)
	}
	replication_relationship_state = prometheus.NewDesc(prefix_replication+"relationship_state",
		"State of the policy-based replication relationship. 0-consistent_synchronized; 1-consistent_copying; 2-inconsistent_copying; 3-inconsistent_disconnected; 4-other.",
		labelnames, nil)
	return &replicationCollector{}, nil
}

func (*replicationCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- replication_relationship_state
}

func (c *replicationCollector) Collect(sClient utils.SpectrumClient, ch chan<- prometheus.Metric) error {
	logger.Debugln("entering replication collector ...")
	respData, err := sClient.CallSpectrumAPI("lsreplicationrelationship", true)
	if err != nil {
		logger.Errorf("executing lsreplicationrelationship cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lsreplicationrelationship: ", respData)
	/* Sample output of lsreplicationrelationship
	[
	  {
	    "id": "0",
	    "name": "reprel0",
	    "policy_id": "0",
	    "policy_name": "policy0",
	    "primary_vdisk_id": "0",
	    "primary_vdisk_name": "vol0",
	    "aux_vdisk_id": "1",
	    "aux_vdisk_name": "vol0_rep",
	    "state": "consistent_synchronized",
	    "progress": "100",
	    "sync": "yes"
	  }
	] */
	if !gjson.Valid(respData) {
		return fmt.Errorf("invalid json for lsreplicationrelationship:\n%v", respData)
	}

	gjson.Parse(respData).ForEach(func(key, rel gjson.Result) bool {
		name := rel.Get("name").String()
		policy_name := rel.Get("policy_name").String()
		primary_vol := rel.Get("primary_vdisk_name").String()
		aux_vol := rel.Get("aux_vdisk_name").String()
		state := rel.Get("state").String()

		labelvalues := []string{sClient.Hostname, name, policy_name, primary_vol, aux_vol}
		if len(utils.ExtraLabelValues) > 0 {
			labelvalues = append(labelvalues, utils.ExtraLabelValues...)
		}

		v_state := 4 // other/unknown
		switch state {
		case "consistent_synchronized":
			v_state = 0
		case "consistent_copying":
			v_state = 1
		case "inconsistent_copying":
			v_state = 2
		case "inconsistent_disconnected":
			v_state = 3
		}
		ch <- prometheus.MustNewConstMetric(replication_relationship_state, prometheus.GaugeValue, float64(v_state), labelvalues...)
		return true
	})

	logger.Debugln("exit replication collector")
	return nil
}
