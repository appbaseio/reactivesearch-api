package main

import "fmt"
import "io/ioutil"
import "os"
import "time"
import "github.com/dgrijalva/jwt-go"

func main() {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		    "username": "foo",
		        "iat": time.Now().Unix(),
			"exp": time.Now().Unix() + 1000,
		})
        // generate rsa private key using ssh-keygen
        // or use sample/rsa-private
	buf, err1 := ioutil.ReadFile(os.Getenv("JWT_RSA_PRIVATE_KEY"))
	if err1 != nil {
		panic(err1)
	}
	pvt_key, err3 := jwt.ParseRSAPrivateKeyFromPEM(buf)
        if err3 != nil {
		panic(err3)
        }
	tokenString, err4 := token.SignedString(pvt_key)
	if err4  != nil {
		panic(err4)
	}
	fmt.Println(tokenString)
}
