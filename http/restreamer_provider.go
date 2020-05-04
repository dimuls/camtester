package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/sirupsen/logrus"
)

type RestreamerProvider struct {
	uri string
}

func NewRestreamerProvider(uri string) *RestreamerProvider {
	return &RestreamerProvider{uri: uri}
}

func (rp *RestreamerProvider) ProvideRestreamer(uri string) (string, error) {
	uriEscaped := url.QueryEscape(uri)

	res, err := http.Get(rp.uri + "/host?uri=" + uriEscaped)
	if err != nil {
		return "", fmt.Errorf("HTTP get host: %w", err)
	}

	defer func() {
		err = res.Body.Close()
		if err != nil {
			logrus.WithField("subsystem", "http_restreamer_provider").
				WithError(err).Error("failed to close HTTP response body")
		}
	}()

	var host string

	err = json.NewDecoder(res.Body).Decode(&host)
	if err != nil {
		return "", fmt.Errorf("JSON decode host: %w", err)
	}

	return host, nil
}
