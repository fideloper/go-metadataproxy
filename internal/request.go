package internal

import (
	"fmt"
	"net/http"

	metrics "github.com/armon/go-metrics"
	"github.com/gorilla/mux"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

const (
	telemetryPrefix = "metadataproxy"
)

type Request struct {
	id            string
	log           *logrus.Entry
	metricsLabels []metrics.Label
	loggingLabels logrus.Fields
}

func NewRequest() *Request {
	id := uuid.NewV4()

	return &Request{
		id:            id.String(),
		log:           logrus.WithField("request_id", id.String()),
		metricsLabels: make([]metrics.Label, 0),
		loggingLabels: logrus.Fields{},
	}
}

func (r *Request) setLabel(key, value string) {
	r.setLabels(map[string]string{key: value})
}

func (r *Request) setLabels(pairs map[string]string) {
	for key, value := range pairs {
		r.metricsLabels = append(r.metricsLabels, metrics.Label{Name: key, Value: value})
		r.loggingLabels[key] = value
	}

	r.log = r.log.WithFields(r.loggingLabels)
}

func (r *Request) incrCounterWithLabels(path []string, val float32) {
	path = append([]string{telemetryPrefix}, path...)
	metrics.IncrCounterWithLabels(path, val, r.metricsLabels)
}

func (r *Request) setGaugeWithLabels(path []string, val float32) {
	path = append([]string{telemetryPrefix}, path...)
	metrics.SetGaugeWithLabels(path, val, r.metricsLabels)
}

func (r *Request) setResponseHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Powered-By", "go-metadataproxy")
	w.Header().Set("X-Request-ID", r.id)
}

func (r *Request) setLabelsFromRequest(name, path string, httpRequest *http.Request) {
	labels := make(map[string]string)
	vars := mux.Vars(httpRequest)

	r.setLabel("aws_api_version", vars["api_version"])
	r.setLabel("handler_name", name)
	r.setLabel("request_path", path)

	r.log = r.log.WithField("remote_addr", remoteIP(httpRequest.RemoteAddr))

	if isDataDogEnabled() {
		if span, found := tracer.SpanFromContext(httpRequest.Context()); found {
			r.log = r.log.WithFields(logrus.Fields{
				"dd.trace_id": fmt.Sprintf("%d", span.Context().TraceID()),
				"dd.span_id":  fmt.Sprintf("%d", span.Context().SpanID()),
			})
		}
	}

	if len(copyRequestHeaders) >= 0 {
		for _, label := range copyRequestHeaders {
			if v := httpRequest.Header.Get("label"); v != "" {
				labels[labelName("header", label)] = v
			}
		}
	}

	r.setLabels(labels)
}
