package transport

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/MunifTanjim/pushport/internal/app"
)

type CredsProvider interface {
	GetCredentials(ctx context.Context, appID string) (app.Credentials, error)
}

type bundle struct {
	apns                        Transport // production APNs client
	apnsSandbox                 Transport // sandbox APNs client
	fcm                         Transport
	webpush                     Transport
	apnsErr, fcmErr, webpushErr error
}

type Dispatcher struct {
	creds  CredsProvider
	client *http.Client
	logger *slog.Logger

	mu    sync.Mutex
	cache map[string]*bundle
}

type SendOptions struct {
	Sandbox bool
}

func (d *Dispatcher) SetLogger(l *slog.Logger) { d.logger = l }

func (d *Dispatcher) logBuildErr(appID, name string, err error) {
	if d.logger != nil {
		d.logger.Warn("transport build failed", "app", appID, "transport", name, "err", err.Error())
	}
}

func NewDispatcher(creds CredsProvider, client *http.Client) *Dispatcher {
	if client == nil {
		client = http.DefaultClient
	}
	return &Dispatcher{creds: creds, client: client, cache: map[string]*bundle{}}
}

func (d *Dispatcher) Invalidate(appID string) {
	d.mu.Lock()
	delete(d.cache, appID)
	d.mu.Unlock()
}

func (d *Dispatcher) bundleFor(ctx context.Context, appID string) (*bundle, error) {
	d.mu.Lock()
	if b, ok := d.cache[appID]; ok {
		d.mu.Unlock()
		return b, nil
	}
	d.mu.Unlock()

	c, err := d.creds.GetCredentials(ctx, appID)
	if err != nil {
		return nil, err
	}
	b := &bundle{}
	if c.APNs != nil {
		tr, err := NewAPNs(*c.APNs, "", false, d.client)
		if err != nil {
			b.apnsErr = err
			d.logBuildErr(appID, "apns", err)
		} else {
			b.apns = tr
			s, err := NewAPNs(*c.APNs, "", true, d.client)
			if err != nil {
				b.apnsErr = err
				d.logBuildErr(appID, "apns", err)
			} else {
				b.apnsSandbox = s
			}
		}
	}
	if c.FCM != nil {
		projectID, err := app.FCMProjectID(c.FCM.ServiceAccountJSON)
		if err != nil {
			b.fcmErr = err
			d.logBuildErr(appID, "fcm", err)
		} else if ts, err := FCMTokenSourceFromJSON(c.FCM.ServiceAccountJSON, d.client); err != nil {
			b.fcmErr = err
			d.logBuildErr(appID, "fcm", err)
		} else {
			b.fcm = NewFCM(projectID, ts, "", d.client)
		}
	}
	if c.WebPush != nil {
		if tr, err := NewWebPush(*c.WebPush, d.client); err == nil {
			b.webpush = tr
		} else {
			b.webpushErr = err
			d.logBuildErr(appID, "webpush", err)
		}
	}
	d.mu.Lock()
	d.cache[appID] = b
	d.mu.Unlock()
	return b, nil
}

func (d *Dispatcher) Send(ctx context.Context, appID, transportName, transportRef string, opts SendOptions, msg Message) (Result, error) {
	b, err := d.bundleFor(ctx, appID)
	if err != nil {
		return Result{}, err
	}
	var tr Transport
	var buildErr error
	switch transportName {
	case "apns":
		tr, buildErr = b.apns, b.apnsErr
		if opts.Sandbox {
			tr = b.apnsSandbox
		}
	case "fcm":
		tr, buildErr = b.fcm, b.fcmErr
	case "webpush":
		tr, buildErr = b.webpush, b.webpushErr
	}
	if tr == nil {
		if buildErr != nil {
			return Result{}, ErrCredentialsInvalid
		}
		return Result{}, ErrNotConfigured
	}
	return tr.Send(ctx, transportRef, msg)
}
