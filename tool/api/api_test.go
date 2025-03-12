package api

import (
	"context"
	"fmt"
	"testing"
)

func TestAPI(t *testing.T) {
	tools, _ := New(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Swagger Petstore
  license:
    name: MIT
servers:
  - url: https://petstore.swagger.io/v1
paths:
  /pets/{petId}:
    get:
      summary: Info for a specific pet
      operationId: showPetById
      tags:
        - pets
      parameters:
        - name: petId
          in: path
          required: true
          description: The id of the pet to retrieve
          schema:
            type: string`)
	result, err := tools[0].Call(context.Background(), "{\"petId\":\"1\"}")
	if err != nil {
		panic(err)
	}
	fmt.Println(result)
}

func TestAPI2(t *testing.T) {
	tools, _ := New(`
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Swagger Petstore
  license:
    name: MIT
servers:
  - url: https://petstore.swagger.io/v1
paths:
  /pets:
    get:
      summary: List all pets
      operationId: listPets
      tags:
        - pets
      parameters:
        - name: limit
          in: query
          description: How many items to return at one time (max 100)
          required: false
          schema:
            type: integer
            maximum: 100
            format: int32
`)
	result, err := tools[0].Call(context.Background(), "{\"limit\":100}")
	if err != nil {
		panic(err)
	}
	fmt.Println(result)
}
