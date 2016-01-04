package main

import (
	"github.com/google/flatbuffers/go"
	"github.com/knollit/http_frontend/endpoints"
)

type endpoint struct {
	ID           string
	Organization string
	URL          string
	action       int8
	err          error
}

func (e *endpoint) toFlatBufferBytes(b *flatbuffers.Builder) []byte {
	b.Reset()

	idPosition := b.CreateByteString([]byte(e.ID))
	orgPosition := b.CreateByteString([]byte(e.Organization))
	urlPosition := b.CreateByteString([]byte(e.URL))

	endpoints.EndpointStart(b)

	endpoints.EndpointAddId(b, idPosition)
	endpoints.EndpointAddOrganization(b, orgPosition)
	endpoints.EndpointAddURL(b, urlPosition)
	endpoints.EndpointAddAction(b, e.action)

	endpointPosition := endpoints.EndpointEnd(b)
	b.Finish(endpointPosition)
	return b.Bytes[b.Head():]
}
