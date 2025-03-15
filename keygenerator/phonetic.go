package keygenerator

import (
	"crypto/rand"
	"math/big"
	"strings"
)

type PhoneticKeyGenerator struct {
}

var vowels string = "aeiou"
var consonants string = "bcdfghjklmnpqrstvwxyz"

var twoBig = big.NewInt(2)

func randomFromStr(str string) byte {
	maxRandomIndex := big.NewInt(int64(len(str)))
	index, _ := rand.Int(rand.Reader, maxRandomIndex)
	return str[index.Int64()]
}

func NewPhoneticKeyGenerator() *PhoneticKeyGenerator {
	return &PhoneticKeyGenerator{}
}

func (p *PhoneticKeyGenerator) Generate(length int) string {
	var out strings.Builder
	defer out.Reset()

	start, err := rand.Int(rand.Reader, twoBig)
	if err != nil {
		return ""
	}

	for i := 0; i < length; i++ {
		if i%2 == int(start.Int64()) {
			out.WriteByte(randomFromStr(consonants))
		} else {
			out.WriteByte(randomFromStr(vowels))
		}
	}

	return out.String()
}
