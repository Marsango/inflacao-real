// admin-password reads a password from stdin and emits an Argon2id PHC hash.
package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/argon2"
)

func main() {
	value, err := io.ReadAll(io.LimitReader(os.Stdin, 4097))
	if err != nil || len(value) == 0 || len(value) > 4096 {
		fmt.Fprintln(os.Stderr, "provide 1–4096 password bytes on stdin")
		os.Exit(1)
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		fmt.Fprintln(os.Stderr, "generate salt:", err)
		os.Exit(1)
	}
	digest := argon2.IDKey(value, salt, 3, 65536, 1, 32)
	fmt.Printf("$argon2id$v=19$m=65536,t=3,p=1$%s$%s\n", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(digest))
}
