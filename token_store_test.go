package es

import (
	"fmt"
	"github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/oauth2.v3/models"
	"os"
	"testing"
	"time"
)

var uri string

func TestMain(m *testing.M) {
	uri = os.Getenv("ELASTICSEARCH_URI")
	if uri == "" {
		fmt.Println("Env variable ELASTICSEARCH_URI is required to run the tests")
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestSimple(t *testing.T) {
	client, err := elastic.NewClient(
		elastic.SetSniff(false),
		elastic.SetURL(os.Getenv("ELASTICSEARCH_URI")),
		elastic.SetBasicAuth(os.Getenv("ELASTICSEARCH_USER"), os.Getenv("ELASTICSEARCH_PASSWORD")),
	)

	if err != nil {
		t.Fatal(err)
	}

	store, err := NewTokenStore(client)
	if err != nil {
		t.Fatal(err)
	}

	tokenCode := models.NewToken()
	code := fmt.Sprintf("code %s", time.Now().String())
	tokenCode.SetCode(code)
	tokenCode.SetCodeCreateAt(time.Now())
	tokenCode.SetCodeExpiresIn(time.Minute)
	require.NoError(t, store.Create(tokenCode))

	token, err := store.GetByCode(code)
	require.NoError(t, err)
	assert.Equal(t, code, token.GetCode())

	require.NoError(t, store.RemoveByCode(code))

	_, err = store.GetByCode(code)
	if err != nil {
		t.Fatal(err)
	}
}