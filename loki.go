package loki_sink_for_zap

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// invalidResponse captures the HTTP Response back from any attempt to Sync() to the provided LOKI URL
type invalidResponse struct {
	code int
	body string
}

func (r invalidResponse) Error() string {
	return fmt.Sprintf("pushToLoki: Expected HTTP 204 No Content, got %d %s with body: \n%s", r.code, http.StatusText(r.code), r.body)
}

func NewLokiSink(client *http.Client) func(u *url.URL) (zap.Sink, error) {
	return func(u *url.URL) (zap.Sink, error) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if u.Query().Get("UNSAFE_secure") == "false" {
			u.Scheme = "http"
		} else {
			u.Scheme = "https"
		}
		u.Path = ""
		u.RawQuery = ""

		if client == nil {
			client = &http.Client{}
		}

		return &LokiWriteSyncer{
			ctx,
			cancel,
			client,
			u.String(),
			LokiTags{},
			[]lokiValue{},
		}, nil
	}
}

type LokiTags map[string]interface{}

type lokiPush struct {
	Streams []lokiStream `json:"streams"`
}

type lokiValue []string

func newLokiValue(line []byte) lokiValue {
	return []string{strconv.FormatInt(time.Now().UnixNano(), 10), string(line)}
}

type lokiStream struct {
	Stream map[string]interface{} `json:"stream"`
	Values []lokiValue            `json:"values"`
}

type LokiWriteSyncer struct {
	ctx    context.Context
	close  context.CancelFunc
	client *http.Client
	url    string
	tags   LokiTags
	values []lokiValue
}

func (l LokiWriteSyncer) prepareForLokiPush() (io.Reader, error) {
	b := bytes.NewBuffer([]byte{})

	gz := gzip.NewWriter(b)

	if err := json.NewEncoder(gz).Encode(lokiPush{
		Streams: []lokiStream{
			{Stream: l.tags,
				Values: l.values,
			},
		},
	}); err != nil {
		return b, err
	}

	return b, gz.Close()
}

func (l LokiWriteSyncer) pushToLoki() (err error) {
	reader, err := l.prepareForLokiPush()

	if err != nil {
		return err
	}

	request, reqErr := http.NewRequest(http.MethodPost, l.url+"/loki/api/v1/push", reader)

	if reqErr != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Content-Encoding", "gzip")
	request.WithContext(l.ctx)

	res, postErr := l.client.Do(request)

	if postErr != nil {
		return postErr
	}

	if res.StatusCode != 204 {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}

		return invalidResponse{
			code: res.StatusCode,
			body: string(body),
		}
	}

	return nil
}

func (l *LokiWriteSyncer) Write(p []byte) (n int, err error) {
	l.values = append(l.values, newLokiValue(p))
	return len(p), nil
}

func (l LokiWriteSyncer) Sync() error {
	if err := l.pushToLoki(); err != nil {
		return err
	}

	return nil
}

func (l LokiWriteSyncer) Close() error {
	l.close()
	return nil
}
