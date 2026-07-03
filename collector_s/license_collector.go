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

const prefix_license = "license_"

var (
	license_feature *prometheus.Desc
)

func init() {
	registerCollector("lslicense", defaultEnabled, NewLicenseCollector)
}

type licenseCollector struct{}

func NewLicenseCollector() (Collector, error) {
	labelnames := []string{"resource", "feature"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames = append(labelnames, utils.ExtraLabelNames...)
	}
	license_feature = prometheus.NewDesc(prefix_license+"feature_enabled",
		"Whether a Storage Virtualize licensed feature is active. 1-licensed; 0-not licensed.",
		labelnames, nil)
	return &licenseCollector{}, nil
}

func (*licenseCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- license_feature
}

func (c *licenseCollector) Collect(sClient utils.SpectrumClient, ch chan<- prometheus.Metric) error {
	logger.Debugln("entering license collector ...")
	respData, err := sClient.CallSpectrumAPI("lslicense", true)
	if err != nil {
		logger.Errorf("executing lslicense cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lslicense: ", respData)
	/* Sample output of lslicense (key-value pairs, not an array)
	{
	  "used_flash": "0",
	  "used_remote": "0",
	  "used_virtualization": "0",
	  "license_flash": "0",
	  "license_remote": "0",
	  "license_virtualization": "0",
	  "license_compression_capacity": "0",
	  "license_compression_enclosures": "0",
	  "license_encryption": "no",
	  "license_easy_tier": "no",
	  "license_replication": "no"
	} */
	if !gjson.Valid(respData) {
		return fmt.Errorf("invalid json for lslicense:\n%v", respData)
	}

	parsed := gjson.Parse(respData)

	// emit a labeled gauge per boolean license flag
	boolFeatures := []string{
		"license_encryption",
		"license_easy_tier",
		"license_replication",
	}
	for _, feature := range boolFeatures {
		val := parsed.Get(feature).String()
		v := 0.0
		if val == "yes" {
			v = 1.0
		}
		labelvalues := []string{sClient.Hostname, feature}
		if len(utils.ExtraLabelValues) > 0 {
			labelvalues = append(labelvalues, utils.ExtraLabelValues...)
		}
		ch <- prometheus.MustNewConstMetric(license_feature, prometheus.GaugeValue, v, labelvalues...)
	}

	// numeric capacity licenses: licensed > 0 means feature is enabled
	numericFeatures := []string{
		"license_flash",
		"license_remote",
		"license_virtualization",
		"license_compression_capacity",
		"license_compression_enclosures",
	}
	for _, feature := range numericFeatures {
		val := parsed.Get(feature).Float()
		v := 0.0
		if val > 0 {
			v = 1.0
		}
		labelvalues := []string{sClient.Hostname, feature}
		if len(utils.ExtraLabelValues) > 0 {
			labelvalues = append(labelvalues, utils.ExtraLabelValues...)
		}
		ch <- prometheus.MustNewConstMetric(license_feature, prometheus.GaugeValue, v, labelvalues...)
	}

	logger.Debugln("exit license collector")
	return nil
}
