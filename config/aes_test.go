package config

import (
	"fmt"
	"testing"
)

func Test_AES(t *testing.T) {
	decrypt := AesDecrypt("F5BM56YG+SoN4wm0Wo5Mzw==", Key)
	fmt.Println(decrypt)
}
