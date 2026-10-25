package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

func safeIP(ip net.IP) bool {
	return ip != nil && !ip.IsUnspecified() && !ip.IsMulticast() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast()
}
func safeDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, e := net.SplitHostPort(address)
	if e != nil {
		return nil, e
	}
	ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
	if e != nil {
		return nil, errors.New("destination lookup failed")
	}
	if len(ips) == 0 {
		return nil, errors.New("destination has no addresses")
	}
	for _, ip := range ips {
		if !safeIP(ip.IP) {
			return nil, errors.New("destination uses a prohibited address")
		}
	}
	var last error
	d := net.Dialer{Timeout: 5 * time.Second}
	for _, ip := range ips {
		c, e := d.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if e == nil {
			return c, nil
		}
		last = e
	}
	return nil, last
}
func sendWebhook(ctx context.Context, w Webhook) (string, string) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(w.TimeoutMS)*time.Millisecond)
	defer cancel()
	var body io.Reader
	if w.Payload != "" {
		body = strings.NewReader(w.Payload)
	}
	req, e := http.NewRequestWithContext(ctx, w.Method, w.URL, body)
	if e != nil {
		return "webhook_error", "Invalid webhook request"
	}
	for k, v := range w.Headers {
		req.Header.Set(k, v)
	}
	if w.Payload != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "RF-Button-Manager/"+Version)
	// A fresh, non-reused transport also prevents net/http's transparent retry of
	// idempotent requests on stale pooled connections. Each trigger is one attempt.
	transport := &http.Transport{DialContext: safeDial, DisableKeepAlives: true, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: time.Duration(w.TimeoutMS) * time.Millisecond, MaxResponseHeaderBytes: 16384, Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, e := client.Do(req)
	if e != nil {
		return "webhook_unknown", "Request failed or timed out; action outcome unknown. Not retried."
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "webhook_ok", fmt.Sprintf("Webhook accepted (HTTP %d)", resp.StatusCode)
	}
	return "webhook_error", fmt.Sprintf("Webhook returned HTTP %d; not retried", resp.StatusCode)
}
func (a *App) dispatch(id string, epoch uint64, test bool) {
	a.actionGate.RLock()
	defer a.actionGate.RUnlock()
	a.mu.Lock()
	a.expireLocked()
	b := findButton(&a.config, id)
	if a.closed || a.learning != nil || a.captureSuppressed || epoch != a.epoch || b == nil || a.storageErr || (!test && !b.Enabled) {
		a.mu.Unlock()
		return
	}
	if a.demo {
		a.eventLocked("demo", "Demo: webhook execution suppressed", id, nil)
		a.mu.Unlock()
		return
	}
	w := clone(b.Webhook)
	if e := validateWebhook(w, a.config.Settings); e != nil {
		a.eventLocked("webhook_error", "Destination validation failed", id, nil)
		a.mu.Unlock()
		return
	}
	if w.URL == "" {
		a.mu.Unlock()
		return
	}
	if !a.eventLocked("webhook_start", "Sending one webhook request", id, nil) {
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	kind, message := sendWebhook(a.ctx, w)
	a.mu.Lock()
	if !a.closed {
		a.eventLocked(kind, message, id, nil)
	}
	a.mu.Unlock()
}

// Bounded concurrency, one pending request per button, and no stale action queue.
func (a *App) scheduleLocked(id string, epoch uint64, test bool) error {
	if a.inFlight[id] {
		return errors.New("This button already has an action in progress; press was not queued")
	}
	select {
	case a.actionSlots <- struct{}{}:
	default:
		return errors.New("Eight actions are already in progress; press was not queued")
	}
	a.inFlight[id] = true
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer func() { a.mu.Lock(); delete(a.inFlight, id); a.mu.Unlock(); <-a.actionSlots }()
		a.dispatch(id, epoch, test)
	}()
	return nil
}
