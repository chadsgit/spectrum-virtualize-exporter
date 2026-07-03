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

const prefix_fcmap = "fcmap_"

var (
	fcmap_status   *prometheus.Desc
	fcmap_progress *prometheus.Desc
)

func init() {
	registerCollector("lsfcmap", defaultEnabled, NewFCMapCollector)
}

type fcMapCollector struct{}

func NewFCMapCollector() (Collector, error) {
	labelnames := []string{"resource", "map_name", "source_volume", "target_volume"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames = append(labelnames, utils.ExtraLabelNames...)
	}
	fcmap_status = prometheus.NewDesc(prefix_fcmap+"status",
		"Status of the FlashCopy map. 0-idle_or_copied; 1-copying; 2-stopping; 3-preparing; 4-prepared; 5-suspended; 6-other.",
		labelnames, nil)
	fcmap_progress = prometheus.NewDesc(prefix_fcmap+"progress",
		"FlashCopy map copy progress as a percentage (0-100).",
		labelnames, nil)
	return &fcMapCollector{}, nil
}

func (*fcMapCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- fcmap_status
	ch <- fcmap_progress
}

func (c *fcMapCollector) Collect(sClient utils.SpectrumClient, ch chan<- prometheus.Metric) error {
	logger.Debugln("entering fcmap collector ...")
	respData, err := sClient.CallSpectrumAPI("lsfcmap", true)
	if err != nil {
		logger.Errorf("executing lsfcmap cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lsfcmap: ", respData)
	/* Sample output of lsfcmap
	[
	  {
	    "id": "0",
	    "name": "fcmap0",
	    "source_vdisk_id": "0",
	    "source_vdisk_name": "vol0",
	    "target_vdisk_id": "1",
	    "target_vdisk_name": "vol0_snap",
	    "group_id": "",
	    "group_name": "",
	    "status": "idle_or_copied",
	    "progress": "0",
	    "copy_rate": "50",
	    "start_time": "",
	    "dependent_mappings": "0",
	    "autodelete": "off",
	    "clean_progress": "100",
	    "clean_rate": "50",
	    "incremental": "off",
	    "difference": "256",
	    "grain_size": "256",
	    "IO_group_id": "0",
	    "IO_group_name": "io_grp0",
	    "write_snapshots": "no",
	    "restoring": "no",
	    "rc_controlled": "no",
	    "keep_target": "no"
	  }
	] */
	if !gjson.Valid(respData) {
		return fmt.Errorf("invalid json for lsfcmap:\n%v", respData)
	}

	gjson.Parse(respData).ForEach(func(key, fcmap gjson.Result) bool {
		name := fcmap.Get("name").String()
		source := fcmap.Get("source_vdisk_name").String()
		target := fcmap.Get("target_vdisk_name").String()
		status := fcmap.Get("status").String()
		progress := fcmap.Get("progress").Float()

		labelvalues := []string{sClient.Hostname, name, source, target}
		if len(utils.ExtraLabelValues) > 0 {
			labelvalues = append(labelvalues, utils.ExtraLabelValues...)
		}

		v_status := 6 // other/unknown
		switch status {
		case "idle_or_copied":
			v_status = 0
		case "copying":
			v_status = 1
		case "stopping":
			v_status = 2
		case "preparing":
			v_status = 3
		case "prepared":
			v_status = 4
		case "suspended":
			v_status = 5
		}
		ch <- prometheus.MustNewConstMetric(fcmap_status, prometheus.GaugeValue, float64(v_status), labelvalues...)
		ch <- prometheus.MustNewConstMetric(fcmap_progress, prometheus.GaugeValue, progress, labelvalues...)
		return true
	})

	logger.Debugln("exit fcmap collector")
	return nil
}
