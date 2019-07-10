package es

import (
	"context"
	jsoniter "github.com/json-iterator/go"
	"github.com/olivere/elastic/v7"
	"gopkg.in/oauth2.v3"
	"gopkg.in/oauth2.v3/models"
)

type ClientStore struct {
	client *elastic.Client
	index  string
}

// ClientStoreItem data item
type ClientStoreItem struct {
	ID     string `json:"id"`
	Secret string `json:"secret"`
	Domain string `json:"domain"`
	Data   []byte `json:"data"`
}

func NewClientStore(client *elastic.Client, options ...ClientStoreOption) (*ClientStore, error) {
	store := &ClientStore{
		client:     client,
		index:      "oauth2_clients",
	}

	for _, o := range options {
		o(store)
	}

	if err := store.initIndex(); err != nil {
		return store, err
	}

	return store, nil
}


func (s *ClientStore) initIndex() error {
	exists, err := s.client.IndexExists(s.index).Do(context.TODO())
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	_, err = s.client.CreateIndex(s.index).BodyString(`
		{
          "mappings": {
		    "properties": {
			  "id": {
			    "type": "keyword"
			  },
			  "secret": {
			    "type": "keyword"
			  },
			  "domain": {
			    "type": "keyword"
			  },
			  "data": {
			    "type": "keyword"
			  }
		    }
          }
		}
	`).Do(context.TODO())

	return err // TODO: check res
}

func (s *ClientStore) toClientInfo(data []byte) (oauth2.ClientInfo, error) {
	var cm models.Client
	err := jsoniter.Unmarshal(data, &cm)
	return &cm, err
}

// GetByID retrieves and returns client information by id
func (s *ClientStore) GetByID(id string) (oauth2.ClientInfo, error) {
	if id == "" {
		return nil, nil
	}

	var item TokenStoreItem

	res, err := s.client.Search(s.index).Query(elastic.NewQueryStringQuery("id:\"" + id + "\"")).Size(1).Do(context.TODO())
	if err != nil {
		return nil, err
	}

	if res.Hits == nil || len(res.Hits.Hits) == 0 {
		return nil, err
	}

	itemJson, err := res.Hits.Hits[0].Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	if err := jsoniter.Unmarshal(itemJson, &item); err != nil {
		return nil, err
	}

	return s.toClientInfo(item.Data)
}

// Create creates and stores the new client information
func (s *ClientStore) Create(info oauth2.ClientInfo) error {
	data, err := jsoniter.Marshal(info)
	if err != nil {
		return err
	}

	_, err = s.client.Index().Index(s.index).Id(info.GetID()).BodyJson(ClientStoreItem{
		ID: info.GetID(),
		Secret: info.GetSecret(),
		Domain: info.GetDomain(),
		Data: data,
	}).Do(context.TODO())
	// TODO: resp
	return err
}
