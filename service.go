package main

import (
	"io"
	"net"

	"github.com/google/flatbuffers/go"
	"github.com/mikeraimondi/prefixedio"
)

type serviceMsg interface {
	fromBytes([]byte)
	toFlatBufferBytes(*flatbuffers.Builder) []byte
	new() serviceMsg
	getConn(*server) (net.Conn, error)
}

type service struct {
	builder *flatbuffers.Builder
	buf     prefixedio.Buffer
	host    *server
}

func newService(s *server) *service {
	return &service{
		builder: flatbuffers.NewBuilder(0),
		host:    s,
	}
}

func (svc *service) reset() {
	svc.builder.Reset()
}

func (svc *service) sync(req serviceMsg) (resp []serviceMsg, err error) {
	conn, err := req.getConn(svc.host)
	if err != nil {
		return
	}
	defer conn.Close()

	if _, err = prefixedio.WriteBytes(conn, req.toFlatBufferBytes(svc.builder)); err != nil {
		return
	}
	for {
		_, err = svc.buf.ReadFrom(conn)
		if err == io.EOF {
			err = nil
			break
		} else if err != nil {
			return
		}
		thisResp := req.new()
		thisResp.fromBytes(svc.buf.Bytes())
		resp = append(resp, thisResp)
	}
	return
}
