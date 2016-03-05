package main

import (
	"github.com/google/flatbuffers/go"
	"github.com/knollit/http_frontend/endpoints"
)

const notFoundErrMsg = "not found"

type endpoint struct {
	ID             string
	OrganizationID string
	URL            string
	Schema         string
	Action         int8 `json:"-"`
	err            error
}

func (e *endpoint) fromFlatBufferMsg(msg *endpoints.Endpoint) {
	e.URL = string(msg.URL())
	e.ID = string(msg.Id())
	e.OrganizationID = string(msg.OrganizationID())
}

func (e *endpoint) toFlatBufferBytes(b *flatbuffers.Builder) []byte {
	b.Reset()

	idPosition := b.CreateByteString([]byte(e.ID))
	orgPosition := b.CreateByteString([]byte(e.OrganizationID))
	urlPosition := b.CreateByteString([]byte(e.URL))
	schemaPosition := b.CreateByteString([]byte(e.Schema))
	var errPosition flatbuffers.UOffsetT
	if e.err != nil {
		errPosition = b.CreateByteString([]byte(e.err.Error()))
	}

	endpoints.EndpointStart(b)

	endpoints.EndpointAddId(b, idPosition)
	endpoints.EndpointAddOrganizationID(b, orgPosition)
	endpoints.EndpointAddURL(b, urlPosition)
	endpoints.EndpointAddSchema(b, schemaPosition)
	if e.err != nil {
		endpoints.EndpointAddError(b, errPosition)
	}
	endpoints.EndpointAddAction(b, e.Action)

	endpointPosition := endpoints.EndpointEnd(b)
	b.Finish(endpointPosition)

	return b.FinishedBytes()
}
