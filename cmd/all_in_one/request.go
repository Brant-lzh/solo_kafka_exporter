package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type req struct {
	//Cloud     string   `json:"cloud"`     //云商
	SoloID string `json:"solo_id"` //solo id
	//CmdbID    string   `json:"cmdb_id"`   //有需要可以去CMDB查询
	//CloudID   string   `json:"cloud_id"`  //云商ID
	Addresses []string `json:"addresses"` //地址
	Version   string   `json:"version"`
}

func (r *req) String() string {
	var addresses string
	for _, address := range r.Addresses {
		addresses = addresses + address + ","
	}
	return r.SoloID + ":" + addresses + ":" + r.Version
}

func banding(r *http.Request) (*req, error) {
	values := r.URL.Query()
	var request req
	if version := values.Get("version"); len(version) != 0 {
		request.Version = version
	} else {
		return nil, fmt.Errorf("Request parameter error  %s  ", "version")
	}

	if addresses := values.Get("addresses"); len(addresses) != 0 {
		request.Addresses = make([]string, 0, strings.Count(addresses, ":"))
		for _, address := range strings.Split(addresses, ",") {
			request.Addresses = append(request.Addresses, address)
		}
		//TODO 校验ip格式
		sort.Strings(request.Addresses)
	} else {
		return nil, fmt.Errorf("Request parameter error  %s  ", "addresses")
	}
	if soloID := values.Get("solo_id"); len(soloID) != 0 {
		request.SoloID = soloID
	} else {
		return nil, fmt.Errorf("Request parameter error  %s  ", "solo_id")
	}
	return &request, nil
}
