package api_test

import (
	"testing"

	"github.com/c1r5/open-streaming/src/api"
)

const imdbapi string = "http://api.imdbapi.dev"

func TestIMDBClient_Search(t *testing.T) {
	c := api.CreateIMDB(imdbapi)

	titles, err := c.Search("kingsman")

	if err != nil {
		t.Errorf("Error: %s", err.Error())
	}

	if len(titles) < 1 {
		t.Error("empty_result")
	}
}
