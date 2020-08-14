package utils

import (
	"fmt"
	"strconv"
	"bytes"
	"encoding/json"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
)

func QueryES(url string, query map[string]interface{}, index string, timeField string) ([]interface{}, error) {
	cfg := elasticsearch7.Config {
		Addresses: []string{url},
		//Logger: &estransport.ColorLogger{Output: os.Stdout},
	}
	es, _ := elasticsearch7.NewClient(cfg)

	if timeField == "" {
		timeField = "@timestamp"
	}

	query["size"] = 100

	results := make([]interface{}, 0)
	lastTS := ""
	for {
		if lastTS != "" {
			query["search_after"] = []string{lastTS}
		}

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(query); err != nil {
			fmt.Println("ERROR encoding query", err)
		}
		res, err := es.Search(
			es.Search.WithIndex(index),
			es.Search.WithPretty(),
			es.Search.WithBody(&buf),
			es.Search.WithRestTotalHitsAsInt(true),
		)
		//fmt.Println(q)
		if err != nil {
			fmt.Println("ERROR doing search")
			return results, err
		}
		defer res.Body.Close()

		if res.IsError() {
			return results, err
		}

		var r map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			return results, err
		}

		resSlice := r["hits"].(map[string]interface{})["hits"].([]interface{})
		numResults := len(resSlice)
		if numResults == 0 {
			break
		}
		for _, result := range resSlice {
			results = append(results, result.(map[string]interface{})["_source"])
		}
		lastTS = ParseInterface(resSlice[numResults-1], "_source", timeField).(string)
	}

	return results, nil
}

func GetAuditLogs(start int64, end int64, url string) ([]string, error) {
	q := map[string]interface{} {
		"sort": []map[string]string{map[string]string{"@timestamp": "asc"}},
		"query": map[string]interface{} {
			"range": map[string]interface{} {
				"@timestamp": map[string]interface{} {
					"gte": strconv.FormatInt(start, 10),
					"lte": strconv.FormatInt(end, 10),
					"format": "epoch_second",
				},
			},
		},
	}

	logStrings := make([]string, 0)
	res, err := QueryES(url, q, "filebeat*", "")
	if err != nil {
		return logStrings, err
	}

	for _, r := range res {
		logStrings = append(logStrings, r.(map[string]interface{})["message"].(string))
	}

	return logStrings, nil
}
