package openai

import (
	"encoding/json"
	"reflect"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func templateMessages() []ChatGPTMessage {
	return []ChatGPTMessage{
		{
			Role:    "system",
			Content: "You're a helpful assistant.",
		},
		{
			Role:    "system",
			Content: "Image Medium is https://s.gr-assets.com/assets/nophoto/book/111x148-bcc042a9c91a29c1d680899eff700a03.png, Title is The Batman Chronicles, Vol. 1, Language Code is en-US, Image is https://s.gr-assets.com/assets/nophoto/book/111x148-bcc042a9c91a29c1d680899eff700a03.png, Authors is Bill Finger, Gardner F. Fox, Bob Kane, Jerry Robinson, Sheldon Moldoff, Original Title is Batman Chronicles: Volume 1, Isbn is 1401204457, Original Series is ",
		},
		{
			Role:    "system",
			Content: "Image Medium is https://images.gr-assets.com/books/1344369854m/12791521.jpg, Title is Batman: Earth One, Volume 1, Language Code is eng, Image is https://images.gr-assets.com/books/1344369854l/12791521.jpg, Authors is Geoff Johns, Gary Frank, Jon Sibal, Brad Anderson, Rob Leigh, Original Title is Batman: Earth One, Volume 1, Isbn is 1401232086, Original Series is ",
		},
		{
			Role:    "system",
			Content: "Image Medium is https://s.gr-assets.com/assets/nophoto/book/111x148-bcc042a9c91a29c1d680899eff700a03.png, Title is Chronicles, Vol. 1, Language Code is eng, Image is https://s.gr-assets.com/assets/nophoto/book/111x148-bcc042a9c91a29c1d680899eff700a03.png, Authors is Bob Dylan, Original Title is Chronicles: Volume One, Isbn is 743244583, Original Series is ",
		},
		{
			Role:    "user",
			Content: "Answer the query: 'batman chronicles: volume 1'. Think step-by-step, cite the source after the answer and ensure the source is from the provided context.",
		},
	}
}

func TestTokenCountCalculation(t *testing.T) {
	Convey("GPT-3.5 Turbo token calculation", t, func() {
		messageToPass := "Here is a test message that is not very long and some extra stuff just for fun"
		modelToUse := "gpt-3.5-turbo"
		tokenCount, calculateErr := CalculateTokens(messageToPass, modelToUse)
		So(calculateErr, ShouldBeNil)
		So(tokenCount, ShouldEqual, 17)
	})
	Convey("GPT-4 token calculation", t, func() {
		messageToPass := "Here is a test message that is not very long and some extra stuff just for fun"
		modelToUse := "gpt-4"
		tokenCount, calculateErr := CalculateTokens(messageToPass, modelToUse)
		So(calculateErr, ShouldBeNil)
		So(tokenCount, ShouldEqual, 17)
	})
	Convey("GPT-3.5 Turbo 16k token calculation", t, func() {
		messageToPass := "Here is a test message that is not very long and some extra stuff just for fun"
		modelToUse := "gpt-3.5-turbo-16k"
		tokenCount, calculateErr := CalculateTokens(messageToPass, modelToUse)
		So(calculateErr, ShouldBeNil)
		So(tokenCount, ShouldEqual, 17)
	})
	Convey("GPT-4-32k token calculation", t, func() {
		messageToPass := "Here is a test message that is not very long and some extra stuff just for fun"
		modelToUse := "gpt-4-32k"
		tokenCount, calculateErr := CalculateTokens(messageToPass, modelToUse)
		So(calculateErr, ShouldBeNil)
		So(tokenCount, ShouldEqual, 17)
	})
}

func TestTrimMessagesAsPerModel(t *testing.T) {
	startingMessages := templateMessages()
	Convey("Within Limit messages", t, func() {
		messagesToPass := startingMessages
		updatedMessages, trimErr := TrimMessagesAsPerModel(messagesToPass, "gpt-3.5-turbo", 4)
		So(trimErr, ShouldBeNil)
		So(reflect.DeepEqual(messagesToPass, updatedMessages), ShouldBeTrue)
	})
	Convey("when token limit exceeds", t, func() {
		messagesToPass := startingMessages
		counter := 0
		for counter <= 30 {
			messagesToPass = append(messagesToPass, []ChatGPTMessage{
				{
					Role:    "assistant",
					Content: "Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum.",
				},
				{
					Role:    "user",
					Content: "What does the above even mean?",
				},
			}...)
			counter += 1
		}
		updatedMessages, trimErr := TrimMessagesAsPerModel(messagesToPass, "gpt-3.5-turbo", 4)

		So(trimErr, ShouldBeNil)
		So(len(updatedMessages), ShouldEqual, 58)
	})

	Convey("when token limit exceed with gpt-3.5-turbo-16k", t, func() {
		messagesToPass := startingMessages
		counter := 0
		for counter <= 30 {
			messagesToPass = append(messagesToPass, []ChatGPTMessage{
				{
					Role:    "assistant",
					Content: "Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum.",
				},
				{
					Role:    "user",
					Content: "What does the above even mean?",
				},
			}...)
			counter += 1
		}
		updatedMessages, trimErr := TrimMessagesAsPerModel(messagesToPass, "gpt-3.5-turbo-16k", 4)
		So(trimErr, ShouldBeNil)
		So(len(updatedMessages), ShouldEqual, 67)
	})

	Convey("when token limit exceed with gpt-4", t, func() {
		messagesToPass := startingMessages
		counter := 0
		for counter <= 50 {
			messagesToPass = append(messagesToPass, []ChatGPTMessage{
				{
					Role:    "assistant",
					Content: "Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum.",
				},
				{
					Role:    "user",
					Content: "What does the above even mean?",
				},
			}...)
			counter += 1
		}
		updatedMessages, trimErr := TrimMessagesAsPerModel(messagesToPass, "gpt-4", 4)

		So(trimErr, ShouldBeNil)
		So(len(updatedMessages), ShouldEqual, 107)
	})

	Convey("when token limit exceed with gpt-4-32k", t, func() {
		messagesToPass := startingMessages
		counter := 0
		for counter <= 50 {
			messagesToPass = append(messagesToPass, []ChatGPTMessage{
				{
					Role:    "assistant",
					Content: "Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum.",
				},
				{
					Role:    "user",
					Content: "What does the above even mean?",
				},
			}...)
			counter += 1
		}
		updatedMessages, trimErr := TrimMessagesAsPerModel(messagesToPass, "gpt-4-32k", 4)

		So(trimErr, ShouldBeNil)
		So(len(updatedMessages), ShouldEqual, 107)
	})
}

func TestMessageCountCalculation(t *testing.T) {
	startingMessages := []map[string]interface{}{
		{
			"content": "You're a helpful assistant.",
			"role":    "system",
		},
		{
			"content": "Free Guy is A bank teller called Guy realizes he is a background character in an open world video game called Free City that will soon go offline. with url as https://www.themoviedb.org/t/p/w1280/8Y43POKjjKDGI9MH89NW0NAzzp8.jpg",
			"role":    "system",
		},
		{
			"content": "Free Willy is When maladjusted orphan Jesse vandalizes a theme park, he is placed with foster parents and must work at the park to make amends. There he meets Willy, a young Orca whale who has been separated from his family. Sensing kinship, they form a bond and, with the help of kindly whale trainer Rae Lindley, develop a routine of tricks. However, greedy park owner Dial soon catches wind of the duo and makes plans to profit from them. with url as https://www.themoviedb.org/t/p/w1280/z2oqTYqFZys9YkAseiUd18Gqqto.jpg",
			"role":    "system",
		},
		{
			"content": "Free Fire is A crime drama set in 1970s Boston, about a gun sale which goes wrong. with url as https://www.themoviedb.org/t/p/w1280/qUg005Yer0DQOW3l45jrwP0rTvo.jpg",
			"role":    "system",
		},
		{
			"content": "Can you tell me about Free Guy?",
			"role":    "user",
		},
	}

	Convey("Count token limit for messages array", t, func() {
		tokenCount := CalculateTokensForMessages(startingMessages, "gpt-3.5-turbo")
		So(tokenCount, ShouldEqual, 305)
	})
	Convey("Check limit for messages array for gpt-4", t, func() {
		tokenCount := CalculateTokensForMessages(startingMessages, "gpt-4")
		So(tokenCount, ShouldEqual, 300)
	})
	Convey("Check limit for messages array for gpt-3.5-16k", t, func() {
		tokenCount := CalculateTokensForMessages(startingMessages, "gpt-3.5-turbo-16k")
		So(tokenCount, ShouldEqual, 305)
	})
	Convey("Check limit for messages array for gpt-4-32k", t, func() {
		tokenCount := CalculateTokensForMessages(startingMessages, "gpt-4-32k")
		So(tokenCount, ShouldEqual, 300)
	})
}

func TestTrimContextAsPerLimits(t *testing.T) {
	startingMessages := templateMessages()

	messagesAsBytes, _ := json.Marshal(startingMessages)
	messagesAsMap := make([]map[string]interface{}, 0)
	json.Unmarshal(messagesAsBytes, &messagesAsMap)

	openAIInstance := Instance()
	Convey("Within Limit messages 1 trimmed", t, func() {
		messagesToPass := messagesAsMap
		updatedMessages, maxTokensToUse, err := openAIInstance.TrimContextAsPerLimits(messagesToPass, 2048, 3700, false)

		So(err, ShouldBeNil)
		So(len(updatedMessages), ShouldEqual, len(messagesToPass)-1)
		So(maxTokensToUse, ShouldEqual, 2048)
	})

	Convey("Within Limit messages 2 trimmed", t, func() {
		messagesToPass := messagesAsMap
		updatedMessages, maxTokensToUse, err := openAIInstance.TrimContextAsPerLimits(messagesToPass, 2048, 3900, false)

		So(err, ShouldBeNil)
		So(len(updatedMessages), ShouldEqual, len(messagesToPass)-2)
		So(maxTokensToUse, ShouldEqual, 2048)
	})

	Convey("Within Limit messages 3 trimmed", t, func() {
		messagesToPass := messagesAsMap
		updatedMessages, maxTokensToUse, err := openAIInstance.TrimContextAsPerLimits(messagesToPass, 2048, 4000, false)

		So(err, ShouldBeNil)
		So(len(updatedMessages), ShouldEqual, len(messagesToPass)-3)
		So(maxTokensToUse, ShouldEqual, 2048)
	})

	Convey("Failed to get under limit with 2 trimmed and strictSelection is `true`", t, func() {
		messagesToPass := messagesAsMap
		updatedMessages, maxTokensToUse, err := openAIInstance.TrimContextAsPerLimits(messagesToPass, 2048, 4003, true)

		So(err, ShouldNotBeNil)
		So(len(updatedMessages), ShouldEqual, len(messagesToPass)-2)
		So(maxTokensToUse, ShouldEqual, 2048)
	})
}
