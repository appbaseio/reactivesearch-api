package request

type contextKey string

// Maker is a key against which a request maker's identifier is stored.
const Maker = contextKey("request_maker")
