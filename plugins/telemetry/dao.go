package telemetry

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/appbaseio/reactivesearch-api/util"
	badger "github.com/dgraph-io/badger/v3"
	log "github.com/sirupsen/logrus"
)

func postTelemetryToACCAPI(record interface{}) error {
	url := util.ACCAPI + "telemetry"
	requestBody, err := json.Marshal(record)
	if err != nil {
		log.Errorln("error while un-marshalling telemetry request body:", err)
		return err
	}
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")

	res, err := util.HTTPClient().Do(req)
	if err != nil {
		log.Errorln("error while recording telemetry:", err)
		return err
	}
	if res.StatusCode != http.StatusOK {
		log.Errorln("error while recording telemetry status code", res.StatusCode)
	}
	defer res.Body.Close()
	return nil
}

func (t *Telemetry) syncTelemetryRecords() {
	records := map[int][]map[string]interface{}{}
	err := t.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		keyCounter := 0
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			keyCounter += 1
			err := item.Value(func(v []byte) error {
				recordCounter := keyCounter/totalEventsPerRequest + 1
				var recordAsMap map[string]interface{}
				err := json.Unmarshal(v, &recordAsMap)
				if err != nil {
					log.Errorln(logTag, ":", err)
				}
				if records[recordCounter] == nil {
					records[recordCounter] = []map[string]interface{}{recordAsMap}
				} else {
					records[recordCounter] = append(records[recordCounter], recordAsMap)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Errorln(logTag, "error connecting badger db", err)
	}
	for _, v := range records {
		err := postTelemetryToACCAPI(v)
		if err != nil {
			log.Println(logTag, "error encountered while reporting telemetry to new relic")
		}
	}
}
