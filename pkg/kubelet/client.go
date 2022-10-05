package kubelet

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/pfnet-research/strict-supplementalgroups-container-runtime/pkg/config"
)

type Client struct {
	kubeletUrl url.URL
	restConfig *rest.Config
	httpClient *http.Client
	logger     zerolog.Logger
}

func NewKubeletClient(
	cfg *config.Config,
) (*Client, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", cfg.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to read kubeconfg %s: %v", cfg.KubeConfig, err)
	}

	var cert tls.Certificate
	if restConfig.TLSClientConfig.CertFile != "" {
		cert, err = tls.LoadX509KeyPair(restConfig.TLSClientConfig.CertFile, restConfig.TLSClientConfig.KeyFile)
	} else {
		cert, err = tls.X509KeyPair(restConfig.TLSClientConfig.CertData, restConfig.TLSClientConfig.KeyData)
	}
	if err != nil {
		return nil, err
	}

	var caCert []byte
	if restConfig.TLSClientConfig.CAFile != "" {
		caCert, err = ioutil.ReadFile(restConfig.TLSClientConfig.CAFile)
		if err != nil {
			return nil, err
		}
	} else {
		caCert = restConfig.TLSClientConfig.CAData
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
		TLSClientConfig: &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: true,
		},
	}

	url, err := url.Parse(cfg.KubeletUrl)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse kubeletUrl: %v", err)
	}

	return &Client{
		kubeletUrl: *url,
		restConfig: restConfig,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   time.Second * 20,
		},
		logger: zlog.With().Str("Kubelet", url.String()).Logger(),
	}, nil
}

func (c *Client) Pod(namespace, name string) (*corev1.Pod, error) {
	url := c.kubeletUrl
	url.Path = path.Join(url.Path, "pods")
	req, _ := http.NewRequest("GET", url.String(), nil)

	c.logger.Trace().Msg("Running 'GET /pods'")
	resp, err := (*c.httpClient).Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to run HTTP request: %v", err)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read HTTP response body: %v", err)
	}
	c.logger.Trace().Bytes("Body", bodyBytes).Msg("Read HTTP response body succeeded")

	var pods corev1.PodList
	err = json.Unmarshal(bodyBytes, &pods)
	if err != nil {
		return nil, fmt.Errorf("Failed to unmarshal HTTP response body to v1.PodList: %v", err)
	}
	c.logger.Trace().Interface("PodList", pods).Msg("Marshal HTTP response body succeeded")

	for _, pod := range pods.Items {
		if pod.Namespace == namespace && pod.Name == name {
			return &pod, nil
		}
	}
	return nil, fmt.Errorf("Pod not found: %s/%s", namespace, name)
}
