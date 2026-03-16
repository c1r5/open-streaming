package imdb

import (
	"testing"
)

const imdbapi string = "http://api.imdbapi.dev"

func TestIMDBClient_Search(t *testing.T) {
	c := CreateIMDB(imdbapi)

	titles, err := c.Search("kingsman")

	if err != nil {
		t.Errorf("Error: %s", err.Error())
	}

	if len(titles) < 1 {
		t.Error("empty_result")
	}
}
