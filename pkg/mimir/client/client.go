package client

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/grafana/dskit/crypto/tls"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	rulerAPIPath  = "/prometheus/config/v1/rules"
	legacyAPIPath = "/api/v1/rules"
)

var (
	ErrNoConfig         = errors.New("No config exists for this user")
	ErrResourceNotFound = errors.New("requested resource not found")
)

// Config is used to configure a MimirClient.
type Config struct {
	User            string `yaml:"user"`
	Key             string `yaml:"key"`
	Address         string `yaml:"address"`
	ID              string `yaml:"id"`
	TLS             tls.ClientConfig
	UseLegacyRoutes bool   `yaml:"use_legacy_routes"`
	AuthToken       string `yaml:"auth_token"`
}

type Interface interface {
	CreateRuleGroup(ctx context.Context, namespace string, rg RuleGroup) error
	DeleteRuleGroup(ctx context.Context, namespace, groupName string) error
	ListRules(ctx context.Context, namespace string) (map[string][]RuleGroup, error)
}

// MimirClient is a client to the Mimir API.
type MimirClient struct {
	user      string
	key       string
	id        string
	endpoint  *url.URL
	Client    http.Client
	apiPath   string
	authToken string
}

// New returns a new MimirClient.
func New(cfg Config) (*MimirClient, error) {
	endpoint, err := url.Parse(cfg.Address)
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"address": cfg.Address,
		"id":      cfg.ID,
	}).Debugln("New ruler client created")

	client := http.Client{}

	// Setup TLS client
	tlsConfig, err := cfg.TLS.GetTLSConfig()
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"tls-ca":   cfg.TLS.CAPath,
			"tls-cert": cfg.TLS.CertPath,
			"tls-key":  cfg.TLS.KeyPath,
		}).Errorf("error loading tls files")
		return nil, fmt.Errorf("client initialization unsuccessful")
	}

	if tlsConfig != nil {
		transport := &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: tlsConfig,
		}
		client = http.Client{Transport: transport}
	}

	path := rulerAPIPath
	if cfg.UseLegacyRoutes {
		path = legacyAPIPath
	}

	return &MimirClient{
		user:      cfg.User,
		key:       cfg.Key,
		id:        cfg.ID,
		endpoint:  endpoint,
		Client:    client,
		apiPath:   path,
		authToken: cfg.AuthToken,
	}, nil
}

// Query executes a PromQL query against the Mimir cluster.
func (r *MimirClient) Query(ctx context.Context, query string) (*http.Response, error) {
	req := fmt.Sprintf("/prometheus/api/v1/query?query=%s&time=%d", url.QueryEscape(query), time.Now().Unix())

	res, err := r.doRequest(req, "GET", nil)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (r *MimirClient) doRequest(path, method string, payload []byte) (*http.Response, error) {
	req, err := buildRequest(path, method, *r.endpoint, payload)
	if err != nil {
		return nil, err
	}

	if (r.user != "" || r.key != "") && r.authToken != "" {
		err := errors.New("atmost one of basic auth or auth token should be configured")
		log.WithFields(log.Fields{
			"url":    req.URL.String(),
			"method": req.Method,
			"error":  err,
		}).Errorln("error during request to Mimir api")
		return nil, err
	}

	if r.user != "" {
		req.SetBasicAuth(r.user, r.key)
	} else if r.key != "" {
		req.SetBasicAuth(r.id, r.key)
	}

	if r.authToken != "" {
		req.Header.Add("Authorization", "Bearer "+r.authToken)
	}

	req.Header.Add("X-Scope-OrgID", r.id)

	log.WithFields(log.Fields{
		"url":    req.URL.String(),
		"method": req.Method,
	}).Debugln("sending request to Mimir api")

	resp, err := r.Client.Do(req)
	if err != nil {
		log.WithFields(log.Fields{
			"url":    req.URL.String(),
			"method": req.Method,
			"error":  err.Error(),
		}).Errorln("error during request to Mimir api")
		return nil, err
	}

	err = checkResponse(resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// checkResponse checks the API response for errors
func checkResponse(r *http.Response) error {
	log.WithFields(log.Fields{
		"status": r.Status,
	}).Debugln("checking response")
	if 200 <= r.StatusCode && r.StatusCode <= 299 {
		return nil
	}

	var msg, errMsg string
	scanner := bufio.NewScanner(io.LimitReader(r.Body, 512))
	if scanner.Scan() {
		msg = scanner.Text()
	}

	if msg == "" {
		errMsg = fmt.Sprintf("server returned HTTP status %s", r.Status)
	} else {
		errMsg = fmt.Sprintf("server returned HTTP status %s: %s", r.Status, msg)
	}

	if r.StatusCode == http.StatusNotFound {
		log.WithFields(log.Fields{
			"status": r.Status,
			"msg":    msg,
		}).Debugln(errMsg)
		return ErrResourceNotFound
	}

	log.WithFields(log.Fields{
		"status": r.Status,
		"msg":    msg,
	}).Errorln(errMsg)

	return errors.New(errMsg)
}

func joinPath(baseURLPath, targetPath string) string {
	// trim exactly one slash at the end of the base URL, this expects target
	// path to always start with a slash
	return strings.TrimSuffix(baseURLPath, "/") + targetPath
}

func buildRequest(p, m string, endpoint url.URL, payload []byte) (*http.Request, error) {
	// parse path parameter again (as it already contains escaped path information
	pURL, err := url.Parse(p)
	if err != nil {
		return nil, err
	}

	// if path or endpoint contains escaping that requires RawPath to be populated, also join rawPath
	if pURL.RawPath != "" || endpoint.RawPath != "" {
		endpoint.RawPath = joinPath(endpoint.EscapedPath(), pURL.EscapedPath())
	}
	endpoint.Path = joinPath(endpoint.Path, pURL.Path)
	return http.NewRequest(m, endpoint.String(), bytes.NewBuffer(payload))
}
