package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func newDesc(name, help string) *prometheus.Desc {
	return prometheus.NewDesc(
		"hubitat_"+name,
		help,
		[]string{"name", "label", "model", "manufacturer", "room"}, nil,
	)
}

const (
	listenAddressFlag      = "listen-address"
	listenAddressEnv       = "LISTEN_ADDRESS"
	hubitatAddressFlag     = "hubitat-address"
	hubitatAddressEnv      = "HUBITAT_ADDRESS"
	hubitatAccessTokenFlag = "hubitat-access-token"
	hubitatAccessTokenEnv  = "HUBITAT_ACCESS_TOKEN"
)

var (
	listenAddress      = flag.String(listenAddressFlag, "", "Address to listen on. Can also be specified via the "+listenAddressEnv+" environment variable.")
	hubitatAddress     = flag.String(hubitatAddressFlag, "", "Address of the Hubitat hub. Can also be specified via the "+hubitatAddressEnv+" environment variable.")
	hubitatAccessToken = flag.String(hubitatAccessTokenFlag, "", "Access token for the Hubitat hub. Can also be specified via the "+hubitatAccessTokenEnv+" environment variable.")

	promDesc = map[string]*prometheus.Desc{
		"temperature": newDesc("temperature_celsius", "Temperature in degrees Celsius."),
		"humidity":    newDesc("humidity_percent", "Relative humidity in percent."),
		"pressure":    newDesc("pressure_hpa", "Atmospheric pressure in hectopascals."),
		"battery":     newDesc("battery_percent", "Battery level in percent."),
	}
)

type device struct {
	Name         string            `json:"name"`
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
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, floatValue, device.Name, device.Label, device.Model, device.Manufacturer, device.Room)
			}
		}
	}
}

func main() {
	flag.Parse()
	if *listenAddress == "" {
		if *listenAddress = os.Getenv(listenAddressEnv); *listenAddress == "" {
			log.Fatal("Listen address must be specified via the -" + listenAddressFlag + " flag or the " + listenAddressEnv + " environment variable.")
		}
	}
	if *hubitatAddress == "" {
		if *hubitatAddress = os.Getenv(hubitatAddressEnv); *hubitatAddress == "" {
			log.Fatal("Hubitat address must be specified via the -" + hubitatAddressFlag + " flag or the " + hubitatAddressEnv + " environment variable.")
		}
	}
	if *hubitatAccessToken == "" {
		if *hubitatAccessToken = os.Getenv(hubitatAccessTokenEnv); *hubitatAccessToken == "" {
			log.Fatal("Hubitat access token must be specified via the -" + hubitatAccessTokenFlag + " flag or the " + hubitatAccessTokenEnv + " environment variable.")
		}
	}

	collector := &hubitatCollector{*hubitatAddress, *hubitatAccessToken}
	prometheus.WrapRegistererWith(prometheus.Labels{"hubitat_address": *hubitatAddress}, prometheus.DefaultRegisterer).MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Listening on %s", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		log.Fatalf("Error listening: %v", err)
	}
}
