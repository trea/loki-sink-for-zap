package loki_sink_for_zap_test

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	loki_sink_for_zap "github.com/trea/loki-sink-for-zap"
	"go.uber.org/zap"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSetupLoki(t *testing.T) {

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	testServerUrl := strings.Replace(ts.URL, "https://", "", 1)

	var lokiSink zap.Sink

	if err := zap.RegisterSink("loki", func(url *url.URL) (zap.Sink, error) {
		sink, err := loki_sink_for_zap.NewLokiSink(ts.Client())(url, map[string]interface{}{
			"Service": "LokiTest",
		})
		lokiSink = sink
		return lokiSink, err
	}); err != nil {
		t.Fatalf("Unable  to register Sink: %+v", err)
	}

	conf := zap.NewDevelopmentConfig()
	conf.Encoding = "json"
	conf.OutputPaths = []string{"loki://" + testServerUrl}
	conf.ErrorOutputPaths = []string{"loki://" + testServerUrl}

	logger, err := conf.Build()
	if err != nil {
		log.Fatal(err)
	}

	zlogger := logger.Sugar()

	zlogger.Warnf("Hello from TestSetupLoki")

	err = zlogger.Sync()

	if err != nil {
		t.Errorf("Sync shouldn't fail: %+v", err)
	}

	if err := lokiSink.Close(); err != nil {
		t.Errorf("Close should work: %+v", err)
	}
}

func TestFakeLokiError(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		data, _ := json.Marshal(map[string]interface{}{
			"errors": []string{
				"This is a test error",
			},
		})
		w.Write(data)
	}))
	defer ts.Close()

	lokiUrl := strings.Replace(ts.URL, "https://", "", 1)

	if err := zap.RegisterSink("lokifake", func(url *url.URL) (zap.Sink, error) {
		return loki_sink_for_zap.NewLokiSink(ts.Client())(url, map[string]interface{}{
			"service": "LokiTest",
		})
	}); err != nil {
		t.Fatalf("Unable  to register Sink: %+v", err)
	}

	conf := zap.NewDevelopmentConfig()
	conf.Encoding = "json"
	conf.OutputPaths = []string{"lokifake://" + lokiUrl}
	conf.ErrorOutputPaths = []string{"lokifake://" + lokiUrl}

	logger, err := conf.Build()
	if err != nil {
		log.Fatal(err)
	}

	zlogger := logger.Sugar()

	zlogger.Warnf("Hello from TestFakeLokiError")

	err = zlogger.Sync()

	if err == nil {
		t.Error("Sync shouldn't work")
	}
}

func TestSyncClearsPreviouslySentLogs(t *testing.T) {
	var requests []string

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Error(err)
		}

		body, err := io.ReadAll(reader)
		if err != nil {
			t.Error(err)
		}

		requests = append(requests, string(body))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	lokiUrl := strings.Replace(ts.URL, "https://", "", 1)

	if err := zap.RegisterSink("lokifakeintegration", func(url *url.URL) (zap.Sink, error) {
		return loki_sink_for_zap.NewLokiSink(ts.Client())(url, map[string]interface{}{
			"service": "LokiTest",
		})
	}); err != nil {
		t.Fatalf("Unable  to register Sink: %+v", err)
	}

	conf := zap.NewDevelopmentConfig()
	conf.Encoding = "json"
	conf.OutputPaths = []string{"lokifakeintegration://" + lokiUrl}
	conf.ErrorOutputPaths = []string{"lokifakeintegration://" + lokiUrl}

	logger, err := conf.Build()
	if err != nil {
		log.Fatal(err)
	}

	zlogger := logger.Sugar()

	zlogger.Warnf("Hello from TestSyncClearsPreviouslySentLogs")

	err = zlogger.Sync()

	if err != nil {
		t.Errorf("Sync should work, got err: %+v", err)
	}

	err = zlogger.Sync()

	if err != nil {
		t.Errorf("Sync should work, got err: %+v", err)
	}

	count := 0
	for _, req := range requests {
		if strings.Contains(req, "TestSyncClearsPreviouslySentLogs") {
			count += 1
		}
	}

	if count != 1 {
		t.Errorf("Unexpected number of matching requests, expected 1, got %d", count)
	}
}

func TestUnsafeInsecureHttpSink(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	lokiUrl := strings.Replace(ts.URL, "http://", "", 1)

	if err := zap.RegisterSink("lokifakeunsafe", func(url *url.URL) (zap.Sink, error) {
		return loki_sink_for_zap.NewLokiSink(ts.Client())(url, map[string]interface{}{
			"service": "LokiTest",
		})
	}); err != nil {
		t.Fatalf("Unable  to register Sink: %+v", err)
	}

	conf := zap.NewDevelopmentConfig()
	conf.Encoding = "json"
	conf.OutputPaths = []string{"lokifakeunsafe://" + lokiUrl + "/?UNSAFE_secure=false"}
	conf.ErrorOutputPaths = []string{"lokifakeunsafe://" + lokiUrl + "/?UNSAFE_secure=false"}

	logger, err := conf.Build()
	if err != nil {
		log.Fatal(err)
	}

	zlogger := logger.Sugar()

	zlogger.Warnf("Hello from TestUnsafeInsecureHttpSink")

	err = zlogger.Sync()

	if err != nil {
		t.Error("Sync should work")
	}
}

func TestIntegration(t *testing.T) {
	lokiAddr := os.Getenv("LOKI_ADDR")

	if lokiAddr == "" {
		t.Skipf("Integration test skipping, no LOKI_ADDR set")
	}

	lokiUrl := strings.Replace(lokiAddr, "http://", "", 1)

	if err := zap.RegisterSink("lokiintegration", func(url *url.URL) (zap.Sink, error) {
		return loki_sink_for_zap.NewLokiSink(http.DefaultClient)(url, map[string]interface{}{
			"service": "LokiTest",
		})
	}); err != nil {
		t.Fatalf("Unable  to register Sink: %+v", err)
	}

	conf := zap.NewDevelopmentConfig()
	conf.Encoding = "json"
	conf.OutputPaths = []string{"lokiintegration://" + lokiUrl + "/?UNSAFE_secure=false"}
	conf.ErrorOutputPaths = []string{"lokiintegration://" + lokiUrl + "/?UNSAFE_secure=false"}

	logger, err := conf.Build()
	if err != nil {
		log.Fatal(err)
	}

	zlogger := logger.Sugar()

	logContent := fmt.Sprintf("Hello from TestIntegration %d", time.Now().Unix())

	zlogger.Warnf(logContent)

	err = zlogger.Sync()

	if err != nil {
		t.Error("Sync should work")
	}

	lokiGetAddr, _ := url.JoinPath(lokiAddr, "/loki/api/v1/query")

	lokiQueryUrl, _ := url.Parse(lokiGetAddr)

	val := lokiQueryUrl.Query()
	val.Add("query", `{service="LokiTest"}`)

	lokiQueryUrl.RawQuery = val.Encode()

	res, err := http.Get(lokiQueryUrl.String())

	if err != nil {
		t.Fatalf("Unable to query Loki: %+v", err)
	}

	bodyBytes, err := io.ReadAll(res.Body)

	if res.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got status %d\nBody:\n\n%s\nErr:%+v", res.StatusCode, bodyBytes, err)
	}

	if !strings.Contains(string(bodyBytes), logContent) {
		t.Fatalf("Expected to find log line '%s' in response from Loki query: %+v", logContent, string(bodyBytes))
	}
}
