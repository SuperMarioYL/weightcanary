// Package notify will deliver change alerts as plain HTTP POSTs to Feishu,
// DingTalk and generic webhook bots.
//
// v0.1 milestone status: m1 ships the snapshot + report + ledger; this
// package is the m2 seam. The card payload schemas for Feishu and DingTalk
// are UNVERIFIED — they need a real bot token and a live send to confirm,
// which is exactly what the m2 milestone does before claiming alert support.
package notify

import (
	"context"
	"errors"
)

// ErrNotImplemented marks the m2 seam: no notifier is wired up in v0.1, and
// the Feishu/DingTalk card payloads are unverified until a live bot send
// confirms them.
var ErrNotImplemented = errors.New("notify: alert delivery is not implemented yet (m2 milestone); the Feishu/DingTalk card payloads are unverified")

// Alert is one deduplicated change worth paging someone about.
type Alert struct {
	ModelID  string
	Kind     string // takedown / restore / divergence (see the diff package)
	Registry string
	Summary  string
}

// Notifier posts alerts to one delivery target.
type Notifier interface {
	Name() string
	Send(ctx context.Context, alert Alert) error
}

// webhook is the generic HTTP-POST target the concrete notifiers share.
type webhook struct {
	name string
	url  string
}

// NewWebhook returns a generic webhook notifier. Send is an m2 stub.
func NewWebhook(name, url string) Notifier {
	return webhook{name: name, url: url}
}

func (w webhook) Name() string { return w.name }

func (w webhook) Send(ctx context.Context, alert Alert) error {
	return ErrNotImplemented
}
