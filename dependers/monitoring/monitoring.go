package monitoring

import (
	"crypto/sha1"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
)

type (
	IMonitoring interface {
		Counter(*Metric, float64) error
		Inc(*Metric) error
		ExecutionTime(*Metric, func() error) (float64, error)
		ObserveExecutionTime(metric *Metric, executionTime time.Duration) error
		Gauge(*Metric, float64) error
		IncGauge(*Metric) error
		DecGauge(*Metric) error
	}

	Metric struct {
		Namespace   string
		Subsystem   string
		Name        string
		ConstLabels prometheus.Labels
	}

	PrometheusMonitoring struct {
		collectors map[string]prometheus.Collector

		lock sync.Mutex

		pushURL      string
		username     string
		password     string
		jobName      string
		instanceName string
		serviceName  string
	}

	Config struct {
		PushURL      string `yaml:"pushURL"`
		Username     string `yaml:"username"`
		Password     string `yaml:"password"`
		JobName      string `yaml:"jobName"`
		InstanceName string `yaml:"instanceName"`
	}

	collectorType int16
)

const (
	counter collectorType = iota
	histogram
	gauge
)

func NewPrometheusMonitoring(config *Config) IMonitoring {
	monitoring := &PrometheusMonitoring{
		collectors:   map[string]prometheus.Collector{},
		pushURL:      config.PushURL,
		username:     config.Username,
		password:     config.Password,
		jobName:      config.JobName,
		instanceName: config.InstanceName,
	}

	go monitoring.push()

	return monitoring
}

func (m *PrometheusMonitoring) push() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("monitoring pusher crushed:", err, "restarting...")

			time.Sleep(time.Second)

			go m.push()
		}
	}()

	ticker := time.NewTicker(time.Second * 5)

	for range ticker.C {
		pushJob := push.
			New(m.pushURL, m.jobName).
			Gatherer(prometheus.DefaultGatherer).
			Grouping("instance", m.instanceName).
			BasicAuth(m.username, m.password)
		if err := pushJob.Add(); err != nil {
			log.Println("error push metrics", m.pushURL, err)
		}
	}
}

func (m *PrometheusMonitoring) Gauge(metric *Metric, val float64) error {
	collector, err := m.collector(gauge, metric)
	if err != nil {
		return err
	}

	g, ok := collector.(prometheus.Gauge)
	if !ok {
		return fmt.Errorf("incorrect collector type. Required prometheus.Gauge, got %T", collector)
	}

	g.Set(val)

	return nil
}

func (m *PrometheusMonitoring) IncGauge(metric *Metric) error {
	collector, err := m.collector(gauge, metric)
	if err != nil {
		return err
	}

	g, ok := collector.(prometheus.Gauge)
	if !ok {
		return fmt.Errorf("incorrect collector type. Required prometheus.Gauge, got %T", collector)
	}

	g.Inc()

	return nil
}

func (m *PrometheusMonitoring) DecGauge(metric *Metric) error {
	collector, err := m.collector(gauge, metric)
	if err != nil {
		return err
	}

	g, ok := collector.(prometheus.Gauge)
	if !ok {
		return fmt.Errorf("incorrect collector type. Required prometheus.Gauge, got %T", collector)
	}

	g.Dec()

	return nil
}

func (m *PrometheusMonitoring) Counter(metric *Metric, count float64) error {
	collector, err := m.collector(counter, metric)
	if err != nil {
		return err
	}

	counter, ok := collector.(prometheus.Counter)
	if !ok {
		return fmt.Errorf("incorrect collector type. Required prometheus.Counter, got %T", collector)
	}

	counter.Add(count)

	return nil
}

func (m *PrometheusMonitoring) Inc(metric *Metric) (err error) {
	return m.Counter(metric, 1)
}

func (m *PrometheusMonitoring) ExecutionTime(metric *Metric, h func() error) (float64, error) {
	collector, err := m.collector(histogram, metric)
	if err != nil {
		return 0, h()
	}

	histogram, ok := collector.(prometheus.Histogram)
	if !ok {
		return 0, h()
	}

	err = h()

	executionTime := time.Since(time.Now()).Seconds()

	histogram.Observe(executionTime)

	return executionTime, nil
}

func (m *PrometheusMonitoring) ObserveExecutionTime(metric *Metric, executionTime time.Duration) error {
	collector, err := m.collector(histogram, metric)
	if err != nil {
		return err
	}

	histogram, ok := collector.(prometheus.Histogram)
	if !ok {
		return nil
	}

	histogram.Observe(executionTime.Seconds())

	return nil
}

func (m *PrometheusMonitoring) Handler() http.Handler {
	return promhttp.Handler()
}

func (m *PrometheusMonitoring) collector(t collectorType, metric *Metric) (prometheus.Collector, error) {
	m.lock.Lock()

	defer m.lock.Unlock()

	name := metric.String()

	if _, ok := m.collectors[name]; !ok {
		switch t {
		case counter:
			m.collectors[name] = prometheus.NewCounter(prometheus.CounterOpts{
				Namespace:   metric.Namespace,
				Subsystem:   metric.Subsystem,
				Name:        metric.Name,
				ConstLabels: metric.ConstLabels,
				Help:        "counter " + metric.Name,
			})
		case gauge:
			m.collectors[name] = prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace:   metric.Namespace,
				Subsystem:   metric.Subsystem,
				Name:        metric.Name,
				ConstLabels: metric.ConstLabels,
				Help:        "gauge " + metric.Name,
			})
		case histogram:
			m.collectors[name] = prometheus.NewHistogram(prometheus.HistogramOpts{
				Namespace:   metric.Namespace,
				Subsystem:   metric.Subsystem,
				Name:        metric.Name,
				ConstLabels: metric.ConstLabels,
				Help:        "histogram " + metric.Name,
			})
		}

		if err := prometheus.Register(m.collectors[name]); err != nil {
			return nil, err
		}
	}

	return m.collectors[name], nil
}

func (m Metric) String() string {
	keys := make([]string, len(m.ConstLabels))

	i := 0
	for k := range m.ConstLabels {
		keys[i] = k
		i++
	}

	sort.Strings(keys)

	h := sha1.New()
	h.Write([]byte(m.Namespace + m.Subsystem + m.Name))

	for _, k := range keys {
		h.Write([]byte(k + m.ConstLabels[k]))
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}
