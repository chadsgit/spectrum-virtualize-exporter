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

const prefix_enclosureslot = "enclosureslot_"

var (
	enclosureslot_fault_led    *prometheus.Desc
	enclosureslot_drive_present *prometheus.Desc
)

func init() {
	registerCollector("lsenclosureslot", defaultEnabled, NewEnclosureSlotCollector)
}

type enclosureSlotCollector struct{}

func NewEnclosureSlotCollector() (Collector, error) {
	labelnames := []string{"resource", "enclosure_id", "slot_id"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames = append(labelnames, utils.ExtraLabelNames...)
	}
	enclosureslot_fault_led = prometheus.NewDesc(prefix_enclosureslot+"fault_led",
		"Fault LED state for the drive slot. 0-off (healthy); 1-on (drive fault); 2-slow_flashing (identify light active).",
		labelnames, nil)
	enclosureslot_drive_present = prometheus.NewDesc(prefix_enclosureslot+"drive_present",
		"Whether a drive is present in this slot. 1-present; 0-empty.",
		labelnames, nil)
	return &enclosureSlotCollector{}, nil
}

func (*enclosureSlotCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- enclosureslot_fault_led
	ch <- enclosureslot_drive_present
}

func (c *enclosureSlotCollector) Collect(sClient utils.SpectrumClient, ch chan<- prometheus.Metric) error {
	logger.Debugln("entering enclosureslot collector ...")
	respData, err := sClient.CallSpectrumAPI("lsenclosureslot", true)
	if err != nil {
		logger.Errorf("executing lsenclosureslot cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lsenclosureslot: ", respData)
	/* Sample output of lsenclosureslot
	[
	  {
	    "enclosure_id": "1",
	    "slot_id": "1",
	    "drive_present": "yes",
	    "drive_id": "0",
	    "fault_LED": "off",
	    "identify_LED": "off",
	    "port_1_status": "online",
	    "port_2_status": "online",
	    "powered": "yes"
	  }
	] */
	if !gjson.Valid(respData) {
		return fmt.Errorf("invalid json for lsenclosureslot:\n%v", respData)
	}

	gjson.Parse(respData).ForEach(func(key, slot gjson.Result) bool {
		enclosure_id := slot.Get("enclosure_id").String()
		slot_id := slot.Get("slot_id").String()
		drive_present := slot.Get("drive_present").String()
		fault_led := slot.Get("fault_LED").String()

		labelvalues := []string{sClient.Hostname, enclosure_id, slot_id}
		if len(utils.ExtraLabelValues) > 0 {
			labelvalues = append(labelvalues, utils.ExtraLabelValues...)
		}

		v_drive_present := 0.0
		if drive_present == "yes" {
			v_drive_present = 1.0
		}
		ch <- prometheus.MustNewConstMetric(enclosureslot_drive_present, prometheus.GaugeValue, v_drive_present, labelvalues...)

		v_fault_led := 0.0
		switch fault_led {
		case "off":
			v_fault_led = 0
		case "on":
			v_fault_led = 1
		case "slow_flashing":
			v_fault_led = 2
		}
		ch <- prometheus.MustNewConstMetric(enclosureslot_fault_led, prometheus.GaugeValue, v_fault_led, labelvalues...)
		return true
	})

	logger.Debugln("exit enclosureslot collector")
	return nil
}
