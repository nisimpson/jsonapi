package jsonapi_test

import (
	"testing"
	"time"

	"github.com/nisimpson/jsonapi"
	"github.com/stretchr/testify/assert"
)

type Product struct {
	ID    string `jsonapi:"primary,product"`
	Name  string `jsonapi:"attr,name"`
	Price int    `jsonapi:"attr,price"`
}

type Order struct {
	ID       string    `jsonapi:"primary,order"`
	Amount   int       `jsonapi:"attr,amount"`
	Customer *Customer `jsonapi:"relation,customer"`
	Products []Product `jsonapi:"relation,products,omitempty"`
}

type Customer struct {
	ID      string  `jsonapi:"primary,customer"`
	Name    string  `jsonapi:"attr,name"`
	Address Address `jsonapi:"attr,address,omitempty"`
	Orders  []Order `jsonapi:"relation,orders"`
	Tags    []string
}

type Address struct {
	Street string `json:"street"`
	City   string `json:"city"`
	Zip    string `json:"zip"`
}

type Timestamp struct {
	CreatedAt time.Time `jsonapi:"attr,created_at"`
	UpdatedAt time.Time `jsonapi:"attr,updated_at"`
}

type User struct {
	Timestamp
	ID   string `jsonapi:"primary,user"`
	Name string `jsonapi:"attr,name"`
}

func TestMarshalResource(t *testing.T) {
	t.Run("marshal customer with no orders", func(t *testing.T) {
		customer := &Customer{
			ID:   "1",
			Name: "John Doe",
			Address: Address{
				Street: "123 Main St",
				City:   "Anytown",
				Zip:    "12345",
			},
			Tags: []string{"tag1"},
		}

		res, err := jsonapi.MarshalResource(customer)
		assert.NoError(t, err)

		assert.EqualValues(t, jsonapi.Resource{
			Type: "customers",
			ID:   "1",
			Attributes: map[string]interface{}{
				"name": "John Doe",
				"address": map[string]interface{}{
					"street": "123 Main St",
					"city":   "Anytown",
					"zip":    "12345",
				},
			},
			Relationships: map[string]jsonapi.Relationship{
				"orders": {
					Data: jsonapi.MultiResource(),
				},
			},
		}, res)
	})

	t.Run("marshal customer with orders", func(t *testing.T) {
		customer := &Customer{
			ID:   "1",
			Name: "John Doe",
			Orders: []Order{
				{
					ID:     "1",
					Amount: 100,
				},
				{
					ID:     "2",
					Amount: 200,
				},
			},
		}

		res, err := jsonapi.MarshalResource(customer)
		assert.NoError(t, err)

		assert.EqualValues(t, jsonapi.Resource{
			Type: "customers",
			ID:   "1",
			Attributes: map[string]interface{}{
				"name": "John Doe",
			},
			Relationships: map[string]jsonapi.Relationship{
				"orders": {
					Data: jsonapi.MultiResource(
						jsonapi.Resource{Type: "orders", ID: "1"},
						jsonapi.Resource{Type: "orders", ID: "2"},
					),
				},
			},
		}, res)
	})

	t.Run("marshal order", func(t *testing.T) {
		order := &Order{
			ID:     "1",
			Amount: 100,
		}

		res, err := jsonapi.MarshalResource(order)
		assert.NoError(t, err)

		assert.EqualValues(t, jsonapi.Resource{
			Type: "orders",
			ID:   "1",
			Attributes: map[string]interface{}{
				"amount": 100,
			},
			Relationships: map[string]jsonapi.Relationship{
				"customer": {
					Data: jsonapi.NullResource(),
				},
			},
		}, res)
	})

	t.Run("marshal with embedded struct", func(t *testing.T) {
		user := User{ID: "1", Name: "Nathan"}
		user.CreatedAt = time.Now()
		user.UpdatedAt = time.Now()

		out, err := jsonapi.MarshalResource(user)
		assert.NoError(t, err)
		assert.EqualValues(t, jsonapi.Resource{
			Type: "users",
			ID:   "1",
			Attributes: map[string]interface{}{
				"name":       "Nathan",
				"created_at": user.CreatedAt,
				"updated_at": user.UpdatedAt,
			},
			Relationships: map[string]jsonapi.Relationship{},
		}, out)
	})
}
