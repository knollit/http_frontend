package main

import (
	"github.com/google/flatbuffers/go"
	"github.com/knollit/http_frontend/organizations"
)

type organization struct {
	Name   string `json:"name"`
	action int8
	err    string
}

func organizationFromFlatBuffer(org *organizations.Organization) organization {
	return organization{
		Name: string(org.Name()),
		err:  string(org.Error()),
	}
}

func (org *organization) toFlatBufferBytes(b *flatbuffers.Builder) []byte {
	b.Reset()

	namePosition := b.CreateByteString([]byte(org.Name))

	organizations.OrganizationStart(b)

	organizations.OrganizationAddName(b, namePosition)
	organizations.OrganizationAddAction(b, org.action)

	orgPosition := organizations.OrganizationEnd(b)
	b.Finish(orgPosition)
	return b.Bytes[b.Head():]
}
