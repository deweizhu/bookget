package router

import (
	"bookget/app"
	"bookget/site/Europe/berlin"
	"bookget/site/Europe/bluk"
	"bookget/site/Europe/oxacuk"
	"bookget/site/USA/stanford"
)

type OxacUk struct{}

func (p OxacUk) getRouterInit(sUrl []string) (map[string]interface{}, error) {
	for i, s := range sUrl {
		oxacuk.Init(i+1, s)
	}
	return nil, nil
}

type DigitalBerlin struct{}

func (p DigitalBerlin) getRouterInit(sUrl []string) (map[string]interface{}, error) {
	for i, s := range sUrl {
		berlin.Init(i+1, s)
	}
	return nil, nil
}

type BlUk struct{}

func (p BlUk) getRouterInit(sUrl []string) (map[string]interface{}, error) {
	for i, s := range sUrl {
		bluk.Init(i+1, s)
	}
	return nil, nil
}

type Sammlungen struct{}

func (p Sammlungen) getRouterInit(sUrl []string) (map[string]interface{}, error) {
	for i, s := range sUrl {
		var sammlungen = app.Sammlungen{}
		sammlungen.Init(i+1, s)
	}
	return nil, nil
}

type SearchworksStanford struct{}

func (p SearchworksStanford) getRouterInit(sUrl []string) (map[string]interface{}, error) {
	for i, s := range sUrl {
		stanford.Init(i+1, s)
	}
	return nil, nil
}

type FamilySearch struct{}

func (p FamilySearch) getRouterInit(sUrl []string) (map[string]interface{}, error) {
	for i, s := range sUrl {
		var familysearch app.Familysearch
		familysearch.Init(i+1, s)
	}
	return nil, nil
}

type SiEdu struct{}

func (p SiEdu) getRouterInit(sUrl []string) (map[string]interface{}, error) {
	for i, s := range sUrl {
		var siedu app.SiEdu
		siedu.Init(i+1, s)
	}
	return nil, nil
}

type Berkeley struct{}

func (p Berkeley) getRouterInit(sUrl []string) (map[string]interface{}, error) {
	for i, s := range sUrl {
		var berkeley app.Berkeley
		berkeley.Init(i+1, s)
	}
	return nil, nil
}
