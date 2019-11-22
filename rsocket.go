package rsocket

import (
	"github.com/rsocket/rsocket-go/internal/socket"
	"github.com/rsocket/rsocket-go/payload"
	"github.com/rsocket/rsocket-go/rx"
	"github.com/rsocket/rsocket-go/rx/flux"
	"github.com/rsocket/rsocket-go/rx/mono"
)

type (
	// ServerAcceptor is alias for server accepter.
	ServerAcceptor = func(setup payload.SetupPayload, sendingSocket CloseableRSocket) (socket RSocket, err error)

	// RSocket is a contract providing different interaction models for RSocket protocol.
	RSocket interface {
		// FireAndForget is a single one-way message.
		FireAndForget(msg payload.Payload)
		// MetadataPush sends asynchronous Metadata frame.
		MetadataPush(msg payload.Payload)
		// RequestResponse request single response.
		RequestResponse(msg payload.Payload) mono.Mono
		// RequestStream request a completable stream.
		RequestStream(msg payload.Payload) flux.Flux
		// RequestChannel request a completable stream in both directions.
		RequestChannel(msgs rx.Publisher) flux.Flux
	}

	// CloseableRSocket is a RSocket which support more events.
	CloseableRSocket interface {
		socket.Closeable
		RSocket
	}

	// OptAbstractSocket is option for abstract socket.
	OptAbstractSocket func(*socket.AbstractRSocket)
)

// NewAbstractSocket returns an abstract implementation of RSocket.
// You can specify the actual implementation of any request.
func NewAbstractSocket(opts ...OptAbstractSocket) RSocket {
	sk := &socket.AbstractRSocket{}
	for _, fn := range opts {
		fn(sk)
	}
	return sk
}

// MetadataPush register request handler for MetadataPush.
func MetadataPush(fn func(msg payload.Payload)) OptAbstractSocket {
	return func(socket *socket.AbstractRSocket) {
		socket.MP = fn
	}
}

// FireAndForget register request handler for FireAndForget.
func FireAndForget(fn func(msg payload.Payload)) OptAbstractSocket {
	return func(opts *socket.AbstractRSocket) {
		opts.FF = fn
	}
}

// RequestResponse register request handler for RequestResponse.
func RequestResponse(fn func(msg payload.Payload) mono.Mono) OptAbstractSocket {
	return func(opts *socket.AbstractRSocket) {
		opts.RR = fn
	}
}

// RequestStream register request handler for RequestStream.
func RequestStream(fn func(msg payload.Payload) flux.Flux) OptAbstractSocket {
	return func(opts *socket.AbstractRSocket) {
		opts.RS = fn
	}
}

// RequestChannel register request handler for RequestChannel.
func RequestChannel(fn func(msgs rx.Publisher) flux.Flux) OptAbstractSocket {
	return func(opts *socket.AbstractRSocket) {
		opts.RC = fn
	}
}
