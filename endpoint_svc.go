package main

import (
	"io"
	"net"

	"github.com/google/flatbuffers/go"
	"github.com/knollit/http_frontend/endpoints"
	"github.com/mikeraimondi/prefixedio"
)

type endpointService struct {
	builder  *flatbuffers.Builder
	buf      prefixedio.Buffer
	connFunc func() (net.Conn, error)
}

func newEndpointService(cf func() (net.Conn, error)) *endpointService {
	return &endpointService{
		builder:  flatbuffers.NewBuilder(0),
		connFunc: cf,
	}
}

func (svc *endpointService) reset() {
	svc.builder.Reset()
}

func (svc *endpointService) sync(endpointReq *endpoint) (endpointResponses []*endpoint, err error) {
	conn, err := svc.connFunc()
	if err != nil {
		return
	}
	defer conn.Close()

	if _, err = prefixedio.WriteBytes(conn, endpointReq.toFlatBufferBytes(svc.builder)); err != nil {
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
		endpointResp := &endpoint{}
		endpointResp.fromFlatBufferMsg(endpoints.GetRootAsEndpoint(svc.buf.Bytes(), 0))
		endpointResponses = append(endpointResponses, endpointResp)
	}
	return
}
