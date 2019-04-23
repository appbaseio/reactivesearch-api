package proxy

type proxyService interface {
	sendRequest(url, method string, reqBody []byte) ([]byte, int, error)
	getArcID() (string, error)
	getEmail() (string, error)
	getSubID() (string, error)
}
