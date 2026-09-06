package api

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MunifTanjim/pushport/internal/db"
	"github.com/MunifTanjim/pushport/internal/instance"
	"github.com/MunifTanjim/pushport/internal/seal"
	"github.com/MunifTanjim/pushport/internal/transport"
)

type Sender interface {
	Send(ctx context.Context, appID, transportName, transportRef string, msg transport.Message) (transport.Result, error)
}

type Admitter interface {
	Admit(ctx context.Context, appID, instanceID, instancePlanID string) (bool, time.Duration, string)
}

type SendHandler struct {
	instances *instance.Service
	seal      *seal.Service
	sender    Sender
	admit     Admitter
	maxBytes  int64
}

func (h *SendHandler) SetAdmitter(a Admitter) { h.admit = a }

func NewSendHandler(instances *instance.Service, sealSvc *seal.Service, sender Sender, maxBytes int64) *SendHandler {
	return &SendHandler{instances: instances, seal: sealSvc, sender: sender, maxBytes: maxBytes}
}

func (h *SendHandler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /push/{token}", h.send)
}

func (h *SendHandler) send(w http.ResponseWriter, r *http.Request) {
	raw, ok := bearer(r)
	if !ok {
		SendError(w, r, ErrorUnauthorized())
		return
	}
	inst, err := h.instances.Authenticate(r.Context(), raw)
	if err != nil {
		SendError(w, r, ErrorUnauthorized())
		return
	}

	tokenApp, payload, err := h.seal.Open(r.Context(), r.PathValue("token"))
	if err != nil {
		switch {
		case errors.Is(err, seal.ErrExpired):
			SendError(w, r, ErrorGone().WithMessage("endpoint expired"))
		case errors.Is(err, seal.ErrMalformed), errors.Is(err, db.ErrNotFound):
			SendError(w, r, ErrorNotFound().WithMessage("unknown endpoint"))
		default:
			SendError(w, r, ErrorInternalServerError().WithCause(err))
		}
		return
	}
	if tokenApp != inst.AppID {
		SendError(w, r, ErrorForbidden())
		return
	}

	if h.admit != nil {
		planID := ""
		if inst.UsagePlanID.Valid {
			planID = inst.UsagePlanID.String
		}
		if ok, retry, _ := h.admit.Admit(r.Context(), tokenApp, inst.ID, planID); !ok {
			e := ErrorTooManyRequests()
			if retry > 0 {
				e.WithRetryAfter(retryAfterSeconds(retry))
			}
			SendError(w, r, e)
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxBytes)
	ciphertext, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			SendError(w, r, ErrorPayloadTooLarge())
		} else {
			SendError(w, r, ErrorInternalServerError().WithCause(err))
		}
		return
	}

	msg := transport.Message{
		Ciphertext: ciphertext,
		Encoding:   r.Header.Get("Content-Encoding"),
		TTL:        atoiDefault(r.Header.Get("TTL"), 0),
		Urgency:    r.Header.Get("Urgency"),
		Topic:      r.Header.Get("Topic"),
	}

	res, err := h.sender.Send(r.Context(), tokenApp, payload.Transport, payload.TransportRef, msg)
	if err != nil {
		switch {
		case errors.Is(err, transport.ErrNotConfigured):
			SendError(w, r, ErrorServiceUnavailable().WithMessage("transport not configured"))
		case errors.Is(err, transport.ErrCredentialsInvalid):
			SendError(w, r, ErrorInternalServerError().WithMessage("credential configuration error"))
		default:
			SendError(w, r, ErrorBadGateway().WithMessage("upstream error").WithCause(err))
		}
		return
	}
	switch {
	case res.Permanent:
		SendError(w, r, ErrorGone().WithMessage("endpoint gone"))
	case res.Delivered:
		SendData(w, r, http.StatusAccepted, nil)
	case res.RetryAfter > 0 || res.StatusCode == http.StatusTooManyRequests || res.StatusCode == http.StatusServiceUnavailable:
		e := ErrorServiceUnavailable().WithMessage("upstream unavailable")
		if res.RetryAfter > 0 {
			e.WithRetryAfter(retryAfterSeconds(res.RetryAfter))
		}
		SendError(w, r, e)
	default:
		SendError(w, r, ErrorBadGateway().WithMessage("upstream rejected"))
	}
}

func bearer(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const p = "Bearer "
	if len(h) <= len(p) || !strings.EqualFold(h[:len(p)], p) {
		return "", false
	}
	return h[len(p):], true
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

func retryAfterSeconds(d time.Duration) string {
	secs := max(int(math.Ceil(d.Seconds())), 1)
	return strconv.Itoa(secs)
}
