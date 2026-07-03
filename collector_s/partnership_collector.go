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

const prefix_partnership = "partnership_"

var (
	partnership_status *prometheus.Desc
)

func init() {
	registerCollector("lspartnership", defaultEnabled, NewPartnershipCollector)
}

type partnershipCollector struct{}

func NewPartnershipCollector() (Collector, error) {
	labelnames := []string{"resource", "partner_name", "type", "location"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames = append(labelnames, utils.ExtraLabelNames...)
	}
	partnership_status = prometheus.NewDesc(prefix_partnership+"status",
		"Status of the IP partnership link to a remote system. 0-fully_configured; 1-partially_configured; 2-not_present; 3-other.",
		labelnames, nil)
	return &partnershipCollector{}, nil
}

func (*partnershipCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- partnership_status
}

func (c *partnershipCollector) Collect(sClient utils.SpectrumClient, ch chan<- prometheus.Metric) error {
	logger.Debugln("entering partnership collector ...")
	respData, err := sClient.CallSpectrumAPI("lspartnership", true)
	if err != nil {
		logger.Errorf("executing lspartnership cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lspartnership: ", respData)
	/* Sample output of lspartnership
	[
	  {
	    "id": "000002006081A564",
	    "name": "FlashSystem2",
	    "location": "remote",
	    "partnership_type": "ipv4",
	    "cluster_ip": "10.0.0.2",
	    "cluster_ip_6": "",
	    "status": "fully_configured",
	    "bandwidth": "50",
	    "backgroundcopyrate": "50",
	    "compress": "no",
	    "link_utilization": "0"
	  }
	] */
	if !gjson.Valid(respData) {
		return fmt.Errorf("invalid json for lspartnership:\n%v", respData)
	}

	gjson.Parse(respData).ForEach(func(key, partner gjson.Result) bool {
		name := partner.Get("name").String()
		ptype := partner.Get("partnership_type").String()
		location := partner.Get("location").String()
		status := partner.Get("status").String()

		labelvalues := []string{sClient.Hostname, name, ptype, location}
		if len(utils.ExtraLabelValues) > 0 {
			labelvalues = append(labelvalues, utils.ExtraLabelValues...)
		}

		v_status := 3 // other/unknown
		switch status {
		case "fully_configured":
			v_status = 0
		case "partially_configured":
			v_status = 1
		case "not_present":
			v_status = 2
		}
		ch <- prometheus.MustNewConstMetric(partnership_status, prometheus.GaugeValue, float64(v_status), labelvalues...)
		return true
	})

	logger.Debugln("exit partnership collector")
	return nil
}
