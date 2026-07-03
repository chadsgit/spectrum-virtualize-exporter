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

const prefix_rcconsistgrp = "rcconsistgrp_"

var (
	rcconsistgrp_state            *prometheus.Desc
	rcconsistgrp_relationship_count *prometheus.Desc
)

func init() {
	registerCollector("lsrcconsistgrp", defaultEnabled, NewRCConsistGrpCollector)
}

type rcConsistGrpCollector struct{}

func NewRCConsistGrpCollector() (Collector, error) {
	// Labels: resource, group_name, copy_type, primary
	labelnames := []string{"resource", "group_name", "copy_type", "primary"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames = append(labelnames, utils.ExtraLabelNames...)
	}
	rcconsistgrp_state = prometheus.NewDesc(prefix_rcconsistgrp+"state",
		"State of the RC consistency group. 0-consistent_synchronized; 1-consistent_copying; 2-consistent_stopped; 3-inconsistent_copying; 4-inconsistent_disconnected; 5-other.",
		labelnames, nil)
	rcconsistgrp_relationship_count = prometheus.NewDesc(prefix_rcconsistgrp+"relationship_count",
		"Number of RC relationships in the consistency group.",
		labelnames, nil)
	return &rcConsistGrpCollector{}, nil
}

func (*rcConsistGrpCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- rcconsistgrp_state
	ch <- rcconsistgrp_relationship_count
}

func (c *rcConsistGrpCollector) Collect(sClient utils.SpectrumClient, ch chan<- prometheus.Metric) error {
	logger.Debugln("entering rcconsistgrp collector ...")
	respData, err := sClient.CallSpectrumAPI("lsrcconsistgrp", true)
	if err != nil {
		logger.Errorf("executing lsrcconsistgrp cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lsrcconsistgrp: ", respData)
	/* Sample output of lsrcconsistgrp
	[
	  {
	    "id": "0",
	    "name": "consistgrp0",
	    "master_cluster_id": "0000020060E02716",
	    "master_cluster_name": "FlashSystem1",
	    "aux_cluster_id": "0000020060E02717",
	    "aux_cluster_name": "FlashSystem2",
	    "primary": "master",
	    "state": "consistent_synchronized",
	    "relationship_count": "3",
	    "copy_type": "metro",
	    "freeze_time": ""
	  }
	] */
	if !gjson.Valid(respData) {
		return fmt.Errorf("invalid json for lsrcconsistgrp:\n%v", respData)
	}

	gjson.Parse(respData).ForEach(func(key, grp gjson.Result) bool {
		name := grp.Get("name").String()
		copy_type := grp.Get("copy_type").String()
		primary := grp.Get("primary").String()
		state := grp.Get("state").String()
		rel_count := grp.Get("relationship_count").Float()

		labelvalues := []string{sClient.Hostname, name, copy_type, primary}
		if len(utils.ExtraLabelValues) > 0 {
			labelvalues = append(labelvalues, utils.ExtraLabelValues...)
		}

		v_state := 5 // other/unknown
		switch state {
		case "consistent_synchronized":
			v_state = 0
		case "consistent_copying":
			v_state = 1
		case "consistent_stopped":
			v_state = 2
		case "inconsistent_copying":
			v_state = 3
		case "inconsistent_disconnected":
			v_state = 4
		}
		ch <- prometheus.MustNewConstMetric(rcconsistgrp_state, prometheus.GaugeValue, float64(v_state), labelvalues...)
		ch <- prometheus.MustNewConstMetric(rcconsistgrp_relationship_count, prometheus.GaugeValue, rel_count, labelvalues...)
		return true
	})

	logger.Debugln("exit rcconsistgrp collector")
	return nil
}
