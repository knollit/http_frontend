package main

import (
	"io"
	"net"

	"github.com/google/flatbuffers/go"
	"github.com/knollit/http_frontend/organizations"
	"github.com/mikeraimondi/prefixedio"
)

type organizationService struct {
	builder  *flatbuffers.Builder
	buf      prefixedio.Buffer
	connFunc func() (net.Conn, error)
}

func newOrganizationService(cf func() (net.Conn, error)) *organizationService {
	return &organizationService{
		builder:  flatbuffers.NewBuilder(0),
		connFunc: cf,
	}
}

func (svc *organizationService) reset() {
	svc.builder.Reset()
}

func (svc *organizationService) sync(org *organization) (orgs []*organization, err error) {
	conn, err := svc.connFunc()
	if err != nil {
		return
	}
	defer conn.Close()

	if _, err = prefixedio.WriteBytes(conn, org.toFlatBufferBytes(svc.builder)); err != nil {
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
		orgResp := &organization{}
		orgResp.fromFlatBufferMsg(organizations.GetRootAsOrganization(svc.buf.Bytes(), 0))
		orgs = append(orgs, orgResp)
	}
	return
}
