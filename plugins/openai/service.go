package openai

import "context"

type openaiService interface {
	getSettings(ctx context.Context) (OpenAIConfig, error)
	saveSettings(openAISettings OpenAIConfig, ctx context.Context) error
}

type openaiAnalyticsService interface {
	saveSession(ctx context.Context, sessionDetails AISessionDoc, sessionId string) error
	getSession(ctx context.Context, sessionId string) (*AISessionDoc, error)
	getAISessionAnalytics(ctx context.Context, from, to int64, size int) ([]byte, error)
	filterAISessionAnalytics(ctx context.Context, queryParams FilterQueryParams) ([]AISessionDoc, error)
}

type openaiFAQServiceEs interface {
	createFAQ(ctx context.Context, item FAQBody) error
	getFAQ(ctx context.Context, faqId string) ([]byte, error)
	deleteFAQ(ctx context.Context, faqId string) error
	getFAQs(ctx context.Context, from, size int) ([]byte, error)
	getFAQsBySearchBox(ctx context.Context, searchboxId string, from, size int) ([]byte, error)
	getFAQCount(ctx context.Context) (int64, error)
	getNextFAQOrder(ctx context.Context) (int, int64, error)
}

type openaiFAQService interface {
	createFAQZinc(item FAQBody) error
	getFAQZinc(faqId string) ([]byte, error)
	deleteFAQZinc(faqId string) error
	getFAQsZinc(from, size int) ([]byte, error)
	SyncFAQToZinc(index string, zincIndex string) error
}
