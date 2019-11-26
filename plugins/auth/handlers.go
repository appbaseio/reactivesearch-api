package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/dgrijalva/jwt-go"

	"github.com/appbaseio/arc/util"
)

func (a *Auth) savePublicKey(ctx context.Context, indexName string, record PublicKey) (interface{}, error) {
	var jwtRsaPublicKey *rsa.PublicKey
	if record.PublicKey != "" {
		publicKeyBuf, err := util.DecodeBase64Key(record.PublicKey)
		if err != nil {
			log.Printf("%s: error indexing public key record", logTag)
			return false, err
		}
		jwtRsaPublicKey, err = jwt.ParseRSAPublicKeyFromPEM(publicKeyBuf)
		if err != nil {
			return false, err
		}
	} else {
		return false, errors.New("Public key is missing in the request body")
	}
	if strings.TrimSpace(record.RoleKey) == "" {
		record.RoleKey = "role"
	}

	// Update es index
	_, err := a.es.savePublicKey(ctx, indexName, record)
	if err != nil {
		log.Printf("%s: error indexing public key record", logTag)
		return false, err
	}

	// Update cached public key
	if jwtRsaPublicKey != nil {
		a.jwtRsaPublicKey = jwtRsaPublicKey
		a.jwtRoleKey = record.RoleKey
	}

	return true, nil
}

func (a *Auth) getPublicKey() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		record, _ := a.es.getPublicKey(req.Context())
		rawPermission, err := json.Marshal(record)
		if err != nil {
			msg := fmt.Sprintf(`public key record not found`)
			util.WriteBackError(w, msg, http.StatusNotFound)
		}
		util.WriteBackRaw(w, rawPermission, http.StatusOK)
	}
}

func (a *Auth) setPublicKey() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		var body PublicKey
		err = json.Unmarshal(reqBody, &body)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, "Can't parse request body", http.StatusBadRequest)
			return
		}

		publicKeyIndex := os.Getenv(envPublicKeyEsIndex)
		if publicKeyIndex == "" {
			publicKeyIndex = defaultPublicKeyEsIndex
		}

		_, err = a.savePublicKey(req.Context(), publicKeyIndex, body)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		raw, err2 := json.Marshal(map[string]interface{}{
			"message": "Public key saved successfully.",
		})
		if err2 != nil {
			log.Printf("%s: %v\n", logTag, err2)
			util.WriteBackError(w, err2.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}
