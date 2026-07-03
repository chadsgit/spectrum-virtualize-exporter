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

const prefix_eventlog = "eventlog_"

var (
	eventlog_unfixed_alerts *prometheus.Desc
)

func init() {
	registerCollector("lseventlog", defaultEnabled, NewEventlogCollector)
}

type eventlogCollector struct{}

func NewEventlogCollector() (Collector, error) {
	labelnames := []string{"resource", "severity"}
	if len(utils.ExtraLabelNames) > 0 {
		labelnames = append(labelnames, utils.ExtraLabelNames...)
	}
	eventlog_unfixed_alerts = prometheus.NewDesc(prefix_eventlog+"unfixed_alert_count",
		"Number of active unfixed alerts on the system, labelled by severity.",
		labelnames, nil)
	return &eventlogCollector{}, nil
}

func (*eventlogCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- eventlog_unfixed_alerts
}

func (c *eventlogCollector) Collect(sClient utils.SpectrumClient, ch chan<- prometheus.Metric) error {
	logger.Debugln("entering eventlog collector ...")
	respData, err := sClient.CallSpectrumAPI("lseventlog", true)
	if err != nil {
		logger.Errorf("executing lseventlog cmd failed: %s", err.Error())
		return err
	}
	logger.Debugln("response of lseventlog: ", respData)
	/* Sample output of lseventlog
	[
	  {
	    "sequence_number": "1",
	    "last_timestamp": "231015120000",
	    "event_id": "1210",
	    "description": "Drive fault",
	    "severity": "error",
	    "alert": "yes",
	    "fixed": "no",
	    "sense_data": ""
	  }
	] */
	if !gjson.Valid(respData) {
		return fmt.Errorf("invalid json for lseventlog:\n%v", respData)
	}

	counts := map[string]float64{
		"error":   0,
		"warning": 0,
		"info":    0,
	}
	gjson.Parse(respData).ForEach(func(key, event gjson.Result) bool {
		// only count active, unfixed, non-expired alerts
		if event.Get("alert").String() != "yes" {
			return true
		}
		if event.Get("fixed").String() == "yes" {
			return true
		}
		if event.Get("expired").String() == "yes" {
			return true
		}
		severity := event.Get("severity").String()
		if _, ok := counts[severity]; ok {
			counts[severity]++
		} else {
			counts["info"]++ // bucket unknown severities as info
		}
		return true
	})

	for severity, count := range counts {
		labelvalues := []string{sClient.Hostname, severity}
		if len(utils.ExtraLabelValues) > 0 {
			labelvalues = append(labelvalues, utils.ExtraLabelValues...)
		}
		ch <- prometheus.MustNewConstMetric(eventlog_unfixed_alerts, prometheus.GaugeValue, count, labelvalues...)
	}

	logger.Debugln("exit eventlog collector")
	return nil
}
