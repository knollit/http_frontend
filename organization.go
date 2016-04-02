package main

import (
	"errors"

	"github.com/google/flatbuffers/go"
	"github.com/knollit/http_frontend/organizations"
)

type organization struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	action int8
	err    error
}

func (org *organization) fromFlatBufferMsg(msg *organizations.Organization) {
	org.Name = string(msg.Name())
	org.ID = string(msg.ID())
	if len(msg.Error()) > 0 {
		org.err = errors.New(string(msg.Error()))
	}
}

func (org *organization) toFlatBufferBytes(b *flatbuffers.Builder) []byte {
	b.Reset()

	idPosition := b.CreateByteString([]byte(org.ID))
	namePosition := b.CreateByteString([]byte(org.Name))

	organizations.OrganizationStart(b)

	organizations.OrganizationAddID(b, idPosition)
	organizations.OrganizationAddName(b, namePosition)
	organizations.OrganizationAddAction(b, org.action)

	orgPosition := organizations.OrganizationEnd(b)
	b.Finish(orgPosition)
	return b.Bytes[b.Head():]
}
