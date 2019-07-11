package es

import (
	"context"
	"github.com/json-iterator/go"
	"github.com/olivere/elastic/v7"
	"gopkg.in/oauth2.v3"
	"gopkg.in/oauth2.v3/models"
	"time"
)

type TokenStore struct {
	client *elastic.Client
	index  string

	gcDisabled bool
	gcInterval time.Duration
	ticker     *time.Ticker
}

type TokenStoreItem struct {
	TokenStoreItemBase
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type TokenStoreItemBase struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Access    string `json:"access"`
	Refresh   string `json:"refresh"`
	Data      []byte `json:"data"`
}

func NewTokenStore(client *elastic.Client, options ...TokenStoreOption) (*TokenStore, error) {
	store := &TokenStore{
		client:     client,
		index:      "oauth2_tokens",
		gcInterval: 10 * time.Minute,
	}

	for _, o := range options {
		o(store)
	}

	if err := store.initIndex(); err != nil {
		return store, err
	}

	if !store.gcDisabled {
		store.ticker = time.NewTicker(store.gcInterval)
		go store.gc()
	}

	return store, nil
}

func (s *TokenStore) Close() error {
	if !s.gcDisabled {
		s.ticker.Stop()
	}
	return nil
}

func (s *TokenStore) gc() {
	for range s.ticker.C {
		s.clean()
	}
}

func (s *TokenStore) clean() {
	if _, err := s.client.DeleteByQuery(s.index).Query(elastic.NewRangeQuery("expires_at").Lte(time.Now())).Do(context.TODO()); err != nil {
		// TODO: err + check resp
	}
	// TODO
	//now := time.Now()
	//err := s.adapter.Exec(fmt.Sprintf("DELETE FROM %s WHERE expires_at <= $1", s.tableName), now)
	//if err != nil {
	//	s.logger.Printf("Error while cleaning out outdated entities: %+v", err)
	//}
}

func (s *TokenStore) initIndex() error {
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
			  "created_at": {
			    "type": "date",
                "format": "yyyy-MM-dd HH:mm:ss"
			  },
			  "expires_at": {
			    "type": "date",
                "format": "yyyy-MM-dd HH:mm:ss"
			  },
			  "code": {
			    "type": "keyword"
			  },
			  "access": {
			    "type": "keyword"
			  },
			  "refresh": {
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

func (s *TokenStore) Create(info oauth2.TokenInfo) error {
	buf, err := jsoniter.Marshal(info)
	if err != nil {
		return err
	}

	item := &TokenStoreItem{
		TokenStoreItemBase: TokenStoreItemBase{
			Data:      buf,
		},
		CreatedAt: time.Now(),
	}

	if code := info.GetCode(); code != "" {
		item.Code = code
		item.ExpiresAt = info.GetCodeCreateAt().Add(info.GetCodeExpiresIn())
	} else {
		item.Access = info.GetAccess()
		item.ExpiresAt = info.GetAccessCreateAt().Add(info.GetAccessExpiresIn())

		if refresh := info.GetRefresh(); refresh != "" {
			item.Refresh = info.GetRefresh()
			item.ExpiresAt = info.GetRefreshCreateAt().Add(info.GetRefreshExpiresIn())
		}
	}

	// TODO: clean up - this whole block accounts for json.unmarshal not handling the time values well

	// first, marshal to bytes
	asJson, err := jsoniter.Marshal(item)
	if err != nil {
		return err
	}

	// then, re-read as a map
	mapVals := make(map[string]string)
	if err := jsoniter.Unmarshal(asJson, &mapVals); err != nil {
		return err
	}

	// overwrite the times as strings
	mapVals["created_at"] = item.CreatedAt.Format("2006-01-02T15:04:05Z07:00") // yyyy-MM-dd HH:mm:ss
	mapVals["expires_at"] = item.ExpiresAt.Format("2006-01-02T15:04:05Z07:00") // yyyy-MM-dd HH:mm:ss

	// save to ES
	_, err = s.client.Index().Index(s.index).BodyJson(mapVals).Refresh("wait_for").Do(context.TODO())
	// TODO: resp
	return err
}

// RemoveByCode deletes the authorization code
func (s *TokenStore) RemoveByCode(code string) error {
	_, err := s.client.DeleteByQuery(s.index).Q("code:\"" + code + "\"").Do(context.TODO())
	if err != nil {
		return err
	}

	// TODO: check result
	return nil
}

// RemoveByAccess uses the access token to delete the token information
func (s *TokenStore) RemoveByAccess(access string) error {
	_, err := s.client.DeleteByQuery(s.index).Q("access:\"" + access + "\"").Do(context.TODO())
	if err != nil {
		return err
	}

	// TODO: check result
	return nil
}

// RemoveByRefresh uses the refresh token to delete the token information
func (s *TokenStore) RemoveByRefresh(refresh string) error {
	_, err := s.client.DeleteByQuery(s.index).Q("refresh:\"" + refresh + "\"").Do(context.TODO())
	if err != nil {
		return err
	}

	// TODO: check result
	return nil
}

func (s *TokenStore) toTokenInfo(data []byte) (oauth2.TokenInfo, error) {
	var tm models.Token
	err := jsoniter.Unmarshal(data, &tm)
	return &tm, err
}

// GetByCode uses the authorization code for token information data
func (s *TokenStore) GetByCode(code string) (oauth2.TokenInfo, error) {
	if code == "" {
		return nil, nil
	}

	var item TokenStoreItemBase

	res, err := s.client.Search(s.index).Query(elastic.NewTermQuery("code", code)).TrackTotalHits(true).Size(1).Do(context.TODO())
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

	return s.toTokenInfo(item.Data)
}

// GetByAccess uses the access token for token information data
func (s *TokenStore) GetByAccess(access string) (oauth2.TokenInfo, error) {
	if access == "" {
		return nil, nil
	}

	var item TokenStoreItem

	res, err := s.client.Search(s.index).Query(elastic.NewTermQuery("access", access)).Size(1).Do(context.TODO())
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

	return s.toTokenInfo(item.Data)
}

// GetByRefresh uses the refresh token for token information data
func (s *TokenStore) GetByRefresh(refresh string) (oauth2.TokenInfo, error) {
	if refresh == "" {
		return nil, nil
	}

	var item TokenStoreItem

	res, err := s.client.Search(s.index).Query(elastic.NewTermQuery("refresh", refresh)).Size(1).Do(context.TODO())
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

	return s.toTokenInfo(item.Data)
}
