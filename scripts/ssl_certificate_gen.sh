openssl req -x509 -new -nodes -key rootCA.key -sha256 -days 1024 -out rootCA.pem
openssl req -new -sha256 -nodes -out server.csr -newkey rsa:2048 -keyout server.key
openssl x509 -req -in server.csr -CA rootCA.pem -CAkey rootCA.key -CAcreateserial -out server.crt
