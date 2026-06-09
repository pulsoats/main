package live

import (
	"crypto/tls"
	"fmt"
)

type ClientFactory struct {
	tlsConfig *tls.Config
}

func NewClientFactory(tlsConfig *tls.Config) (*ClientFactory, error) {
	if tlsConfig == nil {
		return nil, fmt.Errorf("live grpc client factory: tls config is nil")
	}
	return &ClientFactory{tlsConfig: tlsConfig}, nil
}

func (f *ClientFactory) NewClient(addr string) (*Client, error) {
	return NewClient(addr, f.tlsConfig)
}
