package main

import "fmt"
import "io/ioutil"
import "os"
import "time"
import "strings"
import "flag"

import "github.com/dgrijalva/jwt-go"

func encode() {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			*keyFlag: *valueFlag,
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

func decode() {
	tokenString, err1 := ioutil.ReadAll(os.Stdin)
	if err1 != nil {
		panic(err1)
	}
	// generate the public key from the private key in pkcs8
        // using the command:
        // ssh-keygen -e -m pkcs8 -f *privatekeyloc*

	public_key_loc := os.Getenv("JWT_RSA_PUBLIC_KEY_LOC")
	if public_key_loc == "" {
		public_key_loc = "sample/rsa-public"
        }
	buf, err2 := ioutil.ReadFile(public_key_loc)
	if err2 != nil {
		panic(err2)
	}
	public_key, err4 := jwt.ParseRSAPublicKeyFromPEM(buf)
	if err4 != nil {
		panic(err4)
	}
	//token, err5 := jwt.Parse(string(tokenString), func(token *jwt.Token) (interface{}, error) {
	token, err5 := jwt.Parse(strings.TrimSpace(string(tokenString)), func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return public_key, nil

	})
	if err1 != nil || err2 != nil || err4 != nil || err5 != nil {
		fmt.Println(err1, err2, err4)
		if err6, ok := err5.(*jwt.ValidationError); ok {
			fmt.Println(err6.Inner, err6.Errors, err6.Error())
			fmt.Println(token.Signature)
			_, err7 := jwt.DecodeSegment(strings.TrimSpace(token.Signature))
			fmt.Println(err7)
		}
	}
	fmt.Println(token.Claims)
}

var decodeFlag = flag.Bool("decode", false, "decode the provided jwt")
var encodeFlag = flag.Bool("encode", false, "encode a jwt")
var valueFlag = flag.String("value", "foo", "provide the value")
var keyFlag = flag.String("key", "username", "provide the key")

func main() {
	flag.Parse()
	if *encodeFlag {
		encode()
	} else if *decodeFlag {
		decode()
	}
}
