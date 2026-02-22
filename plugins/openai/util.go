package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/buger/jsonparser"
	"github.com/lithammer/shortuuid/v4"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"moul.io/http2curl"
)

var (
	sessionSingleton *SessionIdToChatGPTResponse
	sessionOnce      sync.Once
)

const (
	OpenAIAPIURL     string = "https://api.openai.com/v1"
	defaultModel     string = "gpt-3.5-turbo"
	defaultPrompt    string = "You're a helpful assistant."
	defaultMaxTokens int    = 300
	defaultMinTokens int    = 100
)

type OpenAIConfig struct {
	Enable                *bool           `json:"enable,omitempty"`
	OpenAIKey             *string         `json:"open_ai_key,omitempty"`
	Model                 *string         `json:"model,omitempty"`
	DefaultEmbeddingModel *EmbeddingModel `json:"embeddingModel"`
	SystemPrompt          *string         `json:"systemPrompt,omitempty"`
	MaxTokens             *int            `json:"maxTokens,omitempty"`
	MinTokens             *int            `json:"minTokens,omitempty"`
	Indexes               *[]string       `json:"enabledIndexes,omitempty"`
	APIType               *APIType        `json:"apiType,omitempty"`
	AzureBaseURL          *string         `json:"azureBaseURL"`
	AzureVersion          *string         `json:"azureVersion"`
}

// Store the OpenAI Config cached value. This should be the source
// of truth.
var openAIConfigCache OpenAIConfig = GetDefaultConfig()

// SetConfig will set the OpenAI config value
func (r *OpenAI) SetConfig(configPassed OpenAIConfig) {
	openAIConfigCache = configPassed
}

// GetConfig will get the OpenAI config value
func (r *OpenAI) GetConfig() OpenAIConfig {
	return openAIConfigCache
}

type InternalChatGPTResponse struct {
	RequestBody             InternalChatGPTRequest `json:"request_body"`
	ResponseInBytes         []byte                 `json:"response_in_bytes"`
	AddedAt                 int64                  `json:"added_at"`
	RemoveAt                int64                  `json:"remove_at"`
	IsReady                 bool                   `json:"is_ready"`
	IsFailed                bool                   `json:"is_failed"`
	StartTrimmingAtPosition int                    `json:"start_trimming_at_position"`
	StreamChannel           *chan []byte           `json:"-"`
	sessionDoc              *AISessionDoc
	IsStreaming             bool
}

type InternalChatGPTRequest struct {
	InBytes      []byte         `json:"in_bytes"`
	AsStruct     ChatGPTRequest `json:"as_struct"`
	UnmarshalErr error          `json:"unmarshal_err"`
}

type ChatGPTRequest struct {
	Model       string           `json:"model"`
	Messages    []ChatGPTMessage `json:"messages"`
	MaxTokens   *int             `json:"maxTokens"`
	Temperature *float64         `json:"temperature"`
	DocumentIds []string         `json:"documentIds"`
}

type ChatGPTMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Bytes will return the ChatGPT request body in bytes
func (i InternalChatGPTRequest) Bytes() []byte {
	return i.InBytes
}

// IsStructReady will indicate whether or not the struct is
// ready for use
func (i InternalChatGPTRequest) IsStructReady() bool {
	return i.Error() == nil
}

// Error will return the unmarshalErr for the struct.
//
// This method will not check whether the error is nil or not
func (i InternalChatGPTRequest) Error() error {
	return i.UnmarshalErr
}

// Struct will return the request body in the custom datatype
//
// This method will not check whether or not the struct is valid.
// That can be checked by using the `IsStructReady()` method or the
// `Error()` method.
func (i InternalChatGPTRequest) Struct() ChatGPTRequest {
	return i.AsStruct
}

// Response will return the ChatGPT response
func (i InternalChatGPTResponse) Response() []byte {
	return i.ResponseInBytes
}

// Request will return the ChatGPT request sent
func (i InternalChatGPTResponse) Request() InternalChatGPTRequest {
	return i.RequestBody
}

// TrimAt will return the position at which trimming the messages can start
func (i InternalChatGPTResponse) TrimAt() int {
	return i.StartTrimmingAtPosition
}

// IsReady will indicate if the response is ready to be
// read
func (i InternalChatGPTResponse) GetIsReady() bool {
	return i.IsReady
}

// IsFailed will indicate whether the request failed to execute
// or complete execution
func (i InternalChatGPTResponse) GetIsFailed() bool {
	return i.IsFailed
}

// GetIsStreaming will return the value for IsStreaming indicating whether
// the response is being streamed.
func (i InternalChatGPTResponse) GetIsStreaming() bool {
	return i.IsStreaming
}

// SetIsReady will set the value of isReady to the passed value
func (i InternalChatGPTResponse) SetIsReady(newVal bool) {
	i.IsReady = newVal
}

// RemoveAt will return the time when the response should be
// removed at
func (i InternalChatGPTResponse) GetRemoveAt() int64 {
	return i.RemoveAt
}

// GetChannel will return the channel attached to the session
func (i InternalChatGPTResponse) GetChannel() *chan []byte {
	return i.StreamChannel
}

// SetChannel will set the passed channel in the response
func (i InternalChatGPTResponse) SetChannel(passedChan chan []byte) {
	i.StreamChannel = &passedChan
}

// UpdateSession will update the AISessionDoc from the request, response
// values present in the current ChatGPT response.
func (i InternalChatGPTResponse) UpdateSession(sessionId string) {
	// Increment invocation count
	if i.sessionDoc.InvocationCount == nil {
		newInvocCount := 0
		i.sessionDoc.InvocationCount = &newInvocCount
	}
	*(i.sessionDoc.InvocationCount)++

	// Make sure inputTokens and outputTokens are not nil
	if i.sessionDoc.InputTokens == nil {
		i.sessionDoc.InputTokens = new(int64)
		*i.sessionDoc.InputTokens = 0
	}
	if i.sessionDoc.OutputTokens == nil {
		i.sessionDoc.OutputTokens = new(int64)
		*i.sessionDoc.OutputTokens = 0
	}

	// Extract inputTokens, outputTokens, model from response
	inputTokenCount, fetchErr := jsonparser.GetInt(i.Response(), "usage", "prompt_tokens")
	if fetchErr != nil {
		// Seems like the usage key doesn't exist, can't do anything here
		log.Warnln(logTag, ": couldn't extract inputToken details from response")
	} else {
		*(i.sessionDoc.InputTokens) += inputTokenCount
	}

	outputTokenCount, fetchErr := jsonparser.GetInt(i.Response(), "usage", "completion_tokens")
	if fetchErr != nil {
		// Seems like the usage key doesn't exist, can't do anything here
		log.Warnln(logTag, ": couldn't extract outputToken details from response")
	} else {
		*(i.sessionDoc.OutputTokens) += outputTokenCount
	}

	// Extract the model only if the model field is not populated
	if i.sessionDoc.Model == nil {
		modelAsStr, modelFetchErr := jsonparser.GetString(i.Response(), "model")
		if modelFetchErr != nil {
			log.Warnln(logTag, ": couldn't extract model from response")
		} else {
			i.sessionDoc.Model = &modelAsStr
		}
	}

	// Extract older messages from request and add the response choice
	// and update the messages array

	// It's not likely but the request might not be populated yet
	allMessages := i.Request().Struct().Messages

	// Add the response's choice into there as well
	choiceObject, choiceType, _, choiceFetchErr := jsonparser.Get(i.Response(), "choices", "[0]", "message")
	if choiceFetchErr != nil || choiceType != jsonparser.Object {
		log.Warnln(logTag, ": error while extracting latest response from GPT!")
	} else {
		var choiceAsType ChatGPTMessage
		unmarshalErr := json.Unmarshal(choiceObject, &choiceAsType)
		if unmarshalErr != nil {
			log.Warnln(logTag, ": error while unmarshalling choice into an object: ", unmarshalErr.Error())
		} else {
			allMessages = append(allMessages, choiceAsType)
		}
	}

	i.sessionDoc.Messages = &allMessages

	// Finally, capture the internal response structure at this time as well
	responseMarshalled, marshalErr := json.Marshal(i)
	if marshalErr != nil {
		log.Warnln(logTag, ": error while marshalling internal response: ", marshalErr.Error())
	} else {
		marshalledAsStr := string(responseMarshalled)
		i.sessionDoc.InternalResponse = &marshalledAsStr
	}

	// Update the updated_at time value
	currentTime := time.Now().Unix()
	i.sessionDoc.UpdatedAt = &currentTime

	// Trigger ES update as well
	saveErr := Instance().analyticsEs.saveSession(context.Background(), *i.sessionDoc, sessionId)
	if saveErr != nil {
		log.Errorln(logTag, ": error while saving session details in upstream: ", saveErr.Error())
	}
}

// GetSession will return the session details attached to the response
func (i InternalChatGPTResponse) GetSession() *AISessionDoc {
	sessionDocCopy := new(AISessionDoc)
	*sessionDocCopy = *i.sessionDoc
	return sessionDocCopy
}

type SessionIdToChatGPTResponse struct {
	mu         sync.Mutex
	storageMap map[string]*InternalChatGPTResponse
}

// GetResponse will return the ChatGPT response for the passed
// session ID
func (s *SessionIdToChatGPTResponse) GetResponse(sessionId string) *InternalChatGPTResponse {
	s.mu.Lock()
	responseDetails := s.storageMap[sessionId]
	s.mu.Unlock()

	// If response details are not found locally, we will need to pull it
	// from ES
	if responseDetails != nil {
		return responseDetails
	}

	// Try to pull from ES
	responseDetails = s.FetchSession(context.Background(), sessionId)

	if responseDetails == nil {
		return responseDetails
	}

	// Save this fetched session in storage as well
	s.mu.Lock()
	s.storageMap[sessionId] = responseDetails
	s.mu.Unlock()

	return responseDetails
}

// FetchSession will try to fetch the session details by using the passed
// sessionId and accordingly populate the local session details
func (s *SessionIdToChatGPTResponse) FetchSession(ctx context.Context, sessionId string) *InternalChatGPTResponse {
	responseFromEs, fetchErr := Instance().analyticsEs.getSession(ctx, sessionId)
	if fetchErr != nil {
		log.Warnln(logTag, ": ", fetchErr.Error())
		return nil
	}

	var responseAsType InternalChatGPTResponse
	unmarshalErr := json.Unmarshal([]byte(*responseFromEs.InternalResponse), &responseAsType)

	if unmarshalErr != nil {
		log.Warnln(logTag, ": ", unmarshalErr.Error())
		return nil
	}

	responseAsType.sessionDoc = responseFromEs
	defaultChannel := GetChannelWithSize(2000)
	responseAsType.StreamChannel = &defaultChannel

	// Update the RemoveAt to be 30 minutes for now
	responseAsType.RemoveAt = time.Now().Add(30 * time.Minute).Unix()

	return &responseAsType
}

// RegisterResponse will register a new response for the passed
// sessionId indicating that the response is in progress
func (s *SessionIdToChatGPTResponse) RegisterResponse(sessionId string, userId string, indices []string, documentIds []string) {
	createdTime := time.Now().Unix()

	// Convert the timestamp to milliseconds since we're using go 1.16
	// and it doesn't have the Milli() method
	timeNow := time.Now().UnixNano() / int64(time.Millisecond)

	defaultChannel := GetChannelWithSize(2000)

	s.mu.Lock()
	s.storageMap[sessionId] = &InternalChatGPTResponse{
		IsReady:       false,
		IsFailed:      false,
		IsStreaming:   false,
		StreamChannel: &defaultChannel,
		sessionDoc: &AISessionDoc{
			SessionId:   &sessionId,
			Useful:      nil,
			CreatedAt:   &createdTime,
			UpdatedAt:   &createdTime,
			TimeStamp:   &timeNow,
			UserId:      &userId,
			Index:       indices,
			DocumentIds: documentIds,
		},
	}
	s.mu.Unlock()
}

// SetResolvedAt will set the time of the ChatGPT call resolution
func (s *SessionIdToChatGPTResponse) SetResolvedAt(sessionId string, resolvedAt int64, req *http.Request, firstByteReadAt int64, closedAt int64, requestInBytes []byte) {
	response := s.GetResponse(sessionId)
	if response == nil {
		response = &InternalChatGPTResponse{}
	}

	response.sessionDoc.ResponseResolution.ResolvedAt = resolvedAt
	response.sessionDoc.ResponseResolution.FirstByteReadAt = firstByteReadAt
	response.sessionDoc.ResponseResolution.ClosedAt = closedAt

	// Update the authorization header
	req.Header.Set("Authorization", "Bearer sk-test")

	// Generate the cURL from the passed request.
	curlGenerated, curlGenErr := http2curl.GetCurlCommand(req)
	if curlGenErr != nil {
		errMsg := fmt.Sprint("Error while generating cURL command from request: ", curlGenErr.Error())
		response.sessionDoc.ResponseResolution.AICallCurl = errMsg
	} else {
		response.sessionDoc.ResponseResolution.AICallCurl = curlGenerated.String()
	}

	// Store the request body separately since the above cURL command is ignoring it.
	response.sessionDoc.ResponseResolution.AIBody = string(requestInBytes)

	s.mu.Lock()
	s.storageMap[sessionId] = response
	s.mu.Unlock()
}

// SetSSECallAt will set the sse call at and sse call streaming start time
func (s *SessionIdToChatGPTResponse) SetSSECallAt(sessionId string, sseCallAt int64, sseCallStreamAt int64, firstWrittenAt int64) {
	response := s.GetResponse(sessionId)
	if response == nil {
		response = &InternalChatGPTResponse{}
	}

	response.sessionDoc.ResponseResolution.CallReceivedAt = sseCallAt
	response.sessionDoc.ResponseResolution.CallStreamResolvedAt = sseCallStreamAt
	response.sessionDoc.ResponseResolution.CallFirstWrittenAt = firstWrittenAt

	s.mu.Lock()
	s.storageMap[sessionId] = response
	s.mu.Unlock()
}

// SetRequest will set the request body for later access for the
// passed sessionId
func (s *SessionIdToChatGPTResponse) SetRequest(sessionId string, requestBody []byte, trimmingAt int) {

	response := s.GetResponse(sessionId)
	if response == nil {
		response = &InternalChatGPTResponse{}
	}

	response.IsReady = false
	response.RequestBody.InBytes = requestBody
	response.StartTrimmingAtPosition = trimmingAt

	// Unmarshal the body into the custom struct
	var requestAsStruct ChatGPTRequest
	unmarshalErr := json.Unmarshal(requestBody, &requestAsStruct)
	if unmarshalErr != nil {
		log.Warnln(logTag, ": error while unmarshalling ChatGPT request with error: ", unmarshalErr.Error())
		response.RequestBody.UnmarshalErr = unmarshalErr
	} else {
		response.RequestBody.AsStruct = requestAsStruct
		response.RequestBody.UnmarshalErr = nil
	}

	s.mu.Lock()
	s.storageMap[sessionId] = response
	s.mu.Unlock()
}

// AddResponse will add a new response to the session ID to ChatGPT
// response storage
func (s *SessionIdToChatGPTResponse) AddResponse(sessionId string, responseInBytes []byte) {
	// Consider the possibility of the response being registered
	// Try to fetch the response using the sessionId
	response := s.GetResponse(sessionId)
	if response == nil {
		response = &InternalChatGPTResponse{}
	}

	addedAt := time.Now()

	// Modify the response now with the new response
	response.ResponseInBytes = responseInBytes
	response.AddedAt = addedAt.Unix()
	response.RemoveAt = addedAt.Add(30 * time.Minute).Unix()
	response.IsReady = true
	response.IsStreaming = false

	// Empty the channel of any data present
	streamChannel := response.GetChannel()
	for len(*streamChannel) > 0 {
		<-*streamChannel
	}

	s.mu.Lock()
	s.storageMap[sessionId] = response
	s.mu.Unlock()

	// Trigger session doc update
	go response.UpdateSession(sessionId)
}

// GetChannel will return the channel associated to the passed SessionID
func (s *SessionIdToChatGPTResponse) GetChannel(sessionId string) *chan []byte {
	s.mu.Lock()
	responseDetails := s.storageMap[sessionId]
	s.mu.Unlock()

	if responseDetails == nil {
		defaultChan := make(chan []byte)
		return &defaultChan
	}

	return responseDetails.GetChannel()
}

// SetChannel will set the channel and associate it to the passed sessionId
func (s *SessionIdToChatGPTResponse) SetChannel(sessionId string, passedChan chan []byte) {
	s.mu.Lock()
	responseDetails := s.storageMap[sessionId]
	s.mu.Unlock()

	if responseDetails == nil {
		return
	}

	responseDetails.SetChannel(passedChan)

	s.mu.Lock()
	s.storageMap[sessionId] = responseDetails
	s.mu.Unlock()
}

// SetIsStreaming will set the value for isStreaming in the response
func (s *SessionIdToChatGPTResponse) SetIsStreaming(sessionId string, value bool) {
	s.mu.Lock()
	responseDetails := s.storageMap[sessionId]
	s.mu.Unlock()

	if responseDetails == nil {
		return
	}

	responseDetails.IsStreaming = value

	s.mu.Lock()
	s.storageMap[sessionId] = responseDetails
	s.mu.Unlock()
}

// SetIsFailed will set the passed value in the IsFailed field
//
// This function returns an error if the response is not registered or
// the passed sessionId is invalid
func (s *SessionIdToChatGPTResponse) SetIsFailed(sessionId string, value bool) error {
	response := s.GetResponse(sessionId)
	if response == nil {
		return fmt.Errorf("sessionId is invalid")
	}

	response.IsFailed = value
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storageMap[sessionId] = response

	return nil
}

// SetUseful will set the useful flag with other details for the passed
// sessionId
func (s *SessionIdToChatGPTResponse) SetUseful(sessionId string, useful UsefulAnalytics) error {
	response := s.GetResponse(sessionId)
	if response == nil {
		return fmt.Errorf("sessionId is invalid")
	}

	if useful.IsUseful == nil {
		return fmt.Errorf("`useful` is a required key")
	}

	response.sessionDoc.Useful = useful.IsUseful
	response.sessionDoc.Reason = useful.Reason

	if useful.UserID != nil && strings.TrimSpace(*useful.UserID) != "" {
		response.sessionDoc.UserId = useful.UserID
	}

	if useful.Meta != nil {
		response.sessionDoc.Meta = useful.Meta
	}

	// Update the updated_at time value
	currentTime := time.Now().Unix()
	response.sessionDoc.UpdatedAt = &currentTime

	// Trigger ES update as well
	saveErr := Instance().analyticsEs.saveSession(context.Background(), *response.sessionDoc, sessionId)
	if saveErr != nil {
		log.Errorln(logTag, ": error while saving session details in upstream: ", saveErr.Error())
		return saveErr
	}

	return nil
}

// CloneSession will clone the passed sessionId's session and create a copy
// session from it. Returns the new sessionId for the session created from the
// source session.
func (s *SessionIdToChatGPTResponse) CloneSession(oldSessionId string, userId string, usageInBytes []byte) (string, error) {
	// Use the passed sessionId and get the sessionDoc from it.
	olderSessionResponse := s.GetResponse(oldSessionId)
	if olderSessionResponse == nil {
		return "", fmt.Errorf("Error while fetching session from passed sessionId: %s", oldSessionId)
	}

	olderResponseInBytes, marshalErr := json.Marshal(olderSessionResponse)
	if marshalErr != nil {
		return "", marshalErr
	}

	var responseAsType InternalChatGPTResponse
	unmarshalErr := json.Unmarshal(olderResponseInBytes, &responseAsType)
	if unmarshalErr != nil {
		return "", unmarshalErr
	}

	olderSessionDoc := olderSessionResponse.sessionDoc
	olderSessionDocInBytes, marshalErr := json.Marshal(olderSessionDoc)
	if marshalErr != nil {
		return "", marshalErr
	}

	var sessionDoc = new(AISessionDoc)
	unmarshalErr = json.Unmarshal(olderSessionDocInBytes, sessionDoc)
	if unmarshalErr != nil {
		return "", unmarshalErr
	}

	// Register new session ID
	newSessionId := GenerateSessionId()

	// Update sessionDoc values
	defaultReason := ""
	sessionDoc.SessionId = &newSessionId
	sessionDoc.UserId = &userId
	sessionDoc.Useful = nil
	sessionDoc.Reason = &defaultReason

	// Trim the messages to 6
	if len(*sessionDoc.Messages) > 6 {
		trimmedMessages := (*sessionDoc.Messages)[:6]
		sessionDoc.Messages = &trimmedMessages
	}

	// Set invocation count to 1
	defaultInvocationCount := 1
	sessionDoc.InvocationCount = &defaultInvocationCount

	// Set the token values from the response
	//
	// We are doing this because the sessionDoc will increment the
	// output/input tokens with every invocation thus it becomes
	// impossible to find the exact value of input/output tokens on the
	// first call.
	outputTokens, outputTokensParseErr := jsonparser.GetInt(usageInBytes, "completion_tokens")
	if outputTokensParseErr != nil {
		outputTokens = 0
	}
	inputTokens, inputTokensParseErr := jsonparser.GetInt(usageInBytes, "prompt_tokens")
	if inputTokensParseErr != nil {
		inputTokens = 0
	}
	sessionDoc.OutputTokens = &outputTokens
	sessionDoc.InputTokens = &inputTokens

	createdTime := time.Now().Unix()

	// Convert the timestamp to milliseconds since we're using go 1.16
	// and it doesn't have the Milli() method
	timeNow := time.Now().UnixNano() / int64(time.Millisecond)

	sessionDoc.CreatedAt = &createdTime
	sessionDoc.UpdatedAt = &createdTime
	sessionDoc.TimeStamp = &timeNow

	responseAsType.sessionDoc = sessionDoc
	defaultChannel := GetChannelWithSize(2000)
	responseAsType.StreamChannel = &defaultChannel

	// Register the response against the generated sessionId
	s.mu.Lock()
	s.storageMap[newSessionId] = &responseAsType
	s.mu.Unlock()

	// Trigger updating the response in ES
	go func(responseAsType InternalChatGPTResponse, newSessionId string) {
		responseMarshalled, marshalErr := json.Marshal(responseAsType)
		if marshalErr != nil {
			log.Warnln(logTag, ": error while marshalling internal response: ", marshalErr.Error())
		} else {
			marshalledAsStr := string(responseMarshalled)
			responseAsType.sessionDoc.InternalResponse = &marshalledAsStr
		}

		// Update the updated_at time value
		currentTime := time.Now().Unix()
		responseAsType.sessionDoc.UpdatedAt = &currentTime

		// Trigger ES update as well
		saveErr := Instance().analyticsEs.saveSession(context.Background(), *responseAsType.sessionDoc, newSessionId)
		if saveErr != nil {
			log.Errorln(logTag, ": error while saving session details in upstream: ", saveErr.Error())
		}
	}(responseAsType, newSessionId)

	// Update the RemoveAt to be 30 minutes for now
	responseAsType.RemoveAt = time.Now().Add(30 * time.Minute).Unix()

	return newSessionId, nil
}

// SessionFromBytes will create a new session from the passed session
// doc in bytes
func (s *SessionIdToChatGPTResponse) SessionFromBytes(sessionInBytes []byte, userId string) (string, error) {
	var sessionAsType AISessionDoc
	unmarshalErr := json.Unmarshal(sessionInBytes, &sessionAsType)
	if unmarshalErr != nil {
		return "", unmarshalErr
	}

	var responseAsType InternalChatGPTResponse
	unmarshalErr = json.Unmarshal([]byte(*sessionAsType.InternalResponse), &responseAsType)

	if unmarshalErr != nil {
		log.Warnln(logTag, ": ", unmarshalErr.Error())
		return "", nil
	}

	responseAsType.sessionDoc = &sessionAsType
	defaultChannel := GetChannelWithSize(2000)
	responseAsType.StreamChannel = &defaultChannel

	// Update the RemoveAt to be 30 minutes for now
	responseAsType.RemoveAt = time.Now().Add(30 * time.Minute).Unix()

	// Register new session ID
	newSessionId := GenerateSessionId()

	// Update sessionDoc values
	sessionDoc := responseAsType.sessionDoc
	defaultReason := ""
	sessionDoc.SessionId = &newSessionId
	sessionDoc.UserId = &userId
	sessionDoc.Useful = nil
	sessionDoc.Reason = &defaultReason

	responseAsType.sessionDoc = sessionDoc

	// Update the RemoveAt to be 30 minutes for now
	responseAsType.RemoveAt = time.Now().Add(30 * time.Minute).Unix()

	// Register the response against the generated sessionId
	s.mu.Lock()
	s.storageMap[newSessionId] = &responseAsType
	s.mu.Unlock()

	// Trigger updating the response in ES
	go func(responseAsType *InternalChatGPTResponse, newSessionId string) {
		responseMarshalled, marshalErr := json.Marshal(responseAsType)
		if marshalErr != nil {
			log.Warnln(logTag, ": error while marshalling internal response: ", marshalErr.Error())
		} else {
			marshalledAsStr := string(responseMarshalled)
			responseAsType.sessionDoc.InternalResponse = &marshalledAsStr
		}

		// Update the updated_at time value
		currentTime := time.Now().Unix()
		responseAsType.sessionDoc.UpdatedAt = &currentTime

		// Trigger ES update as well
		saveErr := Instance().analyticsEs.saveSession(context.Background(), *responseAsType.sessionDoc, newSessionId)
		if saveErr != nil {
			log.Errorln(logTag, ": error while saving session details in upstream: ", saveErr.Error())
		}
	}(&responseAsType, newSessionId)

	return newSessionId, nil
}

// deleteExpired will delete all the expired responses
func (s *SessionIdToChatGPTResponse) deleteExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	storageMapUpdated := s.storageMap
	for sessionId, response := range s.storageMap {
		// Since the response is not even populated yet, we cannot
		// delete it
		if !response.GetIsReady() {
			continue
		}

		if response.GetRemoveAt() <= time.Now().Unix() {
			// Response is expired and we can remove it.
			log.Debug(logTag, ": removing session details with ID: ", sessionId)
			delete(storageMapUpdated, sessionId)
		}
	}
}

func SessionInstance() *SessionIdToChatGPTResponse {
	sessionOnce.Do(func() {
		sessionSingleton = &SessionIdToChatGPTResponse{
			storageMap: make(map[string]*InternalChatGPTResponse, 0),
		}

		// Start the cronjob to delete expired entries
		expiredSessionRemovalJob := cron.New()
		expiredSessionRemovalJob.AddFunc("@every 1m", func() {
			log.Debug(logTag, ": running delete expired cronjob")
			sessionSingleton.deleteExpired()
		})
		expiredSessionRemovalJob.Start()
	})

	return sessionSingleton
}

// IsOpenAIAllowed will indicate whether on not the plan
// has access to use OpenAI
func IsOpenAIAllowed() bool {
	return util.ValidatePlans(validPlans, util.GetFeatureOpenAI())
}

// IsOpenAIEnabled will indicate whether OpenAI is enabled or not
func (r *OpenAI) IsOpenAIEnabled() bool {
	isEnabled := r.GetConfig().Enable
	if isEnabled == nil {
		return false
	}
	return *isEnabled
}

// IsKeyValid returns a bool indicating whether or not the
// key is valid.
func (o OpenAIConfig) IsKeyValid() bool {
	return o.OpenAIKey != nil && strings.TrimSpace(*o.OpenAIKey) != ""
}

// Key returns the OpenAI key
func (o OpenAIConfig) Key() string {
	if !o.IsKeyValid() {
		return ""
	}

	return *o.OpenAIKey
}

// GetModel will return the model to be used for the OpenAI
// call.
//
// This will prioritize the model that user has configured, if
// any and fallback to `gpt-3.5-turbo`
func (o OpenAIConfig) GetModel() string {
	modelSet := o.Model
	if modelSet == nil {
		return defaultModel
	}
	return *modelSet
}

// GetSystemPrompt will return the system prompt set by the
// user.
//
// This will prioritize the systemPrompt set by the user, if
// present and fall back to `You're a good helpful assistant`
func (o OpenAIConfig) GetSystemPrompt() string {
	promptSet := o.SystemPrompt
	if promptSet == nil {
		return defaultPrompt
	}
	return *promptSet
}

// GetMaxTokens will return the max tokens set by the user
//
// This will prioritize the maxTokens set by the user, if
// present and fall back to `300`
func (o OpenAIConfig) GetMaxTokens() int {
	maxTokensSet := o.MaxTokens
	if maxTokensSet == nil {
		return defaultMaxTokens
	}
	return *maxTokensSet
}

// GetMinTokens will return the min tokens set by the user
//
// This will prioritize minTokens set by the user, if present
// and will fall back to `100`
func (o OpenAIConfig) GetMinTokens() int {
	minTokensSet := o.MinTokens
	if minTokensSet == nil {
		return defaultMinTokens
	}

	return *minTokensSet
}

// IsIndexWhitelisted will indicate whether the passed index is whitelisted by checking
// if the index exists in the whitelisted indexes list
func (o OpenAIConfig) IsIndexWhitelisted(index string) bool {
	wlIndexes := o.Indexes
	if wlIndexes == nil {
		return false
	}

	for _, whiteListedIndex := range *wlIndexes {
		if whiteListedIndex == index {
			return true
		}
	}

	return false
}

// GetAPIType will return the API type specified for the instance
func (o OpenAIConfig) GetAPIType() APIType {
	if o.APIType == nil {
		return OpenAIType
	}

	return *o.APIType
}

// GetAzureURL will return the Azure URL for OpenAI calls
func (o OpenAIConfig) GetAzureURL() string {
	if o.AzureBaseURL == nil {
		return ""
	}

	return *o.AzureBaseURL
}

// GetAzureVersion will return the azure version for OpenAI calls
func (o OpenAIConfig) GetAzureVersion() string {
	if o.AzureVersion == nil {
		return ""
	}

	return *o.AzureVersion
}

// GetDefaultConfig will return the default config for OpenAI
func GetDefaultConfig() OpenAIConfig {
	defaultModel := defaultModel
	defaultEmbeddingModel := TextEmbedding3Small
	defaultPrompt := defaultPrompt
	indexes := make([]string, 0)
	defaultEnabled := false
	defaultMaxTokens := defaultMaxTokens
	defaultMinTokens := defaultMinTokens
	defaultAPIType := OpenAIType
	defaultAzureURL := ""

	return OpenAIConfig{
		Enable:                &defaultEnabled,
		OpenAIKey:             nil,
		SystemPrompt:          &defaultPrompt,
		Model:                 &defaultModel,
		DefaultEmbeddingModel: &defaultEmbeddingModel,
		MaxTokens:             &defaultMaxTokens,
		MinTokens:             &defaultMinTokens,
		Indexes:               &indexes,
		APIType:               &defaultAPIType,
		AzureBaseURL:          &defaultAzureURL,
		AzureVersion:          &defaultAzureURL,
	}
}

type EmbeddingModel int

const (
	TextEmbedding3Small EmbeddingModel = iota
	TextEmbedding3Large
	TextEmbeddingAda002
)

// String is the implementation of stringer interface that returns the
// string representation of EmbeddingModel
func (a EmbeddingModel) String() string {
	return [...]string{
		"text-embedding-3-small",
		"text-embedding-3-large",
		"text-embedding-ada-002",
	}[a]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for
// unmarshalling EmbeddingModel.
func (a *EmbeddingModel) UnmarshalJSON(bytes []byte) error {
	var embeddingModel string
	err := json.Unmarshal(bytes, &embeddingModel)
	if err != nil {
		return err
	}

	switch embeddingModel {
	case TextEmbedding3Small.String():
		*a = TextEmbedding3Small
	case TextEmbedding3Large.String():
		*a = TextEmbedding3Large
	case TextEmbeddingAda002.String():
		*a = TextEmbeddingAda002
	default:
		return fmt.Errorf("invalid text embedding value encountered: %v. Should be one of 'text-embedding-3-small', 'text-embedding-3-large', 'text-embedding-ada-002'", embeddingModel)
	}

	return nil
}

// MarshalJSON is the implementation of the Marshaler interface
// for marshaling EmbeddingModel
func (a EmbeddingModel) MarshalJSON() ([]byte, error) {
	var embeddingModel string
	switch a {
	case TextEmbedding3Small:
		embeddingModel = TextEmbedding3Small.String()
	case TextEmbedding3Large:
		embeddingModel = TextEmbedding3Large.String()
	case TextEmbeddingAda002:
		embeddingModel = TextEmbeddingAda002.String()
	default:
		return nil, fmt.Errorf("invalid text embedding encountered: %v. Should be one of 'text-embedding-3-small', 'text-embedding-3-large', 'text-embedding-ada-002'", a)
	}

	return json.Marshal(embeddingModel)
}

type APIType int

const (
	OpenAIType APIType = iota
	AzureType
)

// String is the implementation of stringer interface that returns the
// string representation of APIType
func (a APIType) String() string {
	return [...]string{
		"openai",
		"azure",
	}[a]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for
// unmarshalling APIType.
func (a *APIType) UnmarshalJSON(bytes []byte) error {
	var apiType string
	err := json.Unmarshal(bytes, &apiType)
	if err != nil {
		return err
	}

	switch apiType {
	case OpenAIType.String():
		*a = OpenAIType
	case AzureType.String():
		*a = AzureType
	default:
		return fmt.Errorf("invalid api type encountered: %v", apiType)
	}

	return nil
}

// MarshalJSON is the implementation of the Marshaler interface
// for marshaling APIType
func (a APIType) MarshalJSON() ([]byte, error) {
	var apiType string
	switch a {
	case OpenAIType:
		apiType = OpenAIType.String()
	case AzureType:
		apiType = AzureType.String()
	default:
		return nil, fmt.Errorf("invalid api type encountered: %v", a)
	}

	return json.Marshal(apiType)
}

// ConfigIn will contain details of the configuration
// passed by the user
type ConfigDetails struct {
	Enable                *bool           `json:"enable,omitempty"`
	ApiKey                *string         `json:"apiKey,omitempty"`
	DefaultModel          *string         `json:"defaultModel,omitempty"`
	DefaultEmbeddingModel *EmbeddingModel `json:"defaultEmbeddingModel,omitempty"`
	DefaultSystemPrompt   *string         `json:"defaultSystemPrompt,omitempty"`
	DefaultMaxTokens      *int            `json:"defaultMaxTokens,omitempty"`
	DefaultMinTokens      *int            `json:"defaultMinTokens,omitempty"`
	EnabledIndexes        *[]string       `json:"enabledIndexes,omitempty"`
	APIType               *APIType        `json:"apiType,omitempty"`
	AzureBaseURL          *string         `json:"azureBaseURL,omitempty"`
	AzureVersion          *string         `json:"apiVersion,omitempty"`
}

// ToInternalConfig will return the config details in the internal
// config structure
func (c ConfigDetails) ToInternalConfig() OpenAIConfig {
	if c.Enable == nil {
		defaultEnable := false
		c.Enable = &defaultEnable
	}

	if c.ApiKey == nil {
		defaultAPIKey := ""
		c.ApiKey = &defaultAPIKey
	}

	if c.DefaultModel == nil {
		defaultModelStr := defaultModel
		c.DefaultModel = &defaultModelStr
	}

	if c.DefaultEmbeddingModel == nil {
		defaultEmbeddingModel := TextEmbedding3Small
		c.DefaultEmbeddingModel = &defaultEmbeddingModel
	}

	if c.DefaultSystemPrompt == nil {
		defaultPromptStr := defaultPrompt
		c.DefaultSystemPrompt = &defaultPromptStr
	}

	if c.DefaultMaxTokens == nil {
		defaultMaxTokens := 300
		c.DefaultMaxTokens = &defaultMaxTokens
	}

	if c.DefaultMinTokens == nil {
		defaultMinTokens := 100
		c.DefaultMinTokens = &defaultMinTokens
	}

	if c.EnabledIndexes == nil {
		defaultIndexes := make([]string, 0)
		c.EnabledIndexes = &defaultIndexes
	}

	if c.APIType == nil {
		defaultAPIType := OpenAIType
		c.APIType = &defaultAPIType
	}

	if *c.APIType != AzureType {
		c.AzureBaseURL = nil
		c.AzureVersion = nil
	}

	return OpenAIConfig{
		Enable:                c.Enable,
		OpenAIKey:             c.ApiKey,
		Model:                 c.DefaultModel,
		DefaultEmbeddingModel: c.DefaultEmbeddingModel,
		SystemPrompt:          c.DefaultSystemPrompt,
		MaxTokens:             c.DefaultMaxTokens,
		MinTokens:             c.DefaultMinTokens,
		Indexes:               c.EnabledIndexes,
		APIType:               c.APIType,
		AzureBaseURL:          c.AzureBaseURL,
		AzureVersion:          c.AzureVersion,
	}
}

// ToExternalConfig will return the config details in the external
// config structure
func (o OpenAIConfig) ToExternalConfig() ConfigDetails {
	return ConfigDetails{
		Enable:                o.Enable,
		ApiKey:                o.OpenAIKey,
		DefaultModel:          o.Model,
		DefaultEmbeddingModel: o.DefaultEmbeddingModel,
		DefaultSystemPrompt:   o.SystemPrompt,
		DefaultMaxTokens:      o.MaxTokens,
		DefaultMinTokens:      o.MinTokens,
		EnabledIndexes:        o.Indexes,
		APIType:               o.APIType,
		AzureBaseURL:          o.AzureBaseURL,
		AzureVersion:          o.AzureVersion,
	}
}

// ValidateConfigPassed will validate the passed config
// body and accordingly throw errors, if any
func ValidateConfigPassed(config ConfigDetails) (ConfigDetails, error) {
	// If enable is false, then we don't need to verify anything else
	if config.Enable != nil && !*config.Enable {
		defaultEnable := false
		config.Enable = &defaultEnable
		return config, nil
	}

	if config.ApiKey == nil {
		return config, fmt.Errorf("`apiKey` is necessary when enabling OpenAI")
	}

	if config.DefaultModel == nil {
		defaultModelAsStr := defaultModel
		config.DefaultModel = &defaultModelAsStr
	}

	if config.DefaultEmbeddingModel == nil {
		defaultEmbeddingModel := TextEmbedding3Small
		config.DefaultEmbeddingModel = &defaultEmbeddingModel
	}

	if config.DefaultSystemPrompt == nil {
		defaultPromptAsStr := defaultPrompt
		config.DefaultSystemPrompt = &defaultPromptAsStr
	}

	if config.DefaultMaxTokens == nil {
		defaultMaxTokens := 300
		config.DefaultMaxTokens = &defaultMaxTokens
	}

	if config.DefaultMinTokens == nil {
		defaultMinTokens := 100
		config.DefaultMinTokens = &defaultMinTokens
	}

	// Validate that the `maxTokens` value is not above
	// half of the maximum limit of the model.
	limitForModel := LimitForModel(*config.DefaultModel)
	if limitForModel == -1 {
		limitForModel = 2048
	}
	allowedMaxTokensLimit := limitForModel / 2

	if *config.DefaultMaxTokens > allowedMaxTokensLimit {
		return config, fmt.Errorf("`maxTokens` cannot exceed the upper limit of '%d' which is half of the model's maximum limit.", allowedMaxTokensLimit)
	}

	// Validate that the `minTokens` value is not lower than 100
	if *config.DefaultMinTokens < 100 {
		return config, fmt.Errorf("`minTokens` cannot be lower than 100")
	}

	// Validate that the `minTokens` value is not greater than maxTokens
	if *config.DefaultMinTokens > *config.DefaultMaxTokens {
		return config, fmt.Errorf("`minTokens` cannot be greater than `maxTokens`")
	}

	if config.EnabledIndexes == nil {
		enabledIndexes := make([]string, 0)
		config.EnabledIndexes = &enabledIndexes
	}

	// Set the default API type as OpenAI
	if config.APIType == nil {
		defaultAPIType := OpenAIType
		config.APIType = &defaultAPIType
	}

	// If the API type is OpenAI, we don't need to validate anything more.
	if *config.APIType == OpenAIType {
		return config, nil
	}

	// Verify that AzureURL is passed
	if config.AzureBaseURL == nil || strings.TrimSpace(*config.AzureBaseURL) == "" {
		return config, fmt.Errorf("`azureBaseURL` is a required value when API type is `azure`")
	}

	// Verify the API version for Azure
	if config.AzureVersion == nil || strings.TrimSpace(*config.AzureVersion) == "" {
		return config, fmt.Errorf("`apiVersion` is a required value when API type is `azure`")
	}

	return config, nil
}

// BuildChatGPTBody will build the chatGPT body that will be sent to the API
func BuildChatGPTBody(model string, messages []map[string]interface{}, maxTokens *int, temperature *float64, isStream bool, apiType APIType) map[string]interface{} {
	requestBodyAsMap := map[string]interface{}{
		"messages": messages,
	}

	if apiType != AzureType {
		requestBodyAsMap["model"] = model
	}

	// If maxTokens is passed, inject it in the request body
	if maxTokens != nil {
		requestBodyAsMap["max_tokens"] = *maxTokens
	}

	// If temperature is passed, inject it in the request body
	if temperature != nil {
		requestBodyAsMap["temperature"] = *temperature
	}

	if isStream {
		requestBodyAsMap["stream"] = true
	}

	return requestBodyAsMap
}

// MakeChatGPTRequest will make the ChatGPT request and return the response accordingly
func MakeChatGPTRequest(
	model string,
	messages []map[string]interface{},
	apiKey string,
	maxTokens *int,
	temperature *float64,
	apiType APIType,
	azureUrl string,
	azureVersion string,
) (*http.Response, []byte, []byte, int64, *http.Request, error) {
	// Build the request to send to ChatGPT API
	URLToHit := BuildOpenAIURL(apiType, azureUrl, model, azureVersion)

	requestBodyAsMap := BuildChatGPTBody(model, messages, maxTokens, temperature, false, apiType)

	bodyAsBytes, bodyMarshalErr := json.Marshal(requestBodyAsMap)
	if bodyMarshalErr != nil {
		return nil, nil, nil, 0, nil, fmt.Errorf("error while marshalling request body into bytes: %s", bodyMarshalErr.Error())
	}

	request, requestCreateErr := http.NewRequest(http.MethodPost, URLToHit, bytes.NewReader(bodyAsBytes))

	if requestCreateErr != nil {
		errMsg := fmt.Sprint("error while creating the request to send OpenAI, ", requestCreateErr)
		log.Errorln(logTag, ": ", errMsg)

		return nil, nil, bodyAsBytes, 0, nil, fmt.Errorf(errMsg)
	}

	// Set the authorization header
	// Set the authorization header
	if apiType == OpenAIType {
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	} else if apiType == AzureType {
		request.Header.Add("api-key", apiKey)
	}
	request.Header.Add("Content-Type", "application/json")

	response, reqErr := util.HTTPClient().Do(request)
	resolvedAt := time.Now().Unix()
	if reqErr != nil {
		errMsg := fmt.Sprint("error while sending request to ChatGPT, ", reqErr)
		log.Warnln(logTag, ": ", errMsg)

		return nil, nil, bodyAsBytes, resolvedAt, request, fmt.Errorf(errMsg)
	}

	// Read the body.
	responseInBytes, readErr := ioutil.ReadAll(response.Body)
	if readErr != nil {
		errMsg := fmt.Sprint("error while reading the response body from ChatGPT, ", readErr)
		log.Warnln(logTag, ": ", errMsg)

		return nil, nil, bodyAsBytes, resolvedAt, request, fmt.Errorf(errMsg)
	}

	// Verify the status code received, a non 200 OK status code will return an
	// error since it means the embedding failed
	if response.StatusCode != http.StatusOK {
		return nil, responseInBytes, bodyAsBytes, resolvedAt, request, fmt.Errorf("non 200 OK status code received from ChatGPT: %s", response.Status)
	}

	return response, responseInBytes, bodyAsBytes, resolvedAt, request, nil
}

// Build the OpenAI API URL based on the api type.
func BuildOpenAIURL(apiType APIType, azureUrl string, model string, azureVersion string) string {
	// Build the request to send to ChatGPT API
	endpointToHit := "/chat/completions"

	URLToHit := OpenAIAPIURL + endpointToHit
	if apiType == AzureType {
		URLToHit = fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s", azureUrl, model, azureVersion)
	}

	return URLToHit
}

// MakeChatGPTRequestWithStream will make the ChatGPT request but stream it so that each
// chunk is written to the passed channel until the `[DONE]` text is received.
func MakeChatGPTRequestWithStream(
	model string,
	messages []map[string]interface{},
	apiKey string,
	maxTokens *int,
	temperature *float64,
	responseChan *chan []byte,
	apiType APIType,
	azureUrl string,
	azureVersion string,
) ([][]byte, []byte, int64, int64, int64, *http.Request, error) {
	URLToHit := BuildOpenAIURL(apiType, azureUrl, model, azureVersion)

	log.Debug(logTag, ": Hitting OpenAI URL: ", URLToHit)

	requestBodyAsMap := BuildChatGPTBody(model, messages, maxTokens, temperature, true, apiType)

	bodyAsBytes, bodyMarshalErr := json.Marshal(requestBodyAsMap)
	if bodyMarshalErr != nil {
		return nil, nil, 0, 0, 0, nil, fmt.Errorf("error while marshalling request body into bytes: %s", bodyMarshalErr.Error())
	}

	request, requestCreateErr := http.NewRequest(http.MethodPost, URLToHit, bytes.NewReader(bodyAsBytes))

	if requestCreateErr != nil {
		errMsg := fmt.Sprint("error while creating the request to send OpenAI, ", requestCreateErr)
		log.Errorln(logTag, ": ", errMsg)

		return nil, bodyAsBytes, 0, 0, 0, nil, fmt.Errorf(errMsg)
	}

	responseDataToReturn := make([][]byte, 0)

	// Set the authorization header
	if apiType == OpenAIType {
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	} else if apiType == AzureType {
		request.Header.Add("api-key", apiKey)
	}

	request.Header.Add("Content-Type", "application/json")

	// Set SSE specific headers
	request.Header.Set("Cache-Control", "no-cache")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Connection", "keep-alive")

	response, reqErr := util.HTTPClient().Do(request)
	resolvedAt := time.Now().Unix()
	if reqErr != nil {
		errMsg := fmt.Sprint("error while sending request to ChatGPT, ", reqErr)
		log.Warnln(logTag, ": ", errMsg)

		return nil, bodyAsBytes, resolvedAt, 0, 0, request, fmt.Errorf(errMsg)
	}

	defer response.Body.Close()

	// Create a buffer to store a copy of the response body
	var buf bytes.Buffer
	tee := io.TeeReader(response.Body, &buf)

	// Read the response body
	body, readErr := ioutil.ReadAll(tee)
	if readErr != nil {
		log.Fatalf("Error reading response body: %v", readErr)
	}
	value, dataType, _, chatGPTResponseErr := jsonparser.Get(body, "error")
	if chatGPTResponseErr != nil && dataType != jsonparser.NotExist {
		// If there's an error parsing, return the error
		log.Warnln(logTag, ": error while parsing response body: ", chatGPTResponseErr)
	}
	if dataType == jsonparser.Object {
		return nil, body, resolvedAt, 0, 0, request, fmt.Errorf("%s", string(value))
	}

	// Restore the response body so it can be read again
	response.Body = ioutil.NopCloser(&buf)
	// NOTE: If the body is read for debugging purposes, ensure
	// that the body is passed on in the following as go response
	// bodies are read once only so by the time the chunks are actually
	// read below, the body will be empty.

	var firstByteRead int64 = 0
	var streamClosedAt int64 = 0

	for {
		data := make([]byte, 1024)
		_, err := response.Body.Read(data)
		if err != nil {
			// End of file reached, exit the loop gracefully
			if err == io.EOF {
				break
			}

			return nil, bodyAsBytes, resolvedAt, 0, 0, request, fmt.Errorf("error while reading chunk: %s", err.Error())
		}

		if firstByteRead == 0 {
			firstByteRead = time.Now().Unix()
		}

		*responseChan <- data
		responseDataToReturn = append(responseDataToReturn, data)

		if strings.Contains(string(data), "[DONE]") {
			streamClosedAt = time.Now().Unix()
			break
		}
	}

	return responseDataToReturn, bodyAsBytes, resolvedAt, firstByteRead, streamClosedAt, request, nil
}

// FetchChatGPTForSession will fetch the ChatGPT response and
// accordingly update it in the session map
func FetchChatGPTForSession(
	sessionId string,
	sessionMap *SessionIdToChatGPTResponse,
	modelUsed string,
	messages []map[string]interface{},
	DocumentIds []string,
	apiKey string,
	maxTokens *int,
	temperature *float64,
	apiType APIType,
	azureUrl string,
	azureVersion string,
	skipStoringFailedResponse bool,
) error {
	// Make ChatGPT request
	_, responseInBytes, requestInBytes, resolvedAt, httpReq, chatGPTErr := MakeChatGPTRequest(modelUsed, messages, apiKey, maxTokens, temperature, apiType, azureUrl, azureVersion)
	if chatGPTErr != nil {
		// Create the error message that is to be returned
		errAsStr := chatGPTErr.Error()
		errMsg := fmt.Errorf("error while sending request to chatGPT: %s", errAsStr)
		log.Warnln(logTag, ": ", errMsg)

		// If storing the failed response is skipped, we will return the error right away
		// and not do anything.
		if skipStoringFailedResponse {
			return fmt.Errorf(`{"error": %s}`, responseInBytes)
		}

		// We will need to store the error in a way that it can be returned to the user directly.
		responseToStoreInBytes := fmt.Sprintf(`{"error": "%s"}`, chatGPTErr.Error())

		// If response is valid, we will need to store it accordingly.
		if responseInBytes != nil {
			updatedResponse, setErr := jsonparser.Set([]byte(responseToStoreInBytes), responseInBytes, "response")
			if setErr != nil {
				log.Warnln(logTag, ": error while setting response in body to store: ", setErr.Error())
			} else {
				responseToStoreInBytes = string(updatedResponse)
			}
		}

		// If request is valid, store it against the sessionId
		if requestInBytes != nil {
			sessionMap.SetRequest(sessionId, requestInBytes, len(messages)-1)
		}

		// Finally, store the error response
		sessionMap.AddResponse(sessionId, []byte(responseToStoreInBytes))

		// SetIsFailed for the session since the request failed
		sessionMap.SetIsFailed(sessionId, true)

		// Set the resolved time
		sessionMap.SetResolvedAt(sessionId, resolvedAt, httpReq, 0, 0, requestInBytes)

		return errMsg
	}

	// Set the request body for the session
	sessionMap.SetRequest(sessionId, requestInBytes, len(messages))
	sessionMap.SetResolvedAt(sessionId, resolvedAt, httpReq, 0, 0, requestInBytes)

	// Inject the ID's sent into the ChatGPT response
	documentIdsAsBytes, marshalErr := json.Marshal(DocumentIds)
	if marshalErr != nil {
		errMsg := fmt.Errorf("error while marshalling sentID's into bytes to inject into ChatGPT response: %s", marshalErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return errMsg
	}

	var idsSetErr error
	responseInBytes, idsSetErr = jsonparser.Set(responseInBytes, documentIdsAsBytes, "documentIds")
	if idsSetErr != nil {
		log.Warnln(logTag, ": error while injecting ids sent into chatGPT response")
		return idsSetErr
	}

	// Set the response in the sessionMap
	sessionMap.AddResponse(sessionId, responseInBytes)

	// Once the response resolves, it will be automatically updated by the cache itself.
	return nil
}

// Parse the streamed response received from GPT 4 or newer API models
func ParseGPT4Responses(responsesArr [][]byte) (map[string]interface{}, string, *error) {
	isModelExtracted := false
	responseAsMap := make(map[string]interface{})
	completeText := ""

	combinedText := ""

	// We will split the responses bytes by the newlines.
	// Then we will join them with an empty string to get a complete
	// string which will then be splitted using the `data: ` separator
	// to get a list of parse-able stringified JSON bodies.
	for _, val := range responsesArr {
		responseInStr := string(val)
		splittedResponse := strings.Split(responseInStr, "\n\n")

		// Join the splitted response and then break it down using `data: `
		// as a separator.
		combinedText += strings.Join(splittedResponse, "")
	}

	textSplittedByData := strings.Split(combinedText, "data: ")

	// Iterate through the splitted JSON bodies.
	for position, valEach := range textSplittedByData {
		// Skip the value if it is an empty string
		if valEach == "" {
			continue
		}

		if position == len(textSplittedByData)-1 {
			// Last one, so probably empty, skip it!
			continue
		}

		if !isModelExtracted {
			// This is the first one that we can skip.

			// Extract the model
			modelAsStr, extractErr := jsonparser.GetString([]byte(valEach), "model")
			if extractErr != nil {
				errMsg := fmt.Errorf("error while extracting model: %s", extractErr.Error())
				log.Warnln(logTag, ": ", errMsg)
				return responseAsMap, completeText, &errMsg
			}

			responseAsMap["model"] = modelAsStr
			isModelExtracted = true
		}

		contentEach, extractErr := jsonparser.GetString([]byte(valEach), "choices", "[0]", "delta", "content")
		if extractErr != nil {
			errMsg := fmt.Errorf("error while extracting value: %s", extractErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			continue
		}

		completeText += contentEach
	}

	return responseAsMap, completeText, nil
}

// IsValidJSON checks if a string is a valid JSON.
func IsValidJSON(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}

// FetchChatGPTForSessionWithStream will fetch the ChatGPT response with streaming
// enabled.
func FetchChatGPTForSessionWithStream(
	sessionId string,
	sessionMap *SessionIdToChatGPTResponse,
	modelUsed string,
	messages []map[string]interface{},
	DocumentIds []string,
	apiKey string,
	maxTokens *int,
	temperature *float64,
	apiType APIType,
	azureURL string,
	azureVersion string,
	skipStoringFailedResponse bool,
) error {
	// Create the channel and set it for the passed sessionId
	responseChannel := sessionMap.GetChannel(sessionId)
	sessionMap.SetIsStreaming(sessionId, true)

	// Make ChatGPT request
	responsesArr, requestInBytes, resolvedAt, firstByteRead, streamClosedAt, httpReq, chatGPTErr := MakeChatGPTRequestWithStream(
		modelUsed, messages, apiKey, maxTokens, temperature, responseChannel, apiType, azureURL, azureVersion,
	)

	if chatGPTErr != nil {
		// We will need to store the error in a way that it can be returned to the user directly.
		errorStr := fmt.Sprintf(`{"error": "%s"}`, chatGPTErr.Error())
		if IsValidJSON(chatGPTErr.Error()) {
			errorStr = fmt.Sprintf(`{"error": %s}`, chatGPTErr.Error())
		}

		errMsg := fmt.Errorf("error while sending request to chatGPT: %s", chatGPTErr.Error())
		log.Warnln(logTag, ": ", errMsg)

		if skipStoringFailedResponse {
			// Set the streaming as false.
			sessionMap.SetIsStreaming(sessionId, false)
			return fmt.Errorf(errorStr)
		}

		responseToStoreInBytes := errorStr

		// SetIsFailed for the session since the request failed
		sessionMap.SetIsFailed(sessionId, true)

		// If request is valid, store it against the sessionId
		if requestInBytes != nil {
			sessionMap.SetRequest(sessionId, requestInBytes, len(messages)-1)
		}

		// Finally, store the error response
		sessionMap.AddResponse(sessionId, []byte(responseToStoreInBytes))

		// Set the resolved time
		sessionMap.SetResolvedAt(sessionId, resolvedAt, httpReq, firstByteRead, streamClosedAt, requestInBytes)

		return errMsg
	}

	// Set the request body for the session
	sessionMap.SetRequest(sessionId, requestInBytes, len(messages))

	// Set the resolved time
	sessionMap.SetResolvedAt(sessionId, resolvedAt, httpReq, firstByteRead, streamClosedAt, requestInBytes)

	// Inject the ID's sent into the ChatGPT response
	documentIdsAsBytes, marshalErr := json.Marshal(DocumentIds)
	if marshalErr != nil {
		errMsg := fmt.Errorf("error while marshalling sentID's into bytes to inject into ChatGPT response: %s", marshalErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return errMsg
	}

	// Extract all the details from the channel and build a response structure
	//
	// We won't have access to the prompt tokens here.

	responseAsMap := make(map[string]interface{})

	completeText := ""
	isModelExtracted := false

	if !strings.Contains(strings.ToLower(modelUsed), "gpt-3") {
		var parseErr *error
		responseAsMap, completeText, parseErr = ParseGPT4Responses(responsesArr)
		if parseErr != nil {
			return fmt.Errorf("Error while parsing azure OpenAI response: %s", (*parseErr).Error())
		}
	} else {
		for _, val := range responsesArr {
			val = bytes.Replace(val, []byte("data: "), []byte(""), -1)

			responseInStr := string(val)
			splittedResponse := strings.Split(responseInStr, "\n\n")

			for position, valEach := range splittedResponse {
				if position == len(splittedResponse)-1 {
					// Last one, so probably empty, skip it!
					continue
				}

				if !isModelExtracted {
					// This is the first one that we can skip.

					// Extract the model
					modelAsStr, extractErr := jsonparser.GetString([]byte(valEach), "model")
					if extractErr != nil {
						errMsg := fmt.Errorf("error while extracting model: %s", extractErr.Error())
						log.Warnln(logTag, ": ", errMsg)
						return errMsg
					}

					responseAsMap["model"] = modelAsStr
					isModelExtracted = true
				}

				log.Debug(logTag, ": val each: ", valEach)

				contentEach, extractErr := jsonparser.GetString([]byte(valEach), "choices", "[0]", "delta", "content")
				if extractErr != nil {
					errMsg := fmt.Errorf("error while extracting value: %s", extractErr.Error())
					log.Warnln(logTag, ": ", errMsg)
					continue
				}

				log.Debug(logTag, ": content: ", contentEach)
				completeText += contentEach
			}
		}
	}

	responseAsMap["choices"] = []map[string]ChatGPTMessage{
		{
			"message": {
				Role:    "assistant",
				Content: completeText,
			},
		},
	}

	// Calculate the token count based on the messages

	messagesAsType := make([]ChatGPTMessage, 0)
	for _, msgEach := range messages {
		msgEachType := ChatGPTMessage{
			Role:    msgEach["role"].(string),
			Content: msgEach["content"].(string),
		}
		messagesAsType = append(messagesAsType, msgEachType)
	}

	promptTokenCount := CheckLimit(messagesAsType, modelUsed)
	completionCount, calculateErr := CalculateTokens(completeText, modelUsed)
	if calculateErr != nil {
		log.Warnln(logTag, ": error while calculating tokens for message: ", completeText, ", with err: ", calculateErr.Error())
		completionCount = 0
	}

	responseAsMap["usage"] = map[string]interface{}{
		"prompt_tokens":     promptTokenCount,
		"completion_tokens": completionCount,
		"total_tokens":      0,
	}

	// convert the response into bytes
	responseInBytes, marshalErr := json.Marshal(responseAsMap)
	if marshalErr != nil {
		errMsg := fmt.Sprint("error while marshalling response into bytes: ", marshalErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return fmt.Errorf(errMsg)
	}

	var idsSetErr error
	responseInBytes, idsSetErr = jsonparser.Set(responseInBytes, documentIdsAsBytes, "documentIds")
	if idsSetErr != nil {
		log.Warnln(logTag, ": error while injecting ids sent into chatGPT response")
		return idsSetErr
	}

	// Set the response in the sessionMap
	sessionMap.AddResponse(sessionId, responseInBytes)

	// Once the response resolves, it will be automatically updated by the cache itself.
	return nil
}

// FetchFromOldSession will use the older session Id to pull the older
// request body and make a new call with those details.
//
// This function should be used in the cache of a partial cache hit where
// there is a cache hit for RS but the OpenAI part cache is not updated
// (because of OpenAI call failing or any other reason).
func FetchFromOldSession(oldSessionId string, sessionMap *SessionIdToChatGPTResponse) (string, error) {
	// Use the older sessionId to get the request details
	olderResponse := sessionMap.GetResponse(oldSessionId)
	olderRequestInBytes := olderResponse.Request().InBytes
	olderRequestAsStruct := olderResponse.Request().AsStruct

	// Extract the messages sent in the older sessionId
	olderMessages, _, _, messageFetchErr := jsonparser.Get(olderRequestInBytes, "messages")
	if messageFetchErr != nil {
		if messageFetchErr == jsonparser.KeyPathNotFoundError {
			return "", fmt.Errorf("error while finding messages from older session!")
		}

		return "", fmt.Errorf("Error extracting older messages from sessionId: %s", messageFetchErr.Error())
	}

	// Unmarshal the older messages into an array of interface
	olderMessagesAsArr := make([]map[string]interface{}, 0)
	unmarshalErr := json.Unmarshal(olderMessages, &olderMessagesAsArr)
	if unmarshalErr != nil {
		errMsg := fmt.Sprint("Error while unmarshalling older messages into array of map: ", unmarshalErr.Error())
		return "", fmt.Errorf(errMsg)
	}

	openAIInstance := Instance()

	newSessionId := GenerateSessionId()

	// Register the sessionId
	// Register the response using the passed sessionId
	sessionMap.RegisterResponse(newSessionId, *olderResponse.sessionDoc.UserId, olderResponse.sessionDoc.Index, olderResponse.sessionDoc.DocumentIds)

	go func(newSessionId string, sessionMap *SessionIdToChatGPTResponse, openAIInstance *OpenAI, olderMessagesAsArr []map[string]interface{}, olderResponse *InternalChatGPTResponse, olderRequestAsStruct ChatGPTRequest) {
		FetchChatGPTForSessionWithStream(
			newSessionId,
			sessionMap,
			openAIInstance.GetConfig().GetModel(),
			olderMessagesAsArr,
			olderResponse.sessionDoc.DocumentIds,
			openAIInstance.GetConfig().Key(),
			olderRequestAsStruct.MaxTokens,
			olderRequestAsStruct.Temperature,
			openAIInstance.GetConfig().GetAPIType(),
			openAIInstance.GetConfig().GetAzureURL(),
			openAIInstance.GetConfig().GetAzureVersion(),
			false,
		)
	}(newSessionId, sessionMap, openAIInstance, olderMessagesAsArr, olderResponse, olderRequestAsStruct)

	return newSessionId, nil
}

// FollowUpRequest will accept the follow-up request for
// the AI Answer flow
type FollowUpRequest struct {
	Request  *ChatGPTRequest `json:"request,omitempty"`
	Question *string         `json:"question,omitempty"`
}

// BuildFollowUpBody will build the follow-up body for the
// response passed
func BuildFollowUpBody(response *InternalChatGPTResponse, followUp FollowUpRequest) (*ChatGPTRequest, error) {
	// Check if the follow-up request has the ChatGPT request
	var olderRequest *ChatGPTRequest
	if followUp.Request != nil {
		olderRequest = followUp.Request
	}

	// Whether the sessionId passed is valid or not
	isSessionValid := response != nil

	if isSessionValid && olderRequest == nil {
		// Try to extract the older request
		// Check if the older request is ready for usage
		if !response.Request().IsStructReady() {
			errMsg := fmt.Sprint("older ChatGPT request is not usable with error: ", response.Request().Error())
			return nil, fmt.Errorf(errMsg)
		}

		olderRequestBody := response.Request().Struct()
		olderRequest = &olderRequestBody
	}

	startTrimmingAt := -1

	// If sessionId is valid, we will need to append the last response, else we
	// expect this to be taken care of in the request body
	// when passed.
	if isSessionValid {
		// Add the latest response as well as the new question to the messages array.
		latestResponseMessage, _, _, getErr := jsonparser.Get(response.Response(), "choices", "[0]", "message")
		if getErr != nil {
			errMsg := fmt.Sprint("error while trying to extract last response's message: ", getErr.Error())
			return nil, fmt.Errorf(errMsg)
		}

		// Unmarshal the response in message into a message object
		var lastMessage ChatGPTMessage
		unmarshalErr := json.Unmarshal(latestResponseMessage, &lastMessage)
		if unmarshalErr != nil {
			errMsg := fmt.Errorf("error while unmarshalling last response into a message structure: %s", unmarshalErr.Error())
			return nil, errMsg
		}

		// Append the last message now
		olderRequest.Messages = append(olderRequest.Messages, lastMessage)

		// Set the trimming position from the response details
		startTrimmingAt = response.TrimAt()
	}

	// Finally add the follow-up question
	olderRequest.Messages = append(olderRequest.Messages, ChatGPTMessage{Role: "user", Content: *followUp.Question})

	// Check if trimming is required based on the messages array and accordingly try to
	// trim the message
	totalTokens := CheckLimit(olderRequest.Messages, olderRequest.Model)
	allowedTokens := LimitForModel(olderRequest.Model)

	// We keep a buffer of 100 to make sure the prompt can be returned within the token limit
	allowedTokens -= 100

	// If trimming is not required, return right away
	if totalTokens <= allowedTokens {
		return olderRequest, nil
	}

	// We will need to trim the messages, we can try it at-least

	// If the trim position is not known yet, we will need to determine it ourselves
	if startTrimmingAt == -1 {
		trimPosition := FindTrimPosition(olderRequest.Messages)
		// This is fatal since we cannot trim at all, we will have to ignore
		// this and not try to try anymore.
		//
		// However, the TrimMessagesAsPerModel function will pick this up and throw
		// an error anyway, so we don't need to do anything here.
		startTrimmingAt = trimPosition
	}

	// We can try to trim now
	trimmedMessages, trimErr := TrimMessagesAsPerModel(olderRequest.Messages, olderRequest.Model, startTrimmingAt)

	// If there is an error with trimming, report that to stderr and skip the trimming step.
	if trimErr != nil {
		errMsg := fmt.Sprint("error while trying to trim messages: ", trimErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return olderRequest, nil
	}

	// Since trimming was successful, we can update the older request messages
	olderRequest.Messages = trimmedMessages
	return olderRequest, nil
}

// SessionContext will accept the session context for a new
// session initiation
type SessionContext struct {
	Question     *string           `json:"question,omitempty"`
	OlderContext *[]ChatGPTMessage `json:"olderContext,omitempty"`
	MaxTokens    *int              `json:"maxTokens,omitempty"`
	Temperature  *float64          `json:"temperature,omitempty"`
	Model        *string           `json:"model,omitempty"`
}

// FindTrimPosition will iterate the messages array and find the position from where
// trimming can start. Essentially this will be the first message object that has the role
// as `user` indicating we can start trimming from the first question that user asked to ChatGPT
func FindTrimPosition(messages []ChatGPTMessage) int {
	for position, messageCtx := range messages {
		if messageCtx.Role == "user" {
			return position
		}
	}

	// We were not able to figure out where to trim
	return -1
}

// GetChannelWithSize will return a channel of the specified
// size. If size is passed as 0, it will return an
// un-buffered channel
func GetChannelWithSize(size int) chan []byte {
	if size == 0 {
		return make(chan []byte)
	}

	return make(chan []byte, size)
}

// GenerateSessionId will generate a sessionId for the ChatGPT request
func GenerateSessionId() string {
	return shortuuid.New()
}

// PingChatGPTWithDetails will make a ping call to chatGPT based on the
// passed details to check whether the configuration is good to go.
func PingChatGPTWithDetails(
	model string,
	apiKey string,
	maxTokens *int,
	temperature *float64,
	apiType APIType,
	azureUrl string,
	azureVersion string,
	systemPrompt string,
) *util.Error {
	messagesArrToPassChatGPT := make([]map[string]interface{}, 0)
	messagesArrToPassChatGPT = append(messagesArrToPassChatGPT, map[string]interface{}{
		"role":    "system",
		"content": systemPrompt,
	})
	messagesArrToPassChatGPT = append(messagesArrToPassChatGPT, map[string]interface{}{
		"role":    "user",
		"content": "Ping. If you got the ping, reply with pong.",
	})

	// Build the messages body.
	_, responseInBytes, _, _, _, chatGPTErr := MakeChatGPTRequest(
		model,
		messagesArrToPassChatGPT,
		apiKey,
		maxTokens,
		temperature,
		apiType,
		azureUrl,
		azureVersion,
	)

	if chatGPTErr != nil {
		return &util.Error{
			Err:     chatGPTErr,
			Message: fmt.Sprintf("Error received while making request: %s", chatGPTErr.Error()),
		}
	}

	// Check the response and accordingly determine whether it was successful or not.
	pingResponse, getErr := jsonparser.GetString(responseInBytes, "choices", "[0]", "message", "content")
	if getErr != nil {
		return &util.Error{
			Err:     getErr,
			Message: "Failed to parse response received from upstream",
		}
	}

	if strings.Contains(strings.ToLower(pingResponse), "pong") {
		return nil
	}

	return &util.Error{
		Err:     fmt.Errorf("`pong` not found in response message: %s", pingResponse),
		Message: "Did not find `pong` in the response message",
	}
}
