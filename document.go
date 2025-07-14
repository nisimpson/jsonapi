package jsonapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"iter"
)

type Document struct {
	Meta   map[string]interface{} `json:"meta,omitempty"`
	Data   PrimaryData            `json:"data,omitempty"`
	Errors []Error                `json:"errors,omitempty"`
	Links  map[string]Link        `json:"links,omitempty"`
}

type PrimaryData struct {
	one  Resource
	many []Resource
	null bool
}

func SingleResource(resource Resource) PrimaryData {
	return PrimaryData{one: resource}
}

func NullResource() PrimaryData {
	return PrimaryData{null: true}
}

func MultiResource(resources ...Resource) PrimaryData {
	return PrimaryData{many: resources}
}

func (p PrimaryData) Null() bool { return p.null }

func (p PrimaryData) One() (Resource, bool) {
	if p.many != nil {
		return Resource{}, false
	}
	return p.one, true
}

func (p PrimaryData) Many() ([]Resource, bool) {
	if p.many == nil || p.null {
		return []Resource{}, false
	}
	return p.many, true
}

func (p PrimaryData) Iter() iter.Seq[Resource] {
	return func(yield func(Resource) bool) {
		if p.Null() {
			return
		}

		var items = []Resource{}
		if one, ok := p.One(); ok {
			items = append(items, one)
		}

		items = p.many
		for _, item := range items {
			if !yield(item) {
				return
			}
		}
	}
}

func (p PrimaryData) MarshalJSON() ([]byte, error) {
	if p.Null() {
		return []byte("null"), nil
	}

	if one, ok := p.One(); ok {
		return json.Marshal(one)
	}

	return json.Marshal(p.many)
}

func (p *PrimaryData) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		p.null = true
		return nil
	}

	if bytes.HasPrefix(data, []byte{'{'}) {
		err := json.Unmarshal(data, &p.one)
		return err
	}

	if bytes.HasPrefix(data, []byte{'['}) {
		p.many = []Resource{}
		err := json.Unmarshal(data, &p.many)
		return err
	}

	return fmt.Errorf("could not determine if data is one, many, or null")
}

type Resource struct {
	ID            string                  `json:"id"`
	Type          string                  `json:"type"`
	Meta          map[string]interface{}  `json:"meta,omitempty"`
	Attributes    map[string]interface{}  `json:"attributes,omitempty"`
	Relationships map[string]Relationship `json:"relationships,omitempty"`
	Links         map[string]Link         `json:"links,omitempty"`
}

func (r Resource) Ref() Resource {
	r.Attributes = nil
	r.Relationships = nil
	r.Links = nil
	return r
}

type Error struct {
	ID     string                 `json:"id,omitempty"`
	Status string                 `json:"status"`
	Code   string                 `json:"code"`
	Title  string                 `json:"title"`
	Detail string                 `json:"detail"`
	Source map[string]interface{} `json:"source,omitempty"`
	Links  map[string]interface{} `json:"links,omitempty"`
}

type Relationship struct {
	Meta  map[string]interface{} `json:"meta,omitempty"`
	Links map[string]Link        `json:"links,omitempty"`
	Data  PrimaryData            `json:"data,omitempty"`
}

func (r Relationship) MarshalJSON() ([]byte, error) {

	type relationship struct {
		Meta  map[string]interface{} `json:"meta,omitempty"`
		Links map[string]Link        `json:"links,omitempty"`
		Data  PrimaryData            `json:"data,omitempty"`
	}

	if one, ok := r.Data.One(); ok {
		r.Data.one = one.Ref()
	}

	if many, ok := r.Data.Many(); ok {
		for i := range many {
			r.Data.many[i] = many[i].Ref()
		}
	}

	return json.Marshal(relationship(r))
}

type Link struct {
	Href string                 `json:"href,omitempty"`
	Meta map[string]interface{} `json:"meta,omitempty"`
}

func (l Link) MarshalJSON() ([]byte, error) {
	if l.Href == "" {
		return []byte("null"), nil
	}

	type link struct {
		Href string                 `json:"href,omitempty"`
		Meta map[string]interface{} `json:"meta,omitempty"`
	}

	return json.Marshal(link(l))
}

func (l *Link) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		l.Href = ""
		return nil
	}

	type link struct {
		Href string                 `json:"href,omitempty"`
		Meta map[string]interface{} `json:"meta,omitempty"`
	}

	if bytes.HasPrefix(data, []byte{'{'}) {
		var tmp link
		err := json.Unmarshal(data, &tmp)
		l.Href = tmp.Href
		l.Meta = tmp.Meta
		return err
	}

	return json.Unmarshal(data, &l.Href)
}
