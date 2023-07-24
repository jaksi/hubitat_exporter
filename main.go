package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func newDesc(name, help string) *prometheus.Desc {
	return prometheus.NewDesc(
		"hubitat_"+name,
		help,
		[]string{"label", "model", "manufacturer", "room"}, nil,
	)
}

var (
	listenAddress      = flag.String("listen-address", ":1123", "Address to listen on.")
	hubitatAddress     = flag.String("hubitat-address", "", "Address of the Hubitat hub.")
	hubitatAccessToken = flag.String("hubitat-access-token", "", "Access token for the Hubitat hub.")

	promDesc = map[string]*prometheus.Desc{
		"temperature": newDesc("temperature_celsius", "Temperature in degrees Celsius."),
		"humidity":    newDesc("humidity_percent", "Relative humidity in percent."),
		"pressure":    newDesc("pressure_hpa", "Atmospheric pressure in hectopascals."),
		"battery":     newDesc("battery_percent", "Battery level in percent."),
	}
)

type device struct {
	Label        string            `json:"label"`
	Type         string            `json:"type"`
	Model        string            `json:"model"`
	Manufacturer string            `json:"manufacturer"`
	Room         string            `json:"room"`
	Attributes   map[string]string `json:"attributes"`
}

type hubitatCollector struct {
	address     string
	accessToken string
}

func (c *hubitatCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

func (c *hubitatCollector) Collect(ch chan<- prometheus.Metric) {
	rsp, err := http.Get(c.address + "/apps/api/4/devices/all?access_token=" + c.accessToken)
	if err != nil {
		log.Printf("Error getting devices: %v", err)
		return
	}
	defer rsp.Body.Close()
	var devices []device
	if err := json.NewDecoder(rsp.Body).Decode(&devices); err != nil {
		log.Printf("Error decoding devices: %v", err)
		return
	}
	for _, device := range devices {
		for attr, value := range device.Attributes {
			if desc, ok := promDesc[attr]; ok {
				floatValue, err := strconv.ParseFloat(value, 64)
				if err != nil {
					log.Printf("Error parsing value %q for attribute %q: %v", value, attr, err)
					continue
				}
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, floatValue, device.Label, device.Model, device.Manufacturer, device.Room)
			}
		}
	}
}

func main() {
	flag.Parse()
	if *hubitatAddress == "" {
		log.Fatal("Hubitat address must be specified.")
	}
	if *hubitatAccessToken == "" {
		log.Fatal("Hubitat access token must be specified.")
	}

	collector := &hubitatCollector{*hubitatAddress, *hubitatAccessToken}
	prometheus.WrapRegistererWith(prometheus.Labels{"hubitat_address": *hubitatAddress}, prometheus.DefaultRegisterer).MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		log.Fatalf("Error listening: %v", err)
	}
}
