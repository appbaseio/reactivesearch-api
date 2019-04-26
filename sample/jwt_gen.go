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
	
        private_key_loc := os.Getenv("JWT_RSA_PRIVATE_KEY_LOC")
	if private_key_loc == "" {
		private_key_loc = "sample/rsa-private"
        }
	buf, err1 := ioutil.ReadFile(private_key_loc)
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
