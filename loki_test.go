package loki_sink_for_zap_test

import (
	"encoding/json"
	loki_sink_for_zap "github.com/trea/loki-sink-for-zap"
	"go.uber.org/zap"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSetupLoki(t *testing.T) {

	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	testServerUrl := strings.Replace(ts.URL, "https://", "", 1)

	var lokiSink zap.Sink

	if err := zap.RegisterSink("loki", func(url *url.URL) (zap.Sink, error) {
		sink, err := loki_sink_for_zap.NewLokiSink(ts.Client())(url)
		lokiSink = sink
		return lokiSink, err
	}); err != nil {
		t.Fatalf("Unable  to register Sink: %+v", err)
	}

	conf := zap.NewDevelopmentConfig()
	conf.InitialFields = map[string]interface{}{
		"Service": "LokiTest",
	}
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
		return loki_sink_for_zap.NewLokiSink(ts.Client())(url)
	}); err != nil {
		t.Fatalf("Unable  to register Sink: %+v", err)
	}

	conf := zap.NewDevelopmentConfig()
	conf.InitialFields = map[string]interface{}{
		"Service": "LokiTest",
	}
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

	t.Logf("TestFakeLoki: %+v", err)

	if err == nil {
		t.Error("Sync shouldn't work")
	}
}

func TestUnsafeInsecureHttpSink(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	lokiUrl := strings.Replace(ts.URL, "http://", "", 1)

	if err := zap.RegisterSink("lokifakeunsafe", func(url *url.URL) (zap.Sink, error) {
		return loki_sink_for_zap.NewLokiSink(ts.Client())(url)
	}); err != nil {
		t.Fatalf("Unable  to register Sink: %+v", err)
	}

	conf := zap.NewDevelopmentConfig()
	conf.InitialFields = map[string]interface{}{
		"Service": "LokiTest",
	}
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

	t.Logf("TestUnsafeInsecureHttpSink: %+v", err)

	if err != nil {
		t.Error("Sync should work")
	}
}