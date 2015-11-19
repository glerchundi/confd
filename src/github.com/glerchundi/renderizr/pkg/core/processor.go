package core

import (
	"time"
	"sync"

	"github.com/docker/libkv/store"
)

type Processor interface {
	Run() error
}

//
// On Demand Processor
//

type OnDemandProcessor struct {
	template *Template
	client   store.Store
}

func NewOnDemandProcessor(template *Template, client store.Store) *OnDemandProcessor {
	return &OnDemandProcessor{
		template: template,
		client:   client,
	}
}

func (p *OnDemandProcessor) Run() error {
	pairs, err := p.client.List(p.template.config.Prefix)
	if err != nil {
		println(err.Error())
		return err
	}

	return p.template.Render(mapKVPairs(pairs))
}

//
// Interval Processor
//

type IntervalProcessor struct {
	interval  time.Duration
	processor Processor

	stopChan  <-chan struct{}
	doneChan  chan bool
	errChan   chan error
}

func NewIntervalProcessor(interval time.Duration, processor Processor,
                          stopChan <-chan struct{}, doneChan chan bool, errChan chan error) *IntervalProcessor {
	return &IntervalProcessor{
		interval, processor,
		stopChan, doneChan, errChan,
	}
}

func (p *IntervalProcessor) Run() error {
	defer close(p.doneChan)
	for {
		if err := p.processor.Run(); err != nil {
			p.errChan <- err
		}

		select {
		case <-p.stopChan:
			break
		case <-time.After(p.interval):
			continue
		}
	}
	return nil
}

//
// Watch Processor
//

type WatchProcessor struct {
	template  *Template
	client    store.Store

	stopChan  <-chan struct{}
	doneChan  chan bool
	errChan   chan error
}

func NewWatchProcessor(template *Template, client store.Store,
                       stopChan <-chan struct{}, doneChan chan bool, errChan chan error) *WatchProcessor {
	return &WatchProcessor{
		template, client,
		stopChan, doneChan, errChan,
	}
}

func (p *WatchProcessor) Run() error {
	defer close(p.doneChan)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			events, err := p.client.WatchTree(p.template.config.Prefix, p.stopChan)
			if err != nil {
				p.errChan <- err
				// Prevent backend errors from consuming all resources.
				time.Sleep(time.Second * 2)
				continue
			}

			for {
				select {
				case pairs := <-events:
					if err := p.template.Render(mapKVPairs(pairs)); err != nil {
						p.errChan <- err
					}
				}
			}
		}
	}()
	wg.Wait()

	return nil
}

func mapKVPairs(pairs []*store.KVPair) map[string]string {
	kvs := make(map[string]string)
	for _, kv := range pairs {
		kvs[kv.Key] = string(kv.Value)
	}
	return kvs
}