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

const prefix_systeminfo = "system_"

var (
	system_info *prometheus.Desc
)

func init() {
	registerCollector("lssysteminfo", defaultEnabled, NewSystemInfoCollector)
}

// systemInfoCollector emits a single info metric (value=1) combining
// system-level VPD from lssystem and the primary enclosure MTM/serial from lsenclosure.
type systemInfoCollector struct{}

func NewSystemInfoCollector() (Collector, error) {
	labelnames := []string{"resource", "system_name", "product_name", "code_level", "product_mtm", "serial_number"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames = append(labelnames, utils.ExtraLabelNames...)
	}
	system_info = prometheus.NewDesc(
		prefix_systeminfo+"info",
		"System identity and installed code level. All data is in labels; metric value is always 1.",
		labelnames, nil,
	)
	return &systemInfoCollector{}, nil
}

// Describe describes the metrics
func (*systemInfoCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- system_info
}

// Collect collects metrics from Spectrum Virtualize Restful API
func (c *systemInfoCollector) Collect(sClient utils.SpectrumClient, ch chan<- prometheus.Metric) error {
	logger.Debugln("entering systemInfo collector ...")

	// lssystem — product name, code level, friendly system name
	sysData, err := sClient.CallSpectrumAPI("lssystem", true)
	if err != nil {
		logger.Errorf("executing lssystem cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lssystem: ", sysData)
	if !gjson.Valid(sysData) {
		return fmt.Errorf("invalid json for lssystem:\n%v", sysData)
	}
	system_name := gjson.Get(sysData, "name").String()
	product_name := gjson.Get(sysData, "product_name").String()
	code_level := gjson.Get(sysData, "code_level").String()

	// lsenclosure — machine type/model (product_MTM) and serial number from the control enclosure
	encData, err := sClient.CallSpectrumAPI("lsenclosure", true)
	if err != nil {
		logger.Errorf("executing lsenclosure cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lsenclosure: ", encData)
	if !gjson.Valid(encData) {
		return fmt.Errorf("invalid json for lsenclosure:\n%v", encData)
	}
	product_mtm := ""
	serial_number := ""
	gjson.Parse(encData).ForEach(func(_, enc gjson.Result) bool {
		// Prefer the control enclosure; fall back to first if none is typed "control"
		if enc.Get("type").String() == "control" || product_mtm == "" {
			product_mtm = enc.Get("product_MTM").String()
			serial_number = enc.Get("serial_number").String()
		}
		return product_mtm == "" // stop once we have a control enclosure
	})

	labelvalues := []string{sClient.Hostname, system_name, product_name, code_level, product_mtm, serial_number}
	if len(utils.ExtraLabelValues) > 0 {
		labelvalues = append(labelvalues, utils.ExtraLabelValues...)
	}

	ch <- prometheus.MustNewConstMetric(system_info, prometheus.GaugeValue, 1, labelvalues...)

	logger.Debugln("exit systemInfo collector")
	return nil
}
