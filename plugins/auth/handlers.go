package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/dgrijalva/jwt-go"

	"github.com/appbaseio/reactivesearch-api/util"
)

func (a *Auth) savePublicKey(ctx context.Context, req *http.Request, indexName string, record publicKey) (interface{}, error) {
	if strings.TrimSpace(record.RoleKey) == "" {
		record.RoleKey = "role"
	}

	// Update es index
	_, err := a.es.savePublicKey(ctx, req, indexName, record)
	if err != nil {
		log.Errorln(logTag, ": error indexing public key record", logTag)
		return false, err
	}

	return true, nil
}

func (a *Auth) getPublicKey() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		record, _ := a.es.getPublicKey(req.Context(), req)
		rawPermission, err := json.Marshal(record)
		if err != nil {
			msg := "public key record not found"
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}
		util.WriteBackRaw(w, rawPermission, http.StatusOK)
	}
}

func (a *Auth) setPublicKey() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		var body publicKey
		err = json.Unmarshal(reqBody, &body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't parse request body", http.StatusBadRequest)
			return
		}

		publicKeyIndex := os.Getenv(envPublicKeyEsIndex)
		if publicKeyIndex == "" {
			publicKeyIndex = defaultPublicKeyEsIndex
		}

		jwtRsaPublicKey, err := getJWTPublickKey(body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// update public key locally
			a.updateLocalPublicKey(jwtRsaPublicKey, body.RoleKey)
			util.WriteBackMessage(w, "Public key saved successfully.", http.StatusOK)
			return
		}
		_, err = a.savePublicKey(req.Context(), req, publicKeyIndex, body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Invoke ACCAPI
		// unmarshal body
		var bodyJSON map[string]interface{}
		err2 := json.Unmarshal(reqBody, &bodyJSON)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			util.WriteBackError(w, err2.Error(), http.StatusBadRequest)
			return
		}
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPut,
				URL:    "/_public_key",
				Body:   bodyJSON, // forward body
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered updating public key")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			// Update local state
			a.updateLocalPublicKey(jwtRsaPublicKey, body.RoleKey)
		}
		util.WriteBackMessage(w, "Public key saved successfully.", http.StatusOK)
	}
}

func (a *Auth) updateLocalPublicKey(jwtRsaPublicKey *rsa.PublicKey, role string) {
	if strings.TrimSpace(role) == "" {
		role = "role"
	}
	// Update cached public key
	if jwtRsaPublicKey != nil {
		a.jwtRsaPublicKey = jwtRsaPublicKey
		a.jwtRoleKey = role
	}
}

func getJWTPublickKey(record publicKey) (*rsa.PublicKey, error) {
	var jwtRsaPublicKey *rsa.PublicKey
	if record.PublicKey != "" {
		publicKeyBuf, err := util.DecodeBase64Key(record.PublicKey)
		if err != nil {
			log.Errorln(logTag, ": error indexing public key record", err)
			return nil, err
		}
		jwtRsaPublicKey, err = jwt.ParseRSAPublicKeyFromPEM(publicKeyBuf)
		if err != nil {
			log.Errorln(logTag, ": error indexing public key record", err)
			return jwtRsaPublicKey, err
		}
		return jwtRsaPublicKey, nil
	}
	return nil, errors.New("public key is missing in the request body")
}
