package main

import (
	"flag"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/amimof/huego"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	userCode = flag.String("user", "", "the user to use")
	alert    = flag.Bool("alert", false, "whether to alert all lights")

	bind = flag.String("bind", ":8449", "address to bind to")
	poll = flag.Duration("poll", 1*time.Minute, "poll rate")
)

var counters = map[string]prometheus.Gauge{}

func getCounter(key string) prometheus.Gauge {
	key = "hue:" + key
	counter, ok := counters[key]
	if !ok {
		counter = promauto.NewGauge(prometheus.GaugeOpts{
			Name: key,
		})
		counters[key] = counter
	}
	return counter
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatalf("%+v", err)
	}
}
func run() error {
	bridge, err := huego.Discover()
	if err != nil {
		return err
	}
	user := *userCode
	if len(user) == 0 {
		user, err = bridge.CreateUser("huerice") // Link button needs to be pressed
		if err != nil {
			return err
		}
		log.Printf("created user %q", user)
	}
	bridge = bridge.Login(user)
	lights, err := bridge.GetLights()
	if err != nil {
		return err
	}

	if *alert {
		for _, l := range lights {
			if err := l.Alert("select"); err != nil {
				return err
			}
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	go func() {
		ticker := time.NewTicker(*poll)
		for {
			sensors, err := bridge.GetSensors()
			if err != nil {
				log.Fatalf("%+v", err)
			}
			for _, s := range sensors {
				log.Printf("%s %+v", s.Name, s.State)

				slug := strings.ToLower(strings.Replace(s.Name, " ", "_", -1))
				for key, val := range s.State {
					counter := getCounter(slug + ":" + key)
					switch val := val.(type) {
					case float64:
						counter.Set(val)
					case bool:
						if val {
							counter.Set(1)
						} else {
							counter.Set(0)
						}
					default:
						//log.Printf("unknown %+v %T", val, val)
					}
				}

			}
			<-ticker.C
		}
	}()

	return http.ListenAndServe(*bind, mux)
}
