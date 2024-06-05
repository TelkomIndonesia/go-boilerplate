package main

import (
	"log"
	"os"

	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/jwt"
	"github.com/tink-crypto/tink-go/v2/keyderivation"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/mac"
	"github.com/tink-crypto/tink-go/v2/prf"
)

func main() {
	{
		aeadp := "aead.json"
		if len(os.Args) > 1 {
			aeadp = os.Args[1]
		}
		template, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), aead.AES128GCMKeyTemplate())
		if err != nil {
			log.Fatal(err)
		}
		h, err := keyset.NewHandle(template)
		if err != nil {
			log.Fatal(err)
		}
		aeadf, err := os.Create(aeadp)
		if err != nil {
			log.Fatal(err)
		}
		insecurecleartextkeyset.Write(h, keyset.NewJSONWriter(aeadf))
	}

	{

		macp := "mac.json"
		if len(os.Args) > 2 {
			macp = os.Args[2]
		}
		template, err := keyderivation.CreatePRFBasedKeyTemplate(prf.HKDFSHA256PRFKeyTemplate(), mac.HMACSHA256Tag256KeyTemplate())
		if err != nil {
			log.Fatal(err)
		}
		h, err := keyset.NewHandle(template)
		if err != nil {
			log.Fatal(err)
		}
		macf, err := os.Create(macp)
		if err != nil {
			log.Fatal(err)
		}
		insecurecleartextkeyset.Write(h, keyset.NewJSONWriter(macf))
	}

	{
		jwtmacp := "jwtmac.json"
		if len(os.Args) > 3 {
			jwtmacp = os.Args[3]
		}
		h, err := keyset.NewHandle(jwt.HS256Template())
		if err != nil {
			log.Fatal(err)
		}
		jwtmacf, err := os.Create(jwtmacp)
		if err != nil {
			log.Fatal(err)
		}
		insecurecleartextkeyset.Write(h, keyset.NewJSONWriter(jwtmacf))
	}

}
