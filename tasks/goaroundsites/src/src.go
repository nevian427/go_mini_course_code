package src

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	// You can limit concurrent net request. It's optional
	MaxGoroutines = 3
	// timeout for net requests
	Timeout = 2 * time.Second
)

type SiteStatus struct {
	Name          string
	StatusCode    int
	TimeOfRequest time.Time
}

type Monitor struct {
	StatusMap        map[string]SiteStatus
	Mtx              *sync.Mutex
	G                errgroup.Group
	Sites            []string
	RequestFrequency time.Duration
}

func NewMonitor(sites []string, requestFrequency time.Duration) *Monitor {
	return &Monitor{
		StatusMap:        make(map[string]SiteStatus),
		Mtx:              &sync.Mutex{},
		Sites:            sites,
		RequestFrequency: requestFrequency,
	}
}

func (m *Monitor) Run(ctx context.Context) error {
	// Run printStatuses(ctx) and checkSite(ctx) for m.Sites
	// Renew sites requests to map every m.RequestFrequency
	// Return if context closed
	m.G.SetLimit(MaxGoroutines)

	siteInProgress := make([]int32, len(m.Sites))

	stTimer := time.NewTicker(time.Second)
	defer stTimer.Stop()
	chkTimer := time.NewTicker(m.RequestFrequency)
	defer chkTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := m.G.Wait(); err != nil {
				fmt.Printf("WG err: %s", err)
			}
			return ctx.Err()
		case <-chkTimer.C:
			for i, u := range m.Sites {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if atomic.CompareAndSwapInt32(&siteInProgress[i], 0, 1) {
					u := u
					i := i
					m.G.Go(func() error {
						m.checkSite(ctx, u)
						atomic.StoreInt32(&siteInProgress[i], 0)
						return nil
					})
				}
			}
		case <-stTimer.C:
			m.G.Go(func() error {
				m.printStatuses(ctx)
				return nil
			})
		}
	}
}

func (m *Monitor) checkSite(ctx context.Context, site string) {
	// with http client go through site and write result to m.StatusMap
	var statusCode int
	reqCtx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, site, nil)
	if err != nil {
		fmt.Printf("error creating request [%s]: %s\n", site, err)
		statusCode = 500
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("error exec request [%s]: %s\n", site, err)
		statusCode = 500
	} else {
		defer func() {
			_ = resp.Body.Close()
		}()
		statusCode = resp.StatusCode
	}

	m.Mtx.Lock()
	m.StatusMap[site] = SiteStatus{
		Name:          site,
		StatusCode:    statusCode,
		TimeOfRequest: time.Now(),
	}
	m.Mtx.Unlock()
}

func (m *Monitor) printStatuses(ctx context.Context) error {
	// print results of m.Status every second of until ctx cancelled

	fmt.Printf("%s Current status\n", time.Now())
	m.Mtx.Lock()
	for _, v := range m.StatusMap {
		fmt.Printf("site %s, code: %d,  RT: %v\n", v.Name, v.StatusCode, v.TimeOfRequest)
	}
	m.Mtx.Unlock()

	return nil
}
