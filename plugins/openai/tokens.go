package openai

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tiktoken-go/tokenizer"
)

// These are prefix-matched, so set the longer model name before the shorter one
var MODEL_LIMIT_MAP = map[string]int{
	// Modern GPT-4 variants
	"gpt-4.1-mini":      128000,
	"gpt-4.1":           128000,
	"gpt-4o-mini":       128000,
	"gpt-4o":            128000,
	"gpt-4-turbo":       128000,
	"gpt-4-32k":         32768,
	"gpt-4":             8192,
	"gpt-3.5-16k":       16384,
	"gpt-3.5-turbo-16k": 16384,
	"gpt-3.5":           4096,
	// GPT-5 family (400K context window)
	"gpt-5.2-pro": 400000,
	"gpt-5.2":     400000,
	"gpt-5.1":     400000,
	"gpt-5-pro":   400000,
	"gpt-5-nano":  400000,
	"gpt-5-mini":  400000,
	"gpt-5":       400000,
	// o-series reasoning models (200K context window)
	"o3-pro":  200000,
	"o3-mini": 200000,
	"o3":      200000,
	"o4-mini": 200000,
	"o1":      200000,
}

var MODEL_ENCODER_MAP = map[string]tokenizer.Model{
	// 3.5 family
	"gpt-3.5":           tokenizer.GPT35Turbo,
	"gpt-3.5-16k":       tokenizer.GPT35Turbo,
	"gpt-3.5-turbo-16k": tokenizer.GPT35Turbo,
	// 4 family
	"gpt-4":        tokenizer.GPT4,
	"gpt-4-32k":    tokenizer.GPT4,
	"gpt-4-turbo":  tokenizer.GPT4,
	"gpt-4o":       tokenizer.GPT4,
	"gpt-4o-mini":  tokenizer.GPT4,
	"gpt-4.1":      tokenizer.GPT4,
	"gpt-4.1-mini": tokenizer.GPT4,
	// 5 family (fallback to GPT-4 encoding until tiktoken adds a specific one)
	"gpt-5":       tokenizer.GPT4,
	"gpt-5-mini":  tokenizer.GPT4,
	"gpt-5-nano":  tokenizer.GPT4,
	"gpt-5-pro":   tokenizer.GPT4,
	"gpt-5.1":     tokenizer.GPT4,
	"gpt-5.2":     tokenizer.GPT4,
	"gpt-5.2-pro": tokenizer.GPT4,
	// o-series reasoning models (fallback to GPT-4 encoding)
	"o1":      tokenizer.GPT4,
	"o3-mini": tokenizer.GPT4,
	"o3":      tokenizer.GPT4,
	"o3-pro":  tokenizer.GPT4,
	"o4-mini": tokenizer.GPT4,
}

// EncoderForModel will return the encoder to use for
// the passed model
func EncoderForModel(model string) tokenizer.Model {
	// Pick the encoder using the longest prefix match, falling back to GPT-3.5 Turbo
	modelToReturn := tokenizer.GPT35Turbo
	bestLen := 0
	for modelPrefix, encoder := range MODEL_ENCODER_MAP {
		if strings.HasPrefix(model, modelPrefix) {
			if len(modelPrefix) > bestLen {
				bestLen = len(modelPrefix)
				modelToReturn = encoder
			}
		}
	}

	return modelToReturn
}

// CalculateTokens will use a naive way to calculate the
// number of tokens for the passed string by simply
// calculating the length of the string and dividing it by
// 4.
func CalculateTokens(stringToWorkOn string, model string) (int, error) {
	// Determine the model to encode with
	encodeWithModel := EncoderForModel(model)
	tokenizerCodec, err := tokenizer.ForModel(encodeWithModel)
	if err != nil {
		return 0, err
	}

	_, tokenCount, encodeErr := tokenizerCodec.Encode(stringToWorkOn)
	if encodeErr != nil {
		return 0, encodeErr
	}

	return len(tokenCount), nil
}

// LimitForModel will return the token limit provided by OpenAI
// for their different models that we support using for now
//
// If model is not found then -1 will be returned indicating that
// we don't know what a valid limit is for the passed model.
func LimitForModel(model string) int {
	// Choose the limit using the longest prefix match to avoid ambiguous matches
	bestLen := 0
	limitToReturn := -1
	for modelPrefix, limit := range MODEL_LIMIT_MAP {
		if strings.HasPrefix(model, modelPrefix) {
			if len(modelPrefix) > bestLen {
				bestLen = len(modelPrefix)
				limitToReturn = limit
			}
		}
	}

	return limitToReturn
}

// CheckLimit will check the token limit for the passed messages
// context and accordingly return the results
func CheckLimit(messages []ChatGPTMessage, model string) int {
	// We will need to iterate each message and calculate the token count
	// for that message.
	//
	// For more details on the token calculation, take a look at this
	// file: https://github.com/openai/openai-cookbook/blob/main/examples/How_to_count_tokens_with_tiktoken.ipynb
	// In the above file, the `Counting tokens for chat API calls` section should
	// be checked since it contains details about how tokens are counted for
	// messages.

	tokensPerMessage := 0
	// tokensPerName := 0
	if strings.HasPrefix(model, "gpt-3.5-turbo") {
		tokensPerMessage = 4 // every message follows <|start|>{role/name}\n{content}<|end|>\n
		// tokensPerName = -1   // if there's a name, the role is omitted
	} else if strings.HasPrefix(model, "gpt-4") || strings.HasPrefix(model, "gpt-5") || strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4") {
		tokensPerMessage = 3
		// tokensPerName = 1
	}

	tokenCount := 0
	for _, messageCtx := range messages {
		tokenCount += tokensPerMessage

		tokenCountEach, calculateErr := CalculateTokens(messageCtx.Content, model)
		if calculateErr != nil {
			log.Warnln(logTag, ": error while calculating token for message: ", messageCtx.Content, ", with err: ", calculateErr.Error())
			continue
		}

		tokenCount += tokenCountEach

		// Calculate the role tokens as well
		tokenCountEachRole, roleTokenCalculateErr := CalculateTokens(messageCtx.Role, model)
		if roleTokenCalculateErr != nil {
			log.Warnln(logTag, ": error while calculating token for role: ", messageCtx.Role, ", with err: ", roleTokenCalculateErr.Error())
			continue
		}

		tokenCount += tokenCountEachRole
	}

	// every reply is primed with <|start|>assistant<|message|>
	tokenCount += 3
	return tokenCount
}

// CalculateTokensForMessages will calculate the total toke count
// for the passed messages
func CalculateTokensForMessages(messages []map[string]interface{}, model string) int {
	// Unmarshal the map into a ChatGPTMessage type array
	messagesInBytes, marshalErr := json.Marshal(messages)
	if marshalErr != nil {
		return 0
	}

	messagesInType := make([]ChatGPTMessage, 0)
	unmarshalErr := json.Unmarshal(messagesInBytes, &messagesInType)
	if unmarshalErr != nil {
		return 0
	}

	return CheckLimit(messagesInType, model)
}

// TrimMessagesAsPerLimit will trim the messages array passed as per
// the passed limit
//
// messages should be an array of ChatGPTMessage object
func TrimMessagesAsPerModel(messages []ChatGPTMessage, model string, startTrimmingAt int) ([]ChatGPTMessage, error) {
	// Check if the model is valid and extract the limit for the model
	allowedLimit := LimitForModel(model)
	if allowedLimit == -1 {
		return messages, fmt.Errorf("`%s`: passed model is either invalid or not supported by ReactiveSearch yet", model)
	}

	// Reserve 100 tokens for the prompt response as well
	allowedLimit -= 100

	// Since we know the allowedLimit now, we can calculate the limit for the
	// passed messages.
	calculatedLimit := CheckLimit(messages, model)

	if calculatedLimit <= allowedLimit {
		// We don't have to worry about trimming since the messages are within
		// allowed limit for the passed model.
		return messages, nil
	}

	// Add a check to make sure startTrimmingAt is valid
	if startTrimmingAt < 0 || startTrimmingAt >= len(messages) {
		return messages, fmt.Errorf("invalid value passed for `startTrimmingAt`, should be between 0 and length of messages")
	}

	// Run the following loop in a timer so that in case of worst case
	// scenarios, it doesn't go for an infinite loop
	timerStart := time.Now()

	// We will have to consider trimming the messages in order to
	// keep the messages under the token limit
	for time.Since(timerStart).Seconds() <= 30 {
		// We cannot trim the last position since that is the
		// final message added
		if startTrimmingAt == len(messages)-1 {
			break
		}

		// We can trim at the starting position and re-calculate
		removeTill := startTrimmingAt + 1

		// We also want to remove the response from AI
		if (removeTill+1) < len(messages) && messages[removeTill+1].Role == "assistant" {
			removeTill += 1
		}

		// Remove 1 or 2 messages from the trimming position
		messages = append(messages[:startTrimmingAt], messages[removeTill:]...)

		// Calculate the limit and check again
		newLimit := CheckLimit(messages, model)
		if newLimit <= allowedLimit {
			break
		}
	}

	return messages, nil
}

// TrimContextAsPerLimits will trim the passed context based on the
// maxTokens, minTokens values passed by the user.
func (o *OpenAI) TrimContextAsPerLimits(messagesArrToPassChatGPT []map[string]interface{}, maxTokens int, minTokens int, strictSelection bool) ([]map[string]interface{}, int, error) {
	// Get the token limit for the model
	tokenLimitForModel := LimitForModel(o.GetConfig().GetModel())

	// While trimming, we cannot remove the first and last element with an exceptional case
	// being that we cannot remove the second element as well if `strictSelection` is `true`.
	//
	// However, as a rule of thumb, we can start trimming at the n - 2 index position and go till
	// 3rd without making a check.
	//
	// Once we reach, 2nd, we will do the `strictSelection` check and accordingly remove the
	// 2nd element or not.
	totalMessages := len(messagesArrToPassChatGPT)
	trimAtLocation := totalMessages - 2

	limitCalculated := tokenLimitForModel - CalculateTokensForMessages(messagesArrToPassChatGPT, o.GetConfig().GetModel())
	for limitCalculated < minTokens {
		if trimAtLocation == 0 {
			// We cannot trim the first message which is a system prompt
			break
		}

		if trimAtLocation == 1 && strictSelection {
			// We are at the second element's position now and we cannot
			// continue since `strictSelection` is `true`, thus there
			// are no more messages we can trim.
			break
		}

		log.Debug(logTag, ": Removing element at location: ", trimAtLocation)
		messagesArrToPassChatGPT = append(messagesArrToPassChatGPT[:trimAtLocation], messagesArrToPassChatGPT[trimAtLocation+1:]...)
		trimAtLocation -= 1

		limitCalculated = tokenLimitForModel - CalculateTokensForMessages(messagesArrToPassChatGPT, o.GetConfig().GetModel())
	}

	if minTokens > limitCalculated {
		// Throw error here
		errMsg := fmt.Sprintf("cannot continue as allowed token limit is below minTokens (%d) with context size (%d)", minTokens, limitCalculated)
		log.Warnln(logTag, ": ", errMsg)
		return messagesArrToPassChatGPT, maxTokens, fmt.Errorf("%s", errMsg)
	}

	// Calculate the maxTokens to use
	maxTokensToUse := int(math.Min(float64(maxTokens), float64(limitCalculated)))

	return messagesArrToPassChatGPT, maxTokensToUse, nil
}
