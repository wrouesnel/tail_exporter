// Prometheus tail_exporter - heavily heavily modified from the basic
// graphite_exporter by bbrazil.

package main

import (
	"bufio"
	"bytes"
	"flag"
	"io"
	"net"
	"net/http"

	"github.com/hpcloud/tail"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/wrouesnel/tail_exporter/config"
	"os"
	"strings"
	"sync"

	"fmt"
	"github.com/cornelk/hashmap"
	"unsafe"
)

// Namespace is the metric namespace of this collector
const Namespace string = "tail_collector"

var (
	listeningAddress = flag.String("web.listen-address", ":9130", "Address on which to expose metrics.")
	metricsPath      = flag.String("web.telemetry-path", "/metrics", "Path under which to expose Prometheus metrics.")
	collectorAddress = flag.String("collector.listen-address", ":9129", "TCP and UDP address on which to accept lines")
	configFile       = flag.String("config.file", "", "Configuration file path")
)

// TailCollector implements the main collector process.
type TailCollector struct {
	cfg     *config.Config   // Configuration
	metrics *hashmap.HashMap // map of currently stored metrics
	mmtx    *sync.Mutex      // Metric initialization lock for map writes

	regexCh []chan string // list of regex processors

	numMetrics      prometheus.Gauge       // our own metric + lets initialization succeed
	ingestedLines   prometheus.Counter     // number of lines we've ingested
	rejectedLines   *prometheus.CounterVec // number of rejected values
	timedoutMetrics prometheus.Counter     // number of metrics which have been dropped due to internal timeouts
}

func newTailCollector(cfg *config.Config) *TailCollector {
	c := TailCollector{}
	c.cfg = cfg
	c.metrics = hashmap.New()
	c.mmtx = new(sync.Mutex)
	c.regexCh = make([]chan string, len(cfg.MetricConfigs))

	// Initialize regex processors
	for idx, mp := range cfg.MetricConfigs {
		ch := make(chan string, 1)
		c.regexCh[idx] = ch
		go c.lineProcessor(ch, mp)
	}

	// Set constant metrics
	c.numMetrics = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "metric_regexes",
			Help:      "currently configured number of metric regexes",
		},
	)

	c.numMetrics = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: Namespace,
			Name:      "hashmap_size",
			Help:      "size of the internal hashmap for persisting metric values",
		},
	)

	c.ingestedLines = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "ingested_lines_total",
			Help:      "total number of lines ingested by collection inputs",
		},
	)

	c.rejectedLines = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "rejected_lines_total",
			Help:      "total number of lines rejected during parsing",
		},
		[]string{"reason"},
	)

	c.ingestedLines = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: Namespace,
			Name:      "timedout_metrics_total",
			Help:      "total number of times metrics have been dropped due to no updates",
		},
	)

	c.numMetrics.Set(float64(len(cfg.MetricConfigs)))
	return &c
}

// Reads until the current connection is closed
func (c *TailCollector) processReader(reader io.Reader) {
	lineScanner := bufio.NewScanner(reader)
	for {
		if ok := lineScanner.Scan(); !ok {
			break
		}
		c.IngestLine(lineScanner.Text())
	}
}

// IngestLine consumes a line from the file tailing engine
func (c *TailCollector) IngestLine(line string) {
	c.ingestedLines.Inc()
	// Dispatch the line to all active regex parsers
	for _, ch := range c.regexCh {
		ch <- line
	}
}

// Processes lines through the regexes we have loaded
func (c *TailCollector) lineProcessor(lineCh chan string, cfg config.MetricParser) {
	for line := range lineCh {
		m := cfg.Regex.MatcherString(line, 0)
		if !m.Matches() {
			continue
		}

		// Parse the
		labelPairs, lerr := ParseLabelPairsFromMatch(cfg.Labels, m)
		if lerr != nil {
			log.With("line", line).Warnln("Dropping line due to unparseable labels:", lerr)
			c.rejectedLines.WithLabelValues(lerr.Error()).Inc()
			continue
		}

		// Convert the parsed line into the matching metric definition
		metric, merr := newMetricValue(cfg.Name, cfg.Help, cfg.Type, labelPairs...)
		if merr != nil {
			log.With("line", line).Errorln("Dropping line due to metric parsing error:", merr)
			c.rejectedLines.WithLabelValues(merr.Error()).Inc()
		}

		// Get the value from the metric.
		value, verr := ParseValueFromMatch(cfg.Value, m)
		if verr != nil {
			log.With("line", line).Errorln("Dropping line due value parsing error:", verr)
			c.rejectedLines.WithLabelValues(verr.Error()).Inc()
		}

		// Do a lookup in the hashtable to see if we have this metric
		storedMetricPtr, found := c.metrics.GetStringKey(metric.GetHash())
		if !found {
			log.With("hash", metric.GetHash()).Debugln("Initializing new metric")
			metric.Set(value)
			c.metrics.Set(metric.GetHash(), unsafe.Pointer(&metric))
		} else {
			storedMetric := (*metricValue)(storedMetricPtr)
			// Found a stored metric, do the correct operation for the config
			// on its value
			switch cfg.Value.ValueOp {
			case config.ValueOpAdd:
				storedMetric.Add(value)
			case config.ValueOpSubtract:
				storedMetric.Sub(value)
			case config.ValueOpEquals:
				storedMetric.Set(value)
			default:
				// panic because we should *never* get here and testing
				// should catch it.
				panic(fmt.Sprintf("unknown value source specification in config: %v", cfg.Value.ValueOp))
			}
		}
	}
}

// Collect implements prometheus.Collector.
func (c *TailCollector) Collect(ch chan<- prometheus.Metric) {
	c.numMetrics.Collect(ch)
	c.ingestedLines.Collect(ch)
	c.rejectedLines.Collect(ch)

	for kv := range c.metrics.Iter() {
		metric := (*metricValue)(kv.Value)
		metric.Collect(ch)
	}
}

// Describe implements prometheus.Collector.
func (c *TailCollector) Describe(ch chan<- *prometheus.Desc) {
	c.numMetrics.Describe(ch)
	c.ingestedLines.Describe(ch)
	c.rejectedLines.Describe(ch)

	for kv := range c.metrics.Iter() {
		metric := (*metricValue)(kv.Value)
		metric.Describe(ch)
	}
}

func main() {
	flag.Parse()
	http.Handle(*metricsPath, prometheus.Handler())

	cfg, err := config.LoadFile(*configFile)
	if err != nil {
		log.Fatalln("Configuration file could not be read.", err)
	}

	c := newTailCollector(cfg)
	prometheus.MustRegister(c)

	// If args then start file/fifo collectors
	if len(flag.Args()) > 0 {
		for _, filename := range flag.Args() {
			go func(filename string) {
				var isPipe bool
				st, err := os.Stat(filename)
				if err == nil {
					if st.Mode()&os.ModeNamedPipe == os.ModeNamedPipe {
						isPipe = true
					}
				} else {
					isPipe = false
				}

				t, err := tail.TailFile(filename, tail.Config{
					Location: &tail.SeekInfo{0, os.SEEK_END},
					ReOpen:   true,
					Follow:   isPipe,
				})
				if err != nil {
					log.Errorln("Error tailing file:", filename)
				}

				for line := range t.Lines {
					c.IngestLine(line.Text)
				}

			}(filename)
		}
	}

	// If collector address present, then start port collector.
	if *collectorAddress != "" {
		tcpSock, err := net.Listen("tcp", *collectorAddress)
		if err != nil {
			log.Fatalf("Error binding to TCP socket: %s", err)
		}
		go func() {
			for {
				conn, err := tcpSock.Accept()
				if err != nil {
					log.Errorf("Error accepting TCP connection: %s", err)
					continue
				}
				go func() {
					defer conn.Close()
					c.processReader(conn)
				}()
			}
		}()

		udpAddress, err := net.ResolveUDPAddr("udp", *collectorAddress)
		if err != nil {
			log.Fatalf("Error resolving UDP address: %s", err)
		}
		udpSock, err := net.ListenUDP("udp", udpAddress)
		if err != nil {
			log.Fatalf("Error listening to UDP address: %s", err)
		}
		go func() {
			defer udpSock.Close()
			for {
				buf := make([]byte, 65536)
				chars, srcAddress, err := udpSock.ReadFromUDP(buf)
				if err != nil {
					log.Errorf("Error reading UDP packet from %s: %s", srcAddress, err)
					continue
				}
				go c.processReader(bytes.NewReader(buf[0:chars]))
			}
		}()
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
      <head><title>Tail Exporter</title></head>
      <body>
      <h1>TCP/UDP Tail Exporter</h1>
      <p>Accepting raw lines over TCP and UDP on ` + *collectorAddress + `</p>
      <p>Watching files for lines:` + strings.Join(flag.Args(), ", ") + `</p>
      <p><a href="` + *metricsPath + `">Metrics</a></p>
      <h1>Config</h1>
      <pre>` +
			cfg.Original +
			`</pre>
      </body>
      </html>`))
	})

	log.Infof("Starting Server: %s", *listeningAddress)
	http.ListenAndServe(*listeningAddress, nil)
}
