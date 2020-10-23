package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	//	"reflect"
	"net"
	"net/url"
	"strings"
	"time"
)

func checkStatusCode(res *http.Response) {

	switch {
	case 300 <= res.StatusCode && res.StatusCode < 400:
		log.Info("CheckStatusCode gitea apiKeys connection error: Redirect message")
	case 401 == res.StatusCode:
		log.Info("CheckStatusCode gitea apiKeys connection Error: Unauthorized")
	case 400 <= res.StatusCode && res.StatusCode < 500:
		log.Info("CheckStatusCode gitea apiKeys connection error: Client error")
	case 500 <= res.StatusCode && res.StatusCode < 600:
		log.Info("CheckStatusCode gitea apiKeys connection error Server error")
	}
}

func hasTimedOut(err error) bool {
	switch err := err.(type) {
	case *url.Error:
		if err, ok := err.Err.(net.Error); ok && err.Timeout() {
			return true
		}
	case net.Error:
		if err.Timeout() {
			return true
		}
	case *net.OpError:
		if err.Timeout() {
			return true
		}
	}

	errTxt := "use of closed network connection"
	if err != nil && strings.Contains(err.Error(), errTxt) {
		return true
	}

	return false
}

func requestGet(apiKeys GiteaKeys) []byte {
	cc := &http.Client{Timeout: time.Second * 2}
	url := apiKeys.BaseURL + apiKeys.Command + apiKeys.TokenKey[apiKeys.BruteforceTokenKey]
	res, err := cc.Get(url)
	if err != nil && hasTimedOut(err) {
		log.Fatalf("Get Request to %s failed: %v", url, err)
	}
	checkStatusCode(res)
	b, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}
	res.Body.Close()
	return b
}

func requestPut(apiKeys GiteaKeys) []byte {
	cc := &http.Client{Timeout: time.Second * 2}
	url := apiKeys.BaseURL + apiKeys.Command + apiKeys.TokenKey[apiKeys.BruteforceTokenKey]
	request, err := http.NewRequest("PUT", url, nil)
	res, err := cc.Do(request)
	checkStatusCode(res)
	if err != nil && hasTimedOut(err) {
		log.Fatalf("PUT Request to %s failed: %v", url, err)
	}
	b, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}
	res.Body.Close()
	return b
}

func requestDel(apiKeys GiteaKeys) []byte {

	cc := &http.Client{Timeout: time.Second * 2}
	url := apiKeys.BaseURL + apiKeys.Command + apiKeys.TokenKey[apiKeys.BruteforceTokenKey]
	request, err := http.NewRequest("DELETE", url, nil)
	res, err := cc.Do(request)
	checkStatusCode(res)
	if err != nil && hasTimedOut(err) {
		log.Fatalf("DELETE Request to %s failed: %v", url, err)
	}
	b, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Fatal(readErr)
	}
	res.Body.Close()
	return b
}

func requestSearchResults(APIKeys GiteaKeys) SearchResults {

	b := requestGet(APIKeys)

	var f SearchResults
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatal(jsonErr)
	}
	return f
}

func requestUsersList(APIKeys GiteaKeys) (map[string]Account, int) {

	b := requestGet(APIKeys)
	var AccountU = make(map[string]Account)

	var f []Account
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Errorf("Error unmarshaling json response: %v", jsonErr)
		if APIKeys.BruteforceTokenKey == len(APIKeys.TokenKey)-1 {
			log.Fatal("Token key is unsuitable, call to system administrator ")
		} else {
			log.Error("Can't get UsersList try another token key")
		}
		if APIKeys.BruteforceTokenKey < len(APIKeys.TokenKey)-1 {
			APIKeys.BruteforceTokenKey++
			log.Debugf("BruteforceTokenKey=%d", APIKeys.BruteforceTokenKey)
			AccountU, APIKeys.BruteforceTokenKey = requestUsersList(APIKeys)
		}
	}

	for i := 0; i < len(f); i++ {
		AccountU[f[i].Login] = Account{
			//			Email:     f[i].Email,
			ID:       f[i].ID,
			FullName: f[i].FullName,
			Login:    f[i].Login,
		}
	}
	return AccountU, APIKeys.BruteforceTokenKey
}

func requestOrganizationList(apiKeys GiteaKeys) []Organization {

	b := requestGet(apiKeys)

	var f []Organization
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatalf("Please check setting GITEA_TOKEN, GITEA_URL. Error unmarshaling JSON: %v", jsonErr)
	}
	return f
}

func requestTeamList(apiKeys GiteaKeys) []Team {

	b := requestGet(apiKeys)

	var f []Team
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatalf("Please check setting GITEA_TOKEN, GITEA_URL. Error unmarshaling JSON: %v", jsonErr)
	}
	return f
}

func parseJSON(b []byte) interface{} {
	var f interface{}
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatalf("Please check setting GITEA_TOKEN, GITEA_URL. Error unmarshaling JSON: %v", jsonErr)
	}
	m := f.(interface{})
	return m
}

func parseJSONArray(b []byte) []interface{} {
	var f interface{}
	jsonErr := json.Unmarshal(b, &f)
	if jsonErr != nil {
		log.Fatalf("Please check setting GITEA_TOKEN, GITEA_URL. Error unmarshaling JSON: %v", jsonErr)
	}
	m := f.([]interface{})
	return m
}
