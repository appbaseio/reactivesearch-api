package uibuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func (e *UIBuilder) getSearchPreference() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		preferenceId := vars["id"]
		record, err := e.es.getSearchPreference(req.Context(), preferenceId)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusNotFound)
			return
		}
		response, err1 := json.Marshal(record)
		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			telemetry.WriteBackErrorWithTelemetry(req, w, err1.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, response, http.StatusOK)
	}
}

func (e *UIBuilder) getRecommendationPreference() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		preferenceId := vars["id"]
		record, err := e.es.getRecommendationPreference(req.Context(), preferenceId)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusNotFound)
			return
		}
		response, err1 := json.Marshal(record)
		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			telemetry.WriteBackErrorWithTelemetry(req, w, err1.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, response, http.StatusOK)
	}
}

func (e *UIBuilder) putSearchPreference() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		preferenceId := vars["id"]
		var body SearchPreferenceRequest
		d := json.NewDecoder(req.Body)
		err := d.Decode(&body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		if body.Pipeline == "" {
			telemetry.WriteBackErrorWithTelemetry(req, w, "pipeline property can not be empty", http.StatusBadRequest)
			return
		}
		body.ID = preferenceId
		// Assign type
		body.Type = Search
		if body.Name == "" {
			body.Name = "Search " + body.Pipeline
		}
		updatedAt := time.Now().Unix()
		createdAt := updatedAt
		// Use created_at from existing preference
		preference, err := e.es.getSearchPreference(context.Background(), body.ID)
		if err == nil {
			if preference.CreatedAt != nil {
				createdAt = *preference.CreatedAt
			}
		}
		requestBody := SearchPreference{
			ID:                     body.ID,
			Name:                   body.Name,
			Description:            body.Description,
			Pipeline:               body.Pipeline,
			Type:                   body.Type,
			DeploySettings:         MapToString(body.DeploySettings),
			AuthenticationSettings: MapToString(body.AuthenticationSettings),
			ComponentSettings:      MapToString(body.ComponentSettings),
			PageSettings:           MapToString(body.PageSettings),
			ThemeSettings:          MapToString(body.ThemeSettings),
			FusionSettings:         MapToString(body.FusionSettings),
			GlobalSettings:         MapToString(body.GlobalSettings),
			ResultSettings:         MapToString(body.ResultSettings),
			ExportSettings:         MapToString(body.ExportSettings),
			SearchSettings:         MapToString(body.SearchSettings),
			FacetSettings:          MapToString(body.FacetSettings),
			SyncSettings:           MapToString(body.SyncSettings),
			UpdatedAt:              &updatedAt,
			CreatedAt:              &createdAt,
			IsUsingStringFields:    true,
		}
		err1 := e.es.savePreference(req.Context(), body.ID, requestBody)
		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			telemetry.WriteBackErrorWithTelemetry(req, w, err1.Error(), http.StatusInternalServerError)
			return
		}
		response, _ := json.Marshal(body)
		util.WriteBackRaw(w, response, http.StatusOK)
	}
}

func (e *UIBuilder) putRecommendationPreference() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		preferenceId := vars["id"]
		var body RecommendationPreferenceRequest
		d := json.NewDecoder(req.Body)
		err := d.Decode(&body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		if body.Pipeline == "" {
			telemetry.WriteBackErrorWithTelemetry(req, w, "pipeline property can not be empty", http.StatusBadRequest)
			return
		}
		body.ID = preferenceId
		if body.Name == "" {
			body.Name = "Recommendation " + body.Pipeline
		}
		// Assign type
		body.Type = Recommendation
		updatedAt := time.Now().Unix()
		createdAt := updatedAt
		// Use created_at from existing preference
		preference, err := e.es.getRecommendationPreference(context.Background(), body.ID)
		if err == nil {
			if preference.CreatedAt != nil {
				createdAt = *preference.CreatedAt
			}
		}
		requestBody := RecommendationPreference{
			ID:                     body.ID,
			Name:                   body.Name,
			Description:            body.Description,
			Pipeline:               body.Pipeline,
			Type:                   body.Type,
			DeploySettings:         body.DeploySettings,
			ThemeSettings:          body.ThemeSettings,
			GlobalSettings:         body.GlobalSettings,
			ResultSettings:         body.ResultSettings,
			ExportSettings:         body.ExportSettings,
			RecommendationSettings: body.RecommendationSettings,
			UpdatedAt:              &updatedAt,
			CreatedAt:              &createdAt,
		}
		err1 := e.es.savePreference(req.Context(), body.ID, requestBody)
		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			telemetry.WriteBackErrorWithTelemetry(req, w, err1.Error(), http.StatusInternalServerError)
			return
		}
		response, _ := json.Marshal(body)
		util.WriteBackRaw(w, response, http.StatusOK)
	}
}

func (e *UIBuilder) putUIbuilderCode() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		var body PutObjectToBucketRequest
		d := json.NewDecoder(req.Body)
		err := d.Decode(&body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		bodyInBytes, _ := json.Marshal(body)
		response, err := performRequestToACCAPI(http.MethodPut, uiBuilderId+"/code", bodyInBytes)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		if response != nil {
			bytesResponse, _ := ioutil.ReadAll(response.Body)
			var responseMap map[string]interface{}
			err := json.Unmarshal(bytesResponse, &responseMap)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			if response.StatusCode != http.StatusOK {
				errMsg, ok := responseMap["message"].(string)
				if ok {
					telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, response.StatusCode)
					return
				}
				telemetry.WriteBackErrorWithTelemetry(req, w, "error while updating record: "+string(bytesResponse), response.StatusCode)
				return
			} else {
				versionId, ok := responseMap["version_id"].(string)
				if ok {
					successResponse := map[string]interface{}{
						"version_id": versionId,
						"created_at": time.Now().Unix(),
						"status":     "success",
						"code":       http.StatusOK,
					}
					responseInBytes, _ := json.Marshal(successResponse)
					util.WriteBackRaw(w, responseInBytes, http.StatusOK)
					return
				}
			}
		}
		if util.OfflineBilling {
			telemetry.WriteBackErrorWithTelemetry(req, w, "UI builder version control API is not available for offline mode.", http.StatusBadRequest)
			return
		}
		telemetry.WriteBackErrorWithTelemetry(req, w, "error while updating record", http.StatusInternalServerError)
	}
}

func (e *UIBuilder) getUIbuilderCode() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		response, err := getUIBuilderCode(uiBuilderId, "")
		if err != nil {
			code := err.code
			if code == 0 {
				code = http.StatusInternalServerError
			}
			telemetry.WriteBackErrorWithTelemetry(req, w, err.err.Error(), code)
			return
		}
		if response != nil {
			responseInBytes, _ := json.Marshal(response)
			util.WriteBackRaw(w, responseInBytes, http.StatusOK)
			return
		}
		if util.OfflineBilling {
			telemetry.WriteBackErrorWithTelemetry(req, w, "UI builder version control API is not available for offline mode.", http.StatusBadRequest)
			return
		}
		telemetry.WriteBackErrorWithTelemetry(req, w, "error while updating record", http.StatusInternalServerError)
	}
}

type Error struct {
	code int
	err  error
}

func performRequestToACCAPI(method string, path string, requestBody []byte) (*http.Response, error) {
	// Avoid call to ACCAPI for offline billing
	if util.OfflineBilling {
		return nil, nil
	}
	// Call ACCAPI to trigger update for other nodes
	arcID, err := util.GetArcID()
	if err != nil {
		return nil, err
	}
	// TODO: CHANGE ACCAPI
	req, err := http.NewRequest(method, util.ACCAPI+"arc/uibuilder/"+arcID+"/"+path, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")
	res, err := util.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func getUIBuilderCode(uiBuilderId, versionId string) (*map[string]interface{}, *Error) {
	path := uiBuilderId + "/code"
	if versionId != "" {
		path = uiBuilderId + "/code/version/" + versionId
	}
	response, err := performRequestToACCAPI(http.MethodGet, path, nil)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, &Error{err: err}
	}
	if response != nil {
		bytesResponse, _ := ioutil.ReadAll(response.Body)
		var responseMap map[string]interface{}
		err := json.Unmarshal(bytesResponse, &responseMap)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil, &Error{err: err}
		}
		if response.StatusCode != http.StatusOK {
			errMsg, ok := responseMap["message"].(string)
			if ok {
				return nil, &Error{err: errors.New(errMsg), code: response.StatusCode}
			}
			return nil, &Error{err: errors.New("error while updating record: " + string(bytesResponse)), code: response.StatusCode}
		}
		return &responseMap, nil
	}
	return nil, &Error{err: errors.New("error while connecting to accapi")}
}

func (e *UIBuilder) getUIbuilderCodeByVersion() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		versionId := vars["versionId"]
		response, err := getUIBuilderCode(uiBuilderId, versionId)
		if err != nil {
			code := err.code
			if code == 0 {
				code = http.StatusInternalServerError
			}
			telemetry.WriteBackErrorWithTelemetry(req, w, err.err.Error(), code)
			return
		}
		if response != nil {
			responseInBytes, _ := json.Marshal(response)
			util.WriteBackRaw(w, responseInBytes, http.StatusOK)
			return
		}
		if util.OfflineBilling {
			telemetry.WriteBackErrorWithTelemetry(req, w, "UI builder version control API is not available for offline mode.", http.StatusBadRequest)
			return
		}
		telemetry.WriteBackErrorWithTelemetry(req, w, "error while updating record", http.StatusInternalServerError)
	}
}

func (e *UIBuilder) getUIbuilderCodeVersions() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		response, err := performRequestToACCAPI(http.MethodGet, uiBuilderId+"/code/versions", nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		if response != nil {
			bytesResponse, _ := ioutil.ReadAll(response.Body)
			var responseMap map[string]interface{}
			err := json.Unmarshal(bytesResponse, &responseMap)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			if response.StatusCode != http.StatusOK {
				errMsg, ok := responseMap["message"].(string)
				if ok {
					telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, response.StatusCode)
					return
				}
				telemetry.WriteBackErrorWithTelemetry(req, w, "error while updating record: "+string(bytesResponse), response.StatusCode)
				return
			} else {
				responseInBytes, _ := json.Marshal(responseMap)
				util.WriteBackRaw(w, responseInBytes, http.StatusOK)
				return
			}
		}
		if util.OfflineBilling {
			telemetry.WriteBackErrorWithTelemetry(req, w, "UI builder version control API is not available for offline mode.", http.StatusBadRequest)
			return
		}
		telemetry.WriteBackErrorWithTelemetry(req, w, "error while updating record", http.StatusInternalServerError)
	}
}

func (e *UIBuilder) deletePreference() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		preferenceId := vars["id"]
		err := e.es.deletePreference(req.Context(), preferenceId)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusNotFound)
			return
		}
		response, _ := json.Marshal(map[string]interface{}{
			"message": "Preferences deleted successfully.",
		})
		util.WriteBackRaw(w, response, http.StatusOK)
	}
}

func (e *UIBuilder) getSearchPreferences() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		record, err := e.es.getSearchPreferences(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusNotFound)
			return
		}
		response, err1 := json.Marshal(record)
		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			telemetry.WriteBackErrorWithTelemetry(req, w, err1.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, response, http.StatusOK)
	}
}

func (e *UIBuilder) getRecommendationPreferences() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		record, err := e.es.getRecommendationPreferences(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusNotFound)
			return
		}
		response, err1 := json.Marshal(record)
		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			telemetry.WriteBackErrorWithTelemetry(req, w, err1.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, response, http.StatusOK)
	}
}

func (e *UIBuilder) deployUIbuilderCode() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		res, err := performDeployRequestToACCAPI(uiBuilderId, http.MethodPut, "", req.URL.Query(), req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) getUIbuilderLatestDeployment() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		res, err := performDeployRequestToACCAPI(uiBuilderId, http.MethodGet, "", req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) getUIbuilderDeploymentByID() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		deploymentId := vars["deploymentID"]
		res, err := performDeployRequestToACCAPI(uiBuilderId, http.MethodGet, "/"+deploymentId, req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) cancelUIbuilderDeploymentByID() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		deploymentId := vars["deploymentID"]
		res, err := performDeployRequestToACCAPI(uiBuilderId, http.MethodPatch, "/"+deploymentId+"/cancel", req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) deleteUIbuilderDeploymentByID() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		deploymentId := vars["deploymentID"]
		res, err := performDeployRequestToACCAPI(uiBuilderId, http.MethodDelete, "/"+deploymentId+"/delete", req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) deleteUIbuilderDomainByID() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		domainID := vars["domainID"]
		res, err := performDomainRequestToACCAPI(uiBuilderId, http.MethodDelete, "/"+domainID, req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) verifyUIbuilderDomainByID() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		domainID := vars["domainID"]
		res, err := performDomainRequestToACCAPI(uiBuilderId, http.MethodPost, "/"+domainID, req.URL.Query(), req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) getUIbuilderDomainConfigByID() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		domainID := vars["domainID"]
		res, err := performDomainRequestToACCAPI(uiBuilderId, http.MethodGet, "/"+domainID+"/config", req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) createUIbuilderDomain() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		res, err := performDomainRequestToACCAPI(uiBuilderId, http.MethodPost, "", req.URL.Query(), req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) getUIbuilderDomains() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		res, err := performDomainRequestToACCAPI(uiBuilderId, http.MethodGet, "", req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (rx *UIBuilder) saveSearchBox() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		searchboxId := vars["searchboxId"]
		// validate searchbox Id
		// Note: Zinc term query doesn't work on field containing hyphen
		if !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(searchboxId) {
			telemetry.WriteBackErrorWithTelemetry(req, w, "Invalid searchbox id. Searchbox can only have numbers, characters and underscore(_).", http.StatusBadRequest)
			return
		}
		var searchboxPreference SearchBoxESModel
		d := json.NewDecoder(req.Body)
		err := d.Decode(&searchboxPreference)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "Can't read request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		searchboxPreference.Id = &searchboxId
		// generate id for featured suggestions
		if searchboxPreference.SearchBox != nil &&
			searchboxPreference.SearchBox.Featured != nil &&
			searchboxPreference.SearchBox.Featured.Layout != nil {
			sectionsWithIds := make([]SectionInfo, 0)
			for _, section := range searchboxPreference.SearchBox.Featured.Layout.Sections {
				newSection := section
				suggestionsWithIds := make([]SearchBoxESFeaturedSuggestionDoc, 0)
				for _, suggestion := range section.Suggestions {
					if suggestion.Id == nil {
						id := uuid.New().String()
						suggestion.Id = &id
					}
					suggestionsWithIds = append(suggestionsWithIds, suggestion)
				}
				newSection.Suggestions = suggestionsWithIds
				sectionsWithIds = append(sectionsWithIds, newSection)
			}
			searchboxPreference.SearchBox.Featured.Layout.Sections = sectionsWithIds
		}
		featuredSuggestions := getFeaturedSuggestionsFromSearchBoxes(
			[]SearchBoxESModel{searchboxPreference},
		)
		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// Index featured suggestions
			err := rx.featuredSuggestionsConfig.UpdateFeaturedSuggestions(
				searchboxId,
				featuredSuggestions,
			)
			if err != nil {
				log.Errorln(logTag, "error while indexing featured suggestions to zinc:", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Update local cache
			AddSearchBoxToCache(searchboxPreference)
			byteResponse, err3 := json.Marshal(map[string]interface{}{
				"id": searchboxId,
			})
			if err3 != nil {
				log.Errorln(logTag, ":", err3)
				telemetry.WriteBackErrorWithTelemetry(req, w, err3.Error(), http.StatusBadRequest)
				return
			}
			util.WriteBackRaw(w, byteResponse, http.StatusOK)
			return
		}
		if searchboxPreference.SearchBox == nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, errors.New("searchbox property must be present").Error(), http.StatusBadRequest)
			return
		}
		for _, v := range featuredSuggestions {
			if strings.TrimSpace(v.Label) == "" {
				telemetry.WriteBackErrorWithTelemetry(req, w, errors.New("label can not be empty for featured suggestion").Error(), http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(v.Value) == "" {
				telemetry.WriteBackErrorWithTelemetry(req, w, errors.New("value can not be empty for featured suggestion").Error(), http.StatusBadRequest)
				return
			}
		}
		// Handle endpoint struct changes to store as string
		if searchboxPreference.SearchBox != nil && searchboxPreference.SearchBox.Endpoint != nil &&
			searchboxPreference.SearchBox.Endpoint.Endpoint != nil {
			if searchboxPreference.SearchBox.Endpoint.Endpoint.BodyMap != nil {
				var b *string
				m, err := json.Marshal(*searchboxPreference.SearchBox.Endpoint.Endpoint.BodyMap)
				if err != nil {
					log.Errorln(logTag, ":", err)
				} else {
					s := string(m)
					b = &s
				}
				(*searchboxPreference.SearchBox.Endpoint.Endpoint).Body = b
				(*searchboxPreference.SearchBox.Endpoint.Endpoint).BodyMap = nil
			}
			if searchboxPreference.SearchBox.Endpoint.Endpoint.HeadersMap != nil {
				(*searchboxPreference.SearchBox.Endpoint.Endpoint).Headers = getHeadersString(searchboxPreference.SearchBox.Endpoint.Endpoint.HeadersMap)
				(*searchboxPreference.SearchBox.Endpoint.Endpoint).HeadersMap = nil
			}
			(*searchboxPreference.SearchBox.Endpoint.Endpoint).IsUsingStringFormat = true
		}

		err2 := rx.esFeaturedSuggestions.saveSearchBox(req.Context(), searchboxId, searchboxPreference)
		if err2 != nil {
			code := http.StatusInternalServerError
			if err2.code != 0 {
				code = err2.code
			}
			log.Errorln(logTag, ":", err2)
			telemetry.WriteBackErrorWithTelemetry(req, w, err2.err.Error(), code)
			return
		}

		// Invoke ACCAPI
		marshalledRequestBody, err5 := json.Marshal(searchboxPreference)
		if err5 != nil {
			log.Errorln(logTag, ":", err5)
			telemetry.WriteBackErrorWithTelemetry(req, w, err5.Error(), http.StatusInternalServerError)
			return
		}
		var bodyJSON map[string]interface{}
		err6 := json.Unmarshal(marshalledRequestBody, &bodyJSON)
		if err6 != nil {
			log.Errorln(logTag, ":", err6)
			telemetry.WriteBackErrorWithTelemetry(req, w, err6.Error(), http.StatusInternalServerError)
			return
		}
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPut,
				URL:    "/_uibuilder/searchbox/" + searchboxId,
				Body:   bodyJSON, // forward body
			})
			if err != nil {
				status := http.StatusInternalServerError
				if res != nil {
					status = res.StatusCode
				}
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), status)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered updating searchbox preferences")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), res.StatusCode)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			// Index featured suggestions
			rx.featuredSuggestionsConfig.UpdateFeaturedSuggestions(
				searchboxId,
				featuredSuggestions,
			)
			if err != nil {
				log.Errorln(logTag, "error while indexing featured suggestions to zinc:", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Update local cache
			AddSearchBoxToCache(searchboxPreference)
		}

		byteResponse, err3 := json.Marshal(map[string]interface{}{
			"id": searchboxId,
		})
		if err3 != nil {
			log.Errorln(logTag, ":", err3)
			telemetry.WriteBackErrorWithTelemetry(req, w, err3.Error(), http.StatusBadRequest)
			return
		}
		util.WriteBackRaw(w, byteResponse, http.StatusOK)
	}
}

func (rx *UIBuilder) getSearchBox() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		searchboxId := vars["searchboxId"]

		response, err2 := rx.esFeaturedSuggestions.getSearchBox(req.Context(), searchboxId)
		if err2 != nil {
			code := http.StatusInternalServerError
			if err2.code != 0 {
				code = err2.code
			}
			log.Errorln(logTag, ":", err2.err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err2.err.Error(), code)
			return
		}
		byteResponse, err3 := json.Marshal(response)
		if err3 != nil {
			log.Errorln(logTag, ":", err3)
			telemetry.WriteBackErrorWithTelemetry(req, w, err3.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, byteResponse, http.StatusOK)
	}
}

func getHeadersString(headers *map[string]string) *string {
	if headers != nil {
		marshalled, err := json.Marshal(*headers)
		if err != nil {
			log.Errorln(logTag, ":", err)
		} else {
			s := string(marshalled)
			return &s
		}
	}
	return nil
}

func (rx *UIBuilder) getSearchBoxes() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		hidden := req.URL.Query().Get("hidden")
		response, err2 := rx.esFeaturedSuggestions.getSearchBoxes(req.Context(), hidden == "true")
		if err2 != nil {
			code := http.StatusInternalServerError
			if err2.code != 0 {
				code = err2.code
			}
			log.Errorln(logTag, ":", err2.err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err2.err.Error(), code)
			return
		}
		byteResponse, err3 := json.Marshal(response)
		if err3 != nil {
			log.Errorln(logTag, ":", err3)
			telemetry.WriteBackErrorWithTelemetry(req, w, err3.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, byteResponse, http.StatusOK)
	}
}

func (rx *UIBuilder) deleteSearchBox() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		searchboxId := vars["searchboxId"]

		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// Update Cache
			err := rx.featuredSuggestionsConfig.DeleteFeaturedSuggestions(searchboxId, nil)
			if err != nil {
				log.Errorln(logTag, "error encountered while deleting featured suggestions from zinc index:", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Update local cache
			DeleteSearchBoxToCache(searchboxId)
			util.WriteBackMessage(w, "deleted suggestions successfully", http.StatusOK)
			return
		}

		err2 := rx.esFeaturedSuggestions.deleteSearchBox(req.Context(), searchboxId)
		if err2 != nil {
			code := http.StatusInternalServerError
			if err2.code != 0 {
				code = err2.code
			}
			log.Errorln(logTag, ":", err2.err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err2.err.Error(), code)
			return
		}

		// Invoke ACCAPI
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodDelete,
				URL:    "/_uibuilder/searchbox/" + searchboxId,
				Body:   nil, // forward body
			})
			if err != nil {
				status := http.StatusInternalServerError
				if res != nil {
					status = res.StatusCode
				}
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), status)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered deleting searchbox")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), res.StatusCode)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			// Update Cache
			err := rx.featuredSuggestionsConfig.DeleteFeaturedSuggestions(searchboxId, nil)
			if err != nil {
				log.Errorln(logTag, "error encountered while deleting featured suggestions from zinc index:", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Update local cache
			DeleteSearchBoxToCache(searchboxId)
		}
		util.WriteBackMessage(w, "deleted suggestions successfully", http.StatusOK)
	}
}

/* auth client related handlers */
func (e *UIBuilder) createAuthClient() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		// check if connection is already set
		preference, err := e.es.getAuthPreference(context.Background())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		if keyExists(preference, "_client_id") && preference["_client_id"] != nil && preference["_client_id"] != "" {
			err = errors.New("a client app already exists, can't create another client")
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
			return
		}
		// create the client app
		path := "uibuilder/authentication/" + appbaseId
		res, err := performHTTPRequestToACCAPI(http.MethodPost, path, req.URL.Query(), req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		// set the saml connnection id in auth preference
		var auth0ClientRes map[string]interface{}
		json.Unmarshal(body, &auth0ClientRes)
		if keyExists(auth0ClientRes, "client_id") {
			prefBody := make(map[string]interface{})
			prefBody["_client_id"] = auth0ClientRes["client_id"]
			_, err := setAuthPreferenceDo(e, context.Background(), prefBody)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) updateAuthClient() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		clientId := vars["client_id"]
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := "uibuilder/authentication/" + appbaseId + "/" + clientId
		res, err := performHTTPRequestToACCAPI(http.MethodPatch, path, req.URL.Query(), req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) getAuthClient() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		clientId := vars["client_id"]
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := "uibuilder/authentication/" + appbaseId + "/" + clientId
		res, err := performHTTPRequestToACCAPI(http.MethodGet, path, req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) deleteAuthClient() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		clientId := vars["client_id"]
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := "uibuilder/authentication/" + appbaseId + "/" + clientId
		res, err := performHTTPRequestToACCAPI(http.MethodDelete, path, req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		// reset the _client_id key in auth preference
		prefBody := make(map[string]interface{})
		prefBody["_client_id"] = ""
		_, err = setAuthPreferenceDo(e, context.Background(), prefBody)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) getAuthConnectionState() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		clientId := vars["client_id"]
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := "uibuilder/auth_connection_state/" + appbaseId + "/" + clientId
		res, err := performHTTPRequestToACCAPI(http.MethodGet, path, req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) updateAuthConnectionState() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		clientId := vars["client_id"]
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := "uibuilder/auth_connection_state/" + appbaseId + "/" + clientId
		res, err := performHTTPRequestToACCAPI(http.MethodPut, path, req.URL.Query(), req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

/* connection related handlers */

func (e *UIBuilder) createAuthConnection() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		// check if connection is already set
		preference, err := e.es.getAuthPreference(context.Background())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		if keyExists(preference, "_saml_conn_id") && preference["_saml_conn_id"] != nil && preference["_saml_conn_id"] != "" {
			err = errors.New("a saml connection already exists, can't create another connection")
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
			return
		}

		path := "uibuilder/auth_connection/" + appbaseId
		res, err := performHTTPRequestToACCAPI(http.MethodPost, path, req.URL.Query(), req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		// set the saml connnection id in auth preference
		var auth0ConnRes map[string]interface{}
		json.Unmarshal(body, &auth0ConnRes)
		if keyExists(auth0ConnRes, "id") {
			prefBody := make(map[string]interface{})
			prefBody["_saml_conn_id"] = auth0ConnRes["id"]
			_, err := setAuthPreferenceDo(e, context.Background(), prefBody)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) updateAuthConnection() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		connId := vars["conn_id"]
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := "uibuilder/auth_connection/" + appbaseId + "/" + connId
		res, err := performHTTPRequestToACCAPI(http.MethodPatch, path, req.URL.Query(), req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) getAuthConnection() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		connId := vars["conn_id"]
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := "uibuilder/auth_connection/" + appbaseId + "/" + connId
		res, err := performHTTPRequestToACCAPI(http.MethodGet, path, req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) deleteAuthConnection() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		connId := vars["conn_id"]
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := "uibuilder/auth_connection/" + appbaseId + "/" + connId
		res, err := performHTTPRequestToACCAPI(http.MethodDelete, path, req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		// reset the _saml_conn_id key in auth preference
		prefBody := make(map[string]interface{})
		prefBody["_saml_conn_id"] = ""
		_, err = setAuthPreferenceDo(e, context.Background(), prefBody)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

/* auth user related handlers */

func (e *UIBuilder) createAuthUser() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := "uibuilder/auth_user/" + appbaseId
		res, err := performHTTPRequestToACCAPI(http.MethodPost, path, req.URL.Query(), req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) updateAuthUser() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		userId := vars["user_id"]
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := "uibuilder/auth_user/" + appbaseId + "/" + userId
		res, err := performHTTPRequestToACCAPI(http.MethodPatch, path, req.URL.Query(), req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) getAuthUser() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		userId := vars["user_id"]
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := "uibuilder/auth_user/" + appbaseId + "/" + userId
		res, err := performHTTPRequestToACCAPI(http.MethodGet, path, req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) getAuthUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var queryString string
		queryStrings := req.URL.Query()
		if queryStrings != nil {
			queryString = queryStrings.Get("q")
		}
		if queryString == "" || !(strings.Contains(queryString, "app_metadata.") && strings.Contains(queryString, "=true")) {
			errString := "query string filtering by an application client's id as `?q=app_metadata.${client_id}=true` is required but not passed"
			log.Errorln(logTag, ":", errString)
			telemetry.WriteBackErrorWithTelemetry(req, w, errString, http.StatusBadRequest)
			return
		}
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := "uibuilder/auth_users/" + appbaseId + "?q=" + queryString
		res, err := performHTTPRequestToACCAPI(http.MethodGet, path, req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) deleteAuthUser() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		userId := vars["user_id"]
		appbaseId, err := util.GetArcID()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		path := "uibuilder/auth_user/" + appbaseId + "/" + userId
		res, err := performHTTPRequestToACCAPI(http.MethodDelete, path, req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) setAuthPreference() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var body map[string]interface{}
		err := json.NewDecoder(req.Body).Decode(&body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		response, err := setAuthPreferenceDo(e, context.Background(), body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, response, http.StatusOK)
	}
}

func (e *UIBuilder) getAuthPreference() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Use created_at from existing auth preference
		preference, err := e.es.getAuthPreference(context.Background())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusNotFound)
			return
		}
		response, _ := json.Marshal(preference)
		util.WriteBackRaw(w, response, http.StatusOK)
	}
}

func (e *UIBuilder) createUIbuilderEnv() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		res, err := performEnvRequestToACCAPI(uiBuilderId, http.MethodPost, "", req.URL.Query(), req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) getUIbuilderEnvs() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		res, err := performEnvRequestToACCAPI(uiBuilderId, http.MethodGet, "", req.URL.Query(), nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) editUIbuilderEnv() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		envId := vars["envID"]
		res, err := performEnvRequestToACCAPI(uiBuilderId, http.MethodPatch, "/"+envId, req.URL.Query(), req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}

func (e *UIBuilder) deleteUIbuilderEnv() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		uiBuilderId := vars["id"]
		envId := vars["envID"]
		res, err := performEnvRequestToACCAPI(uiBuilderId, http.MethodDelete, "/"+envId, req.URL.Query(), req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, body, res.StatusCode)
	}
}
