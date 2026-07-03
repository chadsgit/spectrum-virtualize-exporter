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
	"github.com/IBM/spectrum-virtualize-exporter/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
)

const prefix_mdisk = "mdisk_"

var (
	mdiskCapacity *prometheus.Desc
	mdiskStatus   *prometheus.Desc
)

func init() {
	registerCollector("lsmdisk", defaultEnabled, NewMdiskCollector)
}

// mdiskCollector collects mdisk metrics
type mdiskCollector struct {
}

func NewMdiskCollector() (Collector, error) {
	labelnames := []string{"resource", "name", "status", "mdisk_grp_name", "tier"}
	labelnamesStatus := []string{"resource", "name", "mdisk_grp_name", "tier"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames = append(labelnames, utils.ExtraLabelNames...)
		labelnamesStatus = append(labelnamesStatus, utils.ExtraLabelNames...)
	}
	mdiskCapacity = prometheus.NewDesc(prefix_mdisk+"capacity", "The capacity of the MDisk by pool", labelnames, nil)
	mdiskStatus = prometheus.NewDesc(prefix_mdisk+"status", "Status of the MDisk. 0-online; 1-degraded; 2-offline.", labelnamesStatus, nil)

	return &mdiskCollector{}, nil
}

// Describe describes the metrics
func (*mdiskCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- mdiskCapacity
	ch <- mdiskStatus
}

// Collect collects metrics from Spectrum Virtualize Restful API
func (c *mdiskCollector) Collect(sClient utils.SpectrumClient, ch chan<- prometheus.Metric) error {

	logger.Debugln("entering MDisk collector ...")
	mDiskResp, err := sClient.CallSpectrumAPI("lsmdisk", true)
	if err != nil {
		logger.Errorf("executing lsmdisk cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lsmdisk: ", mDiskResp)
	//This is a sample output of lsmdisk
	// 	[
	//     {
	//         "id": "0",
	//         "name": "mdisk0",
	//         "status": "online",
	//         "mode": "array",
	//         "mdisk_grp_id": "0",
	//         "mdisk_grp_name": "Pool0",
	//         "capacity": "99.1TB",
	//         "ctrl_LUN_#": "",
	//         "controller_name": "",
	//         "UID": "",
	//         "tier": "tier0_flash",
	//         "encrypt": "no",
	//         "site_id": "",
	//         "site_name": "",
	//         "distributed": "yes",
	//         "dedupe": "no",
	//         "over_provisioned": "yes",
	//         "supports_unmap": "yes"
	//     }
	// ]
	mDisks := gjson.Parse(mDiskResp).Array()
	for _, mdisk := range mDisks {
		name := mdisk.Get("name").String()
		status := mdisk.Get("status").String()
		mdisk_grp_name := mdisk.Get("mdisk_grp_name").String()
		tier := mdisk.Get("tier").String()

		capacity_bytes, err := utils.ToBytes(mdisk.Get("capacity").String())
		if err != nil {
			logger.Errorf("converting capacity unit failed: %s", err.Error())
		}
		labelvalues := []string{sClient.Hostname, name, status, mdisk_grp_name, tier}
		if len(utils.ExtraLabelValues) > 0 {
			labelvalues = append(labelvalues, utils.ExtraLabelValues...)
		}
		ch <- prometheus.MustNewConstMetric(mdiskCapacity, prometheus.GaugeValue, float64(capacity_bytes), labelvalues...)

		v_status := 0
		switch status {
		case "online":
			v_status = 0
		case "degraded":
			v_status = 1
		case "offline":
			v_status = 2
		}
		labelvaluesStatus := []string{sClient.Hostname, name, mdisk_grp_name, tier}
		if len(utils.ExtraLabelValues) > 0 {
			labelvaluesStatus = append(labelvaluesStatus, utils.ExtraLabelValues...)
		}
		ch <- prometheus.MustNewConstMetric(mdiskStatus, prometheus.GaugeValue, float64(v_status), labelvaluesStatus...)
	}
	logger.Debugln("exit MDisk collector")
	return nil
}
