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

package collector

import (
	"fmt"

	"github.com/IBM/spectrum-virtualize-exporter/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
)

const prefix_vdiskstats = "vdisk_"

var (
	vdiskstats_r_io *prometheus.Desc
	vdiskstats_w_io *prometheus.Desc
	vdiskstats_r_mb *prometheus.Desc
	vdiskstats_w_mb *prometheus.Desc
	vdiskstats_r_ms *prometheus.Desc
	vdiskstats_w_ms *prometheus.Desc
)

func init() {
	registerCollector("lsvdiskstats", defaultEnabled, NewVdiskStatsCollector)
}

type vdiskStatsCollector struct{}

func NewVdiskStatsCollector() (Collector, error) {
	labelnames := []string{"resource", "vdisk_name"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames = append(labelnames, utils.ExtraLabelNames...)
	}
	vdiskstats_r_io = prometheus.NewDesc(prefix_vdiskstats+"r_io", "Average read I/O operations per second for this volume during the sample period.", labelnames, nil)
	vdiskstats_w_io = prometheus.NewDesc(prefix_vdiskstats+"w_io", "Average write I/O operations per second for this volume during the sample period.", labelnames, nil)
	vdiskstats_r_mb = prometheus.NewDesc(prefix_vdiskstats+"r_mb", "Average read throughput in MB/s for this volume during the sample period.", labelnames, nil)
	vdiskstats_w_mb = prometheus.NewDesc(prefix_vdiskstats+"w_mb", "Average write throughput in MB/s for this volume during the sample period.", labelnames, nil)
	vdiskstats_r_ms = prometheus.NewDesc(prefix_vdiskstats+"r_ms", "Average read response time in milliseconds for this volume during the sample period.", labelnames, nil)
	vdiskstats_w_ms = prometheus.NewDesc(prefix_vdiskstats+"w_ms", "Average write response time in milliseconds for this volume during the sample period.", labelnames, nil)
	return &vdiskStatsCollector{}, nil
}

func (*vdiskStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- vdiskstats_r_io
	ch <- vdiskstats_w_io
	ch <- vdiskstats_r_mb
	ch <- vdiskstats_w_mb
	ch <- vdiskstats_r_ms
	ch <- vdiskstats_w_ms
}

func (c *vdiskStatsCollector) Collect(sClient utils.SpectrumClient, ch chan<- prometheus.Metric) error {
	logger.Debugln("entering vdiskstats collector ...")
	respData, err := sClient.CallSpectrumAPI("lsvdiskstats", true)
	if err != nil {
		logger.Errorf("executing lsvdiskstats cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lsvdiskstats: ", respData)
	/* Sample output of lsvdiskstats
	[
	  {
	    "id": "0",
	    "vdisk_name": "vol0",
	    "r_io": "1200",
	    "w_io": "800",
	    "r_mb": "120",
	    "w_mb": "80",
	    "r_ms": "2",
	    "w_ms": "3",
	    "r_io_a": "1150",
	    "w_io_a": "750",
	    "r_mb_a": "115",
	    "w_mb_a": "75",
	    "r_ms_a": "2",
	    "w_ms_a": "3"
	  }
	] */
	if !gjson.Valid(respData) {
		return fmt.Errorf("invalid json for lsvdiskstats:\n%v", respData)
	}

	gjson.Parse(respData).ForEach(func(key, vdisk gjson.Result) bool {
		name := vdisk.Get("vdisk_name").String()

		labelvalues := []string{sClient.Hostname, name}
		if len(utils.ExtraLabelValues) > 0 {
			labelvalues = append(labelvalues, utils.ExtraLabelValues...)
		}

		ch <- prometheus.MustNewConstMetric(vdiskstats_r_io, prometheus.GaugeValue, vdisk.Get("r_io").Float(), labelvalues...)
		ch <- prometheus.MustNewConstMetric(vdiskstats_w_io, prometheus.GaugeValue, vdisk.Get("w_io").Float(), labelvalues...)
		ch <- prometheus.MustNewConstMetric(vdiskstats_r_mb, prometheus.GaugeValue, vdisk.Get("r_mb").Float(), labelvalues...)
		ch <- prometheus.MustNewConstMetric(vdiskstats_w_mb, prometheus.GaugeValue, vdisk.Get("w_mb").Float(), labelvalues...)
		ch <- prometheus.MustNewConstMetric(vdiskstats_r_ms, prometheus.GaugeValue, vdisk.Get("r_ms").Float(), labelvalues...)
		ch <- prometheus.MustNewConstMetric(vdiskstats_w_ms, prometheus.GaugeValue, vdisk.Get("w_ms").Float(), labelvalues...)
		return true
	})

	logger.Debugln("exit vdiskstats collector")
	return nil
}
