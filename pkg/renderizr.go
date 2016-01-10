package pkg

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/csv"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/boltdb"
	"github.com/docker/libkv/store/consul"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/libkv/store/zookeeper"
	"github.com/glerchundi/renderizr/pkg/config"
	"github.com/glerchundi/renderizr/pkg/core"
	"github.com/glerchundi/renderizr/pkg/util"
	"github.com/golang/glog"
	"github.com/spf13/pflag"
)

// Register libkv supported stores
func init() {
	consul.Register()
	etcd.Register()
	zookeeper.Register()
	boltdb.Register()
}

func Run(gc *config.GlobalConfig, bc config.BackendConfig) {
	// configure logging.
	logLevel := pflag.Lookup("log-level")
	flag.Set("v", logLevel.Value.String())

	// check if templates are available
	tcs := make([]*config.TemplateConfig, 0)
	if len(gc.Templates) <= 0 {
		glog.Fatalf("Provide at least one template parameters\n")
	}

	// parse and map
	for _, t := range gc.Templates {
		reader := csv.NewReader(bytes.NewBufferString(t))
		reader.Comma = ';'
		record, err := reader.Read()
		if err != nil {
			glog.Fatalf("Unable to read template %s: %v\n", t, err)
		}

		tc, err := getTemplateConfigFromRecord(gc.Prefix, record)
		if err != nil {
			glog.Fatalf("Unable to parse template record %s: %v\n", t, err)
		}

		tcs = append(tcs, tc)
	}

	// dump input parameters, just for debugging purposes
	util.Dump(gc)
	for _, tc := range tcs {
		util.Dump(tc)
	}
	util.Dump(bc)

	// prepend global prefix to template prefix (if provided)
	if gc.Prefix != "" {
		for _, tc := range tcs {
			tc.Prefix = filepath.Join("/", gc.Prefix, tc.Prefix)
		}
	}

	// Exit if watch is requested and not supported by backend
	if gc.Watch && !bc.IsWatchSupported() {
		glog.Fatalf("Watch is not supported for backend %s. Exiting...", bc.Type())
	}

	// Notify which backend is going to use
	glog.Infof("Backend set to %s", bc.Type())

	// Create store client instance
	client, err := getStoreFromBackendConfig(bc)
	if err != nil {
		glog.Fatal(err)
	}

	// loop over templates
	stopChan := make(<-chan struct {} )
	doneChan := make(chan bool)
	errChan := make(chan error, 10)

	var lastErr error = nil
	for _, tc := range tcs {
		template := core.NewTemplate(tc, gc.NoOp, gc.KeepStageFile, true)
		processor := core.NewOnDemandProcessor(template, client)
		if gc.Onetime {
			if err := processor.Run(); err != nil {
				lastErr = err
			}
		} else {
			go func() {
				core.NewIntervalProcessor(gc.ResyncInterval, processor, stopChan, doneChan, errChan).Run()
			}()
			if gc.Watch {
				go func() {
					core.NewWatchProcessor(template, client, stopChan, doneChan, errChan).Run()
				}()
			}
		}
	}

	// exit prematurely if any of onetime templates failed
	if gc.Onetime {
		if lastErr == nil {
			os.Exit(0)
		}
		glog.Errorf("%v", lastErr)
		os.Exit(1)
	}

	// wait for signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case err := <-errChan:
			glog.Error(err)
		case s := <-signalChan:
			glog.Infof("Captured %v. Exiting...", s)
			close(doneChan)
		case <-doneChan:
			os.Exit(0)
		}
	}
}

func getStoreFromBackendConfig(bc config.BackendConfig) (s store.Store, err error) {
	var endpoints []string
	var tlsConfig *store.ClientTLSConfig

	switch bc.Type() {
	case store.CONSUL:
		cbc, _ := bc.(*config.ConsulBackendConfig)
		endpoints = cbc.Endpoints
		tlsConfig = &store.ClientTLSConfig{cbc.CertFile, cbc.KeyFile, cbc.CAFile}
		break
	case store.ETCD:
		ebc, _ := bc.(*config.EtcdBackendConfig)
		endpoints = ebc.Endpoints
		tlsConfig = &store.ClientTLSConfig{ebc.CertFile, ebc.KeyFile, ebc.CAFile}
		break
	case store.ZK:
		zbc, _ := bc.(*config.ZookeeperBackendConfig)
		endpoints = zbc.Endpoints
		break
	}

	var tls *tls.Config = nil
	if tlsConfig != nil {
		tls, err = newTLS(tlsConfig.CertFile, tlsConfig.KeyFile, tlsConfig.CACertFile)
		if err != nil {
			return nil, err
		}
	}

	return libkv.NewStore(
		bc.Type(),
		endpoints,
		&store.Config{
			TLS: tls,
			ConnectionTimeout: 10*time.Second,
		},
	)
}

func newTLS(certFile, keyFile, caCertFile string) (*tls.Config, error) {
	if certFile == "" || keyFile == "" || caCertFile == "" {
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	pemByte, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		return nil, err
	}

	for {
		var block *pem.Block
		block, pemByte = pem.Decode(pemByte)
		if block == nil {
			break
		}

		caCert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certPool.AddCert(caCert)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      certPool,
	}, nil
}

// For example:
// "/etc/nginx.conf.tmpl;/etc/nginx.conf;;0600;/usr/sbin/nginx -t -c {{ .src }};/usr/sbin/nginx -s reload"
// 0: *src       = /etc/nginx.conf.tmpl
// 1: *dst       = /etc/nginx.conf
// 2: owner      = empty - inherits ownership
// 3: perms      = 0600
// 4: check-cmd  = /usr/sbin/nginx -t -c {{ .src }}
// 5: reload-cmd = /usr/sbin/nginx -s reload
func getTemplateConfigFromRecord(prefix string, record []string) (*config.TemplateConfig, error) {
	recordLength := len(record)
	if recordLength < 2 {
		return nil, fmt.Errorf("Template record must have at least two elements (src;dst)")
	}

	tc := config.NewTemplateConfig()
	tc.Src = record[0]
	tc.Dest = record[1]

	if recordLength < 3 {
		return tc, nil
	}

	if record[2] != "" {
		parts := strings.Split(record[2], ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("Owner should be provided as uid:gid")
		}

		uid, err := strconv.ParseInt(parts[0], 10, 0)
		if err != nil {
			return nil, err
		}

		gid, err := strconv.ParseInt(parts[1], 10, 0)
		if err != nil {
			return nil, err
		}

		tc.Uid = int(uid)
		tc.Gid = int(gid)
	}

	if recordLength < 4 {
		return tc, nil
	}

	tc.Mode = record[3]

	if recordLength < 5 {
		return tc, nil
	}

	tc.CheckCmd = record[4]

	if recordLength < 6 {
		return tc, nil
	}

	tc.ReloadCmd = record[5]

	return tc, nil
}