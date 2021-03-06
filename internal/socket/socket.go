package socket

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rsocket/rsocket-go/internal/transport"
	"github.com/rsocket/rsocket-go/logger"
	"github.com/rsocket/rsocket-go/payload"
	"github.com/rsocket/rsocket-go/rx"
	"github.com/rsocket/rsocket-go/rx/flux"
	"github.com/rsocket/rsocket-go/rx/mono"
)

var (
	errUnsupportedMetadataPush    = errors.New("unsupported METADATA_PUSH")
	errUnsupportedFireAndForget   = errors.New("unsupported FIRE_AND_FORGET")
	errUnsupportedRequestResponse = errors.New("unsupported REQUEST_RESPONSE")
	errUnsupportedRequestStream   = errors.New("unsupported REQUEST_STREAM")
	errUnsupportedRequestChannel  = errors.New("unsupported REQUEST_CHANNEL")
)

// Closeable represents a closeable target.
type Closeable interface {
	io.Closer
	// OnClose bind a handler when closing.
	OnClose(closer func(error))
}

// Responder is a contract providing different interaction models for RSocket protocol.
type Responder interface {
	// FireAndForget is a single one-way message.
	FireAndForget(message payload.Payload)
	// MetadataPush sends asynchronous Metadata frame.
	MetadataPush(message payload.Payload)
	// RequestResponse request single response.
	RequestResponse(message payload.Payload) mono.Mono
	// RequestStream request a completable stream.
	RequestStream(message payload.Payload) flux.Flux
	// RequestChannel request a completable stream in both directions.
	RequestChannel(messages rx.Publisher) flux.Flux
}

// ClientSocket represents a client-side socket.
type ClientSocket interface {
	Closeable
	Responder
	// Setup setups current socket.
	Setup(ctx context.Context, setup *SetupInfo) (err error)
}

// ServerSocket represents a server-side socket.
type ServerSocket interface {
	Closeable
	Responder
	// SetResponder sets a responder for current socket.
	SetResponder(responder Responder)
	// SetTransport sets a transport for current socket.
	SetTransport(tp *transport.Transport)
	// Pause pause current socket.
	Pause() bool
	// Start starts current socket.
	Start(ctx context.Context) error
	// Token returns token of socket.
	Token() (token []byte, ok bool)
}

// AbstractRSocket represents an abstract RSocket.
type AbstractRSocket struct {
	FF func(payload.Payload)
	MP func(payload.Payload)
	RR func(payload.Payload) mono.Mono
	RS func(payload.Payload) flux.Flux
	RC func(rx.Publisher) flux.Flux
}

// MetadataPush starts a request of MetadataPush.
func (p AbstractRSocket) MetadataPush(message payload.Payload) {
	if p.MP == nil {
		logger.Errorf("%s\n", errUnsupportedMetadataPush)
		return
	}
	p.MP(message)
}

// FireAndForget starts a request of FireAndForget.
func (p AbstractRSocket) FireAndForget(message payload.Payload) {
	if p.FF == nil {
		logger.Errorf("%s\n", errUnsupportedFireAndForget)
		return
	}
	p.FF(message)
}

// RequestResponse starts a request of RequestResponse.
func (p AbstractRSocket) RequestResponse(message payload.Payload) mono.Mono {
	if p.RR == nil {
		return mono.Error(errUnsupportedRequestResponse)
	}
	return p.RR(message)
}

// RequestStream starts a request of RequestStream.
func (p AbstractRSocket) RequestStream(message payload.Payload) flux.Flux {
	if p.RS == nil {
		return flux.Error(errUnsupportedRequestStream)
	}
	return p.RS(message)
}

// RequestChannel starts a request of RequestChannel.
func (p AbstractRSocket) RequestChannel(messages rx.Publisher) flux.Flux {
	if p.RC == nil {
		return flux.Error(errUnsupportedRequestChannel)
	}
	return p.RC(messages)
}

type baseSocket struct {
	socket   *DuplexRSocket
	closers  []func(error)
	once     sync.Once
	reqLease *leaser
}

func (p *baseSocket) refreshLease(ttl time.Duration, n int64) {
	deadline := time.Now().Add(ttl)
	if p.reqLease == nil {
		p.reqLease = newLeaser(deadline, n)
	} else {
		p.reqLease.refresh(deadline, n)
	}
}

func (p *baseSocket) FireAndForget(message payload.Payload) {
	if err := p.reqLease.allow(); err != nil {
		logger.Warnf("request FireAndForget failed: %v\n", err)
	}
	p.socket.FireAndForget(message)
}

func (p *baseSocket) MetadataPush(message payload.Payload) {
	p.socket.MetadataPush(message)
}

func (p *baseSocket) RequestResponse(message payload.Payload) mono.Mono {
	if err := p.reqLease.allow(); err != nil {
		return mono.Error(err)
	}
	return p.socket.RequestResponse(message)
}

func (p *baseSocket) RequestStream(message payload.Payload) flux.Flux {
	if err := p.reqLease.allow(); err != nil {
		return flux.Error(err)
	}
	return p.socket.RequestStream(message)
}

func (p *baseSocket) RequestChannel(messages rx.Publisher) flux.Flux {
	if err := p.reqLease.allow(); err != nil {
		return flux.Error(err)
	}
	return p.socket.RequestChannel(messages)
}

func (p *baseSocket) OnClose(fn func(error)) {
	if fn != nil {
		p.closers = append(p.closers, fn)
	}
}

func (p *baseSocket) Close() (err error) {
	p.once.Do(func() {
		err = p.socket.Close()
		for i, l := 0, len(p.closers); i < l; i++ {
			func(fn func(error)) {
				defer func() {
					if e := tryRecover(recover()); e != nil {
						logger.Errorf("handle socket closer failed: %s\n", e)
					}
				}()
				fn(err)
			}(p.closers[l-i-1])
		}
	})
	return
}

func newBaseSocket(rawSocket *DuplexRSocket) *baseSocket {
	return &baseSocket{
		socket: rawSocket,
	}
}
